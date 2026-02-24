package auditlogs

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/kong/kongctl/internal/cmd"
	cmdcommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/processes"
	"github.com/spf13/cobra"
)

const (
	detachedListenerPIDToken        = "%PID%"
	detachedListenerLogFileTemplate = "kongctl-listener-" + detachedListenerPIDToken + ".log"
	detachedListenerProcessKind     = "konnect.audit-logs.listen"
)

// ListenAuditLogsOptions controls listen/audit-log command behavior.
type ListenAuditLogsOptions struct {
	Endpoint            string
	PublicURL           string
	ListenAddress       string
	ListenPath          string
	MaxBodyBytes        int
	Name                string
	LogFormat           string
	SkipSSLVerification bool
	Authorization       string
	ConfigureWebhook    bool
	Tail                bool
	Detach              bool
	JQ                  string
}

// DefaultListenAuditLogsOptions returns defaults shared by listen command variants.
func DefaultListenAuditLogsOptions() ListenAuditLogsOptions {
	return ListenAuditLogsOptions{
		LogFormat:           defaultLogFormatValue,
		ListenAddress:       defaultListenAddress,
		ListenPath:          defaultListenPath,
		MaxBodyBytes:        defaultMaxBodyBytes,
		ConfigureWebhook:    true,
		SkipSSLVerification: false,
	}
}

// AddListenAuditLogsFlags registers listen/audit-log flags onto a command.
func AddListenAuditLogsFlags(cmdObj *cobra.Command, options *ListenAuditLogsOptions) {
	if cmdObj == nil || options == nil {
		return
	}

	cmdObj.Flags().StringVar(&options.Endpoint, "endpoint", options.Endpoint,
		"Explicit destination endpoint URL used for Konnect destination creation.")
	cmdObj.Flags().StringVar(&options.PublicURL, "public-url", options.PublicURL,
		"Externally reachable base URL for this listener; used to build destination endpoint when --endpoint is omitted.")
	cmdObj.Flags().StringVar(&options.ListenAddress, "listen-address", options.ListenAddress,
		"HTTP listen address for incoming audit-log webhooks.")
	cmdObj.Flags().StringVar(&options.ListenPath, "path", options.ListenPath,
		"HTTP path that accepts webhook requests.")
	cmdObj.Flags().IntVar(&options.MaxBodyBytes, "max-body-bytes", options.MaxBodyBytes,
		"Maximum accepted request body size in bytes.")
	cmdObj.Flags().StringVar(&options.Name, "name", options.Name,
		"Destination name. Default: kongctl-<hostname>-<pid>.")
	cmdObj.Flags().StringVar(&options.LogFormat, "log-format", options.LogFormat,
		fmt.Sprintf("Audit-log payload format. Allowed: %s.", strings.Join(allowedLogFormats, "|")))
	cmdObj.Flags().BoolVar(&options.SkipSSLVerification, "skip-ssl-verification", options.SkipSSLVerification,
		"Skip TLS certificate verification for destination delivery.")
	cmdObj.Flags().StringVar(&options.Authorization, "authorization", options.Authorization,
		"Value for the Authorization header Konnect includes when sending audit logs. "+
			"The local listener validates this same value when provided.")
	cmdObj.Flags().BoolVar(&options.ConfigureWebhook, "configure-webhook", options.ConfigureWebhook,
		"Automatically bind and enable the organization webhook with the created destination.")
	cmdObj.Flags().BoolVar(&options.Tail, "tail", options.Tail,
		"Stream received audit-log records to stdout.")
	cmdObj.Flags().BoolVarP(&options.Detach, "detach", "d", options.Detach,
		"Run listener in background as a detached kongctl process (not compatible with --tail).")
	cmdObj.Flags().StringVar(&options.JQ, "jq", options.JQ,
		"Filter streamed JSON records using a jq expression (only used with --tail).")
}

type listenAuditLogsCmd struct {
	*cobra.Command
	options ListenAuditLogsOptions
}

func newListenAuditLogsCmd(
	verb verbs.VerbValue,
	baseCmd *cobra.Command,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	c := &listenAuditLogsCmd{
		options: DefaultListenAuditLogsOptions(),
	}

	baseCmd.Short = "Create Konnect audit-log destination and listen for events locally"
	baseCmd.Long = `Create a Konnect audit-log destination and webhook configuration first,
then start a local listener to accept incoming audit-log events.`
	baseCmd.Example = `  # Build destination endpoint from public base URL and listener path
  kongctl listen audit-logs --public-url https://example.ngrok.app

  # Provide an explicit destination endpoint
  kongctl listen audit-logs --endpoint https://example.ngrok.app/audit-logs

  # Explicit product form
  kongctl listen konnect audit-logs --public-url https://example.ngrok.app`
	baseCmd.RunE = c.runE

	c.Command = baseCmd
	AddListenAuditLogsFlags(c.Command, &c.options)

	if parentPreRun != nil {
		c.PreRunE = parentPreRun
	}
	if addParentFlags != nil {
		addParentFlags(verb, c.Command)
	}

	return c.Command
}

