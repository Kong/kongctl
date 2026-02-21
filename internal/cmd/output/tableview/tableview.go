package tableview

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-isatty"
	"github.com/mattn/go-runewidth"
	"golang.org/x/term"

	"github.com/atotto/clipboard"
	cmdpkg "github.com/kong/kongctl/internal/cmd"
	cmdCommon "github.com/kong/kongctl/internal/cmd/common"
	jqoutput "github.com/kong/kongctl/internal/cmd/output/jq"
	"github.com/kong/kongctl/internal/iostreams"
	kairender "github.com/kong/kongctl/internal/kai/render"
	"github.com/kong/kongctl/internal/theme"
	"github.com/segmentio/cli"
)

type fdProvider interface {
	Fd() uintptr
}

type DetailRenderer func(index int) string

type DetailContextProvider func(index int) any

type RowLoader func(index int) (ChildView, error)

type config struct {
	title         string
	footer        string
	staticFooter  string
	quitKeys      []string
	toggleHelpKey string
	rootLabel     string
	rowLoader     RowLoader
	initialRow    int
	openInitial   bool
	tableStretch  bool
	profileName   string

	// optional behavior
	detailRenderer DetailRenderer
	hasDetail      bool
	detailViewport *viewport.Model

	previewRenderer PreviewRenderer

	customHeaders []string
	customRows    []table.Row

	childParentType string
	detailContext   DetailContextProvider
	childHelper     cmdpkg.Helper
}

// PreviewRenderer allows injecting a synthetic preview above the table that
// updates with the current cursor position.
type PreviewRenderer func(index int) string

const searchIdleTimeout = 10 * time.Second

type searchTimeoutMsg struct {
	deadline time.Time
}

type requestKind int

const (
	requestKindRow requestKind = iota
	requestKindDetail
)

type pendingRequest struct {
	id        string
	started   time.Time
	label     string
	kind      requestKind
	rowIndex  int
	detailID  int
	itemIndex int
	active    bool
}

type rowChildLoadedMsg struct {
	requestID string
	index     int
	child     ChildView
	err       error
}

type detailChildLoadedMsg struct {
	requestID string
	detailID  int
	itemIndex int
	child     ChildView
	err       error
	label     string
}

func newSpinnerModel(p theme.Palette) spinner.Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = p.ForegroundStyle(theme.ColorAccent)
	return s
}

func formatElapsed(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	seconds := int(d.Round(time.Second).Seconds())
	if seconds < 1 {
		fraction := d.Round(100 * time.Millisecond)
		realSeconds := float64(fraction) / float64(time.Second)
		return fmt.Sprintf("%.1fs", realSeconds)
	}
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}
	minutes := seconds / 60
	remainder := seconds % 60
	if remainder == 0 {
		return fmt.Sprintf("%dmin", minutes)
	}
	return fmt.Sprintf("%dmin %ds", minutes, remainder)
}

func formatRequestLabel(label string) string {
	trimmed := strings.TrimSpace(label)
	if trimmed == "" {
		return "request"
	}
	return trimmed
}

func (pr pendingRequest) inflightMessage() string {
	label := strings.TrimSpace(pr.label)
	if label == "" {
		switch pr.kind {
		case requestKindRow:
			label = "selection"
		case requestKindDetail:
			label = "detail"
		default:
			label = "request"
		}
	}
	return fmt.Sprintf("Loading %s...", label)
}

func borderedTableView(style lipgloss.Style, content string, selected lipgloss.Style) string {
	content = NormalizeSelectedRow(content, selected)
	return style.Render(content)
}

func borderedDetailView(style lipgloss.Style, content string) string {
	return style.Render(content)
}

func newTableBoxStyle(p theme.Palette) lipgloss.Style {
	return lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(p.Adaptive(theme.ColorBorder)).
		Padding(0, 1)
}

func newDetailBoxStyle(p theme.Palette) lipgloss.Style {
	return lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(p.Adaptive(theme.ColorBorder)).
		Padding(0, 1).
		MarginLeft(0)
}

func newStatusBoxStyle(p theme.Palette) lipgloss.Style {
	return lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(p.Adaptive(theme.ColorBorder)).
		Padding(0, 1)
}

// NormalizeSelectedRow ensures that selected rows emitted by the table component
// keep the highlight active across all columns when wrapped by another style.
func NormalizeSelectedRow(content string, selected lipgloss.Style) string {
	const reset = "\x1b[0m"

	prefix := selectionPrefix(selected, reset)
	if prefix == "" || !strings.Contains(content, prefix) {
		return content
	}

	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if !strings.Contains(line, prefix) {
			continue
		}

		count := strings.Count(line, reset)
		if count <= 1 {
			continue
		}

		line = strings.ReplaceAll(line, reset+prefix, reset)
		lines[i] = strings.Replace(line, reset, reset+prefix, count-1)
	}

	return strings.Join(lines, "\n")
}

func selectionPrefix(style lipgloss.Style, reset string) string {
	rendered := style.Render("")
	if rendered == "" {
		return ""
	}

	idx := strings.LastIndex(rendered, reset)
	if idx == -1 {
		return ""
	}

	return rendered[:idx]
}

func stylizeDetailContent(content string, palette theme.Palette) string {
	if content == "" {
		return content
	}
	if strings.Contains(content, "\x1b[") {
		return content
	}

	labelStyle := palette.ForegroundStyle(theme.ColorTextSecondary)
	valueStyle := palette.ForegroundStyle(theme.ColorTextPrimary)
	accentStyle := palette.ForegroundStyle(theme.ColorInfo)

	lines := strings.Split(content, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		prefixLen := len(line) - len(strings.TrimLeft(line, " \t"))
		prefix := line[:prefixLen]

		parts := strings.SplitN(strings.TrimLeft(line, " \t"), ":", 2)
		if len(parts) == 2 {
			label := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])

			styled := labelStyle.Render(label + ":")
			if value != "" {
				style := valueStyle
				if shouldAccentDetail(label, value) {
					style = accentStyle
				}
				styled += " " + style.Render(value)
			}
			lines[i] = prefix + styled
			continue
		}

		lines[i] = prefix + valueStyle.Render(strings.TrimSpace(line))
	}

	return strings.Join(lines, "\n")
}

func shouldAccentDetail(label, value string) bool {
	switch strings.ToLower(strings.TrimSpace(label)) {
	case "id":
		return true
	default:
	}
	low := strings.ToLower(strings.TrimSpace(value))
	if strings.Contains(low, "error") || strings.Contains(low, "failed") {
		return true
	}
	return false
}

// Option allows configuring optional behaviour for the table renderer.
type Option func(*config)

// WithTitle adds a title above the rendered table.
func WithTitle(title string) Option {
	return func(cfg *config) {
		cfg.title = title
	}
}

// WithFooter overrides the default footer message when running interactively.
func WithFooter(msg string) Option {
	return func(cfg *config) {
		cfg.footer = msg
	}
}

// WithDetailRenderer enables a side-by-side detail view that updates as the table selection changes.
func WithDetailRenderer(renderer DetailRenderer) Option {
	return func(cfg *config) {
		cfg.detailRenderer = renderer
	}
}

// WithPreviewRenderer renders a preview panel above the table that updates with
// the current cursor position.
func WithPreviewRenderer(renderer PreviewRenderer) Option {
	return func(cfg *config) {
		cfg.previewRenderer = renderer
	}
}

// WithRootLabel sets the root segment for breadcrumb navigation.
func WithRootLabel(label string) Option {
	return func(cfg *config) {
		cfg.rootLabel = strings.TrimSpace(label)
	}
}

// WithDetailContext provides parent objects for detail rows, enabling child loaders.
func WithDetailContext(parentType string, provider DetailContextProvider) Option {
	return func(cfg *config) {
		cfg.childParentType = strings.ToLower(strings.TrimSpace(parentType))
		cfg.detailContext = provider
	}
}

// WithDetailHelper supplies the command helper used to resolve child loaders.
func WithDetailHelper(helper cmdpkg.Helper) Option {
	return func(cfg *config) {
		cfg.childHelper = helper
	}
}

// WithCustomTable allows overriding the automatically generated table columns/rows.
// The provided headers and rows are used only for the interactive table; other formats
// (text/json/yaml) continue to print the original display value.
func WithCustomTable(headers []string, rows []table.Row) Option {
	return func(cfg *config) {
		cfg.customHeaders = append([]string(nil), headers...)
		cfg.customRows = append([]table.Row(nil), rows...)
	}
}

// WithRowLoader registers a loader that creates a child collection view for the currently
// selected row when the user presses enter. The loader should return a fully populated
// ChildView that mirrors the interactive resource view to display.
func WithRowLoader(loader RowLoader) Option {
	return func(cfg *config) {
		cfg.rowLoader = loader
	}
}

// WithInitialRowSelection positions the cursor on the provided row index and optionally opens
// the associated child view immediately when the interactive session starts.
func WithInitialRowSelection(index int, openChild bool) Option {
	return func(cfg *config) {
		cfg.initialRow = index
		cfg.openInitial = openChild
	}
}

// WithTableStretch expands the primary table to occupy the full available width.
func WithTableStretch() Option {
	return func(cfg *config) {
		cfg.tableStretch = true
	}
}

// WithProfileName records the active configuration profile for display in the status area.
func WithProfileName(name string) Option {
	return func(cfg *config) {
		cfg.profileName = strings.TrimSpace(name)
	}
}

