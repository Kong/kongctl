package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync/atomic"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/kong/kongctl/internal/log"
)

const (
	httpSource           = "sdk-konnect-go"
	logTypeRequest       = "http_request"
	logTypeResponse      = "http_response"
	logTypeError         = "http_error"
	redactedValue        = "[REDACTED]"
	maxBodyLogChars      = 4096
	omittedBinaryMessage = "[OMITTED: non-text body]"
)

type redactionPattern struct {
	regex       *regexp.Regexp
	replacement string
}

// NOTE: MustCompile is intentional here. If these patterns become invalid we should fail fast
// rather than run without redaction safeguards.
// These are only applied for trace-level body logging, and body content is bounded by
// maxBodyLogChars before it is written to logs.
var plainTextRedactionPatterns = []redactionPattern{
	{
		regex:       regexp.MustCompile(`(?i)(\bbearer\s+)([A-Za-z0-9\-._~+/]{8,}=*)\b`),
		replacement: `${1}` + redactedValue,
	},
	{
		regex: regexp.MustCompile(`(?i)("?(?:access[_-]?token|refresh[_-]?token|token|password|secret|` +
			`api[_-]?key|apikey|client[_-]?secret|authorization|cookie)"?\s*[:=]\s*")([^"]*)(")`),
		replacement: `${1}` + redactedValue + `${3}`,
	},
	{
		regex: regexp.MustCompile(`(?i)((?:access[_-]?token|refresh[_-]?token|token|password|secret|` +
			`api[_-]?key|apikey|client[_-]?secret|authorization|cookie)\s*[=:]\s*)([^\s&,;]+)`),
		replacement: `${1}` + redactedValue,
	},
}

var sensitiveExactFieldKeys = map[string]struct{}{
	"access_token":        {},
	"refresh_token":       {},
	"id_token":            {},
	"token":               {},
	"api_key":             {},
	"apikey":              {},
	"x_api_key":           {},
	"secret":              {},
	"password":            {},
	"authorization":       {},
	"cookie":              {},
	"credential":          {},
	"private_key":         {},
	"passphrase":          {},
	"client_secret":       {},
	"set_cookie":          {},
	"konnectaccesstoken":  {},
	"konnectrefreshtoken": {},
}

var nonSensitiveTokenFieldKeys = map[string]struct{}{
	"token_count": {},
	"token_type":  {},
}

// LoggingHTTPClient wraps an HTTP client to add centralized request/response logging.
type LoggingHTTPClient struct {
	wrapped        *http.Client
	logger         *slog.Logger
	requestCounter atomic.Uint64
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
	if c == nil || c.wrapped == nil {
		return nil, fmt.Errorf("http client is not configured")
	}
	if req == nil {
		return nil, fmt.Errorf("request is nil")
	}

	ctx := requestContext(req, context.Background())
	debugEnabled := c.logger != nil && c.logger.Enabled(ctx, slog.LevelDebug)
	traceEnabled := c.logger != nil && c.logger.Enabled(ctx, log.LevelTrace)
	if !debugEnabled && !traceEnabled {
		return c.wrapped.Do(req)
	}

	requestID := c.nextRequestID()
	start := time.Now()

	var requestBody []byte
	var requestBodyErr error
	if traceEnabled {
		requestBody, requestBodyErr = c.peekRequestBody(req)
	}
	c.logRequest(ctx, req, requestID, traceEnabled, requestBody, requestBodyErr)

	resp, err := c.wrapped.Do(req)
	duration := time.Since(start)
	if err != nil {
		c.logRequestError(ctx, req, requestID, duration, err)
		return nil, err
	}

	var responseBody []byte
	var responseBodyErr error
	if traceEnabled {
		responseBody, responseBodyErr = c.peekResponseBody(resp)
	}

	c.logResponse(
		requestContext(resp.Request, ctx),
		resp,
		requestID,
		duration,
		traceEnabled,
		responseBody,
		responseBodyErr,
	)
	return resp, nil
}

