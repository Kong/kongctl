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
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/kong/kongctl/internal/art"
	cmdpkg "github.com/kong/kongctl/internal/cmd"
	cmdcommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/iostreams"
	"github.com/kong/kongctl/internal/theme"
	"github.com/kong/kongctl/internal/util"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

const (
	Verb                     = verbs.Roar
	outputFlagUnsupportedMsg = "flags -o/--" + cmdcommon.OutputFlagName + " are not supported for the roar command"
	climberWidthFlagName     = "climber-width"
	legacyWidthFlagName      = "width"
	autoWidthValue           = "auto"
	fallbackWidth            = 48
	colorFlagName            = "color"
	autoColorValue           = "auto"
	nativeColorValue         = "native"
	offColorValue            = "off"
	climberArtFlagName       = "climber-art"
	legacyArtFlagName        = "art"
	autoArtValue             = "auto"
	climberFlagName          = "climber"
	noAnimateFlagName        = "no-animate"
	loopsFlagName            = "loops"
	locationFlagName         = "location"
	legacyPlacementFlagName  = "placement"
	defaultAnimationLoops    = 2
	defaultLocationValue     = "top-left"
	defaultColorValue        = nativeColorValue
	minimumRenderableWidth   = fallbackWidth
	defaultStaticFrameNumber = art.KongRoarAnimationFrameCount / 3
	defaultStaticFrameIndex  = defaultStaticFrameNumber - 1
	animationLuminanceFloor  = 24
	animationLuminanceRange  = 255 - animationLuminanceFloor
	animationLuminanceRamp   = " .:-=+*#%@"
)

const (
	NativeColorValue = nativeColorValue
	PlacementTopLeft = placementTopLeft
)

var (
	roarUse   = Verb.String()
	roarShort = i18n.T("root.verbs.roar.short", "Feel the Kong power!")

	detectTerminalData      = terminalData
	roarAnimationFrameDelay = time.Duration(art.KongRoarAnimationDurationMS) * time.Millisecond
	hiddenRootFlags         = []string{
		cmdcommon.ConfigFilePathFlagName,
		cmdcommon.ProfileFlagName,
		cmdcommon.ColorThemeFlagName,
		cmdcommon.LogLevelFlagName,
		cmdcommon.LogFileFlagName,
	}
	unsupportedRootFlags = []string{
		cmdcommon.ConfigFilePathFlagName,
		cmdcommon.ProfileFlagName,
		cmdcommon.ColorThemeFlagName,
	}
)

type roarCmd struct {
	width      string
	colorMode  string
	bannerType string
	climber    bool
	noAnimate  bool
	loops      int
	location   string
}

type terminalCapabilities struct {
	width  int
	height int
	isTTY  bool
}

type TerminalCapabilities = terminalCapabilities

func NewTerminalCapabilities(width, height int, isTTY bool) TerminalCapabilities {
	return terminalCapabilities{
		width:  width,
		height: height,
		isTTY:  isTTY,
	}
}

type roarAnimationTickMsg time.Time

type roarPlacement string

type Placement = roarPlacement

const (
	placementTopLeft     roarPlacement = "top-left"
	placementTop         roarPlacement = "top"
	placementTopRight    roarPlacement = "top-right"
	placementLeft        roarPlacement = "left"
	placementCenter      roarPlacement = "center"
	placementRight       roarPlacement = "right"
	placementBottomLeft  roarPlacement = "bottom-left"
	placementBottom      roarPlacement = "bottom"
	placementBottomRight roarPlacement = "bottom-right"
)

type roarAnimationModel struct {
	frames    []string
	frame     int
	maxFrames int
	width     int
	height    int
	placement roarPlacement
}