// Render displays structured data using the Bubble Tea table component when
// the output stream is a TTY. For non-interactive streams it falls back to a
// static table rendering with truncated columns.
func Render(streams *iostreams.IOStreams, data any, opts ...Option) error {
	if streams == nil || streams.Out == nil {
		return errors.New("tableview: output stream is not available")
	}

	cfg := config{
		footer:        "",
		staticFooter:  "",
		quitKeys:      []string{"q", "Q", "ctrl+c"},
		toggleHelpKey: "?",
		initialRow:    -1,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	palette := theme.Current()
	tableStyle := newTableBoxStyle(palette)
	detailStyle := newDetailBoxStyle(palette)
	statusStyle := newStatusBoxStyle(palette)

	termWidth, termHeight, isTTY := resolveTerminal(streams.Out)

	var headers []string
	var matrix [][]string
	var tableRows []table.Row
	var err error

	if len(cfg.customRows) > 0 {
		headers = append([]string(nil), cfg.customHeaders...)
		matrix = rowsToMatrix(cfg.customRows)
		matrix = abbreviateMatrixIDs(headers, matrix)
		tableRows = convertRows(matrix, len(headers))
	} else {
		headers, matrix, err = buildRows(data)
		if err != nil {
			return err
		}
		matrix = abbreviateMatrixIDs(headers, matrix)
		tableRows = convertRows(matrix, len(headers))
	}

	if len(headers) == 0 {
		return writeStaticMessage(streams.Out, cfg.title, "No data to display.")
	}

	colWidths, minWidths := calculateColumnWidths(headers, matrix, termWidth)
	columns := make([]table.Column, len(headers))
	for i, header := range headers {
		columns[i] = table.Column{
			Title: header,
			Width: colWidths[i],
		}
	}

	if len(tableRows) == 0 {
		return writeStaticMessage(streams.Out, cfg.title, "No data to display.")
	}

	styles := table.DefaultStyles()
	styles.Header = styles.Header.
		Foreground(palette.Adaptive(theme.ColorTextPrimary)).
		Background(palette.Adaptive(theme.ColorSurface))
	styles.Cell = styles.Cell.
		Foreground(palette.Adaptive(theme.ColorTextPrimary))
	styles.Selected = styles.Selected.
		Foreground(palette.Adaptive(theme.ColorAccentText)).
		Background(palette.Adaptive(theme.ColorAccent))
	paddingWidth := max(
		lipgloss.Width(styles.Header.Render("")),
		lipgloss.Width(styles.Cell.Render("")),
	)

	if cfg.tableStretch && len(columns) > 0 && termWidth > 0 {
		frameWidth, _ := tableStyle.GetFrameSize()
		desired := termWidth - frameWidth - paddingWidth*len(columns)
		if desired > 0 {
			perColumn := desired / len(columns)
			if perColumn < 1 {
				perColumn = desired
			}
			for i := range columns {
				if columns[i].Width < perColumn {
					columns[i].Width = perColumn
				}
				colWidths[i] = columns[i].Width
			}
		}
	}

	keyMap := table.DefaultKeyMap()
	keyMap.LineUp = key.NewBinding(
		key.WithKeys("up", "k", "ctrl+p"),
		key.WithHelp("↑/k/ctrl+p", "up"),
	)
	keyMap.LineDown = key.NewBinding(
		key.WithKeys("down", "j", "ctrl+j"),
		key.WithHelp("↓/j/ctrl+j", "down"),
	)

	tbl := table.New(
		table.WithColumns(columns),
		table.WithRows(tableRows),
		table.WithFocused(true),
		table.WithStyles(styles),
		table.WithKeyMap(keyMap),
	)

	totalWidth := sum(colWidths) + paddingWidth*len(colWidths)
	minTotalWidth := sum(minWidths) + paddingWidth*len(minWidths)
	if termWidth > 0 && totalWidth > termWidth {
		if minTotalWidth > termWidth {
			totalWidth = minTotalWidth
		} else {
			totalWidth = termWidth
		}
	}
	if totalWidth > 0 {
		tbl.SetWidth(totalWidth)
	}

	if isTTY {
		tbl.Focus()
	} else {
		tbl.Blur()
		noHighlight := styles
		noHighlight.Selected = noHighlight.Cell
		tbl.SetStyles(noHighlight)
	}

	var preview string
	if cfg.previewRenderer != nil {
		index := clamp(tbl.Cursor(), 0, len(tableRows)-1)
		preview = cfg.previewRenderer(index)
	}

	previewHeight := 0
	if preview != "" {
		previewHeight = lipgloss.Height(preview)
	}

	setTableHeight(&tbl, len(tableRows), termHeight, isTTY, previewHeight)

	if cfg.detailRenderer != nil {
		cfg.hasDetail = true
		// account for the full frame (border, padding, margin) of both panels
		tableFrameWidth, _ := tableStyle.GetFrameSize()
		detailFrameWidth, _ := detailStyle.GetFrameSize()

		detailWidth := termWidth - tbl.Width() - tableFrameWidth - detailFrameWidth
		if termWidth <= 0 {
			detailWidth = 40
		}
		if detailWidth < 10 {
			detailWidth = 10
		}
		detailHeight := tbl.Height()
		if detailHeight <= 0 {
			detailHeight = len(tableRows) + 1
		}
		index := clamp(tbl.Cursor(), 0, len(tableRows)-1)
		content := ""
		if index >= 0 {
			rawDetail := cfg.detailRenderer(index)
			var parent any
			if cfg.detailContext != nil {
				parent = cfg.detailContext(index)
			}
			sanitized := sanitizePreviewDetailContent(rawDetail, cfg.childParentType, parent)
			content = stylizeDetailContent(sanitized, palette)
		}
		targetHeight := detailHeight
		contentHeight := lipgloss.Height(content)
		if contentHeight > 0 && (targetHeight <= 0 || contentHeight < targetHeight) {
			targetHeight = contentHeight
		}
		if targetHeight < 1 {
			targetHeight = 1
		}
		dv := viewport.New(detailWidth, targetHeight)
		dv.Width = detailWidth
		dv.Height = targetHeight
		dv.SetContent(content)
		cfg.detailViewport = &dv
	}

	if !isTTY {
		tableBox := borderedTableView(tableStyle, tbl.View(), styles.Selected)
		view := tableBox
		if cfg.detailRenderer != nil {
			index := clamp(tbl.Cursor(), 0, len(tableRows)-1)
			var detail string
			if index >= 0 {
				rawDetail := cfg.detailRenderer(index)
				var parent any
				if cfg.detailContext != nil {
					parent = cfg.detailContext(index)
				}
				sanitized := sanitizePreviewDetailContent(rawDetail, cfg.childParentType, parent)
				detail = stylizeDetailContent(sanitized, palette)
			}
			detailRendered := borderedDetailView(detailStyle, detail)
			view = lipgloss.JoinHorizontal(lipgloss.Top, tableBox, detailRendered)
		}
		var sections []string
		if preview != "" {
			sections = append(sections, preview)
		}
		if cfg.title != "" {
			sections = append(sections, cfg.title)
		}
		sections = append(sections, view)
		if cfg.staticFooter != "" {
			sections = append(sections, cfg.staticFooter)
		}
		output := lipgloss.JoinVertical(lipgloss.Left, sections...)
		_, err = fmt.Fprintln(streams.Out, output)
		return err
	}

	model := newBubbleModel(
		tbl,
		cfg,
		tableStyle,
		detailStyle,
		statusStyle,
		styles.Selected,
		cfg.previewRenderer,
		palette,
		termWidth,
		termHeight,
		len(tableRows),
		headers,
	)
	program := tea.NewProgram(model,
		tea.WithInput(streams.In),
		tea.WithOutput(streams.Out),
		tea.WithAltScreen(),
	)

	_, err = program.Run()
	return err
}

func writeStaticMessage(out io.Writer, title, message string) error {
	if out == nil {
		return errors.New("tableview: output stream is not available")
	}
	content := message
	if title != "" {
		content = lipgloss.JoinVertical(lipgloss.Left, title, message)
	}
	_, err := fmt.Fprintln(out, content)
	return err
}

func resolveTerminal(out io.Writer) (width int, height int, isTTY bool) {
	const defaultWidth = 120
	const defaultHeight = 24

	width, height = defaultWidth, defaultHeight

	fd, ok := getFD(out)
	if !ok {
		return width, height, false
	}

	isTTY = isTerminal(fd)

	if w, h, err := term.GetSize(int(fd)); err == nil {
		width, height = w, h
	}

	return width, height, isTTY
}

func getFD(w io.Writer) (uintptr, bool) {
	if fp, ok := w.(fdProvider); ok {
		fd := fp.Fd()
		if fd == ^uintptr(0) {
			return 0, false
		}
		return fd, true
	}
	return 0, false
}

func isTerminal(fd uintptr) bool {
	return isatty.IsTerminal(fd) || isatty.IsCygwinTerminal(fd)
}

func buildRows(data any) ([]string, [][]string, error) {
	if data == nil {
		return nil, nil, errors.New("tableview: nil data provided")
	}

	value := reflect.ValueOf(data)
	value = deref(value)

	kind := value.Kind()
	if kind == reflect.Slice || kind == reflect.Array {
		return rowsFromSlice(value)
	}
	if kind == reflect.Struct {
		slice := reflect.MakeSlice(reflect.SliceOf(value.Type()), 0, 1)
		slice = reflect.Append(slice, value)
		return rowsFromSlice(slice)
	}
	return nil, nil, fmt.Errorf("tableview: unsupported data kind %s", value.Kind())
}

func rowsFromSlice(slice reflect.Value) ([]string, [][]string, error) {
	if slice.Kind() != reflect.Slice && slice.Kind() != reflect.Array {
		return nil, nil, fmt.Errorf("tableview: expected slice kind but received %s", slice.Kind())
	}

	length := slice.Len()
	if length == 0 {
		elemType := derefType(slice.Type().Elem())
		if elemType.Kind() != reflect.Struct {
			return nil, nil, fmt.Errorf("tableview: slice element kind %s is unsupported", elemType.Kind())
		}
		meta := extractStructMeta(elemType)
		return meta.headers, nil, nil
	}

	first := deref(slice.Index(0))
	if first.Kind() != reflect.Struct {
		return nil, nil, fmt.Errorf("tableview: slice element kind %s is unsupported", first.Kind())
	}

	meta := extractStructMeta(first.Type())
	headers := meta.headers
	rowCount := slice.Len()
	rows := make([][]string, 0, rowCount)

	for i := 0; i < rowCount; i++ {
		item := deref(slice.Index(i))
		if !item.IsValid() {
			continue
		}
		row := make([]string, len(meta.indices))
		for j, fieldIndex := range meta.indices {
			if fieldIndex >= item.NumField() {
				row[j] = ""
				continue
			}
			field := item.Field(fieldIndex)
			if !field.CanInterface() {
				row[j] = ""
				continue
			}
			row[j] = fmt.Sprint(field.Interface())
		}
		rows = append(rows, row)
	}

	return headers, rows, nil
}

func deref(value reflect.Value) reflect.Value {
	for value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return reflect.Value{}
		}
		value = value.Elem()
	}
	return value
}

func derefType(t reflect.Type) reflect.Type {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t
}

type structMeta struct {
	headers []string
	indices []int
}

func extractStructMeta(t reflect.Type) structMeta {
	meta := structMeta{
		headers: make([]string, 0, t.NumField()),
		indices: make([]int, 0, t.NumField()),
	}
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.PkgPath != "" {
			continue
		}
		meta.headers = append(meta.headers, formatHeader(field.Name))
		meta.indices = append(meta.indices, i)
	}
	return meta
}

func formatHeader(field string) string {
	if field == "" {
		return field
	}

	var words []string
	var current strings.Builder

	runes := []rune(field)

	for i, r := range runes {
		if r == '_' {
			words = appendWord(words, current.String())
			current.Reset()
			continue
		}

		if i == 0 {
			current.WriteRune(r)
			continue
		}

		prev := runes[i-1]
		var next rune
		if i+1 < len(runes) {
			next = runes[i+1]
		}

		if shouldBreak(prev, r, next) {
			words = appendWord(words, current.String())
			current.Reset()
		}

		current.WriteRune(r)
	}

	words = appendWord(words, current.String())
	for i, w := range words {
		words[i] = strings.ToUpper(strings.TrimSpace(w))
	}
	return strings.Join(words, " ")
}

func shouldBreak(prev, current, next rune) bool {
	isUpper := unicode.IsUpper(current)
	prevIsUpper := unicode.IsUpper(prev)
	nextIsLower := unicode.IsLower(next)

	if unicode.IsDigit(current) && !unicode.IsDigit(prev) {
		return true
	}

	if !isUpper {
		return false
	}

	if !prevIsUpper {
		return true
	}

	return nextIsLower
}

func appendWord(words []string, word string) []string {
	word = strings.TrimSpace(word)
	if word == "" {
		return words
	}
	return append(words, word)
}

func convertRows(rows [][]string, columnCount int) []table.Row {
	renderRows := make([]table.Row, len(rows))
	for i, row := range rows {
		record := make(table.Row, columnCount)
		for j := 0; j < columnCount; j++ {
			if j < len(row) {
				record[j] = row[j]
			} else {
				record[j] = ""
			}
		}
		renderRows[i] = record
	}
	return renderRows
}

func abbreviateMatrixIDs(headers []string, rows [][]string) [][]string {
	if len(headers) == 0 || len(rows) == 0 {
		return rows
	}

	idCols := identifyIDColumns(headers)
	if len(idCols) == 0 {
		return rows
	}

	result := make([][]string, len(rows))
	for i, row := range rows {
		copyRow := append([]string(nil), row...)
		for _, idx := range idCols {
			if idx < len(copyRow) {
				copyRow[idx] = abbreviateIDValue(copyRow[idx])
			}
		}
		result[i] = copyRow
	}
	return result
}

func identifyIDColumns(headers []string) []int {
	indices := make([]int, 0, len(headers))
	for i, header := range headers {
		key := normalizeHeaderKey(header)
		if isIDHeaderKey(key) {
			indices = append(indices, i)
		}
	}
	return indices
}

func isIDHeaderKey(key string) bool {
	key = strings.TrimSpace(key)
	if key == "" {
		return false
	}
	switch key {
	case "id", "uuid", "uid", "identifier":
		return true
	}
	for _, suffix := range []string{" id", " uuid", " uid", " identifier"} {
		if strings.HasSuffix(key, suffix) {
			return true
		}
	}
	return false
}

func truncateWithEllipsis(s string, maxLen int) string {
	if maxLen <= 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen == 1 {
		return "…"
	}
	return string(runes[:maxLen-1]) + "…"
}

func abbreviateIDValue(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return value
	}
	if !isLikelyUUID(trimmed) {
		return value
	}
	return truncateWithEllipsis(trimmed, 5)
}

func rowsToMatrix(rows []table.Row) [][]string {
	matrix := make([][]string, len(rows))
	for i, row := range rows {
		matrix[i] = append([]string(nil), row...)
	}
	return matrix
}

func buildHeaderLookup(headers []string) map[string]int {
	lookup := make(map[string]int, len(headers))
	for i, header := range headers {
		key := normalizeHeaderKey(header)
		if key == "" {
			continue
		}
		if _, exists := lookup[key]; !exists {
			lookup[key] = i
		}
	}
	return lookup
}

func normalizeHeaderKey(header string) string {
	header = strings.TrimSpace(header)
	if header == "" {
		return ""
	}
	header = strings.ReplaceAll(header, "_", " ")
	header = strings.ReplaceAll(header, "-", " ")
	fields := strings.Fields(strings.ToLower(header))
	return strings.Join(fields, " ")
}

func abbreviateValue(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 {
		return value
	}

	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}

	if isLikelyUUID(value) && limit >= 5 {
		return truncateWithEllipsis(value, 5)
	}

	if limit <= 1 {
		return string(runes[:limit])
	}

	return truncateWithEllipsis(value, limit)
}

func isLikelyUUID(value string) bool {
	if value == "" {
		return false
	}
	trimmed := strings.TrimSpace(value)
	if len(trimmed) == 36 {
		for i, r := range trimmed {
			switch i {
			case 8, 13, 18, 23:
				if r != '-' {
					return false
				}
			default:
				if !isHexRune(r) {
					return false
				}
			}
		}
		return true
	}
	if len(trimmed) == 32 {
		for _, r := range trimmed {
			if !isHexRune(r) {
				return false
			}
		}
		return true
	}
	return false
}

func isHexRune(r rune) bool {
	switch {
	case r >= '0' && r <= '9':
		return true
	case r >= 'a' && r <= 'f':
		return true
	case r >= 'A' && r <= 'F':
		return true
	default:
		return false
	}
}

func renderMarkdownContent(raw string, width int) string {
	if width <= 0 {
		width = 80
	}
	return kairender.Markdown(raw, kairender.Options{Width: width})
}

func defaultRootLabel(_ []string) string {
	return "Items"
}

var rootLabelOverrides = map[string]string{
	"api":           "apis",
	"portal":        "portals",
	"auth-strategy": "auth-strategies",
	"control-plane": "control-planes",
}

func formatRootLabel(label string) string {
	label = strings.TrimSpace(label)
	if label == "" {
		return ""
	}
	key := strings.ToLower(strings.ReplaceAll(label, "_", "-"))
	if override, ok := rootLabelOverrides[key]; ok {
		return override
	}
	return label
}

