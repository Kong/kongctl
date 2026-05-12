package telemetry

import (
	"strings"
	"testing"
	"time"
)

func TestFormatEvent_Shape(t *testing.T) {
	ts := time.Date(2026, 5, 12, 14, 30, 45, 0, time.UTC)
	got := formatEventForSplunk(Event{
		SchemaVersion: 1,
		Timestamp:     ts,
		Version:       "0.4.0",
		OS:            "darwin",
		Arch:          "arm64",
		CommandPath:   "kongctl plan",
		ExecArea:      "declarative",
	})

	want := syslogPrefix + `signal=kongctl;arch=arm64;command_path="kongctl plan";` +
		`exec_area=declarative;os=darwin;schema_version=1;` +
		`timestamp=2026-05-12T14:30:45Z;version=0.4.0` + "\n"
	if got != want {
		t.Errorf("formatEvent mismatch\n got: %q\nwant: %q", got, want)
	}
}

func TestFormatEvent_QuotingAndEmpty(t *testing.T) {
	got := formatEventForSplunk(Event{
		SchemaVersion: 1,
		Timestamp:     time.Unix(0, 0).UTC(),
		CommandPath:   `weird "value", with spaces`,
		// Version / OS / Arch / ExecArea intentionally empty.
	})

	// Empty values must be quoted so the receiver sees `key=""` and not a
	// stray `key=` that could merge with the next pair.
	if !strings.Contains(got, `version=""`) {
		t.Errorf("empty version not quoted: %s", got)
	}
	// Embedded quotes get escaped, not stripped.
	// Note: commas no longer trigger quoting (separator is now ';'), but
	// spaces and quotes still do.
	if !strings.Contains(got, `command_path="weird \"value\", with spaces"`) {
		t.Errorf("command_path not quoted/escaped correctly: %s", got)
	}
	// Syslog prefix must be present.
	if !strings.HasPrefix(got, syslogPrefix) {
		t.Errorf("missing syslog prefix: %s", got)
	}
}

func TestFormatEvent_NewlinesEscaped(t *testing.T) {
	// A value containing \n or \r must not split the syslog payload across
	// lines — downstream key=value parsing reads one event per line.
	got := formatEventForSplunk(Event{
		SchemaVersion: 1,
		Timestamp:     time.Unix(0, 0).UTC(),
		CommandPath:   "line1\nline2\rline3",
	})

	// Exactly one trailing newline (the syslog frame terminator) — no raw
	// newlines or CRs survived from the value.
	if strings.Count(got, "\n") != 1 {
		t.Errorf("expected exactly one trailing newline, got %d: %q", strings.Count(got, "\n"), got)
	}
	if strings.ContainsRune(strings.TrimSuffix(got, "\n"), '\n') {
		t.Errorf("raw newline survived into payload: %q", got)
	}
	if strings.ContainsRune(got, '\r') {
		t.Errorf("raw CR survived into payload: %q", got)
	}
	// Literal `\n` / `\r` escape sequences should be visible in the quoted value.
	if !strings.Contains(got, `command_path="line1\nline2\rline3"`) {
		t.Errorf("escaped newline/CR not present: %q", got)
	}
}

func TestFormatEvent_LoneCarriageReturnQuoted(t *testing.T) {
	// A value whose only trigger character is \r must still be quoted and
	// escaped — otherwise a raw CR would survive into the payload and break
	// line-based parsing at the receiver.
	got := formatEventForSplunk(Event{
		SchemaVersion: 1,
		Timestamp:     time.Unix(0, 0).UTC(),
		CommandPath:   "a\rb",
	})

	if strings.ContainsRune(got, '\r') {
		t.Errorf("raw CR survived into payload: %q", got)
	}
	if !strings.Contains(got, `command_path="a\rb"`) {
		t.Errorf("CR not escaped inside quoted value: %q", got)
	}
}

func TestUDPSink_BadAddrSurfacesError(t *testing.T) {
	// Invalid port forces the dial to fail on first Emit.
	sink := NewUDPSink("127.0.0.1:not-a-port")
	defer func() { _ = sink.Close(t.Context()) }()

	if err := sink.Emit(t.Context(), Event{SchemaVersion: 1}); err == nil {
		t.Error("Emit on bad addr returned nil error, want non-nil")
	}
}
