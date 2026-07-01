package konnect

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	roarcmd "github.com/kong/kongctl/internal/cmd/root/verbs/roar"
	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/iostreams"
	"github.com/kong/kongctl/internal/konnect/auth"
	"github.com/kong/kongctl/internal/konnect/httpclient"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/telemetry"
	"github.com/kong/kongctl/internal/theme"
	"github.com/kong/kongctl/internal/util"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

var (
	loginKonnectShort = i18n.T("root.products.konnect.loginKonnectShort", "Login to Konnect")
	loginKonnectLong  = i18n.T("root.products.konnect.loginKonnectLong",
		"Initiate a login to Konnect using the browser based machine code authorization flow.")
	loginKonnectExample = normalizers.Examples(
		i18n.T("root.products.konnect.loginKonnectExample",
			fmt.Sprintf(`
# Login to Konnect
%[1]s login konnect`, meta.CLIName)),
	)

	httpClient *http.Client

	loginInputIsTerminal  = isTerminalReader
	loginOutputIsTerminal = isTerminalWriter
	loginTerminalData     = roarcmd.DetectTerminalData
)

const (
	loginNoAnimateFlagName = "no-animate"
	loginNoImageFlagName   = "no-image"
	loginAnimationLoops    = 2
)

var errDeviceAuthorizationExpired = errors.New("device authorization request has expired")

type loginUIStyles struct {
	heading lipgloss.Style
	label   lipgloss.Style
	value   lipgloss.Style
	code    lipgloss.Style
	muted   lipgloss.Style
	success lipgloss.Style
}

type loginKonnectCmd struct {
	*cobra.Command
	noAnimate bool
	noImage   bool
}

type loginPollTokenFunc func(context.Context) (*auth.AccessToken, error)

type loginAnimationTickMsg time.Time

type loginPollMsg struct {
	token *auth.AccessToken
	err   error
}

type loginAnimationModel struct {
	ctx          context.Context
	frames       []string
	frame        int
	maxFrames    int
	instructions string
	pollInterval time.Duration
	expiresAt    time.Time
	poll         loginPollTokenFunc
	token        *auth.AccessToken
	err          error
}

// resolveAuthURLs returns the fully constructed auth and token poll URLs from cfg.
func resolveAuthURLs(cfg config.Hook) (authURL, pollURL string) {
	authBaseURL := cfg.GetString(common.AuthBaseURLConfigPath)
	if authBaseURL == "" {
		authBaseURL = common.AuthBaseURLDefault
	}
	return authBaseURL + cfg.GetString(common.AuthPathConfigPath),
		authBaseURL + cfg.GetString(common.TokenURLPathConfigPath)
}

func (c *loginKonnectCmd) validate(helper cmd.Helper) error {
	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	authURL, pollURL := resolveAuthURLs(cfg)
	if err := auth.ValidateKonnectURL(authURL); err != nil {
		return cmd.PrepareExecutionErrorWithHelper(helper, "invalid auth URL", err)
	}
	if err := auth.ValidateKonnectURL(pollURL); err != nil {
		return cmd.PrepareExecutionErrorWithHelper(helper, "invalid token poll URL", err)
	}

	return nil
}

func displayUserInstructions(w io.Writer, resp auth.DeviceCodeResponse, styled bool) {
	if styled {
		displayStyledUserInstructions(w, resp)
		return
	}
	displayPlainUserInstructions(w, resp)
}

func displayPlainUserInstructions(w io.Writer, resp auth.DeviceCodeResponse) {
	userResp := fmt.Sprintf("Logging your CLI into Kong Konnect with the browser...\n\n"+
		" To login, go to the following URL in your browser:\n\n"+
		"   %s\n\n"+
		" Or copy this one-time code: %s\n\n"+
		" And open your browser to %s\n\n"+
		" (Code expires in %d seconds)\n\n"+
		" Waiting for user to Login...",
		resp.VerificationURIComplete, resp.UserCode, resp.VerificationURI, resp.ExpiresIn)

	fmt.Fprintln(w, userResp)
}

