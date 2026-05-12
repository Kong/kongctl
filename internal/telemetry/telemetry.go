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
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/kong/kongctl/internal/build"
	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/iostreams"
	"github.com/kong/kongctl/internal/meta"
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

// Environment variables that override profile config for telemetry.
// Checked at the top of NewRecorder so users can toggle without needing to
// know which profile is active.
const (
	// EnvTelemetryEnabled is the profile-agnostic kongctl override. Accepts
	// only "true" or "false" (case-insensitive) to keep the toggle
	// unambiguous
	EnvTelemetryEnabled = "KONGCTL_TELEMETRY_ENABLED"
	// EnvDoNotTrack is the cross-vendor hard kill switch. Highest priority.
	// Only the spec value "1" disables telemetry
	EnvDoNotTrack = "DO_NOT_TRACK"
)

// resolveEnabled returns whether telemetry should be active for this run.
// Precedence: DO_NOT_TRACK=1 → KONGCTL_TELEMETRY_ENABLED → config (which
// itself honors KONGCTL_<PROFILE>_TELEMETRY_ENABLED via viper).
func resolveEnabled(cfg config.Hook) bool {
	if os.Getenv(EnvDoNotTrack) == "1" {
		return false
	}
	if v, ok := envBool(EnvTelemetryEnabled); ok {
		return v
	}
	if cfg == nil {
		return false
	}
	return cfg.GetBool(ConfigKeyEnabled)
}

// envBool reads name and reports whether it was set to a recognized boolean.
// Only "true" and "false" (case-insensitive) are recognized.
func envBool(name string) (val, ok bool) {
	switch strings.ToLower(os.Getenv(name)) {
	case "true":
		return true, true
	case "false":
		return false, true
	}
	return false, false
}

// telemetryLogFileName is the file in the kongctl config directory that the
// default fileSink appends JSONL events to.
const telemetryLogFileName = "telemetry.log"

// flushTimeout caps how long Close blocks the shutdown path.
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
	Path string
	Area string
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
// false, it returns a Recorder whose dispatch path is a no-op.
func NewRecorder(
	_ context.Context,
	cfg config.Hook,
	bi *build.Info,
	_ *iostreams.IOStreams,
	logger *slog.Logger,
) *Recorder {
	logger = loggerOrDiscard(logger)

	if !resolveEnabled(cfg) {
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
	// telemetry.debug routes events to a local JSONL file only, skipping the
	// UDP sink so developer runs don't pollute the Splunk dataset.
	if cfg.GetBool(ConfigKeyDebug) {
		path := filepath.Join(filepath.Dir(cfg.GetPath()), telemetryLogFileName)
		return NewFileSink(path)
	}

	return NewUDPSink(reportsAddr)
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
// root PersistentPreRun once Cobra has resolved the leaf. The binary name
// is stripped from info.Path so emitted events carry e.g. "get apis"
// rather than "kongctl get apis" — every event is a kongctl invocation by
// definition, so the prefix is redundant on the wire.
//
// A bare "kongctl" invocation (no subcommand) trims to "" and is treated
// as no-command: cmdSet stays false and Finalize will skip the event.
func (r *Recorder) SetCommand(info CommandInfo) {
	if r == nil {
		return
	}
	info.Path = trimBinaryPrefix(info.Path)
	if info.Path == "" {
		return
	}
	r.mu.Lock()
	r.cmdInfo = info
	r.cmdSet = true
	r.mu.Unlock()
}

// trimBinaryPrefix strips the leading "kongctl" / "kongctl " from a cobra
// CommandPath() so the wire format stays binary-name-free. A bare
// "kongctl" (no subcommand) collapses to the empty string.
func trimBinaryPrefix(path string) string {
	path = strings.TrimSpace(path)
	if path == meta.CLIName {
		return ""
	}
	return strings.TrimPrefix(path, meta.CLIName+" ")
}

// Finalize builds the final Event, and enqueues it for
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
	// cancelled. Using a fresh background context as the parent is intentional.
	//
	// Each sink call gets its own bounded child context so a slow or stuck
	// transport (e.g. a future network sink against an unreachable host) cannot
	// keep this goroutine alive past process exit after Close gives up at
	// flushTimeout. Sink implementations are required to honor ctx; see the
	// Sink contract in sink.go.
	for ev := range r.events {
		emitCtx, cancel := context.WithTimeout(context.Background(), flushTimeout)
		if err := r.sink.Emit(emitCtx, ev); err != nil {
			r.logger.Debug("telemetry: sink emit failed", "error", err)
		}
		cancel()
	}
	closeCtx, cancel := context.WithTimeout(context.Background(), flushTimeout)
	defer cancel()
	if err := r.sink.Close(closeCtx); err != nil {
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
