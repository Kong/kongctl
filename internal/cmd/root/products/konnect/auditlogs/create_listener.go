package auditlogs

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/kong/kongctl/internal/auditlogs"
	"github.com/kong/kongctl/internal/cmd"
	cmdcommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

const (
	defaultListenAddress = "127.0.0.1:19090"
	defaultListenPath    = "/audit-logs"
	defaultMaxBodyBytes  = 1024 * 1024
)

var errDecodedBodyTooLarge = errors.New("decoded request body too large")

type createListenerState struct {
	StartedAt     time.Time `json:"started_at"`
	Profile       string    `json:"profile"`
	ListenAddress string    `json:"listen_address"`
	ListenPath    string    `json:"listen_path"`
	LocalEndpoint string    `json:"local_endpoint"`
	PublicURL     string    `json:"public_url,omitempty"`
	EventsFile    string    `json:"events_file"`
}

type createListenerCmd struct {
	*cobra.Command

	listenAddress         string
	listenPath            string
	publicURL             string
	maxBodyBytes          int
	expectedAuthorization string
	onStarted             func(helper cmd.Helper, state createListenerState) error
	onRecords             func(records [][]byte) error
}

func newCreateListenerCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	c := &createListenerCmd{}
	cmdObj := &cobra.Command{
		Use:   "listener",
		Short: "Start a local audit-log webhook listener",
		Long: `Start a local HTTP endpoint that accepts Konnect audit-log webhook events
and persists each payload under the current profile in XDG config storage.`,
		Example: `  # Start a local listener
  kongctl create audit-logs listener

  # Listen on a custom address and path
  kongctl create audit-logs listener --listen-address 0.0.0.0:8080 --path /konnect/audit-logs`,
		RunE: c.runE,
	}

	c.Command = cmdObj
	c.Flags().StringVar(&c.listenAddress, "listen-address", defaultListenAddress,
		"HTTP listen address for incoming audit-log webhooks.")
	c.Flags().StringVar(&c.listenPath, "path", defaultListenPath,
		"HTTP path that accepts webhook requests.")
	c.Flags().StringVar(&c.publicURL, "public-url", "",
		"Externally reachable base URL for this listener (used for operator guidance).")
	c.Flags().IntVar(&c.maxBodyBytes, "max-body-bytes", defaultMaxBodyBytes,
		"Maximum accepted request body size in bytes.")
	c.Flags().StringVar(&c.expectedAuthorization, "authorization", "",
		"Expected Authorization header value for incoming audit-log webhook requests.")

	if parentPreRun != nil {
		c.PreRunE = parentPreRun
	}
	if addParentFlags != nil {
		addParentFlags(verb, c.Command)
	}

	return c.Command
}

