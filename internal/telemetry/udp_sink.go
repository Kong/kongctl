package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net"
	"slices"
	"strconv"
	"strings"
	"sync"
)

const (
	signalName  = "kongctl"
	reportsAddr = "kong-hf.konghq.com:61829"
)

// udpSink fires one datagram per Emit at a UDP receiver. Best-effort by
// design: write errors bubble up to the dispatcher's debug log but never
// block command shutdown.
type udpSink struct {
	mu   sync.Mutex
	addr string
	conn *net.UDPConn
}

// NewUDPSink returns a Sink that serializes events as Splunk key=value
// lines and writes one datagram per Emit. addr is host:port. The UDP socket
// is dialed lazily on first Emit so a configured-but-never-used sink
// imposes no startup cost.
func NewUDPSink(addr string) Sink {
	return &udpSink{addr: addr}
}

func (s *udpSink) Emit(ctx context.Context, e Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureConnLocked(); err != nil {
		return err
	}

	if deadline, ok := ctx.Deadline(); ok {
		_ = s.conn.SetWriteDeadline(deadline)
	}

	_, err := s.conn.Write([]byte(formatEvent(e)))
	return err
}

func (s *udpSink) Close(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn == nil {
		return nil
	}
	err := s.conn.Close()
	s.conn = nil
	return err
}

func (s *udpSink) ensureConnLocked() error {
	if s.conn != nil {
		return nil
	}
	udpAddr, err := net.ResolveUDPAddr("udp", s.addr)
	if err != nil {
		return fmt.Errorf("resolve udp addr %q: %w", s.addr, err)
	}
	conn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		return fmt.Errorf("dial udp %q: %w", s.addr, err)
	}
	s.conn = conn
	return nil
}

// formatEvent renders Event as a Splunk-style key=value line. The Event
// struct's json tags are the single source of truth: marshal the value,
// flatten the resulting object into key=value pairs sorted by key. Adding
// a tagged field to Event shows up here automatically, and `omitempty`
// fields drop out naturally.
//
// Values containing whitespace, commas, or quotes are double-quoted with
// `"` escaping so the receiver's extractions stay deterministic. Trailing
// newline keeps line-oriented inputs tidy.
func formatEvent(e Event) string {
	var b strings.Builder
	writeKV(&b, "signal", signalName)

	// Normalize to UTC so the wire timestamp ends in Z regardless of how the
	// caller constructed it; json.Marshal preserves the original offset
	// otherwise.
	e.Timestamp = e.Timestamp.UTC()

	data, err := json.Marshal(e)
	if err != nil {
		b.WriteByte('\n')
		return b.String()
	}

	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber() // keep schema_version etc. as integers, not float64
	var m map[string]any
	if err := dec.Decode(&m); err != nil {
		b.WriteByte('\n')
		return b.String()
	}

	for _, k := range slices.Sorted(maps.Keys(m)) {
		writeKV(&b, k, jsonValueToString(m[k]))
	}
	b.WriteByte('\n')
	return b.String()
}

func jsonValueToString(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case bool:
		return strconv.FormatBool(x)
	case json.Number:
		return string(x)
	case []any:
		parts := make([]string, len(x))
		for i, item := range x {
			parts[i] = jsonValueToString(item)
		}
		return strings.Join(parts, "|")
	default:
		return fmt.Sprint(x)
	}
}

func writeKV(b *strings.Builder, k, v string) {
	if b.Len() > 0 {
		b.WriteByte(',')
	}
	b.WriteString(k)
	b.WriteByte('=')
	if needsQuoting(v) {
		b.WriteByte('"')
		b.WriteString(strings.ReplaceAll(v, `"`, `\"`))
		b.WriteByte('"')
	} else {
		b.WriteString(v)
	}
}

func needsQuoting(v string) bool {
	// Empty values are quoted so the receiver sees `key=""` instead of a
	// bare `key=`, which key=value parsers can read ambiguously (either as
	// an empty value or as a value that runs into the next pair).
	if v == "" {
		return true
	}
	return strings.ContainsAny(v, " ,\"\n")
}
