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

func TestUDPSink_BadAddrSurfacesError(t *testing.T) {
	// Invalid port forces ResolveUDPAddr to fail on first Emit.
	sink := NewUDPSink("127.0.0.1:not-a-port")
	defer func() { _ = sink.Close(t.Context()) }()

	if err := sink.Emit(t.Context(), Event{SchemaVersion: 1}); err == nil {
		t.Error("Emit on bad addr returned nil error, want non-nil")
	}
}
