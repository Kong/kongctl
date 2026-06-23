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
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/kong/kongctl/internal/art"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	roarcmd "github.com/kong/kongctl/internal/cmd/root/verbs/roar"
	"github.com/kong/kongctl/internal/iostreams"
	"github.com/kong/kongctl/internal/konnect/auth"
	"github.com/kong/kongctl/internal/telemetry"
	"github.com/mattn/go-runewidth"
	"github.com/spf13/cobra"
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

func TestDisplayStaticLoginBannerInteractiveWritesOutput(t *testing.T) {
	streams, _, out, _ := iostreams.NewTestIOStreams()
	stubLoginTerminals(t, true, true)
	stubLoginTerminalData(t, roarcmd.NewTerminalCapabilities(
		art.KongRoarAnimationWidth,
		art.KongRoarAnimationHeight,
		true,
	))

	if err := displayStaticLoginBanner(streams); err != nil {
		t.Fatalf("displayStaticLoginBanner: %v", err)
	}

	output := out.String()
	if strings.TrimSpace(output) == "" {
		t.Fatal("expected banner output")
	}
	if containsBraillePattern(output) {
		t.Fatalf("expected login banner to use static animation frame, got climber braille output:\n%s", output)
	}
	if got := maxLineWidth(output); got != art.KongRoarAnimationWidth {
		t.Fatalf("login banner width = %d, want %d\noutput:\n%s", got, art.KongRoarAnimationWidth, output)
	}
	if got := lineCount(output); got != art.KongRoarAnimationHeight {
		t.Fatalf("login banner height = %d, want %d\noutput:\n%s", got, art.KongRoarAnimationHeight, output)
	}
}

func TestDisplayStaticLoginBannerFallsBackToClimberInNarrowTerminal(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("LC_ALL", "en_US.UTF-8")
	streams, _, out, _ := iostreams.NewTestIOStreams()
	stubLoginTerminals(t, true, true)
	stubLoginTerminalData(t, roarcmd.NewTerminalCapabilities(79, art.KongRoarAnimationHeight, true))

	if err := displayStaticLoginBanner(streams); err != nil {
		t.Fatalf("displayStaticLoginBanner: %v", err)
	}
	output := out.String()
	if !containsBraillePattern(output) {
		t.Fatalf("expected narrow terminal to use climber braille fallback, got:\n%s", output)
	}
	if got := maxLineWidth(output); got != 48 {
		t.Fatalf("login banner width = %d, want 48\noutput:\n%s", got, output)
	}
}

func TestDisplayStaticLoginBannerSkipsTooNarrowTerminal(t *testing.T) {
	streams, _, out, _ := iostreams.NewTestIOStreams()
	stubLoginTerminals(t, true, true)
	stubLoginTerminalData(t, roarcmd.NewTerminalCapabilities(47, art.KongRoarAnimationHeight, true))

	if err := displayStaticLoginBanner(streams); err != nil {
		t.Fatalf("displayStaticLoginBanner: %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("expected no banner output for too narrow terminal, got:\n%s", out.String())
	}
}

func TestDisplayLoginBannerSkipsNoImage(t *testing.T) {
	streams, _, out, _ := iostreams.NewTestIOStreams()
	stubLoginTerminals(t, true, true)
	stubLoginTerminalData(t, roarcmd.NewTerminalCapabilities(
		art.KongRoarAnimationWidth,
		art.KongRoarAnimationHeight,
		true,
	))

	if err := displayLoginBanner(streams, true); err != nil {
		t.Fatalf("displayLoginBanner: %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("expected no banner output with no-image, got:\n%s", out.String())
	}
}

func TestDisplayStaticLoginBannerSkipsNonInteractive(t *testing.T) {
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

			if err := displayStaticLoginBanner(streams); err != nil {
				t.Fatalf("displayStaticLoginBanner: %v", err)
			}
			if out.Len() != 0 {
				t.Fatalf("expected no banner output, got:\n%s", out.String())
			}
		})
	}
}

func TestShouldAnimateLoginBannerHonorsNoImage(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("LC_ALL", "en_US.UTF-8")
	streams := iostreams.NewTestIOStreamsOnly()
	stubLoginTerminals(t, true, true)
	stubLoginTerminalData(t, roarcmd.NewTerminalCapabilities(
		art.KongRoarAnimationWidth,
		art.KongRoarAnimationHeight,
		true,
	))

	if !shouldAnimateLoginBanner(streams, false, false) {
		t.Fatal("expected supported terminal to animate login banner")
	}
	if shouldAnimateLoginBanner(streams, false, true) {
		t.Fatal("expected no-image to disable login banner animation")
	}
}

func TestLoginAnimationModelViewIncludesFrameAndInstructions(t *testing.T) {
	model := newLoginAnimationModel(
		t.Context(),
		[]string{"frame-1\n", "frame-2\n"},
		"instructions",
		time.Second,
		time.Now().Add(time.Minute),
		func(context.Context) (*auth.AccessToken, error) {
			return nil, &auth.DAGError{ErrorCode: auth.AuthorizationPendingErrorCode}
		},
	)

	output := model.View().Content
	if !strings.Contains(output, "frame-1") {
		t.Fatalf("expected view to include current frame:\n%s", output)
	}
	if !strings.Contains(output, "instructions") {
		t.Fatalf("expected view to include instructions:\n%s", output)
	}
	if model.View().AltScreen {
		t.Fatal("expected login animation to use the normal terminal buffer")
	}
}