type detailItem struct {
	Label  string
	Value  string
	Loader ChildLoader
}

type detailTableDecorator struct {
	columns        []table.Column
	cellStyles     []lipgloss.Style
	labelStyle     lipgloss.Style
	valueStyle     lipgloss.Style
	accentStyle    lipgloss.Style
	selectedLabel  lipgloss.Style
	selectedValue  lipgloss.Style
	selectedAccent lipgloss.Style
	selectedStyle  lipgloss.Style
	selectedPrefix string
	highlight      bool
}

func newDetailTableDecorator(
	columns []table.Column,
	palette theme.Palette,
	selected lipgloss.Style,
	highlight bool,
) detailTableDecorator {
	if len(columns) == 0 {
		return detailTableDecorator{}
	}

	cellStyles := make([]lipgloss.Style, len(columns))
	for i, col := range columns {
		cellStyles[i] = lipgloss.NewStyle().
			Width(col.Width).
			MaxWidth(col.Width).
			Inline(true)
	}

	labelStyle := palette.ForegroundStyle(theme.ColorTextSecondary)
	valueStyle := palette.ForegroundStyle(theme.ColorTextPrimary)
	accentStyle := palette.ForegroundStyle(theme.ColorAccent)
	selectedText := palette.ForegroundStyle(theme.ColorAccentText)

	const reset = "\x1b[0m"
	return detailTableDecorator{
		columns:        append([]table.Column(nil), columns...),
		cellStyles:     cellStyles,
		labelStyle:     labelStyle,
		valueStyle:     valueStyle,
		accentStyle:    accentStyle,
		selectedLabel:  selectedText,
		selectedValue:  selectedText,
		selectedAccent: selectedText,
		selectedStyle:  selected,
		selectedPrefix: selectionPrefix(selected, reset),
		highlight:      highlight,
	}
}

func (d detailTableDecorator) isReady() bool {
	return len(d.columns) > 0 && len(d.columns) == len(d.cellStyles)
}

func (d detailTableDecorator) stylize(view string) string {
	if !d.isReady() || view == "" {
		return view
	}

	lines := strings.Split(view, "\n")
	if len(lines) <= 1 {
		return view
	}

	rendered := make([]string, 0, len(lines))
	rendered = append(rendered, lines[0])

	totalWidth := 0
	for _, col := range d.columns {
		totalWidth += col.Width
	}

	for _, line := range lines[1:] {
		plain := ansi.Strip(line)
		if strings.TrimSpace(plain) == "" {
			rendered = append(rendered, line)
			continue
		}

		if ansi.StringWidth(plain) < totalWidth {
			padding := strings.Repeat(" ", totalWidth-ansi.StringWidth(plain))
			plain += padding
		}

		colValues := make([]string, len(d.columns))
		start := 0
		for i, col := range d.columns {
			end := start + col.Width
			segment := ansi.Cut(plain, start, end)
			colValues[i] = segment
			start = end
		}

		isSelected := d.highlight && d.selectedPrefix != "" && strings.Contains(line, d.selectedPrefix)

		renderedCols := make([]string, len(colValues))
		for i, value := range colValues {
			display := strings.TrimRight(value, " ")
			// Placeholder for potential indicator logic (removed caret indicator)
			cell := d.cellStyles[i].Render(display)

			style := d.valueStyle
			if isSelected {
				style = d.selectedValue
			}

			switch i {
			case 0:
				style = d.labelStyle
				if isSelected {
					style = d.selectedLabel
				}
			case 1:
				labelText := strings.TrimSpace(colValues[0])
				valueText := strings.TrimSpace(value)
				if shouldAccentDetail(labelText, valueText) {
					style = d.accentStyle
					if isSelected {
						style = d.selectedAccent
					}
				}
			default:
			}
			renderedCols[i] = style.Render(cell)
		}

		row := lipgloss.JoinHorizontal(lipgloss.Top, renderedCols...)
		if d.highlight && isSelected {
			row = NormalizeSelectedRow(d.selectedStyle.Render(row), d.selectedStyle)
		}

		rendered = append(rendered, row)
	}

	return strings.Join(rendered, "\n")
}

const (
	childFieldIndicator        = "[...]"
	complexNilIndicator        = "[nil]"
	complexEmptyIndicator      = "[]"
	complexExpandableIndicator = "[...]"
	complexStructIndicator     = "{...}"
)

type mapEntry struct {
	Key     string
	Value   any
	Summary string
}

func parseDetailContent(content string) []detailItem {
	lines := strings.Split(content, "\n")
	items := make([]detailItem, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			continue
		}

		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		indent := len(line) - len(strings.TrimLeft(line, " \t"))

		if indent == 0 {
			idx := strings.Index(trimmed, ":")
			isField := idx != -1 && (idx == len(trimmed)-1 || trimmed[idx+1] == ' ')
			if isField {
				label := strings.TrimSpace(trimmed[:idx])
				value := strings.TrimSpace(trimmed[idx+1:])
				items = append(items, detailItem{
					Label: label,
					Value: value,
				})
				continue
			}

			if len(items) == 0 {
				continue
			}

			prev := &items[len(items)-1]
			if prev.Value == "" {
				prev.Value = trimmed
			} else {
				prev.Value += "\n" + trimmed
			}
			continue
		}

		if len(items) == 0 {
			continue
		}

		prev := &items[len(items)-1]
		if prev.Value == "" {
			prev.Value = "..."
		}
	}
	return items
}

func sanitizePreviewDetailContent(raw string, parentType string, parent any) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}

	items := parseDetailContent(raw)
	if len(items) == 0 {
		return raw
	}

	enriched := enrichDetailItems(items, parentType, parent)
	filtered := filterPreviewDetailItems(enriched)
	if len(filtered) == 0 {
		return raw
	}

	return renderDetailItems(filtered)
}

func filterPreviewDetailItems(items []detailItem) []detailItem {
	filtered := make([]detailItem, 0, len(items))
	for _, item := range items {
		label := strings.TrimSpace(item.Label)
		if label == "" {
			continue
		}

		value := strings.TrimSpace(item.Value)
		if item.Loader != nil {
			continue
		}
		if isPreviewIndicatorValue(value) {
			continue
		}

		filtered = append(filtered, detailItem{
			Label: item.Label,
			Value: item.Value,
		})
	}
	return filtered
}

func isPreviewIndicatorValue(value string) bool {
	switch value {
	case childFieldIndicator, complexStructIndicator, complexNilIndicator, complexEmptyIndicator:
		return true
	default:
		return false
	}
}

func renderDetailItems(items []detailItem) string {
	if len(items) == 0 {
		return ""
	}

	var builder strings.Builder
	for _, item := range items {
		label := strings.TrimSpace(item.Label)
		if label == "" {
			continue
		}

		value := item.Value
		if strings.TrimSpace(value) == "" {
			builder.WriteString(label)
			builder.WriteString(":\n")
			continue
		}

		lines := strings.Split(value, "\n")
		if len(lines) == 1 {
			fmt.Fprintf(&builder, "%s: %s\n", label, strings.TrimSpace(lines[0]))
			continue
		}

		fmt.Fprintf(&builder, "%s:\n", label)
		for _, line := range lines {
			trimmed := strings.TrimRight(line, " \t")
			if strings.TrimSpace(trimmed) == "" {
				builder.WriteString("\n")
				continue
			}
			builder.WriteString("  ")
			builder.WriteString(trimmed)
			builder.WriteString("\n")
		}
	}

	return strings.TrimRight(builder.String(), "\n")
}

func enrichDetailItems(items []detailItem, parentType string, parent any) []detailItem {
	items = applyRegisteredChildLoaders(items, parentType)
	items = ensureRegisteredChildFields(items, parentType)
	items = applyComplexValueLoaders(items, parent)
	return items
}

func applyRegisteredChildLoaders(items []detailItem, parentType string) []detailItem {
	if parentType == "" {
		return items
	}
	for i := range items {
		if loader := getChildLoader(parentType, items[i].Label); loader != nil {
			items[i].Loader = loader
			items[i].Value = childFieldIndicator
		}
	}
	return items
}

func ensureRegisteredChildFields(items []detailItem, parentType string) []detailItem {
	if parentType == "" {
		return items
	}

	for _, reg := range childLoaderFields(parentType) {
		if detailItemsContain(items, reg.field) {
			continue
		}
		items = append(items, detailItem{
			Label:  reg.field,
			Value:  childFieldIndicator,
			Loader: reg.loader,
		})
	}
	return items
}

func applyComplexValueLoaders(items []detailItem, parent any) []detailItem {
	if parent == nil {
		return items
	}

	parentValue := reflect.ValueOf(parent)
	for i := range items {
		info, ok := complexValueInfoForItem(parentValue, items[i].Label)
		if !ok {
			continue
		}
		if info.indicator != "" {
			items[i].Value = info.indicator
		}
		if info.loader != nil {
			items[i].Loader = info.loader
		}
	}
	return items
}

type complexValueInfo struct {
	indicator string
	loader    ChildLoader
}

func complexValueInfoForItem(parent reflect.Value, label string) (complexValueInfo, bool) {
	if !parent.IsValid() {
		return complexValueInfo{}, false
	}

	value, ok := valueForLabel(parent, label)
	if !ok {
		return complexValueInfo{}, false
	}

	info, handled := analyzeComplexValue(label, value)
	return info, handled
}

func valueForLabel(parent reflect.Value, label string) (reflect.Value, bool) {
	parent = derefValueDeep(parent)
	if !parent.IsValid() {
		return reflect.Value{}, false
	}

	kind := parent.Kind()
	if kind == reflect.Struct {
		target := normalizeHeaderKey(label)
		typ := parent.Type()
		targetCompact := strings.ReplaceAll(target, " ", "")
		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			if field.PkgPath != "" {
				continue
			}
			fieldLabel := normalizeHeaderKey(formatHeader(field.Name))
			fieldCompact := strings.ReplaceAll(fieldLabel, " ", "")
			if fieldLabel == target || fieldCompact == targetCompact {
				return parent.Field(i), true
			}
		}
	} else if kind == reflect.Map {
		iter := parent.MapRange()
		target := normalizeHeaderKey(label)
		for iter.Next() {
			keyStr := normalizeHeaderKey(fmt.Sprint(iter.Key().Interface()))
			if keyStr == target {
				return iter.Value(), true
			}
		}
	}

	return reflect.Value{}, false
}

func derefValueDeep(value reflect.Value) reflect.Value {
	for value.IsValid() {
		kind := value.Kind()
		if kind != reflect.Pointer && kind != reflect.Interface {
			return value
		}
		if value.IsNil() {
			return reflect.Value{}
		}
		value = value.Elem()
	}
	return reflect.Value{}
}

func analyzeComplexValue(label string, value reflect.Value) (complexValueInfo, bool) {
	if !value.IsValid() {
		return complexValueInfo{indicator: complexNilIndicator}, true
	}

	original := value
	value = derefValueDeep(value)
	if !value.IsValid() {
		return complexValueInfo{indicator: complexNilIndicator}, true
	}

	//exhaustive:ignore
	switch value.Kind() {
	case reflect.Map:
		indicator := complexStructIndicator
		var entries []mapEntry
		if value.IsNil() {
			indicator = complexNilIndicator
		} else {
			entries = mapEntriesFromValue(value)
			if len(entries) == 0 {
				indicator = complexEmptyIndicator
			}
		}

		data := value.Interface()
		return complexValueInfo{
			indicator: indicator,
			loader: func(_ context.Context, _ cmdpkg.Helper, _ any) (ChildView, error) {
				return buildChildViewForMap(label, entries, data), nil
			},
		}, true
	case reflect.Slice, reflect.Array:
		data := original.Interface()
		indicator := complexExpandableIndicator
		if value.Kind() == reflect.Slice && value.IsNil() {
			indicator = complexNilIndicator
		} else if value.Len() == 0 {
			indicator = complexEmptyIndicator
		}
		return complexValueInfo{
			indicator: indicator,
			loader: func(_ context.Context, _ cmdpkg.Helper, _ any) (ChildView, error) {
				return buildChildViewFromSliceValue(label, data)
			},
		}, true
	case reflect.Struct:
		typ := value.Type()
		if typ.PkgPath() == "time" {
			return complexValueInfo{}, false
		}
		structValue := value.Interface()
		return complexValueInfo{
			indicator: complexStructIndicator,
			loader: func(_ context.Context, _ cmdpkg.Helper, parent any) (ChildView, error) {
				if parent != nil {
					parentValue := reflect.ValueOf(parent)
					if fieldValue, ok := valueForLabel(parentValue, label); ok && fieldValue.IsValid() {
						return buildChildViewFromStructValue(label, fieldValue.Interface())
					}
				}
				return buildChildViewFromStructValue(label, structValue)
			},
		}, true
	default:
		return complexValueInfo{}, false
	}
}

func mapEntriesFromValue(value reflect.Value) []mapEntry {
	keys := value.MapKeys()
	entries := make([]mapEntry, 0, len(keys))
	for _, key := range keys {
		val := value.MapIndex(key)
		entry := mapEntry{
			Key:   fmt.Sprint(key.Interface()),
			Value: nil,
		}
		if val.IsValid() {
			entry.Value = val.Interface()
			entry.Summary = summarizeValue(entry.Value)
		} else {
			entry.Summary = "nil"
		}
		entries = append(entries, entry)
	}

	sort.Slice(entries, func(i, j int) bool {
		return strings.Compare(entries[i].Key, entries[j].Key) < 0
	})
	return entries
}

