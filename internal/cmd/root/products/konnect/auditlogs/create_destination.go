package auditlogs

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
	"unicode"

	"github.com/kong/kongctl/internal/auditlogs"
	"github.com/kong/kongctl/internal/cmd"
	cmdcommon "github.com/kong/kongctl/internal/cmd/common"
	konnectcommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/konnect/apiutil"
	"github.com/kong/kongctl/internal/konnect/httpclient"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

const (
	createDestinationPath = "/v3/audit-log-destinations"
	listDestinationPath   = "/v3/audit-log-destinations"
	updateWebhookPath     = "/v3/audit-log-webhook"
	deleteWebhookPathV3   = "/v3/audit-log-webhook"
	webhookPathV2         = "/v2/audit-log-webhook"
	deleteDestinationV2   = "/v2/audit-log-destinations"

	defaultLogFormatValue = "json"

	cleanupTimeout            = 60 * time.Second
	deleteRetryInitialBackoff = 250 * time.Millisecond
	deleteRetryMaxBackoff     = 5 * time.Second
)

var allowedLogFormats = []string{
	"cef",
	"json",
	"cps",
}

var allowedLogFormatsSet = map[string]struct{}{
	"cef":  {},
	"json": {},
	"cps":  {},
}

type createDestinationRequest struct {
	Name                string `json:"name,omitempty" yaml:"name,omitempty"`
	Endpoint            string `json:"endpoint" yaml:"endpoint"`
	LogFormat           string `json:"log_format,omitempty" yaml:"log_format,omitempty"`
	SkipSSLVerification bool   `json:"skip_ssl_verification" yaml:"skip_ssl_verification"`
	Authorization       string `json:"authorization,omitempty" yaml:"authorization,omitempty"`
}

type createDestinationOutput struct {
	CreatedAt               time.Time `json:"created_at" yaml:"created_at"`
	Profile                 string    `json:"profile" yaml:"profile"`
	DestinationID           string    `json:"destination_id,omitempty" yaml:"destination_id,omitempty"`
	DestinationName         string    `json:"destination_name" yaml:"destination_name"`
	DestinationEndpoint     string    `json:"destination_endpoint" yaml:"destination_endpoint"`
	LogFormat               string    `json:"log_format,omitempty" yaml:"log_format,omitempty"`
	SkipSSLVerification     bool      `json:"skip_ssl_verification" yaml:"skip_ssl_verification"`
	AuthorizationConfigured bool      `json:"authorization_configured" yaml:"authorization_configured"`
	WebhookConfigured       bool      `json:"webhook_configured" yaml:"webhook_configured"`
	EventsFile              string    `json:"events_file" yaml:"events_file"`
	DestinationStateFile    string    `json:"destination_state_file" yaml:"destination_state_file"`
	RawDestination          any       `json:"destination_response,omitempty" yaml:"destination_response,omitempty"`
	RawWebhook              any       `json:"webhook_response,omitempty" yaml:"webhook_response,omitempty"`
}

type createDestinationCmd struct {
	*cobra.Command

	endpoint            string
	name                string
	logFormat           string
	skipSSLVerification bool
	authorization       string
	configureWebhook    bool
}

func newCreateDestinationCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	c := &createDestinationCmd{
		logFormat:        defaultLogFormatValue,
		configureWebhook: true,
	}
	cmdObj := &cobra.Command{
		Use:   "destination",
		Short: "Create a Konnect audit-log destination",
		Long: `Create a Konnect audit-log destination that points to an externally reachable
HTTP endpoint (for example, a tunnel URL that fronts your local listener).`,
		Example: `  # Create a destination for your exposed listener
  kongctl create audit-logs destination --endpoint https://example.ngrok.app/audit-logs

  # Include delivery options
  kongctl create audit-logs destination \
    --endpoint https://example.ngrok.app/audit-logs \
    --log-format cef \
    --skip-ssl-verification \
    --authorization "Bearer my-secret-token"`,
		RunE: c.runE,
	}

	c.Command = cmdObj
	c.Flags().StringVar(&c.endpoint, "endpoint", "", "Destination webhook URL.")
	c.Flags().StringVar(&c.name, "name", "", "Destination name. Default: kongctl-<hostname>-<pid>.")
	c.Flags().StringVar(&c.logFormat, "log-format", defaultLogFormatValue,
		fmt.Sprintf("Audit-log payload format. Allowed: %s.", strings.Join(allowedLogFormats, "|")))
	c.Flags().BoolVar(&c.skipSSLVerification, "skip-ssl-verification", false,
		"Skip TLS certificate verification for destination delivery.")
	c.Flags().StringVar(&c.authorization, "authorization", "",
		"Value for the Authorization header Konnect includes when sending audit logs.")
	c.Flags().BoolVar(&c.configureWebhook, "configure-webhook", true,
		"Automatically bind and enable the organization webhook with the created destination.")

	_ = c.MarkFlagRequired("endpoint")

	if parentPreRun != nil {
		c.PreRunE = parentPreRun
	}
	if addParentFlags != nil {
		addParentFlags(verb, c.Command)
	}

	return c.Command
}

func (c *createDestinationCmd) runE(cmdObj *cobra.Command, args []string) error {
	helper := cmd.BuildHelper(cmdObj, args)
	if len(helper.GetArgs()) > 0 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("the destination command does not accept arguments"),
		}
	}

	output, err := c.execute(helper)
	if err != nil {
		return err
	}

	return renderCreateDestinationOutput(helper, output)
}

