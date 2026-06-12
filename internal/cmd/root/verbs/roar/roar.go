package roar

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image/color"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/kong/kongctl/internal/art"
	cmdpkg "github.com/kong/kongctl/internal/cmd"
	cmdcommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/iostreams"
	"github.com/kong/kongctl/internal/theme"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

const (
	Verb                     = verbs.Roar
	outputFlagUnsupportedMsg = "flags -o/--" + cmdcommon.OutputFlagName + " are not supported for the roar command"
	widthFlagName            = "width"
	autoWidthValue           = "auto"
	fallbackWidth            = 48
	colorFlagName            = "color"
	autoColorValue           = "auto"
	offColorValue            = "off"
	artFlagName              = "art"
	autoArtValue             = "auto"
)

var (
	roarUse   = Verb.String()
	roarShort = i18n.T("root.verbs.roar.short", "Print the kongctl banner")

	detectTerminalWidth = terminalWidth
)

type roarCmd struct {
	width      string
	colorMode  string
	bannerType string
}

func NewRoarCmd() *cobra.Command {
	roar := roarCmd{
		width:      autoWidthValue,
		colorMode:  offColorValue,
		bannerType: autoArtValue,
	}
	cmd := &cobra.Command{
		Use:    roarUse,
		Short:  roarShort,
		Args:   cobra.NoArgs,
		Hidden: true,
		PersistentPreRunE: func(c *cobra.Command, _ []string) error {
			c.SetContext(context.WithValue(c.Context(), verbs.Verb, Verb))
			return nil
		},
		RunE: roar.run,
	}
	cmd.Flags().StringVar(&roar.width, widthFlagName, autoWidthValue,
		fmt.Sprintf("Banner width. Use %q or one of: %s.", autoWidthValue, supportedWidthValues()))
	cmd.Flags().StringVar(&roar.bannerType, artFlagName, autoArtValue,
		fmt.Sprintf("Banner art type. Use %q or one of: %s.", autoArtValue, supportedArtValues()))
	cmd.Flags().StringVar(&roar.colorMode, colorFlagName, offColorValue,
		`Banner color. Use "off", "auto", a hex color (#RGB or #RRGGBB), or an ANSI color code (0-255).`)

	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		if strings.Contains(err.Error(), "--"+cmdcommon.OutputFlagName) ||
			strings.Contains(err.Error(), "-"+cmdcommon.OutputFlagShort) {
			return &cmdpkg.UsageError{Err: errors.New(outputFlagUnsupportedMsg)}
		}
		return err
	})
	cmdcommon.SkipOutputFormatValidation(cmd)

	return cmd
}

func (c *roarCmd) run(command *cobra.Command, _ []string) error {
	if outputFlag := command.Flag(cmdcommon.OutputFlagName); outputFlag != nil && outputFlag.Changed {
		return &cmdpkg.UsageError{Err: errors.New(outputFlagUnsupportedMsg)}
	}

	streams, _ := command.Context().Value(iostreams.StreamsKey).(*iostreams.IOStreams)
	if streams == nil || streams.Out == nil {
		return fmt.Errorf("no output stream configured")
	}

	width, err := resolveBannerWidth(c.width, streams.Out)
	if err != nil {
		return &cmdpkg.ConfigurationError{Err: err}
	}
	bannerType, err := resolveBannerType(c.bannerType)
	if err != nil {
		return &cmdpkg.ConfigurationError{Err: err}
	}
	bannerColor, err := resolveBannerColor(c.colorMode)
	if err != nil {
		return &cmdpkg.ConfigurationError{Err: err}
	}

	if err := renderRoarBanner(streams.Out, width, bannerType, bannerColor); err != nil {
		return &cmdpkg.ConfigurationError{Err: err}
	}
	return nil
}

func supportedWidthValues() string {
	widths := art.SupportedKongBannerWidths()
	values := make([]string, 0, len(widths))
	for _, width := range widths {
		values = append(values, fmt.Sprintf("%d", width))
	}
	return strings.Join(values, ", ")
}

func supportedArtValues() string {
	bannerTypes := art.SupportedKongBannerTypes()
	values := make([]string, 0, len(bannerTypes))
	for _, bannerType := range bannerTypes {
		values = append(values, bannerType.String())
	}
	return strings.Join(values, ", ")
}