func NewRoarCmd() *cobra.Command {
	roar := roarCmd{
		width:      autoWidthValue,
		colorMode:  defaultColorValue,
		bannerType: autoArtValue,
		loops:      defaultAnimationLoops,
		location:   defaultLocationValue,
	}
	cmd := &cobra.Command{
		Use:   roarUse,
		Short: roarShort,
		Args:  cobra.NoArgs,
		PersistentPreRunE: func(c *cobra.Command, _ []string) error {
			c.SetContext(context.WithValue(c.Context(), verbs.Verb, Verb))
			return nil
		},
		RunE: roar.run,
	}
	cmd.Flags().StringVar(&roar.width, climberWidthFlagName, autoWidthValue,
		fmt.Sprintf("Climber banner width. Use %q or one of: %s.", autoWidthValue, supportedWidthValues()))
	cmd.Flags().StringVar(&roar.width, legacyWidthFlagName, autoWidthValue,
		fmt.Sprintf("Deprecated alias for --%s.", climberWidthFlagName))
	util.CheckError(cmd.Flags().MarkHidden(legacyWidthFlagName))
	cmd.Flags().StringVar(&roar.bannerType, climberArtFlagName, autoArtValue,
		fmt.Sprintf(
			"Climber banner art type; selecting a concrete art type skips animation. Use %q or one of: %s.",
			autoArtValue,
			supportedArtValues(),
		))
	cmd.Flags().StringVar(&roar.bannerType, legacyArtFlagName, autoArtValue,
		fmt.Sprintf("Deprecated alias for --%s.", climberArtFlagName))
	util.CheckError(cmd.Flags().MarkHidden(legacyArtFlagName))
	cmd.Flags().StringVar(&roar.colorMode, colorFlagName, defaultColorValue,
		"Roar output color; animated and static banners use this as a whole-banner tint. "+
			`Use "native", "off", "auto", a hex color (#RGB or #RRGGBB), or an ANSI color code (0-255).`)
	cmd.Flags().BoolVar(&roar.climber, climberFlagName, false,
		"Print the static climber banner instead of the animation or fallback frame.")
	cmd.Flags().BoolVar(&roar.noAnimate, noAnimateFlagName, false, "Print a static frame instead of animating.")
	cmd.Flags().IntVar(&roar.loops, loopsFlagName, defaultAnimationLoops,
		"Number of animation loops to play when animation is supported.")
	cmd.Flags().StringVar(&roar.location, locationFlagName, defaultLocationValue,
		fmt.Sprintf("Animation location. Use one of: %s.", supportedPlacementValues()))
	cmd.Flags().StringVar(&roar.location, legacyPlacementFlagName, defaultLocationValue,
		fmt.Sprintf("Deprecated alias for --%s.", locationFlagName))
	util.CheckError(cmd.Flags().MarkHidden(legacyPlacementFlagName))

	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		if strings.Contains(err.Error(), "--"+cmdcommon.OutputFlagName) ||
			strings.Contains(err.Error(), "-"+cmdcommon.OutputFlagShort) {
			return &cmdpkg.UsageError{Err: errors.New(outputFlagUnsupportedMsg)}
		}
		return err
	})
	cmdcommon.SkipOutputFormatValidation(cmd)
	cmdcommon.HideInheritedFlags(cmd, hiddenRootFlags...)

	return cmd
}

func (c *roarCmd) run(command *cobra.Command, _ []string) error {
	if outputFlag := command.Flag(cmdcommon.OutputFlagName); outputFlag != nil && outputFlag.Changed {
		return &cmdpkg.UsageError{Err: errors.New(outputFlagUnsupportedMsg)}
	}
	if err := rejectUnsupportedRootFlags(command); err != nil {
		return &cmdpkg.UsageError{Err: err}
	}

	streams, _ := command.Context().Value(iostreams.StreamsKey).(*iostreams.IOStreams)
	if streams == nil || streams.Out == nil {
		return fmt.Errorf("no output stream configured")
	}
	if c.loops < 1 {
		return &cmdpkg.ConfigurationError{Err: fmt.Errorf("--%s must be greater than 0", loopsFlagName)}
	}
	placement, err := resolveRoarPlacement(c.location)
	if err != nil {
		return &cmdpkg.ConfigurationError{Err: err}
	}

	width, err := resolveBannerWidth(c.width, streams.Out)
	if err != nil {
		return &cmdpkg.ConfigurationError{Err: err}
	}
	bannerType, err := resolveBannerType(c.bannerType)
	if err != nil {
		return &cmdpkg.ConfigurationError{Err: err}
	}
	terminal := detectTerminalData(streams.Out)
	bannerColor, err := resolveEffectiveBannerColor(c.colorMode, terminal)
	if err != nil {
		return &cmdpkg.ConfigurationError{Err: err}
	}

	useNativeAnimationColor := shouldUseNativeAnimationColor(c.colorMode, terminal)
	useClimberBanner := c.climber || climberBannerFlagChanged(command)
	if shouldRenderAnimation(c.noAnimate || useClimberBanner, terminal) {
		if err := renderRoarAnimation(
			command.Context(),
			streams.In,
			streams.Out,
			c.loops,
			terminal,
			placement,
			bannerColor,
			useNativeAnimationColor,
		); err != nil {
			return &cmdpkg.ConfigurationError{Err: err}
		}
		return nil
	}

	if useClimberBanner {
		if !canRenderOutputWidth(terminal, width) {
			writeNarrowTerminalMessage(streams.ErrOut)
			return nil
		}
		if err := renderRoarBanner(streams.Out, width, bannerType, bannerColor); err != nil {
			return &cmdpkg.ConfigurationError{Err: err}
		}
		return nil
	}

	if canRenderOutputWidth(terminal, art.KongRoarAnimationWidth) {
		if err := renderRoarStaticFrame(streams.Out, bannerColor, useNativeAnimationColor); err != nil {
			return &cmdpkg.ConfigurationError{Err: err}
		}
		return nil
	}

	rendered, err := RenderFallbackClimber(streams.Out, terminal, bannerColor)
	if err != nil {
		return &cmdpkg.ConfigurationError{Err: err}
	}
	if !rendered {
		writeNarrowTerminalMessage(streams.ErrOut)
	}
	return nil
}