func displayStyledUserInstructions(w io.Writer, resp auth.DeviceCodeResponse) {
	styles := loginUI()
	fmt.Fprintf(w, "%s %s\n\n",
		styles.value.Render("\u203a"),
		styles.heading.Render("Kong Konnect browser login"))
	fmt.Fprintf(w, "%s\n  %s\n\n",
		styles.label.Render("Open this URL in your browser:"),
		styles.value.Render(resp.VerificationURIComplete))
	fmt.Fprintf(w, "%s %s\n\n",
		styles.label.Render("Or copy this one-time code:"),
		styles.code.Render(resp.UserCode))
	fmt.Fprintf(w, "%s\n  %s\n\n",
		styles.label.Render("Then open:"),
		styles.value.Render(resp.VerificationURI))
	fmt.Fprintf(w, "%s\n\n",
		styles.muted.Render(fmt.Sprintf("Code expires in %d seconds", resp.ExpiresIn)))
	fmt.Fprintf(w, "%s\n", styles.muted.Render("Waiting for authorization..."))
}

func displayLoginSuccess(w io.Writer, styled bool) {
	if styled {
		styles := loginUI()
		fmt.Fprintf(w, "\n%s %s\n",
			styles.success.Render("\u2713"),
			styles.success.Render("User successfully authorized"))
		return
	}
	fmt.Fprintln(w, "\nUser successfully authorized")
}

func loginUI() loginUIStyles {
	palette := theme.Current()
	return loginUIStyles{
		heading: palette.ForegroundStyle(theme.ColorPrimary).Bold(true),
		label:   palette.ForegroundStyle(theme.ColorTextSecondary).Bold(true),
		value:   palette.ForegroundStyle(theme.ColorAccent),
		code:    palette.ForegroundStyle(theme.ColorSuccess).Bold(true),
		muted:   palette.ForegroundStyle(theme.ColorTextMuted),
		success: palette.ForegroundStyle(theme.ColorSuccess).Bold(true),
	}
}

func handleTelemetryPreference(
	ctx context.Context,
	streams *iostreams.IOStreams,
	cfg config.Hook,
	rec *telemetry.Recorder,
) error {
	if streams == nil || cfg == nil || rec == nil || !rec.Enabled() || telemetry.PreferenceFileExists(cfg) {
		return nil
	}

	if !loginInputIsTerminal(streams.In) {
		writeTelemetryDisclosure(streams.ErrOut, false)
		return nil
	}

	writeTelemetryDisclosure(streams.Out, true)
	enabled, ok, err := promptTelemetryPreference(ctx, streams.In, streams.Out)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	if !enabled {
		_ = rec.Disable(context.Background())
	}
	if err := telemetry.WritePreference(cfg, enabled); err != nil {
		fmt.Fprintf(streams.ErrOut, "warning: failed to save telemetry preference: %v\n", err)
	}
	return nil
}

func writeTelemetryDisclosure(w io.Writer, prompt bool) {
	fmt.Fprint(w, `kongctl collects limited usage data to help Kong understand CLI usage.

Collected:
  - kongctl version
  - operating system and architecture
  - command path, such as "login" or "get apis"

Not collected:
  - command arguments or flag values
  - resource names or IDs
  - auth tokens, request bodies, or response bodies
  - config file contents, file paths, hostnames, usernames, or email addresses

Telemetry can be disabled at any time with:
  kongctl --no-telemetry <command>
  KONGCTL_NO_TELEMETRY=true kongctl <command>
  DO_NOT_TRACK=1 kongctl <command>

`)
	if prompt {
		fmt.Fprint(w, "Allow kongctl to collect usage data on this device? [Y/n]: ")
	}
}

func promptTelemetryPreference(ctx context.Context, in io.Reader, out io.Writer) (bool, bool, error) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	defer signal.Stop(sigCh)

	return readTelemetryPreferenceAnswer(ctx, in, out, sigCh)
}

