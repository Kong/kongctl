// Package telemetry emits one best-effort event per kongctl command
// execution. The package is backend-agnostic: it owns a stable internal
// schema (Event), centralizes error categorization (Categorize), and routes
// events through a replaceable Sink.
//
// Telemetry is opt-out: enabled by default. Users opt out per-invocation
// with --no-telemetry, per-process with KONGCTL_NO_TELEMETRY=true or
// DO_NOT_TRACK=1, or persistently through the local telemetry preference file
// or telemetry.enabled=false in their profile config.
package telemetry

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
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
	// EnvNoTelemetry is the profile-agnostic kongctl kill switch, named to
	// mirror the --no-telemetry CLI flag. It is one-way: only "true"
	// (case-insensitive) disables telemetry. Any other value — including
	// "false", unset, or garbage — falls through to config.
	EnvNoTelemetry = "KONGCTL_NO_TELEMETRY"
	// EnvDoNotTrack is the cross-vendor hard kill switch. Highest priority.
	// Only the spec value "1" disables telemetry
	EnvDoNotTrack = "DO_NOT_TRACK"
)

// PreferenceFileName is the profile-agnostic, per-device telemetry opt-in/out
// file stored beside kongctl's config.yaml.
const PreferenceFileName = ".telemetry-enabled"

// resolveEnabled returns whether telemetry should be active for this run.
// Precedence (highest to lowest):
//  1. forceDisabled — the --no-telemetry CLI flag. Per-invocation kill switch
//     so a user can opt out of a single command without touching env vars or
//     config.
//  2. DO_NOT_TRACK=1 — cross-vendor hard kill switch.
//  3. KONGCTL_NO_TELEMETRY=true — profile-agnostic kill switch. Only "true"
//     is honored; any other value falls through to config.
//  4. local preference file — profile-agnostic per-device preference written
//     from the interactive login disclosure.
//  5. config — itself honors KONGCTL_<PROFILE>_TELEMETRY_ENABLED via viper.
//     Default is true (opt-out).
//
// Opt-out is the project stance: the only way to land here with telemetry off
// is an explicit opt-out signal. Absence of config (cfg==nil) or absence of a
// telemetry key in config is treated as "no opt-out", i.e. enabled.
func resolveEnabled(cfg config.Hook, forceDisabled bool, logger *slog.Logger) bool {
	if forceDisabled {
		return false
	}
	if os.Getenv(EnvDoNotTrack) == "1" {
		return false
	}
	if disabled, ok := envBool(EnvNoTelemetry); ok && disabled {
		return false
	}
	if cfg == nil {
		return true
	}
	if preference, ok := ReadPreference(cfg, logger); ok {
		logPreferenceConfigConflict(cfg, preference, logger)
		return preference
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

func PreferenceFilePath(cfg config.Hook) (string, error) {
	if cfg == nil {
		return "", fmt.Errorf("telemetry preference path requires config")
	}
	configPath := strings.TrimSpace(cfg.GetPath())
	if configPath == "" {
		return "", fmt.Errorf("telemetry preference path requires config path")
	}
	return filepath.Join(filepath.Dir(configPath), PreferenceFileName), nil
}

func PreferenceFileExists(cfg config.Hook) bool {
	path, err := PreferenceFilePath(cfg)
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}

func ReadPreference(cfg config.Hook, logger *slog.Logger) (bool, bool) {
	path, err := PreferenceFilePath(cfg)
	if err != nil {
		return false, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			loggerOrDiscard(logger).Warn("telemetry: failed to read preference file", "path", path, "error", err)
			return false, true
		}
		return false, false
	}
	value := strings.TrimSpace(string(data))
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		loggerOrDiscard(logger).Warn("telemetry: ignoring invalid preference file", "path", path, "value", value)
		return false, true
	}
	return parsed, true
}

func WritePreference(cfg config.Hook, enabled bool) error {
	path, err := PreferenceFilePath(cfg)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strconv.FormatBool(enabled)+"\n"), 0o600)
}

func logPreferenceConfigConflict(cfg config.Hook, preference bool, logger *slog.Logger) {
	if !cfg.InConfig(ConfigKeyEnabled) || cfg.GetBool(ConfigKeyEnabled) == preference {
		return
	}
	loggerOrDiscard(logger).Warn(
		"telemetry: local preference file overrides telemetry.enabled config",
		"preference", preference,
		"config", cfg.GetBool(ConfigKeyEnabled),
	)
}