func rejectUnsupportedRootFlags(command *cobra.Command) error {
	var changed []string
	for _, flagName := range unsupportedRootFlags {
		flag := command.Flag(flagName)
		if flag != nil && flag.Changed {
			changed = append(changed, "--"+flagName)
		}
	}
	if len(changed) == 0 {
		return nil
	}
	if len(changed) == 1 {
		return fmt.Errorf("flag %s is not supported for the roar command", changed[0])
	}
	return fmt.Errorf("flags %s are not supported for the roar command", strings.Join(changed, ", "))
}

func climberBannerFlagChanged(command *cobra.Command) bool {
	for _, flagName := range []string{climberWidthFlagName, legacyWidthFlagName} {
		widthFlag := command.Flag(flagName)
		if widthFlag != nil && widthFlag.Changed {
			return true
		}
	}

	for _, flagName := range []string{climberArtFlagName, legacyArtFlagName} {
		artFlag := command.Flag(flagName)
		if artFlag != nil && artFlag.Changed &&
			!strings.EqualFold(strings.TrimSpace(artFlag.Value.String()), autoArtValue) {
			return true
		}
	}
	return false
}

func supportedWidthValues() string {
	widths := art.SupportedKongBannerWidths()
	parts := make([]string, len(widths))
	for i, w := range widths {
		parts[i] = strconv.Itoa(w)
	}
	return strings.Join(parts, ", ")
}

func supportedArtValues() string {
	bannerTypes := art.SupportedKongBannerTypes()
	parts := make([]string, len(bannerTypes))
	for i, bannerType := range bannerTypes {
		parts[i] = bannerType.String()
	}
	return strings.Join(parts, ", ")
}

