package httpclient

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockHTTPClient is a simple HTTP client that returns canned responses.
type mockHTTPClient struct {
	calls     int
	responses []*mockResponse
}

type mockResponse struct {
	statusCode int
	err        error
}

func (m *mockHTTPClient) Do(_ *http.Request) (*http.Response, error) {
	idx := m.calls
	m.calls++
	if idx >= len(m.responses) {
		idx = len(m.responses) - 1
	}
	r := m.responses[idx]
	if r.err != nil {
		return nil, r.err
	}
	return &http.Response{
		StatusCode: r.statusCode,
		Body:       io.NopCloser(strings.NewReader("")),
	}, nil
}

func defaultRetryConfig() RetryConfig {
	return RetryConfig{
		Strategy:              RetryStrategyBackoff,
		MaxAttempts:           3,
		InitialIntervalMS:     1, // very short for tests
		MaxIntervalMS:         5,
		BackoffFactor:         1.0,
		RetryConnectionErrors: false,
	}
}

func newTestRequest(t *testing.T) *http.Request {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com/path", nil)
	require.NoError(t, err)
	return req
}

func TestRetryingHTTPClient_StrategyNone(t *testing.T) {
	inner := &mockHTTPClient{
		responses: []*mockResponse{{statusCode: 500}},
	}
	cfg := defaultRetryConfig()
	cfg.Strategy = RetryStrategyNone

	c := NewRetryingHTTPClient(inner, cfg, nil)
	resp, err := c.Do(newTestRequest(t))
	require.NoError(t, err)
	assert.Equal(t, 500, resp.StatusCode)
	assert.Equal(t, 1, inner.calls)
}

func TestRetryingHTTPClient_MaxAttemptsOne(t *testing.T) {
	inner := &mockHTTPClient{
		responses: []*mockResponse{{statusCode: 503}},
	}
	cfg := defaultRetryConfig()
	cfg.MaxAttempts = 1

	c := NewRetryingHTTPClient(inner, cfg, nil)
	resp, err := c.Do(newTestRequest(t))
	require.NoError(t, err)
	assert.Equal(t, 503, resp.StatusCode)
	assert.Equal(t, 1, inner.calls)
}

func TestRetryingHTTPClient_NoRetryOnSuccess(t *testing.T) {
	inner := &mockHTTPClient{
		responses: []*mockResponse{{statusCode: 200}},
	}
	c := NewRetryingHTTPClient(inner, defaultRetryConfig(), nil)
	resp, err := c.Do(newTestRequest(t))
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, 1, inner.calls)
}

func TestRetryingHTTPClient_NoRetryOnNonRetryableCode(t *testing.T) {
	inner := &mockHTTPClient{
		responses: []*mockResponse{{statusCode: 404}},
	}
	c := NewRetryingHTTPClient(inner, defaultRetryConfig(), nil)
	resp, err := c.Do(newTestRequest(t))
	require.NoError(t, err)
	assert.Equal(t, 404, resp.StatusCode)
	assert.Equal(t, 1, inner.calls)
}

func TestRetryingHTTPClient_RetriesOnRetryableCodes(t *testing.T) {
	for _, code := range []int{429, 403, 500, 502, 503, 504} {
		t.Run(http.StatusText(code), func(t *testing.T) {
			inner := &mockHTTPClient{
				responses: []*mockResponse{
					{statusCode: code},
					{statusCode: code},
					{statusCode: 200},
				},
			}
			c := NewRetryingHTTPClient(inner, defaultRetryConfig(), nil)
			resp, err := c.Do(newTestRequest(t))
			require.NoError(t, err)
			assert.Equal(t, 200, resp.StatusCode)
			assert.Equal(t, 3, inner.calls)
		})
	}
}

