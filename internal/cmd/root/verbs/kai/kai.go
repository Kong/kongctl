package kai

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
	"github.com/kong/kongctl/internal/build"
	"github.com/kong/kongctl/internal/cmd"
	cmdcommon "github.com/kong/kongctl/internal/cmd/common"
	konnectcommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/iostreams"
	kaipkg "github.com/kong/kongctl/internal/kai"
	"github.com/kong/kongctl/internal/kai/render"
	kaistui "github.com/kong/kongctl/internal/kai/tui"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/theme"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/mattn/go-isatty"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

const (
	Verb        = verbs.Kai
	askFlagName = "ask"
)

const maxResumeSessions = 10

var (
	kaiShort = i18n.T("root.verbs.kai.short", "Launch an interactive session with the Konnect Kai agent")
	kaiLong  = normalizers.LongDesc(i18n.T(
		"root.verbs.kai.long",
		"Start an interactive Konnect Kai agent chat session.",
	))
)

func addBaseFlags(command *cobra.Command) {
	command.Flags().String(konnectcommon.BaseURLFlagName, konnectcommon.BaseURLDefault,
		fmt.Sprintf(`Base URL for Konnect API requests.
- Config path: [ %s ]`,
			konnectcommon.BaseURLConfigPath))

	command.Flags().String(konnectcommon.PATFlagName, "",
		fmt.Sprintf(`Konnect Personal Access Token (PAT) used to authenticate the CLI.
Setting this value overrides tokens obtained from the login command.
- Config path: [ %s ]`,
			konnectcommon.PATConfigPath))

	colorMode := cmd.NewEnum([]string{
		cmdcommon.ColorModeAuto.String(),
		cmdcommon.ColorModeAlways.String(),
		cmdcommon.ColorModeNever.String(),
	},
		cmdcommon.DefaultColorMode)

	command.Flags().Var(colorMode, cmdcommon.ColorFlagName,
		fmt.Sprintf(`Controls colorized terminal output.
- Config path: [ %s ]
- Allowed    : [ %s ]`,
			cmdcommon.ColorConfigPath, strings.Join(colorMode.Allowed, "|")))
}

func addFlags(command *cobra.Command) {
	addBaseFlags(command)
	command.Flags().BoolP(askFlagName, "a", false, "Send a single prompt to the agent and print the response")
}

func bindFlags(c *cobra.Command, args []string) error {
	helper := cmd.BuildHelper(c, args)
	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	f := c.Flags().Lookup(konnectcommon.BaseURLFlagName)
	if err := cfg.BindFlag(konnectcommon.BaseURLConfigPath, f); err != nil {
		return err
	}

	f = c.Flags().Lookup(konnectcommon.PATFlagName)
	if f != nil {
		if err := cfg.BindFlag(konnectcommon.PATConfigPath, f); err != nil {
			return err
		}
	}

	f = c.Flags().Lookup(cmdcommon.ColorFlagName)
	if f != nil {
		if err := cfg.BindFlag(cmdcommon.ColorConfigPath, f); err != nil {
			return err
		}
	}

	return nil
}

func NewKaiCmd() (*cobra.Command, error) {
	command := &cobra.Command{
		Use:    Verb.String(),
		Short:  kaiShort,
		Long:   kaiLong,
		Args:   validateArgs,
		Hidden: true,
		PersistentPreRunE: func(c *cobra.Command, args []string) error {
			ctx := c.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			ctx = context.WithValue(ctx, verbs.Verb, Verb)
			c.SetContext(ctx)
			return bindFlags(c, args)
		},
		RunE: func(c *cobra.Command, args []string) error {
			helper := cmd.BuildHelper(c, args)
			isAsk, err := c.Flags().GetBool(askFlagName)
			if err != nil {
				return err
			}
			if isAsk {
				return runAsk(helper)
			}
			return runInteractive(helper)
		},
	}

	addFlags(command)

	resumeCmd := newResumeCmd()
	command.AddCommand(resumeCmd)

	return command, nil
}

func newResumeCmd() *cobra.Command {
	command := &cobra.Command{
		Use:   "resume [session-id]",
		Short: "Resume a previous kai session",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			helper := cmd.BuildHelper(c, args)
			var sessionID string
			if len(args) > 0 {
				sessionID = strings.TrimSpace(args[0])
			}
			return runResume(helper, sessionID)
		},
	}

	command.Hidden = true
	command.Aliases = []string{"res"}
	command.Example = "kongctl kai resume\nkongctl kai resume 123e4567-e89b-12d3-a456-426614174000"

	command.PersistentPreRunE = func(c *cobra.Command, args []string) error {
		ctx := c.Context()
		if ctx == nil {
			ctx = context.Background()
		}
		ctx = context.WithValue(ctx, verbs.Verb, Verb)
		c.SetContext(ctx)
		return bindFlags(c, args)
	}

	addBaseFlags(command)

	return command
}