func TestLoginAnimationModelStopsAnimatingAfterTwoLoops(t *testing.T) {
	model := newLoginAnimationModel(
		t.Context(),
		[]string{"first", "second"},
		"instructions",
		time.Second,
		time.Now().Add(time.Minute),
		nil,
	)

	nextModel, cmd := model.Update(loginAnimationTickMsg(time.Now()))
	advanced := nextModel.(loginAnimationModel)
	if advanced.frame != 1 {
		t.Fatalf("frame = %d, want 1", advanced.frame)
	}
	if cmd == nil {
		t.Fatal("expected second frame tick to schedule next tick")
	}

	nextModel, cmd = advanced.Update(loginAnimationTickMsg(time.Now()))
	advanced = nextModel.(loginAnimationModel)
	if advanced.frame != 2 {
		t.Fatalf("frame = %d, want 2", advanced.frame)
	}
	if cmd == nil {
		t.Fatal("expected third frame tick to schedule next tick")
	}

	nextModel, cmd = advanced.Update(loginAnimationTickMsg(time.Now()))
	advanced = nextModel.(loginAnimationModel)
	if advanced.frame != 3 {
		t.Fatalf("frame = %d, want 3", advanced.frame)
	}
	if cmd == nil {
		t.Fatal("expected fourth frame tick to schedule final tick")
	}

	nextModel, cmd = advanced.Update(loginAnimationTickMsg(time.Now()))
	stopped := nextModel.(loginAnimationModel)
	if stopped.frame != 3 {
		t.Fatalf("frame = %d, want animation to stop at final frame 3", stopped.frame)
	}
	if cmd != nil {
		t.Fatal("expected animation to stop after two loops")
	}
}

func TestLoginAnimationModelQuitsOnAuthorizedToken(t *testing.T) {
	model := newLoginAnimationModel(
		t.Context(),
		[]string{"frame"},
		"instructions",
		time.Second,
		time.Now().Add(time.Minute),
		nil,
	)
	token := &auth.AccessToken{Token: &auth.AccessTokenResponse{AuthToken: "token"}}

	nextModel, cmd := model.Update(loginPollMsg{token: token})
	authorized := nextModel.(loginAnimationModel)
	if authorized.token != token {
		t.Fatal("expected authorized token to be stored on model")
	}
	if cmd == nil {
		t.Fatal("expected authorized poll to quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("quit command message type = %T, want tea.QuitMsg", cmd())
	}
}

func TestLoginAnimationModelQuitsOnCtrlC(t *testing.T) {
	model := newLoginAnimationModel(t.Context(), []string{"frame"}, "", time.Second, time.Now().Add(time.Minute), nil)

	nextModel, cmd := model.Update(tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl}))
	cancelled := nextModel.(loginAnimationModel)
	if !errors.Is(cancelled.err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", cancelled.err)
	}
	if cmd == nil {
		t.Fatal("expected ctrl-c to quit")
	}
}

func TestWaitForDeviceAuthorizationExpiresWhilePending(t *testing.T) {
	resp := testDeviceCodeResponse()
	resp.ExpiresIn = -1
	resp.Interval = 0

	_, err := waitForDeviceAuthorization(t.Context(), resp, func(context.Context) (*auth.AccessToken, error) {
		return nil, &auth.DAGError{ErrorCode: auth.AuthorizationPendingErrorCode}
	})
	if !errors.Is(err, errDeviceAuthorizationExpired) {
		t.Fatalf("err = %v, want errDeviceAuthorizationExpired", err)
	}
}

func TestLoginKonnectNoAnimateFlag(t *testing.T) {
	command := &cobra.Command{Use: "login"}
	addParentFlags := func(verbs.VerbValue, *cobra.Command) {}
	parentPreRun := func(*cobra.Command, []string) error {
		return nil
	}
	loginCmd := newLoginKonnectCmd(verbs.Login, command, addParentFlags, parentPreRun)

	flag := loginCmd.Flags().Lookup(loginNoAnimateFlagName)
	if flag == nil {
		t.Fatal("expected no-animate flag")
	}
	if !strings.Contains(flag.Usage, "static login banner") {
		t.Fatalf("expected no-animate usage to mention static login banner, got %q", flag.Usage)
	}
}

func TestLoginKonnectNoImageFlag(t *testing.T) {
	command := &cobra.Command{Use: "login"}
	addParentFlags := func(verbs.VerbValue, *cobra.Command) {}
	parentPreRun := func(*cobra.Command, []string) error {
		return nil
	}
	loginCmd := newLoginKonnectCmd(verbs.Login, command, addParentFlags, parentPreRun)

	flag := loginCmd.Flags().Lookup(loginNoImageFlagName)
	if flag == nil {
		t.Fatal("expected no-image flag")
	}
	if !strings.Contains(flag.Usage, "only login text") {
		t.Fatalf("expected no-image usage to mention login text, got %q", flag.Usage)
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
	streams := iostreams.NewTestIOStreamsOnly()

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
	streams := iostreams.NewTestIOStreamsOnly()
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

func stubLoginTerminalData(t *testing.T, terminal roarcmd.TerminalCapabilities) {
	t.Helper()
	original := loginTerminalData
	t.Cleanup(func() {
		loginTerminalData = original
	})
	loginTerminalData = func(io.Writer) roarcmd.TerminalCapabilities {
		return terminal
	}
}

func containsBraillePattern(s string) bool {
	for _, r := range s {
		if r >= '\u2800' && r <= '\u28ff' {
			return true
		}
	}
	return false
}

func maxLineWidth(value string) int {
	maxWidth := 0
	for line := range strings.Lines(value) {
		maxWidth = max(maxWidth, runewidth.StringWidth(strings.TrimSuffix(line, "\n")))
	}
	return maxWidth
}

func lineCount(value string) int {
	count := 0
	for range strings.Lines(value) {
		count++
	}
	return count
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