func supportedPlacementValues() string {
	values := []roarPlacement{
		placementTopLeft,
		placementTop,
		placementTopRight,
		placementLeft,
		placementCenter,
		placementRight,
		placementBottomLeft,
		placementBottom,
		placementBottomRight,
	}
	parts := make([]string, len(values))
	for i, value := range values {
		parts[i] = string(value)
	}
	return strings.Join(parts, ", ")
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

func RenderFallbackClimber(out io.Writer, terminal TerminalCapabilities, bannerColor color.Color) (bool, error) {
	if !canRenderOutputWidth(terminal, fallbackWidth) {
		return false, nil
	}
	return true, renderRoarBanner(out, fallbackWidth, autoBannerType(), bannerColor)
}

func renderRoarStaticFrame(out io.Writer, bannerColor color.Color, useNativeColor bool) error {
	if out == nil {
		return nil
	}

	frames, err := art.KongRoarAnimationFrames()
	if err != nil {
		return err
	}
	frame, err := selectDefaultStaticFrame(frames)
	if err != nil {
		return err
	}

	rendered := prepareAnimationFrames([]string{frame}, bannerColor, useNativeColor)[0]
	if !strings.HasSuffix(rendered, "\n") {
		rendered += "\n"
	}
	_, err = io.Copy(out, bytes.NewBufferString(rendered))
	return err
}

func RenderStaticFrame(out io.Writer, bannerColor color.Color, useNativeColor bool) error {
	return renderRoarStaticFrame(out, bannerColor, useNativeColor)
}

func selectDefaultStaticFrame(frames []string) (string, error) {
	if len(frames) == 0 {
		return "", fmt.Errorf("no roar animation frames available")
	}

	index := defaultStaticFrameIndex
	if index >= len(frames) {
		index = len(frames) - 1
	}
	return frames[index], nil
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

func renderRoarAnimation(
	ctx context.Context,
	in io.Reader,
	out io.Writer,
	loops int,
	terminal terminalCapabilities,
	placement roarPlacement,
	bannerColor color.Color,
	useNativeColor bool,
) error {
	if out == nil {
		return nil
	}
	if loops < 1 {
		return fmt.Errorf("animation loops must be greater than 0")
	}

	frames, err := AnimationFrames(bannerColor, useNativeColor)
	if err != nil {
		return err
	}

	model := newRoarAnimationModel(frames, loops, terminal, placement)
	programOpts := []tea.ProgramOption{
		tea.WithContext(ctx),
		tea.WithOutput(out),
		tea.WithWindowSize(terminal.width, terminal.height),
	}
	if in != nil {
		programOpts = append(programOpts, tea.WithInput(in))
	}
	if iostreams.HasTrueColorEnv() {
		programOpts = append(programOpts, tea.WithColorProfile(colorprofile.TrueColor))
	}

	_, err = tea.NewProgram(model, programOpts...).Run()
	if errors.Is(err, tea.ErrInterrupted) || errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}

func AnimationFrames(bannerColor color.Color, useNativeColor bool) ([]string, error) {
	frames, err := art.KongRoarAnimationFrames()
	if err != nil {
		return nil, err
	}
	return prepareAnimationFrames(frames, bannerColor, useNativeColor), nil
}

func AnimationFrameDelayMS() int {
	return art.KongRoarAnimationDurationMS
}

func RenderAnimation(
	ctx context.Context,
	in io.Reader,
	out io.Writer,
	loops int,
	terminal TerminalCapabilities,
	placement Placement,
	bannerColor color.Color,
	useNativeColor bool,
) error {
	return renderRoarAnimation(ctx, in, out, loops, terminal, placement, bannerColor, useNativeColor)
}

func prepareAnimationFrames(frames []string, bannerColor color.Color, useNativeColor bool) []string {
	if useNativeColor {
		return slices.Clone(frames)
	}

	colorized := make([]string, 0, len(frames))
	for _, frame := range frames {
		frame = monochromeAnimationFrame(frame)
		if bannerColor != nil {
			style := lipgloss.NewStyle().
				Foreground(bannerColor).
				Inline(true)
			frame = colorizeBannerLines(frame, style)
		}
		colorized = append(colorized, frame)
	}
	return colorized
}

func monochromeAnimationFrame(frame string) string {
	var b strings.Builder
	var foreground color.Color
	for len(frame) > 0 {
		if strings.HasPrefix(frame, "\x1b[") {
			params, rest, ok := strings.Cut(frame[2:], "m")
			if ok {
				foreground = sgrForeground(params, foreground)
				frame = rest
				continue
			}
		}

		r, size := utf8.DecodeRuneInString(frame)
		if r == utf8.RuneError && size == 0 {
			break
		}
		switch r {
		case '\n':
			b.WriteRune('\n')
		case '\r':
		case ' ', '\t':
			b.WriteRune(' ')
		default:
			b.WriteRune(animationRampRune(foreground))
		}
		frame = frame[size:]
	}
	return b.String()
}

func sgrForeground(params string, current color.Color) color.Color {
	parts := strings.Split(params, ";")
	for i := 0; i < len(parts); i++ {
		value, err := strconv.Atoi(parts[i])
		if err != nil {
			continue
		}

		switch value {
		case 0, 39:
			current = nil
		case 38:
			if i+4 >= len(parts) || parts[i+1] != "2" {
				continue
			}
			r, errR := strconv.Atoi(parts[i+2])
			g, errG := strconv.Atoi(parts[i+3])
			b, errB := strconv.Atoi(parts[i+4])
			if errR == nil && errG == nil && errB == nil {
				current = color.RGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: 255}
			}
			i += 4
		case 48:
			if i+1 >= len(parts) {
				continue
			}
			switch parts[i+1] {
			case "2":
				if i+4 < len(parts) {
					i += 4
				}
			case "5":
				if i+2 < len(parts) {
					i += 2
				}
			}
		}
	}
	return current
}

func animationRampRune(c color.Color) rune {
	if c == nil {
		return ' '
	}

	r, g, b, _ := c.RGBA()
	luminance := int(0.2126*float64(r>>8) + 0.7152*float64(g>>8) + 0.0722*float64(b>>8) + 0.5)
	if luminance < animationLuminanceFloor {
		return ' '
	}

	ramp := []rune(animationLuminanceRamp)
	index := (luminance - animationLuminanceFloor) * (len(ramp) - 1) / animationLuminanceRange
	return ramp[index]
}

func newRoarAnimationModel(
	frames []string,
	loops int,
	terminal terminalCapabilities,
	placement roarPlacement,
) roarAnimationModel {
	return roarAnimationModel{
		frames:    frames,
		maxFrames: loops * len(frames),
		width:     terminal.width,
		height:    terminal.height,
		placement: placement,
	}
}

func (m roarAnimationModel) Init() tea.Cmd {
	return tickRoarAnimation()
}

func (m roarAnimationModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			return m, tea.Quit
		default:
			return m, nil
		}
	case roarAnimationTickMsg:
		m.frame++
		if m.frame >= m.maxFrames {
			return m, tea.Quit
		}
		return m, tickRoarAnimation()
	default:
		return m, nil
	}
}