func validateArgs(cmd *cobra.Command, args []string) error {
	isAsk, err := cmd.Flags().GetBool(askFlagName)
	if err != nil {
		return err
	}

	if isAsk {
		return cobra.MinimumNArgs(1)(cmd, args)
	}

	return cobra.NoArgs(cmd, args)
}

func runInteractive(helper cmd.Helper) error {
	nameFactory := func() string {
		return fmt.Sprintf("kai-%s-%s", time.Now().Format("20060102-150405"), uuid.NewString()[:8])
	}
	return runInteractiveCore(helper, "", nil, nameFactory)
}

func runInteractiveResume(helper cmd.Helper, sessionID string, history *kaipkg.SessionHistory) error {
	return runInteractiveCore(helper, sessionID, history, nil)
}

func runInteractiveCore(
	helper cmd.Helper,
	sessionID string,
	history *kaipkg.SessionHistory,
	nameFactory func() string,
) error {
	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	logger, err := helper.GetLogger()
	if err != nil {
		return err
	}

	baseURL := cfg.GetString(konnectcommon.BaseURLConfigPath)
	if baseURL == "" {
		baseURL = konnectcommon.BaseURLDefault
	}

	token, err := konnectcommon.GetAccessToken(cfg, logger)
	if err != nil {
		return cmd.PrepareExecutionError("failed to resolve Konnect access token", err, helper.GetCmd())
	}

	streams := helper.GetStreams()
	if !isTerminal(streams.Out) {
		return cmd.PrepareExecutionError(
			"interactive chat requires a TTY",
			fmt.Errorf("output stream is not a terminal"),
			helper.GetCmd(),
		)
	}

	colorModeStr := cfg.GetString(cmdcommon.ColorConfigPath)
	colorMode, err := cmdcommon.ColorModeStringToIota(colorModeStr)
	if err != nil {
		return err
	}

	useColor := shouldUseColor(colorMode, streams.Out)

	ctx := helper.GetContext()
	if ctx == nil {
		ctx = context.Background()
	}

	version := "dev"
	if info, ok := ctx.Value(build.InfoKey).(*build.Info); ok && info != nil {
		if v := strings.TrimSpace(info.Version); v != "" {
			version = v
		}
	}

	var lookupControlPlane func(context.Context, string) (string, error)
	sdkFactory := konnectcommon.GetSDKFactory()
	if sdkFactory != nil {
		sdk, err := sdkFactory(cfg, logger)
		if err == nil && sdk != nil {
			if cpAPI := sdk.GetControlPlaneAPI(); cpAPI != nil {
				lookupControlPlane = func(ctx context.Context, name string) (string, error) {
					return helpers.GetControlPlaneID(ctx, cpAPI, name)
				}
			}
		}
	}

	var initialTasks []kaipkg.TaskDetails
	if strings.TrimSpace(sessionID) != "" {
		tasks, err := kaipkg.ListActiveTasks(ctx, nil, baseURL, token, sessionID)
		if err != nil {
			return cmd.PrepareExecutionError("failed to load active tasks", err, helper.GetCmd())
		}
		initialTasks = tasks
	}

	return kaistui.Run(ctx, streams, kaistui.Options{
		BaseURL:            baseURL,
		Token:              token,
		UseColor:           useColor,
		SessionNameFactory: nameFactory,
		SessionID:          sessionID,
		SessionHistory:     history,
		LookupControlPlane: lookupControlPlane,
		InitialTasks:       initialTasks,
		Version:            version,
		Theme:              theme.FromContext(ctx),
	})
}