func readTelemetryPreferenceAnswer(
	ctx context.Context,
	in io.Reader,
	out io.Writer,
	interrupt <-chan os.Signal,
) (bool, bool, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	reader := bufio.NewReader(in)
	for attempt := range 2 {
		lineCh := make(chan string, 1)
		errCh := make(chan error, 1)
		// A terminal line read cannot be cancelled directly. In the login
		// prompt we accept this fire-and-forget goroutine so Ctrl-C can abort
		// immediately; the process exits shortly after this path returns.
		go func() {
			line, err := reader.ReadString('\n')
			if err != nil {
				errCh <- err
				return
			}
			lineCh <- line
		}()

		var line string
		select {
		case <-ctx.Done():
			return false, false, ctx.Err()
		case <-interrupt:
			fmt.Fprintln(out)
			return false, false, context.Canceled
		case <-errCh:
			return false, false, nil
		case line = <-lineCh:
		}

		switch strings.ToLower(strings.TrimSpace(line)) {
		case "", "y", "yes":
			return true, true, nil
		case "n", "no":
			return false, true, nil
		default:
			if attempt == 0 {
				fmt.Fprint(out, "Please answer y or n: ")
			}
		}
	}
	return false, false, nil
}

func isTerminalReader(in io.Reader) bool {
	file, ok := in.(*os.File)
	if !ok {
		return false
	}
	fd := file.Fd()
	return isatty.IsTerminal(fd) || isatty.IsCygwinTerminal(fd)
}

func isTerminalWriter(out io.Writer) bool {
	file, ok := out.(*os.File)
	if !ok {
		return false
	}
	fd := file.Fd()
	return isatty.IsTerminal(fd) || isatty.IsCygwinTerminal(fd)
}

func displayStaticLoginBanner(streams *iostreams.IOStreams) error {
	if streams == nil || streams.In == nil || streams.Out == nil {
		return nil
	}
	if !loginInputIsTerminal(streams.In) || !loginOutputIsTerminal(streams.Out) {
		return nil
	}

	terminal := loginTerminalData(streams.Out)
	if roarcmd.CanRenderFrameWidth(terminal) {
		useNativeColor := roarcmd.ShouldUseNativeAnimationColor(roarcmd.NativeColorValue, terminal)
		return roarcmd.RenderStaticFrame(streams.Out, nil, useNativeColor)
	}

	_, err := roarcmd.RenderFallbackClimber(streams.Out, terminal, nil)
	return err
}

func displayLoginBanner(streams *iostreams.IOStreams, noImage bool) error {
	if noImage {
		return nil
	}
	return displayStaticLoginBanner(streams)
}

func shouldAnimateLoginBanner(streams *iostreams.IOStreams, noAnimate, noImage bool) bool {
	if noImage {
		return false
	}
	if streams == nil || streams.In == nil || streams.Out == nil {
		return false
	}
	if !loginInputIsTerminal(streams.In) || !loginOutputIsTerminal(streams.Out) {
		return false
	}

	return roarcmd.ShouldRenderAnimation(noAnimate, loginTerminalData(streams.Out))
}

func shouldStyleLoginOutput(streams *iostreams.IOStreams) bool {
	if streams == nil || streams.Out == nil {
		return false
	}
	if _, disabled := os.LookupEnv("NO_COLOR"); disabled {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(os.Getenv("TERM")), "dumb") {
		return false
	}
	return loginOutputIsTerminal(streams.Out)
}

func loginInstructions(resp auth.DeviceCodeResponse, styled bool) string {
	var b strings.Builder
	displayUserInstructions(&b, resp, styled)
	return strings.TrimSuffix(b.String(), "\n")
}

