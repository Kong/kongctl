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

	"github.com/kong/kongctl/internal/meta"
)

const (
	signalName   = meta.CLIName
	reportsAddr  = "kong-hf.konghq.com:61829"
	syslogPrefix = "<14>"
)

// udpSink fires one datagram per Emit at a UDP receiver. Best-effort by
// design: write errors bubble up to the dispatcher's debug log but never
// block command shutdown.
type udpSink struct {
	mu   sync.Mutex
	addr string
	conn net.Conn
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

	if err := s.ensureConnLocked(ctx); err != nil {
		return err
	}

	if deadline, ok := ctx.Deadline(); ok {
		_ = s.conn.SetWriteDeadline(deadline)
	}

	_, err := s.conn.Write([]byte(formatEventForSplunk(e)))
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

// ensureConnLocked dials the UDP socket through net.Dialer.DialContext so
// DNS resolution and the dial itself observe the caller's context. Without
// the ctx-aware dialer a stalled host lookup would block the dispatcher
// past flushTimeout and defeat the Recorder's bounded-shutdown guarantee.
func (s *udpSink) ensureConnLocked(ctx context.Context) error {
	if s.conn != nil {
		return nil
	}
	var d net.Dialer
	conn, err := d.DialContext(ctx, "udp", s.addr)
	if err != nil {
		return fmt.Errorf("dial udp %q: %w", s.addr, err)
	}
	s.conn = conn
	return nil
}

// formatEventForSplunk renders Event as a syslog-framed key=value line compatible
// with the Splunk UDP input .
func formatEventForSplunk(e Event) string {
	var b strings.Builder
	writeKV(&b, "signal", signalName)

	// Normalize to UTC so the wire timestamp ends in Z regardless of how the
	// caller constructed it; json.Marshal preserves the original offset
	// otherwise.
	e.Timestamp = e.Timestamp.UTC()

	data, err := json.Marshal(e)
	if err == nil {
		dec := json.NewDecoder(bytes.NewReader(data))
		dec.UseNumber() // keep schema_version etc. as integers, not float64
		var m map[string]any
		if err = dec.Decode(&m); err == nil {
			for _, k := range slices.Sorted(maps.Keys(m)) {
				writeKV(&b, k, jsonValueToString(m[k]))
			}
		}
	}

	return syslogPrefix + b.String() + "\n"
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
		const arraySep = "|"
		parts := make([]string, len(x))
		for i, item := range x {
			parts[i] = jsonValueToString(item)
		}
		return strings.Join(parts, arraySep)
	default:
		return fmt.Sprint(x)
	}
}

func writeKV(b *strings.Builder, k, v string) {
	const (
		pairSep = ';' // separates one key=value from the next
		kvSep   = '=' // separates a key from its value
		quote   = '"' // wraps values that need quoting
	)

	// this escapes characters that would otherwise break the
	// quoted key="value" form on the wire.
	quotedValueEscaper := strings.NewReplacer(
		`"`, `\"`,
		"\n", `\n`,
		"\r", `\r`,
	)
	if b.Len() > 0 {
		b.WriteByte(pairSep)
	}
	b.WriteString(k)
	b.WriteByte(kvSep)
	if needsQuoting(v) {
		b.WriteByte(quote)
		b.WriteString(quotedValueEscaper.Replace(v))
		b.WriteByte(quote)
	} else {
		b.WriteString(v)
	}
}

func needsQuoting(v string) bool {
	// quoteTriggers is the set of characters that force the value into
	// `"..."` form. Space and ; would break key=value tokenization at the
	// receiver; `"` and \n would break the quoted form itself.
	const quoteTriggers = " ;\"\n"

	// Empty values are quoted so the receiver sees `key=""` instead of a
	// bare `key=`, which key=value parsers can read ambiguously (either as
	// an empty value or as a value that runs into the next pair).
	if v == "" {
		return true
	}
	return strings.ContainsAny(v, quoteTriggers)
}