func TestRetryingHTTPClient_ExhaustsRetries(t *testing.T) {
	cfg := defaultRetryConfig()
	cfg.MaxAttempts = 3
	inner := &mockHTTPClient{
		responses: []*mockResponse{
			{statusCode: 503},
			{statusCode: 503},
			{statusCode: 503},
		},
	}
	c := NewRetryingHTTPClient(inner, cfg, nil)
	resp, err := c.Do(newTestRequest(t))
	require.NoError(t, err)
	assert.Equal(t, 503, resp.StatusCode)
	assert.Equal(t, 3, inner.calls)
}

func TestRetryingHTTPClient_LogsRetryPolicyAttemptAndSuccess(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	inner := &mockHTTPClient{
		responses: []*mockResponse{
			{statusCode: 403},
			{statusCode: 200},
		},
	}

	c := NewRetryingHTTPClient(inner, defaultRetryConfig(), logger)
	resp, err := c.Do(newTestRequest(t))
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)

	out := logs.String()
	require.Contains(t, out, "log_type=http_retry")
	require.Contains(t, out, "http_source=sdk-konnect-go")
	require.Contains(t, out, "event=retry_policy")
	require.Contains(t, out, "event=retry_attempt")
	require.Contains(t, out, "event=retry_succeeded")
	require.Contains(t, out, "max_attempts=3")
	require.Contains(t, out, "initial_interval_ms=1")
	require.Contains(t, out, "status_code=403")
	require.Contains(t, out, "status_code=200")
	require.Contains(t, out, "body_replayable=true")
}

func TestRetryingHTTPClient_LogsRetryAttemptAndSuccessAtWarn(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelWarn}))
	inner := &mockHTTPClient{
		responses: []*mockResponse{
			{statusCode: 403},
			{statusCode: 200},
		},
	}

	c := NewRetryingHTTPClient(inner, defaultRetryConfig(), logger)
	resp, err := c.Do(newTestRequest(t))
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)

	out := logs.String()
	require.Contains(t, out, "level=WARN")
	require.Contains(t, out, "log_type=http_retry")
	require.Contains(t, out, "event=retry_attempt")
	require.Contains(t, out, "event=retry_succeeded")
	require.Contains(t, out, "status_code=403")
	require.Contains(t, out, "status_code=200")
	require.Contains(t, out, "max_attempts=3")
	require.Contains(t, out, "initial_interval_ms=1")
	require.Contains(t, out, "max_interval_ms=5")
	require.Contains(t, out, "backoff_factor=1")
	require.NotContains(t, out, "event=retry_policy")
}

func TestRetryingHTTPClient_LogsRetryExhaustedAtWarn(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelWarn}))
	cfg := defaultRetryConfig()
	cfg.MaxAttempts = 2
	inner := &mockHTTPClient{
		responses: []*mockResponse{
			{statusCode: 403},
			{statusCode: 403},
		},
	}

	c := NewRetryingHTTPClient(inner, cfg, logger)
	resp, err := c.Do(newTestRequest(t))
	require.NoError(t, err)
	require.Equal(t, 403, resp.StatusCode)

	out := logs.String()
	require.Contains(t, out, "level=WARN")
	require.Contains(t, out, "log_type=http_retry")
	require.Contains(t, out, "event=retry_exhausted")
	require.Contains(t, out, "status_code=403")
	require.Contains(t, out, "attempt=2")
	require.Contains(t, out, "max_attempts=2")
	require.Contains(t, out, "initial_interval_ms=1")
	require.Contains(t, out, "max_interval_ms=5")
	require.Contains(t, out, "backoff_factor=1")
	require.Contains(t, out, "retry_connection_errors=false")
	require.Contains(t, out, "event=retry_attempt")
}

func TestRetryingHTTPClient_BodyReplay(t *testing.T) {
	bodyContent := "request body"
	inner := &mockHTTPClient{
		responses: []*mockResponse{
			{statusCode: 503},
			{statusCode: 200},
		},
	}

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"http://example.com/path",
		strings.NewReader(bodyContent),
	)
	require.NoError(t, err)
	// Set GetBody so the body is replayable.
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader(bodyContent)), nil
	}

	c := NewRetryingHTTPClient(inner, defaultRetryConfig(), nil)
	resp, err := c.Do(req)
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, 2, inner.calls)
}

