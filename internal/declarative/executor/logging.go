package executor

import (
	"context"
	"log/slog"

	"github.com/kong/kongctl/internal/log"
)

func loggerFromContext(ctx context.Context) *slog.Logger {
	if ctx != nil {
		if logger, ok := ctx.Value(log.LoggerKey).(*slog.Logger); ok && logger != nil {
			return logger
		}
	}
	return slog.Default()
}
