//go:build e2e

package harness

import (
	"fmt"
	"testing"
	"time"
)

func TestShouldRetry_TimeoutThreshold(t *testing.T) {
	timeout := 60 * time.Second

	tests := []struct {
		name     string
		result   Result
		timeout  time.Duration
		wantSkip bool // true means ShouldRetry should return false
	}{
		{
			name: "full timeout (100%) should not retry",
			result: Result{
				ExitCode: -1,
				TimedOut: true,
				Duration: 60 * time.Second,
			},
			timeout:  timeout,
			wantSkip: true,
		},
		{
			name: "at threshold (90%) should not retry",
			result: Result{
				ExitCode: -1,
				TimedOut: true,
				Duration: 54 * time.Second,
			},
			timeout:  timeout,
			wantSkip: true,
		},
		{
			name: "just below threshold (89%) should retry",
			result: Result{
				ExitCode: -1,
				TimedOut: true,
				Duration: time.Duration(float64(timeout) * 0.89),
			},
			timeout:  timeout,
			wantSkip: false,
		},
		{
			name: "TimedOut false with exit -1 should retry",
			result: Result{
				ExitCode: -1,
				TimedOut: false,
				Duration: 60 * time.Second,
			},
			timeout:  timeout,
			wantSkip: false,
		},
		{
			name: "zero timeout bypasses threshold check",
			result: Result{
				ExitCode: -1,
				TimedOut: true,
				Duration: 60 * time.Second,
			},
			timeout:  0,
			wantSkip: false,
		},
		{
			name:    "zero Result and zero timeout (HTTP callers)",
			result:  Result{},
			timeout: 0,
			// err with exit -1 won't match because Result.ExitCode is 0;
			// the error itself triggers the ExitCode == -1 path via CommandError
			wantSkip: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build an error that would normally be retryable (CommandError with ExitCode -1)
			err := &CommandError{
				Result: tt.result,
				Err:    fmt.Errorf("signal: killed"),
			}

			got := ShouldRetry(err, "signal: killed", nil, nil, tt.result, tt.timeout)

			if tt.wantSkip && got {
				t.Errorf("ShouldRetry() = true, want false (should skip retry)")
			}
			if !tt.wantSkip && !got {
				t.Errorf("ShouldRetry() = false, want true (should retry)")
			}
		})
	}
}

func TestShouldRetry_NilError(t *testing.T) {
	got := ShouldRetry(nil, "", nil, nil, Result{}, 0)
	if got {
		t.Error("ShouldRetry(nil) = true, want false")
	}
}

func TestShouldRetry_NonTimeoutError(t *testing.T) {
	// A retryable pattern in the detail should still cause a retry
	err := fmt.Errorf("connection refused")
	got := ShouldRetry(err, "dial tcp: connection refused", nil, nil, Result{}, 0)
	if !got {
		t.Error("ShouldRetry() = false for retryable pattern, want true")
	}
}

func TestShouldRetry_NeverPattern(t *testing.T) {
	err := &CommandError{
		Result: Result{ExitCode: -1, TimedOut: false},
		Err:    fmt.Errorf("fatal error"),
	}
	got := ShouldRetry(err, "fatal error", nil, []string{"fatal"}, Result{ExitCode: -1}, 0)
	if got {
		t.Error("ShouldRetry() = true with never pattern match, want false")
	}
}
