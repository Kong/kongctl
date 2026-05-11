// Package telemetry emits one best-effort event per kongctl command
// execution. The package is backend-agnostic: it owns a stable internal
// schema (Event), centralizes error categorization (Categorize), and routes
// events through a replaceable Sink.
// At the moment, the default Sink is NoopSink; opting in
// is a single config flag (telemetry.enabled).
package telemetry

import (
	"context"
	"log/slog"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/kong/kongctl/internal/build"
	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/iostreams"
)

// Config keys read by NewRecorder. Defaults registered in
// internal/config/config.go.
const (
	ConfigKeyEnabled = "telemetry.enabled"
	// ConfigKeyDebug enables the local JSONL file sink. Events are only
	// persisted to disk when both telemetry.enabled and telemetry.debug
	// are true. Intended for kongctl developers verifying emitted events;
	// end users should leave this off so no telemetry data lands on their
	// machine.
	ConfigKeyDebug = "telemetry.debug"
)

// telemetryLogFileName is the file in the kongctl config directory that the
// default fileSink appends JSONL events to.
const telemetryLogFileName = "telemetry.log"

// flushTimeout caps how long Close blocks the shutdown path. Picked to be
// below human-noticeable hang threshold while leaving headroom for one
// HTTPS POST when an HTTP sink lands.
const flushTimeout = 500 * time.Millisecond

// channelBuffer is the in-flight event buffer size. kongctl emits one event
// per process today; "1 + headroom" is enough.
const channelBuffer = 8

// recorderKey keys the Recorder onto the command context.
type recorderKey struct{}

// ContextWithRecorder returns ctx with rec attached. PersistentPreRun looks
// it up via FromContext to populate per-command fields.
func ContextWithRecorder(ctx context.Context, rec *Recorder) context.Context {
	return context.WithValue(ctx, recorderKey{}, rec)
}

// FromContext returns the Recorder attached to ctx, or nil if telemetry is
// disabled.
func FromContext(ctx context.Context) *Recorder {
	if ctx == nil {
		return nil
	}
	r, _ := ctx.Value(recorderKey{}).(*Recorder)
	return r
}

// CommandInfo is the data SetCommand expects from the root PersistentPreRun
// hook.
type CommandInfo struct {
	Path     string
	FlagsSet []string
	Area     string
}

// Recorder buffers a single event for the duration of one command execution
// and flushes it to a Sink on Close. A Recorder is single-use: Begin → (one
// SetCommand) → Finalize → Close. When telemetry is disabled, NewRecorder
// returns a Recorder backed by NoopSink so the call shape stays uniform.
type Recorder struct {
	enabled bool
	logger  *slog.Logger

	cfg config.Hook

	staticEvent Event // pre-populated Version/OS/Arch

	mu        sync.Mutex
	startedAt time.Time
	cmdInfo   CommandInfo
	cmdSet    bool

	sink   Sink
	events chan Event
	done   chan struct{}
}

// NewRecorder builds a Recorder. It reads telemetry.enabled from cfg; if
// false, it returns a Recorder whose dispatch path is a no-op. Errors during
// install-ID load are logged at debug level and treated as "telemetry off"
// for this run — telemetry must never fail a command.
func NewRecorder(
	_ context.Context,
	cfg config.Hook,
	bi *build.Info,
	_ *iostreams.IOStreams,
	logger *slog.Logger,
) *Recorder {
	logger = loggerOrDiscard(logger)

	if cfg == nil || !cfg.GetBool(ConfigKeyEnabled) {
		return &Recorder{
			enabled: false,
			logger:  logger,
			cfg:     cfg,
			sink:    NoopSink{},
		}
	}

	rec := &Recorder{
		enabled: true,
		logger:  logger,
		cfg:     cfg,
		staticEvent: Event{
			SchemaVersion: SchemaVersion,
			OS:            runtime.GOOS,
			Arch:          runtime.GOARCH,
		},
		sink:   buildDefaultSink(cfg),
		events: make(chan Event, channelBuffer),
		done:   make(chan struct{}),
	}
	if bi != nil {
		rec.staticEvent.Version = bi.Version
	}

	// Dispatcher runs for the lifetime of the recorder, not the request. It
	// must outlive the command context so it can drain queued events during
	// the bounded Close flush after a cancellation.
	go rec.dispatch() //nolint:gosec // G118: intentional process-scoped goroutine

	return rec
}

