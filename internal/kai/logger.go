package kai

import (
	"context"
	"log/slog"

	applog "github.com/kong/kongctl/internal/log"
)

func loggerFromContext(ctx context.Context) *slog.Logger {
	if ctx == nil {
		return nil
	}
	if val := ctx.Value(applog.LoggerKey); val != nil {
		if logger, ok := val.(*slog.Logger); ok {
			return logger
		}
	}
	return nil
}

// ContextLogger exposes the slog logger stored in the provided context, if any.
func ContextLogger(ctx context.Context) *slog.Logger {
	return loggerFromContext(ctx)
}

func logDebug(ctx context.Context, msg string, attrs ...slog.Attr) {
	if logger := loggerFromContext(ctx); logger != nil {
		logger.LogAttrs(ctx, slog.LevelDebug, msg, attrs...)
	}
}

func logInfo(ctx context.Context, msg string, attrs ...slog.Attr) {
	if logger := loggerFromContext(ctx); logger != nil {
		logger.LogAttrs(ctx, slog.LevelInfo, msg, attrs...)
	}
}

func logError(ctx context.Context, msg string, attrs ...slog.Attr) {
	if logger := loggerFromContext(ctx); logger != nil {
		logger.LogAttrs(ctx, slog.LevelError, msg, attrs...)
	}
}

func logTrace(ctx context.Context, msg string, attrs ...slog.Attr) {
	if logger := loggerFromContext(ctx); logger != nil {
		logger.LogAttrs(ctx, applog.LevelTrace, msg, attrs...)
	}
}
