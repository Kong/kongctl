package ask

import (
	"bytes"
	"testing"

	cmdcommon "github.com/kong/kongctl/internal/cmd/common"
)

type fakeFDWriter struct {
	fd uintptr
}

func (f fakeFDWriter) Write(p []byte) (int, error) {
	return len(p), nil
}

func (f fakeFDWriter) Fd() uintptr {
	return f.fd
}

func TestShouldUseColor(t *testing.T) {
	t.Run("always", func(t *testing.T) {
		if !shouldUseColor(cmdcommon.ColorModeAlways, &bytes.Buffer{}) {
			t.Fatalf("expected color when mode is always")
		}
	})

	t.Run("never", func(t *testing.T) {
		if shouldUseColor(cmdcommon.ColorModeNever, &bytes.Buffer{}) {
			t.Fatalf("expected color disabled when mode is never")
		}
	})

	t.Run("auto terminal", func(t *testing.T) {
		original := terminalDetector
		terminalDetector = func(uintptr) bool { return true }
		t.Cleanup(func() { terminalDetector = original })

		if !shouldUseColor(cmdcommon.ColorModeAuto, fakeFDWriter{fd: 1}) {
			t.Fatalf("expected color when terminal is detected")
		}
	})

	t.Run("auto non-terminal", func(t *testing.T) {
		original := terminalDetector
		terminalDetector = func(uintptr) bool { return false }
		t.Cleanup(func() { terminalDetector = original })

		if shouldUseColor(cmdcommon.ColorModeAuto, &bytes.Buffer{}) {
			t.Fatalf("expected no color when terminal is not detected")
		}
	})

	t.Run("auto no color env", func(t *testing.T) {
		original := terminalDetector
		terminalDetector = func(uintptr) bool { return true }
		t.Cleanup(func() { terminalDetector = original })

		t.Setenv("NO_COLOR", "1")

		if shouldUseColor(cmdcommon.ColorModeAuto, fakeFDWriter{fd: 1}) {
			t.Fatalf("expected no color when NO_COLOR is set")
		}
	})
}