func writeClipboardText(value string) error {
	return clipboard.WriteAll(value)
}

func formatStatusValue(value string) string {
	clean := strings.TrimSpace(value)
	if clean == "" {
		return "(empty)"
	}
	clean = strings.ReplaceAll(clean, "\n", " ")
	clean = strings.Join(strings.Fields(clean), " ")
	return abbreviateValue(clean, 60)
}

func buildChildViewForMap(label string, entries []mapEntry, data any) ChildView {
	detail := func(int) string {
		if len(entries) == 0 {
			return "(no data)"
		}
		var builder strings.Builder
		for _, entry := range entries {
			value := formatDetailValue(entry.Value)
			if value == "" {
				value = "(empty)"
			}
			fmt.Fprintf(&builder, "%s: %s\n", entry.Key, value)
		}
		content := strings.TrimRight(builder.String(), "\n")
		if content == "" {
			return "(no data)"
		}
		return content
	}

	contextProvider := func(int) any {
		return data
	}

	return ChildView{
		DetailRenderer: detail,
		Title:          titleFromLabel(label),
		ParentType:     normalizeHeaderKey(label),
		DetailContext:  contextProvider,
		Mode:           ChildViewModeDetail,
	}
}

func buildChildViewFromSliceValue(label string, data any) (ChildView, error) {
	value := reflect.ValueOf(data)
	value = derefValueDeep(value)
	if !value.IsValid() {
		return ChildView{}, fmt.Errorf("tableview: invalid slice data for %s", label)
	}

	length := value.Len()
	if length == 0 {
		return ChildView{
			Headers:        []string{"VALUE"},
			Rows:           nil,
			DetailRenderer: func(int) string { return "(no data)" },
			Title:          titleFromLabel(label),
			ParentType:     normalizeHeaderKey(label),
		}, nil
	}

	elemType := value.Type().Elem()
	elemType = derefType(elemType)

	if elemType.Kind() == reflect.Struct {
		headers, matrix, err := buildRows(data)
		if err != nil {
			return ChildView{}, err
		}
		rows := convertRows(matrix, len(headers))
		return ChildView{
			Headers: rowsHeaders(headers),
			Rows:    rows,
			DetailRenderer: func(index int) string {
				if !isValidIndex(index, length) {
					return ""
				}
				return renderStructDetail(value.Index(index).Interface())
			},
			Title:      titleFromLabel(label),
			ParentType: normalizeHeaderKey(label),
			DetailContext: func(index int) any {
				if !isValidIndex(index, length) {
					return nil
				}
				return value.Index(index).Interface()
			},
		}, nil
	}
	rows := make([]table.Row, length)
	values := make([]any, length)
	for i := 0; i < length; i++ {
		entry := value.Index(i)
		val := entry.Interface()
		values[i] = val
		rows[i] = table.Row{
			strconv.Itoa(i + 1),
			summarizeValue(val),
		}
	}
	return ChildView{
		Headers: []string{"#", "VALUE"},
		Rows:    rows,
		DetailRenderer: func(index int) string {
			if !isValidIndex(index, len(values)) {
				return ""
			}
			return fmt.Sprintf("value: %s", formatDetailValue(values[index]))
		},
		Title:      titleFromLabel(label),
		ParentType: normalizeHeaderKey(label),
		DetailContext: func(index int) any {
			if !isValidIndex(index, len(values)) {
				return nil
			}
			return values[index]
		},
	}, nil
}

func buildChildViewFromStructValue(label string, data any) (ChildView, error) {
	value := reflect.ValueOf(data)
	value = derefValueDeep(value)
	if !value.IsValid() {
		return ChildView{}, fmt.Errorf("tableview: invalid struct data for %s", label)
	}
	if value.Kind() != reflect.Struct {
		return ChildView{}, fmt.Errorf("tableview: struct loader expected struct value for %s", label)
	}

	copied := value.Interface()
	rendered := renderStructDetail(copied)
	if strings.TrimSpace(rendered) == "" {
		rendered = "(no data)"
	}

	return ChildView{
		DetailRenderer: func(int) string { return rendered },
		Title:          titleFromLabel(label),
		ParentType:     normalizeHeaderKey(label),
		DetailContext:  func(int) any { return copied },
		Mode:           ChildViewModeDetail,
	}, nil
}

func rowsHeaders(headers []string) []string {
	copied := make([]string, len(headers))
	copy(copied, headers)
	return copied
}

func titleFromLabel(label string) string {
	label = strings.TrimSpace(label)
	if label == "" {
		return ""
	}
	segments := strings.FieldsFunc(label, func(r rune) bool {
		return r == '_' || r == '-' || unicode.IsSpace(r)
	})
	for i := range segments {
		if segments[i] == "" {
			continue
		}
		segments[i] = strings.ToUpper(segments[i][:1]) + strings.ToLower(segments[i][1:])
	}
	return strings.Join(segments, " ")
}

func summarizeValue(val any) string {
	if val == nil {
		return "nil"
	}
	switch v := val.(type) {
	case string:
		return abbreviateValue(v, 60)
	default:
		return abbreviateValue(fmt.Sprint(val), 60)
	}
}

func formatDetailValue(val any) string {
	if val == nil {
		return "nil"
	}

	rv := reflect.ValueOf(val)
	rv = derefValueDeep(rv)
	if !rv.IsValid() {
		return "nil"
	}

	kind := rv.Kind()
	if kind == reflect.Slice || kind == reflect.Array {
		if rv.Len() == 0 {
			return "[]"
		}
		var builder strings.Builder
		for i := 0; i < rv.Len(); i++ {
			fmt.Fprintf(&builder, "- %v\n", rv.Index(i).Interface())
		}
		return strings.TrimRight(builder.String(), "\n")
	}
	if kind == reflect.Map {
		if rv.Len() == 0 {
			return "{}"
		}
		keys := rv.MapKeys()
		type keyValue struct {
			keyStr string
			key    reflect.Value
		}
		kvs := make([]keyValue, len(keys))
		for i, k := range keys {
			kvs[i] = keyValue{
				keyStr: fmt.Sprint(k.Interface()),
				key:    k,
			}
		}
		sort.Slice(kvs, func(i, j int) bool {
			return kvs[i].keyStr < kvs[j].keyStr
		})
		var builder strings.Builder
		for _, kv := range kvs {
			fmt.Fprintf(&builder, "%v: %v\n", kv.key.Interface(), rv.MapIndex(kv.key).Interface())
		}
		return strings.TrimRight(builder.String(), "\n")
	}
	if kind == reflect.Struct {
		return renderStructDetail(rv.Interface())
	}
	return fmt.Sprint(val)
}

func renderStructDetail(data any) string {
	if data == nil {
		return ""
	}

	value := reflect.ValueOf(data)
	value = derefValueDeep(value)
	if !value.IsValid() {
		return ""
	}

	typ := value.Type()
	var builder strings.Builder
	for i := 0; i < value.NumField(); i++ {
		field := typ.Field(i)
		if field.PkgPath != "" {
			continue
		}
		label := strings.ToLower(strings.ReplaceAll(formatHeader(field.Name), " ", "_"))
		fieldValue := value.Field(i)
		builder.WriteString(label)
		builder.WriteString(": ")
		builder.WriteString(formatDetailValue(fieldValue.Interface()))
		builder.WriteString("\n")
	}
	return strings.TrimRight(builder.String(), "\n")
}

func buildDetailTable(
	items []detailItem,
	width int,
	height int,
	palette theme.Palette,
	highlight bool,
) (table.Model, detailTableDecorator) {
	if len(items) == 0 {
		items = append(items, detailItem{
			Label: "(no data)",
			Value: "",
		})
	}

	maxLabel := 10
	for _, item := range items {
		if w := runewidth.StringWidth(item.Label); w > maxLabel {
			maxLabel = w
		}
	}

	const minLabelWidth = 12
	const maxLabelWidth = 40
	labelWidth := clamp(maxLabel+2, minLabelWidth, maxLabelWidth)

	if width <= 0 {
		width = labelWidth + 40
	}

	valueWidth := width - labelWidth - 4
	if valueWidth < 20 {
		valueWidth = 20
	}

	columns := []table.Column{
		{Title: "FIELD", Width: labelWidth},
		{Title: "VALUE", Width: valueWidth},
	}

	rows := make([]table.Row, len(items))
	for i, item := range items {
		value := strings.ReplaceAll(item.Value, "\n", " ")
		value = strings.Join(strings.Fields(value), " ")
		rows[i] = table.Row{item.Label, value}
	}

	styles := table.DefaultStyles()
	styles.Header = styles.Header.
		PaddingLeft(0).
		PaddingRight(0).
		Foreground(palette.Adaptive(theme.ColorTextPrimary)).
		Background(palette.Adaptive(theme.ColorSurface))
	styles.Cell = styles.Cell.
		PaddingLeft(0).
		PaddingRight(0)
	if highlight {
		styles.Selected = styles.Selected.
			Foreground(palette.Adaptive(theme.ColorAccentText)).
			Background(palette.Adaptive(theme.ColorAccent))
	} else {
		styles.Selected = lipgloss.NewStyle()
	}

	tbl := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithStyles(styles),
	)

	setTableHeight(&tbl, len(rows), height, true, 0)
	if width > 0 {
		tbl.SetWidth(width)
	}

	decorator := newDetailTableDecorator(columns, palette, styles.Selected, highlight)
	return tbl, decorator
}

func buildChildTable(state *childViewState, width, height int, palette theme.Palette) table.Model {
	if state == nil {
		return table.New()
	}

	headers := append([]string(nil), state.headers...)
	if len(headers) == 0 {
		headers = []string{""}
	}

	rows := make([]table.Row, len(state.rows))
	for i, row := range state.rows {
		rows[i] = append(table.Row(nil), row...)
	}

	matrix := rowsToMatrix(rows)
	matrix = abbreviateMatrixIDs(headers, matrix)
	colWidths, minWidths := calculateColumnWidths(headers, matrix, width)

	columns := make([]table.Column, len(headers))
	for i, header := range headers {
		columns[i] = table.Column{
			Title: header,
			Width: colWidths[i],
		}
	}

	styles := table.DefaultStyles()
	styles.Header = styles.Header.
		Foreground(palette.Adaptive(theme.ColorTextPrimary)).
		Background(palette.Adaptive(theme.ColorSurface))
	styles.Cell = styles.Cell.
		Foreground(palette.Adaptive(theme.ColorTextPrimary))
	styles.Selected = styles.Selected.
		Foreground(palette.Adaptive(theme.ColorAccentText)).
		Background(palette.Adaptive(theme.ColorAccent))

	tbl := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithStyles(styles),
	)

	paddingWidth := max(
		lipgloss.Width(styles.Header.Render("")),
		lipgloss.Width(styles.Cell.Render("")),
	)

	totalWidth := sum(colWidths) + paddingWidth*len(colWidths)
	minTotalWidth := sum(minWidths) + paddingWidth*len(minWidths)
	if width > 0 && totalWidth > width {
		if minTotalWidth > width {
			totalWidth = minTotalWidth
		} else {
			totalWidth = width
		}
	}
	if totalWidth > 0 {
		tbl.SetWidth(totalWidth)
	}

	setTableHeight(&tbl, len(rows), height, true, 0)
	tbl.Focus()

	return tbl
}

func quoteBreadcrumbSegment(segment string) string {
	trimmed := strings.TrimSpace(segment)
	if trimmed == "" {
		return ""
	}
	if strings.ContainsAny(trimmed, " >") {
		return fmt.Sprintf("%q", trimmed)
	}
	return trimmed
}

func calculateColumnWidths(headers []string, rows [][]string, widthLimit int) ([]int, []int) {
	const minColumnWidth = 6
	const maxColumnWidth = 60

	widths := make([]int, len(headers))
	minWidths := make([]int, len(headers))
	for i, header := range headers {
		headerWidth := runewidth.StringWidth(header)
		minWidth := clamp(headerWidth, minColumnWidth, maxColumnWidth)
		minWidths[i] = minWidth

		maxWidth := headerWidth
		for _, row := range rows {
			if i < len(row) {
				if w := runewidth.StringWidth(row[i]); w > maxWidth {
					maxWidth = w
				}
			}
		}
		maxWidth = clamp(maxWidth, minColumnWidth, maxColumnWidth)
		if maxWidth < minWidth {
			maxWidth = minWidth
		}
		widths[i] = maxWidth
	}

	if widthLimit <= 0 {
		return widths, minWidths
	}

	total := sum(widths)
	for total > widthLimit {
		idx := widestColumnAboveMin(widths, minWidths)
		if idx == -1 {
			break
		}
		widths[idx]--
		total--
	}

	return widths, minWidths
}

func sum(values []int) int {
	total := 0
	for _, v := range values {
		total += v
	}
	return total
}

func widestColumnAboveMin(widths, minWidths []int) int {
	idx := -1
	maxWidth := math.MinInt
	for i, width := range widths {
		if width > maxWidth && width > minWidths[i] {
			maxWidth = width
			idx = i
		}
	}
	return idx
}

