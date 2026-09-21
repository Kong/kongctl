//go:build e2e

package harness

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"net/http/httptrace"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Capture phase progress before completion; omit URLs, headers, addresses and errors.
func traceSyntheticRequest(ctx context.Context, dir string) (context.Context, func()) {
	if os.Getenv("KONGCTL_E2E_LOG_LEVEL") != "trace" {
		return ctx, func() {}
	}
	f, err := os.OpenFile(filepath.Join(dir, "http-phases.jsonl"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return ctx, func() {}
	}
	var mu sync.Mutex
	closed := false
	start := time.Now()
	record := func(phase string) {
		mu.Lock()
		defer mu.Unlock()
		if !closed {
			_ = json.NewEncoder(f).Encode(map[string]any{"phase": phase, "elapsed_ms": time.Since(start).Milliseconds()})
		}
	}
	trace := &httptrace.ClientTrace{
		GetConn:              func(string) { record("connection_start") },
		GotConn:              func(httptrace.GotConnInfo) { record("connection_acquired") },
		DNSStart:             func(httptrace.DNSStartInfo) { record("dns_start") },
		DNSDone:              func(httptrace.DNSDoneInfo) { record("dns_done") },
		ConnectStart:         func(string, string) { record("connect_start") },
		ConnectDone:          func(string, string, error) { record("connect_done") },
		TLSHandshakeStart:    func() { record("tls_start") },
		TLSHandshakeDone:     func(tls.ConnectionState, error) { record("tls_done") },
		WroteRequest:         func(httptrace.WroteRequestInfo) { record("request_write_done") },
		GotFirstResponseByte: func() { record("first_response_byte") },
	}
	return httptrace.WithClientTrace(ctx, trace), func() { mu.Lock(); defer mu.Unlock(); closed = true; _ = f.Close() }
}
