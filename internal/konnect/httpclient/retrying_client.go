package httpclient

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"math/rand/v2"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	kk "github.com/Kong/sdk-konnect-go"

	"github.com/kong/kongctl/internal/log"
)

const (
	logTypeRetry = "http_retry"
)

// idempotentMethods are the HTTP methods safe to retry on connection errors.
// IETF RFC 7231 §4.2 — safe and idempotent methods.
var idempotentMethods = []string{
	http.MethodDelete,
	http.MethodGet,
	http.MethodHead,
	http.MethodOptions,
	http.MethodPut,
}

// defaultRetryCodes are the HTTP status codes that trigger a retry by default.
// Matches the SDK's built-in retry status codes.
var defaultRetryCodes = []int{403, 429, 500, 502, 503, 504}

// RetryClientOption is a functional option for RetryingHTTPClient.
type RetryClientOption func(*RetryingHTTPClient)

// WithRetryableCodes overrides the set of HTTP status codes that trigger a retry.
func WithRetryableCodes(codes ...int) RetryClientOption {
	return func(c *RetryingHTTPClient) {
		c.retryCodes = codes
	}
}

// WithRetryableMethods restricts retries to specific HTTP methods. An empty
// slice (the default) means all methods are eligible for retry.
func WithRetryableMethods(methods ...string) RetryClientOption {
	return func(c *RetryingHTTPClient) {
		c.retryMethods = methods
	}
}

// RetryingHTTPClient wraps an inner kk.HTTPClient and transparently retries
// requests that fail with retryable HTTP status codes, using exponential
// backoff with jitter. It is intended to be placed in the HTTP client chain
// between the RefreshingHTTPClient and the SDK so that retries are handled
// at the transport layer rather than per-operation via SDK options.
type RetryingHTTPClient struct {
	inner          kk.HTTPClient
	cfg            RetryConfig
	retryCodes     []int
	retryMethods   []string
	logger         *slog.Logger
	requestCounter atomic.Uint64
}

// NewRetryingHTTPClient creates a new RetryingHTTPClient. When cfg.Strategy is
// RetryStrategyNone or cfg.MaxAttempts <= 1, Do simply delegates to inner
// without any retry logic.
func NewRetryingHTTPClient(
	inner kk.HTTPClient,
	cfg RetryConfig,
	logger *slog.Logger,
	opts ...RetryClientOption,
) *RetryingHTTPClient {
	c := &RetryingHTTPClient{
		inner:        inner,
		cfg:          cfg,
		retryCodes:   defaultRetryCodes,
		retryMethods: nil, // nil = all methods
		logger:       logger,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Do executes the request, retrying on retryable status codes with
// exponential backoff and jitter. Each retry is logged at debug level.
func (c *RetryingHTTPClient) Do(req *http.Request) (*http.Response, error) {
	if c == nil || c.inner == nil {
		return nil, fmt.Errorf("http client is not configured")
	}
	if req == nil {
		return nil, fmt.Errorf("request is nil")
	}

	if c.cfg.Strategy != RetryStrategyBackoff || c.cfg.MaxAttempts <= 1 {
		return c.inner.Do(req)
	}

	// Body must be replayable for retries. If it is non-nil but GetBody is
	// absent we can still attempt once — just skip retrying on failure.
	bodyReplayable := req.Body == nil || req.Body == http.NoBody || req.GetBody != nil

	requestID := fmt.Sprintf("kretry-%06d", c.requestCounter.Add(1))
	var (
		resp *http.Response
		err  error
	)

	for attempt := range c.cfg.MaxAttempts {
		// Replay body on retries.
		if attempt > 0 && req.Body != nil && req.Body != http.NoBody {
			body, getErr := req.GetBody()
			if getErr != nil {
				return nil, fmt.Errorf("replay request body for retry: %w", getErr)
			}
			req.Body = body
		}

		resp, err = c.inner.Do(req)

		isLast := attempt == c.cfg.MaxAttempts-1

		if err != nil {
			if isLast || !c.shouldRetryError(req, err) {
				return nil, err
			}
			next := c.nextInterval(attempt)
			c.logRetry(req, requestID, 0, attempt+1, next, err.Error())
			if waitErr := c.wait(req.Context(), next); waitErr != nil {
				return nil, waitErr
			}
			continue
		}

		if isLast || !c.shouldRetryResponse(req, resp) {
			return resp, nil
		}

		// Body must be replayable to attempt a retry. If it is not, return
		// the current response as-is without retrying.
		if !bodyReplayable {
			return resp, nil
		}

		// We will retry — consume and close the response body to free the
		// connection back to the pool.
		drainBody(resp)

		next := c.nextInterval(attempt)
		c.logRetry(req, requestID, resp.StatusCode, attempt+1, next, "")

		if waitErr := c.wait(req.Context(), next); waitErr != nil {
			return nil, waitErr
		}
	}

	return resp, err
}

// shouldRetryResponse reports whether a response with the given status code
// should be retried given the configured codes and methods.
func (c *RetryingHTTPClient) shouldRetryResponse(req *http.Request, resp *http.Response) bool {
	if !slices.Contains(c.retryCodes, resp.StatusCode) {
		return false
	}
	return c.methodAllowed(req.Method)
}

// shouldRetryError reports whether a transport-level error should be retried.
// Mirrors the SDK's connection-error retry logic (internal/utils/retries.go):
//   - url.Error with Temporary() or Timeout() → retry regardless of method
//   - url.Error wrapping io.EOF + idempotent method → retry (server closed conn)
//   - net.OpError with EPIPE or ECONNRESET + idempotent method → retry
//
// All other errors are treated as permanent when RetryConnectionErrors is false.
func (c *RetryingHTTPClient) shouldRetryError(req *http.Request, err error) bool {
	if !c.cfg.RetryConnectionErrors {
		return false
	}

	method := req.Method

	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		// Temporary or timeout errors are safe to retry on any method.
		if urlErr.Temporary() || urlErr.Timeout() {
			return true
		}

		// Fall back to the method embedded in the url.Error operation name
		// when the request method is unavailable (matches SDK behaviour).
		if method == "" {
			method = strings.ToUpper(urlErr.Op)
		}

		// Connection closed by the server mid-flight — safe to retry on
		// idempotent methods only.
		if errors.Is(urlErr.Err, io.EOF) && slices.Contains(idempotentMethods, method) {
			return true
		}

		return false
	}

	// Broken pipe or connection reset — safe to retry on idempotent methods.
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		if (errors.Is(err, syscall.EPIPE) || errors.Is(err, syscall.ECONNRESET)) &&
			slices.Contains(idempotentMethods, method) {
			return true
		}
	}

	return false
}