func (c *LoggingHTTPClient) logRequest(
	ctx context.Context,
	req *http.Request,
	requestID string,
	traceEnabled bool,
	body []byte,
	bodyErr error,
) {
	attrs := []slog.Attr{
		slog.String("log_type", logTypeRequest),
		slog.String("http_source", httpSource),
		slog.String("request_id", requestID),
		slog.String("method", req.Method),
		slog.String("route", routeFromURL(req.URL)),
	}
	attrs = append(attrs, log.HTTPLogContextAttrs(ctx)...)

	if req.URL != nil {
		attrs = append(attrs, slog.String("host", req.URL.Host))
		if query := sanitizeQuery(req.URL.Query()); len(query) > 0 {
			attrs = append(attrs, slog.Any("query_params", query))
		}
	}

	if req.ContentLength > 0 {
		attrs = append(attrs, slog.Int64("request_content_length", req.ContentLength))
	}

	if traceEnabled {
		if headers := sanitizeHeaders(req.Header); len(headers) > 0 {
			attrs = append(attrs, slog.Any("request_headers", headers))
		}
		contentType := req.Header.Get("Content-Type")
		if contentType != "" {
			attrs = append(attrs, slog.String("request_content_type", contentType))
		}
		if bodyErr != nil {
			attrs = append(attrs, slog.String("request_body_error", bodyErr.Error()))
		} else if len(body) > 0 {
			attrs = append(attrs, slog.String("request_body", redactBody(body, contentType)))
		}
	}

	c.logger.LogAttrs(ctx, slog.LevelDebug, "request", attrs...)
}

func (c *LoggingHTTPClient) logRequestError(
	ctx context.Context,
	req *http.Request,
	requestID string,
	duration time.Duration,
	reqErr error,
) {
	attrs := []slog.Attr{
		slog.String("log_type", logTypeError),
		slog.String("http_source", httpSource),
		slog.String("request_id", requestID),
		slog.String("method", req.Method),
		slog.String("route", routeFromURL(req.URL)),
		slog.Duration("duration", duration),
		slog.String("error", reqErr.Error()),
	}
	attrs = append(attrs, log.HTTPLogContextAttrs(ctx)...)

	if req.URL != nil {
		attrs = append(attrs, slog.String("host", req.URL.Host))
		if query := sanitizeQuery(req.URL.Query()); len(query) > 0 {
			attrs = append(attrs, slog.Any("query_params", query))
		}
	}

	c.logger.LogAttrs(ctx, slog.LevelWarn, "request failed", attrs...)
}

func (c *LoggingHTTPClient) logResponse(
	ctx context.Context,
	resp *http.Response,
	requestID string,
	duration time.Duration,
	traceEnabled bool,
	body []byte,
	bodyErr error,
) {
	attrs := []slog.Attr{
		slog.String("log_type", logTypeResponse),
		slog.String("http_source", httpSource),
		slog.String("request_id", requestID),
		slog.Int("status_code", resp.StatusCode),
		slog.String("status_text", resp.Status),
		slog.Duration("duration", duration),
	}
	attrs = append(attrs, log.HTTPLogContextAttrs(ctx)...)

	if resp.ContentLength > 0 {
		attrs = append(attrs, slog.Int64("response_content_length", resp.ContentLength))
	}
	contentType := resp.Header.Get("Content-Type")
	if contentType != "" {
		attrs = append(attrs, slog.String("response_content_type", contentType))
	}

	if resp.Request != nil {
		// Keep route metadata on responses for easy filtering/aggregation without joining to request
		// logs via request_id (for example: response-only analysis by method/route/status).
		attrs = append(attrs,
			slog.String("method", resp.Request.Method),
			slog.String("route", routeFromURL(resp.Request.URL)),
		)
		if resp.Request.URL != nil {
			attrs = append(attrs, slog.String("host", resp.Request.URL.Host))
			if query := sanitizeQuery(resp.Request.URL.Query()); len(query) > 0 {
				attrs = append(attrs, slog.Any("query_params", query))
			}
		}
	}

	if traceEnabled {
		if headers := sanitizeHeaders(resp.Header); len(headers) > 0 {
			attrs = append(attrs, slog.Any("response_headers", headers))
		}
		if bodyErr != nil {
			attrs = append(attrs, slog.String("response_body_error", bodyErr.Error()))
		} else if len(body) > 0 {
			attrs = append(attrs, slog.String("response_body", redactBody(body, contentType)))
		}
	}

	c.logger.LogAttrs(ctx, slog.LevelDebug, "response", attrs...)
}

