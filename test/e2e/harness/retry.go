//go:build e2e

package harness

import (
	"context"
	"errors"
	"math/rand"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Default retry/backoff settings shared across the e2e harness.
const (
	DefaultRetryAttempts      = 6
	DefaultRetryInterval      = 1500 * time.Millisecond
	DefaultRetryMaxInterval   = 10 * time.Second
	DefaultRetryBackoffFactor = 2.0
	DefaultRetryJitter        = 500 * time.Millisecond
	// DefaultTimeoutRetryThreshold is the ratio of actual duration to configured
	// timeout above which a timed-out command will not be retried. A value of 0.9
	// means that if a command consumed >=90% of the timeout before being killed,
	// retrying is unlikely to help and the attempt is skipped.
	DefaultTimeoutRetryThreshold = 0.9
)

var defaultRetryablePatterns = []string{
	"dial tcp",
	"connection reset",
	"connection refused",
	"client.timeout exceeded",
	"context deadline exceeded",
	"tls handshake timeout",
	"i/o timeout",
	"temporary failure in name resolution",
	"no such host",
	"eof",
	"too many requests",
	"rate limit",
	"status=429",
	"429",
	"503 service unavailable",
	"504 gateway timeout",
	"gateway timeout",
	"bad gateway",
	"retry-after",
}

// BackoffConfig captures retry/backoff parameters for reusable helpers.
type BackoffConfig struct {
	Attempts int
	Base     time.Duration
	Max      time.Duration
	Factor   float64
	Jitter   time.Duration
}

// NormalizeBackoffConfig applies harness defaults where values are missing or invalid.
func NormalizeBackoffConfig(cfg BackoffConfig) BackoffConfig {
	if cfg.Attempts < 1 {
		cfg.Attempts = DefaultRetryAttempts
	}
	if cfg.Base <= 0 {
		cfg.Base = DefaultRetryInterval
	}
	if cfg.Max <= 0 {
		cfg.Max = DefaultRetryMaxInterval
	}
	if cfg.Factor <= 0 {
		cfg.Factor = DefaultRetryBackoffFactor
	}
	if cfg.Jitter < 0 {
		cfg.Jitter = 0
	}
	return cfg
}

// BuildBackoffSchedule returns a slice of delays (length attempts-1) respecting the config.
func BuildBackoffSchedule(cfg BackoffConfig) []time.Duration {
	cfg = NormalizeBackoffConfig(cfg)
	if cfg.Attempts <= 1 {
		return nil
	}
	schedule := make([]time.Duration, cfg.Attempts-1)
	next := cfg.Base
	for i := 0; i < cfg.Attempts-1; i++ {
		delay := next
		if cfg.Jitter > 0 {
			delay += jitterDuration(cfg.Jitter)
		}
		if cfg.Max > 0 && delay > cfg.Max {
			delay = cfg.Max
		}
		schedule[i] = delay
		next = time.Duration(float64(next) * cfg.Factor)
		if next <= 0 {
			next = cfg.Base
		}
	}
	return schedule
}

// BackoffDelay returns the delay for a given attempt index (0-based). Returns 0 if none left.
func BackoffDelay(schedule []time.Duration, attempt int) time.Duration {
	if attempt < len(schedule) {
		return schedule[attempt]
	}
	return 0
}

// ShouldRetry determines whether an error/detail warrants another attempt under the given policy.
// result and timeout are used to suppress retries when a subprocess consumed nearly the entire
// timeout (indicating a hang rather than a transient failure). Pass a zero Result and 0 timeout
// for non-subprocess callers (e.g. HTTP helpers in reset.go).
func ShouldRetry(err error, detail string, only, never []string, result Result, timeout time.Duration) bool {
	if err == nil {
		return false
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr != nil {
		return true
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	detailLower := strings.ToLower(detail)
	for _, pat := range never {
		if strings.Contains(detailLower, strings.ToLower(pat)) {
			return false
		}
	}

	if len(only) > 0 {
		for _, pat := range only {
			if strings.Contains(detailLower, strings.ToLower(pat)) {
				return true
			}
		}
		return false
	}

	// If the command timed out and consumed nearly the full timeout, retrying is
	// unlikely to help — skip rather than waste another full timeout period.
	if result.TimedOut && timeout > 0 {
		ratio := float64(result.Duration) / float64(timeout)
		if ratio >= DefaultTimeoutRetryThreshold {
			return false
		}
	}

	var cmdErr *CommandError
	if errors.As(err, &cmdErr) && cmdErr != nil && cmdErr.Result.ExitCode == -1 {
		return true
	}

	for _, pat := range defaultRetryablePatterns {
		if strings.Contains(detailLower, pat) {
			return true
		}
	}

	return false
}

type RetryClass string

const (
	RetryClassNone      RetryClass = "none"
	RetryClassThrottle  RetryClass = "throttle"
	RetryClassTimeout   RetryClass = "timeout"
	RetryClassNetwork   RetryClass = "network"
	RetryClassTransient RetryClass = "transient"
)

func ClassifyRetry(err error, detail string) RetryClass {
	if err == nil {
		return RetryClassNone
	}
	if _, ok := RetryAfterDelay(err); ok {
		return RetryClassThrottle
	}

	var httpErr *httpError
	if errors.As(err, &httpErr) && httpErr != nil {
		switch httpErr.status {
		case http.StatusTooManyRequests, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
			return RetryClassThrottle
		case http.StatusBadGateway:
			return RetryClassTransient
		}
	}

	detailLower := strings.ToLower(detail)
	switch {
	case strings.Contains(detailLower, "too many requests"),
		strings.Contains(detailLower, "rate limit"),
		strings.Contains(detailLower, "status=429"),
		strings.Contains(detailLower, "503 service unavailable"),
		strings.Contains(detailLower, "504 gateway timeout"),
		strings.Contains(detailLower, "retry-after"):
		return RetryClassThrottle
	case IsTimeoutRetry(err, detailLower):
		return RetryClassTimeout
	case strings.Contains(detailLower, "dial tcp"),
		strings.Contains(detailLower, "connection reset"),
		strings.Contains(detailLower, "connection refused"),
		strings.Contains(detailLower, "temporary failure in name resolution"),
		strings.Contains(detailLower, "no such host"),
		strings.Contains(detailLower, "eof"):
		return RetryClassNetwork
	}

	if ShouldRetry(err, detail, nil, nil, Result{}, 0) {
		return RetryClassTransient
	}
	return RetryClassNone
}

func IsTimeoutRetry(err error, detail string) bool {
	if err == nil {
		return false
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr != nil && netErr.Timeout() {
		return true
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	detailLower := strings.ToLower(detail)
	return strings.Contains(detailLower, "client.timeout exceeded") ||
		strings.Contains(detailLower, "context deadline exceeded") ||
		strings.Contains(detailLower, "tls handshake timeout") ||
		strings.Contains(detailLower, "i/o timeout")
}

func RetryAfterDelay(err error) (time.Duration, bool) {
	var httpErr *httpError
	if !errors.As(err, &httpErr) || httpErr == nil || httpErr.header == nil {
		return 0, false
	}
	value := strings.TrimSpace(httpErr.header.Get("Retry-After"))
	if value == "" {
		return 0, false
	}
	if seconds, parseErr := strconv.Atoi(value); parseErr == nil {
		if seconds < 0 {
			return 0, false
		}
		return time.Duration(seconds) * time.Second, true
	}
	when, parseErr := http.ParseTime(value)
	if parseErr != nil {
		return 0, false
	}
	delay := time.Until(when)
	if delay < 0 {
		delay = 0
	}
	return delay, true
}

func RetryDelayForError(err error, schedule []time.Duration, attempt int) time.Duration {
	delay := BackoffDelay(schedule, attempt)
	retryAfter, ok := RetryAfterDelay(err)
	if ok && retryAfter > delay {
		return retryAfter
	}
	return delay
}

// ShouldRetryHTTPAttempt applies the generic retry patterns, but treats repeated
// full request timeouts differently from CLI subprocess timeouts. A single
// near-full request timeout is allowed to retry once; after that, the harness
// fails fast instead of spending several more full timeout windows retrying.
func ShouldRetryHTTPAttempt(
	err error,
	detail string,
	timeout time.Duration,
	duration time.Duration,
	only []string,
	never []string,
	priorFullTimeouts int,
) bool {
	if !ShouldRetry(err, detail, only, never, Result{}, 0) {
		return false
	}
	if !IsTimeoutRetry(err, detail) || timeout <= 0 {
		return true
	}
	if priorFullTimeouts < 1 {
		return true
	}
	ratio := float64(duration) / float64(timeout)
	return ratio < DefaultTimeoutRetryThreshold
}

func jitterDuration(max time.Duration) time.Duration {
	if max <= 0 {
		return 0
	}
	n := rand.New(rand.NewSource(time.Now().UnixNano())).Int63n(int64(max))
	if n < 0 {
		return 0
	}
	return time.Duration(n)
}
