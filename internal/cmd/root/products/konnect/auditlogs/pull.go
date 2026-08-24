package auditlogs

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/kong/kongctl/internal/cmd"
	cmdcommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/cmd/output/columns"
	jqoutput "github.com/kong/kongctl/internal/cmd/output/jq"
	konnectcommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/konnect/apiutil"
	"github.com/kong/kongctl/internal/konnect/httpclient"
	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"
)

const (
	auditLogsPullPath       = "/v3/audit-logs"
	jsonLinesOutput         = "jsonl"
	defaultPollInterval     = 10 * time.Second
	defaultFollowLookback   = 5 * time.Minute
	defaultAuditLogLimit    = 50
	followOverlap           = time.Minute
	maximumAuditLogPageSize = 1000
)

// DefaultPullPageSize is the default number of audit-log records requested per API page.
const DefaultPullPageSize = 100

var auditLogNow = time.Now

type pullAuditLogsOptions struct {
	StartTime    string
	EndTime      string
	Since        time.Duration
	EventType    string
	Limit        int
	Follow       bool
	PollInterval time.Duration
}

type auditLogWindow struct {
	Start *time.Time
	End   *time.Time
}

type auditLogEnvelope struct {
	Metadata auditLogOutputMetadata `json:"metadata" yaml:"metadata"`
	Data     []json.RawMessage      `json:"data"     yaml:"data"`
}

type auditLogOutputMetadata struct {
	Count     int  `json:"count"     yaml:"count"`
	Truncated bool `json:"truncated" yaml:"truncated"`
}

type auditLogPage struct {
	Data       []json.RawMessage
	NextCursor string
}

type auditLogPullClient struct {
	baseURL     string
	tokenSource apiutil.TokenSource
	httpClient  apiutil.Doer
}

type auditLogRequest struct {
	Window   auditLogWindow
	Type     string
	PageSize int
	After    string
}

type auditLogResponseError struct {
	status int
	body   string
}

func (e *auditLogResponseError) Error() string {
	if e.body == "" {
		return fmt.Sprintf("audit-log request failed with status %d", e.status)
	}
	return fmt.Sprintf("audit-log request failed with status %d: %s", e.status, e.body)
}

func addPullAuditLogsFlags(command *cobra.Command, options *pullAuditLogsOptions) {
	command.Flags().StringVar(&options.StartTime, "start-time", options.StartTime,
		`Inclusive RFC3339 lower bound for event timestamps.
Accepts UTC (Z) or a numeric UTC offset.
- UTC example   : [ 2026-08-23T14:00:00Z ]
- Offset example: [ 2026-08-23T09:00:00-05:00 ]`)
	command.Flags().StringVar(&options.EndTime, "end-time", options.EndTime,
		`Inclusive RFC3339 upper bound for event timestamps.
Accepts UTC (Z) or a numeric UTC offset.
- UTC example   : [ 2026-08-24T14:00:00Z ]
- Offset example: [ 2026-08-24T09:00:00-05:00 ]`)
	command.Flags().DurationVar(&options.Since, "since", options.Since,
		`Retrieve events from the specified lookback period.
- Examples: [ 30s, 15m, 2h, 24h, 168h, 1h30m ]`)
	command.Flags().StringVar(&options.EventType, "type", options.EventType,
		"Filter by event type: authentication, authorization, or gateway_access.")
	command.Flags().IntVar(&options.Limit, "limit", options.Limit,
		`Maximum total events to return.
Defaults to 50 when no time window is specified.
Time-window queries are unlimited unless --limit is specified.
Set to 0 for unlimited.`)
	command.Flags().BoolVarP(&options.Follow, "follow", "F", options.Follow,
		"Poll continuously for new events until interrupted.")
	command.Flags().DurationVar(&options.PollInterval, "poll-interval", options.PollInterval,
		"Interval between successful polling cycles in follow mode.")
	jqoutput.AddFlags(command.Flags())
	columns.AddFlags(command.Flags())
	cmdcommon.AllowExtraOutputFormats(command, jsonLinesOutput)
}

