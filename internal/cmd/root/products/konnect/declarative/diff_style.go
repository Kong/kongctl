package declarative

import (
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/colorprofile"
	cmdcommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/iostreams"
	"github.com/kong/kongctl/internal/theme"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

// Carry styling with the output so recursive renderers also work with plain
// writers, without global styling state or coloring already-formatted values.
type diffOutput struct {
	io.Writer
	palette theme.Palette
	styled  bool
}

func diffOutputFor(out io.Writer) *diffOutput {
	if output, ok := out.(*diffOutput); ok {
		return output
	}
	return &diffOutput{Writer: out}
}

func (out *diffOutput) paint(token theme.Token, text string) string {
	if !out.styled {
		return text
	}
	lines := strings.Split(text, "\n")
	style := out.palette.ForegroundStyle(token)
	for i, line := range lines {
		lines[i] = style.Render(line)
	}
	return strings.Join(lines, "\n")
}

func diffColorMode(command *cobra.Command) (cmdcommon.ColorMode, error) {
	value := cmdcommon.DefaultColorMode
	if command.Flags().Lookup(cmdcommon.ColorFlagName) != nil {
		value, _ = command.Flags().GetString(cmdcommon.ColorFlagName)
	}
	return cmdcommon.ColorModeStringToIota(value)
}

func diffColorEnabled(mode cmdcommon.ColorMode, terminal bool) bool {
	switch mode {
	case cmdcommon.ColorModeAlways:
		return true
	case cmdcommon.ColorModeNever:
		return false
	case cmdcommon.ColorModeAuto:
		_, disabled := os.LookupEnv("NO_COLOR")
		return terminal && !disabled && !strings.EqualFold(strings.TrimSpace(os.Getenv("TERM")), "dumb")
	default:
		return false
	}
}

func newDiffOutput(command *cobra.Command) (*diffOutput, error) {
	mode, err := diffColorMode(command)
	if err != nil {
		return nil, err
	}
	out := command.OutOrStdout()
	terminal := false
	if file, ok := out.(interface{ Fd() uintptr }); ok {
		terminal = isatty.IsTerminal(file.Fd()) || isatty.IsCygwinTerminal(file.Fd())
	}
	result := &diffOutput{Writer: out, styled: diffColorEnabled(mode, terminal)}
	if result.styled {
		result.palette = theme.FromContext(command.Context())
		writer := colorprofile.NewWriter(out, os.Environ())
		if mode == cmdcommon.ColorModeAlways || iostreams.HasTrueColorEnv() {
			writer.Profile = colorprofile.TrueColor
		}
		result.Writer = writer
	}
	return result, nil
}
