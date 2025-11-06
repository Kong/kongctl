//go:build e2e

package harness

import (
	"context"
	"errors"
	"math/rand"
	"net"
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
func ShouldRetry(err error, detail string, only, never []string) bool {
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
