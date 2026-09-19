//go:build e2e

package harness

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReadHTTPPhases(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kongctl.log")
	file, err := os.Create(path)
	require.NoError(t, err)
	defer file.Close()
	logger := slog.New(slog.NewTextHandler(file, nil))
	logger.Info("phase=request_done log_type=http_phase request_id=khttp-000099", "log_type", "http_response")
	emit := func(id, phase string, elapsed int, attrs ...any) {
		fields := []any{
			"log_type", "http_phase", "request_id", id,
			"phase", phase, "elapsed_ms", elapsed, "http_timeout_ms", 15000,
		}
		logger.Info("HTTP connection phase", append(fields, attrs...)...)
	}
	emit("khttp-000001", "request_start", 0)
	emit("khttp-000001", "request_written", 5)
	emit("khttp-000002", "request_start", 0)
	emit("khttp-000002", "connect_done", 10, "failed", true)
	emit("khttp-000002", "request_done", 11, "outcome", "failed")
	emit("khttp-000002", "dns_done", 12)
	emit("khttp-000003", "request_start", 0)
	emit("khttp-000003", "first_response_byte", 2)
	emit("khttp-000003", "request_done", 3, "outcome", "response_headers")
	emit("secret", "request_written", 0)
	requests, err := ReadHTTPPhases(path)
	require.NoError(t, err)
	require.Len(t, requests, 3)
	require.Equal(t, HTTPPhaseDiagnostic{
		RequestID: "khttp-000001", LastPhase: "request_written", ElapsedMS: 5,
		HTTPTimeoutMS: 15000, Outcome: "unfinished",
	}, requests[0])
	require.Equal(t, "connect_done", requests[1].LastPhase)
	require.EqualValues(t, 10, requests[1].ElapsedMS)
	require.Equal(t, "failed", requests[1].Outcome)
	require.Equal(t, "response_headers", requests[2].Outcome)
}
