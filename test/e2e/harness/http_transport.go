//go:build e2e

package harness

import (
	"net"
	"net/http"
	"time"
)

const (
	DefaultHTTPDialTimeout = 30 * time.Second
	DefaultHTTPKeepAlive   = 30 * time.Second
)

type HTTPTransportOptions struct {
	TCPUserTimeout            time.Duration
	DisableKeepAlives         bool
	RecycleConnectionsOnError bool
}

func newHTTPTransport(options HTTPTransportOptions) *http.Transport {
	base, ok := http.DefaultTransport.(*http.Transport)
	if !ok || base == nil {
		base = &http.Transport{}
	}
	transport := base.Clone()
	transport.DisableKeepAlives = options.DisableKeepAlives

	dialer := &net.Dialer{
		Timeout:   DefaultHTTPDialTimeout,
		KeepAlive: DefaultHTTPKeepAlive,
	}
	configureTCPUserTimeout(dialer, options.TCPUserTimeout)
	transport.DialContext = dialer.DialContext

	return transport
}

func maybeRecycleHTTPConnectionsOnError(client *http.Client, options HTTPTransportOptions, err error) {
	if client == nil || err == nil || !options.RecycleConnectionsOnError {
		return
	}
	client.CloseIdleConnections()
	Debugf("closed idle HTTP connections after error: %v", err)
}
