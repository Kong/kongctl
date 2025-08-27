package httpclient

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/kong/kongctl/internal/log"
)

// LoggingHTTPClient wraps an HTTP client to add trace logging
type LoggingHTTPClient struct {
	wrapped *http.Client
	logger  *slog.Logger
}

// NewLoggingHTTPClient creates a new logging HTTP client
func NewLoggingHTTPClient(logger *slog.Logger) *LoggingHTTPClient {
	return &LoggingHTTPClient{
		wrapped: &http.Client{
			Timeout: 60 * time.Second,
		},
		logger: logger,
	}
}

// NewLoggingHTTPClientWithClient wraps an existing HTTP client
func NewLoggingHTTPClientWithClient(client *http.Client, logger *slog.Logger) *LoggingHTTPClient {
	return &LoggingHTTPClient{
		wrapped: client,
		logger:  logger,
	}
}

// Do implements the HTTPClient interface with logging
func (c *LoggingHTTPClient) Do(req *http.Request) (*http.Response, error) {
	// Only log if trace level is enabled
	if !c.logger.Enabled(req.Context(), log.LevelTrace) {
		return c.wrapped.Do(req)
	}

	start := time.Now()

	// Log request
	c.logRequest(req)

	// Perform the actual request
	resp, err := c.wrapped.Do(req)

	// Log response or error
	duration := time.Since(start)
	if err != nil {
		c.logger.LogAttrs(req.Context(), log.LevelTrace, "HTTP request failed",
			slog.String("method", req.Method),
			slog.String("url", req.URL.String()),
			slog.Duration("duration", duration),
			slog.String("error", err.Error()),
		)
		return nil, err
	}

	c.logResponse(resp, duration)

	return resp, nil
}

func (c *LoggingHTTPClient) logRequest(req *http.Request) {
	attrs := []slog.Attr{
		slog.String("method", req.Method),
		slog.String("url", req.URL.String()),
		slog.String("host", req.Host),
	}

	// Log headers (with redaction)
	headers := make(map[string]string)
	for k, v := range req.Header {
		key := strings.ToLower(k)
		if key == "authorization" || key == "x-api-key" || strings.Contains(key, "token") {
			headers[k] = "[REDACTED]"
		} else {
			headers[k] = strings.Join(v, ", ")
		}
	}
	attrs = append(attrs, slog.Any("headers", headers))

	// Log body size if present
	if req.Body != nil && req.ContentLength > 0 {
		attrs = append(attrs, slog.Int64("content_length", req.ContentLength))
	}

	c.logger.LogAttrs(req.Context(), log.LevelTrace, "HTTP request", attrs...)
}

func (c *LoggingHTTPClient) logResponse(resp *http.Response, duration time.Duration) {
	attrs := []slog.Attr{
		slog.Int("status", resp.StatusCode),
		slog.String("status_text", resp.Status),
		slog.Duration("duration", duration),
	}

	// Log headers (with redaction)
	headers := make(map[string]string)
	for k, v := range resp.Header {
		key := strings.ToLower(k)
		if key == "set-cookie" || strings.Contains(key, "token") {
			headers[k] = "[REDACTED]"
		} else {
			headers[k] = strings.Join(v, ", ")
		}
	}
	attrs = append(attrs, slog.Any("headers", headers))

	// Log body size if present
	if resp.ContentLength > 0 {
		attrs = append(attrs, slog.Int64("content_length", resp.ContentLength))
	}

	// For trace logging of error responses, try to read and log the body
	if resp.StatusCode >= 400 && c.logger.Enabled(resp.Request.Context(), log.LevelTrace) {
		body, err := c.peekResponseBody(resp)
		if err == nil && len(body) > 0 {
			// Truncate large bodies
			maxLen := 1000
			if len(body) > maxLen {
				body = fmt.Sprintf("%s... [truncated, total %d bytes]", body[:maxLen], len(body))
			}
			attrs = append(attrs, slog.String("error_body", body))
		}
	}

	c.logger.LogAttrs(resp.Request.Context(), log.LevelTrace, "HTTP response", attrs...)
}

// peekResponseBody reads the response body without consuming it
func (c *LoggingHTTPClient) peekResponseBody(resp *http.Response) (string, error) {
	if resp.Body == nil {
		return "", nil
	}

	// Read the body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// Restore the body for the SDK to read
	resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	return string(bodyBytes), nil
}
