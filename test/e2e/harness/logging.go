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

var (
	// fileLogLevel controls what gets written to run.log (and propagated to kongctl).
	fileLogLevel = envLogLevel("KONGCTL_E2E_LOG_LEVEL", levelWarn)
	// consoleLogLevel controls what gets written to stderr; defaults to fileLogLevel for
	// backward compatibility.
	consoleLogLevel = envLogLevel("KONGCTL_E2E_CONSOLE_LOG_LEVEL", fileLogLevel)
	runLogFile      *os.File
	runDirPath      string
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
	lvl := levelValue(level)
	if lvl < 0 {
		return
	}
	writeFile := shouldWrite(lvl, fileLogLevel)
	writeConsole := shouldWrite(lvl, consoleLogLevel)
	if !writeFile && !writeConsole {
		return
	}
	ts := time.Now().Format(time.RFC3339)
	msg := fmt.Sprintf("%s [e2e %s] "+format+"\n", append([]any{ts, strings.ToUpper(level)}, args...)...)
	if writeConsole {
		_, _ = os.Stderr.WriteString(msg)
	}
	if writeFile {
		switch {
		case runLogFile != nil:
			_, _ = runLogFile.WriteString(msg)
		case !writeConsole:
			// Fallback to stderr if the run log is unavailable to avoid losing logs.
			_, _ = os.Stderr.WriteString(msg)
		}
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
	if strings.Contains(upper, "TOKEN") || strings.Contains(upper, "PAT") || strings.Contains(upper, "PASSWORD") ||
		strings.Contains(upper, "SECRET") {
		if len(val) > 0 {
			return key + "=***"
		}
	}
	return kv
}

// getHarnessLogLevel returns the current harness log level string for propagation to kongctl.
func getHarnessLogLevel() string {
	return logLevelString(fileLogLevel)
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

func envLogLevel(env string, emptyDefault int) int {
	v := os.Getenv(env)
	return parseLogLevel(v, emptyDefault)
}

func parseLogLevel(v string, emptyDefault int) int {
	switch strings.ToLower(strings.TrimSpace(v)) {
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
		return emptyDefault
	default:
		return levelInfo
	}
}

func shouldWrite(msgLevel, minLevel int) bool {
	return minLevel <= msgLevel
}

func levelValue(l string) int {
	switch strings.ToUpper(strings.TrimSpace(l)) {
	case "TRACE":
		return levelTrace
	case "DEBUG":
		return levelDebug
	case "INFO":
		return levelInfo
	case "WARN":
		return levelWarn
	case "ERROR":
		return levelError
	default:
		return -1
	}
}

func logLevelString(level int) string {
	switch level {
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
