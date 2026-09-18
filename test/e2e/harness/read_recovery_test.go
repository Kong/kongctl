//go:build e2e

package harness

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kong/kongctl/internal/konnect/httpclient"
)

// A real subprocess exercises the request deadline against the harness kill
// deadline without requiring credentials or a production fault-injection flag.
func TestReadRecoveryProcess(t *testing.T) {
	if os.Getenv("KONGCTL_TEST_READ_RECOVERY_PROCESS") != "1" {
		return
	}
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	client := httpclient.NewRetryingHTTPClient(
		httpclient.NewLoggingHTTPClientWithClient(httpclient.NewHTTPClient(500*time.Millisecond), logger),
		httpclient.RetryConfig{
			Strategy: httpclient.RetryStrategyBackoff, MaxAttempts: 2,
			InitialIntervalMS: 1, MaxIntervalMS: 1, BackoffFactor: 2, RetryReadErrors: true,
		}, logger,
	)
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, os.Getenv("KONGCTL_TEST_READ_RECOVERY_URL"), nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
}

func TestRequestTimeoutPrecedesSubprocessDeadline(t *testing.T) {
	for _, persistent := range []bool{false, true} {
		t.Run(map[bool]string{false: "recovered", true: "exhausted"}[persistent], func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if calls.Add(1) == 1 || persistent {
					<-r.Context().Done()
					return
				}
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()
			bin, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			cli := &CLI{BinPath: bin, Env: os.Environ(), TestDir: t.TempDir()}
			res, err := cli.RunWithEnvTimeout(context.Background(), map[string]string{
				"KONGCTL_TEST_READ_RECOVERY_PROCESS": "1",
				"KONGCTL_TEST_READ_RECOVERY_URL":     server.URL,
				// Race-instrumented helpers otherwise sleep one second at exit.
				"GORACE": "atexit_sleep_ms=0",
			}, 5*time.Second, "-test.run=^TestReadRecoveryProcess$")
			defer func() {
				if t.Failed() {
					t.Logf("received_requests=%d elapsed=%s timed_out=%t\nstdout:\n%s\nstderr:\n%s",
						calls.Load(), res.Duration, res.TimedOut, res.Stdout, res.Stderr)
				}
			}()
			if (err != nil) != persistent {
				t.Fatalf("unexpected result: %v\n%s", err, res.Stderr)
			}
			if res.TimedOut {
				t.Fatalf("request reached subprocess deadline: %s", res.Stderr)
			}
			if calls.Load() != 2 {
				t.Fatalf("request attempts=%d, want 2", calls.Load())
			}
		})
	}
}