// ConfigureTailPullCommand configures a command as a continuous audit-log pull.
func ConfigureTailPullCommand(command *cobra.Command, addListener bool) *cobra.Command {
	options := pullAuditLogsOptions{
		Follow:       true,
		PollInterval: defaultPollInterval,
	}
	command.Short = "Follow Konnect organization audit logs"
	command.Long = `Retrieve a five-minute audit-log catch-up window, then poll for new
events until interrupted. Use the listener child for the webhook-based flow.`
	command.Example = `  # Follow organization audit logs
  kongctl tail audit-logs

  # Follow as JSON Lines
  kongctl tail audit-logs --output jsonl

  # Use the webhook listener flow
  kongctl tail audit-logs listener --endpoint https://example.test/audit-logs --authorization "Bearer <token>"`
	command.RunE = func(cmdObj *cobra.Command, args []string) error {
		return executePullAuditLogs(cmdObj, args, options)
	}
	addPullAuditLogsFlags(command, &options)
	if addListener {
		command.AddCommand(newTailListenerAuditLogsCmd())
	}
	return command
}

func executePullAuditLogs(cobraCmd *cobra.Command, args []string, options pullAuditLogsOptions) error {
	helper := cmd.BuildHelper(cobraCmd, args)
	if len(args) != 0 {
		return &cmd.ConfigurationError{Err: errors.New("the audit-logs command does not accept positional arguments")}
	}

	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}
	output, err := auditLogOutputFormat(cobraCmd, cfg)
	if err != nil {
		return &cmd.ConfigurationError{Err: err}
	}
	if err := validateAuditLogOutputOptions(cobraCmd, cfg, output); err != nil {
		return &cmd.ConfigurationError{Err: err}
	}
	pageSize := cfg.GetInt(konnectcommon.RequestPageSizeConfigPath)
	if pageSize == 0 {
		pageSize = DefaultPullPageSize
	}
	options.Limit = resolveAuditLogLimit(cobraCmd, options)
	if err := validatePullAuditLogsOptions(cobraCmd, options, output, pageSize); err != nil {
		return &cmd.ConfigurationError{Err: err}
	}

	now := auditLogNow().UTC()
	window, err := resolveAuditLogWindow(options, now)
	if err != nil {
		return &cmd.ConfigurationError{Err: err}
	}
	client, err := newAuditLogPullClient(helper, cfg)
	if err != nil {
		return cmd.PrepareExecutionError("failed to prepare audit-log retrieval", err, cobraCmd)
	}

	if options.Follow {
		return runAuditLogFollow(helper, client, options, output, pageSize, window)
	}
	return runFiniteAuditLogPull(helper, client, options, output, pageSize, window)
}

func resolveAuditLogLimit(command *cobra.Command, options pullAuditLogsOptions) int {
	if options.Follow || options.Since > 0 || strings.TrimSpace(options.StartTime) != "" ||
		strings.TrimSpace(options.EndTime) != "" {
		return options.Limit
	}
	if command != nil && command.Flags().Changed("limit") {
		return options.Limit
	}
	return defaultAuditLogLimit
}

func validatePullAuditLogsOptions(
	cobraCmd *cobra.Command,
	options pullAuditLogsOptions,
	output string,
	pageSize int,
) error {
	if pageSize < 1 || pageSize > maximumAuditLogPageSize {
		return fmt.Errorf("--%s must be between 1 and %d", konnectcommon.RequestPageSizeFlagName,
			maximumAuditLogPageSize)
	}
	if options.Since < 0 {
		return errors.New("--since must be positive")
	}
	if options.Since > 0 && (strings.TrimSpace(options.StartTime) != "" || strings.TrimSpace(options.EndTime) != "") {
		return errors.New("--since cannot be combined with --start-time or --end-time")
	}
	if options.Limit < 0 {
		return errors.New("--limit must be non-negative")
	}
	if options.PollInterval <= 0 {
		return errors.New("--poll-interval must be positive")
	}
	if options.EventType != "" && !slices.Contains([]string{"authentication", "authorization", "gateway_access"},
		options.EventType) {
		return fmt.Errorf("invalid --type %q: must be authentication, authorization, or gateway_access", options.EventType)
	}
	if options.Follow {
		if strings.TrimSpace(options.EndTime) != "" {
			return errors.New("--end-time cannot be used with --follow")
		}
		if options.Limit != 0 {
			return errors.New("--limit cannot be used with --follow")
		}
		if output != cmdcommon.TEXT.String() && output != jsonLinesOutput {
			return fmt.Errorf("--follow requires row-oriented output: use --output text or --output jsonl")
		}
	}
	if output == jsonLinesOutput {
		if values, _ := cobraCmd.Flags().GetStringArray(columns.FlagName); len(values) > 0 {
			return errors.New("--columns is only supported with --output text")
		}
	}
	return nil
}

