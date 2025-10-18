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
	"sync"
	"unicode"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
	"github.com/mattn/go-runewidth"
	"golang.org/x/term"

	cmdpkg "github.com/kong/kongctl/internal/cmd"
	cmdCommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/iostreams"
	"github.com/kong/kongctl/internal/theme"
	"github.com/segmentio/cli"
	clipboard "golang.design/x/clipboard"
)

type fdProvider interface {
	Fd() uintptr
}

type DetailRenderer func(index int) string

type DetailContextProvider func(index int) any

type config struct {
	title         string
	footer        string
	staticFooter  string
	quitKeys      []string
	toggleHelpKey string
	rootLabel     string

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
		Padding(0, 1).
		MarginRight(1)
}

func newDetailBoxStyle(p theme.Palette) lipgloss.Style {
	return lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(p.Adaptive(theme.ColorBorder)).
		Padding(0, 1).
		MarginLeft(0)
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
	accentStyle := palette.ForegroundStyle(theme.ColorAccent)

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

// Render displays structured data using the Bubble Tea table component when
// the output stream is a TTY. For non-interactive streams it falls back to a
// static table rendering with truncated columns.
func Render(streams *iostreams.IOStreams, data any, opts ...Option) error {
	if streams == nil || streams.Out == nil {
		return errors.New("tableview: output stream is not available")
	}

	cfg := config{
		footer:        "Press q or esc to quit · arrows/j/k navigate",
		staticFooter:  "",
		quitKeys:      []string{"q", "Q", "ctrl+c"},
		toggleHelpKey: "?",
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	palette := theme.Current()
	tableStyle := newTableBoxStyle(palette)
	detailStyle := newDetailBoxStyle(palette)

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
			dv.SetContent(stylizeDetailContent(cfg.detailRenderer(index), palette))
		}
		cfg.detailViewport = &dv
	}

	if !isTTY {
		tableBox := borderedTableView(tableStyle, tbl.View(), styles.Selected)
		view := tableBox
		if cfg.detailRenderer != nil {
			index := clamp(tbl.Cursor(), 0, len(tableRows)-1)
			detail := stylizeDetailContent(cfg.detailRenderer(index), palette)
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

	model := newBubbleModel(tbl, cfg, tableStyle, detailStyle, styles.Selected, cfg.previewRenderer, palette, termWidth, termHeight, len(tableRows), headers)
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
	if limit <= 3 {
		return string(runes[:limit])
	}
	return string(runes[:limit-3]) + "..."
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
	return key
}

type detailItem struct {
	Label  string
	Value  string
	Loader ChildLoader
}

const (
	childFieldIndicator        = "{...}"
	complexNilIndicator        = "[nil]"
	complexEmptyIndicator      = "[]"
	complexExpandableIndicator = "[...]"
)

type mapEntry struct {
	Key     string
	Value   any
	Summary string
}

var (
	clipboardInitOnce sync.Once
	clipboardInitErr  error
)

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

	switch parent.Kind() {
	case reflect.Struct:
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
	case reflect.Map:
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
		switch value.Kind() {
		case reflect.Pointer, reflect.Interface:
			if value.IsNil() {
				return reflect.Value{}
			}
			value = value.Elem()
		default:
			return value
		}
	}
	return value
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

	switch value.Kind() {
	case reflect.Map:
		if value.IsNil() {
			return complexValueInfo{indicator: complexNilIndicator}, true
		}
		entries := mapEntriesFromValue(value)
		if len(entries) == 0 {
			return complexValueInfo{indicator: complexEmptyIndicator}, true
		}
		return complexValueInfo{
			indicator: complexExpandableIndicator,
			loader: func(_ context.Context, _ cmdpkg.Helper, _ any) (ChildView, error) {
				return buildChildViewForMap(label, entries), nil
			},
		}, true
	case reflect.Slice, reflect.Array:
		if value.Kind() == reflect.Slice && value.IsNil() {
			return complexValueInfo{indicator: complexNilIndicator}, true
		}
		length := value.Len()
		if length == 0 {
			return complexValueInfo{indicator: complexEmptyIndicator}, true
		}
		data := original.Interface()
		return complexValueInfo{
			indicator: complexExpandableIndicator,
			loader: func(_ context.Context, _ cmdpkg.Helper, _ any) (ChildView, error) {
				return buildChildViewFromSliceValue(label, data)
			},
		}, true
	case reflect.Struct:
		return complexValueInfo{indicator: childFieldIndicator}, true
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
	clipboardInitOnce.Do(func() {
		clipboardInitErr = clipboard.Init()
	})
	if clipboardInitErr != nil {
		return clipboardInitErr
	}
	done := clipboard.Write(clipboard.FmtText, []byte(value))
	if done == nil {
		return errors.New("clipboard write failed")
	}
	go func() {
		<-done
	}()
	return nil
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

func buildChildViewForMap(label string, entries []mapEntry) ChildView {
	rows := make([]table.Row, len(entries))
	for i, entry := range entries {
		rows[i] = table.Row{entry.Key, entry.Summary}
	}

	detail := func(index int) string {
		if index < 0 || index >= len(entries) {
			return ""
		}
		entry := entries[index]
		return fmt.Sprintf("key: %s\nvalue: %s", entry.Key, formatDetailValue(entry.Value))
	}

	contextProvider := func(index int) any {
		if index < 0 || index >= len(entries) {
			return nil
		}
		return entries[index].Value
	}

	return ChildView{
		Headers:        []string{"KEY", "VALUE"},
		Rows:           rows,
		DetailRenderer: detail,
		Title:          titleFromLabel(label),
		ParentType:     normalizeHeaderKey(label),
		DetailContext:  contextProvider,
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
			DetailRenderer: func(int) string { return "" },
			Title:          titleFromLabel(label),
			ParentType:     normalizeHeaderKey(label),
		}, nil
	}

	elemType := value.Type().Elem()
	elemType = derefType(elemType)

	switch elemType.Kind() {
	case reflect.Struct:
		headers, matrix, err := buildRows(data)
		if err != nil {
			return ChildView{}, err
		}
		rows := convertRows(matrix, len(headers))
		return ChildView{
			Headers: rowsHeaders(headers),
			Rows:    rows,
			DetailRenderer: func(index int) string {
				if index < 0 || index >= length {
					return ""
				}
				return renderStructDetail(value.Index(index).Interface())
			},
			Title:      titleFromLabel(label),
			ParentType: normalizeHeaderKey(label),
			DetailContext: func(index int) any {
				if index < 0 || index >= length {
					return nil
				}
				return value.Index(index).Interface()
			},
		}, nil
	default:
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
				if index < 0 || index >= len(values) {
					return ""
				}
				return fmt.Sprintf("value: %s", formatDetailValue(values[index]))
			},
			Title:      titleFromLabel(label),
			ParentType: normalizeHeaderKey(label),
			DetailContext: func(index int) any {
				if index < 0 || index >= len(values) {
					return nil
				}
				return values[index]
			},
		}, nil
	}
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

	switch rv.Kind() {
	case reflect.Slice, reflect.Array:
		if rv.Len() == 0 {
			return "[]"
		}
		var builder strings.Builder
		for i := 0; i < rv.Len(); i++ {
			fmt.Fprintf(&builder, "- %v\n", rv.Index(i).Interface())
		}
		return strings.TrimRight(builder.String(), "\n")
	case reflect.Map:
		if rv.Len() == 0 {
			return "{}"
		}
		keys := rv.MapKeys()
		sort.Slice(keys, func(i, j int) bool {
			return fmt.Sprint(keys[i].Interface()) < fmt.Sprint(keys[j].Interface())
		})
		var builder strings.Builder
		for _, key := range keys {
			fmt.Fprintf(&builder, "%v: %v\n", key.Interface(), rv.MapIndex(key).Interface())
		}
		return strings.TrimRight(builder.String(), "\n")
	case reflect.Struct:
		return renderStructDetail(rv.Interface())
	default:
		return fmt.Sprint(val)
	}
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

func buildDetailTable(items []detailItem, width, height int, palette theme.Palette) table.Model {
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
		Foreground(palette.Adaptive(theme.ColorTextSecondary)).
		Background(palette.Adaptive(theme.ColorSurface))
	styles.Cell = styles.Cell.
		PaddingLeft(0).
		PaddingRight(0)
	styles.Selected = lipgloss.NewStyle().
		Foreground(palette.Adaptive(theme.ColorAccentText)).
		Background(palette.Adaptive(theme.ColorAccent))

	tbl := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithStyles(styles),
	)

	if height > 0 {
		tbl.SetHeight(height)
	}
	if width > 0 {
		tbl.SetWidth(width)
	}
	return tbl
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

type detailView struct {
	table      *table.Model
	items      []detailItem
	label      string
	parent     any
	parentType string
	title      string
	child      *childViewState
}

type childViewState struct {
	headers        []string
	rows           []table.Row
	detailRenderer DetailRenderer
	context        DetailContextProvider
	parentType     string
	title          string
	headerLookup   map[string]int
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
	if index < 0 || index >= len(c.rows) {
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
	selectedStyle   lipgloss.Style
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
}

func newBubbleModel(
	tbl table.Model,
	cfg config,
	tableStyle,
	detailStyle,
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
		detailFooter:    "Press enter to select · esc/backspace to go back · arrows/j/k navigate",
		toggleKey:       cfg.toggleHelpKey,
		quitKeys:        cfg.quitKeys,
		tableStyle:      tableStyle,
		detailStyle:     detailStyle,
		selectedStyle:   selectedStyle,
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

func (m *bubbleModel) inDetailMode() bool {
	return len(m.detailStack) > 0
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

	tableModel := buildDetailTable(items, width, height, m.palette)
	m.clearStatus()
	m.detailStack = append(m.detailStack, detailView{
		table:      &tableModel,
		items:      items,
		label:      label,
		parent:     parent,
		parentType: parentType,
		title:      label,
	})
	if label != "" {
		m.breadcrumbs = append(m.breadcrumbs, label)
	}
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
			continue
		}
		newTable := buildDetailTable(detail.items, width, height, m.palette)
		if cursor > 0 && cursor < len(detail.items) {
			newTable.SetCursor(cursor)
		}
		detail.table = &newTable
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
			return strings.ToLower(strings.TrimSpace(others[i].Label)) < strings.ToLower(strings.TrimSpace(others[j].Label))
		}
		return keyI < keyJ
	})

	result := make([]detailItem, 0, len(items))
	result = append(result, ids...)
	result = append(result, names...)
	result = append(result, others...)
	return result
}

