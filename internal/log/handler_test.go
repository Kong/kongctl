package log

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestDualHandlerMirrorsErrorsToSecondary(t *testing.T) {
	t.Cleanup(EnableErrorMirroring)
	EnableErrorMirroring()

	var primaryBuf bytes.Buffer
	var secondaryBuf bytes.Buffer

	primary := slog.NewTextHandler(&primaryBuf, &slog.HandlerOptions{Level: slog.LevelInfo})
	secondary := slog.NewTextHandler(&secondaryBuf, &slog.HandlerOptions{Level: slog.LevelError})
	logger := slog.New(NewDualHandler(primary, secondary))

	logger.Error("boom", slog.String("foo", "bar"))
	logger.Info("still going")

	if got := primaryBuf.String(); !strings.Contains(got, "boom") || !strings.Contains(got, "still going") {
		t.Fatalf("expected primary log to contain both messages, got %q", got)
	}

	if got := secondaryBuf.String(); !strings.Contains(got, "boom") {
		t.Fatalf("expected secondary log to contain error message, got %q", got)
	}

	if got := secondaryBuf.String(); strings.Contains(got, "still going") {
		t.Fatalf("secondary log should not contain info message, got %q", got)
	}
}

func TestDualHandlerCanDisableMirroring(t *testing.T) {
	t.Cleanup(EnableErrorMirroring)
	DisableErrorMirroring()

	var primaryBuf bytes.Buffer
	var secondaryBuf bytes.Buffer

	primary := slog.NewTextHandler(&primaryBuf, &slog.HandlerOptions{Level: slog.LevelInfo})
	secondary := slog.NewTextHandler(&secondaryBuf, &slog.HandlerOptions{Level: slog.LevelError})
	logger := slog.New(NewDualHandler(primary, secondary))

	logger.Error("boom")

	if got := primaryBuf.String(); !strings.Contains(got, "boom") {
		t.Fatalf("expected primary log to contain error message, got %q", got)
	}

	if got := secondaryBuf.String(); got != "" {
		t.Fatalf("expected secondary log to be empty when mirroring disabled, got %q", got)
	}
}
