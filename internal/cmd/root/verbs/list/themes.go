package list

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/kong/kongctl/internal/cmd"
	cmdcommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/cmd/output/tableview"
	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/theme"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/mattn/go-isatty"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

func newThemesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "themes",
		Short: "List available color themes",
		Long: normalizers.LongDesc(`Display all registered color themes and a small sample
of their palette.`),
		RunE: func(c *cobra.Command, args []string) error {
			helper := cmd.BuildHelper(c, args)
			return runListThemes(helper)
		},
	}

	return cmd
}

func runListThemes(helper cmd.Helper) error {
	streams := helper.GetStreams()
	if streams == nil {
		return fmt.Errorf("output streams unavailable")
	}

	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	outFormat, err := helper.GetOutputFormat()
	if err != nil {
		return err
	}

	interactive, err := helper.IsInteractive()
	if err != nil {
		return err
	}

	var printer cli.PrintFlusher
	if !interactive {
		printer, err = cli.Format(outFormat.String(), streams.Out)
		if err != nil {
			return err
		}
		defer printer.Flush()
	}

	useColor := shouldRenderColor(cfg, interactive, streams.Out)
	activeTheme := activeThemeName(cfg)

	rows := buildThemeRows(useColor, activeTheme)

	return tableview.RenderForFormat(
		interactive,
		outFormat,
		printer,
		streams,
		rows.display,
		rows.raw,
		"Available Themes",
		tableview.WithCustomTable(rows.headers, rows.tableRows),
		tableview.WithPreviewRenderer(newThemePreviewRenderer(rows.palettes)),
		tableview.WithRootLabel(helper.GetCmd().Name()),
	)
}

func shouldRenderColor(cfg config.Hook, interactive bool, outWriter io.Writer) bool {
	if !interactive {
		return false
	}

	modeStr := strings.ToLower(strings.TrimSpace(cfg.GetString(cmdcommon.ColorConfigPath)))
	mode, err := cmdcommon.ColorModeStringToIota(modeStr)
	if err != nil {
		mode = cmdcommon.ColorModeAuto
	}

	return shouldUseColor(mode, outWriter)
}

func shouldUseColor(mode cmdcommon.ColorMode, out io.Writer) bool {
	switch mode {
	case cmdcommon.ColorModeAlways:
		return true
	case cmdcommon.ColorModeNever:
		return false
	case cmdcommon.ColorModeAuto:
		fp, ok := out.(fdProvider)
		if !ok {
			return false
		}
		fd := fp.Fd()
		if fd == ^uintptr(0) {
			return false
		}
		return isatty.IsTerminal(fd) || isatty.IsCygwinTerminal(fd)
	default:
		return false
	}
}

type fdProvider interface {
	Fd() uintptr
}

func activeThemeName(cfg config.Hook) string {
	name := strings.ToLower(strings.TrimSpace(cfg.GetString(cmdcommon.ColorThemeConfigPath)))
	if name == "" {
		name = cmdcommon.DefaultColorTheme
	}
	return name
}

type themeOutput struct {
	ID        string `json:"id"              yaml:"id"`
	Active    bool   `json:"active"          yaml:"active"`
	Primary   string `json:"primary"         yaml:"primary"`
	Secondary string `json:"secondary"       yaml:"secondary"`
	About     string `json:"about,omitempty" yaml:"about,omitempty"`
}

type themeRaw struct {
	ID        string      `json:"id"              yaml:"id"`
	Active    bool        `json:"active"          yaml:"active"`
	Primary   theme.Color `json:"primary"         yaml:"primary"`
	Secondary theme.Color `json:"secondary"       yaml:"secondary"`
	About     string      `json:"about,omitempty" yaml:"about,omitempty"`
}

type themeRowsData struct {
	headers   []string
	tableRows []table.Row
	display   []themeOutput
	raw       []themeRaw
	palettes  []theme.Palette
}

type sampleSlot struct {
	label      string
	background theme.Token
	foreground theme.Token
}

var sampleSlots = []sampleSlot{
	{"Primary", theme.ColorPrimary, theme.ColorPrimaryText},
	{"Secondary", theme.ColorAccent, theme.ColorAccentText},
}

type previewDetailSpec struct {
	label  string
	value  func(theme.Palette) string
	accent bool
}

var (
	themePreviewColumns = []table.Column{
		{Title: "ID", Width: 12},
		{Title: "TITLE", Width: 26},
	}
	themePreviewRows = []table.Row{
		{"506f4a94", "Kong API"},
		{"9c5aa6ce", "APIs"},
		{"fa506787", "Getting started"},
		{"838cd88f", "Guides"},
	}
	themePreviewDetailSpecs = []previewDetailSpec{
		{
			label: "Title",
			value: func(theme.Palette) string { return "Kong API" },
		},
		{
			label: "Theme",
			value: func(p theme.Palette) string {
				name := strings.TrimSpace(p.DisplayName)
				if name == "" {
					name = strings.TrimSpace(p.Name)
				}
				if name == "" {
					name = "Unnamed Theme"
				}
				return name
			},
		},
		{
			label: "Primary",
			value: func(p theme.Palette) string { return p.Color(theme.ColorPrimary).Light },
		},
		{
			label:  "Accent",
			value:  func(p theme.Palette) string { return p.Color(theme.ColorAccent).Light },
			accent: true,
		},
		{
			label:  "Status",
			value:  func(theme.Palette) string { return "published" },
			accent: true,
		},
		{
			label: "Updated",
			value: func(theme.Palette) string { return "2025-10-10 15:43:02" },
		},
	}
)

