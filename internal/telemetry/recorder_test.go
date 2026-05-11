package telemetry

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/kong/kongctl/internal/build"
	"github.com/spf13/pflag"
)

// fakeCfg is a minimal config.Hook used by tests. Only GetBool and GetPath
// matter for the recorder; the rest satisfy the interface.
type fakeCfg struct {
	bools map[string]bool
	path  string
}

func (f *fakeCfg) GetString(string) string             { return "" }
func (f *fakeCfg) GetBool(k string) bool               { return f.bools[k] }
func (f *fakeCfg) GetInt(string) int                   { return 0 }
func (f *fakeCfg) GetIntOrElse(_ string, or int) int   { return or }
func (f *fakeCfg) GetStringSlice(string) []string      { return nil }
func (f *fakeCfg) SetString(string, string)            {}
func (f *fakeCfg) Set(string, any)                     {}
func (f *fakeCfg) Get(string) any                      { return nil }
func (f *fakeCfg) BindFlag(string, *pflag.Flag) error  { return nil }
func (f *fakeCfg) GetProfile() string                  { return "default" }
func (f *fakeCfg) GetPath() string                     { return f.path }

// capturingSink records every event seen for inspection. Safe for concurrent
// use because the dispatcher and the test goroutine both touch it.
type capturingSink struct {
	mu     sync.Mutex
	events []Event
	closed bool
}

func (s *capturingSink) Emit(_ context.Context, e Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, e)
	return nil
}

func (s *capturingSink) Close(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}

func (s *capturingSink) Events() []Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Event, len(s.events))
	copy(out, s.events)
	return out
}

// blockingSink holds Emit until release is closed. Used to exercise the
// channel-full drop path and the bounded Close flush.
type blockingSink struct {
	started chan struct{}
	release chan struct{}

	mu        sync.Mutex
	emitCount int
}

func (s *blockingSink) Emit(_ context.Context, _ Event) error {
	select {
	case s.started <- struct{}{}:
	default:
	}
	<-s.release
	s.mu.Lock()
	s.emitCount++
	s.mu.Unlock()
	return nil
}

func (s *blockingSink) Close(_ context.Context) error { return nil }

func (s *blockingSink) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.emitCount
}

// newTestRecorder builds an enabled Recorder wired to sink and starts the
// dispatcher. Manual construction avoids the data race that would result
// from swapping the sink after NewRecorder.
func newTestRecorder(sink Sink) *Recorder {
	rec := &Recorder{
		enabled: true,
		logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		staticEvent: Event{
			SchemaVersion: SchemaVersion,
			Version:       "test",
			OS:            "testos",
			Arch:          "testarch",
		},
		sink:   sink,
		events: make(chan Event, channelBuffer),
		done:   make(chan struct{}),
	}
	go rec.dispatch()
	return rec
}

func TestFromContext_NilAndMissing(t *testing.T) {
	if got := FromContext(nil); got != nil { //nolint:staticcheck // intentional nil ctx
		t.Errorf("FromContext(nil) = %v, want nil", got)
	}
	if got := FromContext(t.Context()); got != nil {
		t.Errorf("FromContext(empty ctx) = %v, want nil", got)
	}
}

func TestContextWithRecorder_RoundTrip(t *testing.T) {
	rec := &Recorder{}
	ctx := ContextWithRecorder(t.Context(), rec)
	if got := FromContext(ctx); got != rec {
		t.Errorf("FromContext after ContextWithRecorder = %v, want %v", got, rec)
	}
}

func TestNewRecorder_NilCfg_Disabled(t *testing.T) {
	rec := NewRecorder(t.Context(), nil, nil, nil, nil)
	if rec == nil {
		t.Fatal("NewRecorder returned nil")
	}
	if rec.enabled {
		t.Errorf("enabled = true, want false when cfg is nil")
	}
	if _, ok := rec.sink.(NoopSink); !ok {
		t.Errorf("sink = %T, want NoopSink", rec.sink)
	}
}

func TestNewRecorder_FlagOff_Disabled(t *testing.T) {
	cfg := &fakeCfg{bools: map[string]bool{ConfigKeyEnabled: false}}
	rec := NewRecorder(t.Context(), cfg, nil, nil, nil)
	if rec.enabled {
		t.Errorf("enabled = true, want false when telemetry.enabled=false")
	}
	if _, ok := rec.sink.(NoopSink); !ok {
		t.Errorf("sink = %T, want NoopSink", rec.sink)
	}
}

func TestNewRecorder_FlagOn_Enabled(t *testing.T) {
	cfg := &fakeCfg{
		bools: map[string]bool{ConfigKeyEnabled: true, ConfigKeyDebug: false},
		path:  t.TempDir() + "/config.yaml",
	}
	bi := &build.Info{Version: "1.2.3"}
	rec := NewRecorder(t.Context(), cfg, bi, nil, nil)
	if !rec.enabled {
		t.Fatalf("enabled = false, want true when telemetry.enabled=true")
	}
	if rec.staticEvent.Version != "1.2.3" {
		t.Errorf("staticEvent.Version = %q, want %q", rec.staticEvent.Version, "1.2.3")
	}
	if err := rec.Close(t.Context()); err != nil {
		t.Errorf("Close: %v", err)
	}
}