func TestRetryingHTTPClient_NonReplayableBodySkipsRetry(t *testing.T) {
	inner := &mockHTTPClient{
		responses: []*mockResponse{
			{statusCode: 503},
			{statusCode: 200},
		},
	}

	// Use an opaque io.ReadCloser so http.NewRequestWithContext cannot detect
	// the underlying *strings.Reader and will NOT set GetBody automatically.
	body := struct{ io.Reader }{strings.NewReader("body")}
	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"http://example.com/path",
		body,
	)
	require.NoError(t, err)
	// Confirm GetBody is nil — body is not replayable.
	require.Nil(t, req.GetBody)

	c := NewRetryingHTTPClient(inner, defaultRetryConfig(), nil)
	resp, err := c.Do(req)
	require.NoError(t, err)
	// Should get the 503 back without retrying.
	assert.Equal(t, 503, resp.StatusCode)
	assert.Equal(t, 1, inner.calls)
}

func TestRetryingHTTPClient_ContextCancellation(t *testing.T) {
	cfg := defaultRetryConfig()
	// Use a long wait to ensure the ctx cancel fires first.
	cfg.InitialIntervalMS = 1_000
	cfg.MaxIntervalMS = 60_000

	inner := &mockHTTPClient{
		responses: []*mockResponse{
			{statusCode: 503},
			{statusCode: 503},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://example.com/path", nil)
	require.NoError(t, err)

	c := NewRetryingHTTPClient(inner, cfg, nil)
	_, err = c.Do(req)
	require.Error(t, err)
	assert.True(t, errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled))
}

func TestRetryingHTTPClient_CustomRetryCodes(t *testing.T) {
	inner := &mockHTTPClient{
		responses: []*mockResponse{
			{statusCode: 418},
			{statusCode: 200},
		},
	}
	c := NewRetryingHTTPClient(inner, defaultRetryConfig(), nil, WithRetryableCodes(418))
	resp, err := c.Do(newTestRequest(t))
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, 2, inner.calls)
}

func TestRetryingHTTPClient_CustomRetryMethods(t *testing.T) {
	// Only GET should retry; POST should not.
	inner := &mockHTTPClient{
		responses: []*mockResponse{
			{statusCode: 500},
			{statusCode: 200},
		},
	}
	c := NewRetryingHTTPClient(inner, defaultRetryConfig(), nil, WithRetryableMethods(http.MethodGet))

	// POST should NOT retry.
	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"http://example.com/path",
		nil,
	)
	require.NoError(t, err)
	resp, err := c.Do(req)
	require.NoError(t, err)
	assert.Equal(t, 500, resp.StatusCode)
	assert.Equal(t, 1, inner.calls)
}

func TestRetryingHTTPClient_NilRequest(t *testing.T) {
	inner := &mockHTTPClient{}
	c := NewRetryingHTTPClient(inner, defaultRetryConfig(), nil)
	_, err := c.Do(nil)
	require.Error(t, err)
}

func TestRetryingHTTPClient_ConnectionErrorRetryDisabled(t *testing.T) {
	urlErr := &url.Error{Op: "Get", URL: "http://example.com", Err: &mockNetError{temporary: true}}
	inner := &mockHTTPClient{
		responses: []*mockResponse{
			{err: urlErr},
		},
	}
	cfg := defaultRetryConfig()
	cfg.RetryConnectionErrors = false

	c := NewRetryingHTTPClient(inner, cfg, nil)
	_, err := c.Do(newTestRequest(t))
	require.Error(t, err)
	assert.Equal(t, 1, inner.calls)
}