func buildThemeRows(useColor bool, activeName string) themeRowsData {
	ids := theme.Available()
	outputs := make([]themeOutput, 0, len(ids))
	raws := make([]themeRaw, 0, len(ids))
	tableRows := make([]table.Row, 0, len(ids))
	palettes := make([]theme.Palette, 0, len(ids))

	for _, id := range ids {
		pal, ok := theme.Get(id)
		if !ok {
			continue
		}

		active := strings.ToLower(pal.Name) == activeName
		displayID := pal.Name
		if active {
			displayID = "*" + displayID
		}

		rowSamples, displayPrimary, displaySecondary, rawPrimary, rawSecondary := buildSamples(pal, useColor)

		outputs = append(outputs, themeOutput{
			ID:        pal.Name,
			Active:    active,
			Primary:   displayPrimary,
			Secondary: displaySecondary,
			About:     strings.TrimSpace(pal.About),
		})
		raws = append(raws, themeRaw{
			ID:        pal.Name,
			Active:    active,
			Primary:   rawPrimary,
			Secondary: rawSecondary,
			About:     strings.TrimSpace(pal.About),
		})

		row := table.Row{displayID}
		row = append(row, rowSamples...)
		tableRows = append(tableRows, row)
		palettes = append(palettes, pal)
	}

	headers := []string{"ID", "Primary", "Secondary"}

	return themeRowsData{
		headers:   headers,
		tableRows: tableRows,
		display:   outputs,
		raw:       raws,
		palettes:  palettes,
	}
}

func buildSamples(p theme.Palette, useColor bool) ([]string, string, string, theme.Color, theme.Color) {
	primarySlot := sampleSlots[0]
	secondarySlot := sampleSlots[1]

	primaryColor := p.Color(primarySlot.background)
	secondaryColor := p.Color(secondarySlot.background)

	primaryDisplay := primaryColor.Light
	secondaryDisplay := secondaryColor.Light

	rowSamples := []string{
		renderBlock(p, primarySlot, useColor, primaryDisplay),
		renderBlock(p, secondarySlot, useColor, secondaryDisplay),
	}

	return rowSamples, primaryDisplay, secondaryDisplay, primaryColor, secondaryColor
}

func renderBlock(p theme.Palette, slot sampleSlot, useColor bool, fallback string) string {
	if !useColor {
		return fallback
	}

	const blockWidth = len("Secondary")
	block := p.BackgroundStyle(slot.background).Render(strings.Repeat(" ", blockWidth))
	return block
}

func newThemePreviewRenderer(palettes []theme.Palette) tableview.PreviewRenderer {
	if len(palettes) == 0 {
		return nil
	}

	return func(index int) string {
		if index < 0 {
			index = 0
		}
		if index >= len(palettes) {
			index = len(palettes) - 1
		}
		return renderThemePreviewPanel(palettes[index])
	}
}

func renderThemePreviewPanel(p theme.Palette) string {
	columns := append([]table.Column(nil), themePreviewColumns...)
	rows := make([]table.Row, len(themePreviewRows))
	for i, row := range themePreviewRows {
		rows[i] = append(table.Row(nil), row...)
	}

	styles := table.DefaultStyles()
	styles.Header = styles.Header.
		Foreground(p.Adaptive(theme.ColorTextPrimary)).
		Background(p.Adaptive(theme.ColorSurface))
	styles.Cell = styles.Cell.
		Foreground(p.Adaptive(theme.ColorTextPrimary))
	styles.Selected = styles.Selected.
		Foreground(p.Adaptive(theme.ColorAccentText)).
		Background(p.Adaptive(theme.ColorAccent))

	tbl := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithStyles(styles),
	)
	tbl.SetCursor(0)
	tbl.SetHeight(len(rows) + 1)

	width := 0
	for _, col := range columns {
		width += col.Width
	}
	paddingWidth := lipgloss.Width(styles.Cell.Render(""))
	if headerWidth := lipgloss.Width(styles.Header.Render("")); headerWidth > paddingWidth {
		paddingWidth = headerWidth
	}
	if width > 0 {
		tbl.SetWidth(width + paddingWidth*len(columns))
	}

	tableContent := tableview.NormalizeSelectedRow(tbl.View(), styles.Selected)
	tableBox := lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(p.Adaptive(theme.ColorBorder)).
		Padding(0, 1).
		MarginRight(1).
		Render(tableContent)

	labelStyle := p.ForegroundStyle(theme.ColorTextSecondary)
	valueStyle := p.ForegroundStyle(theme.ColorTextPrimary)
	accentStyle := p.ForegroundStyle(theme.ColorAccent)

	detailLines := make([]string, 0, len(themePreviewDetailSpecs))
	for _, spec := range themePreviewDetailSpecs {
		value := spec.value(p)
		valueRendered := valueStyle.Render(value)
		if spec.accent {
			valueRendered = accentStyle.Render(value)
		}
		detailLines = append(detailLines, fmt.Sprintf("%s %s",
			labelStyle.Render(spec.label+":"),
			valueRendered,
		))
	}
	detailContent := strings.Join(detailLines, "\n")
	detailBox := lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(p.Adaptive(theme.ColorBorder)).
		Padding(0, 1).
		Render(detailContent)

	title := p.ForegroundStyle(theme.ColorTextPrimary).
		Bold(true).
		Render("Theme Preview")

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		lipgloss.JoinHorizontal(lipgloss.Top, tableBox, detailBox),
	)
}
