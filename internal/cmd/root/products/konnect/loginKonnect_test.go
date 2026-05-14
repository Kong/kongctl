package konnect

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kong/kongctl/internal/iostreams"
	"github.com/kong/kongctl/internal/telemetry"
	"github.com/spf13/pflag"
)

type telemetryPromptConfig struct {
	bools map[string]bool
	path  string
}

func (c telemetryPromptConfig) GetString(string) string               { return "" }
func (c telemetryPromptConfig) GetBool(k string) bool                 { return c.bools[k] }
func (c telemetryPromptConfig) GetInt(string) int                     { return 0 }
func (c telemetryPromptConfig) GetIntOrElse(_ string, orElse int) int { return orElse }
func (c telemetryPromptConfig) GetStringSlice(string) []string        { return nil }
func (c telemetryPromptConfig) SetString(string, string)              {}
func (c telemetryPromptConfig) Set(string, any)                       {}
func (c telemetryPromptConfig) Get(string) any                        { return nil }
func (c telemetryPromptConfig) BindFlag(string, *pflag.Flag) error    { return nil }
func (c telemetryPromptConfig) GetProfile() string                    { return "default" }
func (c telemetryPromptConfig) GetPath() string                       { return c.path }

func TestHandleTelemetryPreference_OptOutWritesFileAndDisablesRecorder(t *testing.T) {
	rec, streams, cfg, out := newTelemetryPreferencePromptTest(t, "n\n")

	handleTelemetryPreference(streams, cfg, rec)

	data, err := os.ReadFile(filepath.Join(filepath.Dir(cfg.GetPath()), telemetry.PreferenceFileName))
	if err != nil {
		t.Fatalf("read preference: %v", err)
	}
	if got, want := string(data), "false\n"; got != want {
		t.Fatalf("preference = %q, want %q", got, want)
	}
	if rec.Enabled() {
		t.Fatal("recorder still enabled after user opted out")
	}
	if !strings.Contains(out.String(), "Allow kongctl to collect usage data on this device? [Y/n]:") {
		t.Fatalf("prompt missing from output:\n%s", out.String())
	}
}

func TestHandleTelemetryPreference_EnterWritesEnabled(t *testing.T) {
	rec, streams, cfg, _ := newTelemetryPreferencePromptTest(t, "\n")
	t.Cleanup(func() {
		_ = rec.Close(t.Context())
	})

	handleTelemetryPreference(streams, cfg, rec)

	data, err := os.ReadFile(filepath.Join(filepath.Dir(cfg.GetPath()), telemetry.PreferenceFileName))
	if err != nil {
		t.Fatalf("read preference: %v", err)
	}
	if got, want := string(data), "true\n"; got != want {
		t.Fatalf("preference = %q, want %q", got, want)
	}
	if !rec.Enabled() {
		t.Fatal("recorder disabled after user accepted telemetry")
	}
}

func TestHandleTelemetryPreference_PreferenceFileSkipsPrompt(t *testing.T) {
	rec, streams, cfg, out := newTelemetryPreferencePromptTest(t, "n\n")
	t.Cleanup(func() {
		_ = rec.Close(t.Context())
	})
	if err := telemetry.WritePreference(cfg, true); err != nil {
		t.Fatalf("WritePreference: %v", err)
	}

	handleTelemetryPreference(streams, cfg, rec)

	if out.Len() != 0 {
		t.Fatalf("output = %q, want no prompt", out.String())
	}
}

func TestHandleTelemetryPreference_NonInteractiveWritesDisclosureOnly(t *testing.T) {
	rec, streams, cfg, out := newTelemetryPreferencePromptTest(t, "")
	t.Cleanup(func() {
		_ = rec.Close(t.Context())
	})
	loginInputIsTerminal = func(io.Reader) bool { return false }

	handleTelemetryPreference(streams, cfg, rec)

	if telemetry.PreferenceFileExists(cfg) {
		t.Fatal("preference file written for non-interactive input")
	}
	output := out.String()
	if !strings.Contains(output, "kongctl collects limited usage data") {
		t.Fatalf("disclosure missing from output:\n%s", output)
	}
	if strings.Contains(output, "Allow kongctl to collect usage data on this device?") {
		t.Fatalf("non-interactive output included prompt:\n%s", output)
	}
}

func TestReadTelemetryPreferenceAnswerInvalidTwiceSkipsWrite(t *testing.T) {
	var out bytes.Buffer
	_, ok := readTelemetryPreferenceAnswer(strings.NewReader("maybe\nstill maybe\n"), &out)
	if ok {
		t.Fatal("ok = true, want false after invalid answers")
	}
	if !strings.Contains(out.String(), "Please answer y or n:") {
		t.Fatalf("retry prompt missing from output: %q", out.String())
	}
}

func newTelemetryPreferencePromptTest(
	t *testing.T,
	input string,
) (*telemetry.Recorder, *iostreams.IOStreams, telemetryPromptConfig, *bytes.Buffer) {
	t.Helper()
	originalIsTerminal := loginInputIsTerminal
	t.Cleanup(func() {
		loginInputIsTerminal = originalIsTerminal
	})
	loginInputIsTerminal = func(io.Reader) bool { return true }

	dir := t.TempDir()
	cfg := telemetryPromptConfig{
		bools: map[string]bool{telemetry.ConfigKeyEnabled: true},
		path:  filepath.Join(dir, "config.yaml"),
	}
	in := bytes.NewBufferString(input)
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	streams := &iostreams.IOStreams{
		In:     in,
		Out:    out,
		ErrOut: errOut,
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	rec := telemetry.NewRecorder(context.Background(), cfg, nil, streams, logger, false)
	return rec, streams, cfg, out
}