func TestRetryingHTTPClient_URLError_Temporary_IdempotentRetries(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodPut, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			urlErr := &url.Error{Op: method, URL: "http://example.com", Err: &mockNetError{temporary: true}}
			inner := &mockHTTPClient{
				responses: []*mockResponse{
					{err: urlErr},
					{statusCode: 200},
				},
			}
			cfg := defaultRetryConfig()
			cfg.RetryConnectionErrors = true

			req, err := http.NewRequestWithContext(
				context.Background(), method, "http://example.com/path", nil)
			require.NoError(t, err)

			c := NewRetryingHTTPClient(inner, cfg, nil)
			resp, err := c.Do(req)
			require.NoError(t, err)
			assert.Equal(t, 200, resp.StatusCode)
			assert.Equal(t, 2, inner.calls, "method %s should have retried", method)
		})
	}
}

func TestRetryingHTTPClient_URLError_Temporary_NonIdempotentNoRetry(t *testing.T) {
	urlErr := &url.Error{Op: "Post", URL: "http://example.com", Err: &mockNetError{temporary: true}}
	inner := &mockHTTPClient{
		responses: []*mockResponse{
			{err: urlErr},
		},
	}
	cfg := defaultRetryConfig()
	cfg.RetryConnectionErrors = true

	req, err := http.NewRequestWithContext(
		context.Background(), http.MethodPost, "http://example.com/path", nil)
	require.NoError(t, err)

	c := NewRetryingHTTPClient(inner, cfg, nil)
	_, err = c.Do(req)
	require.Error(t, err)
	assert.Equal(t, 1, inner.calls)
}

func TestRetryingHTTPClient_URLError_Timeout_IdempotentRetries(t *testing.T) {
	urlErr := &url.Error{Op: "Get", URL: "http://example.com", Err: &mockNetError{timeout: true}}
	inner := &mockHTTPClient{
		responses: []*mockResponse{
			{err: urlErr},
			{statusCode: 200},
		},
	}
	cfg := defaultRetryConfig()
	cfg.RetryConnectionErrors = true

	req, err := http.NewRequestWithContext(
		context.Background(), http.MethodGet, "http://example.com/path", nil)
	require.NoError(t, err)

	c := NewRetryingHTTPClient(inner, cfg, nil)
	resp, err := c.Do(req)
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, 2, inner.calls)
}

func TestRetryingHTTPClient_URLError_Timeout_NonIdempotentNoRetry(t *testing.T) {
	urlErr := &url.Error{Op: "Post", URL: "http://example.com", Err: &mockNetError{timeout: true}}
	inner := &mockHTTPClient{
		responses: []*mockResponse{
			{err: urlErr},
		},
	}
	cfg := defaultRetryConfig()
	cfg.RetryConnectionErrors = true

	req, err := http.NewRequestWithContext(
		context.Background(), http.MethodPost, "http://example.com/path", nil)
	require.NoError(t, err)

	c := NewRetryingHTTPClient(inner, cfg, nil)
	_, err = c.Do(req)
	require.Error(t, err)
	assert.Equal(t, 1, inner.calls)
}

func TestRetryingHTTPClient_URLError_EOF_IdempotentRetries(t *testing.T) {
	urlErr := &url.Error{Op: "Get", URL: "http://example.com", Err: io.EOF}
	inner := &mockHTTPClient{
		responses: []*mockResponse{
			{err: urlErr},
			{statusCode: 200},
		},
	}
	cfg := defaultRetryConfig()
	cfg.RetryConnectionErrors = true

	c := NewRetryingHTTPClient(inner, cfg, nil)
	resp, err := c.Do(newTestRequest(t)) // GET is idempotent
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, 2, inner.calls)
}

