package httpclient

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestReadRecovery(t *testing.T) {
	for _, tc := range []struct {
		name, method, fault string
		persistent          bool
		attempts            int
	}{
		{"stalled-get", http.MethodGet, "stall", false, 2},
		{"reset-get", http.MethodGet, "reset", false, 2},
		{"persistent-stall", http.MethodGet, "stall", true, 2},
		{"persistent-reset", http.MethodGet, "reset", true, 2},
		{"post", http.MethodPost, "stall", false, 1},
		{"put", http.MethodPut, "reset", false, 1},
		{"delete", http.MethodDelete, "reset", false, 1},
		{"bad-request", http.MethodGet, "status", true, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				attempt := calls.Add(1)
				if attempt == 1 || tc.persistent {
					switch tc.fault {
					case "stall":
						<-r.Context().Done()
						return
					case "reset":
						conn, _, err := w.(http.Hijacker).Hijack()
						if err != nil {
							t.Error(err)
							return
						}
						_ = conn.Close()
						return
					case "status":
						w.WriteHeader(http.StatusBadRequest)
						return
					}
				}
				_, _ = io.WriteString(w, `{}`)
			}))
			defer server.Close()
			var logs bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
			cfg := RetryConfig{
				Strategy: RetryStrategyBackoff, MaxAttempts: 2, InitialIntervalMS: 1, MaxIntervalMS: 1,
				BackoffFactor: 2, RetryReadErrors: true,
			}
			inner := NewLoggingHTTPClientWithClient(NewHTTPClient(500*time.Millisecond), logger)
			client := NewRetryingHTTPClient(inner, cfg, logger)
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			started := time.Now()
			defer func() {
				if t.Failed() {
					t.Logf("received_requests=%d elapsed=%s outer_context_error=%v\nretry logs:\n%s",
						calls.Load(), time.Since(started), ctx.Err(), logs.String())
				}
			}()
			req, err := http.NewRequestWithContext(ctx, tc.method, server.URL, nil)
			require.NoError(t, err)
			resp, err := client.Do(req)
			if tc.fault == "status" {
				require.NoError(t, err)
				require.Equal(t, http.StatusBadRequest, resp.StatusCode)
			} else if tc.persistent || tc.attempts == 1 {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, http.StatusOK, resp.StatusCode)
				require.Contains(t, logs.String(), retryEventSucceeded)
			}
			if resp != nil {
				require.NoError(t, resp.Body.Close())
			}
			require.EqualValues(t, tc.attempts, calls.Load())
			require.NoError(t, ctx.Err(), "request recovery must finish before the command budget")
			if tc.fault == "stall" {
				require.Contains(t, logs.String(), `"error_class":"http_timeout"`)
			}
			if tc.persistent && tc.fault != "status" {
				require.Contains(t, logs.String(), retryEventExhausted)
			}
		})
	}
}

func TestReadRecoveryRespectsCancellationAndMethodFilter(t *testing.T) {
	cfg := defaultRetryConfig()
	cfg.RetryReadErrors = true
	for _, canceled := range []bool{false, true} {
		inner := &mockHTTPClient{responses: []*mockResponse{{err: io.EOF}}}
		method := http.MethodHead
		if canceled {
			method = http.MethodGet
		}
		client := NewRetryingHTTPClient(inner, cfg, nil, WithRetryableMethods(method))
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		if canceled {
			cancel()
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.test", nil)
		require.NoError(t, err)
		_, err = client.Do(req)
		require.Error(t, err)
		require.Equal(t, 1, inner.calls)
	}
}

func TestReadRecoveryWrappedReset(t *testing.T) {
	cfg := defaultRetryConfig()
	cfg.RetryReadErrors = true
	inner := &mockHTTPClient{responses: []*mockResponse{
		{err: &url.Error{Op: "Get", URL: "https://example.test", Err: &net.OpError{Op: "read", Err: syscall.ECONNRESET}}},
		{statusCode: http.StatusOK},
	}}
	client := NewRetryingHTTPClient(inner, cfg, nil)
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://example.test", nil)
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)
	require.Equal(t, 2, inner.calls)
	require.NoError(t, resp.Body.Close())
}

func TestReadErrorsOnlyDoesNotEnableStatusRetries(t *testing.T) {
	cfg := defaultRetryConfig()
	cfg.RetryReadErrors = true
	cfg.ReadErrorsOnly = true
	inner := &mockHTTPClient{responses: []*mockResponse{{statusCode: http.StatusServiceUnavailable}}}
	client := NewRetryingHTTPClient(inner, cfg, nil)
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://example.test", nil)
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	require.Equal(t, 1, inner.calls)
	require.NoError(t, resp.Body.Close())
}