func renderRoarBanner(out io.Writer, width int, bannerType art.KongBannerType, bannerColor color.Color) error {
	if out == nil {
		return nil
	}

	var banner strings.Builder
	if err := art.RenderKongBannerType(&banner, width, bannerType); err != nil {
		return err
	}

	output := banner.String()
	if bannerColor != nil {
		style := lipgloss.NewStyle().
			Foreground(bannerColor).
			Inline(true)
		output = colorizeBannerLines(output, style)
	}

	_, err := io.Copy(out, bytes.NewBufferString(output))
	return err
}

func colorizeBannerLines(output string, style lipgloss.Style) string {
	var b strings.Builder
	for line := range strings.Lines(output) {
		line = strings.TrimSuffix(line, "\n")
		b.WriteString(style.Render(line))
		b.WriteString("\n")
	}
	return b.String()
}

func resolveBannerWidth(value string, out io.Writer) (int, error) {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" || value == autoWidthValue {
		return autoBannerWidth(out), nil
	}

	width, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("--%s must be %q or one of: %s", widthFlagName, autoWidthValue, supportedWidthValues())
	}
	if !slices.Contains(art.SupportedKongBannerWidths(), width) {
		return 0, fmt.Errorf("unsupported kong banner width %d; supported widths: %v",
			width, art.SupportedKongBannerWidths())
	}
	return width, nil
}

func resolveBannerType(value string) (art.KongBannerType, error) {
	value = strings.TrimSpace(strings.ToLower(value))
	switch value {
	case "", autoArtValue:
		return autoBannerType(), nil
	case art.KongBannerASCII.String():
		return art.KongBannerASCII, nil
	case art.KongBannerBraille.String():
		return art.KongBannerBraille, nil
	default:
		return "", fmt.Errorf("--%s must be %q or one of: %s", artFlagName, autoArtValue, supportedArtValues())
	}
}

func resolveBannerColor(value string) (color.Color, error) {
	value = strings.TrimSpace(value)
	switch {
	case value == "" || strings.EqualFold(value, offColorValue):
		return nil, nil
	case strings.EqualFold(value, autoColorValue):
		return theme.Current().Adaptive(theme.ColorAccent), nil
	case isExplicitColorCode(value):
		return lipgloss.Color(value), nil
	default:
		return nil, fmt.Errorf(
			"--%s must be %q, %q, a hex color (#RGB or #RRGGBB), or an ANSI color code (0-255)",
			colorFlagName, autoColorValue, offColorValue,
		)
	}
}

func isExplicitColorCode(value string) bool {
	if strings.HasPrefix(value, "#") {
		return isHexColorCode(value)
	}

	code, err := strconv.Atoi(value)
	return err == nil && code >= 0 && code <= 255
}

func isHexColorCode(value string) bool {
	if len(value) != 4 && len(value) != 7 {
		return false
	}
	for _, r := range value[1:] {
		switch {
		case r >= '0' && r <= '9':
		case r >= 'a' && r <= 'f':
		case r >= 'A' && r <= 'F':
		default:
			return false
		}
	}
	return true
}

func autoBannerType() art.KongBannerType {
	if strings.EqualFold(strings.TrimSpace(os.Getenv("TERM")), "dumb") || !localeLooksUTF8() {
		return art.KongBannerASCII
	}
	return art.KongBannerBraille
}

func localeLooksUTF8() bool {
	for _, key := range []string{"LC_ALL", "LC_CTYPE", "LANG"} {
		value := strings.TrimSpace(os.Getenv(key))
		if value == "" {
			continue
		}
		value = strings.ToUpper(value)
		return strings.Contains(value, "UTF-8") || strings.Contains(value, "UTF8")
	}
	return true
}

func autoBannerWidth(out io.Writer) int {
	terminalWidth, ok := detectTerminalWidth(out)
	if !ok {
		return fallbackWidth
	}

	selected := fallbackWidth
	for _, width := range art.SupportedKongBannerWidths() {
		if width <= terminalWidth {
			selected = width
		}
	}
	return selected
}

func terminalWidth(out io.Writer) (int, bool) {
	type fdProvider interface {
		Fd() uintptr
	}
	fdOut, ok := out.(fdProvider)
	if !ok {
		return 0, false
	}
	width, _, err := term.GetSize(int(fdOut.Fd()))
	if err != nil || width <= 0 {
		return 0, false
	}
	return width, true
}