func (m *bubbleModel) activateDetailSelection() bool {
	if len(m.detailStack) == 0 {
		return false
	}
	detail := &m.detailStack[len(m.detailStack)-1]
	if detail.child != nil {
		return m.openChildRowDetail(detail)
	}
	if detail.table == nil || len(detail.items) == 0 {
		return false
	}
	row := detail.table.Cursor()
	if row < 0 || row >= len(detail.items) {
		return false
	}
	item := detail.items[row]
	if item.Loader != nil {
		return m.openChildCollection(detail)
	}
	m.copyDetailItemValue(item)
	return false
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

func (m *bubbleModel) openChildCollection(detail *detailView) bool {
	if detail == nil || detail.table == nil || len(detail.items) == 0 {
		return false
	}

	row := detail.table.Cursor()
	if row < 0 || row >= len(detail.items) {
		return false
	}

	if detail.items[row].Loader == nil {
		return false
	}

	if m.helper == nil {
		detail.items[row].Value = "loader unavailable"
		m.rebuildDetailItemsTable(detail, row)
		return false
	}

	childView, err := detail.items[row].Loader(m.helper.GetContext(), m.helper, detail.parent)
	if err != nil {
		detail.items[row].Value = fmt.Sprintf("error: %v", err)
		m.rebuildDetailItemsTable(detail, row)
		return false
	}

	state := newChildViewState(childView)
	childTable := buildChildTable(&state, m.detailViewportWidth(), m.detailViewportHeight(), m.palette)

	label := strings.TrimSpace(detail.items[row].Label)
	if label == "" {
		label = state.title
	}
	if label == "" {
		label = fmt.Sprintf("Item %d", row+1)
	}

	next := detailView{
		table:      &childTable,
		label:      label,
		parent:     detail.parent,
		parentType: state.parentType,
		title:      state.title,
		child:      &state,
	}

	if next.title == "" {
		next.title = label
	}
	if next.label == "" {
		next.label = next.title
	}

	m.detailStack = append(m.detailStack, next)
	if next.label != "" {
		m.breadcrumbs = append(m.breadcrumbs, next.label)
	}
	m.clearStatus()
	return true
}

func (m *bubbleModel) rebuildDetailItemsTable(detail *detailView, cursor int) {
	if detail == nil {
		return
	}
	detail.items = reorderDetailItems(detail.items)
	newTable := buildDetailTable(detail.items, m.detailViewportWidth(), m.detailViewportHeight(), m.palette)
	if cursor >= 0 && cursor < len(detail.items) {
		newTable.SetCursor(cursor)
	}
	detail.table = &newTable
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

	tableModel := buildDetailTable(items, m.detailViewportWidth(), m.detailViewportHeight(), m.palette)
	next := detailView{
		table:      &tableModel,
		items:      items,
		label:      label,
		parent:     parent,
		parentType: parentType,
		title:      label,
	}

	m.detailStack = append(m.detailStack, next)
	if label != "" {
		m.breadcrumbs = append(m.breadcrumbs, label)
	}
	m.clearStatus()
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
	if index < 0 || index >= len(rows) {
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

func (m *bubbleModel) preferredColumnValue(row table.Row, index int) string {
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

	baseStyle := m.palette.ForegroundStyle(theme.ColorTextSecondary)
	activeStyle := m.palette.ForegroundStyle(theme.ColorAccent)
	if len(m.breadcrumbs) == 1 {
		activeStyle = baseStyle
	}

	var builder strings.Builder
	const separator = " › "

	for i, segment := range m.breadcrumbs {
		if i > 0 {
			builder.WriteString(baseStyle.Render(separator))
		}
		style := baseStyle
		if i == len(m.breadcrumbs)-1 {
			style = activeStyle
		}
		builder.WriteString(style.Render(quoteBreadcrumbSegment(segment)))
	}
	return builder.String()
}

func (m *bubbleModel) renderBreadcrumbRow() string {
	breadcrumb := m.renderBreadcrumb()

	var hint string
	if !m.showHelp {
		hint = lipgloss.NewStyle().Faint(true).Render("Press ? for help")
	}

	if breadcrumb == "" && hint == "" {
		return ""
	}

	if m.windowWidth <= 0 {
		switch {
		case breadcrumb == "":
			return hint
		case hint == "":
			return breadcrumb
		default:
			return breadcrumb + "  " + hint
		}
	}

	if hint == "" {
		return breadcrumb
	}

	if breadcrumb == "" {
		spaces := m.windowWidth - lipgloss.Width(hint)
		if spaces < 0 {
			spaces = 0
		}
		return strings.Repeat(" ", spaces) + hint
	}

	spaces := m.windowWidth - lipgloss.Width(breadcrumb) - lipgloss.Width(hint)
	if spaces < 1 {
		spaces = 1
	}
	return breadcrumb + strings.Repeat(" ", spaces) + hint
}

func (m *bubbleModel) setStatus(msg string) {
	m.statusMessage = strings.TrimSpace(msg)
}

func (m *bubbleModel) clearStatus() {
	m.statusMessage = ""
}

func (m *bubbleModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) { //nolint:ireturn
	var cmds []tea.Cmd
	tableHandled := false

	switch key := msg.(type) {
	case tea.KeyMsg:
		for _, k := range m.quitKeys {
			if key.String() == k {
				return m, tea.Quit
			}
		}
		if key.String() == "esc" && !m.inDetailMode() {
			return m, tea.Quit
		}
		if key.String() == m.toggleKey {
			m.showHelp = !m.showHelp
			return m, nil
		}

		if m.inDetailMode() {
			switch key.String() {
			case "esc", "backspace":
				m.popDetailView()
				return m, tea.Batch(cmds...)
			case "enter":
				if m.activateDetailSelection() {
					return m, tea.Batch(cmds...)
				}
			}
		} else {
			if key.String() == "enter" {
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
	}

	if m.inDetailMode() {
		index := len(m.detailStack) - 1
		if index >= 0 {
			detail := &m.detailStack[index]
			if detail.table != nil {
				newTable, detailCmd := detail.table.Update(msg)
				detail.table = &newTable
				if detailCmd != nil {
					cmds = append(cmds, detailCmd)
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
		index := m.table.Cursor()
		m.detail.SetContent(stylizeDetailContent(m.detailRenderer(index), m.palette))
	}

	return m, tea.Batch(cmds...)
}

func (m *bubbleModel) View() string {
	var sections []string

	if row := m.renderBreadcrumbRow(); row != "" {
		sections = append(sections, row)
	}

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
			var view string
			if detail.table != nil {
				view = detail.table.View()
			}
			detailBox := borderedDetailView(m.detailStyle, view)
			sections = append(sections, detailBox)
		}
	} else {
		tableBox := borderedTableView(m.tableStyle, m.table.View(), m.selectedStyle)
		main := tableBox
		if m.hasDetail && m.detail != nil {
			detailBox := borderedDetailView(m.detailStyle, m.detail.View())
			main = lipgloss.JoinHorizontal(lipgloss.Top, tableBox, detailBox)
		}
		sections = append(sections, main)
	}

	if m.statusMessage != "" {
		status := lipgloss.NewStyle().Faint(true).Render(m.statusMessage)
		sections = append(sections, status)
	}

	if m.showHelp {
		help := "Use arrows or j/k to navigate · q or esc to quit · ? to hide this help"
		sections = append(sections, lipgloss.NewStyle().Faint(true).Render(help))
	}

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}
