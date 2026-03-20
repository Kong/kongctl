package httpclient

import (
	"net"
	"net/http"
	"time"
)

const (
	DefaultHTTPClientTimeout = 60 * time.Second
	defaultHTTPDialTimeout   = 30 * time.Second
	defaultHTTPKeepAlive     = 30 * time.Second
)

type ClientConfig struct {
	Timeout          time.Duration
	Jar              http.CookieJar
	TransportOptions TransportOptions
}

type TransportOptions struct {
	TCPUserTimeout            time.Duration
	DisableKeepAlives         bool
	RecycleConnectionsOnError bool
}

func NewHTTPClient(timeout time.Duration) *http.Client {
	return NewHTTPClientWithConfig(ClientConfig{Timeout: timeout})
}

func NewHTTPClientWithConfig(cfg ClientConfig) *http.Client {
	client := &http.Client{
		Jar:       cfg.Jar,
		Transport: newHTTPTransport(cfg.TransportOptions),
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