func runAnimatedLogin(
	ctx context.Context,
	streams *iostreams.IOStreams,
	resp auth.DeviceCodeResponse,
	styled bool,
	poll loginPollTokenFunc,
) (*auth.AccessToken, error) {
	if streams == nil || streams.In == nil || streams.Out == nil {
		return waitForDeviceAuthorization(ctx, resp, poll)
	}

	terminal := loginTerminalData(streams.Out)
	useNativeColor := roarcmd.ShouldUseNativeAnimationColor(roarcmd.NativeColorValue, terminal)
	frames, err := roarcmd.AnimationFrames(nil, useNativeColor)
	if err != nil {
		return nil, err
	}

	model := newLoginAnimationModel(
		ctx,
		frames,
		loginInstructions(resp, styled),
		time.Duration(resp.Interval)*time.Second,
		time.Now().Add(time.Duration(resp.ExpiresIn)*time.Second),
		poll,
	)
	programOpts := []tea.ProgramOption{
		tea.WithContext(ctx),
		tea.WithInput(streams.In),
		tea.WithOutput(streams.Out),
	}
	if iostreams.HasTrueColorEnv() {
		programOpts = append(programOpts, tea.WithColorProfile(colorprofile.TrueColor))
	}

	finalModel, err := tea.NewProgram(model, programOpts...).Run()
	if errors.Is(err, tea.ErrInterrupted) || errors.Is(err, context.Canceled) {
		return nil, context.Canceled
	}
	if err != nil {
		return nil, err
	}
	model, ok := finalModel.(loginAnimationModel)
	if !ok {
		return nil, fmt.Errorf("unexpected login animation model type %T", finalModel)
	}
	if model.err != nil {
		return nil, model.err
	}
	return model.token, nil
}

func newLoginAnimationModel(
	ctx context.Context,
	frames []string,
	instructions string,
	pollInterval time.Duration,
	expiresAt time.Time,
	poll loginPollTokenFunc,
) loginAnimationModel {
	if ctx == nil {
		ctx = context.Background()
	}
	return loginAnimationModel{
		ctx:          ctx,
		frames:       frames,
		maxFrames:    loginAnimationLoops * len(frames),
		instructions: instructions,
		pollInterval: pollInterval,
		expiresAt:    expiresAt,
		poll:         poll,
	}
}

func (m loginAnimationModel) Init() tea.Cmd {
	return tea.Batch(tickLoginAnimation(), pollLoginAfter(m.ctx, m.pollInterval, m.poll))
}

func (m loginAnimationModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			m.err = context.Canceled
			return m, tea.Quit
		default:
			return m, nil
		}
	case loginAnimationTickMsg:
		if m.frame+1 < m.maxFrames {
			m.frame++
			return m, tickLoginAnimation()
		}
		return m, nil
	case loginPollMsg:
		return m.handlePoll(msg)
	default:
		return m, nil
	}
}

func (m loginAnimationModel) View() tea.View {
	frame := ""
	if len(m.frames) > 0 {
		frame = strings.TrimSuffix(m.frames[m.frame%len(m.frames)], "\n")
	}

	content := frame
	if m.instructions != "" {
		if content != "" {
			content += "\n\n"
		}
		content += m.instructions
	}
	return tea.NewView(content)
}

func (m loginAnimationModel) handlePoll(msg loginPollMsg) (tea.Model, tea.Cmd) {
	var dagError *auth.DAGError
	if errors.As(msg.err, &dagError) && dagError.ErrorCode == auth.AuthorizationPendingErrorCode {
		if time.Now().After(m.expiresAt) {
			m.err = errDeviceAuthorizationExpired
			return m, tea.Quit
		}
		return m, pollLoginAfter(m.ctx, m.pollInterval, m.poll)
	}
	if msg.err != nil {
		m.err = msg.err
		return m, tea.Quit
	}
	if msg.token != nil && msg.token.Token != nil && msg.token.Token.AuthToken != "" {
		m.token = msg.token
		return m, tea.Quit
	}
	if time.Now().After(m.expiresAt) {
		m.err = errDeviceAuthorizationExpired
		return m, tea.Quit
	}
	return m, pollLoginAfter(m.ctx, m.pollInterval, m.poll)
}

