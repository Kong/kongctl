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
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	kk "github.com/Kong/sdk-konnect-go"

	"github.com/kong/kongctl/internal/log"
)

const (
	logTypeRetry          = "http_retry"
	retryEventPolicy      = "retry_policy"
	retryEventAttempt     = "retry_attempt"
	retryEventSucceeded   = "retry_succeeded"
	retryEventExhausted   = "retry_exhausted"
	retryEventSkipped     = "retry_skipped"
	retryEventInterrupted = "retry_interrupted"
	retryEventDisabled    = "retry_disabled"
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
// exponential backoff and jitter. Retry attempts and outcomes are logged at
// warn level.
func (c *RetryingHTTPClient) Do(req *http.Request) (*http.Response, error) {
	if c == nil || c.inner == nil {
		return nil, fmt.Errorf("http client is not configured")
	}
	if req == nil {
		return nil, fmt.Errorf("request is nil")
	}

	if c.cfg.Strategy != RetryStrategyBackoff || c.cfg.MaxAttempts <= 1 {
		c.logRetryDisabled(req, "retry strategy disabled or max attempts <= 1")
		return c.inner.Do(req)
	}

	// Body must be replayable for retries. If it is non-nil but GetBody is
	// absent we can still attempt once — just skip retrying on failure.
	bodyReplayable := req.Body == nil || req.Body == http.NoBody || req.GetBody != nil

	requestID := fmt.Sprintf("kretry-%06d", c.requestCounter.Add(1))
	start := time.Now()
	c.logRetryPolicy(req, requestID, bodyReplayable)

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
			if !c.shouldRetryError(req, err) {
				if attempt > 0 {
					c.logRetrySkipped(req, requestID, attempt+1, 0, "error is not retryable", time.Since(start), err.Error())
				}
				return nil, err
			}
			// If the body cannot be replayed we cannot safely retry the error.
			if !bodyReplayable {
				c.logRetrySkipped(req, requestID, attempt+1, 0, "request body is not replayable", time.Since(start), err.Error())
				return nil, err
			}
			if isLast {
				c.logRetryExhausted(req, requestID, attempt+1, 0, time.Since(start), err.Error())
				return nil, err
			}
			next := c.nextInterval(attempt)
			c.logRetryAttempt(req, requestID, 0, attempt+1, next, err.Error())
			if waitErr := c.wait(req.Context(), next); waitErr != nil {
				c.logRetryInterrupted(req, requestID, attempt+1, next, time.Since(start), waitErr.Error())
				return nil, waitErr
			}
			continue
		}

		if !c.shouldRetryResponse(req, resp) {
			if attempt > 0 {
				c.logRetrySucceeded(req, requestID, resp, attempt+1, time.Since(start))
			}
			return resp, nil
		}

		// Body must be replayable to attempt a retry. If it is not, return
		// the current response as-is without retrying.
		if !bodyReplayable {
			c.logRetrySkipped(
				req, requestID, attempt+1, resp.StatusCode, "request body is not replayable", time.Since(start), "",
			)
			return resp, nil
		}
		if isLast {
			c.logRetryExhausted(req, requestID, attempt+1, resp.StatusCode, time.Since(start), "")
			return resp, nil
		}

		// We will retry — consume and close the response body to free the
		// connection back to the pool.
		drainBody(resp)

		next := c.nextIntervalForResponse(resp, attempt)
		c.logRetryAttempt(req, requestID, resp.StatusCode, attempt+1, next, "")

		if waitErr := c.wait(req.Context(), next); waitErr != nil {
			c.logRetryInterrupted(req, requestID, attempt+1, next, time.Since(start), waitErr.Error())
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
//   - url.Error with Temporary() or Timeout() + idempotent method → retry
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
		// Fall back to the method embedded in the url.Error operation name
		if method == "" {
			method = strings.ToUpper(urlErr.Op)
		}

		// Temporary or timeout errors are safe to retry on idempotent methods.
		if (urlErr.Temporary() || urlErr.Timeout()) && slices.Contains(idempotentMethods, method) {
			return true
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
// (0-based). Uses standard exponential backoff:
//
//	interval = initialInterval × factor^attempt
//	+ jitter of ±25%
//	capped at maxInterval
//
// With factor=2 and initial=1s: attempt 0 → 1s, attempt 1 → 2s, attempt 2 → 4s.
func (c *RetryingHTTPClient) nextInterval(attempt int) time.Duration {
	initial := float64(c.cfg.InitialIntervalMS) * float64(time.Millisecond)
	maximum := float64(c.cfg.MaxIntervalMS) * float64(time.Millisecond)
	factor := c.cfg.BackoffFactor

	interval := initial * math.Pow(factor, float64(attempt))
	interval = min(interval, maximum)

	jitter := rand.Float64() * 0.25 * interval //nolint:gosec // jitter does not need crypto-quality randomness
	if rand.IntN(2) == 0 {                     //nolint:gosec // jitter does not need crypto-quality randomness
		interval += jitter
	} else {
		interval -= jitter
	}
	interval = min(max(interval, 0), maximum)

	return time.Duration(interval)
}

func (c *RetryingHTTPClient) nextIntervalForResponse(resp *http.Response, attempt int) time.Duration {
	if retryAfter, ok := c.retryAfterInterval(resp); ok {
		return retryAfter
	}
	return c.nextInterval(attempt)
}

func (c *RetryingHTTPClient) retryAfterInterval(resp *http.Response) (time.Duration, bool) {
	if resp == nil {
		return 0, false
	}

	value := strings.TrimSpace(resp.Header.Get("Retry-After"))
	if value == "" {
		return 0, false
	}

	if seconds, err := strconv.Atoi(value); err == nil {
		if seconds <= 0 {
			return 0, false
		}
		return c.clampRetryAfter(time.Duration(seconds) * time.Second), true
	}

	when, err := http.ParseTime(value)
	if err != nil {
		return 0, false
	}

	delay := time.Until(when)
	if delay <= 0 {
		return 0, false
	}
	return c.clampRetryAfter(delay), true
}

func (c *RetryingHTTPClient) clampRetryAfter(delay time.Duration) time.Duration {
	minimum := time.Duration(c.cfg.InitialIntervalMS) * time.Millisecond
	maximum := time.Duration(c.cfg.MaxIntervalMS) * time.Millisecond

	delay = max(delay, minimum)
	return min(delay, maximum)
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

func (c *RetryingHTTPClient) retryLogContext(req *http.Request) context.Context {
	ctx := req.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	return ctx
}

func (c *RetryingHTTPClient) retryBaseAttrs(req *http.Request, requestID, event string) []slog.Attr {
	ctx := c.retryLogContext(req)
	attrs := []slog.Attr{
		slog.String("log_type", logTypeRetry),
		slog.String("http_source", httpSource),
		slog.String("event", event),
		slog.String("request_id", requestID),
		slog.String("method", req.Method),
		slog.String("route", routeFromURL(req.URL)),
	}
	attrs = append(attrs, log.HTTPLogContextAttrs(ctx)...)

	if req.URL != nil {
		attrs = append(attrs, slog.String("host", req.URL.Host))
	}

	return attrs
}

func (c *RetryingHTTPClient) retryConfigAttrs() []slog.Attr {
	methods := any("all")
	if len(c.retryMethods) > 0 {
		methods = c.retryMethods
	}

	return []slog.Attr{
		slog.String("strategy", c.cfg.Strategy),
		slog.Int("max_attempts", c.cfg.MaxAttempts),
		slog.Int("initial_interval_ms", c.cfg.InitialIntervalMS),
		slog.Int("max_interval_ms", c.cfg.MaxIntervalMS),
		slog.Float64("backoff_factor", c.cfg.BackoffFactor),
		slog.Bool("retry_connection_errors", c.cfg.RetryConnectionErrors),
		slog.Any("retry_status_codes", c.retryCodes),
		slog.Any("retry_methods", methods),
	}
}

func (c *RetryingHTTPClient) logRetryPolicy(req *http.Request, requestID string, bodyReplayable bool) {
	if c.logger == nil {
		return
	}
	ctx := c.retryLogContext(req)
	if !c.logger.Enabled(ctx, slog.LevelDebug) {
		return
	}

	attrs := c.retryBaseAttrs(req, requestID, retryEventPolicy)
	attrs = append(attrs, c.retryConfigAttrs()...)
	attrs = append(attrs, slog.Bool("body_replayable", bodyReplayable))

	c.logger.LogAttrs(ctx, slog.LevelDebug, "http retry policy active", attrs...)
}

func (c *RetryingHTTPClient) logRetryDisabled(req *http.Request, reason string) {
	if c.logger == nil {
		return
	}
	ctx := c.retryLogContext(req)
	if !c.logger.Enabled(ctx, slog.LevelDebug) {
		return
	}

	requestID := fmt.Sprintf("kretry-%06d", c.requestCounter.Add(1))
	attrs := c.retryBaseAttrs(req, requestID, retryEventDisabled)
	attrs = append(attrs, c.retryConfigAttrs()...)
	attrs = append(attrs, slog.String("reason", reason))

	c.logger.LogAttrs(ctx, slog.LevelDebug, "http retry disabled", attrs...)
}

// logRetryAttempt emits a debug-level log entry before sleeping for another attempt.
func (c *RetryingHTTPClient) logRetryAttempt(
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
	ctx := c.retryLogContext(req)
	if !c.logger.Enabled(ctx, slog.LevelWarn) {
		return
	}

	attrs := c.retryBaseAttrs(req, requestID, retryEventAttempt)
	attrs = append(
		attrs,
		slog.Int("attempt", retryAttempt),
		slog.Int("next_attempt", retryAttempt+1),
		slog.Int("attempts_remaining", c.cfg.MaxAttempts-retryAttempt),
		slog.Int64("next_interval_ms", nextInterval.Milliseconds()),
	)
	attrs = append(attrs, c.retryConfigAttrs()...)
	if statusCode > 0 {
		attrs = append(attrs, slog.Int("status_code", statusCode))
	}
	if errMsg != "" {
		attrs = append(attrs, slog.String("error", errMsg))
	}

	c.logger.LogAttrs(ctx, slog.LevelWarn, "retrying request", attrs...)
}

func (c *RetryingHTTPClient) logRetrySucceeded(
	req *http.Request,
	requestID string,
	resp *http.Response,
	attempt int,
	duration time.Duration,
) {
	if c.logger == nil {
		return
	}
	ctx := c.retryLogContext(req)
	if !c.logger.Enabled(ctx, slog.LevelWarn) {
		return
	}

	attrs := c.retryBaseAttrs(req, requestID, retryEventSucceeded)
	attrs = append(
		attrs,
		slog.Int("attempt", attempt),
		slog.Int("max_attempts", c.cfg.MaxAttempts),
		slog.Duration("duration", duration),
	)
	if resp != nil {
		attrs = append(attrs, slog.Int("status_code", resp.StatusCode))
	}

	c.logger.LogAttrs(ctx, slog.LevelWarn, "request succeeded after retry", attrs...)
}

func (c *RetryingHTTPClient) logRetrySkipped(
	req *http.Request,
	requestID string,
	attempt int,
	statusCode int,
	reason string,
	duration time.Duration,
	errMsg string,
) {
	if c.logger == nil {
		return
	}
	ctx := c.retryLogContext(req)
	if !c.logger.Enabled(ctx, slog.LevelWarn) {
		return
	}

	attrs := c.retryBaseAttrs(req, requestID, retryEventSkipped)
	attrs = append(
		attrs,
		slog.Int("attempt", attempt),
		slog.Int("max_attempts", c.cfg.MaxAttempts),
		slog.Duration("duration", duration),
		slog.String("reason", reason),
	)
	if statusCode > 0 {
		attrs = append(attrs, slog.Int("status_code", statusCode))
	}
	if errMsg != "" {
		attrs = append(attrs, slog.String("error", errMsg))
	}

	c.logger.LogAttrs(ctx, slog.LevelWarn, "retry skipped", attrs...)
}

func (c *RetryingHTTPClient) logRetryExhausted(
	req *http.Request,
	requestID string,
	attempt int,
	statusCode int,
	duration time.Duration,
	errMsg string,
) {
	if c.logger == nil {
		return
	}
	ctx := c.retryLogContext(req)
	if !c.logger.Enabled(ctx, slog.LevelWarn) {
		return
	}

	attrs := c.retryBaseAttrs(req, requestID, retryEventExhausted)
	attrs = append(
		attrs,
		slog.Int("attempt", attempt),
		slog.Duration("duration", duration),
	)
	attrs = append(attrs, c.retryConfigAttrs()...)
	if statusCode > 0 {
		attrs = append(attrs, slog.Int("status_code", statusCode))
	}
	if errMsg != "" {
		attrs = append(attrs, slog.String("error", errMsg))
	}

	c.logger.LogAttrs(ctx, slog.LevelWarn, "http retries exhausted", attrs...)
}

func (c *RetryingHTTPClient) logRetryInterrupted(
	req *http.Request,
	requestID string,
	attempt int,
	nextInterval time.Duration,
	duration time.Duration,
	errMsg string,
) {
	if c.logger == nil {
		return
	}
	ctx := c.retryLogContext(req)
	if !c.logger.Enabled(ctx, slog.LevelWarn) {
		return
	}

	attrs := c.retryBaseAttrs(req, requestID, retryEventInterrupted)
	attrs = append(
		attrs,
		slog.Int("attempt", attempt),
		slog.Int64("next_interval_ms", nextInterval.Milliseconds()),
		slog.Duration("duration", duration),
		slog.String("error", errMsg),
	)
	attrs = append(attrs, c.retryConfigAttrs()...)

	c.logger.LogAttrs(ctx, slog.LevelWarn, "http retry interrupted", attrs...)
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