func (c *listenAuditLogsCmd) runE(cmdObj *cobra.Command, args []string) error {
	return ExecuteListenAuditLogs(cmdObj, args, c.options)
}

// ExecuteListenAuditLogs performs destination create -> listener run -> destination delete.
func ExecuteListenAuditLogs(cmdObj *cobra.Command, args []string, options ListenAuditLogsOptions) error {
	helper := cmd.BuildHelper(cmdObj, args)
	if len(helper.GetArgs()) > 0 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("the audit-logs command does not accept positional arguments"),
		}
	}
	logger, err := helper.GetLogger()
	if err != nil {
		return err
	}
	removeDetachedProcessRecord := false
	defer func() {
		cleanupDetachedProcessRecord(logger, removeDetachedProcessRecord)
	}()

	if err := validateListenAuditLogsOptions(options); err != nil {
		return &cmd.ConfigurationError{Err: err}
	}

	listenPath := normalizePath(options.ListenPath)
	endpoint := strings.TrimSpace(options.Endpoint)
	publicURL := strings.TrimSpace(options.PublicURL)
	tailEnabled := options.Tail
	logger.Debug(
		"listen audit-logs command started",
		"endpoint_provided", endpoint != "",
		"public_url_provided", publicURL != "",
		"listen_address", strings.TrimSpace(options.ListenAddress),
		"listen_path", listenPath,
		"max_body_bytes", options.MaxBodyBytes,
		"name_provided", strings.TrimSpace(options.Name) != "",
		"log_format", strings.TrimSpace(options.LogFormat),
		"skip_ssl_verification", options.SkipSSLVerification,
		"authorization_configured", strings.TrimSpace(options.Authorization) != "",
		"configure_webhook", options.ConfigureWebhook,
		"tail_enabled", tailEnabled,
		"detach_enabled", options.Detach,
		"jq_configured", strings.TrimSpace(options.JQ) != "",
	)

	var tailEmitter *tailEventEmitter
	if tailEnabled {
		tailEmitter, err = newTailEventEmitter(helper.GetStreams(), strings.TrimSpace(options.JQ))
		if err != nil {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf("invalid tail configuration: %w", err),
			}
		}
	}

	if endpoint == "" {
		if publicURL == "" {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf("provide either --endpoint or --public-url"),
			}
		}
		derived, err := buildEndpointFromPublicURL(publicURL, listenPath)
		if err != nil {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf("failed to build endpoint from --public-url and --path: %w", err),
			}
		}
		endpoint = derived
		logger.Debug("resolved destination endpoint from public URL", "endpoint", sanitizeEndpointForLog(endpoint))
	}

	if options.Detach {
		return launchDetachedListenProcess(helper, logger)
	}

	destCmd := &createDestinationCmd{
		endpoint:            endpoint,
		name:                strings.TrimSpace(options.Name),
		logFormat:           strings.TrimSpace(options.LogFormat),
		skipSSLVerification: options.SkipSSLVerification,
		authorization:       strings.TrimSpace(options.Authorization),
		configureWebhook:    options.ConfigureWebhook,
	}
	destOutput, err := destCmd.execute(helper)
	if err != nil {
		return err
	}
	logger.Debug(
		"audit-log destination ready for listener",
		"destination_id", destOutput.DestinationID,
		"name", destOutput.DestinationName,
		"endpoint", sanitizeEndpointForLog(destOutput.DestinationEndpoint),
		"webhook_configured", destOutput.WebhookConfigured,
	)

	if strings.TrimSpace(destOutput.DestinationID) == "" {
		return cmd.PrepareExecutionError(
			"failed to listen for audit logs",
			fmt.Errorf("destination creation succeeded but destination ID is missing"),
			helper.GetCmd(),
		)
	}

	listenerCmd := &createListenerCmd{
		listenAddress:         strings.TrimSpace(options.ListenAddress),
		listenPath:            listenPath,
		publicURL:             publicURL,
		maxBodyBytes:          options.MaxBodyBytes,
		expectedAuthorization: strings.TrimSpace(options.Authorization),
		onStarted: func(helper cmd.Helper, state createListenerState) error {
			if tailEnabled {
				return renderTailStartedOutput(logger, destOutput, state)
			}
			return renderListenStartedOutput(helper, destOutput, state)
		},
	}
	if tailEmitter != nil {
		listenerCmd.onRecords = tailEmitter.EmitRecords
	}
	logger.Debug(
		"starting local audit-log listener",
		"listen_address", listenerCmd.listenAddress,
		"listen_path", listenerCmd.listenPath,
	)
	listenerErr := listenerCmd.runE(cmdObj, nil)
	if listenerErr != nil {
		logger.Debug("audit-log listener exited with error", "error", listenerErr)
	} else {
		logger.Debug("audit-log listener exited cleanly")
	}

	cleanupErr := deleteDestinationForHelper(helper, destOutput.DestinationID, destOutput.WebhookConfigured)
	if cleanupErr == nil {
		if tailEnabled {
			renderTailStoppedOutput(logger, destOutput.DestinationID)
		} else {
			if err := renderListenStoppedOutput(helper, destOutput.DestinationID); err != nil {
				return err
			}
		}
	}

	if listenerErr != nil {
		if cleanupErr != nil {
			logger.Debug(
				"listener error and destination cleanup error",
				"listener_error", listenerErr,
				"cleanup_error", cleanupErr,
			)
			return cmd.PrepareExecutionError(
				"listener terminated and destination cleanup failed",
				errors.Join(listenerErr, fmt.Errorf("cleanup error: %w", cleanupErr)),
				helper.GetCmd(),
			)
		}
		return listenerErr
	}

	if cleanupErr != nil {
		logger.Debug("destination cleanup failed after listener exit", "cleanup_error", cleanupErr)
		return cmd.PrepareExecutionError(
			"listener stopped but destination cleanup failed",
			cleanupErr,
			helper.GetCmd(),
		)
	}
	logger.Debug("listen audit-logs command completed successfully", "destination_id", destOutput.DestinationID)
	removeDetachedProcessRecord = true

	return nil
}

