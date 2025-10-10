package log

import (
	"context"
	"log/slog"
	"sync/atomic"
)

// mirrorErrors controls whether error level logs should also be mirrored to the
// secondary (typically STDERR) handler. It defaults to true so that errors are
// surfaced to the user unless explicitly disabled (e.g. for interactive TUIs).
var mirrorErrors atomic.Bool

func init() {
	mirrorErrors.Store(true)
}

// EnableErrorMirroring ensures error level logs are mirrored to the secondary
// handler when one is configured.
func EnableErrorMirroring() {
	mirrorErrors.Store(true)
}

// DisableErrorMirroring stops mirroring error level logs to the secondary
// handler. This is used for interactive commands where stderr output would
// disrupt the UI.
func DisableErrorMirroring() {
	mirrorErrors.Store(false)
}

// errorMirroringEnabled returns whether error logs should be mirrored to the
// secondary handler.
func errorMirroringEnabled() bool {
	return mirrorErrors.Load()
}

// NewDualHandler wraps a primary slog.Handler and, optionally, a secondary
// handler that only receives error level records when mirroring is enabled.
func NewDualHandler(primary slog.Handler, secondary slog.Handler) slog.Handler {
	return &dualHandler{
		primary:   primary,
		secondary: secondary,
	}
}

type dualHandler struct {
	primary   slog.Handler
	secondary slog.Handler
}

func (h *dualHandler) Enabled(ctx context.Context, level slog.Level) bool {
	if h.primary != nil && h.primary.Enabled(ctx, level) {
		return true
	}

	if !h.shouldMirror(level) {
		return false
	}

	return h.secondary != nil && h.secondary.Enabled(ctx, level)
}

func (h *dualHandler) Handle(ctx context.Context, record slog.Record) error {
	if h.primary != nil && h.primary.Enabled(ctx, record.Level) {
		if err := h.primary.Handle(ctx, record); err != nil {
			return err
		}
	}

	if !h.shouldMirror(record.Level) {
		return nil
	}

	if h.secondary != nil && h.secondary.Enabled(ctx, record.Level) {
		clone := record.Clone()
		if err := h.secondary.Handle(ctx, clone); err != nil {
			return err
		}
	}

	return nil
}

func (h *dualHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	var primary slog.Handler
	if h.primary != nil {
		primary = h.primary.WithAttrs(attrs)
	}

	var secondary slog.Handler
	if h.secondary != nil {
		secondary = h.secondary.WithAttrs(attrs)
	}

	return &dualHandler{
		primary:   primary,
		secondary: secondary,
	}
}

func (h *dualHandler) WithGroup(name string) slog.Handler {
	var primary slog.Handler
	if h.primary != nil {
		primary = h.primary.WithGroup(name)
	}

	var secondary slog.Handler
	if h.secondary != nil {
		secondary = h.secondary.WithGroup(name)
	}

	return &dualHandler{
		primary:   primary,
		secondary: secondary,
	}
}

func (h *dualHandler) shouldMirror(level slog.Level) bool {
	return h.secondary != nil && level >= slog.LevelError && errorMirroringEnabled()
}