func tickLoginAnimation() tea.Cmd {
	return tea.Tick(time.Duration(roarcmd.AnimationFrameDelayMS())*time.Millisecond, func(t time.Time) tea.Msg {
		return loginAnimationTickMsg(t)
	})
}

func pollLoginAfter(ctx context.Context, interval time.Duration, poll loginPollTokenFunc) tea.Cmd {
	return tea.Tick(interval, func(time.Time) tea.Msg {
		if poll == nil {
			return loginPollMsg{err: fmt.Errorf("no login token poller configured")}
		}
		token, err := poll(ctx)
		return loginPollMsg{token: token, err: err}
	})
}

func waitForDeviceAuthorization(
	ctx context.Context,
	resp auth.DeviceCodeResponse,
	poll loginPollTokenFunc,
) (*auth.AccessToken, error) {
	expiresAt := time.Now().Add(time.Duration(resp.ExpiresIn) * time.Second)
	for {
		var err error
		var pollResp *auth.AccessToken
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Duration(resp.Interval) * time.Second):
			pollResp, err = poll(ctx)
		}
		var dagError *auth.DAGError
		if errors.As(err, &dagError) && dagError.ErrorCode == auth.AuthorizationPendingErrorCode {
			if time.Now().After(expiresAt) {
				return nil, errDeviceAuthorizationExpired
			}
			continue
		}
		if err != nil {
			return nil, err
		}

		if time.Now().After(expiresAt) {
			return nil, errDeviceAuthorizationExpired
		}

		if pollResp != nil && pollResp.Token != nil && pollResp.Token.AuthToken != "" {
			return pollResp, nil
		}
	}
}

func (c *loginKonnectCmd) run(helper cmd.Helper) error {
	logger, err := helper.GetLogger()
	if err != nil {
		return err
	}

	httpClient = httpclient.NewHTTPClient(15 * time.Second)

	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	streams := helper.GetStreams()
	styledLoginOutput := shouldStyleLoginOutput(streams)
	ctx := helper.GetContext()

	if err := handleTelemetryPreference(
		ctx,
		streams,
		cfg,
		telemetry.FromContext(ctx),
	); err != nil {
		c.SilenceUsage = true
		c.SilenceErrors = true
		return err
	}

	// Device authorization endpoints default to the global Konnect API host but can be overridden.
	authURL, pollURL := resolveAuthURLs(cfg)

	clientID := cfg.GetString(common.MachineClientIDConfigPath)

	resp, err := auth.RequestDeviceCode(httpClient, authURL, clientID, logger)
	if err != nil {
		return cmd.PrepareExecutionErrorWithHelper(helper, "failed to request device code", err)
	}

	if resp.UserCode == "" || resp.VerificationURI == "" || resp.VerificationURIComplete == "" ||
		resp.Interval == 0 || resp.ExpiresIn == 0 {
		return cmd.PrepareExecutionErrorMsg(helper,
			fmt.Sprintf("invalid device code request response from Konnect: %v", resp))
	}

	poll := func(ctx context.Context) (*auth.AccessToken, error) {
		return auth.PollForToken(ctx, httpClient, pollURL, clientID, resp.DeviceCode, logger)
	}

	var pollResp *auth.AccessToken
	if shouldAnimateLoginBanner(streams, c.noAnimate, c.noImage) {
		pollResp, err = runAnimatedLogin(ctx, streams, resp, styledLoginOutput, poll)
	} else {
		if err := displayLoginBanner(streams, c.noImage); err != nil {
			return cmd.PrepareExecutionErrorWithHelper(helper, "failed to render login banner", err)
		}
		displayUserInstructions(streams.Out, resp, styledLoginOutput)
		pollResp, err = waitForDeviceAuthorization(ctx, resp, poll)
	}
	if errors.Is(err, context.Canceled) {
		c.SilenceUsage = true
		c.SilenceErrors = true
		return err
	}
	if errors.Is(err, errDeviceAuthorizationExpired) {
		return cmd.PrepareExecutionErrorMsg(helper, errDeviceAuthorizationExpired.Error())
	}
	if err != nil {
		return cmd.PrepareExecutionErrorWithHelper(helper, "failed to poll for token", err)
	}
	displayLoginSuccess(streams.Out, styledLoginOutput)
	if err := auth.SaveAccessToken(cfg, pollResp); err != nil {
		return cmd.PrepareExecutionErrorWithHelper(helper, "failed to save tokens", err)
	}

	return nil
}

