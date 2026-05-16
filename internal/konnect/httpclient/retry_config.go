package httpclient

const (
	// Retry strategy values consumed by RetryConfig.Strategy.
	RetryStrategyNone    = "none"
	RetryStrategyBackoff = "backoff"

	// RetryStrategyDefault is used when retry strategy is not explicitly set.
	RetryStrategyDefault = RetryStrategyBackoff

	// Default values applied by ResolveRetryConfig when the user does not provide
	// a specific retry setting.

	// DefaultRetryMaxAttempts is the default total number of attempts,
	// including the first request.
	// Example: with factor=2 and 3 total attempts there are 2 waits: 1s then 2s.
	DefaultRetryMaxAttempts = 3

	// DefaultRetryInitialIntervalMS is the default initial backoff interval in milliseconds.
	DefaultRetryInitialIntervalMS = 1_000

	// DefaultRetryMaxIntervalMS is the default maximum backoff interval in milliseconds.
	DefaultRetryMaxIntervalMS = 60_000

	// DefaultRetryBackoffFactor is the default exponential backoff multiplier.
	DefaultRetryBackoffFactor = 2.0

	// DefaultRetryConnectionErrors controls whether transport-level connection
	// errors are retried by default.
	DefaultRetryConnectionErrors = false

	// Allowed value ranges for user-provided retry settings.

	// MaxRetryMaxAttempts is the upper bound for total attempts.
	// Allowed attempts range in practice: [1..MaxRetryMaxAttempts] when enabled.
	MaxRetryMaxAttempts = 10

	// MinRetryInitialIntervalMS is the minimum configurable initial backoff interval.
	// Allowed range: [MinRetryInitialIntervalMS..MaxRetryInitialIntervalMS].
	// Lower values would hammer the server too aggressively.
	MinRetryInitialIntervalMS = 200

	// MaxRetryInitialIntervalMS is the maximum configurable initial backoff interval.
	// Allowed range: [MinRetryInitialIntervalMS..MaxRetryInitialIntervalMS].
	MaxRetryInitialIntervalMS = 30_000

	// MinRetryMaxIntervalMS is the minimum configurable backoff ceiling.
	// Allowed range: [MinRetryMaxIntervalMS..MaxRetryMaxIntervalMS].
	MinRetryMaxIntervalMS = 1_000

	// MaxRetryMaxIntervalMS is the maximum configurable backoff ceiling.
	// Allowed range: [MinRetryMaxIntervalMS..MaxRetryMaxIntervalMS].
	// 2 minutes covers any realistic API rate-limit reset window.
	MaxRetryMaxIntervalMS = 120_000
)

// RetryConfig holds retry/backoff parameters resolved from flags/config.
// All time intervals are stored as milliseconds. MaxAttempts is total
// attempts including the first request.
type RetryConfig struct {
	Strategy              string
	MaxAttempts           int
	InitialIntervalMS     int
	MaxIntervalMS         int
	BackoffFactor         float64
	RetryConnectionErrors bool
}