func clamp(val, minVal, maxVal int) int {
	if val < minVal {
		return minVal
	}
	if val > maxVal {
		return maxVal
	}
	return val
}

func isValidIndex(index, length int) bool {
	return index >= 0 && index < length
}

func setTableHeight(tbl *table.Model, rowCount, termHeight int, interactive bool, reservedHeight int) {
	if tbl == nil {
		return
	}

	if !interactive {
		tbl.SetHeight(rowCount + 1) // include header
		return
	}

	const minHeight = 3
	const margin = 4

	target := rowCount + 1
	if termHeight > 0 {
		available := termHeight - margin - reservedHeight
		if available < minHeight {
			available = minHeight
		}
		target = clamp(target, minHeight, available)
	}
	tbl.SetHeight(target)
}

// RenderForFormat renders structured data according to the requested output format.
// For interactive output it delegates to Render, otherwise it uses the provided printer.
func RenderForFormat(
	helper cmdpkg.Helper,
	interactive bool,
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	streams *iostreams.IOStreams,
	display any,
	raw any,
	title string,
	extraOpts ...Option,
) error {
	if helper != nil {
		cfg, err := helper.GetConfig()
		if err != nil {
			return err
		}

		settings, err := jqoutput.ResolveSettings(helper.GetCmd(), cfg)
		if err != nil {
			return err
		}

		if err := jqoutput.ValidateOutputFormat(outType, settings); err != nil {
			return err
		}

		if jqoutput.HasFilter(settings) {
			if interactive {
				return &cmdpkg.ConfigurationError{
					Err: fmt.Errorf(
						"--%s is not supported for interactive output; use --output json or --output yaml",
						jqoutput.FlagName,
					),
				}
			}

			filteredRaw, handled, err := jqoutput.ApplyToRaw(raw, outType, settings, streams.Out)
			if err != nil {
				return cmdpkg.PrepareExecutionErrorWithHelper(helper, "jq filter failed", err)
			}
			if handled {
				return nil
			}
			raw = filteredRaw
		}
	}

	if interactive {
		var opts []Option
		if title != "" {
			opts = append(opts, WithTitle(title))
		}
		opts = append(opts, extraOpts...)
		return Render(streams, display, opts...)
	}

	switch outType {
	case cmdCommon.TEXT:
		if printer != nil {
			printer.Print(display)
		}
		return nil
	case cmdCommon.JSON, cmdCommon.YAML:
		if printer != nil {
			printer.Print(raw)
		}
		return nil
	default:
		return fmt.Errorf("tableview: unsupported output format %s", outType.String())
	}
}

type detailView struct {
	id              int
	table           *table.Model
	items           []detailItem
	label           string
	parent          any
	parentType      string
	title           string
	child           *childViewState
	decorator       detailTableDecorator
	rawContent      string
	contentViewport *viewport.Model
	highlight       bool
}

type childViewState struct {
	headers        []string
	rows           []table.Row
	detailRenderer DetailRenderer
	context        DetailContextProvider
	parentType     string
	title          string
	headerLookup   map[string]int
	mode           ChildViewMode
}

func newChildViewState(view ChildView) childViewState {
	rows := make([]table.Row, len(view.Rows))
	for i, row := range view.Rows {
		rows[i] = append(table.Row(nil), row...)
	}

	return childViewState{
		headers:        append([]string(nil), view.Headers...),
		rows:           rows,
		detailRenderer: view.DetailRenderer,
		context:        view.DetailContext,
		parentType:     strings.ToLower(strings.TrimSpace(view.ParentType)),
		title:          strings.TrimSpace(view.Title),
		headerLookup:   buildHeaderLookup(view.Headers),
		mode:           view.Mode,
	}
}

func (c *childViewState) valueForHeader(row table.Row, header string) string {
	if c == nil || len(row) == 0 {
		return ""
	}

	key := normalizeHeaderKey(header)
	idx, ok := c.headerLookup[key]
	if !ok {
		return ""
	}
	if idx < 0 || idx >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[idx])
}

func (c *childViewState) preferredColumnValue(row table.Row) string {
	if c == nil {
		return ""
	}

	priorities := [][]string{
		{"name"},
		{"title"},
		{"display name"},
		{"slug"},
		{"version"},
	}

	for _, keys := range priorities {
		for _, key := range keys {
			if val := c.valueForHeader(row, key); val != "" {
				return val
			}
		}
	}

	for key, colIdx := range c.headerLookup {
		if strings.Contains(key, "name") {
			if colIdx < len(row) {
				if val := strings.TrimSpace(row[colIdx]); val != "" {
					return val
				}
			}
		}
	}

	idCandidates := []string{
		"id",
		"uuid",
		"uid",
		"identifier",
	}

	for _, key := range idCandidates {
		if val := c.valueForHeader(row, key); val != "" {
			return abbreviateValue(val, 12)
		}
	}

	return ""
}

func (c *childViewState) labelForIndex(index int) string {
	if c == nil {
		return ""
	}
	if !isValidIndex(index, len(c.rows)) {
		return fmt.Sprintf("Item %d", index+1)
	}

	row := c.rows[index]
	if len(row) == 0 {
		return fmt.Sprintf("Item %d", index+1)
	}

	if val := c.preferredColumnValue(row); val != "" {
		return val
	}

	for _, cell := range row {
		if trimmed := strings.TrimSpace(cell); trimmed != "" {
			return trimmed
		}
	}

	return fmt.Sprintf("Item %d", index+1)
}

type bubbleModel struct {
	table           table.Model
	title           string
	footer          string
	detailFooter    string
	showHelp        bool
	toggleKey       string
	quitKeys        []string
	tableStyle      lipgloss.Style
	detailStyle     lipgloss.Style
	statusStyle     lipgloss.Style
	selectedStyle   lipgloss.Style
	profileName     string
	useAltScreen    bool
	hasDetail       bool
	detail          *viewport.Model
	detailRenderer  DetailRenderer
	previewRenderer PreviewRenderer
	palette         theme.Palette
	windowWidth     int
	windowHeight    int
	detailStack     []detailView
	rowCount        int
	headers         []string
	headerLookup    map[string]int
	breadcrumbs     []string
	parentType      string
	detailContext   DetailContextProvider
	helper          cmdpkg.Helper
	statusMessage   string
	rowLoader       RowLoader
	searchActive    bool
	searchBuffer    []rune
	searchPrompt    string
	searchDeadline  time.Time
	spinner         spinner.Model
	pendingRequest  pendingRequest
	nextRequestID   int
	nextDetailID    int
	initCmd         tea.Cmd
	availableThemes []string
	themeIndex      int
}

func newBubbleModel(
	tbl table.Model,
	cfg config,
	tableStyle,
	detailStyle,
	statusStyle,
	selectedStyle lipgloss.Style,
	previewRenderer PreviewRenderer,
	palette theme.Palette,
	initialWidth,
	initialHeight,
	rowCount int,
	headers []string,
) *bubbleModel {
	rootLabel := strings.TrimSpace(cfg.rootLabel)
	rootLabel = formatRootLabel(rootLabel)
	if rootLabel == "" {
		rootLabel = formatRootLabel(cfg.title)
	}
	if rootLabel == "" {
		rootLabel = defaultRootLabel(headers)
	}

	parentType := strings.ToLower(strings.TrimSpace(cfg.childParentType))

	m := &bubbleModel{
		table:           tbl,
		title:           cfg.title,
		footer:          cfg.footer,
		detailFooter:    "Press enter to select or copy · esc/backspace to go back · arrows/j/k navigate",
		toggleKey:       cfg.toggleHelpKey,
		quitKeys:        cfg.quitKeys,
		tableStyle:      tableStyle,
		detailStyle:     detailStyle,
		statusStyle:     statusStyle,
		selectedStyle:   selectedStyle,
		profileName:     strings.TrimSpace(cfg.profileName),
		useAltScreen:    true,
		hasDetail:       cfg.hasDetail,
		detailRenderer:  cfg.detailRenderer,
		previewRenderer: previewRenderer,
		palette:         palette,
		windowWidth:     initialWidth,
		windowHeight:    initialHeight,
		rowCount:        rowCount,
		headers:         append([]string(nil), headers...),
		headerLookup:    buildHeaderLookup(headers),
		breadcrumbs:     []string{rootLabel},
		parentType:      parentType,
		detailContext:   cfg.detailContext,
		helper:          cfg.childHelper,
		rowLoader:       cfg.rowLoader,
		spinner:         newSpinnerModel(palette),
		nextRequestID:   1,
		nextDetailID:    1,
		availableThemes: theme.Available(),
		themeIndex:      themeIndexOf(theme.Available(), palette.Name),
	}

	if cfg.hasDetail && cfg.detailViewport != nil {
		detailCopy := *cfg.detailViewport
		m.detail = &detailCopy
	}

	if cfg.initialRow >= 0 && cfg.initialRow < rowCount {
		m.table.SetCursor(cfg.initialRow)
	}

	if cfg.openInitial && m.rowLoader != nil && cfg.initialRow >= 0 && cfg.initialRow < rowCount {
		if started, cmd := m.openRowChild(); started {
			if cmd != nil {
				if m.initCmd != nil {
					m.initCmd = tea.Batch(m.initCmd, cmd)
				} else {
					m.initCmd = cmd
				}
			}
		}
	}

	return m
}

func themeIndexOf(names []string, name string) int {
	for i, n := range names {
		if n == name {
			return i
		}
	}
	return 0
}

// applyPalette updates all palette-sensitive styles and redraws the table and
// any open detail views with the new theme.
func (m *bubbleModel) applyPalette(p theme.Palette) {
	m.palette = p
	m.tableStyle = newTableBoxStyle(p)
	m.detailStyle = newDetailBoxStyle(p)
	m.statusStyle = newStatusBoxStyle(p)
	m.spinner = newSpinnerModel(p)

	styles := table.DefaultStyles()
	styles.Header = styles.Header.
		Foreground(p.Adaptive(theme.ColorTextPrimary)).
		Background(p.Adaptive(theme.ColorSurface))
	styles.Cell = styles.Cell.
		Foreground(p.Adaptive(theme.ColorTextPrimary))
	styles.Selected = styles.Selected.
		Foreground(p.Adaptive(theme.ColorAccentText)).
		Background(p.Adaptive(theme.ColorAccent))
	m.selectedStyle = styles.Selected
	m.table.SetStyles(styles)

	for i := range m.detailStack {
		dv := &m.detailStack[i]
		if dv.table != nil && dv.child != nil {
			newTbl := buildChildTable(dv.child, m.detailViewportWidth(), m.detailViewportHeight(), p)
			dv.table = &newTbl
		} else if dv.items != nil {
			newTbl, dec := buildDetailTable(dv.items, m.detailViewportWidth(), m.detailViewportHeight(), p, dv.highlight)
			dv.table = &newTbl
			dv.decorator = dec
		}
	}
}

func (m *bubbleModel) Init() tea.Cmd {
	if m.initCmd != nil {
		return m.initCmd
	}
	return func() tea.Msg { return nil }
}

func (m *bubbleModel) nextDetailIdentifier() int {
	id := m.nextDetailID
	m.nextDetailID++
	return id
}

func (m *bubbleModel) pushDetailView(view detailView) string {
	if view.id == 0 {
		view.id = m.nextDetailIdentifier()
	} else if view.id >= m.nextDetailID {
		m.nextDetailID = view.id + 1
	}
	if strings.TrimSpace(view.title) == "" {
		view.title = view.label
	}
	if strings.TrimSpace(view.label) == "" {
		view.label = view.title
	}
	trimmedLabel := strings.TrimSpace(view.label)
	m.detailStack = append(m.detailStack, view)
	if trimmedLabel != "" {
		m.breadcrumbs = append(m.breadcrumbs, trimmedLabel)
	}
	m.clearStatus()
	return trimmedLabel
}

func (m *bubbleModel) beginRequest(
	kind requestKind,
	label string,
	rowIndex int,
	detailID int,
	itemIndex int,
) (string, tea.Cmd) {
	if m.pendingRequest.active {
		return "", nil
	}
	id := fmt.Sprintf("req-%d", m.nextRequestID)
	m.nextRequestID++
	m.pendingRequest = pendingRequest{
		id:        id,
		started:   time.Now(),
		label:     label,
		kind:      kind,
		rowIndex:  rowIndex,
		detailID:  detailID,
		itemIndex: itemIndex,
		active:    true,
	}
	m.spinner = newSpinnerModel(m.palette)
	m.clearStatus()
	return id, m.spinner.Tick
}

func (m *bubbleModel) completeRequest(id string) (pendingRequest, time.Duration, bool) {
	if !m.pendingRequest.active || m.pendingRequest.id != id {
		return pendingRequest{}, 0, false
	}
	pr := m.pendingRequest
	duration := time.Since(pr.started)
	m.pendingRequest = pendingRequest{}
	m.spinner = newSpinnerModel(m.palette)
	return pr, duration, true
}

func (m *bubbleModel) renderPendingRequestStatus() string {
	if !m.pendingRequest.active {
		return ""
	}
	elapsed := time.Since(m.pendingRequest.started)
	segments := []string{m.spinner.View(), m.pendingRequest.inflightMessage(), formatElapsed(elapsed)}
	return strings.Join(segments, " ")
}

func (m *bubbleModel) findDetailByID(id int) *detailView {
	for i := range m.detailStack {
		if m.detailStack[i].id == id {
			return &m.detailStack[i]
		}
	}
	return nil
}

