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
	"github.com/kong/kongctl/internal/konnect/auth"
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

func TestDisplayLoginBannerInteractiveWritesOutput(t *testing.T) {
	streams, _, out, _ := iostreams.NewTestIOStreams()
	stubLoginTerminals(t, true, true)

	if err := displayLoginBanner(streams); err != nil {
		t.Fatalf("displayLoginBanner: %v", err)
	}

	output := out.String()
	if !containsBraillePattern(output) {
		t.Fatalf("expected banner output to contain braille glyphs, got:\n%s", output)
	}
}

func TestDisplayLoginBannerSkipsNonInteractive(t *testing.T) {
	tests := []struct {
		name      string
		inputTTY  bool
		outputTTY bool
	}{
		{
			name:      "stdin is not terminal",
			inputTTY:  false,
			outputTTY: true,
		},
		{
			name:      "stdout is not terminal",
			inputTTY:  true,
			outputTTY: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			streams, _, out, _ := iostreams.NewTestIOStreams()
			stubLoginTerminals(t, tt.inputTTY, tt.outputTTY)

			if err := displayLoginBanner(streams); err != nil {
				t.Fatalf("displayLoginBanner: %v", err)
			}
			if out.Len() != 0 {
				t.Fatalf("expected no banner output, got:\n%s", out.String())
			}
		})
	}
}

func TestDisplayUserInstructionsPlain(t *testing.T) {
	var out bytes.Buffer
	displayUserInstructions(&out, testDeviceCodeResponse(), false)

	output := out.String()
	for _, want := range []string{
		"Logging your CLI into Kong Konnect with the browser...",
		"To login, go to the following URL in your browser:",
		"https://global.api.konghq.com/device/complete",
		"Or copy this one-time code: ABCD-EFGH",
		"And open your browser to https://global.api.konghq.com/device",
		"(Code expires in 900 seconds)",
		"Waiting for user to Login...",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected plain instructions to contain %q\noutput:\n%s", want, output)
		}
	}
	if strings.Contains(output, "\x1b[") {
		t.Fatalf("plain instructions included ANSI escape sequences:\n%q", output)
	}
}

func TestDisplayUserInstructionsStyled(t *testing.T) {
	var out bytes.Buffer
	displayUserInstructions(&out, testDeviceCodeResponse(), true)

	output := out.String()
	for _, want := range []string{
		"\u203a",
		"Kong Konnect browser login",
		"Open this URL in your browser:",
		"https://global.api.konghq.com/device/complete",
		"Or copy this one-time code:",
		"ABCD-EFGH",
		"Then open:",
		"https://global.api.konghq.com/device",
		"Code expires in 900 seconds",
		"Waiting for authorization...",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected styled instructions to contain %q\noutput:\n%s", want, output)
		}
	}
	if !strings.Contains(output, "\x1b[") {
		t.Fatalf("styled instructions did not include ANSI escape sequences:\n%q", output)
	}
}

func TestDisplayLoginSuccessStyled(t *testing.T) {
	var out bytes.Buffer
	displayLoginSuccess(&out, true)

	output := out.String()
	for _, want := range []string{"\u2713", "User successfully authorized", "\x1b["} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected styled success to contain %q\noutput:\n%q", want, output)
		}
	}
}

func TestShouldStyleLoginOutputRequiresTTYAndHonorsNoColor(t *testing.T) {
	streams, _, _, _ := iostreams.NewTestIOStreams()

	unsetEnvForTest(t, "NO_COLOR")
	stubLoginTerminals(t, true, true)
	if !shouldStyleLoginOutput(streams) {
		t.Fatal("expected terminal output without NO_COLOR to use styled login output")
	}

	t.Setenv("NO_COLOR", "1")
	if shouldStyleLoginOutput(streams) {
		t.Fatal("expected NO_COLOR to disable styled login output")
	}
}

func TestShouldStyleLoginOutputSkipsNonTerminalStdout(t *testing.T) {
	streams, _, _, _ := iostreams.NewTestIOStreams()
	unsetEnvForTest(t, "NO_COLOR")
	stubLoginTerminals(t, true, false)

	if shouldStyleLoginOutput(streams) {
		t.Fatal("expected non-terminal stdout to disable styled login output")
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

func stubLoginTerminals(t *testing.T, inputTTY, outputTTY bool) {
	t.Helper()
	originalInput := loginInputIsTerminal
	originalOutput := loginOutputIsTerminal
	t.Cleanup(func() {
		loginInputIsTerminal = originalInput
		loginOutputIsTerminal = originalOutput
	})
	loginInputIsTerminal = func(io.Reader) bool { return inputTTY }
	loginOutputIsTerminal = func(io.Writer) bool { return outputTTY }
}

func containsBraillePattern(s string) bool {
	for _, r := range s {
		if r >= '\u2800' && r <= '\u28ff' {
			return true
		}
	}
	return false
}

func testDeviceCodeResponse() auth.DeviceCodeResponse {
	return auth.DeviceCodeResponse{
		UserCode:                "ABCD-EFGH",
		VerificationURI:         "https://global.api.konghq.com/device",
		VerificationURIComplete: "https://global.api.konghq.com/device/complete",
		ExpiresIn:               900,
	}
}

func unsetEnvForTest(t *testing.T, key string) {
	t.Helper()
	original, ok := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("unset %s: %v", key, err)
	}
	t.Cleanup(func() {
		if ok {
			if err := os.Setenv(key, original); err != nil {
				t.Fatalf("restore %s: %v", key, err)
			}
			return
		}
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("restore unset %s: %v", key, err)
		}
	})
}