func (c *createDestinationCmd) execute(helper cmd.Helper) (createDestinationOutput, error) {
	endpoint := strings.TrimSpace(c.endpoint)
	if endpoint == "" {
		return createDestinationOutput{}, &cmd.ConfigurationError{Err: fmt.Errorf("endpoint is required")}
	}
	if _, err := url.ParseRequestURI(endpoint); err != nil {
		return createDestinationOutput{}, &cmd.ConfigurationError{
			Err: fmt.Errorf("endpoint must be a valid URL: %w", err),
		}
	}

	logFormat, err := normalizeLogFormat(c.logFormat)
	if err != nil {
		return createDestinationOutput{}, &cmd.ConfigurationError{Err: err}
	}

	cfg, err := helper.GetConfig()
	if err != nil {
		return createDestinationOutput{}, err
	}
	logger, err := helper.GetLogger()
	if err != nil {
		return createDestinationOutput{}, err
	}

	safeEndpoint := sanitizeEndpointForLog(endpoint)
	logger.Debug(
		"audit-log destination setup started",
		"endpoint", safeEndpoint,
		"name_provided", strings.TrimSpace(c.name) != "",
		"log_format", strings.TrimSpace(c.logFormat),
		"skip_ssl_verification", c.skipSSLVerification,
		"authorization_configured", strings.TrimSpace(c.authorization) != "",
		"configure_webhook", c.configureWebhook,
	)

	token, err := konnectcommon.GetAccessToken(cfg, logger)
	if err != nil {
		return createDestinationOutput{}, cmd.PrepareExecutionError("failed to resolve Konnect access token", err, helper.GetCmd())
	}

	ctx := helper.GetContext()
	if ctx == nil {
		ctx = context.Background()
	}
	client := httpclient.NewLoggingHTTPClient(logger)
	regionalBaseURL := ""
	if c.configureWebhook {
		regionalBaseURL, err = konnectcommon.ResolveBaseURL(cfg)
		if err != nil {
			return createDestinationOutput{}, cmd.PrepareExecutionError(
				"failed to resolve Konnect base URL",
				err,
				helper.GetCmd(),
			)
		}
		logger.Debug(
			"resolved regional Konnect base URL for webhook operations",
			"base_url", regionalBaseURL,
		)
		if err := ensureNoActiveRegionalWebhook(ctx, client, regionalBaseURL, token, logger); err != nil {
			return createDestinationOutput{}, cmd.PrepareExecutionError(
				"active audit-log webhook already configured for this region",
				err,
				helper.GetCmd(),
			)
		}
	}

	requestBody := createDestinationRequest{
		Name:                strings.TrimSpace(c.name),
		Endpoint:            endpoint,
		LogFormat:           logFormat,
		SkipSSLVerification: c.skipSSLVerification,
		Authorization:       strings.TrimSpace(c.authorization),
	}
	if requestBody.Name == "" {
		requestBody.Name = defaultDestinationName()
		logger.Debug("generated default audit-log destination name", "name", requestBody.Name)
	}

	nameTaken, err := destinationNameExists(ctx, client, token, requestBody.Name, logger)
	if err != nil {
		return createDestinationOutput{}, cmd.PrepareExecutionError(
			"failed to verify destination name uniqueness",
			err,
			helper.GetCmd(),
		)
	}
	if nameTaken {
		logger.Debug("audit-log destination name is already in use", "name", requestBody.Name)
		return createDestinationOutput{}, cmd.PrepareExecutionError(
			"failed to create audit-log destination",
			fmt.Errorf("destination name %q already exists", requestBody.Name),
			helper.GetCmd(),
		)
	}

	encoded, err := json.Marshal(requestBody)
	if err != nil {
		return createDestinationOutput{}, cmd.PrepareExecutionError("failed to encode destination request", err, helper.GetCmd())
	}

	destinationResult, err := apiutil.Request(
		ctx,
		client,
		http.MethodPost,
		konnectcommon.GlobalBaseURL,
		createDestinationPath,
		token,
		map[string]string{
			"Content-Type": "application/json",
		},
		bytes.NewReader(encoded),
	)
	if err != nil {
		return createDestinationOutput{}, cmd.PrepareExecutionError(
			"failed to create audit-log destination",
			err,
			helper.GetCmd(),
		)
	}
	logger.Debug(
		"audit-log destination create API call completed",
		"path", createDestinationPath,
		"status_code", destinationResult.StatusCode,
		"endpoint", safeEndpoint,
	)
	if destinationResult.StatusCode < http.StatusOK || destinationResult.StatusCode >= http.StatusMultipleChoices {
		return createDestinationOutput{}, cmd.PrepareExecutionError(
			"failed to create audit-log destination",
			buildResponseError(destinationResult.StatusCode, destinationResult.Body),
			helper.GetCmd(),
		)
	}

	destinationPayload := decodeMaybeJSON(destinationResult.Body)
	destinationID := findFirstString(destinationPayload,
		"id",
		"audit_log_destination_id",
		"destination_id",
	)
	logger.Debug(
		"parsed audit-log destination create response",
		"destination_id_present", strings.TrimSpace(destinationID) != "",
	)

	output := createDestinationOutput{
		CreatedAt:               time.Now().UTC(),
		Profile:                 cfg.GetProfile(),
		DestinationID:           destinationID,
		DestinationName:         requestBody.Name,
		DestinationEndpoint:     requestBody.Endpoint,
		LogFormat:               requestBody.LogFormat,
		SkipSSLVerification:     requestBody.SkipSSLVerification,
		AuthorizationConfigured: requestBody.Authorization != "",
		RawDestination:          destinationPayload,
	}

	if c.configureWebhook {
		if destinationID == "" {
			return createDestinationOutput{}, cmd.PrepareExecutionError(
				"failed to configure audit-log webhook",
				fmt.Errorf("destination created but no destination ID was returned"),
				helper.GetCmd(),
			)
		}

		webhookBody := map[string]any{
			"enabled":                  true,
			"audit_log_destination_id": destinationID,
		}
		encodedWebhook, err := json.Marshal(webhookBody)
		if err != nil {
			return createDestinationOutput{}, cmd.PrepareExecutionError(
				"failed to encode audit-log webhook request",
				err,
				helper.GetCmd(),
			)
		}

		webhookResult, err := apiutil.Request(
			ctx,
			client,
			http.MethodPatch,
			regionalBaseURL,
			updateWebhookPath,
			token,
			map[string]string{
				"Content-Type": "application/json",
			},
			bytes.NewReader(encodedWebhook),
		)
		if err != nil {
			return createDestinationOutput{}, cmd.PrepareExecutionError(
				"failed to configure audit-log webhook",
				err,
				helper.GetCmd(),
			)
		}
		logger.Debug(
			"audit-log webhook configure API call completed",
			"path", updateWebhookPath,
			"base_url", regionalBaseURL,
			"status_code", webhookResult.StatusCode,
			"destination_id", destinationID,
		)
		if webhookResult.StatusCode < http.StatusOK || webhookResult.StatusCode >= http.StatusMultipleChoices {
			return createDestinationOutput{}, cmd.PrepareExecutionError(
				"failed to configure audit-log webhook",
				buildResponseError(webhookResult.StatusCode, webhookResult.Body),
				helper.GetCmd(),
			)
		}
		output.WebhookConfigured = true
		output.RawWebhook = decodeMaybeJSON(webhookResult.Body)
	}

	paths, err := auditlogs.ResolvePaths(cfg.GetProfile())
	if err == nil {
		output.EventsFile = paths.EventsFile
		output.DestinationStateFile = paths.DestinationStateFile
		_ = auditlogs.WriteState(paths.DestinationStateFile, output)
		logger.Debug(
			"persisted audit-log destination state",
			"destination_state_file", paths.DestinationStateFile,
			"events_file", paths.EventsFile,
		)
	}

	logger.Debug(
		"audit-log destination setup completed",
		"destination_id", output.DestinationID,
		"name", output.DestinationName,
		"endpoint", safeEndpoint,
		"webhook_configured", output.WebhookConfigured,
		"webhook_base_url", regionalBaseURL,
	)

	return output, nil
}

