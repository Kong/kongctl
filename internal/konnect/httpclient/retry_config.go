package httpclient

import "time"

const (
	RetryStrategyNone    = "none"
	RetryStrategyBackoff = "backoff"
	RetryStrategyDefault = RetryStrategyBackoff

	// DefaultRetryMaxAttempts is the default number of total attempts including the first.
	DefaultRetryMaxAttempts = 5
	// DefaultRetryInitialInterval is the default initial backoff interval.
	DefaultRetryInitialInterval = 500 * time.Millisecond
	// DefaultRetryMaxInterval is the default maximum backoff interval.
	DefaultRetryMaxInterval = 10 * time.Second
	// DefaultRetryBackoffFactor is the default exponential backoff multiplier.
	DefaultRetryBackoffFactor = 2.0
)

// RetryConfig holds retry/backoff parameters resolved from flags/config.
// MaxAttempts is total attempts including the first request.
type RetryConfig struct {
	Strategy              string
	MaxAttempts           int
	InitialInterval       time.Duration
	MaxInterval           time.Duration
	BackoffFactor         float64
	RetryConnectionErrors bool
}