func (m *bubbleModel) previewDetailContent(index int) string {
	if m.detailRenderer == nil || index < 0 || index >= m.rowCount {
		return ""
	}

	raw := m.detailRenderer(index)
	var parent any
	if m.detailContext != nil {
		parent = m.detailContext(index)
	}

	sanitized := sanitizePreviewDetailContent(raw, m.parentType, parent)
	return stylizeDetailContent(sanitized, m.palette)
}

func (m *bubbleModel) inDetailMode() bool {
	return len(m.detailStack) > 0
}

func (m *bubbleModel) presentRowChild(index int, childView ChildView) string {
	state := newChildViewState(childView)
	label := strings.TrimSpace(childView.Title)
	if label == "" {
		label = m.labelForIndex(index)
	}
	if label == "" {
		label = fmt.Sprintf("Item %d", index+1)
	}

	var parent any
	if state.context != nil {
		ctxIndex := index
		if state.mode == ChildViewModeDetail {
			ctxIndex = 0
		}
		parent = state.context(ctxIndex)
	}

	if state.mode == ChildViewModeDetail {
		if label == "" {
			label = fmt.Sprintf("Item %d", index+1)
		}
		m.pushChildDetailState(&state, label, parent, true)
		return label
	}

	childWidth := m.detailViewportWidth()
	childHeight := m.detailViewportHeight()
	childTable := buildChildTable(&state, childWidth, childHeight, m.palette)

	return m.pushDetailView(detailView{
		table:      &childTable,
		label:      label,
		parent:     parent,
		parentType: state.parentType,
		title:      childView.Title,
		child:      &state,
	})
}

func (m *bubbleModel) openRowChild() (bool, tea.Cmd) {
	if m.rowLoader == nil {
		return false, nil
	}
	if m.pendingRequest.active {
		return false, nil
	}

	index := clamp(m.table.Cursor(), 0, m.rowCount-1)
	if index < 0 {
		return false, nil
	}

	label := m.labelForIndex(index)
	requestID, tickCmd := m.beginRequest(requestKindRow, label, index, 0, 0)
	if requestID == "" {
		return false, nil
	}

	loader := m.rowLoader
	loadCmd := func() tea.Msg {
		childView, err := loader(index)
		return rowChildLoadedMsg{
			requestID: requestID,
			index:     index,
			child:     childView,
			err:       err,
		}
	}

	if tickCmd != nil {
		return true, tea.Batch(tickCmd, loadCmd)
	}
	return true, loadCmd
}

func (m *bubbleModel) openDetailView() bool {
	if m.detailRenderer == nil {
		return false
	}

	index := clamp(m.table.Cursor(), 0, m.rowCount-1)
	if index < 0 {
		return false
	}

	rawContent := m.detailRenderer(index)
	items := parseDetailContent(rawContent)
	width := m.detailViewportWidth()
	height := m.detailViewportHeight()
	label := m.labelForIndex(index)
	parentType := m.parentType

	var parent any
	if m.detailContext != nil {
		parent = m.detailContext(index)
	}

	items = enrichDetailItems(items, parentType, parent)
	items = reorderDetailItems(items)

	tableModel, decorator := buildDetailTable(items, width, height, m.palette, true)
	m.pushDetailView(detailView{
		table:      &tableModel,
		items:      items,
		label:      label,
		parent:     parent,
		parentType: parentType,
		title:      label,
		decorator:  decorator,
		highlight:  true,
	})
	return true
}

func (m *bubbleModel) popDetailView() {
	if len(m.detailStack) == 0 {
		return
	}
	m.detailStack = m.detailStack[:len(m.detailStack)-1]
	if len(m.breadcrumbs) > 1 {
		m.breadcrumbs = m.breadcrumbs[:len(m.breadcrumbs)-1]
	}
	m.clearStatus()
}

func (m *bubbleModel) resizeDetailViews() {
	if len(m.detailStack) == 0 {
		m.clearStatus()
		return
	}

	width := m.detailViewportWidth()
	height := m.detailViewportHeight()
	for i := range m.detailStack {
		detail := &m.detailStack[i]
		if detail.contentViewport != nil {
			width := m.detailViewportWidth()
			maxHeight := m.detailViewportHeight()
			offset := detail.contentViewport.YOffset
			rendered := renderMarkdownContent(detail.rawContent, width)
			targetHeight := maxHeight
			contentHeight := lipgloss.Height(rendered)
			if contentHeight > 0 && (targetHeight <= 0 || contentHeight < targetHeight) {
				targetHeight = contentHeight
			}
			if targetHeight < 1 {
				targetHeight = 1
			}
			detail.contentViewport.Width = width
			detail.contentViewport.Height = targetHeight
			detail.contentViewport.SetContent(rendered)
			detail.contentViewport.SetYOffset(offset)
			continue
		}
		cursor := 0
		if detail.table != nil {
			cursor = detail.table.Cursor()
		}
		if detail.child != nil {
			newTable := buildChildTable(detail.child, width, height, m.palette)
			if cursor > 0 && cursor < len(detail.child.rows) {
				newTable.SetCursor(cursor)
			}
			detail.table = &newTable
			detail.decorator = detailTableDecorator{}
			continue
		}
		newTable, decorator := buildDetailTable(detail.items, width, height, m.palette, detail.highlight)
		if cursor > 0 && cursor < len(detail.items) {
			newTable.SetCursor(cursor)
		}
		detail.table = &newTable
		detail.decorator = decorator
	}
}

func detailItemsContain(items []detailItem, label string) bool {
	normalized := normalizeHeaderKey(label)
	for _, item := range items {
		if normalizeHeaderKey(item.Label) == normalized {
			return true
		}
	}
	return false
}

func reorderDetailItems(items []detailItem) []detailItem {
	if len(items) == 0 {
		return items
	}

	var ids, names, others []detailItem
	for _, item := range items {
		switch normalizeHeaderKey(item.Label) {
		case "id":
			ids = append(ids, item)
		case "name":
			names = append(names, item)
		default:
			others = append(others, item)
		}
	}

	sort.SliceStable(others, func(i, j int) bool {
		keyI := normalizeHeaderKey(others[i].Label)
		keyJ := normalizeHeaderKey(others[j].Label)
		if keyI == keyJ {
			return strings.ToLower(
				strings.TrimSpace(others[i].Label),
			) < strings.ToLower(
				strings.TrimSpace(others[j].Label),
			)
		}
		return keyI < keyJ
	})

	result := make([]detailItem, 0, len(items))
	result = append(result, ids...)
	result = append(result, names...)
	result = append(result, others...)
	return result
}

func (m *bubbleModel) activateDetailSelection() (bool, tea.Cmd) {
	if len(m.detailStack) == 0 {
		return false, nil
	}
	detail := &m.detailStack[len(m.detailStack)-1]
	if detail.child != nil {
		return m.openChildRowDetail(detail), nil
	}
	if detail.table == nil || len(detail.items) == 0 {
		return false, nil
	}
	row := detail.table.Cursor()
	if row < 0 || row >= len(detail.items) {
		return false, nil
	}
	item := detail.items[row]
	if item.Loader != nil {
		return m.openChildCollection(detail, row)
	}
	m.copyDetailItemValue(item)
	return false, nil
}

func (m *bubbleModel) copyDetailItemValue(item detailItem) {
	value := strings.TrimSpace(item.Value)
	if value == "" {
		m.setStatus("No value to copy.")
		return
	}
	if err := writeClipboardText(value); err != nil {
		m.setStatus(fmt.Sprintf("Copy failed: %v", err))
		return
	}
	m.setStatus(fmt.Sprintf("Copied '%s' to buffer...", formatStatusValue(value)))
}

func (m *bubbleModel) openChildCollection(detail *detailView, row int) (bool, tea.Cmd) {
	if detail == nil || detail.table == nil || len(detail.items) == 0 {
		return false, nil
	}
	if row < 0 || row >= len(detail.items) {
		return false, nil
	}
	if detail.items[row].Loader == nil {
		return false, nil
	}
	if m.helper == nil {
		detail.items[row].Value = "loader unavailable"
		m.rebuildDetailItemsTable(detail, row)
		return false, nil
	}
	if m.pendingRequest.active {
		return false, nil
	}

	label := strings.TrimSpace(detail.items[row].Label)
	ctx := m.helper.GetContext()
	helper := m.helper
	parent := detail.parent
	loader := detail.items[row].Loader
	requestID, tickCmd := m.beginRequest(requestKindDetail, label, -1, detail.id, row)
	if requestID == "" {
		return false, nil
	}

	loadCmd := func() tea.Msg {
		childView, err := loader(ctx, helper, parent)
		return detailChildLoadedMsg{
			requestID: requestID,
			detailID:  detail.id,
			itemIndex: row,
			child:     childView,
			err:       err,
			label:     label,
		}
	}

	if tickCmd != nil {
		return true, tea.Batch(tickCmd, loadCmd)
	}
	return true, loadCmd
}

func (m *bubbleModel) presentDetailChild(detail *detailView, row int, childView ChildView, labelHint string) string {
	if detail == nil {
		return ""
	}

	state := newChildViewState(childView)

	label := strings.TrimSpace(labelHint)
	if label == "" && row >= 0 && row < len(detail.items) {
		label = strings.TrimSpace(detail.items[row].Label)
	}
	if label == "" {
		label = state.title
	}
	if label == "" {
		label = fmt.Sprintf("Item %d", row+1)
	}

	var parent any
	if state.context != nil {
		ctxIndex := row
		if state.mode == ChildViewModeDetail {
			ctxIndex = 0
		}
		parent = state.context(ctxIndex)
	}
	if parent == nil {
		parent = detail.parent
	}

	if state.mode == ChildViewModeDetail {
		m.pushChildDetailState(&state, label, parent, true)
		return label
	}

	if len(state.rows) == 0 && childView.DetailRenderer != nil {
		raw := strings.TrimSpace(childView.DetailRenderer(0))
		if raw == "" {
			raw = "(content is empty)"
		}

		width := m.detailViewportWidth()
		maxHeight := m.detailViewportHeight()
		rendered := renderMarkdownContent(raw, width)
		targetHeight := maxHeight
		contentHeight := lipgloss.Height(rendered)
		if contentHeight > 0 && (targetHeight <= 0 || contentHeight < targetHeight) {
			targetHeight = contentHeight
		}
		if targetHeight < 1 {
			targetHeight = 1
		}
		vp := viewport.New(width, targetHeight)
		vp.Width = width
		vp.Height = targetHeight
		vp.SetContent(rendered)

		return m.pushDetailView(detailView{
			label:           label,
			parent:          parent,
			parentType:      state.parentType,
			title:           state.title,
			rawContent:      raw,
			contentViewport: &vp,
		})
	}

	childTable := buildChildTable(&state, m.detailViewportWidth(), m.detailViewportHeight(), m.palette)

	return m.pushDetailView(detailView{
		table:      &childTable,
		label:      label,
		parent:     parent,
		parentType: state.parentType,
		title:      state.title,
		child:      &state,
	})
}

func (m *bubbleModel) pushChildDetailState(state *childViewState, label string, parent any, highlight bool) {
	if state == nil {
		return
	}

	raw := ""
	if state.detailRenderer != nil {
		raw = state.detailRenderer(0)
	}

	items := parseDetailContent(raw)
	items = enrichDetailItems(items, state.parentType, parent)
	items = reorderDetailItems(items)

	tableModel, decorator := buildDetailTable(
		items,
		m.detailViewportWidth(),
		m.detailViewportHeight(),
		m.palette,
		highlight,
	)
	m.pushDetailView(detailView{
		table:      &tableModel,
		items:      items,
		label:      label,
		parent:     parent,
		parentType: state.parentType,
		title:      state.title,
		decorator:  decorator,
		highlight:  highlight,
	})
}

func (m *bubbleModel) rebuildDetailItemsTable(detail *detailView, cursor int) {
	if detail == nil {
		return
	}
	detail.items = reorderDetailItems(detail.items)
	detailWidth := m.detailViewportWidth()
	detailHeight := m.detailViewportHeight()
	newTable, decorator := buildDetailTable(detail.items, detailWidth, detailHeight, m.palette, detail.highlight)
	if cursor >= 0 && cursor < len(detail.items) {
		newTable.SetCursor(cursor)
	}
	detail.table = &newTable
	detail.decorator = decorator
}

func (m *bubbleModel) openChildRowDetail(detail *detailView) bool {
	if detail == nil || detail.child == nil || detail.table == nil {
		return false
	}

	row := detail.table.Cursor()
	if row < 0 || row >= len(detail.child.rows) {
		return false
	}

	if detail.child.detailRenderer == nil {
		return false
	}

	raw := detail.child.detailRenderer(row)
	items := parseDetailContent(raw)
	parentType := detail.child.parentType
	label := strings.TrimSpace(detail.child.labelForIndex(row))
	if label == "" {
		label = fmt.Sprintf("Item %d", row+1)
	}

	var parent any
	if detail.child.context != nil {
		parent = detail.child.context(row)
	}

	items = enrichDetailItems(items, parentType, parent)
	items = reorderDetailItems(items)

	tableModel, decorator := buildDetailTable(items, m.detailViewportWidth(), m.detailViewportHeight(), m.palette, true)
	m.pushDetailView(detailView{
		table:      &tableModel,
		items:      items,
		label:      label,
		parent:     parent,
		parentType: parentType,
		title:      label,
		decorator:  decorator,
		highlight:  true,
	})
	return true
}