func renderCreateDestinationOutput(helper cmd.Helper, output createDestinationOutput) error {
	outType, err := helper.GetOutputFormat()
	if err != nil {
		return err
	}
	streams := helper.GetStreams()

	if outType == cmdcommon.TEXT {
		if _, err := fmt.Fprintln(streams.Out, "Audit-log destination created"); err != nil {
			return err
		}
		if output.DestinationID != "" {
			if _, err := fmt.Fprintf(streams.Out, "  destination id: %s\n", output.DestinationID); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(streams.Out, "  name: %s\n", output.DestinationName); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(streams.Out, "  endpoint: %s\n", output.DestinationEndpoint); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(streams.Out, "  log format: %s\n", output.LogFormat); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(streams.Out, "  skip ssl verification: %t\n", output.SkipSSLVerification); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(streams.Out, "  authorization configured: %t\n", output.AuthorizationConfigured); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(streams.Out, "  webhook configured: %t\n", output.WebhookConfigured); err != nil {
			return err
		}
		if output.DestinationStateFile != "" {
			if _, err := fmt.Fprintf(streams.Out, "  state file: %s\n", output.DestinationStateFile); err != nil {
				return err
			}
		}
		return nil
	}

	printer, err := cli.Format(outType.String(), streams.Out)
	if err != nil {
		return err
	}
	defer printer.Flush()
	printer.Print(output)
	return nil
}

func deleteDestinationForHelper(helper cmd.Helper, destinationID string, disableWebhook bool) error {
	destinationID = strings.TrimSpace(destinationID)
	if destinationID == "" {
		return fmt.Errorf("destination ID is required")
	}

	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}
	logger, err := helper.GetLogger()
	if err != nil {
		return err
	}
	token, err := konnectcommon.GetAccessToken(cfg, logger)
	if err != nil {
		return fmt.Errorf("resolve Konnect access token: %w", err)
	}
	logger.Debug("deleting audit-log destination", "destination_id", destinationID)

	// Listener shutdown cancels the command context; use a detached context
	// so destination cleanup still runs on Ctrl+C.
	ctx, cancel := context.WithTimeout(context.Background(), cleanupTimeout)
	defer cancel()

	baseURL, err := konnectcommon.ResolveBaseURL(cfg)
	if err != nil {
		return fmt.Errorf("resolve Konnect base URL: %w", err)
	}
	client := httpclient.NewLoggingHTTPClient(logger)

	if disableWebhook {
		if err := releaseRegionalWebhook(ctx, client, baseURL, token, logger); err != nil {
			logger.Debug(
				"regional audit-log webhook release had non-fatal errors",
				"error", err,
			)
		}
	}

	path := fmt.Sprintf("%s/%s", deleteDestinationV2, url.PathEscape(destinationID))
	return deleteDestinationWithRetry(ctx, client, baseURL, path, token, destinationID, logger)
}