func validateListenAuditLogsOptions(options ListenAuditLogsOptions) error {
	if options.Detach && options.Tail {
		return fmt.Errorf("--detach is not supported with --tail")
	}

	if strings.TrimSpace(options.JQ) != "" && !options.Tail {
		return fmt.Errorf("--jq requires --tail")
	}

	return nil
}

func launchDetachedListenProcess(helper cmd.Helper, logger *slog.Logger) error {
	childLogTemplate, err := resolveDetachedChildLogTemplate(helper)
	if err != nil {
		return cmd.PrepareExecutionError(
			"failed to resolve detached listener log file",
			err,
			helper.GetCmd(),
		)
	}
	processRecordTemplate, err := processes.ResolvePathTemplate()
	if err != nil {
		return cmd.PrepareExecutionError(
			"failed to resolve detached process record path",
			err,
			helper.GetCmd(),
		)
	}

	childArgs := buildDetachedChildArgs(os.Args[1:], childLogTemplate)
	execPath, err := os.Executable()
	if err != nil {
		return cmd.PrepareExecutionError(
			"failed to determine executable path for detached listener",
			err,
			helper.GetCmd(),
		)
	}

	logger.Debug(
		"launching detached audit-log listener process",
		"executable", execPath,
		"child_log_file_template", childLogTemplate,
		"process_record_template", processRecordTemplate,
	)

	child := exec.Command(execPath, childArgs...)
	stdioSink, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		return cmd.PrepareExecutionError(
			"failed to open detached listener stdio sink",
			err,
			helper.GetCmd(),
		)
	}
	defer stdioSink.Close()

	child.Stdout = stdioSink
	child.Stderr = stdioSink
	child.Stdin = nil
	child.Env = withEnvVar(
		os.Environ(),
		processes.ProcessRecordPathEnv,
		processRecordTemplate,
	)

	if err := child.Start(); err != nil {
		return cmd.PrepareExecutionError(
			"failed to start detached listener process",
			err,
			helper.GetCmd(),
		)
	}

	childPID := child.Process.Pid
	childLogFile := detachedLogFileForPID(childLogTemplate, childPID)
	processRecordPath := processes.ResolvePathFromTemplate(processRecordTemplate, childPID)
	startTicks := processes.WaitForStartTimeTicks(childPID, 500*time.Millisecond)

	cfg, err := helper.GetConfig()
	if err != nil {
		_ = child.Process.Kill()
		return err
	}

	processRecord := processes.Record{
		PID:            childPID,
		Kind:           detachedListenerProcessKind,
		Profile:        cfg.GetProfile(),
		CreatedAt:      time.Now().UTC(),
		LogFile:        childLogFile,
		Args:           childArgs,
		StartTimeTicks: startTicks,
	}
	if err := processes.WriteRecord(processRecordPath, processRecord); err != nil {
		_ = child.Process.Kill()
		return cmd.PrepareExecutionError(
			"failed to write detached process record",
			err,
			helper.GetCmd(),
		)
	}

	if err := child.Process.Release(); err != nil {
		logger.Debug(
			"failed to release detached child process handle",
			"error", err,
			"child_pid", childPID,
		)
	}

	logger.Info(
		"launched detached audit-log listener process",
		"child_pid", childPID,
		"child_log_file", childLogFile,
		"process_record_file", processRecordPath,
	)

	if err := renderDetachedStartedOutput(helper, childPID, childLogFile, processRecordPath); err != nil {
		return err
	}

	return nil
}

