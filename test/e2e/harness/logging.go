//go:build e2e

package harness

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// Log levels for harness logging and optional CLI propagation.
const (
	levelTrace = iota
	levelDebug
	levelInfo
	levelWarn
	levelError
)

var harnessLevel = func() int {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("KONGCTL_E2E_LOG_LEVEL")))
	switch v {
	case "trace":
		return levelTrace
	case "debug":
		return levelDebug
	case "warn":
		return levelWarn
	case "error":
		return levelError
	case "info":
		return levelInfo
	case "":
		// Default to warn to keep test output minimal by default
		return levelWarn
	default:
		return levelInfo
	}
}()

var (
	runLogFile *os.File
	runDirPath string
)

// initRunLogging configures logging to also tee into <runDir>/run.log
func initRunLogging(runDir string) {
	runDirPath = runDir
	// best-effort: ignore errors, continue logging to stderr
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(runDir+string(os.PathSeparator)+"run.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err == nil {
		runLogFile = f
	}
}

func logf(level string, format string, args ...any) {
	if !levelEnabled(level) {
		return
	}
	ts := time.Now().Format(time.RFC3339)
	msg := fmt.Sprintf("%s [e2e %s] "+format+"\n", append([]any{ts, level}, args...)...)
	// Always write to stderr
	_, _ = os.Stderr.WriteString(msg)
	// Optionally tee into run.log
	if runLogFile != nil {
		_, _ = runLogFile.WriteString(msg)
	}
}

func levelEnabled(l string) bool {
	switch strings.ToUpper(l) {
	case "TRACE":
		return harnessLevel <= levelTrace
	case "DEBUG":
		return harnessLevel <= levelDebug
	case "INFO":
		return harnessLevel <= levelInfo
	case "WARN":
		return harnessLevel <= levelWarn
	case "ERROR":
		return harnessLevel <= levelError
	default:
		return true
	}
}

func Debugf(format string, args ...any) { logf("DEBUG", format, args...) }
func Infof(format string, args ...any)  { logf("INFO", format, args...) }
func Warnf(format string, args ...any)  { logf("WARN", format, args...) }
func Errorf(format string, args ...any) { logf("ERROR", format, args...) }

// redactEnv formats key=value pairs redacting sensitive values.
func redactEnv(kv string) string {
	// Expect "KEY=VALUE"; do not fail if missing '='
	i := strings.IndexByte(kv, '=')
	if i <= 0 {
		return kv
	}
	key := kv[:i]
	val := kv[i+1:]
	upper := strings.ToUpper(key)
	if strings.Contains(upper, "TOKEN") || strings.Contains(upper, "PAT") || strings.Contains(upper, "PASSWORD") || strings.Contains(upper, "SECRET") {
		if len(val) > 0 {
			return key + "=***"
		}
	}
	return kv
}

// getHarnessLogLevel returns the current harness log level string for propagation to kongctl.
func getHarnessLogLevel() string {
	switch harnessLevel {
	case levelTrace:
		return "trace"
	case levelDebug:
		return "debug"
	case levelWarn:
		return "warn"
	case levelError:
		return "error"
	case levelInfo:
		fallthrough
	default:
		return "info"
	}
}

// getHarnessDefaultOutput returns the harness default output format (defaults to json).
func getHarnessDefaultOutput() string {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("KONGCTL_E2E_OUTPUT")))
	switch v {
	case "json", "yaml", "text":
		return v
	case "":
		return "json"
	default:
		// fallback to json for unknown values
		return "json"
	}
}

// jsonStrictEnabled controls whether RunJSON disallows unknown fields.
func jsonStrictEnabled() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("KONGCTL_E2E_JSON_STRICT")))
	switch v {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// captureEnabled indicates whether per-command files should be saved.
var captureEnabled = func() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("KONGCTL_E2E_CAPTURE")))
	// default enabled
	if v == "" {
		return true
	}
	switch v {
	case "0", "false", "off", "no":
		return false
	default:
		return true
	}
}()