func (c *LoggingHTTPClient) nextRequestID() string {
	return fmt.Sprintf("khttp-%06d", c.requestCounter.Add(1))
}

func requestContext(req *http.Request, fallback context.Context) context.Context {
	if req != nil && req.Context() != nil {
		return req.Context()
	}
	return fallback
}

func routeFromURL(parsedURL *url.URL) string {
	if parsedURL == nil {
		return ""
	}
	if parsedURL.Path == "" {
		return "/"
	}
	return parsedURL.Path
}

func sanitizeQuery(values url.Values) map[string]string {
	if len(values) == 0 {
		return nil
	}

	sanitized := make(map[string]string, len(values))
	for key, currentValues := range values {
		if isSensitiveFieldKey(key) {
			sanitized[key] = redactedValue
			continue
		}
		sanitized[key] = strings.Join(currentValues, ",")
	}
	return sanitized
}

func sanitizeHeaders(headers http.Header) map[string]string {
	if len(headers) == 0 {
		return nil
	}

	sanitized := make(map[string]string, len(headers))
	for key, values := range headers {
		if isSensitiveHeaderKey(key) {
			sanitized[key] = redactedValue
			continue
		}
		sanitized[key] = strings.Join(values, ", ")
	}
	return sanitized
}

func isSensitiveHeaderKey(key string) bool {
	normalized := normalizeKey(key)
	if normalized == "authorization" || normalized == "proxy_authorization" ||
		normalized == "cookie" || normalized == "set_cookie" || normalized == "x_api_key" {
		return true
	}
	return isSensitiveFieldKey(normalized)
}

func isSensitiveFieldKey(key string) bool {
	normalized := normalizeKey(key)
	if normalized == "" {
		return false
	}

	if _, ok := sensitiveExactFieldKeys[normalized]; ok {
		return true
	}

	// Preserve useful diagnostics for metadata fields like token_count/token_type.
	if _, ok := nonSensitiveTokenFieldKeys[normalized]; ok {
		return false
	}

	if containsSegment(normalized, "secret") ||
		containsSegment(normalized, "password") ||
		containsSegment(normalized, "credential") ||
		containsSegment(normalized, "passphrase") ||
		hasSegmentPair(normalized, "private", "key") ||
		hasSegmentPair(normalized, "api", "key") ||
		hasSegmentPair(normalized, "client", "secret") {
		return true
	}

	if strings.Contains(normalized, "access_token") || strings.Contains(normalized, "refresh_token") {
		return true
	}
	if strings.HasSuffix(normalized, "_token") {
		return true
	}

	return false
}

func containsSegment(normalized, segment string) bool {
	for _, current := range strings.Split(normalized, "_") {
		if current == segment {
			return true
		}
	}
	return false
}

func hasSegmentPair(normalized, first, second string) bool {
	parts := strings.Split(normalized, "_")
	for idx := 0; idx < len(parts)-1; idx++ {
		if parts[idx] == first && parts[idx+1] == second {
			return true
		}
	}
	return false
}

func normalizeKey(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}

	runes := []rune(key)
	out := make([]rune, 0, len(runes))
	appendUnderscore := func() {
		if len(out) > 0 && out[len(out)-1] != '_' {
			out = append(out, '_')
		}
	}

	for idx, current := range runes {
		switch {
		case current == '_' || current == '-' || unicode.IsSpace(current):
			appendUnderscore()
		case unicode.IsUpper(current):
			if idx > 0 {
				prev := runes[idx-1]
				nextIsLower := idx+1 < len(runes) && unicode.IsLower(runes[idx+1])
				if unicode.IsLower(prev) || unicode.IsDigit(prev) || (unicode.IsUpper(prev) && nextIsLower) {
					appendUnderscore()
				}
			}
			out = append(out, unicode.ToLower(current))
		default:
			out = append(out, unicode.ToLower(current))
		}
	}

	return strings.Trim(string(out), "_")
}

