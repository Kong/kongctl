package log

import "log/slog"

type Key struct{}

var LoggerKey = Key{}

// LevelTrace is a custom trace level for slog
// Using LevelDebug - 4 which equals -8
const LevelTrace = slog.LevelDebug - 4

func ConfigLevelStringToSlogLevel(level string) slog.Level {
	switch level {
	case "trace":
		return LevelTrace
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelError
	}
}
