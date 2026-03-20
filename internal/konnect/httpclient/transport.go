package httpclient

import (
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	DefaultHTTPClientTimeout = 60 * time.Second
	defaultHTTPDialTimeout   = 30 * time.Second
	defaultHTTPKeepAlive     = 30 * time.Second
)

type ClientConfig struct {
	Timeout time.Duration
	Jar     http.CookieJar
}

type TransportOptions struct {
	TCPUserTimeout            time.Duration
	DisableKeepAlives         bool
	RecycleConnectionsOnError bool
}

func TransportOptionsFromEnv() TransportOptions {
	return TransportOptions{
		TCPUserTimeout: durationEnvFirst(
			0,
			"KONGCTL_HTTP_TCP_USER_TIMEOUT",
			"KONGCTL_E2E_HTTP_TCP_USER_TIMEOUT",
		),
		DisableKeepAlives: boolEnvFirst(
			false,
			"KONGCTL_HTTP_DISABLE_KEEPALIVES",
			"KONGCTL_E2E_HTTP_DISABLE_KEEPALIVES",
		),
		RecycleConnectionsOnError: boolEnvFirst(
			false,
			"KONGCTL_HTTP_RECYCLE_CONNECTIONS_ON_ERROR",
			"KONGCTL_E2E_HTTP_RECYCLE_CONNECTIONS_ON_ERROR",
		),
	}
}

func NewHTTPClient(timeout time.Duration) *http.Client {
	return NewHTTPClientWithConfig(ClientConfig{Timeout: timeoutFromEnv(timeout)})
}

func NewHTTPClientWithConfig(cfg ClientConfig) *http.Client {
	client := &http.Client{
		Jar:       cfg.Jar,
		Transport: newHTTPTransport(TransportOptionsFromEnv()),
	}
	if cfg.Timeout > 0 {
		client.Timeout = cfg.Timeout
	}
	return client
}

type recyclingTransport struct {
	base    *http.Transport
	recycle bool
}

func (t *recyclingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)
	if err != nil && t.recycle {
		t.base.CloseIdleConnections()
	}
	return resp, err
}

func (t *recyclingTransport) CloseIdleConnections() {
	t.base.CloseIdleConnections()
}

func newHTTPTransport(options TransportOptions) http.RoundTripper {
	base, ok := http.DefaultTransport.(*http.Transport)
	if !ok || base == nil {
		base = &http.Transport{}
	}
	transport := base.Clone()
	transport.DisableKeepAlives = options.DisableKeepAlives

	dialer := &net.Dialer{
		Timeout:   defaultHTTPDialTimeout,
		KeepAlive: defaultHTTPKeepAlive,
	}
	configureTCPUserTimeout(dialer, options.TCPUserTimeout)
	transport.DialContext = dialer.DialContext

	return &recyclingTransport{
		base:    transport,
		recycle: options.RecycleConnectionsOnError,
	}
}

func timeoutFromEnv(fallback time.Duration) time.Duration {
	return durationEnvFirst(
		fallback,
		"KONGCTL_HTTP_TIMEOUT",
		"KONGCTL_E2E_HTTP_TIMEOUT",
	)
}

func durationEnvFirst(fallback time.Duration, names ...string) time.Duration {
	raw, ok := firstEnv(names...)
	if !ok {
		return fallback
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return fallback
	}
	return d
}

func boolEnvFirst(fallback bool, names ...string) bool {
	raw, ok := firstEnv(names...)
	if !ok {
		return fallback
	}
	switch strings.ToLower(raw) {
	case "1", "true", "yes", "on", "y":
		return true
	case "0", "false", "no", "off", "n":
		return false
	default:
		return fallback
	}
}

func firstEnv(names ...string) (string, bool) {
	for _, name := range names {
		value := strings.TrimSpace(os.Getenv(name))
		if value != "" {
			return value, true
		}
	}
	return "", false
}