func runResume(helper cmd.Helper, sessionID string) error {
	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	logger, err := helper.GetLogger()
	if err != nil {
		return err
	}

	baseURL := cfg.GetString(konnectcommon.BaseURLConfigPath)
	if baseURL == "" {
		baseURL = konnectcommon.BaseURLDefault
	}

	token, err := konnectcommon.GetAccessToken(cfg, logger)
	if err != nil {
		return cmd.PrepareExecutionError("failed to resolve Konnect access token", err, helper.GetCmd())
	}

	streams := helper.GetStreams()
	if !isTerminal(streams.Out) {
		return cmd.PrepareExecutionError(
			"interactive session resume requires a TTY",
			fmt.Errorf("output stream is not a terminal"),
			helper.GetCmd(),
		)
	}

	ctx := helper.GetContext()
	if ctx == nil {
		ctx = context.Background()
	}

	var history *kaipkg.SessionHistory

	if strings.TrimSpace(sessionID) == "" {
		list, err := kaipkg.ListSessions(ctx, nil, baseURL, token)
		if err != nil {
			return cmd.PrepareExecutionError("failed to list sessions", err, helper.GetCmd())
		}
		if len(list) == 0 {
			return cmd.PrepareExecutionError("no sessions available", fmt.Errorf("no sessions found"), helper.GetCmd())
		}

		selection, err := selectSession(ctx, streams, baseURL, token, list)
		if err != nil {
			return cmd.PrepareExecutionError("session selection failed", err, helper.GetCmd())
		}
		sessionID = selection.ID
		clearScreen(streams.Out)
	}

	history, err = kaipkg.GetSessionHistory(ctx, nil, baseURL, token, sessionID)
	if err != nil {
		return cmd.PrepareExecutionError("failed to load session history", err, helper.GetCmd())
	}

	return runInteractiveResume(helper, sessionID, history)
}

func selectSession(
	ctx context.Context,
	streams *iostreams.IOStreams,
	baseURL, token string,
	sessions kaipkg.SessionList,
) (kaipkg.SessionMetadata, error) {
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].CreatedAt.After(sessions[j].CreatedAt)
	})

	if len(sessions) > maxResumeSessions {
		sessions = sessions[:maxResumeSessions]
	}

	model := newSelectModel(ctx, baseURL, token, streams, sessions)
	prog := tea.NewProgram(model,
		tea.WithInput(streams.In),
		tea.WithOutput(streams.Out),
		tea.WithoutSignalHandler(),
	)

	result, err := prog.Run()
	if err != nil {
		return kaipkg.SessionMetadata{}, err
	}

	res, ok := result.(selectModel)
	if !ok || res.cancelled || res.index < 0 || res.index >= len(sessions) {
		return kaipkg.SessionMetadata{}, fmt.Errorf("selection cancelled")
	}

	return sessions[res.index], nil
}

func renderSessionLine(session kaipkg.SessionMetadata) string {
	created := ""
	if !session.CreatedAt.IsZero() {
		created = session.CreatedAt.Local().Format("2006-01-02 15:04")
	}

	name := collapseWhitespace(strings.TrimSpace(session.Name))
	if name == "" {
		name = session.ID
	}

	name = truncateString(name, 60)
	if created == "" {
		return name
	}
	return fmt.Sprintf("%s  %s", created, name)
}

type selectModel struct {
	ctx        context.Context
	baseURL    string
	token      string
	streams    *iostreams.IOStreams
	sessions   kaipkg.SessionList
	cursor     int
	index      int
	cancelled  bool
	confirmDel bool
}

func newSelectModel(
	ctx context.Context,
	baseURL, token string,
	streams *iostreams.IOStreams,
	sessions kaipkg.SessionList,
) selectModel {
	return selectModel{
		ctx:      ctx,
		baseURL:  baseURL,
		token:    token,
		streams:  streams,
		sessions: sessions,
		index:    -1,
		cursor:   0,
	}
}

func (m selectModel) Init() tea.Cmd { return nil }

func (m selectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			if m.confirmDel {
				return m, nil
			}
			m.index = m.cursor
			return m, tea.Quit
		case "q", "esc", "ctrl+c":
			if m.confirmDel {
				m.confirmDel = false
				return m, nil
			}
			m.cancelled = true
			return m, tea.Quit
		case "up", "k":
			if !m.confirmDel && m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if !m.confirmDel && m.cursor < len(m.sessions)-1 {
				m.cursor++
			}
		case "d":
			if len(m.sessions) == 0 {
				return m, nil
			}
			if !m.confirmDel {
				m.confirmDel = true
				return m, nil
			}
			if err := deleteSession(m.ctx, m.baseURL, m.token, m.sessions[m.cursor].ID); err != nil {
				fmt.Fprintf(m.streams.Out, "\nFailed to delete session: %v\n", err)
				m.confirmDel = false
				return m, nil
			}
			m.sessions = append(m.sessions[:m.cursor], m.sessions[m.cursor+1:]...)
			if m.cursor >= len(m.sessions) && m.cursor > 0 {
				m.cursor--
			}
			if len(m.sessions) == 0 {
				m.cancelled = true
				return m, tea.Quit
			}
			m.confirmDel = false
			return m, nil
		}
	case tea.WindowSizeMsg:
		// no-op
	}

	return m, nil
}

