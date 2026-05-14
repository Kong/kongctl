package httpclient

import (
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	sdkretry "github.com/Kong/sdk-konnect-go/retry"
)

const (
	RetryStrategyNone    = "none"
	RetryStrategyBackoff = "backoff"
	RetryStrategyDefault = RetryStrategyBackoff

	// DefaultRetryMaxAttempts is the default number of total attempts including the first.
	// 3 attempts yields retries at 1s, 2s, 4s
	DefaultRetryMaxAttempts = 3
	// DefaultRetryInitialIntervalMS is the default initial backoff interval in milliseconds.
	DefaultRetryInitialIntervalMS = 1_000
	// DefaultRetryMaxIntervalMS is the default maximum backoff interval in milliseconds.
	DefaultRetryMaxIntervalMS = 60_000
	// DefaultRetryBackoffFactor is the default exponential backoff multiplier.
	DefaultRetryBackoffFactor    = 2.0
	DefaultRetryConnectionErrors = false

	// MaxRetryMaxAttempts is the maximum configurable number of total attempts.
	MaxRetryMaxAttempts = 10

	// MinRetryInitialIntervalMS is the minimum configurable initial backoff interval.
	// Values below this floor would hammer the server too aggressively.
	MinRetryInitialIntervalMS = 200
	// MaxRetryInitialIntervalMS is the maximum configurable initial backoff interval.
	MaxRetryInitialIntervalMS = 30_000

	// MinRetryMaxIntervalMS is the minimum configurable backoff ceiling.
	MinRetryMaxIntervalMS = 1_000
	// MaxRetryMaxIntervalMS is the maximum configurable backoff ceiling.
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

// ToSDKRetryConfig converts a RetryConfig into the sdk-konnect-go retry.Config
// that can be passed to SDK operations via WithRetries.
func (rc RetryConfig) ToSDKRetryConfig() sdkretry.Config {
	cfg := sdkretry.Config{
		Strategy:              rc.Strategy,
		RetryConnectionErrors: rc.RetryConnectionErrors,
	}
	if rc.Strategy == RetryStrategyBackoff {
		cfg.Backoff = &sdkretry.BackoffStrategy{
			InitialInterval: rc.InitialIntervalMS,
			MaxInterval:     rc.MaxIntervalMS,
			Exponent:        rc.BackoffFactor,
			MaxElapsedTime:  rc.MaxIntervalMS * rc.MaxAttempts,
		}
	}
	return cfg
}

// ToSDKOption returns an kkOps.Option that injects this RetryConfig into an SDK call.
func (rc RetryConfig) ToSDKOption() kkOps.Option {
	return kkOps.WithRetries(rc.ToSDKRetryConfig())
}