func releaseRegionalWebhook(
	ctx context.Context,
	client apiutil.Doer,
	baseURL,
	token string,
	logger *slog.Logger,
) error {
	var webhookErr error

	if logger != nil {
		logger.Debug(
			"disabling regional audit-log webhook before destination delete",
			"path", webhookPathV2,
			"base_url", baseURL,
		)
	}

	disableBody := map[string]any{
		"enabled": false,
	}
	encodedDisableBody, err := json.Marshal(disableBody)
	if err != nil {
		return fmt.Errorf("encode disable webhook request: %w", err)
	}

	disableResult, err := apiutil.Request(
		ctx,
		client,
		http.MethodPatch,
		baseURL,
		webhookPathV2,
		token,
		map[string]string{
			"Content-Type": "application/json",
		},
		bytes.NewReader(encodedDisableBody),
	)
	if err != nil {
		webhookErr = errors.Join(webhookErr, fmt.Errorf("disable audit-log webhook: %w", err))
	} else {
		if logger != nil {
			logger.Debug(
				"audit-log webhook disable API call completed",
				"path", webhookPathV2,
				"base_url", baseURL,
				"status_code", disableResult.StatusCode,
			)
		}
		if disableResult.StatusCode != http.StatusNotFound &&
			(disableResult.StatusCode < http.StatusOK || disableResult.StatusCode >= http.StatusMultipleChoices) {
			webhookErr = errors.Join(
				webhookErr,
				fmt.Errorf(
					"disable audit-log webhook failed: %w",
					buildResponseError(disableResult.StatusCode, disableResult.Body),
				),
			)
		}
	}

	if logger != nil {
		logger.Debug(
			"deleting regional audit-log webhook before destination delete",
			"path", deleteWebhookPathV3,
			"base_url", baseURL,
		)
	}
	deleteResult, err := apiutil.Request(
		ctx,
		client,
		http.MethodDelete,
		baseURL,
		deleteWebhookPathV3,
		token,
		nil,
		nil,
	)
	if err != nil {
		webhookErr = errors.Join(webhookErr, fmt.Errorf("delete audit-log webhook: %w", err))
	} else {
		if logger != nil {
			logger.Debug(
				"audit-log webhook delete API call completed",
				"path", deleteWebhookPathV3,
				"base_url", baseURL,
				"status_code", deleteResult.StatusCode,
			)
		}
		if deleteResult.StatusCode != http.StatusNotFound &&
			(deleteResult.StatusCode < http.StatusOK || deleteResult.StatusCode >= http.StatusMultipleChoices) {
			webhookErr = errors.Join(
				webhookErr,
				fmt.Errorf(
					"delete audit-log webhook failed: %w",
					buildResponseError(deleteResult.StatusCode, deleteResult.Body),
				),
			)
		}
	}

	return webhookErr
}

func destinationNameExists(ctx context.Context, client apiutil.Doer, token, name string, logger *slog.Logger) (bool, error) {
	if strings.TrimSpace(name) == "" {
		return false, nil
	}
	if logger != nil {
		logger.Debug("checking audit-log destination name uniqueness", "name", name)
	}

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
		return false, err
	}
	if logger != nil {
		logger.Debug(
			"audit-log destination list API call completed",
			"path", listDestinationPath,
			"status_code", result.StatusCode,
		)
	}
	if result.StatusCode < http.StatusOK || result.StatusCode >= http.StatusMultipleChoices {
		return false, buildResponseError(result.StatusCode, result.Body)
	}

	payload := decodeMaybeJSON(result.Body)
	exists := payloadContainsDestinationName(payload, name)
	if logger != nil {
		logger.Debug("audit-log destination name uniqueness check completed", "name", name, "exists", exists)
	}
	return exists, nil
}

