package httpclient

import (
	"crypto/tls"
	"log/slog"
	"net/http"
	"net/http/httptrace"
	"time"

	"github.com/kong/kongctl/internal/log"
)

// Trace callbacks may run concurrently, and after Do returns. Each event is
// written immediately through slog's synchronized handler, without buffering
// until completion. Never include callback error text, addresses, or payloads.
func (c *LoggingHTTPClient) traceRequest(req *http.Request, id string, start time.Time) *http.Request {
	emit := func(phase string, attrs ...slog.Attr) { c.logHTTPPhase(req, id, start, phase, attrs...) }
	emit("request_start")
	trace := &httptrace.ClientTrace{
		GetConn: func(string) { emit("connection_acquire_start") },
		GotConn: func(info httptrace.GotConnInfo) {
			emit("connection_acquired", slog.Bool("reused", info.Reused),
				slog.Bool("was_idle", info.WasIdle), slog.Int64("idle_ms", info.IdleTime.Milliseconds()))
		},
		DNSStart: func(httptrace.DNSStartInfo) { emit("dns_start") },
		DNSDone: func(info httptrace.DNSDoneInfo) {
			emit("dns_done", slog.Bool("failed", info.Err != nil))
		},
		ConnectStart:      func(string, string) { emit("connect_start") },
		ConnectDone:       func(_, _ string, err error) { emit("connect_done", slog.Bool("failed", err != nil)) },
		TLSHandshakeStart: func() { emit("tls_start") },
		TLSHandshakeDone: func(_ tls.ConnectionState, err error) {
			emit("tls_done", slog.Bool("failed", err != nil))
		},
		WroteHeaders: func() { emit("request_headers_written") },
		WroteRequest: func(info httptrace.WroteRequestInfo) {
			emit("request_written", slog.Bool("failed", info.Err != nil))
		},
		GotFirstResponseByte: func() { emit("first_response_byte") },
	}
	return req.WithContext(httptrace.WithClientTrace(req.Context(), trace))
}

func (c *LoggingHTTPClient) logHTTPPhase(req *http.Request, id string, start time.Time,
	phase string, attrs ...slog.Attr,
) {
	fields := []slog.Attr{
		slog.String("log_type", "http_phase"), slog.String("http_source", httpSource),
		slog.String("request_id", id), slog.String("phase", phase),
		slog.Int64("elapsed_ms", time.Since(start).Milliseconds()),
		slog.Int64("http_timeout_ms", c.wrapped.Timeout.Milliseconds()),
		slog.String("method", req.Method), slog.String("route", routeFromURL(req.URL)),
	}
	if req.URL != nil {
		fields = append(fields, slog.String("host", req.URL.Hostname()))
	}
	fields = append(fields, attrs...)
	c.logger.LogAttrs(req.Context(), log.LevelTrace, "HTTP connection phase", fields...)
}