func validateAuditLogOutputOptions(command *cobra.Command, cfg config.Hook, output string) error {
	settings, err := jqoutput.ResolveSettings(command, cfg)
	if err != nil {
		return err
	}
	if output == jsonLinesOutput {
		if settings.RawOutput && !jqoutput.HasFilter(settings) {
			return fmt.Errorf("--%s requires --%s", jqoutput.RawOutputFlagName, jqoutput.FlagName)
		}
		if values, _ := command.Flags().GetStringArray(columns.FlagName); len(values) > 0 {
			return errors.New("--columns is only supported with --output text")
		}
		return nil
	}

	format, err := cmdcommon.OutputFormatStringToIota(output)
	if err != nil {
		return err
	}
	if err := jqoutput.ValidateOutputFormat(format, settings); err != nil {
		return err
	}
	selected, err := columns.Resolve(command, format)
	if err != nil {
		return err
	}
	if len(selected) > 0 && jqoutput.HasFilter(settings) {
		return fmt.Errorf("--%s cannot be combined with --%s", columns.FlagName, jqoutput.FlagName)
	}
	return nil
}

func resolveAuditLogWindow(options pullAuditLogsOptions, now time.Time) (auditLogWindow, error) {
	window := auditLogWindow{}
	if options.Since > 0 {
		start := now.Add(-options.Since)
		window.Start = &start
		window.End = &now
		return window, nil
	}
	if strings.TrimSpace(options.StartTime) != "" {
		start, err := time.Parse(time.RFC3339, options.StartTime)
		if err != nil {
			return window, fmt.Errorf("invalid --start-time: expected RFC3339: %w", err)
		}
		start = start.UTC()
		window.Start = &start
	}
	if strings.TrimSpace(options.EndTime) != "" {
		end, err := time.Parse(time.RFC3339, options.EndTime)
		if err != nil {
			return window, fmt.Errorf("invalid --end-time: expected RFC3339: %w", err)
		}
		end = end.UTC()
		window.End = &end
	}
	if window.Start != nil && window.End != nil && window.Start.After(*window.End) {
		return window, errors.New("--start-time must not be later than --end-time")
	}
	if options.Follow {
		if window.Start == nil {
			start := now.Add(-defaultFollowLookback)
			window.Start = &start
		}
		window.End = &now
	}
	return window, nil
}

func newAuditLogPullClient(helper cmd.Helper, cfg config.Hook) (*auditLogPullClient, error) {
	logger, err := helper.GetLogger()
	if err != nil {
		return nil, err
	}
	tokenSource, err := konnectcommon.GetAccessTokenSource(cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("resolve Konnect access token: %w", err)
	}
	baseURL, err := konnectcommon.ResolveBaseURL(cfg)
	if err != nil {
		return nil, fmt.Errorf("resolve Konnect base URL: %w", err)
	}
	return &auditLogPullClient{
		baseURL:     baseURL,
		tokenSource: tokenSource,
		httpClient:  httpclient.NewLoggingHTTPClient(logger),
	}, nil
}