func (m selectModel) View() string {
	var b strings.Builder
	b.WriteString("Resume a previous session\n")
	b.WriteString("Use ↑/↓ to move, Enter to resume, d to delete, q to cancel\n\n")

	for i, session := range m.sessions {
		marker := " "
		if i == m.cursor {
			marker = "›"
		}
		line := renderSessionLine(session)
		b.WriteString(fmt.Sprintf("%s %s\n", marker, line))
	}

	if m.confirmDel && len(m.sessions) > 0 {
		b.WriteString("\nPress d again to confirm deletion, Esc to cancel\n")
	}

	return b.String()
}

func collapseWhitespace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func truncateString(s string, limit int) string {
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	if limit <= 1 {
		return string(runes[:limit])
	}
	return string(runes[:limit-1]) + "…"
}

func deleteSession(ctx context.Context, baseURL, token, sessionID string) error {
	if strings.TrimSpace(sessionID) == "" {
		return fmt.Errorf("session id cannot be empty")
	}

	endpoint, err := kaipkg.JoinSessionPath(baseURL, sessionID)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("User-Agent", meta.CLIName)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
	}

	return nil
}

func clearScreen(out io.Writer) {
	if isTerminal(out) {
		fmt.Fprint(out, "\033[2J\033[H")
	} else {
		fmt.Fprintln(out)
	}
}

func runAsk(helper cmd.Helper) error {
	args := helper.GetArgs()
	prompt := strings.TrimSpace(strings.Join(args, " "))
	if prompt == "" {
		return cmd.PrepareExecutionError("prompt is required", fmt.Errorf("prompt cannot be empty"), helper.GetCmd())
	}

	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	logger, err := helper.GetLogger()
	if err != nil {
		return err
	}

	baseURL := cfg.GetString(konnectcommon.BaseURLConfigPath)
	if baseURL == "" {
		baseURL = konnectcommon.BaseURLDefault
	}

	token, err := konnectcommon.GetAccessToken(cfg, logger)
	if err != nil {
		return cmd.PrepareExecutionError("failed to resolve Konnect access token", err, helper.GetCmd())
	}

	ctx := helper.GetContext()
	if ctx == nil {
		ctx = context.Background()
	}

	result, err := kaipkg.Chat(ctx, nil, baseURL, token, prompt)
	if err != nil {
		return cmd.PrepareExecutionError("failed to chat with the Kai agent", err, helper.GetCmd())
	}

	outType, err := helper.GetOutputFormat()
	if err != nil {
		return err
	}

	streams := helper.GetStreams()

	colorModeStr := cfg.GetString(cmdcommon.ColorConfigPath)
	colorMode, err := cmdcommon.ColorModeStringToIota(colorModeStr)
	if err != nil {
		return err
	}

	useColor := shouldUseColor(colorMode, streams.Out)

	switch outType {
	case cmdcommon.TEXT:
		formatted := render.Markdown(result.Response, render.Options{NoColor: !useColor})
		if _, err := fmt.Fprintln(streams.Out, formatted); err != nil {
			return err
		}
		return nil
	case cmdcommon.JSON, cmdcommon.YAML:
		printer, err := cli.Format(outType.String(), streams.Out)
		if err != nil {
			return err
		}
		defer printer.Flush()
		printer.Print(result)
		return nil
	default:
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("unsupported output format %s for %s command", outType.String(), helper.GetCmd().CommandPath()),
		}
	}
}

func shouldUseColor(mode cmdcommon.ColorMode, out io.Writer) bool {
	switch mode {
	case cmdcommon.ColorModeAlways:
		return true
	case cmdcommon.ColorModeNever:
		return false
	case cmdcommon.ColorModeAuto:
		if _, disabled := os.LookupEnv("NO_COLOR"); disabled {
			return false
		}
		return isTerminal(out)
	default:
		if _, disabled := os.LookupEnv("NO_COLOR"); disabled {
			return false
		}
		return isTerminal(out)
	}
}

var kaiTerminalDetector = func(fd uintptr) bool {
	return isatty.IsTerminal(fd) || isatty.IsCygwinTerminal(fd)
}

func isTerminal(w io.Writer) bool {
	type fdWriter interface {
		Fd() uintptr
	}
	if fw, ok := w.(fdWriter); ok {
		return kaiTerminalDetector(fw.Fd())
	}
	return false
}
