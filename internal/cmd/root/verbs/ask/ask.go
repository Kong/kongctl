package ask

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	askpkg "github.com/kong/kongctl/internal/ask"
	"github.com/kong/kongctl/internal/ask/render"
	"github.com/kong/kongctl/internal/cmd"
	cmdcommon "github.com/kong/kongctl/internal/cmd/common"
	konnectcommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/mattn/go-isatty"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

const (
	Verb = verbs.Ask
)

var (
	askUse = fmt.Sprintf("%s <prompt>", Verb.String())

	askShort = i18n.T("root.verbs.ask.askShort", "Ask the Konnect Doctor Who agent a question")

	askLong = normalizers.LongDesc(i18n.T("root.verbs.ask.askLong",
		"Send a prompt to the Konnect Doctor Who agent and stream the response."))

	askExamples = normalizers.Examples(i18n.T("root.verbs.ask.askExamples",
		fmt.Sprintf(`
	# Ask the agent a question
	%[1]s ask "How do I configure a rate limit plugin?"

	# Ask without quotes (arguments joined with spaces)
	%[1]s ask diagnose latency issues`, meta.CLIName)))
)

func addFlags(cmd *cobra.Command) {
	cmd.Flags().String(konnectcommon.BaseURLFlagName, konnectcommon.BaseURLDefault,
		fmt.Sprintf(`Base URL for Konnect API requests.
- Config path: [ %s ]`,
			konnectcommon.BaseURLConfigPath))

	cmd.Flags().String(konnectcommon.PATFlagName, "",
		fmt.Sprintf(`Konnect Personal Access Token (PAT) used to authenticate the CLI.
Setting this value overrides tokens obtained from the login command.
- Config path: [ %s ]`,
			konnectcommon.PATConfigPath))
}

func bindFlags(c *cobra.Command, args []string) error {
	helper := cmd.BuildHelper(c, args)
	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	f := c.Flags().Lookup(konnectcommon.BaseURLFlagName)
	if err = cfg.BindFlag(konnectcommon.BaseURLConfigPath, f); err != nil {
		return err
	}

	f = c.Flags().Lookup(konnectcommon.PATFlagName)
	if f != nil {
		if err = cfg.BindFlag(konnectcommon.PATConfigPath, f); err != nil {
			return err
		}
	}

	return nil
}

func NewAskCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:     askUse,
		Short:   askShort,
		Long:    askLong,
		Example: askExamples,
		Args:    cobra.MinimumNArgs(1),
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
			return run(helper)
		},
	}

	addFlags(cmd)

	return cmd, nil
}

func run(helper cmd.Helper) error {
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

	result, err := askpkg.Chat(ctx, nil, baseURL, token, prompt)
	if err != nil {
		return cmd.PrepareExecutionError("failed to chat with Konnect Doctor Who agent", err, helper.GetCmd())
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
	}

	return nil
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

var terminalDetector = func(fd uintptr) bool {
	return isatty.IsTerminal(fd) || isatty.IsCygwinTerminal(fd)
}

func isTerminal(w io.Writer) bool {
	type fdWriter interface {
		Fd() uintptr
	}
	if fw, ok := w.(fdWriter); ok {
		return terminalDetector(fw.Fd())
	}
	return false
}