func renderDetachedStartedOutput(
	helper cmd.Helper,
	childPID int,
	childLogFile, processRecordPath string,
) error {
	out := helper.GetStreams().Out
	if out == nil {
		return nil
	}

	lines := []string{
		"Detached Konnect audit-log listener started.",
		"  pid: " + sanitizeTerminalOutputValue(strconv.Itoa(childPID)),
		"  log file: " + sanitizeTerminalOutputValue(childLogFile),
		"  process record: " + sanitizeTerminalOutputValue(processRecordPath),
		"Use the log file to inspect listener startup and runtime details.",
	}

	for _, line := range lines {
		if err := writeTerminalLine(out, line); err != nil {
			return err
		}
	}

	return nil
}

func resolveDetachedChildLogTemplate(helper cmd.Helper) (string, error) {
	cfg, err := helper.GetConfig()
	if err != nil {
		return "", err
	}

	logPath := strings.TrimSpace(cfg.GetString(cmdcommon.LogFileConfigPath))
	if logPath == "" {
		configPath := cfg.GetPath()
		configDir := filepath.Dir(configPath)
		logPath = filepath.Join(configDir, "logs", meta.CLIName+".log")
	}

	logDir := filepath.Dir(logPath)
	if logDir == "" {
		logDir = "."
	}
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return "", err
	}

	return filepath.Join(logDir, detachedListenerLogFileTemplate), nil
}

func detachedLogFileForPID(template string, pid int) string {
	return strings.ReplaceAll(template, detachedListenerPIDToken, fmt.Sprintf("%d", pid))
}

func buildDetachedChildArgs(parentArgs []string, childLogTemplate string) []string {
	args := removeBooleanFlag(parentArgs, "--detach", "-d")
	args = removeStringFlag(args, "--"+cmdcommon.LogFileFlagName)
	args = append(args, "--"+cmdcommon.LogFileFlagName, childLogTemplate)

	return args
}

func removeBooleanFlag(args []string, longFlag, shortFlag string) []string {
	filtered := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		value := args[i]
		if value == longFlag || value == shortFlag {
			if i+1 < len(args) && isBoolLiteral(args[i+1]) {
				i++
			}
			continue
		}
		if strings.HasPrefix(value, longFlag+"=") || strings.HasPrefix(value, shortFlag+"=") {
			continue
		}
		filtered = append(filtered, value)
	}

	return filtered
}

func removeStringFlag(args []string, longFlag string) []string {
	filtered := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		value := args[i]
		if value == longFlag {
			if i+1 < len(args) {
				i++
			}
			continue
		}
		if strings.HasPrefix(value, longFlag+"=") {
			continue
		}
		filtered = append(filtered, value)
	}

	return filtered
}

func isBoolLiteral(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "0", "t", "f", "true", "false":
		return true
	default:
		return false
	}
}

func sanitizeTerminalOutputValue(value string) string {
	sanitized := strings.TrimSpace(value)
	sanitized = strings.ReplaceAll(sanitized, "\r", "")
	sanitized = strings.ReplaceAll(sanitized, "\n", "")
	if sanitized == "" {
		return "n/a"
	}
	return sanitized
}

func writeTerminalLine(out io.Writer, line string) error {
	if out == nil {
		return nil
	}

	_, err := out.Write([]byte(line + "\n"))
	return err
}

