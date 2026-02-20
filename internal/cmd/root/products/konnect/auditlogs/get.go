package auditlogs

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"

	"github.com/kong/kongctl/internal/cmd"
	cmdcommon "github.com/kong/kongctl/internal/cmd/common"
	jqoutput "github.com/kong/kongctl/internal/cmd/output/jq"
	konnectcommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/konnect/apiutil"
	"github.com/kong/kongctl/internal/konnect/httpclient"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

type auditLogDestinationRecord struct {
	ID                  string `json:"id,omitempty" yaml:"id,omitempty"`
	Name                string `json:"name,omitempty" yaml:"name,omitempty"`
	Endpoint            string `json:"endpoint,omitempty" yaml:"endpoint,omitempty"`
	LogFormat           string `json:"log_format,omitempty" yaml:"log_format,omitempty"`
	SkipSSLVerification *bool  `json:"skip_ssl_verification,omitempty" yaml:"skip_ssl_verification,omitempty"`
	CreatedAt           string `json:"created_at,omitempty" yaml:"created_at,omitempty"`
	UpdatedAt           string `json:"updated_at,omitempty" yaml:"updated_at,omitempty"`
}

type auditLogWebhookConfig struct {
	Enabled             *bool  `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Endpoint            string `json:"endpoint,omitempty" yaml:"endpoint,omitempty"`
	LogFormat           string `json:"log_format,omitempty" yaml:"log_format,omitempty"`
	SkipSSLVerification *bool  `json:"skip_ssl_verification,omitempty" yaml:"skip_ssl_verification,omitempty"`
	DestinationID       string `json:"audit_log_destination_id,omitempty" yaml:"audit_log_destination_id,omitempty"`
	UpdatedAt           string `json:"updated_at,omitempty" yaml:"updated_at,omitempty"`
}

type getAuditLogsDestinationsCmd struct {
	*cobra.Command
}

type getAuditLogDestinationCmd struct {
	*cobra.Command
}

type getAuditLogWebhookCmd struct {
	*cobra.Command
}

func newGetAuditLogsCmd(
	verb verbs.VerbValue,
	baseCmd *cobra.Command,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	baseCmd.Short = "Get Konnect audit-log destinations and webhook state"
	baseCmd.Long = `Use get audit-logs to inspect Konnect audit-log destinations and
regional webhook configuration.`
	baseCmd.Example = `  # List all audit-log destinations
  kongctl get audit-logs destinations

  # Get a single destination by id or name
  kongctl get audit-logs destination <id|name>

  # Get regional webhook configuration
  kongctl get audit-logs webhook`

	if parentPreRun != nil {
		baseCmd.PreRunE = parentPreRun
	}
	if addParentFlags != nil {
		addParentFlags(verb, baseCmd)
	}

	baseCmd.RunE = func(cmdObj *cobra.Command, args []string) error {
		helper := cmd.BuildHelper(cmdObj, args)
		if _, err := helper.GetOutputFormat(); err != nil {
			return err
		}
		return cmdObj.Help()
	}

	baseCmd.AddCommand(newGetAuditLogsDestinationsCmd(verb, addParentFlags, parentPreRun))
	baseCmd.AddCommand(newGetAuditLogDestinationCmd(verb, addParentFlags, parentPreRun))
	baseCmd.AddCommand(newGetAuditLogWebhookCmd(verb, addParentFlags, parentPreRun))

	return baseCmd
}

func newGetAuditLogsDestinationsCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	c := &getAuditLogsDestinationsCmd{}
	cmdObj := &cobra.Command{
		Use:     "destinations",
		Aliases: []string{"dests", "destination-list"},
		Short:   "List Konnect audit-log destinations",
		Long:    "Retrieve all Konnect audit-log destinations from the global API.",
		Example: `  kongctl get audit-logs destinations`,
		RunE:    c.runE,
	}

	c.Command = cmdObj
	if parentPreRun != nil {
		c.PreRunE = parentPreRun
	}
	if addParentFlags != nil {
		addParentFlags(verb, c.Command)
	}

	return c.Command
}

func newGetAuditLogDestinationCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	c := &getAuditLogDestinationCmd{}
	cmdObj := &cobra.Command{
		Use:     "destination <id|name>",
		Aliases: []string{"dest"},
		Short:   "Get one Konnect audit-log destination",
		Long:    "Get a Konnect audit-log destination by exact ID or exact name.",
		Example: `  kongctl get audit-logs destination <id|name>`,
		RunE:    c.runE,
	}

	c.Command = cmdObj
	if parentPreRun != nil {
		c.PreRunE = parentPreRun
	}
	if addParentFlags != nil {
		addParentFlags(verb, c.Command)
	}

	return c.Command
}

func newGetAuditLogWebhookCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	c := &getAuditLogWebhookCmd{}
	cmdObj := &cobra.Command{
		Use:     "webhook",
		Aliases: []string{"hook"},
		Short:   "Get Konnect regional audit-log webhook configuration",
		Long:    "Retrieve regional Konnect audit-log webhook configuration.",
		Example: `  kongctl get audit-logs webhook`,
		RunE:    c.runE,
	}

	c.Command = cmdObj
	if parentPreRun != nil {
		c.PreRunE = parentPreRun
	}
	if addParentFlags != nil {
		addParentFlags(verb, c.Command)
	}

	return c.Command
}

func (c *getAuditLogsDestinationsCmd) runE(cobraCmd *cobra.Command, args []string) error {
	helper := cmd.BuildHelper(cobraCmd, args)
	if len(helper.GetArgs()) > 0 {
		return &cmd.ConfigurationError{Err: fmt.Errorf("the destinations command does not accept arguments")}
	}

	records, err := fetchAuditLogDestinations(helper)
	if err != nil {
		return cmd.PrepareExecutionError("failed to retrieve audit-log destinations", err, helper.GetCmd())
	}

	return renderAuditLogDestinationsOutput(helper, records)
}

func (c *getAuditLogDestinationCmd) runE(cobraCmd *cobra.Command, args []string) error {
	helper := cmd.BuildHelper(cobraCmd, args)
	if len(helper.GetArgs()) != 1 {
		return &cmd.ConfigurationError{Err: fmt.Errorf("the destination command requires exactly one argument: <id|name>")}
	}

	records, err := fetchAuditLogDestinations(helper)
	if err != nil {
		return cmd.PrepareExecutionError("failed to retrieve audit-log destinations", err, helper.GetCmd())
	}

	record, err := findDestinationRecord(records, helper.GetArgs()[0])
	if err != nil {
		return cmd.PrepareExecutionError("failed to get audit-log destination", err, helper.GetCmd())
	}

	return renderAuditLogDestinationOutput(helper, record)
}

func (c *getAuditLogWebhookCmd) runE(cobraCmd *cobra.Command, args []string) error {
	helper := cmd.BuildHelper(cobraCmd, args)
	if len(helper.GetArgs()) > 0 {
		return &cmd.ConfigurationError{Err: fmt.Errorf("the webhook command does not accept arguments")}
	}

	webhookCfg, err := fetchRegionalWebhookConfig(helper)
	if err != nil {
		return cmd.PrepareExecutionError("failed to retrieve regional audit-log webhook configuration", err, helper.GetCmd())
	}

	return renderAuditLogWebhookOutput(helper, webhookCfg)
}

func fetchAuditLogDestinations(helper cmd.Helper) ([]auditLogDestinationRecord, error) {
	cfg, err := helper.GetConfig()
	if err != nil {
		return nil, err
	}

	logger, err := helper.GetLogger()
	if err != nil {
		return nil, err
	}

	token, err := konnectcommon.GetAccessToken(cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("resolve Konnect access token: %w", err)
	}

	ctx := helper.GetContext()
	if ctx == nil {
		ctx = context.Background()
	}

	client := httpclient.NewLoggingHTTPClient(logger)
	result, err := apiutil.Request(
		ctx,
		client,
		http.MethodGet,
		konnectcommon.GlobalBaseURL,
		listDestinationPath,
		token,
		nil,
		nil,
	)
	if err != nil {
		return nil, err
	}

	logger.Debug(
		"audit-log destination list API call completed",
		"path", listDestinationPath,
		"status_code", result.StatusCode,
	)
	if result.StatusCode < http.StatusOK || result.StatusCode >= http.StatusMultipleChoices {
		return nil, buildResponseError(result.StatusCode, result.Body)
	}

	payload := decodeMaybeJSON(result.Body)
	return extractDestinationRecords(payload), nil
}

func fetchRegionalWebhookConfig(helper cmd.Helper) (auditLogWebhookConfig, error) {
	cfg, err := helper.GetConfig()
	if err != nil {
		return auditLogWebhookConfig{}, err
	}

	logger, err := helper.GetLogger()
	if err != nil {
		return auditLogWebhookConfig{}, err
	}

	token, err := konnectcommon.GetAccessToken(cfg, logger)
	if err != nil {
		return auditLogWebhookConfig{}, fmt.Errorf("resolve Konnect access token: %w", err)
	}

	baseURL, err := konnectcommon.ResolveBaseURL(cfg)
	if err != nil {
		return auditLogWebhookConfig{}, fmt.Errorf("resolve Konnect base URL: %w", err)
	}

	ctx := helper.GetContext()
	if ctx == nil {
		ctx = context.Background()
	}

	client := httpclient.NewLoggingHTTPClient(logger)
	result, err := apiutil.Request(
		ctx,
		client,
		http.MethodGet,
		baseURL,
		webhookPathV2,
		token,
		nil,
		nil,
	)
	if err != nil {
		return auditLogWebhookConfig{}, err
	}

	logger.Debug(
		"regional audit-log webhook config API call completed",
		"path", webhookPathV2,
		"base_url", baseURL,
		"status_code", result.StatusCode,
	)
	if result.StatusCode < http.StatusOK || result.StatusCode >= http.StatusMultipleChoices {
		return auditLogWebhookConfig{}, buildResponseError(result.StatusCode, result.Body)
	}

	payload := decodeMaybeJSON(result.Body)
	return extractWebhookConfig(payload), nil
}

func renderAuditLogDestinationsOutput(helper cmd.Helper, records []auditLogDestinationRecord) error {
	outType, err := helper.GetOutputFormat()
	if err != nil {
		return err
	}

	filteredRaw, handled, err := resolveOutputPayload(helper, outType, records)
	if err != nil {
		return err
	}
	if handled {
		return nil
	}

	streams := helper.GetStreams()

	if outType == cmdcommon.TEXT {
		return renderAuditLogDestinationsText(streams.Out, records)
	}

	printer, err := cli.Format(outType.String(), streams.Out)
	if err != nil {
		return err
	}
	defer printer.Flush()

	printer.Print(filteredRaw)
	return nil
}

func renderAuditLogDestinationOutput(helper cmd.Helper, record auditLogDestinationRecord) error {
	outType, err := helper.GetOutputFormat()
	if err != nil {
		return err
	}

	filteredRaw, handled, err := resolveOutputPayload(helper, outType, record)
	if err != nil {
		return err
	}
	if handled {
		return nil
	}

	streams := helper.GetStreams()

	if outType == cmdcommon.TEXT {
		return renderAuditLogDestinationText(streams.Out, record)
	}

	printer, err := cli.Format(outType.String(), streams.Out)
	if err != nil {
		return err
	}
	defer printer.Flush()

	printer.Print(filteredRaw)
	return nil
}

func renderAuditLogWebhookOutput(helper cmd.Helper, config auditLogWebhookConfig) error {
	outType, err := helper.GetOutputFormat()
	if err != nil {
		return err
	}

	filteredRaw, handled, err := resolveOutputPayload(helper, outType, config)
	if err != nil {
		return err
	}
	if handled {
		return nil
	}

	streams := helper.GetStreams()

	if outType == cmdcommon.TEXT {
		return renderAuditLogWebhookText(streams.Out, config)
	}

	printer, err := cli.Format(outType.String(), streams.Out)
	if err != nil {
		return err
	}
	defer printer.Flush()

	printer.Print(filteredRaw)
	return nil
}

func resolveOutputPayload(
	helper cmd.Helper,
	outType cmdcommon.OutputFormat,
	raw any,
) (any, bool, error) {
	cfg, err := helper.GetConfig()
	if err != nil {
		return nil, false, err
	}

	settings, err := jqoutput.ResolveSettings(helper.GetCmd(), cfg)
	if err != nil {
		return nil, false, err
	}
	if err := jqoutput.ValidateOutputFormat(outType, settings); err != nil {
		return nil, false, err
	}
	if !jqoutput.HasFilter(settings) {
		return raw, false, nil
	}

	filteredRaw, handled, err := jqoutput.ApplyToRaw(raw, outType, settings, helper.GetStreams().Out)
	if err != nil {
		return nil, false, cmd.PrepareExecutionErrorWithHelper(helper, "jq filter failed", err)
	}

	return filteredRaw, handled, nil
}

func renderAuditLogDestinationsText(out io.Writer, records []auditLogDestinationRecord) error {
	if len(records) == 0 {
		_, err := fmt.Fprintln(out, "No Konnect audit-log destinations found.")
		return err
	}

	if _, err := fmt.Fprintf(out, "Konnect audit-log destinations (%d)\n\n", len(records)); err != nil {
		return err
	}

	for idx, record := range records {
		if _, err := fmt.Fprintf(out, "Destination %d\n", idx+1); err != nil {
			return err
		}
		if err := writeDestinationFields(out, record); err != nil {
			return err
		}
		if idx < len(records)-1 {
			if _, err := fmt.Fprintln(out); err != nil {
				return err
			}
		}
	}

	return nil
}

func renderAuditLogDestinationText(out io.Writer, record auditLogDestinationRecord) error {
	if _, err := fmt.Fprintln(out, "Konnect audit-log destination"); err != nil {
		return err
	}
	return writeDestinationFields(out, record)
}

func writeDestinationFields(out io.Writer, record auditLogDestinationRecord) error {
	if _, err := fmt.Fprintf(out, "  id: %s\n", displayOrNA(record.ID)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "  name: %s\n", displayOrNA(record.Name)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "  endpoint: %s\n", displayOrNA(record.Endpoint)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "  log format: %s\n", displayOrNA(record.LogFormat)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(
		out,
		"  skip ssl verification: %s\n",
		formatOptionalBool(record.SkipSSLVerification),
	); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "  created at: %s\n", displayOrNA(record.CreatedAt)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "  updated at: %s\n", displayOrNA(record.UpdatedAt)); err != nil {
		return err
	}

	return nil
}

func renderAuditLogWebhookText(out io.Writer, config auditLogWebhookConfig) error {
	if _, err := fmt.Fprintln(out, "Konnect regional audit-log webhook"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "  enabled: %s\n", formatOptionalBool(config.Enabled)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "  endpoint: %s\n", displayOrNA(config.Endpoint)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "  log format: %s\n", displayOrNA(config.LogFormat)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(
		out,
		"  skip ssl verification: %s\n",
		formatOptionalBool(config.SkipSSLVerification),
	); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "  destination id: %s\n", displayOrNA(config.DestinationID)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "  updated at: %s\n", displayOrNA(config.UpdatedAt)); err != nil {
		return err
	}

	return nil
}

func extractDestinationRecords(payload any) []auditLogDestinationRecord {
	records := make([]auditLogDestinationRecord, 0)
	seen := make(map[string]struct{})

	var walk func(any)
	walk = func(node any) {
		switch typed := node.(type) {
		case map[string]any:
			record, ok := destinationRecordFromMap(typed)
			if ok {
				key := destinationRecordKey(record)
				if _, exists := seen[key]; !exists {
					seen[key] = struct{}{}
					records = append(records, record)
				}
				return
			}
			for _, child := range typed {
				walk(child)
			}
		case []any:
			for _, child := range typed {
				walk(child)
			}
		}
	}

	walk(payload)

	sort.Slice(records, func(i, j int) bool {
		leftName := strings.ToLower(records[i].Name)
		rightName := strings.ToLower(records[j].Name)
		if leftName == rightName {
			return strings.ToLower(records[i].ID) < strings.ToLower(records[j].ID)
		}
		return leftName < rightName
	})

	return records
}

func destinationRecordFromMap(value map[string]any) (auditLogDestinationRecord, bool) {
	record := auditLogDestinationRecord{
		ID:                  mapStringField(value, "id"),
		Name:                mapStringField(value, "name"),
		Endpoint:            mapStringField(value, "endpoint"),
		LogFormat:           mapStringField(value, "log_format"),
		SkipSSLVerification: mapBoolField(value, "skip_ssl_verification"),
		CreatedAt:           mapStringField(value, "created_at"),
		UpdatedAt:           mapStringField(value, "updated_at"),
	}

	if record.Endpoint == "" {
		return auditLogDestinationRecord{}, false
	}
	if record.ID == "" && record.Name == "" {
		return auditLogDestinationRecord{}, false
	}

	return record, true
}

func destinationRecordKey(record auditLogDestinationRecord) string {
	if strings.TrimSpace(record.ID) != "" {
		return "id:" + strings.TrimSpace(record.ID)
	}
	return "name:endpoint:" + strings.TrimSpace(record.Name) + "|" + strings.TrimSpace(record.Endpoint)
}

func mapStringField(value map[string]any, key string) string {
	raw, ok := value[key]
	if !ok {
		return ""
	}

	str, ok := raw.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(str)
}

func mapBoolField(value map[string]any, key string) *bool {
	raw, ok := value[key]
	if !ok {
		return nil
	}

	boolVal, ok := raw.(bool)
	if !ok {
		return nil
	}

	return boolPtr(boolVal)
}

func findDestinationRecord(records []auditLogDestinationRecord, selector string) (auditLogDestinationRecord, error) {
	target := strings.TrimSpace(selector)
	if target == "" {
		return auditLogDestinationRecord{}, fmt.Errorf("destination selector cannot be empty")
	}

	matches := make([]auditLogDestinationRecord, 0, 1)
	for _, record := range records {
		if strings.TrimSpace(record.ID) == target || strings.TrimSpace(record.Name) == target {
			matches = append(matches, record)
		}
	}

	switch len(matches) {
	case 0:
		return auditLogDestinationRecord{}, fmt.Errorf("audit-log destination %q not found", target)
	case 1:
		return matches[0], nil
	default:
		return auditLogDestinationRecord{}, fmt.Errorf(
			"multiple audit-log destinations matched %q; use destination id instead",
			target,
		)
	}
}

func extractWebhookConfig(payload any) auditLogWebhookConfig {
	return auditLogWebhookConfig{
		Enabled:             findOptionalBool(payload, "enabled"),
		Endpoint:            findFirstString(payload, "endpoint"),
		LogFormat:           findFirstString(payload, "log_format"),
		SkipSSLVerification: findOptionalBool(payload, "skip_ssl_verification"),
		DestinationID: findFirstString(
			payload,
			"audit_log_destination_id",
			"destination_id",
		),
		UpdatedAt: findFirstString(payload, "updated_at"),
	}
}

func findOptionalBool(value any, keys ...string) *bool {
	if len(keys) == 0 {
		return nil
	}

	keySet := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		normalized := strings.ToLower(strings.TrimSpace(key))
		if normalized != "" {
			keySet[normalized] = struct{}{}
		}
	}

	var walk func(any) (*bool, bool)
	walk = func(node any) (*bool, bool) {
		switch typed := node.(type) {
		case map[string]any:
			for key, child := range typed {
				normalized := strings.ToLower(strings.TrimSpace(key))
				if _, ok := keySet[normalized]; ok {
					if boolVal, ok := child.(bool); ok {
						return boolPtr(boolVal), true
					}
				}
			}
			for _, child := range typed {
				if boolVal, found := walk(child); found {
					return boolVal, true
				}
			}
		case []any:
			for _, child := range typed {
				if boolVal, found := walk(child); found {
					return boolVal, true
				}
			}
		}

		return nil, false
	}

	boolVal, found := walk(value)
	if !found {
		return nil
	}
	return boolVal
}

func boolPtr(value bool) *bool {
	return &value
}

func formatOptionalBool(value *bool) string {
	if value == nil {
		return "n/a"
	}
	if *value {
		return "true"
	}
	return "false"
}

func displayOrNA(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "n/a"
	}
	return trimmed
}