func ensureNoActiveRegionalWebhook(
	ctx context.Context,
	client apiutil.Doer,
	baseURL,
	token string,
	logger *slog.Logger,
) error {
	configResult, err := apiutil.Request(
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
		return err
	}
	if logger != nil {
		logger.Debug(
			"regional audit-log webhook config API call completed",
			"path", webhookPathV2,
			"base_url", baseURL,
			"status_code", configResult.StatusCode,
		)
	}
	if configResult.StatusCode < http.StatusOK || configResult.StatusCode >= http.StatusMultipleChoices {
		return buildResponseError(configResult.StatusCode, configResult.Body)
	}

	configPayload := decodeMaybeJSON(configResult.Body)
	enabled := findFirstBool(configPayload, "enabled")
	endpoint := strings.TrimSpace(findFirstString(configPayload, "endpoint"))
	if isRegionalWebhookUnconfigured(enabled, endpoint) {
		if logger != nil {
			logger.Debug(
				"regional webhook is unconfigured; startup guard passed",
				"enabled", enabled,
				"endpoint", endpoint,
			)
		}
		return nil
	}

	safeEndpoint := sanitizeEndpointForLog(endpoint)
	return fmt.Errorf(
		"regional audit-log webhook is already configured (enabled=%t endpoint=%q); expected enabled=false and endpoint=\"unconfigured\"",
		enabled,
		safeEndpoint,
	)
}

func isRegionalWebhookUnconfigured(enabled bool, endpoint string) bool {
	if enabled {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(endpoint), "unconfigured")
}

func deleteDestinationWithRetry(
	ctx context.Context,
	client apiutil.Doer,
	baseURL,
	path,
	token,
	destinationID string,
	logger *slog.Logger,
) error {
	delay := deleteRetryInitialBackoff
	if delay <= 0 {
		delay = 250 * time.Millisecond
	}

	attempt := 1
	for {
		result, err := apiutil.Request(
			ctx,
			client,
			http.MethodDelete,
			baseURL,
			path,
			token,
			nil,
			nil,
		)
		if err != nil {
			return err
		}
		if logger != nil {
			logger.Debug(
				"audit-log destination delete API call completed",
				"path", path,
				"base_url", baseURL,
				"status_code", result.StatusCode,
				"destination_id", destinationID,
				"attempt", attempt,
			)
		}

		switch result.StatusCode {
		case http.StatusOK, http.StatusNoContent, http.StatusAccepted, http.StatusNotFound:
			return nil
		case http.StatusConflict:
			if !isDestinationInUseConflict(result.Body) {
				return buildResponseError(result.StatusCode, result.Body)
			}

			if logger != nil {
				logger.Debug(
					"destination delete conflict detected; retrying with backoff",
					"destination_id", destinationID,
					"attempt", attempt,
					"sleep", delay.String(),
				)
			}
			if err := sleepWithContext(ctx, delay); err != nil {
				return fmt.Errorf("wait for destination release: %w", err)
			}
			attempt++
			if delay < deleteRetryMaxBackoff {
				delay = minDuration(delay*2, deleteRetryMaxBackoff)
			}
		default:
			return buildResponseError(result.StatusCode, result.Body)
		}
	}
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

func isDestinationInUseConflict(body []byte) bool {
	lower := strings.ToLower(strings.TrimSpace(string(body)))
	if lower == "" {
		return false
	}
	return strings.Contains(lower, "destination is in use")
}

func sleepWithContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func payloadContainsDestinationName(value any, expectedName string) bool {
	target := strings.TrimSpace(expectedName)
	if target == "" {
		return false
	}

	var walk func(any) bool
	walk = func(node any) bool {
		switch typed := node.(type) {
		case map[string]any:
			if rawName, ok := typed["name"]; ok {
				if name, ok := rawName.(string); ok {
					trimmedName := strings.TrimSpace(name)
					if trimmedName == target {
						if _, hasEndpoint := typed["endpoint"]; hasEndpoint {
							return true
						}
						if _, hasID := typed["id"]; hasID {
							return true
						}
					}
				}
			}
			for _, child := range typed {
				if walk(child) {
					return true
				}
			}
		case []any:
			for _, child := range typed {
				if walk(child) {
					return true
				}
			}
		}
		return false
	}

	return walk(value)
}

func normalizeLogFormat(raw string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	if normalized == "" {
		return defaultLogFormatValue, nil
	}

	if _, ok := allowedLogFormatsSet[normalized]; !ok {
		return "", fmt.Errorf("invalid log-format %q, allowed values are: %s", raw, strings.Join(allowedLogFormats, ", "))
	}

	return normalized, nil
}

func defaultDestinationName() string {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown-host"
	}

	safeHost := sanitizeDestinationNameComponent(hostname)
	return fmt.Sprintf("kongctl-%s-%d", safeHost, os.Getpid())
}