func (m roarAnimationModel) View() tea.View {
	content := ""
	if len(m.frames) > 0 && m.maxFrames > 0 {
		content = m.frames[m.frame%len(m.frames)]
	}
	if m.width >= art.KongRoarAnimationWidth && m.height >= art.KongRoarAnimationHeight {
		horizontal, vertical := m.placement.positions()
		content = lipgloss.Place(m.width, m.height, horizontal, vertical, content)
	}

	view := tea.NewView(content)
	view.AltScreen = true
	return view
}

func tickRoarAnimation() tea.Cmd {
	return tea.Tick(roarAnimationFrameDelay, func(t time.Time) tea.Msg {
		return roarAnimationTickMsg(t)
	})
}

func resolveRoarPlacement(value string) (roarPlacement, error) {
	value = strings.TrimSpace(strings.ToLower(value))
	switch value {
	case "", string(placementTopLeft):
		return placementTopLeft, nil
	case string(placementCenter), "middle", "middle-center":
		return placementCenter, nil
	case string(placementTop), "top-center":
		return placementTop, nil
	case string(placementTopRight):
		return placementTopRight, nil
	case string(placementLeft), "center-left", "middle-left":
		return placementLeft, nil
	case string(placementRight), "center-right", "middle-right":
		return placementRight, nil
	case string(placementBottomLeft):
		return placementBottomLeft, nil
	case string(placementBottom), "bottom-center":
		return placementBottom, nil
	case string(placementBottomRight):
		return placementBottomRight, nil
	default:
		return "", fmt.Errorf("--%s must be one of: %s", locationFlagName, supportedPlacementValues())
	}
}

func (p roarPlacement) positions() (lipgloss.Position, lipgloss.Position) {
	switch p {
	case placementTopLeft:
		return lipgloss.Left, lipgloss.Top
	case placementTop:
		return lipgloss.Center, lipgloss.Top
	case placementTopRight:
		return lipgloss.Right, lipgloss.Top
	case placementLeft:
		return lipgloss.Left, lipgloss.Center
	case placementRight:
		return lipgloss.Right, lipgloss.Center
	case placementBottomLeft:
		return lipgloss.Left, lipgloss.Bottom
	case placementBottom:
		return lipgloss.Center, lipgloss.Bottom
	case placementBottomRight:
		return lipgloss.Right, lipgloss.Bottom
	case placementCenter:
		return lipgloss.Center, lipgloss.Center
	default:
		return lipgloss.Center, lipgloss.Center
	}
}