// telemetryLogFileName is the file in the kongctl config logs directory that
// the default fileSink appends JSONL events to.
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
}

var skippedCommandPrefixes = map[string]struct{}{
	"version":          {},
	"completion":       {},
	"__complete":       {},
	"__completeNoDesc": {},
	"help":             {},
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

	mu      sync.Mutex
	cmdInfo CommandInfo
	cmdSet  bool

	sink    Sink
	events  chan Event
	done    chan struct{}
	stopped bool
}

// NewRecorder builds a Recorder. It reads telemetry.enabled from cfg; if
// false (or forceDisabled is true), it returns a Recorder whose dispatch
// path is a no-op. forceDisabled is the per-invocation kill switch carried
// by the --no-telemetry CLI flag.
func NewRecorder(
	_ context.Context,
	cfg config.Hook,
	bi *build.Info,
	_ *iostreams.IOStreams,
	logger *slog.Logger,
	forceDisabled bool,
) *Recorder {
	logger = loggerOrDiscard(logger)

	if !resolveEnabled(cfg, forceDisabled, logger) {
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
	if cfg != nil && cfg.GetBool(ConfigKeyDebug) {
		// telemetry.debug routes events to a local JSONL file only, skipping
		// the UDP sink so developer runs don't pollute the Splunk dataset.
		path := filepath.Join(filepath.Dir(cfg.GetPath()), "logs", telemetryLogFileName)
		return NewFileSink(path)
	}

	return NewUDPSink(reportsAddr)
}

func (r *Recorder) Enabled() bool {
	if r == nil {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.enabled
}

func (r *Recorder) Disable(ctx context.Context) error {
	if r == nil {
		return nil
	}
	events, done := r.stop()
	if events == nil {
		return nil
	}
	close(events)
	return r.waitDone(ctx, done)
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
	if info.Path == "" || shouldSkipCommand(info.Path) {
		return
	}
	r.mu.Lock()
	r.cmdInfo = info
	r.cmdSet = true
	r.mu.Unlock()
}

func shouldSkipCommand(path string) bool {
	firstSegment, _, _ := strings.Cut(path, " ")
	_, skip := skippedCommandPrefixes[firstSegment]
	return skip
}

// trimBinaryPrefix strips the leading "kongctl" / "kongctl " from a cobra
// CommandPath() so the wire format stays binary-name-free. A bare
// "kongctl" (no subcommand) collapses to the empty string.
func trimBinaryPrefix(path string) string {
	path = strings.TrimSpace(path)
	if path == meta.CLIName {
		return ""
	}
	// Require the trailing space so extension binary names such as
	// "kongctl-extension" are left untouched.
	return strings.TrimPrefix(path, meta.CLIName+" ")
}

// Finalize builds the final Event and enqueues it for dispatch.
// Non-blocking: if the channel is full, the event is dropped rather than
// risk blocking command shutdown.
func (r *Recorder) Finalize(end time.Time) {
	if r == nil {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.enabled || r.stopped {
		return
	}

	info := r.cmdInfo
	cmdSet := r.cmdSet

	if !cmdSet {
		// PersistentPreRun never fired — usually means flag parsing failed
		// before any subcommand resolved. Skip the event rather than emit
		// something with empty command_path.
		return
	}

	ev := r.staticEvent
	ev.Timestamp = end
	ev.CommandPath = info.Path

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
	if r == nil {
		return nil
	}
	events, done := r.stop()
	if events == nil {
		return nil
	}
	close(events)
	return r.waitDone(context.Background(), done)
}

func (r *Recorder) stop() (chan Event, chan struct{}) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.enabled || r.stopped {
		r.enabled = false
		return nil, nil
	}
	r.enabled = false
	r.stopped = true
	events := r.events
	done := r.done
	return events, done
}

func (r *Recorder) waitDone(ctx context.Context, done chan struct{}) error {
	if done == nil {
		return nil
	}
	timer := time.NewTimer(flushTimeout)
	defer timer.Stop()
	select {
	case <-done:
	case <-timer.C:
		r.logger.Debug("telemetry: flush deadline exceeded; abandoning")
	case <-ctx.Done():
		r.logger.Debug("telemetry: flush cancelled", "error", ctx.Err())
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

func loggerOrDiscard(l *slog.Logger) *slog.Logger {
	if l != nil {
		return l
	}
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
