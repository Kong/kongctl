//go:build e2e

package harness

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestRetryAfterDelay(t *testing.T) {
	err := &httpError{
		status: http.StatusTooManyRequests,
		header: http.Header{"Retry-After": []string{"7"}},
	}

	got, ok := RetryAfterDelay(err)
	if !ok {
		t.Fatal("RetryAfterDelay() = not found, want found")
	}
	if got != 7*time.Second {
		t.Fatalf("RetryAfterDelay() = %s, want 7s", got)
	}
}

func TestRetryDelayForErrorPrefersRetryAfter(t *testing.T) {
	err := &httpError{
		status: http.StatusServiceUnavailable,
		header: http.Header{"Retry-After": []string{"11"}},
	}

	got := RetryDelayForError(err, []time.Duration{2 * time.Second}, 0)
	if got != 11*time.Second {
		t.Fatalf("RetryDelayForError() = %s, want 11s", got)
	}
}

func TestClassifyRetry(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		detail string
		want   RetryClass
	}{
		{
			name:   "timeout",
			err:    context.DeadlineExceeded,
			detail: "context deadline exceeded",
			want:   RetryClassTimeout,
		},
		{
			name:   "throttle",
			err:    &httpError{status: http.StatusTooManyRequests},
			detail: "status=429",
			want:   RetryClassThrottle,
		},
		{
			name:   "network",
			err:    &httpError{status: 0},
			detail: "dial tcp: connection refused",
			want:   RetryClassNetwork,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyRetry(tt.err, tt.detail)
			if got != tt.want {
				t.Fatalf("ClassifyRetry() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestShouldRetryHTTPAttempt(t *testing.T) {
	timeout := 10 * time.Second

	if !ShouldRetryHTTPAttempt(
		context.DeadlineExceeded,
		"context deadline exceeded",
		timeout,
		timeout,
		nil,
		nil,
		0,
	) {
		t.Fatal("ShouldRetryHTTPAttempt() = false on first full timeout, want true")
	}

	if ShouldRetryHTTPAttempt(
		context.DeadlineExceeded,
		"context deadline exceeded",
		timeout,
		timeout,
		nil,
		nil,
		1,
	) {
		t.Fatal("ShouldRetryHTTPAttempt() = true on repeated full timeout, want false")
	}

	if !ShouldRetryHTTPAttempt(
		context.DeadlineExceeded,
		"context deadline exceeded",
		timeout,
		5*time.Second,
		nil,
		nil,
		1,
	) {
		t.Fatal("ShouldRetryHTTPAttempt() = false on partial timeout, want true")
	}
}

func TestShouldRetryResetHTTPAttempt(t *testing.T) {
	if !ShouldRetryResetHTTPAttempt(
		context.DeadlineExceeded,
		"context deadline exceeded",
	) {
		t.Fatal("ShouldRetryResetHTTPAttempt() = false on timeout, want true")
	}

	if !ShouldRetryResetHTTPAttempt(
		context.DeadlineExceeded,
		"context deadline exceeded",
	) {
		t.Fatal("ShouldRetryResetHTTPAttempt() = false on repeated full timeout, want true")
	}
}

func TestResetHTTPPolicyFromEnv(t *testing.T) {
	t.Setenv("KONGCTL_E2E_RESET_HTTP_TIMEOUT", "12s")
	t.Setenv("KONGCTL_E2E_RESET_TIMEOUT", "2m")
	t.Setenv("KONGCTL_E2E_RESET_RETRY_ATTEMPTS", "4")
	t.Setenv("KONGCTL_E2E_RESET_RETRY_INTERVAL", "2s")
	t.Setenv("KONGCTL_E2E_RESET_RETRY_MAX_INTERVAL", "6s")
	t.Setenv("KONGCTL_E2E_RESET_RETRY_BACKOFF_FACTOR", "3")
	t.Setenv("KONGCTL_E2E_RESET_RETRY_JITTER", "400ms")

	got := resetHTTPPolicyFromEnv()
	if got.RequestTimeout != 12*time.Second {
		t.Fatalf("RequestTimeout = %s, want 12s", got.RequestTimeout)
	}
	if got.TotalTimeout != 2*time.Minute {
		t.Fatalf("TotalTimeout = %s, want 2m", got.TotalTimeout)
	}
	if got.Backoff.Attempts != 4 {
		t.Fatalf("Attempts = %d, want 4", got.Backoff.Attempts)
	}
	if got.Backoff.Base != 2*time.Second {
		t.Fatalf("Base = %s, want 2s", got.Backoff.Base)
	}
	if got.Backoff.Max != 6*time.Second {
		t.Fatalf("Max = %s, want 6s", got.Backoff.Max)
	}
	if got.Backoff.Factor != 3 {
		t.Fatalf("Factor = %v, want 3", got.Backoff.Factor)
	}
	if got.Backoff.Jitter != 400*time.Millisecond {
		t.Fatalf("Jitter = %s, want 400ms", got.Backoff.Jitter)
	}
}

func TestRawHTTPRetryDefaultsFromEnv(t *testing.T) {
	t.Setenv("KONGCTL_E2E_HTTP_TIMEOUT", "13s")
	t.Setenv("KONGCTL_E2E_HTTP_RETRY_ATTEMPTS", "5")
	t.Setenv("KONGCTL_E2E_HTTP_RETRY_INTERVAL", "1500ms")
	t.Setenv("KONGCTL_E2E_HTTP_RETRY_MAX_INTERVAL", "7s")
	t.Setenv("KONGCTL_E2E_HTTP_RETRY_BACKOFF_FACTOR", "4")
	t.Setenv("KONGCTL_E2E_HTTP_RETRY_JITTER", "300ms")

	if got := HTTPRequestTimeout(); got != 13*time.Second {
		t.Fatalf("HTTPRequestTimeout() = %s, want 13s", got)
	}

	got := RawHTTPRetryDefaults()
	if got.Attempts != 5 {
		t.Fatalf("Attempts = %d, want 5", got.Attempts)
	}
	if got.Base != 1500*time.Millisecond {
		t.Fatalf("Base = %s, want 1500ms", got.Base)
	}
	if got.Max != 7*time.Second {
		t.Fatalf("Max = %s, want 7s", got.Max)
	}
	if got.Factor != 4 {
		t.Fatalf("Factor = %v, want 4", got.Factor)
	}
	if got.Jitter != 300*time.Millisecond {
		t.Fatalf("Jitter = %s, want 300ms", got.Jitter)
	}
}

func TestHTTPTransportOptionsFromEnv(t *testing.T) {
	t.Setenv("KONGCTL_E2E_HTTP_TCP_USER_TIMEOUT", "60s")
	t.Setenv("KONGCTL_E2E_HTTP_DISABLE_KEEPALIVES", "true")
	t.Setenv("KONGCTL_E2E_HTTP_RECYCLE_CONNECTIONS_ON_ERROR", "1")

	got := HTTPTransportOptionsFromEnv()
	if got.TCPUserTimeout != 60*time.Second {
		t.Fatalf("TCPUserTimeout = %s, want 60s", got.TCPUserTimeout)
	}
	if !got.DisableKeepAlives {
		t.Fatal("DisableKeepAlives = false, want true")
	}
	if !got.RecycleConnectionsOnError {
		t.Fatal("RecycleConnectionsOnError = false, want true")
	}
}

func TestNewHTTPClientWithOptions(t *testing.T) {
	client := newHTTPClientWithOptions(9*time.Second, HTTPTransportOptions{
		DisableKeepAlives: true,
	})

	if client.Timeout != 9*time.Second {
		t.Fatalf("client.Timeout = %s, want 9s", client.Timeout)
	}

	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("client.Transport type = %T, want *http.Transport", client.Transport)
	}
	if !transport.DisableKeepAlives {
		t.Fatal("transport.DisableKeepAlives = false, want true")
	}
}

func TestMaybeRecycleHTTPConnectionsOnError(t *testing.T) {
	transport := &idleClosingTransport{}
	client := &http.Client{Transport: transport}

	maybeRecycleHTTPConnectionsOnError(client, HTTPTransportOptions{
		RecycleConnectionsOnError: true,
	}, fmt.Errorf("boom"))

	if transport.closed != 1 {
		t.Fatalf("closed = %d, want 1", transport.closed)
	}

	maybeRecycleHTTPConnectionsOnError(client, HTTPTransportOptions{}, fmt.Errorf("boom"))
	if transport.closed != 1 {
		t.Fatalf("closed after disabled recycle = %d, want 1", transport.closed)
	}

	maybeRecycleHTTPConnectionsOnError(client, HTTPTransportOptions{
		RecycleConnectionsOnError: true,
	}, nil)
	if transport.closed != 1 {
		t.Fatalf("closed after nil error = %d, want 1", transport.closed)
	}
}

func TestResetHTTPSessionRebuildsClientAfterError(t *testing.T) {
	var (
		clients    []*http.Client
		transports []*idleClosingTransport
	)
	session := &resetHTTPSession{
		newClient: func() *http.Client {
			transport := &idleClosingTransport{}
			client := &http.Client{Transport: transport}
			clients = append(clients, client)
			transports = append(transports, transport)
			return client
		},
	}

	first := session.Client()
	if first == nil {
		t.Fatal("session.Client() = nil, want client")
	}
	if again := session.Client(); again != first {
		t.Fatal("session.Client() returned a different client without rebuild")
	}

	session.Rebuild(context.DeadlineExceeded)

	if transports[0].closed != 1 {
		t.Fatalf("transports[0].closed = %d, want 1", transports[0].closed)
	}

	second := session.Client()
	if second == nil {
		t.Fatal("session.Client() after rebuild = nil, want client")
	}
	if second == first {
		t.Fatal("session.Client() after rebuild returned the same client, want a fresh one")
	}
	if len(clients) != 2 {
		t.Fatalf("len(clients) = %d, want 2", len(clients))
	}

	session.Close()
	if transports[1].closed != 1 {
		t.Fatalf("transports[1].closed = %d, want 1", transports[1].closed)
	}
}

type idleClosingTransport struct {
	closed int
}

func (t *idleClosingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, fmt.Errorf("unused")
}

func (t *idleClosingTransport) CloseIdleConnections() {
	t.closed++
}