func cleanupDetachedProcessRecord(logger *slog.Logger, remove bool) {
	recordPath := processes.ResolvePathFromEnv(os.Getpid())
	if strings.TrimSpace(recordPath) == "" {
		return
	}
	if !remove {
		if logger != nil {
			logger.Debug("keeping detached process record due command error", "record_file", recordPath)
		}
		return
	}

	if err := processes.RemoveRecordByPath(recordPath); err != nil {
		if logger != nil {
			logger.Debug("failed to remove detached process record", "record_file", recordPath, "error", err)
		}
		return
	}

	if logger != nil {
		logger.Debug("removed detached process record", "record_file", recordPath)
	}
}

func withEnvVar(env []string, key, value string) []string {
	prefix := key + "="
	updated := make([]string, 0, len(env)+1)
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			continue
		}
		updated = append(updated, entry)
	}
	updated = append(updated, prefix+value)
	return updated
}

func renderListenStartedOutput(
	helper cmd.Helper,
	destination createDestinationOutput,
	listener createListenerState,
) error {
	out := helper.GetStreams().Out
	return renderListenStartedOutputToWriter(out, destination, listener)
}

func renderListenStartedOutputToWriter(
	out io.Writer,
	destination createDestinationOutput,
	listener createListenerState,
) error {
	if out == nil {
		return nil
	}

	if _, err := fmt.Fprintln(out, "Konnect Audit-Log Listener Started"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(out, ""); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(out, "Destination"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "  name: %s\n", destination.DestinationName); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "  id: %s\n", destination.DestinationID); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "  endpoint: %s\n", destination.DestinationEndpoint); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "  log format: %s\n", destination.LogFormat); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "  skip ssl verification: %t\n", destination.SkipSSLVerification); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "  authorization configured: %t\n", destination.AuthorizationConfigured); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "  webhook configured: %t\n", destination.WebhookConfigured); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(out, ""); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(out, "Listener"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "  local endpoint: %s\n", listener.LocalEndpoint); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "  listen address: %s\n", listener.ListenAddress); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "  path: %s\n", listener.ListenPath); err != nil {
		return err
	}
	if strings.TrimSpace(listener.PublicURL) != "" {
		if _, err := fmt.Fprintf(out, "  public base URL: %s\n", listener.PublicURL); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(out, "  events file: %s\n", listener.EventsFile); err != nil {
		return err
	}
	if destination.DestinationStateFile != "" {
		if _, err := fmt.Fprintf(out, "  destination state file: %s\n", destination.DestinationStateFile); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(out, ""); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(out, "Listening for audit-log events. Press Ctrl+C to stop."); err != nil {
		return err
	}

	return nil
}

func renderListenStoppedOutput(helper cmd.Helper, destinationID string) error {
	out := helper.GetStreams().Out
	if out == nil {
		return nil
	}
	_, err := fmt.Fprintf(out, "\nListener stopped. Deleted audit-log destination: %s\n", destinationID)
	return err
}

func renderTailStartedOutput(
	logger *slog.Logger,
	destination createDestinationOutput,
	listener createListenerState,
) error {
	if logger == nil {
		return nil
	}

	logger.Info(
		"Konnect Audit-Log Listener Started (tail mode)",
		"destination_name", destination.DestinationName,
		"destination_id", destination.DestinationID,
		"destination_endpoint", sanitizeEndpointForLog(destination.DestinationEndpoint),
		"log_format", destination.LogFormat,
		"skip_ssl_verification", destination.SkipSSLVerification,
		"authorization_configured", destination.AuthorizationConfigured,
		"webhook_configured", destination.WebhookConfigured,
		"local_endpoint", listener.LocalEndpoint,
		"listen_address", listener.ListenAddress,
		"listen_path", listener.ListenPath,
		"events_file", listener.EventsFile,
		"destination_state_file", destination.DestinationStateFile,
		"public_url", strings.TrimSpace(listener.PublicURL),
	)
	logger.Info("Listening for audit-log events (tail mode).")

	return nil
}

func renderTailStoppedOutput(logger *slog.Logger, destinationID string) {
	if logger == nil {
		return
	}

	logger.Info(
		"Listener stopped. Deleted audit-log destination (tail mode).",
		"destination_id", destinationID,
	)
}

func buildEndpointFromPublicURL(publicBaseURL, listenPath string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(publicBaseURL))
	if err != nil {
		return "", err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("public URL must include scheme and host")
	}

	path := normalizePath(listenPath)
	basePath := strings.TrimRight(parsed.Path, "/")
	if basePath == "" {
		parsed.Path = path
	} else if path == "/" {
		parsed.Path = basePath
	} else {
		parsed.Path = basePath + path
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""

	return parsed.String(), nil
}
