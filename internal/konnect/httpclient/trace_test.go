package httpclient

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kong/kongctl/internal/log"
	"github.com/stretchr/testify/require"
)

func TestHTTPPhaseTrace(t *testing.T) {
	for _, mode := range []string{"success", "reset", "stalled"} {
		t.Run(mode, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch mode {
				case "reset":
					conn, _, err := w.(http.Hijacker).Hijack()
					if err == nil {
						_ = conn.Close()
					}
				case "stalled":
					<-r.Context().Done()
				default:
					_, _ = io.WriteString(w, "ok")
				}
			}))
			defer server.Close()
			path := filepath.Join(t.TempDir(), "trace.log")
			file, err := os.Create(path)
			require.NoError(t, err)
			defer file.Close()
			logger := slog.New(slog.NewJSONHandler(file, &slog.HandlerOptions{Level: log.LevelTrace}))
			underlying := server.Client()
			underlying.Timeout = 500 * time.Millisecond
			client := NewLoggingHTTPClientWithClient(underlying, logger)
			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL+"/resource?token=query-secret", nil)
			require.NoError(t, err)
			req.Header.Set("Authorization", "Bearer header-secret")
			resp, err := client.Do(req)
			if mode == "success" {
				require.NoError(t, err)
				require.NoError(t, resp.Body.Close())
			} else {
				require.Error(t, err)
			}
			data, err := os.ReadFile(path)
			require.NoError(t, err)
			phases := map[string]map[string]any{}
			records := parseJSONLogs(t, string(data))
			requestID := mustFindLogByType(t, records, logTypeRequest)["request_id"]
			for _, record := range records {
				if record["log_type"] != "http_phase" {
					continue
				}
				phases[record["phase"].(string)] = record
				require.Equal(t, requestID, record["request_id"])
				require.EqualValues(t, 500, record["http_timeout_ms"])
				require.NotContains(t, record, "query_params")
				require.NotContains(t, record, "error")
				require.NotContains(t, record, "request_headers")
				require.NotContains(t, record, "request_body")
				require.NotContains(t, record, "response_body")
			}
			for _, phase := range []string{
				"request_start", "connect_start", "connect_done", "tls_start", "tls_done",
				"connection_acquired", "request_written", "request_done",
			} {
				require.Contains(t, phases, phase)
			}
			if mode == "success" {
				require.Contains(t, phases, "first_response_byte")
				require.Equal(t, "response_headers", phases["request_done"]["outcome"])
			} else {
				require.Equal(t, "failed", phases["request_done"]["outcome"])
			}
		})
	}
}

func TestHTTPPhaseTraceComposesCallbacksAndOmitsErrorDetails(t *testing.T) {
	var output strings.Builder
	logger := slog.New(slog.NewJSONHandler(&output, &slog.HandlerOptions{Level: log.LevelTrace}))
	called := false
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		trace := httptrace.ContextClientTrace(req.Context())
		trace.DNSStart(httptrace.DNSStartInfo{Host: "private-host"})
		trace.DNSDone(httptrace.DNSDoneInfo{Err: errors.New("dns-secret")})
		trace.ConnectStart("tcp", "private-address")
		trace.ConnectDone("tcp", "private-address", errors.New("connect-secret"))
		trace.TLSHandshakeStart()
		trace.TLSHandshakeDone(tls.ConnectionState{}, errors.New("tls-secret"))
		return nil, errors.New("transport failed")
	})
	client := NewLoggingHTTPClientWithClient(&http.Client{Transport: transport}, logger)
	req, err := http.NewRequestWithContext(httptrace.WithClientTrace(t.Context(), &httptrace.ClientTrace{
		DNSStart: func(httptrace.DNSStartInfo) { called = true },
	}), http.MethodGet, "https://example.test/resource?token=secret", nil)
	require.NoError(t, err)
	_, err = client.Do(req)
	require.Error(t, err)
	require.True(t, called)
	for _, secret := range []string{"private-host", "private-address", "dns-secret", "connect-secret", "tls-secret"} {
		require.NotContains(t, output.String(), secret)
	}
}

func TestHTTPPhaseTraceProcess(t *testing.T) {
	if os.Getenv("KONGCTL_TEST_TRACE_PROCESS") != "1" {
		return
	}
	file, err := os.Create(os.Getenv("KONGCTL_TEST_TRACE_PATH"))
	require.NoError(t, err)
	defer file.Close()
	logger := slog.New(slog.NewJSONHandler(file, &slog.HandlerOptions{Level: log.LevelTrace}))
	client := NewLoggingHTTPClientWithClient(NewHTTPClient(0), logger)
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, os.Getenv("KONGCTL_TEST_TRACE_URL"), nil)
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
}

func TestHTTPPhaseTraceSurvivesProcessKill(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer server.Close()
	path := filepath.Join(t.TempDir(), "trace.log")
	bin, err := os.Executable()
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "-test.run=^TestHTTPPhaseTraceProcess$")
	cmd.Env = append(os.Environ(), "KONGCTL_TEST_TRACE_PROCESS=1",
		"KONGCTL_TEST_TRACE_PATH="+path, "KONGCTL_TEST_TRACE_URL="+server.URL)
	require.NoError(t, cmd.Start())
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()
	require.Eventually(t, func() bool {
		data, _ := os.ReadFile(path)
		return strings.Contains(string(data), `"phase":"request_written"`)
	}, 5*time.Second, 10*time.Millisecond)
	require.NoError(t, cmd.Process.Kill())
	require.Error(t, cmd.Wait())
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(data), `"phase":"request_written"`)
	require.NotContains(t, string(data), `"phase":"request_done"`)
	require.NotContains(t, string(data), `"phase":"first_response_byte"`)
}

func TestHTTPPhaseTraceMultipleClientsAndConnectionReuse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "ok")
	}))
	defer server.Close()
	file, err := os.Create(filepath.Join(t.TempDir(), "trace.log"))
	require.NoError(t, err)
	defer file.Close()
	logger := slog.New(slog.NewJSONHandler(file, &slog.HandlerOptions{Level: log.LevelTrace}))
	underlying := server.Client()
	for range 2 {
		client := NewLoggingHTTPClientWithClient(underlying, logger)
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL, nil)
		require.NoError(t, err)
		resp, err := client.Do(req)
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())
	}
	data, err := os.ReadFile(file.Name())
	require.NoError(t, err)
	ids := map[string]bool{}
	reused := false
	for _, record := range parseJSONLogs(t, string(data)) {
		if record["log_type"] != "http_phase" {
			continue
		}
		if record["phase"] == "request_start" {
			id := record["request_id"].(string)
			require.NotContains(t, ids, id)
			ids[id] = true
		}
		if record["phase"] == "connection_acquired" && record["reused"] == true {
			reused = true
		}
	}
	require.Len(t, ids, 2)
	require.True(t, reused)
}