func (m *bubbleModel) detailViewportWidth() int {
	width := m.windowWidth
	if width <= 0 {
		width = 120
	}
	return max(10, width-4)
}

func (m *bubbleModel) detailViewportHeight() int {
	height := m.windowHeight
	if height <= 0 {
		height = 24
	}
	return max(5, height-6)
}

func (m *bubbleModel) labelForIndex(index int) string {
	rows := m.table.Rows()
	if !isValidIndex(index, len(rows)) {
		return fmt.Sprintf("Item %d", index+1)
	}
	return m.friendlyLabel(rows[index], index)
}

func (m *bubbleModel) friendlyLabel(row table.Row, index int) string {
	if len(row) == 0 {
		return fmt.Sprintf("Item %d", index+1)
	}

	value := m.preferredColumnValue(row, index)
	if value != "" {
		return value
	}

	for _, cell := range row {
		if trimmed := strings.TrimSpace(cell); trimmed != "" {
			return trimmed
		}
	}

	return fmt.Sprintf("Item %d", index+1)
}

func (m *bubbleModel) preferredColumnValue(row table.Row, _ int) string {
	priorities := [][]string{
		{"name"},
		{"title"},
		{"display name"},
		{"slug"},
	}

	for _, keys := range priorities {
		for _, key := range keys {
			if val := m.valueForHeader(row, key); val != "" {
				return val
			}
		}
	}

	// Any column containing "name"
	for key, colIdx := range m.headerLookup {
		if strings.Contains(key, "name") {
			if colIdx < len(row) {
				if val := strings.TrimSpace(row[colIdx]); val != "" {
					return val
				}
			}
		}
	}

	idCandidates := []string{
		"id",
		"uuid",
		"uid",
		"identifier",
	}

	for _, key := range idCandidates {
		if val := m.valueForHeader(row, key); val != "" {
			return abbreviateValue(val, 12)
		}
	}

	return ""
}

func (m *bubbleModel) valueForHeader(row table.Row, header string) string {
	if len(row) == 0 {
		return ""
	}

	key := normalizeHeaderKey(header)
	idx, ok := m.headerLookup[key]
	if !ok {
		return ""
	}
	if idx < 0 || idx >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[idx])
}

func (m *bubbleModel) renderBreadcrumb() string {
	if len(m.breadcrumbs) == 0 {
		return ""
	}

	homeStyle := m.palette.ForegroundStyle(theme.ColorPrimary)
	baseStyle := m.palette.ForegroundStyle(theme.ColorTextSecondary)
	activeStyle := m.palette.ForegroundStyle(theme.ColorInfo)

	var builder strings.Builder
	home := strings.TrimSpace(m.breadcrumbs[0])
	if home == "" {
		home = "Konnect"
	}
	builder.WriteString(homeStyle.Render(home))

	for i := 1; i < len(m.breadcrumbs); i++ {
		segment := strings.TrimSpace(m.breadcrumbs[i])
		if segment == "" {
			continue
		}

		builder.WriteString(baseStyle.Render(" > "))
		style := baseStyle
		if i == len(m.breadcrumbs)-1 {
			style = activeStyle
		}
		builder.WriteString(style.Render(quoteBreadcrumbSegment(segment)))
	}

	return builder.String()
}

func (m *bubbleModel) renderStatusArea(widthHint int) string {
	width := widthHint
	if width <= 0 {
		width = m.windowWidth
	}
	if m.windowWidth > 0 && width > m.windowWidth {
		width = m.windowWidth
	}
	if width <= 0 {
		width = 80
	}
	if width < 1 {
		width = 1
	}

	frameWidth, _ := m.statusStyle.GetFrameSize()
	innerWidth := width - frameWidth
	if innerWidth < 1 {
		innerWidth = 1
	}

	var content string
	if m.showHelp {
		content = m.renderHelpContent(innerWidth)
	} else {
		rows := m.buildStatusRows(innerWidth)
		if len(rows) == 0 {
			rows = append(rows, strings.Repeat(" ", innerWidth))
		}
		content = strings.Join(rows, "\n")
	}

	return m.statusStyle.Render(content)
}

func (m *bubbleModel) buildStatusRows(innerWidth int) []string {
	var rows []string

	left := strings.TrimSpace(m.renderBreadcrumb())
	right := m.renderStatusHint()
	rows = append(rows, renderStatusRow(left, right, innerWidth))

	profile := m.renderProfileLabel()

	if m.pendingRequest.active {
		pending := strings.TrimSpace(m.renderPendingRequestStatus())
		if pending != "" {
			appendStatusRow(&rows, pending, &profile, innerWidth)
		}
	}

	if m.searchActive {
		prompt := strings.TrimSpace(m.searchPrompt)
		if prompt != "" {
			promptStyle := m.palette.ForegroundStyle(theme.ColorAccent)
			rows = append(rows, renderStatusRow(promptStyle.Render(prompt), profile, innerWidth))
			return rows
		}
	}

	if msg := strings.TrimSpace(m.statusMessage); msg != "" {
		statusStyle := lipgloss.NewStyle().Faint(true)
		appendStatusRow(&rows, statusStyle.Render(msg), &profile, innerWidth)
		return rows
	}

	if footer := strings.TrimSpace(m.footer); footer != "" {
		statusStyle := lipgloss.NewStyle().Faint(true)
		appendStatusRow(&rows, statusStyle.Render(footer), &profile, innerWidth)
		return rows
	}

	if profile != "" {
		rows = append(rows, renderStatusRow("", profile, innerWidth))
	}

	return rows
}

func appendStatusRow(rows *[]string, left string, profile *string, width int) {
	*rows = append(*rows, renderStatusRow(left, *profile, width))
	*profile = ""
}

func (m *bubbleModel) renderStatusHint() string {
	if m.showHelp {
		return ""
	}
	return lipgloss.NewStyle().Faint(true).Render("Press ? for help")
}

func (m *bubbleModel) renderProfileLabel() string {
	profile := strings.TrimSpace(m.profileName)
	if profile == "" {
		return ""
	}

	label := fmt.Sprintf("Profile: %s", profile)
	style := m.palette.ForegroundStyle(theme.ColorTextSecondary)
	return style.Render(label)
}

func (m *bubbleModel) renderHelpContent(innerWidth int) string {
	helpLines := []string{
		"Up/Down j/k     : navigate lists",
		"Enter           : open item or copy value",
		"/<text>         : jump to matching text",
		"Backspace / Esc : go to parent",
		"Ctrl+W          : toggle full screen",
		"t               : cycle color theme",
		"?               : toggle this help",
		"q               : quit",
	}
	helpStyle := lipgloss.NewStyle().Faint(true)
	rendered := make([]string, len(helpLines))
	for i, line := range helpLines {
		rendered[i] = padStatusLine(helpStyle.Render(line), innerWidth)
	}
	return strings.Join(rendered, "\n")
}

func renderStatusRow(left, right string, width int) string {
	leftWidth := lipgloss.Width(left)
	rightWidth := lipgloss.Width(right)

	if width < 1 {
		width = leftWidth + rightWidth
		if width < 1 {
			width = 1
		}
	}

	switch {
	case rightWidth == 0 && leftWidth == 0:
		return strings.Repeat(" ", width)
	case rightWidth == 0:
		if leftWidth >= width {
			return left
		}
		return left + strings.Repeat(" ", width-leftWidth)
	case leftWidth == 0:
		if rightWidth >= width {
			return right
		}
		return strings.Repeat(" ", width-rightWidth) + right
	default:
		gap := width - leftWidth - rightWidth
		if gap < 1 {
			gap = 1
		}
		return left + strings.Repeat(" ", gap) + right
	}
}

func padStatusLine(value string, width int) string {
	if width < 1 {
		return value
	}
	lineWidth := lipgloss.Width(value)
	if lineWidth >= width {
		return value
	}
	return value + strings.Repeat(" ", width-lineWidth)
}

func matchPanelHeights(
	leftContent, rightContent string,
	leftBox, rightBox string,
	leftStyle, rightStyle lipgloss.Style,
	selected lipgloss.Style,
) (string, string) {
	leftHeight := lipgloss.Height(leftBox)
	rightHeight := lipgloss.Height(rightBox)
	if leftHeight == rightHeight {
		return leftBox, rightBox
	}

	_, leftFrameHeight := leftStyle.GetFrameSize()
	_, rightFrameHeight := rightStyle.GetFrameSize()

	leftInner := leftHeight - leftFrameHeight
	if leftInner < 0 {
		leftInner = 0
	}
	rightInner := rightHeight - rightFrameHeight
	if rightInner < 0 {
		rightInner = 0
	}

	targetInner := leftInner
	if rightInner > targetInner {
		targetInner = rightInner
	}

	if leftInner < targetInner {
		leftBox = borderedTableView(leftStyle.Height(targetInner), leftContent, selected)
	}
	if rightInner < targetInner {
		rightBox = borderedDetailView(rightStyle.Height(targetInner), rightContent)
	}

	return leftBox, rightBox
}

func (m *bubbleModel) setStatus(msg string) {
	trimmed := strings.TrimSpace(msg)
	if trimmed == "" {
		m.clearStatus()
		return
	}

	if m.searchActive {
		m.exitSearch(true)
	}

	m.statusMessage = trimmed
}

func (m *bubbleModel) clearStatus() {
	m.statusMessage = ""
}

func (m *bubbleModel) clearSearchStatus() {
	if strings.HasPrefix(m.statusMessage, "No match for '/") {
		m.clearStatus()
	}
}

func (m *bubbleModel) updateSearchPrompt() {
	if !m.searchActive {
		m.searchPrompt = ""
		return
	}

	if len(m.searchBuffer) == 0 {
		m.searchPrompt = "/"
		return
	}

	m.searchPrompt = "/" + string(m.searchBuffer)
}

func (m *bubbleModel) startSearch() tea.Cmd {
	m.clearStatus()
	m.searchActive = true
	m.searchBuffer = m.searchBuffer[:0]
	m.updateSearchPrompt()
	m.clearSearchStatus()
	return m.scheduleSearchTimeout()
}

func (m *bubbleModel) exitSearch(preserveStatus bool) {
	if !m.searchActive {
		return
	}

	m.searchActive = false
	m.searchBuffer = nil
	m.searchPrompt = ""
	m.searchDeadline = time.Time{}
	if !preserveStatus {
		m.clearSearchStatus()
	}
}

func (m *bubbleModel) scheduleSearchTimeout() tea.Cmd {
	if !m.searchActive {
		return nil
	}

	deadline := time.Now().Add(searchIdleTimeout)
	m.searchDeadline = deadline

	return tea.Tick(searchIdleTimeout, func(time.Time) tea.Msg {
		return searchTimeoutMsg{deadline: deadline}
	})
}

func (m *bubbleModel) handleSearchKey(key tea.KeyMsg) (handled bool, propagate bool, cmd tea.Cmd) {
	keyStr := key.String()

	if !m.searchActive {
		if keyStr == "/" && !key.Alt && len(key.Runes) == 1 && key.Runes[0] == '/' {
			if m.activeSearchTable() == nil {
				m.setStatus("Search is not available for this view.")
				return true, false, nil
			}
			return true, false, m.startSearch()
		}
		return false, false, nil
	}

	switch keyStr {
	case "esc":
		m.exitSearch(false)
		return true, false, nil
	case "enter":
		m.exitSearch(false)
		return true, true, nil
	case "backspace":
		if len(m.searchBuffer) > 0 {
			m.searchBuffer = m.searchBuffer[:len(m.searchBuffer)-1]
			m.updateSearchPrompt()
			m.applySearchQuery()
			return true, false, m.scheduleSearchTimeout()
		}
		m.exitSearch(false)
		return true, false, nil
	case "tab", "shift+tab", "up", "down", "left", "right", "pgup", "pgdown", "home", "end":
		m.exitSearch(false)
		return true, true, nil
	}

	if key.Type == tea.KeyRunes && len(key.Runes) > 0 && !key.Alt {
		appended := false
		for _, r := range key.Runes {
			if unicode.IsControl(r) {
				continue
			}
			m.searchBuffer = append(m.searchBuffer, r)
			appended = true
		}
		if appended {
			m.updateSearchPrompt()
			m.applySearchQuery()
			return true, false, m.scheduleSearchTimeout()
		}
		return true, false, nil
	}

	if key.Type != tea.KeyRunes {
		m.exitSearch(false)
		return false, true, nil
	}

	return true, false, nil
}

func (m *bubbleModel) applySearchQuery() {
	query := string(m.searchBuffer)
	if query == "" {
		m.clearSearchStatus()
		return
	}

	var matched bool
	if m.inDetailMode() {
		index := len(m.detailStack) - 1
		if index >= 0 {
			matched = m.applySearchToDetail(&m.detailStack[index], query)
		}
	} else {
		matched = m.applySearchToMain(query)
	}

	if matched {
		m.clearSearchStatus()
	} else {
		m.setStatus(fmt.Sprintf("No match for '/%s'", query))
	}
}

