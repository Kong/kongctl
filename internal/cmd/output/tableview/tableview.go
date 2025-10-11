package tableview

import (
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
	"strings"
	"unicode"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
	"github.com/mattn/go-runewidth"
	"golang.org/x/term"

	cmdCommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/iostreams"
	"github.com/segmentio/cli"
)

type fdProvider interface {
	Fd() uintptr
}

type DetailRenderer func(index int) string

type config struct {
	title         string
	footer        string
	staticFooter  string
	quitKeys      []string
	toggleHelpKey string

	// optional behavior
	detailRenderer DetailRenderer
	hasDetail      bool
	detailViewport *viewport.Model

	customHeaders []string
	customRows    []table.Row
}

var (
	tableBoxStyle = lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(0, 1).
			MarginRight(1)

	detailBoxStyle = lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(0, 1).
			MarginLeft(0)
)

func borderedTableView(content string) string {
	return tableBoxStyle.Render(content)
}

func borderedDetailView(content string) string {
	return detailBoxStyle.Render(content)
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

// WithCustomTable allows overriding the automatically generated table columns/rows.
// The provided headers and rows are used only for the interactive table; other formats
// (text/json/yaml) continue to print the original display value.
func WithCustomTable(headers []string, rows []table.Row) Option {
	return func(cfg *config) {
		cfg.customHeaders = append([]string(nil), headers...)
		cfg.customRows = append([]table.Row(nil), rows...)
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
		footer:        "Press q to quit · arrows navigate",
		staticFooter:  "",
		quitKeys:      []string{"q", "Q", "esc", "ctrl+c"},
		toggleHelpKey: "?",
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	termWidth, termHeight, isTTY := resolveTerminal(streams.Out)

	var headers []string
	var matrix [][]string
	var tableRows []table.Row
	var err error

	if len(cfg.customRows) > 0 {
		headers = append([]string(nil), cfg.customHeaders...)
		matrix = rowsToMatrix(cfg.customRows)
		tableRows = cfg.customRows
	} else {
		headers, matrix, err = buildRows(data)
		if err != nil {
			return err
		}
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
	paddingWidth := max(
		lipgloss.Width(styles.Header.Render("")),
		lipgloss.Width(styles.Cell.Render("")),
	)

	tbl := table.New(
		table.WithColumns(columns),
		table.WithRows(tableRows),
		table.WithFocused(true),
		table.WithStyles(styles),
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

	setTableHeight(&tbl, len(tableRows), termHeight, isTTY)

	if cfg.detailRenderer != nil {
		cfg.hasDetail = true
		const borderAllowance = 4 // approximate border + padding width
		detailWidth := termWidth - tbl.Width() - borderAllowance
		if detailWidth < 20 {
			if termWidth > 0 {
				detailWidth = max(20, termWidth/2)
			} else {
				detailWidth = 40
			}
		}
		if detailWidth < 10 {
			detailWidth = 10
		}
		detailHeight := tbl.Height()
		const minDetailHeight = 10
		if detailHeight < minDetailHeight {
			detailHeight = minDetailHeight
		}
		dv := viewport.New(detailWidth, detailHeight)
		index := clamp(tbl.Cursor(), 0, len(tableRows)-1)
		if index >= 0 {
			dv.SetContent(cfg.detailRenderer(index))
		}
		cfg.detailViewport = &dv
	}

	if !isTTY {
		tableBox := borderedTableView(tbl.View())
		view := tableBox
		if cfg.detailRenderer != nil {
			index := clamp(tbl.Cursor(), 0, len(tableRows)-1)
			detail := cfg.detailRenderer(index)
			detailRendered := borderedDetailView(detail)
			view = lipgloss.JoinHorizontal(lipgloss.Top, tableBox, detailRendered)
		}
		if cfg.title != "" {
			view = lipgloss.JoinVertical(lipgloss.Left, cfg.title, view)
		}
		if cfg.staticFooter != "" {
			view = lipgloss.JoinVertical(lipgloss.Left, view, cfg.staticFooter)
		}
		_, err = fmt.Fprintln(streams.Out, view)
		return err
	}

	model := newBubbleModel(tbl, cfg)
	program := tea.NewProgram(model,
		tea.WithInput(streams.In),
		tea.WithOutput(streams.Out),
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

	switch value.Kind() { //nolint:exhaustive
	case reflect.Slice, reflect.Array:
		return rowsFromSlice(value)
	case reflect.Struct:
		slice := reflect.MakeSlice(reflect.SliceOf(value.Type()), 0, 1)
		slice = reflect.Append(slice, value)
		return rowsFromSlice(slice)
	default:
		return nil, nil, fmt.Errorf("tableview: unsupported data kind %s", value.Kind())
	}
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

func rowsToMatrix(rows []table.Row) [][]string {
	matrix := make([][]string, len(rows))
	for i, row := range rows {
		matrix[i] = append([]string(nil), row...)
	}
	return matrix
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

func setTableHeight(tbl *table.Model, rowCount, termHeight int, interactive bool) {
	if tbl == nil {
		return
	}

	if !interactive {
		tbl.SetHeight(rowCount + 1) // include header
		return
	}

	const minHeight = 5
	const margin = 4

	target := rowCount + 1
	if termHeight > 0 {
		target = clamp(target, minHeight, termHeight-margin)
	}
	if target < minHeight {
		target = minHeight
	}
	tbl.SetHeight(target)
}

// RenderForFormat renders structured data according to the requested output format.
// For interactive output it delegates to Render, otherwise it uses the provided printer.
func RenderForFormat(
	outType cmdCommon.OutputFormat,
	printer cli.PrintFlusher,
	streams *iostreams.IOStreams,
	display any,
	raw any,
	title string,
	extraOpts ...Option,
) error {
	switch outType {
	case cmdCommon.INTERACTIVE:
		var opts []Option
		if title != "" {
			opts = append(opts, WithTitle(title))
		}
		opts = append(opts, extraOpts...)
		return Render(streams, display, opts...)
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

type bubbleModel struct {
	table          table.Model
	title          string
	footer         string
	showHelp       bool
	toggleKey      string
	quitKeys       []string
	renderStyle    lipgloss.Style
	hasDetail      bool
	detail         *viewport.Model
	detailRenderer DetailRenderer
}

func newBubbleModel(tbl table.Model, cfg config) *bubbleModel {
	m := &bubbleModel{
		table:          tbl,
		title:          cfg.title,
		footer:         cfg.footer,
		toggleKey:      cfg.toggleHelpKey,
		quitKeys:       cfg.quitKeys,
		renderStyle:    lipgloss.NewStyle(),
		hasDetail:      cfg.hasDetail,
		detailRenderer: cfg.detailRenderer,
	}

	if cfg.hasDetail && cfg.detailViewport != nil {
		detailCopy := *cfg.detailViewport
		m.detail = &detailCopy
	}

	return m
}

func (m *bubbleModel) Init() tea.Cmd {
	return nil
}

func (m *bubbleModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) { //nolint:ireturn
	switch key := msg.(type) {
	case tea.KeyMsg:
		for _, k := range m.quitKeys {
			if key.String() == k {
				return m, tea.Quit
			}
		}
		if key.String() == m.toggleKey {
			m.showHelp = !m.showHelp
			return m, nil
		}
	}

	newTable, cmd := m.table.Update(msg)
	m.table = newTable

	if m.hasDetail && m.detailRenderer != nil && m.detail != nil {
		index := m.table.Cursor()
		m.detail.SetContent(m.detailRenderer(index))
	}

	return m, cmd
}

func (m *bubbleModel) View() string {
	var sections []string
	if m.title != "" {
		sections = append(sections, m.title)
	}

	tableBox := borderedTableView(m.table.View())
	main := tableBox
	if m.hasDetail && m.detail != nil {
		detailBox := borderedDetailView(m.detail.View())
		main = lipgloss.JoinHorizontal(lipgloss.Top, tableBox, detailBox)
	}
	sections = append(sections, main)

	if m.footer != "" {
		sections = append(sections, m.footer)
	}

	if m.showHelp {
		help := "Use arrows to navigate · q to quit · ? to hide this help"
		sections = append(sections, lipgloss.NewStyle().Faint(true).Render(help))
	} else {
		sections = append(sections, lipgloss.NewStyle().Faint(true).Render("Press ? for help"))
	}

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}
