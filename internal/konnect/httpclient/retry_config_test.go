package httpclient

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestComputeMaxElapsedTimeMS(t *testing.T) {
	tests := []struct {
		name       string
		rc         RetryConfig
		expectedMS int
	}{
		{
			name: "defaults: 3 attempts, 1s initial, 60s max, factor 2",
			rc: RetryConfig{
				Strategy:          RetryStrategyBackoff,
				MaxAttempts:       3,
				InitialIntervalMS: 1_000,
				MaxIntervalMS:     60_000,
				BackoffFactor:     2.0,
			},
			// retry gaps: 1000, 2000
			expectedMS: 3_000,
		},
		{
			name: "single attempt: no retries",
			rc: RetryConfig{
				Strategy:          RetryStrategyBackoff,
				MaxAttempts:       1,
				InitialIntervalMS: 1_000,
				MaxIntervalMS:     60_000,
				BackoffFactor:     2.0,
			},
			expectedMS: 0,
		},
		{
			name: "max config: 10 attempts, 1s initial, 120s max, factor 2",
			rc: RetryConfig{
				Strategy:          RetryStrategyBackoff,
				MaxAttempts:       10,
				InitialIntervalMS: 1_000,
				MaxIntervalMS:     120_000,
				BackoffFactor:     2.0,
			},
			// retry gaps: 1000, 2000, 4000, 8000, 16000, 32000, 64000, 120000, 120000
			expectedMS: 367_000,
		},
		{
			name: "factor 1: all waits equal initial interval",
			rc: RetryConfig{
				Strategy:          RetryStrategyBackoff,
				MaxAttempts:       5,
				InitialIntervalMS: 500,
				MaxIntervalMS:     60_000,
				BackoffFactor:     1.0,
			},
			// retry gaps: 500 × 4
			expectedMS: 2_000,
		},
		{
			name: "cap kicks in early: small max interval",
			rc: RetryConfig{
				Strategy:          RetryStrategyBackoff,
				MaxAttempts:       5,
				InitialIntervalMS: 1_000,
				MaxIntervalMS:     2_000,
				BackoffFactor:     2.0,
			},
			// retry gaps: 1000, 2000, 2000, 2000
			expectedMS: 7_000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expectedMS, tt.rc.computeMaxElapsedTimeMS())
		})
	}
}