// methodAllowed reports whether the given HTTP method is eligible for retry.
// If retryMethods is empty, all methods are allowed.
func (c *RetryingHTTPClient) methodAllowed(method string) bool {
	if len(c.retryMethods) == 0 {
		return true
	}
	return slices.Contains(c.retryMethods, method)
}

// nextInterval computes the backoff wait duration for the given attempt index
// (0-based). The formula mirrors the SDK's nextInterval implementation:
//
//	interval = initialInterval × (attempt+1)^exponent  (capped at maxInterval)
//	+ jitter of ±25%
func (c *RetryingHTTPClient) nextInterval(attempt int) time.Duration {
	initial := float64(c.cfg.InitialIntervalMS) * float64(time.Millisecond)
	maximum := float64(c.cfg.MaxIntervalMS) * float64(time.Millisecond)
	exponent := c.cfg.BackoffFactor

	interval := initial * math.Pow(float64(attempt+1), exponent)
	interval = min(interval, maximum)

	jitter := rand.Float64() * 0.25 * interval //nolint:gosec // jitter does not need crypto-quality randomness
	if rand.IntN(2) == 0 {                     //nolint:gosec // jitter does not need crypto-quality randomness
		interval += jitter
	} else {
		interval -= jitter
	}
	interval = max(interval, 0)

	return time.Duration(interval)
}

// wait blocks for the given duration, returning early if ctx is cancelled.
func (c *RetryingHTTPClient) wait(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}

// logRetry emits a debug-level log entry for a retry event.
func (c *RetryingHTTPClient) logRetry(
	req *http.Request,
	requestID string,
	statusCode int,
	retryAttempt int,
	nextInterval time.Duration,
	errMsg string,
) {
	if c.logger == nil {
		return
	}
	ctx := req.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	if !c.logger.Enabled(ctx, slog.LevelDebug) {
		return
	}

	attrs := []slog.Attr{
		slog.String("log_type", logTypeRetry),
		slog.String("request_id", requestID),
		slog.String("method", req.Method),
		slog.Time("timestamp", time.Now()),
		slog.Int("retry_attempt", retryAttempt),
		slog.Int64("next_interval_ms", nextInterval.Milliseconds()),
	}
	attrs = append(attrs, log.HTTPLogContextAttrs(ctx)...)

	if req.URL != nil {
		attrs = append(attrs, slog.String("path", req.URL.Path))
		attrs = append(attrs, slog.String("host", req.URL.Host))
	}
	if statusCode > 0 {
		attrs = append(attrs, slog.Int("status_code", statusCode))
	}
	if errMsg != "" {
		attrs = append(attrs, slog.String("error", errMsg))
	}

	c.logger.LogAttrs(ctx, slog.LevelDebug, "retrying request", attrs...)
}

// drainBody reads and discards the response body so the underlying TCP
// connection can be returned to the pool, then closes it.
func drainBody(resp *http.Response) {
	if resp == nil || resp.Body == nil {
		return
	}
	// Ignore errors — we're discarding anyway.
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
}