func (c *createListenerCmd) runE(cmdObj *cobra.Command, args []string) error {
	helper := cmd.BuildHelper(cmdObj, args)
	if len(helper.GetArgs()) > 0 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("the listener command does not accept arguments"),
		}
	}

	if strings.TrimSpace(c.listenAddress) == "" {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("listen address cannot be empty"),
		}
	}
	if c.maxBodyBytes <= 0 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("max-body-bytes must be greater than zero"),
		}
	}
	listenPath := normalizePath(c.listenPath)
	expectedAuthorization := strings.TrimSpace(c.expectedAuthorization)
	if !strings.HasPrefix(listenPath, "/") {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("path must start with '/'"),
		}
	}

	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}
	logger, err := helper.GetLogger()
	if err != nil {
		return err
	}

	paths, err := auditlogs.ResolvePaths(cfg.GetProfile())
	if err != nil {
		return cmd.PrepareExecutionError("failed to resolve audit-log storage paths", err, helper.GetCmd())
	}
	store := auditlogs.NewStore(paths.EventsFile)
	logger.Debug(
		"resolved audit-log listener storage paths",
		"profile", cfg.GetProfile(),
		"events_file", paths.EventsFile,
		"listener_state_file", paths.ListenerStateFile,
	)

	mux := http.NewServeMux()
	mux.HandleFunc("/", c.newListenerHandler(listenPath, store, expectedAuthorization, logger))

	server := &http.Server{
		ReadHeaderTimeout: 10 * time.Second,
		Handler:           mux,
	}

	listener, err := net.Listen("tcp", c.listenAddress)
	if err != nil {
		return cmd.PrepareExecutionError("failed to start audit-log listener", err, helper.GetCmd())
	}

	localEndpoint := formatLocalEndpoint(listener.Addr().String(), listenPath)
	state := createListenerState{
		StartedAt:     time.Now().UTC(),
		Profile:       cfg.GetProfile(),
		ListenAddress: listener.Addr().String(),
		ListenPath:    listenPath,
		LocalEndpoint: localEndpoint,
		PublicURL:     strings.TrimSpace(c.publicURL),
		EventsFile:    paths.EventsFile,
	}
	if err := auditlogs.WriteState(paths.ListenerStateFile, state); err != nil {
		_ = listener.Close()
		return cmd.PrepareExecutionError("failed to persist listener state", err, helper.GetCmd())
	}
	logger.Debug(
		"started audit-log listener",
		"listen_address", state.ListenAddress,
		"listen_path", state.ListenPath,
		"local_endpoint", state.LocalEndpoint,
		"public_url_provided", strings.TrimSpace(state.PublicURL) != "",
		"authorization_required", expectedAuthorization != "",
	)

	if c.onStarted != nil {
		if err := c.onStarted(helper, state); err != nil {
			_ = listener.Close()
			return err
		}
	} else if err := c.printStartup(helper, state); err != nil {
		_ = listener.Close()
		return err
	}

	serveErrCh := make(chan error, 1)
	go func() {
		if serveErr := server.Serve(listener); serveErr != nil && serveErr != http.ErrServerClosed {
			serveErrCh <- serveErr
		}
		close(serveErrCh)
	}()

	baseCtx := helper.GetContext()
	if baseCtx == nil {
		baseCtx = context.Background()
	}
	ctx, stop := signal.NotifyContext(baseCtx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case <-ctx.Done():
		logger.Debug("received shutdown signal for audit-log listener")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if shutdownErr := server.Shutdown(shutdownCtx); shutdownErr != nil {
			return cmd.PrepareExecutionError("audit-log listener shutdown failed", shutdownErr, helper.GetCmd())
		}
		logger.Debug("audit-log listener shut down")
		return nil
	case serveErr := <-serveErrCh:
		if serveErr == nil {
			logger.Debug("audit-log listener server exited")
			return nil
		}
		logger.Debug("audit-log listener server exited unexpectedly", "error", serveErr)
		return cmd.PrepareExecutionError("audit-log listener stopped unexpectedly", serveErr, helper.GetCmd())
	}
}

func (c *createListenerCmd) printStartup(helper cmd.Helper, state createListenerState) error {
	outType, err := helper.GetOutputFormat()
	if err != nil {
		return err
	}
	streams := helper.GetStreams()

	if outType == cmdcommon.TEXT {
		_, err := fmt.Fprintln(streams.Out, "Audit-log listener started")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(streams.Out, "  local endpoint: %s\n", state.LocalEndpoint)
		if err != nil {
			return err
		}
		if strings.TrimSpace(state.PublicURL) != "" {
			_, err = fmt.Fprintf(streams.Out, "  public base URL: %s\n", state.PublicURL)
			if err != nil {
				return err
			}
		}
		_, err = fmt.Fprintf(streams.Out, "  events file: %s\n", state.EventsFile)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(streams.Out, "Press Ctrl+C to stop.")
		return err
	}

	printer, err := cli.Format(outType.String(), streams.Out)
	if err != nil {
		return err
	}
	defer printer.Flush()
	printer.Print(state)
	return nil
}