func sanitizeDestinationNameComponent(input string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "unknown-host"
	}

	var b strings.Builder
	for _, r := range trimmed {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r), r == '-', r == '_', r == '.':
			b.WriteRune(unicode.ToLower(r))
		default:
			b.WriteByte('-')
		}
	}

	safe := strings.Trim(b.String(), "-")
	if safe == "" {
		return "unknown-host"
	}
	return safe
}

func decodeMaybeJSON(raw []byte) any {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return nil
	}
	var payload any
	if err := json.Unmarshal([]byte(trimmed), &payload); err == nil {
		return payload
	}
	return trimmed
}

func sanitizeEndpointForLog(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return trimmed
	}
	parsed.User = nil
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

func buildResponseError(statusCode int, body []byte) error {
	statusText := http.StatusText(statusCode)
	if statusText == "" {
		statusText = "unknown status"
	}

	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return fmt.Errorf("status %d %s", statusCode, statusText)
	}
	return fmt.Errorf("status %d %s: %s", statusCode, statusText, trimmed)
}

func findFirstString(value any, keys ...string) string {
	if len(keys) == 0 {
		return ""
	}

	keySet := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		normalized := strings.ToLower(strings.TrimSpace(key))
		if normalized != "" {
			keySet[normalized] = struct{}{}
		}
	}

	var walk func(any) string
	walk = func(node any) string {
		switch typed := node.(type) {
		case map[string]any:
			for key, child := range typed {
				normalized := strings.ToLower(strings.TrimSpace(key))
				if _, ok := keySet[normalized]; ok {
					if s, ok := child.(string); ok && strings.TrimSpace(s) != "" {
						return strings.TrimSpace(s)
					}
				}
			}
			for _, child := range typed {
				if found := walk(child); found != "" {
					return found
				}
			}
		case []any:
			for _, child := range typed {
				if found := walk(child); found != "" {
					return found
				}
			}
		}
		return ""
	}

	return walk(value)
}

func findFirstBool(value any, keys ...string) bool {
	if len(keys) == 0 {
		return false
	}

	keySet := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		normalized := strings.ToLower(strings.TrimSpace(key))
		if normalized != "" {
			keySet[normalized] = struct{}{}
		}
	}

	var walk func(any) (bool, bool)
	walk = func(node any) (bool, bool) {
		switch typed := node.(type) {
		case map[string]any:
			for key, child := range typed {
				normalized := strings.ToLower(strings.TrimSpace(key))
				if _, ok := keySet[normalized]; ok {
					if b, ok := child.(bool); ok {
						return b, true
					}
				}
			}
			for _, child := range typed {
				if b, found := walk(child); found {
					return b, true
				}
			}
		case []any:
			for _, child := range typed {
				if b, found := walk(child); found {
					return b, true
				}
			}
		}

		return false, false
	}

	result, found := walk(value)
	if !found {
		return false
	}
	return result
}