func (c *auditLogPullClient) fetchPage(ctx context.Context, request auditLogRequest) (auditLogPage, error) {
	query := url.Values{}
	query.Set("page[size]", strconv.Itoa(request.PageSize))
	if request.Window.Start != nil {
		query.Set("filter[ts][gte]", request.Window.Start.UTC().Format(time.RFC3339Nano))
	}
	if request.Window.End != nil {
		query.Set("filter[ts][lte]", request.Window.End.UTC().Format(time.RFC3339Nano))
	}
	if request.Type != "" {
		query.Set("filter[type]", request.Type)
	}
	if request.After != "" {
		query.Set("page[after]", request.After)
	}

	result, err := apiutil.RequestWithTokenSource(ctx, c.httpClient, http.MethodGet, c.baseURL,
		auditLogsPullPath+"?"+query.Encode(), c.tokenSource, nil, nil)
	if err != nil {
		return auditLogPage{}, err
	}
	if result.StatusCode < http.StatusOK || result.StatusCode >= http.StatusMultipleChoices {
		return auditLogPage{}, &auditLogResponseError{
			status: result.StatusCode,
			body:   strings.TrimSpace(string(result.Body)),
		}
	}
	return decodeAuditLogPage(result.Body)
}

func decodeAuditLogPage(body []byte) (auditLogPage, error) {
	var payload struct {
		Data []json.RawMessage `json:"data"`
		Meta json.RawMessage   `json:"meta"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return auditLogPage{}, fmt.Errorf("decode audit-log response: %w", err)
	}
	if payload.Data == nil {
		return auditLogPage{}, errors.New("decode audit-log response: missing data array")
	}
	nextCursor, err := extractNextCursor(payload.Meta)
	if err != nil {
		return auditLogPage{}, fmt.Errorf("decode audit-log response: %w", err)
	}
	return auditLogPage{Data: payload.Data, NextCursor: nextCursor}, nil
}

func extractNextCursor(meta json.RawMessage) (string, error) {
	var value map[string]any
	if len(meta) == 0 {
		return "", errors.New("missing pagination metadata")
	}
	if err := json.Unmarshal(meta, &value); err != nil {
		return "", fmt.Errorf("invalid pagination metadata: %w", err)
	}
	if rawNext, exists := value["next"]; exists && rawNext != nil {
		next, ok := rawNext.(string)
		if !ok {
			return "", errors.New("pagination meta.next must be a string")
		}
		return normalizeAuditLogCursor(next)
	}
	if rawPage, exists := value["page"]; exists && rawPage != nil {
		page, ok := rawPage.(map[string]any)
		if !ok {
			return "", errors.New("pagination meta.page must be an object")
		}
		if rawNext, exists := page["next"]; exists && rawNext != nil {
			next, ok := rawNext.(string)
			if !ok {
				return "", errors.New("pagination meta.page.next must be a string")
			}
			return normalizeAuditLogCursor(next)
		}
	}
	return "", nil
}

func normalizeAuditLogCursor(next string) (string, error) {
	next = strings.TrimSpace(next)
	if next == "" {
		return "", nil
	}
	parsed, err := url.Parse(next)
	if err != nil {
		return "", fmt.Errorf("invalid next-page cursor: %w", err)
	}
	if parsed.IsAbs() || strings.HasPrefix(next, "/") {
		if cursor := strings.TrimSpace(parsed.Query().Get("page[after]")); cursor != "" {
			return cursor, nil
		}
		return "", errors.New("next-page link does not contain page[after]")
	}
	return next, nil
}

func fetchAllAuditLogPages(
	ctx context.Context,
	client *auditLogPullClient,
	request auditLogRequest,
	limit int,
	onPage func([]json.RawMessage) error,
) (int, bool, error) {
	count := 0
	seenCursors := map[string]struct{}{}
	for {
		page, err := client.fetchPage(ctx, request)
		if err != nil {
			return count, false, err
		}
		records := page.Data
		if limit > 0 && count+len(records) > limit {
			records = records[:limit-count]
		}
		if len(records) > 0 {
			if err := onPage(records); err != nil {
				return count, false, err
			}
			count += len(records)
		}
		if limit > 0 && count == limit {
			return count, page.NextCursor != "" || len(page.Data) > len(records), nil
		}
		if page.NextCursor == "" {
			return count, false, nil
		}
		if _, exists := seenCursors[page.NextCursor]; exists {
			return count, false, fmt.Errorf("audit-log API returned repeated cursor %q", page.NextCursor)
		}
		seenCursors[page.NextCursor] = struct{}{}
		request.After = page.NextCursor
	}
}

func runFiniteAuditLogPull(
	helper cmd.Helper,
	client *auditLogPullClient,
	options pullAuditLogsOptions,
	output string,
	pageSize int,
	window auditLogWindow,
) error {
	request := auditLogRequest{Window: window, Type: options.EventType, PageSize: pageSize}
	if output == jsonLinesOutput {
		_, _, err := fetchAllAuditLogPages(helper.GetContext(), client, request, options.Limit,
			func(records []json.RawMessage) error { return renderAuditLogJSONLines(helper, records) })
		if err != nil {
			return cmd.PrepareExecutionError("failed to retrieve all audit logs", err, helper.GetCmd())
		}
		return nil
	}

	records := make([]json.RawMessage, 0)
	count, truncated, err := fetchAllAuditLogPages(helper.GetContext(), client, request, options.Limit,
		func(page []json.RawMessage) error {
			records = append(records, page...)
			return nil
		})
	if err != nil {
		return cmd.PrepareExecutionError("failed to retrieve all audit logs", err, helper.GetCmd())
	}
	return renderFiniteAuditLogs(helper, output, auditLogEnvelope{
		Metadata: auditLogOutputMetadata{Count: count, Truncated: truncated},
		Data:     records,
	})
}

func renderFiniteAuditLogs(helper cmd.Helper, output string, envelope auditLogEnvelope) error {
	streams := helper.GetStreams()
	switch output {
	case cmdcommon.JSON.String():
		payload, handled, err := resolveOutputPayload(helper, cmdcommon.JSON, envelope)
		if err != nil || handled {
			return err
		}
		encoder := json.NewEncoder(streams.Out)
		encoder.SetIndent("", "  ")
		return encoder.Encode(payload)
	case cmdcommon.YAML.String():
		payload, handled, err := resolveOutputPayload(helper, cmdcommon.YAML, envelope)
		if err != nil || handled {
			return err
		}
		data, err := yaml.Marshal(payload)
		if err != nil {
			return err
		}
		_, err = streams.Out.Write(data)
		return err
	case cmdcommon.TEXT.String():
		return renderAuditLogText(helper, envelope.Data)
	default:
		return fmt.Errorf("unsupported audit-log output format %q", output)
	}
}

func renderAuditLogJSONLines(helper cmd.Helper, records []json.RawMessage) error {
	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}
	settings, err := jqoutput.ResolveSettings(helper.GetCmd(), cfg)
	if err != nil {
		return err
	}
	out := helper.GetStreams().Out
	for _, raw := range records {
		if jqoutput.HasFilter(settings) {
			if settings.RawOutput {
				if err := jqoutput.ApplyRawFilter(raw, settings.Filter, out); err != nil {
					return err
				}
				continue
			}
			filtered, err := jqoutput.ApplyFilter(raw, settings.Filter)
			if err != nil {
				return err
			}
			raw = filtered
		}
		if _, err := fmt.Fprintln(out, strings.TrimSpace(string(raw))); err != nil {
			return err
		}
	}
	return nil
}

func renderAuditLogText(helper cmd.Helper, records []json.RawMessage) error {
	selected, err := columns.Resolve(helper.GetCmd(), cmdcommon.TEXT)
	if err != nil {
		return &cmd.ConfigurationError{Err: err}
	}
	if len(selected) > 0 {
		headers, rows, err := columns.Project(records, selected)
		if err != nil {
			return err
		}
		return columns.RenderAutoWidth(helper.GetStreams().Out, headers, rows)
	}

	headers := []string{"TIMESTAMP", "TYPE", "PRINCIPAL", "ACTION", "RESULT", "TRACE ID"}
	rows := make([][]string, 0, len(records))
	for _, record := range records {
		rows = append(rows, defaultAuditLogRow(record))
	}
	return columns.RenderAutoWidth(helper.GetStreams().Out, headers, rows)
}

func defaultAuditLogRow(raw json.RawMessage) []string {
	var record map[string]any
	_ = json.Unmarshal(raw, &record)
	typeName := firstString(record, "type", "event_type")
	principal := firstString(record, "principal_id")
	if principal == "" {
		principal = nestedFirstString(record, []string{"principal", "name"}, []string{"principal", "id"},
			[]string{"actor", "name"}, []string{"actor", "id"}, []string{"user", "id"})
	}
	action := firstString(record, "action", "authentication_type")
	result := firstString(record, "outcome", "result")
	if granted, ok := record["granted"].(bool); ok {
		if granted {
			result = "granted"
		} else {
			result = "denied"
		}
	}
	if typeName == "gateway_access" {
		method := nestedFirstString(record, []string{"request", "method"}, []string{"method"})
		uri := nestedFirstString(record, []string{"request", "uri"}, []string{"uri"})
		action = strings.TrimSpace(method + " " + uri)
		result = firstScalar(record, "status_code")
		if result == "" {
			result = nestedFirstScalar(record, []string{"response", "status_code"})
		}
	}
	return []string{
		firstString(record, "ts", "timestamp", "event_ts"),
		typeName,
		principal,
		action,
		result,
		firstString(record, "trace_id", "traceId"),
	}
}

func firstString(record map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := record[key].(string); ok {
			return value
		}
	}
	return ""
}

func firstScalar(record map[string]any, keys ...string) string {
	for _, key := range keys {
		switch value := record[key].(type) {
		case string:
			return value
		case float64:
			return strconv.FormatFloat(value, 'f', -1, 64)
		case bool:
			return strconv.FormatBool(value)
		}
	}
	return ""
}

func nestedFirstString(record map[string]any, paths ...[]string) string {
	for _, path := range paths {
		if value := nestedValue(record, path); value != nil {
			if text, ok := value.(string); ok {
				return text
			}
		}
	}
	return ""
}

func nestedFirstScalar(record map[string]any, paths ...[]string) string {
	for _, path := range paths {
		value := nestedValue(record, path)
		switch typed := value.(type) {
		case string:
			return typed
		case float64:
			return strconv.FormatFloat(typed, 'f', -1, 64)
		}
	}
	return ""
}

func nestedValue(record map[string]any, path []string) any {
	var value any = record
	for _, key := range path {
		object, ok := value.(map[string]any)
		if !ok {
			return nil
		}
		value = object[key]
	}
	return value
}

func auditLogOutputFormat(command *cobra.Command, cfg config.Hook) (string, error) {
	output := strings.TrimSpace(cfg.GetString(cmdcommon.OutputConfigPath))
	if output == "" {
		output = cmdcommon.DefaultOutputFormat
	}
	if err := cmdcommon.ValidateOutputFormat(command, output); err != nil {
		return "", err
	}
	return output, nil
}

func runAuditLogFollow(
	helper cmd.Helper,
	client *auditLogPullClient,
	options pullAuditLogsOptions,
	output string,
	pageSize int,
	window auditLogWindow,
) error {
	seen := map[string]time.Time{}
	checkpoint := *window.End
	initial := true
	backoff := time.Second
	textHeaderWritten := false

	for {
		if !initial {
			if !waitForAuditLogPoll(helper.GetContext(), options.PollInterval) {
				return nil
			}
			end := auditLogNow().UTC()
			start := checkpoint.Add(-followOverlap)
			window = auditLogWindow{Start: &start, End: &end}
		}

		records := make([]json.RawMessage, 0)
		_, _, err := fetchAllAuditLogPages(helper.GetContext(), client,
			auditLogRequest{Window: window, Type: options.EventType, PageSize: pageSize}, 0,
			func(page []json.RawMessage) error {
				records = append(records, page...)
				return nil
			})
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(helper.GetContext().Err(), context.Canceled) {
				return nil
			}
			if !isRetryableAuditLogError(err) {
				return cmd.PrepareExecutionError("failed to follow audit logs", err, helper.GetCmd())
			}
			_, _ = fmt.Fprintf(helper.GetStreams().ErrOut, "audit-log polling failed; retrying in %s: %v\n", backoff, err)
			if !waitForAuditLogPoll(helper.GetContext(), backoff) {
				return nil
			}
			backoff = min(backoff*2, time.Minute)
			continue
		}

		backoff = time.Second
		newRecords := deduplicateAuditLogRecords(records, seen, *window.End)
		sortAuditLogRecords(newRecords)
		if len(newRecords) > 0 {
			if output == jsonLinesOutput {
				err = renderAuditLogJSONLines(helper, newRecords)
			} else {
				err = renderFollowText(helper, newRecords, &textHeaderWritten)
			}
			if err != nil {
				return cmd.PrepareExecutionError("failed to write audit logs", err, helper.GetCmd())
			}
		}
		checkpoint = *window.End
		expireAuditLogDedup(seen, checkpoint.Add(-followOverlap))
		initial = false
	}
}

func waitForAuditLogPoll(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func isRetryableAuditLogError(err error) bool {
	var responseErr *auditLogResponseError
	if !errors.As(err, &responseErr) {
		return true
	}
	return responseErr.status == http.StatusTooManyRequests || responseErr.status >= http.StatusInternalServerError
}

func deduplicateAuditLogRecords(
	records []json.RawMessage,
	seen map[string]time.Time,
	fallbackTime time.Time,
) []json.RawMessage {
	unique := make([]json.RawMessage, 0, len(records))
	for _, record := range records {
		key := auditLogRecordKey(record)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = auditLogRecordTime(record, fallbackTime)
		unique = append(unique, record)
	}
	return unique
}

func auditLogRecordKey(record json.RawMessage) string {
	var value map[string]any
	if json.Unmarshal(record, &value) == nil {
		if signature := firstString(value, "signature"); signature != "" {
			return "signature:" + signature
		}
	}
	decoder := json.NewDecoder(bytes.NewReader(record))
	decoder.UseNumber()
	var canonical any
	if decoder.Decode(&canonical) == nil {
		if encoded, err := json.Marshal(canonical); err == nil {
			record = encoded
		}
	}
	sum := sha256.Sum256(record)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func auditLogRecordTime(record json.RawMessage, fallback time.Time) time.Time {
	var value map[string]any
	if json.Unmarshal(record, &value) == nil {
		if timestamp := firstString(value, "ts", "timestamp", "event_ts"); timestamp != "" {
			if parsed, err := time.Parse(time.RFC3339, timestamp); err == nil {
				return parsed
			}
		}
	}
	return fallback
}

func sortAuditLogRecords(records []json.RawMessage) {
	sort.SliceStable(records, func(i, j int) bool {
		return auditLogRecordTime(records[i], time.Time{}).Before(auditLogRecordTime(records[j], time.Time{}))
	})
}

func expireAuditLogDedup(seen map[string]time.Time, before time.Time) {
	for key, timestamp := range seen {
		if timestamp.Before(before) {
			delete(seen, key)
		}
	}
}

func renderFollowText(helper cmd.Helper, records []json.RawMessage, headerWritten *bool) error {
	selected, err := columns.Resolve(helper.GetCmd(), cmdcommon.TEXT)
	if err != nil {
		return err
	}
	var headers []string
	var rows [][]string
	if len(selected) > 0 {
		headers, rows, err = columns.Project(records, selected)
		if err != nil {
			return err
		}
	} else {
		headers = []string{"TIMESTAMP", "TYPE", "PRINCIPAL", "ACTION", "RESULT", "TRACE ID"}
		rows = make([][]string, 0, len(records))
		for _, record := range records {
			rows = append(rows, defaultAuditLogRow(record))
		}
	}
	if *headerWritten {
		return renderRowsWithoutHeader(helper.GetStreams().Out, rows)
	}
	*headerWritten = true
	if _, err := fmt.Fprintln(helper.GetStreams().Out, strings.Join(headers, "\t")); err != nil {
		return err
	}
	return renderRowsWithoutHeader(helper.GetStreams().Out, rows)
}

func renderRowsWithoutHeader(out io.Writer, rows [][]string) error {
	for _, row := range rows {
		if _, err := fmt.Fprintln(out, strings.Join(row, "\t")); err != nil {
			return err
		}
	}
	return nil
}