func (c *createListenerCmd) newListenerHandler(
	listenPath string,
	store *auditlogs.Store,
	expectedAuthorization string,
	logger *slog.Logger,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != listenPath {
			if logger != nil {
				logger.Debug(
					"ignoring request for non-listener path",
					"request_path", r.URL.Path,
					"listen_path", listenPath,
					"method", r.Method,
				)
			}
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodPost {
			if logger != nil {
				logger.Debug(
					"rejecting request with unsupported method",
					"method", r.Method,
					"listen_path", listenPath,
				)
			}
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		authorized, headerPresent := requestHasExpectedAuthorization(r, expectedAuthorization)
		if !authorized {
			if logger != nil {
				logger.Debug(
					"rejecting request with invalid authorization header",
					"listen_path", listenPath,
					"authorization_required", true,
					"authorization_header_present", headerPresent,
				)
			}
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		defer r.Body.Close()

		body, err := io.ReadAll(io.LimitReader(r.Body, int64(c.maxBodyBytes+1)))
		if err != nil {
			if logger != nil {
				logger.Debug("failed to read audit-log request body", "error", err)
			}
			http.Error(w, "failed to read request body", http.StatusBadRequest)
			return
		}
		if len(body) > c.maxBodyBytes {
			if logger != nil {
				logger.Debug(
					"rejecting audit-log request body that exceeds max size",
					"max_body_bytes", c.maxBodyBytes,
					"received_bytes", len(body),
				)
			}
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
			return
		}

		decodedBody, decodedGzip, err := maybeDecodeRequestBody(
			r.Header.Get("Content-Encoding"),
			body,
			c.maxBodyBytes,
		)
		if err != nil {
			if logger != nil {
				logger.Debug(
					"failed to decode audit-log request body",
					"error", err,
					"content_encoding", strings.TrimSpace(r.Header.Get("Content-Encoding")),
				)
			}
			if errors.Is(err, errDecodedBodyTooLarge) {
				http.Error(w, "decoded request body too large", http.StatusRequestEntityTooLarge)
				return
			}
			http.Error(w, "failed to decode request body", http.StatusBadRequest)
			return
		}

		records := auditlogs.SplitPayloadRecords(decodedBody)
		recordCount, err := store.AppendRecords(records)
		if err != nil {
			if logger != nil {
				logger.Debug("failed to persist audit-log event", "error", err)
			}
			http.Error(w, "failed to persist audit-log event", http.StatusInternalServerError)
			return
		}
		if c.onRecords != nil && len(records) > 0 {
			if err := c.onRecords(records); err != nil && logger != nil {
				logger.Debug("failed to emit streamed audit-log records", "error", err)
			}
		}
		if logger != nil {
			logger.Debug(
				"accepted and stored audit-log event",
				"method", r.Method,
				"path", r.URL.Path,
				"received_body_bytes", len(body),
				"stored_body_bytes", len(decodedBody),
				"stored_record_count", recordCount,
				"decoded_gzip", decodedGzip,
			)
		}

		w.WriteHeader(http.StatusAccepted)
	}
}

func requestHasExpectedAuthorization(r *http.Request, expectedAuthorization string) (bool, bool) {
	if strings.TrimSpace(expectedAuthorization) == "" {
		return true, true
	}

	headerValue := strings.TrimSpace(r.Header.Get("Authorization"))
	if headerValue == "" {
		return false, false
	}

	if subtle.ConstantTimeCompare([]byte(headerValue), []byte(expectedAuthorization)) == 1 {
		return true, true
	}

	return false, true
}

func maybeDecodeRequestBody(contentEncoding string, body []byte, maxBodyBytes int) ([]byte, bool, error) {
	if !hasGzipContentEncoding(contentEncoding) {
		return body, false, nil
	}

	decoded, err := decodeGzipBody(body, maxBodyBytes)
	if err != nil {
		return nil, true, err
	}

	return decoded, true, nil
}

func hasGzipContentEncoding(contentEncoding string) bool {
	if strings.TrimSpace(contentEncoding) == "" {
		return false
	}

	parts := strings.Split(strings.ToLower(contentEncoding), ",")
	for _, part := range parts {
		if strings.TrimSpace(part) == "gzip" {
			return true
		}
	}

	return false
}

func decodeGzipBody(body []byte, maxBodyBytes int) ([]byte, error) {
	if maxBodyBytes <= 0 {
		return nil, fmt.Errorf("max body bytes must be greater than zero")
	}

	reader, err := gzip.NewReader(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	decoded, err := io.ReadAll(io.LimitReader(reader, int64(maxBodyBytes+1)))
	if err != nil {
		return nil, err
	}
	if len(decoded) > maxBodyBytes {
		return nil, errDecodedBodyTooLarge
	}

	return decoded, nil
}

func normalizePath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return defaultListenPath
	}
	if !strings.HasPrefix(trimmed, "/") {
		return "/" + trimmed
	}
	return trimmed
}

func formatLocalEndpoint(address, path string) string {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Sprintf("http://%s%s", strings.TrimSpace(address), path)
	}

	if host == "" || host == "0.0.0.0" || host == "::" || host == "[::]" {
		host = "127.0.0.1"
	}
	if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
		host = "[" + host + "]"
	}
	return fmt.Sprintf("http://%s:%s%s", host, port, path)
}
