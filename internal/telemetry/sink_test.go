package telemetry

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFileSink_AppendsJSONL(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "telemetry.log")
	sink := NewFileSink(path)

	events := []Event{
		{
			SchemaVersion: 1,
			CommandPath:   "kongctl version",
			Outcome:       "success",
			Timestamp:     time.Unix(1, 0),
		},
		{
			SchemaVersion: 1,
			CommandPath:   "kongctl get apis",
			Outcome:       "user_error",
			Timestamp:     time.Unix(2, 0),
		},
		{
			SchemaVersion: 1,
			CommandPath:   "kongctl apply",
			Outcome:       "interrupted",
			Timestamp:     time.Unix(3, 0),
		},
	}
	for _, e := range events {
		if err := sink.Emit(t.Context(), e); err != nil {
			t.Fatalf("Emit: %v", err)
		}
	}
	if err := sink.Close(t.Context()); err != nil {
		t.Fatalf("Close: %v", err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = f.Close() })

	scanner := bufio.NewScanner(f)
	var got []Event
	for scanner.Scan() {
		var e Event
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			t.Fatalf("unmarshal %q: %v", scanner.Text(), err)
		}
		got = append(got, e)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(got) != len(events) {
		t.Fatalf("got %d events, want %d", len(got), len(events))
	}
	for i := range events {
		if got[i].CommandPath != events[i].CommandPath || got[i].Outcome != events[i].Outcome {
			t.Errorf("event %d: got %+v want %+v", i, got[i], events[i])
		}
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if mode := info.Mode().Perm(); mode != fs.FileMode(0o600) {
		t.Errorf("file mode = %o, want 0o600", mode)
	}
}

type errSink struct {
	emitErr   error
	emitCalls int
	closeErr  error
}

func (s *errSink) Emit(_ context.Context, _ Event) error {
	s.emitCalls++
	return s.emitErr
}

func (s *errSink) Close(_ context.Context) error { return s.closeErr }

func TestMultiSink_FansOutAndKeepsGoingOnError(t *testing.T) {
	a := &errSink{emitErr: errors.New("boom")}
	b := &errSink{}
	c := &errSink{}
	m := NewMultiSink(a, b, c)

	err := m.Emit(t.Context(), Event{CommandPath: "kongctl version"})
	if err == nil || err.Error() != "boom" {
		t.Fatalf("expected first error to bubble, got %v", err)
	}
	if a.emitCalls != 1 || b.emitCalls != 1 || c.emitCalls != 1 {
		t.Fatalf("expected all sinks called: a=%d b=%d c=%d", a.emitCalls, b.emitCalls, c.emitCalls)
	}
}

func TestMultiSink_CollapsesNilAndSingle(t *testing.T) {
	if _, ok := NewMultiSink().(NoopSink); !ok {
		t.Errorf("empty multisink should collapse to NoopSink")
	}
	if _, ok := NewMultiSink(nil, nil).(NoopSink); !ok {
		t.Errorf("all-nil multisink should collapse to NoopSink")
	}
	a := &errSink{}
	if got := NewMultiSink(nil, a, nil); got != a {
		t.Errorf("single non-nil multisink should collapse to that sink")
	}
}

func TestNoopSink(t *testing.T) {
	var s NoopSink
	if err := s.Emit(t.Context(), Event{}); err != nil {
		t.Errorf("Emit: %v", err)
	}
	if err := s.Close(t.Context()); err != nil {
		t.Errorf("Close: %v", err)
	}
}