func resolveBannerWidth(value string, out io.Writer) (int, error) {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" || value == autoWidthValue {
		return autoBannerWidth(out), nil
	}

	width, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("--%s must be %q or one of: %s",
			climberWidthFlagName, autoWidthValue, supportedWidthValues())
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
		return "", fmt.Errorf("--%s must be %q or one of: %s", climberArtFlagName, autoArtValue, supportedArtValues())
	}
}

func resolveBannerColor(value string) (color.Color, error) {
	value = strings.TrimSpace(value)
	switch {
	case value == "" || strings.EqualFold(value, nativeColorValue) || strings.EqualFold(value, offColorValue):
		return nil, nil
	case strings.EqualFold(value, autoColorValue):
		return theme.Current().Adaptive(theme.ColorAccent), nil
	case isExplicitColorCode(value):
		return lipgloss.Color(value), nil
	default:
		return nil, fmt.Errorf(
			"--%s must be %q, %q, %q, a hex color (#RGB or #RRGGBB), or an ANSI color code (0-255)",
			colorFlagName, nativeColorValue, autoColorValue, offColorValue,
		)
	}
}

func resolveEffectiveBannerColor(value string, terminal terminalCapabilities) (color.Color, error) {
	value = strings.TrimSpace(value)
	if value == "" || strings.EqualFold(value, nativeColorValue) {
		return nil, nil
	}
	if strings.EqualFold(value, autoColorValue) {
		if !shouldUseAutoColor(terminal) {
			return nil, nil
		}
	}
	return resolveBannerColor(value)
}

func shouldUseNativeAnimationColor(value string, terminal terminalCapabilities) bool {
	return strings.EqualFold(strings.TrimSpace(value), nativeColorValue) && shouldUseAutoColor(terminal)
}

func ShouldUseNativeAnimationColor(value string, terminal TerminalCapabilities) bool {
	return shouldUseNativeAnimationColor(value, terminal)
}

func shouldUseAutoColor(terminal terminalCapabilities) bool {
	if _, disabled := os.LookupEnv("NO_COLOR"); disabled {
		return false
	}
	return terminal.isTTY && !terminalLooksDumb()
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
	if terminalLooksDumb() || !localeLooksUTF8() {
		return art.KongBannerASCII
	}
	return art.KongBannerBraille
}

func terminalLooksDumb() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("TERM")), "dumb")
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
	terminal := detectTerminalData(out)
	if terminal.width <= 0 {
		return fallbackWidth
	}

	selected := fallbackWidth
	for _, width := range art.SupportedKongBannerWidths() {
		if width <= terminal.width {
			selected = width
		}
	}
	return selected
}

func shouldRenderAnimation(noAnimate bool, terminal terminalCapabilities) bool {
	if noAnimate {
		return false
	}
	if !terminal.isTTY || terminalLooksDumb() || !localeLooksUTF8() {
		return false
	}
	if terminal.width < art.KongRoarAnimationWidth || terminal.height < art.KongRoarAnimationHeight {
		return false
	}
	return true
}

func ShouldRenderAnimation(noAnimate bool, terminal TerminalCapabilities) bool {
	return shouldRenderAnimation(noAnimate, terminal)
}

func CanRenderFrameWidth(terminal TerminalCapabilities) bool {
	return canRenderOutputWidth(terminal, art.KongRoarAnimationWidth)
}

func canRenderOutputWidth(terminal terminalCapabilities, width int) bool {
	return terminal.width <= 0 || terminal.width >= width
}

func writeNarrowTerminalMessage(out io.Writer) {
	if out == nil {
		return
	}
	fmt.Fprintf(out, "kongctl roar requires a terminal at least %d columns wide.\n", minimumRenderableWidth)
}

func terminalData(out io.Writer) terminalCapabilities {
	type fdProvider interface {
		Fd() uintptr
	}
	fdOut, ok := out.(fdProvider)
	if !ok {
		return terminalCapabilities{}
	}
	fd := fdOut.Fd()
	if fd == ^uintptr(0) {
		return terminalCapabilities{}
	}

	width, height, err := term.GetSize(int(fd))
	if err != nil {
		width = 0
		height = 0
	}

	return terminalCapabilities{
		width:  width,
		height: height,
		isTTY:  isatty.IsTerminal(fd) || isatty.IsCygwinTerminal(fd),
	}
}

func DetectTerminalData(out io.Writer) TerminalCapabilities {
	return terminalData(out)
}