func TestRetryingHTTPClient_URLError_EOF_NonIdempotentNoRetry(t *testing.T) {
	urlErr := &url.Error{Op: "Post", URL: "http://example.com", Err: io.EOF}
	inner := &mockHTTPClient{
		responses: []*mockResponse{
			{err: urlErr},
		},
	}
	cfg := defaultRetryConfig()
	cfg.RetryConnectionErrors = true

	req, err := http.NewRequestWithContext(
		context.Background(), http.MethodPost, "http://example.com/path", nil)
	require.NoError(t, err)

	c := NewRetryingHTTPClient(inner, cfg, nil)
	_, err = c.Do(req)
	require.Error(t, err)
	assert.Equal(t, 1, inner.calls)
}

func TestRetryingHTTPClient_ECONNRESET_IdempotentRetries(t *testing.T) {
	opErr := &net.OpError{Op: "read", Err: syscall.ECONNRESET}
	inner := &mockHTTPClient{
		responses: []*mockResponse{
			{err: opErr},
			{statusCode: 200},
		},
	}
	cfg := defaultRetryConfig()
	cfg.RetryConnectionErrors = true

	c := NewRetryingHTTPClient(inner, cfg, nil)
	resp, err := c.Do(newTestRequest(t)) // GET is idempotent
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, 2, inner.calls)
}

func TestRetryingHTTPClient_EPIPE_IdempotentRetries(t *testing.T) {
	opErr := &net.OpError{Op: "write", Err: syscall.EPIPE}
	inner := &mockHTTPClient{
		responses: []*mockResponse{
			{err: opErr},
			{statusCode: 200},
		},
	}
	cfg := defaultRetryConfig()
	cfg.RetryConnectionErrors = true

	c := NewRetryingHTTPClient(inner, cfg, nil)
	resp, err := c.Do(newTestRequest(t))
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, 2, inner.calls)
}

func TestRetryingHTTPClient_ECONNRESET_NonIdempotentNoRetry(t *testing.T) {
	opErr := &net.OpError{Op: "read", Err: syscall.ECONNRESET}
	inner := &mockHTTPClient{
		responses: []*mockResponse{
			{err: opErr},
		},
	}
	cfg := defaultRetryConfig()
	cfg.RetryConnectionErrors = true

	req, err := http.NewRequestWithContext(
		context.Background(), http.MethodPost, "http://example.com/path", nil)
	require.NoError(t, err)

	c := NewRetryingHTTPClient(inner, cfg, nil)
	_, err = c.Do(req)
	require.Error(t, err)
	assert.Equal(t, 1, inner.calls)
}

func TestNextInterval(t *testing.T) {
	cfg := RetryConfig{
		InitialIntervalMS: 1_000,
		MaxIntervalMS:     60_000,
		BackoffFactor:     2.0,
	}
	c := &RetryingHTTPClient{cfg: cfg}

	// Verify exponential doubling: each attempt's midpoint should double.
	// With factor=2 and initial=1s: attempt 0 → ~1s, attempt 1 → ~2s, attempt 2 → ~4s.
	// We check within ±25% jitter tolerance.
	expected := []float64{1_000, 2_000, 4_000, 8_000, 16_000}
	for attempt, wantMS := range expected {
		d := c.nextInterval(attempt)
		low := time.Duration(wantMS*0.75) * time.Millisecond
		high := time.Duration(wantMS*1.25) * time.Millisecond
		assert.GreaterOrEqual(t, d, low, "attempt %d interval too low", attempt)
		assert.LessOrEqual(t, d, high, "attempt %d interval too high", attempt)
	}

	// Verify cap: at a high attempt number jitter must not push the interval beyond max.
	maxInterval := time.Duration(cfg.MaxIntervalMS) * time.Millisecond
	for range 100 {
		assert.LessOrEqual(t, c.nextInterval(100), maxInterval)
	}
}

// mockNetError implements net.Error for testing.
type mockNetError struct {
	temporary bool
	timeout   bool
}

func (e *mockNetError) Error() string   { return "mock net error" }
func (e *mockNetError) Timeout() bool   { return e.timeout }
func (e *mockNetError) Temporary() bool { return e.temporary }
