package konnect

import (
	"bytes"
	"context"
	"errors"
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
func (c telemetryPromptConfig) InConfig(string) bool                  { return false }

func TestHandleTelemetryPreference_OptOutWritesFileAndDisablesRecorder(t *testing.T) {
	rec, streams, cfg, out := newTelemetryPreferencePromptTest(t, "n\n")

	if err := handleTelemetryPreference(t.Context(), streams, cfg, rec); err != nil {
		t.Fatalf("handleTelemetryPreference: %v", err)
	}

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

	if err := handleTelemetryPreference(t.Context(), streams, cfg, rec); err != nil {
		t.Fatalf("handleTelemetryPreference: %v", err)
	}

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

	if err := handleTelemetryPreference(t.Context(), streams, cfg, rec); err != nil {
		t.Fatalf("handleTelemetryPreference: %v", err)
	}

	if out.Len() != 0 {
		t.Fatalf("output = %q, want no prompt", out.String())
	}
}

func TestHandleTelemetryPreference_NonInteractiveWritesDisclosureOnly(t *testing.T) {
	rec, streams, cfg, out := newTelemetryPreferencePromptTest(t, "")
	errOut := streams.ErrOut.(*bytes.Buffer)
	t.Cleanup(func() {
		_ = rec.Close(t.Context())
	})
	loginInputIsTerminal = func(io.Reader) bool { return false }

	if err := handleTelemetryPreference(t.Context(), streams, cfg, rec); err != nil {
		t.Fatalf("handleTelemetryPreference: %v", err)
	}

	if telemetry.PreferenceFileExists(cfg) {
		t.Fatal("preference file written for non-interactive input")
	}
	if out.Len() != 0 {
		t.Fatalf("stdout = %q, want disclosure on stderr", out.String())
	}
	output := errOut.String()
	if !strings.Contains(output, "kongctl collects limited usage data") {
		t.Fatalf("disclosure missing from output:\n%s", output)
	}
	if strings.Contains(output, "Allow kongctl to collect usage data on this device?") {
		t.Fatalf("non-interactive output included prompt:\n%s", output)
	}
}

func TestReadTelemetryPreferenceAnswerInvalidTwiceSkipsWrite(t *testing.T) {
	var out bytes.Buffer
	_, ok, err := readTelemetryPreferenceAnswer(t.Context(), strings.NewReader("maybe\nstill maybe\n"), &out, nil)
	if err != nil {
		t.Fatalf("readTelemetryPreferenceAnswer: %v", err)
	}
	if ok {
		t.Fatal("ok = true, want false after invalid answers")
	}
	if !strings.Contains(out.String(), "Please answer y or n:") {
		t.Fatalf("retry prompt missing from output: %q", out.String())
	}
}

func TestReadTelemetryPreferenceAnswerInterruptCancels(t *testing.T) {
	reader, writer := io.Pipe()
	t.Cleanup(func() {
		_ = reader.Close()
		_ = writer.Close()
	})
	interrupt := make(chan os.Signal, 1)
	interrupt <- os.Interrupt

	var out bytes.Buffer
	_, ok, err := readTelemetryPreferenceAnswer(t.Context(), reader, &out, interrupt)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if ok {
		t.Fatal("ok = true, want false after interrupt")
	}
}

func newTelemetryPreferencePromptTest(
	t *testing.T,
	input string,
) (*telemetry.Recorder, *iostreams.IOStreams, telemetryPromptConfig, *bytes.Buffer) {
	t.Helper()
	t.Setenv(telemetry.EnvDoNotTrack, "0")
	t.Setenv(telemetry.EnvNoTelemetry, "false")

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