func (m *bubbleModel) applySearchToMain(query string) bool {
	rows := m.table.Rows()
	rowCount := len(rows)
	if rowCount == 0 {
		return false
	}

	cursor := m.table.Cursor()
	if cursor < 0 || cursor >= rowCount {
		cursor = 0
	}

	match, ok := findMatchIndex(query, cursor, rowCount, func(i int) string {
		return m.friendlyLabel(rows[i], i)
	})
	if !ok {
		return false
	}

	m.table.SetCursor(match)
	return true
}

func (m *bubbleModel) applySearchToDetail(detail *detailView, query string) bool {
	if detail == nil || detail.table == nil {
		return false
	}

	rows := detail.table.Rows()
	rowCount := len(rows)
	if rowCount == 0 {
		return false
	}

	cursor := detail.table.Cursor()
	if cursor < 0 || cursor >= rowCount {
		cursor = 0
	}

	match, ok := findMatchIndex(query, cursor, rowCount, func(i int) string {
		if detail.child != nil {
			return detail.child.labelForIndex(i)
		}
		if i >= 0 && i < len(detail.items) {
			return detail.items[i].Label
		}
		return fallbackRowLabel(rows, i)
	})
	if !ok {
		return false
	}

	detail.table.SetCursor(match)
	return true
}

func (m *bubbleModel) activeSearchTable() *table.Model {
	if m.inDetailMode() {
		index := len(m.detailStack) - 1
		if index >= 0 {
			return m.detailStack[index].table
		}
		return nil
	}
	return &m.table
}

func findMatchIndex(query string, cursor, total int, label func(int) string) (int, bool) {
	if total == 0 {
		return -1, false
	}

	if cursor < 0 || cursor >= total {
		cursor = 0
	}

	needle := strings.ToLower(strings.TrimSpace(query))
	if needle == "" {
		return cursor, true
	}

	bestIdx := -1
	bestScore := 0
	for offset := 0; offset < total; offset++ {
		idx := (cursor + offset) % total
		text := strings.ToLower(strings.TrimSpace(label(idx)))
		if text == "" {
			continue
		}

		score := matchScore(text, needle)
		if score == 0 {
			continue
		}

		if score == 3 {
			return idx, true
		}

		if score > bestScore {
			bestScore = score
			bestIdx = idx
		}
	}

	if bestScore > 0 && bestIdx >= 0 {
		return bestIdx, true
	}

	return -1, false
}

func matchScore(text, needle string) int {
	if strings.HasPrefix(text, needle) {
		return 3
	}
	if strings.Contains(text, needle) {
		return 2
	}
	if fuzzyContains(text, needle) {
		return 1
	}
	return 0
}

func fuzzyContains(text, needle string) bool {
	if len(needle) == 0 {
		return true
	}

	j := 0
	for i := 0; i < len(text) && j < len(needle); i++ {
		if text[i] == needle[j] {
			j++
		}
	}

	return j == len(needle)
}

func fallbackRowLabel(rows []table.Row, index int) string {
	if !isValidIndex(index, len(rows)) {
		return fmt.Sprintf("Item %d", index+1)
	}

	row := rows[index]
	for _, cell := range row {
		if trimmed := strings.TrimSpace(cell); trimmed != "" {
			return trimmed
		}
	}

	return fmt.Sprintf("Item %d", index+1)
}

func (m *bubbleModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) { //nolint:ireturn
	var cmds []tea.Cmd
	tableHandled := false
	skipKeyProcessing := false
	searchConsumed := false

	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		handled, propagate, searchCmd := m.handleSearchKey(keyMsg)
		if handled {
			if searchCmd != nil {
				cmds = append(cmds, searchCmd)
			}
			if !propagate {
				skipKeyProcessing = true
				searchConsumed = true
				tableHandled = true
			}
		}
	}

	switch key := msg.(type) {
	case rowChildLoadedMsg:
		if pr, duration, ok := m.completeRequest(key.requestID); ok {
			if key.err != nil {
				message := fmt.Sprintf("Unable to open: %v", key.err)
				if duration > 0 {
					message = fmt.Sprintf("%s (after %s)", message, formatElapsed(duration))
				}
				m.setStatus(message)
			} else {
				label := m.presentRowChild(key.index, key.child)
				if trimmed := strings.TrimSpace(label); trimmed != "" {
					label = trimmed
				} else if trimmed := strings.TrimSpace(pr.label); trimmed != "" {
					label = trimmed
				}
				m.setStatus(fmt.Sprintf("%s loaded in %s", formatRequestLabel(label), formatElapsed(duration)))
			}
		}
		return m, tea.Batch(cmds...)
	case detailChildLoadedMsg:
		if pr, duration, ok := m.completeRequest(key.requestID); ok {
			detail := m.findDetailByID(key.detailID)
			if key.err != nil {
				label := formatRequestLabel(pr.label)
				if detail != nil && key.itemIndex >= 0 && key.itemIndex < len(detail.items) {
					detail.items[key.itemIndex].Value = fmt.Sprintf("error: %v", key.err)
					m.rebuildDetailItemsTable(detail, key.itemIndex)
					label = formatRequestLabel(detail.items[key.itemIndex].Label)
				}
				if trimmed := strings.TrimSpace(key.label); trimmed != "" {
					label = formatRequestLabel(trimmed)
				}
				message := fmt.Sprintf("Unable to load %s: %v", label, key.err)
				if duration > 0 {
					message = fmt.Sprintf("%s (after %s)", message, formatElapsed(duration))
				}
				m.setStatus(message)
			} else {
				label := formatRequestLabel(pr.label)
				if detail != nil && key.itemIndex >= 0 && key.itemIndex < len(detail.items) {
					label = formatRequestLabel(m.presentDetailChild(detail, key.itemIndex, key.child, key.label))
				}
				m.setStatus(fmt.Sprintf("%s loaded in %s", label, formatElapsed(duration)))
			}
		}
		return m, tea.Batch(cmds...)
	case spinner.TickMsg:
		if m.pendingRequest.active {
			var tickCmd tea.Cmd
			m.spinner, tickCmd = m.spinner.Update(key)
			if tickCmd != nil {
				cmds = append(cmds, tickCmd)
			}
		}
		return m, tea.Batch(cmds...)
	case tea.KeyMsg:
		if skipKeyProcessing {
			break
		}
		if key.String() == "ctrl+w" {
			m.useAltScreen = !m.useAltScreen
			if m.useAltScreen {
				cmds = append(cmds, tea.EnterAltScreen)
			} else {
				cmds = append(cmds, tea.ExitAltScreen)
			}
			tableHandled = true
			break
		}
		for _, k := range m.quitKeys {
			if key.String() == k {
				if m.useAltScreen {
					cmds = append(cmds, tea.ExitAltScreen)
					m.useAltScreen = false
				}
				cmds = append(cmds, tea.Quit)
				return m, tea.Batch(cmds...)
			}
		}
		if key.String() == "esc" && !m.inDetailMode() {
			if m.useAltScreen {
				cmds = append(cmds, tea.ExitAltScreen)
				m.useAltScreen = false
			}
			cmds = append(cmds, tea.Quit)
			return m, tea.Batch(cmds...)
		}
		if key.String() == m.toggleKey {
			m.showHelp = !m.showHelp
			return m, nil
		}
		if key.String() == "t" && !m.searchActive && len(m.availableThemes) > 0 {
			m.themeIndex = (m.themeIndex + 1) % len(m.availableThemes)
			nextName := m.availableThemes[m.themeIndex]
			if p, ok := theme.Get(nextName); ok {
				m.applyPalette(p)
				m.setStatus(
					fmt.Sprintf("Theme: %s (set color-theme: %s in config to persist)", p.DisplayName, nextName),
				)
			}
			tableHandled = true
			break
		}

		if m.inDetailMode() {
			switch key.String() {
			case "esc", "backspace":
				m.popDetailView()
				return m, tea.Batch(cmds...)
			case "enter":
				handled, detailCmd := m.activateDetailSelection()
				if handled {
					if detailCmd != nil {
						cmds = append(cmds, detailCmd)
					}
					return m, tea.Batch(cmds...)
				}
			}
		} else {
			if key.String() == "enter" {
				if m.rowLoader != nil {
					started, loadCmd := m.openRowChild()
					if started {
						if loadCmd != nil {
							cmds = append(cmds, loadCmd)
						}
						return m, tea.Batch(cmds...)
					}
				}
				if m.openDetailView() {
					return m, tea.Batch(cmds...)
				}
			}
		}
	case tea.WindowSizeMsg:
		m.windowWidth = key.Width
		m.windowHeight = key.Height
		m.resizeDetailViews()

		newTable, cmd := m.table.Update(msg)
		m.table = newTable
		m.rowCount = len(m.table.Rows())
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		tableHandled = true
	case searchTimeoutMsg:
		if m.searchActive && key.deadline.Equal(m.searchDeadline) {
			m.exitSearch(false)
		}
	}

	if m.inDetailMode() {
		index := len(m.detailStack) - 1
		if index >= 0 {
			detail := &m.detailStack[index]
			if detail.contentViewport != nil {
				if !searchConsumed {
					vp, detailCmd := detail.contentViewport.Update(msg)
					detail.contentViewport = &vp
					if detailCmd != nil {
						cmds = append(cmds, detailCmd)
					}
				}
			} else if detail.table != nil {
				if !searchConsumed {
					newTable, detailCmd := detail.table.Update(msg)
					detail.table = &newTable
					if detailCmd != nil {
						cmds = append(cmds, detailCmd)
					}
				}
			}
		}
		return m, tea.Batch(cmds...)
	}

	if !tableHandled {
		newTable, cmd := m.table.Update(msg)
		m.table = newTable
		m.rowCount = len(m.table.Rows())
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	if m.hasDetail && m.detailRenderer != nil && m.detail != nil {
		index := clamp(m.table.Cursor(), 0, m.rowCount-1)
		content := m.previewDetailContent(index)
		m.detail.SetContent(content)
		contentHeight := lipgloss.Height(content)
		if contentHeight > 0 && (m.detail.Height <= 0 || contentHeight < m.detail.Height) {
			m.detail.Height = contentHeight
			if m.detail.Height < 1 {
				m.detail.Height = 1
			}
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *bubbleModel) View() string {
	var sections []string

	if m.previewRenderer != nil && !m.inDetailMode() {
		preview := m.previewRenderer(m.table.Cursor())
		if preview != "" {
			sections = append(sections, preview)
		}
	}

	if m.inDetailMode() {
		index := len(m.detailStack) - 1
		if index >= 0 {
			detail := m.detailStack[index]
			if detail.contentViewport != nil {
				content := detail.contentViewport.View()
				detailBox := borderedDetailView(m.detailStyle, content)
				sections = append(sections, detailBox)
			} else if detail.child != nil && detail.table != nil {
				tableView := detail.table.View()
				tableView = detail.decorator.stylize(tableView)
				tableBox := borderedTableView(m.tableStyle, tableView, m.selectedStyle)

				detailFrameWidth, _ := m.detailStyle.GetFrameSize()
				available := m.windowWidth - lipgloss.Width(tableBox) - detailFrameWidth
				if available < 10 {
					available = 10
				}
				var detailContent string
				if detail.child.detailRenderer != nil {
					row := detail.table.Cursor()
					if row < 0 || row >= len(detail.child.rows) {
						row = clamp(row, 0, len(detail.child.rows)-1)
					}
					if row >= 0 && row < len(detail.child.rows) {
						raw := detail.child.detailRenderer(row)
						if strings.TrimSpace(raw) != "" {
							items := parseDetailContent(raw)
							parentType := detail.child.parentType
							var parent any
							if detail.child.context != nil {
								parent = detail.child.context(row)
							} else {
								parent = detail.parent
							}
							items = enrichDetailItems(items, parentType, parent)
							items = filterPreviewDetailItems(items)
							items = reorderDetailItems(items)
							detailTable, decorator := buildDetailTable(items, available, m.detailViewportHeight(), m.palette, false)
							detailContent = decorator.stylize(detailTable.View())
						}
					}
				}

				view := tableBox
				if strings.TrimSpace(detailContent) != "" {
					detailBox := borderedDetailView(m.detailStyle, detailContent)
					tableBox, detailBox = matchPanelHeights(
						tableView,
						detailContent,
						tableBox,
						detailBox,
						m.tableStyle,
						m.detailStyle,
						m.selectedStyle,
					)
					view = lipgloss.JoinHorizontal(lipgloss.Top, tableBox, detailBox)
				}
				sections = append(sections, view)
			} else {
				var view string
				if detail.table != nil {
					view = detail.decorator.stylize(detail.table.View())
				}
				detailBox := borderedDetailView(m.detailStyle, view)
				sections = append(sections, detailBox)
			}
		}
	} else {
		tableContent := m.table.View()
		tableBox := borderedTableView(m.tableStyle, tableContent, m.selectedStyle)
		main := tableBox
		if m.hasDetail && m.detail != nil {
			detailContent := m.detail.View()
			detailBox := borderedDetailView(m.detailStyle, detailContent)
			tableBox, detailBox = matchPanelHeights(
				tableContent,
				detailContent,
				tableBox,
				detailBox,
				m.tableStyle,
				m.detailStyle,
				m.selectedStyle,
			)
			main = lipgloss.JoinHorizontal(lipgloss.Top, tableBox, detailBox)
		}
		sections = append(sections, main)
	}

	widthHint := 0
	for _, section := range sections {
		if w := lipgloss.Width(section); w > widthHint {
			widthHint = w
		}
	}

	sections = append(sections, m.renderStatusArea(widthHint))

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}