func TestRecorder_NilReceiver_Safe(t *testing.T) {
	var rec *Recorder
	rec.Begin(time.Now())
	rec.SetCommand(CommandInfo{Path: "x"})
	rec.Finalize(nil, time.Now())
	if err := rec.Close(t.Context()); err != nil {
		t.Errorf("Close on nil: %v", err)
	}
}

func TestRecorder_Disabled_FinalizeNoop(t *testing.T) {
	sink := &capturingSink{}
	rec := &Recorder{enabled: false, sink: sink}
	rec.SetCommand(CommandInfo{Path: "kongctl plan"})
	rec.Finalize(nil, time.Now())
	if got := sink.Events(); len(got) != 0 {
		t.Errorf("disabled recorder emitted %d events, want 0", len(got))
	}
}

func TestRecorder_FinalizeWithoutSetCommand_Skips(t *testing.T) {
	sink := &capturingSink{}
	rec := newTestRecorder(sink)
	rec.Finalize(nil, time.Now())
	if err := rec.Close(t.Context()); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if got := sink.Events(); len(got) != 0 {
		t.Errorf("emitted %d events without SetCommand, want 0", len(got))
	}
}

func TestRecorder_FinalizeEmitsEvent(t *testing.T) {
	sink := &capturingSink{}
	rec := newTestRecorder(sink)

	end := time.Now()
	rec.SetCommand(CommandInfo{
		Path:     "kongctl plan",
		Area:     AreaDeclarative,
		FlagsSet: []string{"plan"},
	})
	rec.Finalize(nil, end)
	if err := rec.Close(t.Context()); err != nil {
		t.Fatalf("Close: %v", err)
	}

	events := sink.Events()
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	got := events[0]
	if got.CommandPath != "kongctl plan" {
		t.Errorf("CommandPath = %q, want %q", got.CommandPath, "kongctl plan")
	}
	if got.ExecArea != AreaDeclarative {
		t.Errorf("ExecArea = %q, want %q", got.ExecArea, AreaDeclarative)
	}
	if len(got.FlagsSet) != 1 || got.FlagsSet[0] != "plan" {
		t.Errorf("FlagsSet = %v, want [plan]", got.FlagsSet)
	}
	if !got.Timestamp.Equal(end) {
		t.Errorf("Timestamp = %v, want %v", got.Timestamp, end)
	}
	if got.SchemaVersion != SchemaVersion {
		t.Errorf("SchemaVersion = %d, want %d", got.SchemaVersion, SchemaVersion)
	}
	if got.Version != "test" || got.OS != "testos" || got.Arch != "testarch" {
		t.Errorf("static fields not carried: %+v", got)
	}
	if !sink.closed {
		t.Errorf("sink.Close not called during recorder Close")
	}
}

func TestRecorder_FinalizeDropsWhenChannelFull(t *testing.T) {
	sink := &blockingSink{
		started: make(chan struct{}, 1),
		release: make(chan struct{}),
	}
	rec := newTestRecorder(sink)
	rec.SetCommand(CommandInfo{Path: "kongctl x"})

	// First Finalize: dispatcher picks it up and blocks inside Emit.
	rec.Finalize(nil, time.Now())
	select {
	case <-sink.started:
	case <-time.After(time.Second):
		t.Fatal("dispatcher never entered Emit")
	}

	// Fill the buffered channel.
	for range channelBuffer {
		rec.Finalize(nil, time.Now())
	}
	// One extra send must be dropped (channel full + dispatcher blocked).
	rec.Finalize(nil, time.Now())

	close(sink.release)
	if err := rec.Close(t.Context()); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if got, want := sink.Count(), 1+channelBuffer; got != want {
		t.Errorf("emit count = %d, want %d (one event should have been dropped)", got, want)
	}
}

func TestRecorder_Close_RespectsDeadline(t *testing.T) {
	sink := &blockingSink{
		started: make(chan struct{}, 1),
		release: make(chan struct{}),
	}
	rec := newTestRecorder(sink)
	rec.SetCommand(CommandInfo{Path: "kongctl x"})
	rec.Finalize(nil, time.Now())
	<-sink.started

	start := time.Now()
	if err := rec.Close(t.Context()); err != nil {
		t.Fatalf("Close: %v", err)
	}
	elapsed := time.Since(start)

	// Close must abandon the slow sink at flushTimeout; allow generous slack
	// to keep the test stable on busy CI.
	if elapsed > flushTimeout+500*time.Millisecond {
		t.Errorf("Close took %v, want <= %v", elapsed, flushTimeout+500*time.Millisecond)
	}

	// Release the sink so the dispatcher can exit cleanly and not leak.
	close(sink.release)
}