func (c *LoggingHTTPClient) peekRequestBody(req *http.Request) ([]byte, error) {
	if req == nil || req.Body == nil || req.Body == http.NoBody {
		return nil, nil
	}

	bodyBytes, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}

	req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	if req.GetBody == nil {
		req.GetBody = func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(bodyBytes)), nil
		}
	}
	return bodyBytes, nil
}

// peekResponseBody reads the response body without consuming it.
func (c *LoggingHTTPClient) peekResponseBody(resp *http.Response) ([]byte, error) {
	if resp == nil || resp.Body == nil || resp.Body == http.NoBody {
		return nil, nil
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	return bodyBytes, nil
}

func redactBody(body []byte, contentType string) string {
	if len(body) == 0 {
		return ""
	}

	if !isTextualBody(body, contentType) {
		return fmt.Sprintf("%s (%d bytes)", omittedBinaryMessage, len(body))
	}

	contentType = normalizeContentType(contentType)
	if strings.Contains(contentType, "json") {
		if redacted, ok := redactJSONBody(body); ok {
			return truncateBody(redacted)
		}
	}
	if strings.Contains(contentType, "x-www-form-urlencoded") {
		if redacted, ok := redactFormBody(body); ok {
			return truncateBody(redacted)
		}
	}

	redacted := redactPlainTextBody(string(body))
	return truncateBody(redacted)
}

func isTextualBody(body []byte, contentType string) bool {
	contentType = normalizeContentType(contentType)
	if contentType == "" {
		return utf8.Valid(body)
	}

	if strings.HasPrefix(contentType, "text/") {
		return true
	}

	textualMarkers := []string{
		"json",
		"xml",
		"yaml",
		"javascript",
		"x-www-form-urlencoded",
		"html",
		"csv",
		"x-ndjson",
	}
	for _, marker := range textualMarkers {
		if strings.Contains(contentType, marker) {
			return true
		}
	}
	return false
}

func normalizeContentType(contentType string) string {
	contentType = strings.TrimSpace(contentType)
	if idx := strings.Index(contentType, ";"); idx >= 0 {
		contentType = contentType[:idx]
	}
	return strings.ToLower(strings.TrimSpace(contentType))
}

func redactJSONBody(body []byte) (string, bool) {
	var payload any
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return "", false
	}

	payload = redactJSONValue(payload)
	redactedBody, err := json.Marshal(payload)
	if err != nil {
		return "", false
	}

	return string(redactedBody), true
}

func redactJSONValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		for key, nested := range typed {
			if isSensitiveFieldKey(key) {
				typed[key] = redactedValue
				continue
			}
			typed[key] = redactJSONValue(nested)
		}
	case []any:
		for idx, nested := range typed {
			typed[idx] = redactJSONValue(nested)
		}
	}
	return value
}

func redactFormBody(body []byte) (string, bool) {
	values, err := url.ParseQuery(string(body))
	if err != nil {
		return "", false
	}
	for key := range values {
		if isSensitiveFieldKey(key) {
			values[key] = []string{redactedValue}
		}
	}
	return values.Encode(), true
}

func redactPlainTextBody(body string) string {
	redacted := body
	for _, pattern := range plainTextRedactionPatterns {
		redacted = pattern.regex.ReplaceAllString(redacted, pattern.replacement)
	}
	return redacted
}

func truncateBody(body string) string {
	if len(body) <= maxBodyLogChars {
		return body
	}
	return fmt.Sprintf("%s... [truncated, total %d bytes]", body[:maxBodyLogChars], len(body))
}