func buildDefaultSink(cfg config.Hook) Sink {
	// telemetry.debug enables the local JSONL file sink. It is developer-only:
	// end users (telemetry.enabled=true, telemetry.debug=false) must never get
	// a telemetry.log written to their machine.
	if cfg.GetBool(ConfigKeyDebug) {
		path := filepath.Join(filepath.Dir(cfg.GetPath()), telemetryLogFileName)
		return NewFileSink(path)
	}

	// TODO(telemetry): replace with NewSink(...) once a backend is wired.
	// Until then, "telemetry.enabled=true, telemetry.debug=false" has nowhere
	// to send events, so the recorder still runs but discards them.
	// When the new sink lands, both sinks should be composed via NewMultiSink so
	// debug developers see local and remote events together.
	return NoopSink{}
}

// Begin records the command start time. Safe to call when disabled.
func (r *Recorder) Begin(t time.Time) {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.startedAt = t
	r.mu.Unlock()
}

// SetCommand attaches the active leaf command's metadata. Called from the
// root PersistentPreRun once Cobra has resolved the leaf.
func (r *Recorder) SetCommand(info CommandInfo) {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.cmdInfo = info
	r.cmdSet = true
	r.mu.Unlock()
}

// Finalize categorizes err, builds the final Event, and enqueues it for
// dispatch. Non-blocking: if the channel is full, the event is dropped
// rather than risk blocking command shutdown.
func (r *Recorder) Finalize(_ error, end time.Time) {
	if r == nil || !r.enabled {
		return
	}

	r.mu.Lock()

	info := r.cmdInfo
	cmdSet := r.cmdSet
	r.mu.Unlock()

	if !cmdSet {
		// PersistentPreRun never fired — usually means flag parsing failed
		// before any subcommand resolved. Skip the event rather than emit
		// something with empty command_path.
		return
	}

	ev := r.staticEvent
	ev.Timestamp = end
	ev.CommandPath = info.Path
	ev.FlagsSet = info.FlagsSet
	ev.ExecArea = info.Area

	// TODO: Uncomment when we start reporting outcomes.
	// ev.Outcome = string(Categorize(err))
	// ev.Cancelled = err != nil && isCanceled(err)
	// start := r.startedAt
	// if !start.IsZero() {
	// 	ev.DurationMs = end.Sub(start).Milliseconds()
	// }

	select {
	case r.events <- ev:
	default:
		r.logger.Debug("telemetry: event channel full, dropping event")
	}
}

// Close drains the dispatcher with a bounded deadline so a slow sink can
// never wedge command shutdown. Always returns nil — telemetry is
// best-effort.
func (r *Recorder) Close(_ context.Context) error {
	if r == nil || !r.enabled {
		return nil
	}
	close(r.events)
	select {
	case <-r.done:
	case <-time.After(flushTimeout):
		r.logger.Debug("telemetry: flush deadline exceeded; abandoning")
	}
	return nil
}

func (r *Recorder) dispatch() {
	defer close(r.done)
	// The dispatcher outlives any request-scoped context: it must be able to
	// drain in-flight events even when the original command context has been
	// cancelled. Using a fresh background context here is intentional.
	ctx := context.Background()
	for ev := range r.events {
		if err := r.sink.Emit(ctx, ev); err != nil {
			r.logger.Debug("telemetry: sink emit failed", "error", err)
		}
	}
	if err := r.sink.Close(ctx); err != nil {
		r.logger.Debug("telemetry: sink close failed", "error", err)
	}
}

// TODO: Uncomment when we start reporting outcomes.
// func isCanceled(err error) bool {
// 	return Categorize(err) == OutcomeInterrupted
// }

func loggerOrDiscard(l *slog.Logger) *slog.Logger {
	if l != nil {
		return l
	}
	return slog.New(slog.NewTextHandler(discardWriter{}, nil))
}

type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) { return len(p), nil }