func (c *loginKonnectCmd) preRunE(cobraCmd *cobra.Command, args []string) error {
	helper := cmd.BuildHelper(cobraCmd, args)
	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	bindings := []struct{ flag, config string }{
		{common.AuthPathFlagName, common.AuthPathConfigPath},
		{common.AuthBaseURLFlagName, common.AuthBaseURLConfigPath},
		{common.RefreshPathFlagName, common.RefreshPathConfigPath},
		{common.TokenPathFlagName, common.TokenURLPathConfigPath},
		{common.MachineClientIDFlagName, common.MachineClientIDConfigPath},
	}
	for _, b := range bindings {
		if f := c.Flags().Lookup(b.flag); f != nil {
			if err := cfg.BindFlag(b.config, f); err != nil {
				return err
			}
		}
	}

	if err := common.ApplyEnvironmentDefaults(cobraCmd.Root(), cfg); err != nil {
		return err
	}

	return nil
}

func (c *loginKonnectCmd) runE(cobraCmd *cobra.Command, args []string) error {
	helper := cmd.BuildHelper(cobraCmd, args)
	if e := c.validate(helper); e != nil {
		return e
	}

	return c.run(helper)
}

func newLoginKonnectCmd(verb verbs.VerbValue,
	baseCmd *cobra.Command,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *loginKonnectCmd {
	rv := loginKonnectCmd{
		Command: baseCmd,
	}

	rv.Short = loginKonnectShort
	rv.Long = loginKonnectLong
	rv.Example = loginKonnectExample

	addParentFlags(verb, rv.Command)

	rv.Flags().String(common.AuthPathFlagName, common.AuthPathDefault,
		fmt.Sprintf(`URL path used to initiate Konnect Authorization.
- Config path: [ %s ]
-`, // (default ...)
			common.AuthPathConfigPath))

	rv.Flags().String(common.AuthBaseURLFlagName, common.AuthBaseURLDefault,
		fmt.Sprintf(`Base URL used for Konnect Authorization requests.
- Config path: [ %s ]
-`, // (default ...)
			common.AuthBaseURLConfigPath))

	rv.Flags().String(common.RefreshPathFlagName, common.RefreshPathDefault,
		fmt.Sprintf(`URL path used to refresh the Konnect auth token.
- Config path: [ %s ]
-`, // (default ...)
			common.RefreshPathConfigPath))

	rv.Flags().String(common.MachineClientIDFlagName, common.MachineClientIDDefault,
		fmt.Sprintf(`Machine Client ID used to identify the application for Konnect Authorization.
- Config path: [ %s ]
-`, // (default ...)
			common.MachineClientIDConfigPath))
	util.CheckError(rv.Flags().MarkHidden(common.MachineClientIDFlagName))

	rv.Flags().String(common.TokenPathFlagName, common.TokenPathDefault,
		fmt.Sprintf(`URL path used to poll for the Konnect Authorization response token.
- Config path: [ %s ]
-`, // (default ...)
			common.TokenURLPathConfigPath))

	rv.Flags().BoolVar(&rv.noAnimate, loginNoAnimateFlagName, false,
		"Print a static login banner instead of animating when the terminal supports animation.")
	rv.Flags().BoolVar(&rv.noImage, loginNoImageFlagName, false,
		"Show only login text without animation or static image output.")

	rv.PersistentPreRunE = func(c *cobra.Command, args []string) error {
		e := parentPreRun(c, args)
		if e != nil {
			return e
		}
		return rv.preRunE(c, args)
	}
	rv.RunE = rv.runE

	return &rv
}
