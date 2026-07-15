package tableview

import (
	"context"
	"strings"
	"testing"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"

	cmd "github.com/kong/kongctl/internal/cmd"
	cmdCommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/iostreams"
	"github.com/kong/kongctl/internal/theme"
)

type sampleRecord struct {
	ID               string
	DisplayName      string
	LocalUpdatedTime string
}

type labeledRecord struct {
	Name        string
	Description string
	Labels      map[string]string
	Endpoint    string
	ID          string
}

func executeCmd(t *testing.T, model *bubbleModel, cmd tea.Cmd) *bubbleModel {
	t.Helper()
	queue := []tea.Cmd{cmd}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if current == nil {
			continue
		}
		msg := current()
		switch m := msg.(type) {
		case tea.BatchMsg:
			queue = append(queue, []tea.Cmd(m)...)
			continue
		case nil:
			continue
		}
		updated, next := model.Update(msg)
		bm, ok := updated.(*bubbleModel)
		require.True(t, ok)
		model = bm
		if next != nil {
			queue = append(queue, next)
		}
	}
	return model
}

func TestRender_StaticOutput(t *testing.T) {
	streams, _, outBuf, _ := iostreams.NewTestIOStreams()
	data := []sampleRecord{
		{
			ID:               "12345678-1234-1234-1234-123456789012",
			DisplayName:      "Alpha",
			LocalUpdatedTime: "2025-10-10 15:43:04",
		},
		{
			ID:               "22345678-1234-1234-1234-123456789012",
			DisplayName:      "Beta",
			LocalUpdatedTime: "2025-11-11 10:01:02",
		},
	}

	widths, minWidths := calculateColumnWidths(
		[]string{"ID", "DISPLAY NAME", "LOCAL UPDATED TIME"},
		[][]string{{data[0].ID, data[0].DisplayName, data[0].LocalUpdatedTime}},
		120,
	)
	require.GreaterOrEqual(t, len(widths), 3)
	require.GreaterOrEqual(t, len(minWidths), 3)
	require.GreaterOrEqual(t, widths[2], len(data[0].LocalUpdatedTime))
	require.GreaterOrEqual(t, minWidths[2], len("LOCAL UPDATED TIME"))

	err := Render(streams, data, WithTitle("Sample Results"))
	require.NoError(t, err)

	output := outBuf.String()
	require.Contains(t, output, "Sample Results")
	require.Contains(t, output, "DISPLAY NAME")
	require.Contains(t, output, "Alpha")
	require.Contains(t, output, "2025-10-10 15:43:04")
	require.Contains(t, output, "LOCAL UPDATED TIME")
}

func TestNormalizeSelectedRow_HandlesAnsiResetStyle(t *testing.T) {
	selected := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#111111")).
		Background(lipgloss.Color("#AABBCC"))
	cell := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF"))

	prefix, reset := selectionPrefix(selected)
	require.NotEmpty(t, prefix)
	require.Equal(t, ansi.ResetStyle, reset)

	content := selected.Render(cell.Render("foo") + reset + "bar")
	normalized := NormalizeSelectedRow(content, selected)

	require.Equal(t, "foobar", ansi.Strip(normalized))
	require.Contains(t, normalized, prefix)
	require.NotContains(t, normalized, cell.Render("foo"))
}

func TestNormalizeSelectedRow_RepaintsTableCellForeground(t *testing.T) {
	selected := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#000F06")).
		Background(lipgloss.Color("#CCFF00"))
	cell := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF"))

	styles := table.DefaultStyles()
	styles.Cell = styles.Cell.Foreground(lipgloss.Color("#FFFFFF"))
	styles.Selected = selected

	tbl := table.New(
		table.WithColumns([]table.Column{{Title: "RESOURCE", Width: 10}}),
		table.WithRows([]table.Row{{"apis"}}),
		table.WithFocused(true),
		table.WithStyles(styles),
	)
	tbl.SetCursor(0)
	tbl.SetHeight(3)
	tbl.SetWidth(20)

	normalized := NormalizeSelectedRow(tbl.View(), selected)
	prefix, _ := selectionPrefix(selected)

	require.Contains(t, ansi.Strip(normalized), "apis")
	require.Contains(t, normalized, prefix)
	require.NotContains(t, normalized, cell.Render("apis"))
}

func TestNewDetailTableDecorator_TracksSelectedPrefix(t *testing.T) {
	selected := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#111111")).
		Background(lipgloss.Color("#AABBCC"))

	decorator := newDetailTableDecorator(
		[]table.Column{{Title: "FIELD", Width: 10}, {Title: "VALUE", Width: 10}},
		theme.Current(),
		selected,
		true,
	)

	require.NotEmpty(t, decorator.selectedPrefix)
}

func TestFormatHeader(t *testing.T) {
	cases := map[string]string{
		"ID":               "ID",
		"DisplayName":      "DISPLAY NAME",
		"LocalUpdatedTime": "LOCAL UPDATED TIME",
		"DCRProvider":      "DCR PROVIDER",
		"HTTPStatusCode":   "HTTP STATUS CODE",
	}

	for input, expected := range cases {
		result := formatHeader(input)
		if strings.Compare(result, expected) != 0 {
			t.Fatalf("formatHeader(%q) = %q, expected %q", input, result, expected)
		}
	}
}

func TestEnrichDetailItems_MapField(t *testing.T) {
	parent := &struct {
		Labels map[string]string
	}{
		Labels: map[string]string{
			"foo": "bar",
		},
	}

	items := []detailItem{
		{Label: "labels"},
	}

	enriched := enrichDetailItems(items, "", parent)
	require.Len(t, enriched, 1)
	require.Equal(t, complexStructIndicator, enriched[0].Value)
	require.NotNil(t, enriched[0].Loader)

	child, err := enriched[0].Loader(context.Background(), nil, parent)
	require.NoError(t, err)
	require.Equal(t, ChildViewModeDetail, child.Mode)
	require.Empty(t, child.Headers)
	require.Empty(t, child.Rows)
	require.NotNil(t, child.DetailRenderer)
	content := child.DetailRenderer(0)
	require.Contains(t, content, "foo: bar")
}

func TestEnrichDetailItems_EmptyMapField(t *testing.T) {
	parent := &struct {
		Labels map[string]string
	}{
		Labels: map[string]string{},
	}

	items := []detailItem{
		{Label: "labels"},
	}

	enriched := enrichDetailItems(items, "", parent)
	require.Len(t, enriched, 1)
	require.Equal(t, complexEmptyIndicator, enriched[0].Value)
	require.NotNil(t, enriched[0].Loader)

	child, err := enriched[0].Loader(context.Background(), nil, parent)
	require.NoError(t, err)
	require.Equal(t, ChildViewModeDetail, child.Mode)
	require.NotNil(t, child.DetailRenderer)
	require.Contains(t, child.DetailRenderer(0), "(no data)")
}

func TestEnrichDetailItems_SliceField(t *testing.T) {
	parent := &struct {
		IDs []string
	}{
		IDs: []string{"abc", "defg"},
	}

	items := []detailItem{
		{Label: "ids"},
	}

	enriched := enrichDetailItems(items, "", parent)
	require.Len(t, enriched, 1)
	require.Equal(t, complexExpandableIndicator, enriched[0].Value)
	require.NotNil(t, enriched[0].Loader)

	child, err := enriched[0].Loader(context.Background(), nil, parent)
	require.NoError(t, err)
	require.Equal(t, []string{"#", "VALUE"}, child.Headers)
	require.Len(t, child.Rows, 2)
	require.Equal(t, "1", child.Rows[0][0])
	require.Equal(t, "abc", child.Rows[0][1])
	require.Equal(t, "2", child.Rows[1][0])
	require.Equal(t, "defg", child.Rows[1][1])
	require.NotNil(t, child.DetailRenderer)
}

func TestRowLoaderOpensChildView(t *testing.T) {
	columns := []table.Column{{Title: "RESOURCE", Width: 12}}
	rows := []table.Row{{"apis"}}
	tbl := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
	)

	palette := theme.Current()
	loaderCalls := 0

	cfg := config{
		rootLabel: "Konnect",
		rowLoader: func(int) (ChildView, error) {
			loaderCalls++
			return ChildView{
				Title:   "APIs",
				Headers: []string{"ID"},
				Rows:    []table.Row{{"1234"}},
			}, nil
		},
	}

	model := newBubbleModel(
		tbl,
		cfg,
		lipgloss.NewStyle(),
		lipgloss.NewStyle(),
		lipgloss.NewStyle(),
		lipgloss.NewStyle(),
		nil,
		palette,
		80,
		24,
		len(rows),
		[]string{"RESOURCE"},
	)

	started, cmd := model.openRowChild()
	require.True(t, started)
	model = executeCmd(t, model, cmd)
	require.Equal(t, 1, loaderCalls)
	require.Len(t, model.detailStack, 1)
	require.Equal(t, "APIs", model.detailStack[0].title)
	require.Equal(t, []string{"Konnect", "APIs"}, model.breadcrumbs)
	require.NotNil(t, model.detailStack[0].child)
	require.Len(t, model.detailStack[0].child.rows, 1)
}

func TestInitialRowSelectionOpensChildView(t *testing.T) {
	columns := []table.Column{{Title: "RESOURCE", Width: 12}}
	rows := []table.Row{{"apis"}, {"portals"}}
	tbl := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
	)

	palette := theme.Current()
	loaderCalls := 0

	cfg := config{
		rootLabel: "Konnect",
		rowLoader: func(index int) (ChildView, error) {
			loaderCalls++
			return ChildView{
				Title:   strings.ToUpper(rows[index][0]),
				Headers: []string{"NAME"},
				Rows:    []table.Row{{rows[index][0]}},
			}, nil
		},
		initialRow:  1,
		openInitial: true,
	}

	model := newBubbleModel(
		tbl,
		cfg,
		lipgloss.NewStyle(),
		lipgloss.NewStyle(),
		lipgloss.NewStyle(),
		lipgloss.NewStyle(),
		nil,
		palette,
		80,
		24,
		len(rows),
		[]string{"RESOURCE"},
	)

	model = executeCmd(t, model, model.Init())
	require.Equal(t, 1, loaderCalls)
	require.Len(t, model.detailStack, 1)
	require.Equal(t, []string{"Konnect", "PORTALS"}, model.breadcrumbs)
	require.Equal(t, "PORTALS", model.detailStack[0].title)
	require.NotNil(t, model.detailStack[0].child)
	require.Len(t, model.detailStack[0].child.rows, 1)
}

func TestRowLoaderProvidesParentContext(t *testing.T) {
	type api struct{ ID string }

	columns := []table.Column{{Title: "RESOURCE", Width: 12}}
	rows := []table.Row{{"apis"}}
	tbl := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
	)

	palette := theme.Current()
	parent := &api{ID: "a1"}

	cfg := config{
		rootLabel: "Konnect",
		rowLoader: func(index int) (ChildView, error) {
			return ChildView{
				Title:      "APIs",
				Headers:    []string{"ID"},
				Rows:       []table.Row{{"resource"}},
				ParentType: "api",
				DetailContext: func(idx int) any {
					if idx != index {
						return nil
					}
					return parent
				},
			}, nil
		},
	}

	model := newBubbleModel(
		tbl,
		cfg,
		lipgloss.NewStyle(),
		lipgloss.NewStyle(),
		lipgloss.NewStyle(),
		lipgloss.NewStyle(),
		nil,
		palette,
		80,
		24,
		len(rows),
		[]string{"RESOURCE"},
	)

	started, cmd := model.openRowChild()
	require.True(t, started)
	model = executeCmd(t, model, cmd)
	require.Equal(t, parent, model.detailStack[0].parent)
}

func TestSanitizePreviewDetailContent_RemovesComplexFields(t *testing.T) {
	description := "Provides flights"
	type api struct {
		ID          string
		Name        string
		Labels      map[string]string
		Portals     []string
		Description string
	}

	parent := &api{
		ID:          "c2a0",
		Name:        "Flights",
		Labels:      map[string]string{"env": "prod"},
		Portals:     []string{"portal-a"},
		Description: description,
	}

	raw := "id: c2a0\n" +
		"name: Flights\n" +
		"labels: [...]\n" +
		"portals: [...]\n" +
		"description:\n  Provides flights\n"

	sanitized := sanitizePreviewDetailContent(raw, "api", parent)
	require.NotContains(t, sanitized, "labels")
	require.NotContains(t, sanitized, "portals")
	require.Contains(t, sanitized, "id: c2a0")
	require.Contains(t, sanitized, "name: Flights")
	require.Contains(t, sanitized, "description")
}

func TestSanitizePreviewDetailContent_FallbackToRaw(t *testing.T) {
	type api struct {
		Portals []string
	}

	parent := &api{Portals: []string{"portal-a"}}
	raw := "portals: [...]"
	sanitized := sanitizePreviewDetailContent(raw, "api", parent)
	require.Equal(t, raw, sanitized)
}

func TestFilterPreviewDetailItems_RemovesChildRows(t *testing.T) {
	dummy := func(context.Context, cmd.Helper, any) (ChildView, error) {
		return ChildView{}, nil
	}

	items := []detailItem{
		{Label: "id", Value: "123"},
		{Label: "documents", Value: childFieldIndicator, Loader: dummy},
	}

	filtered := filterPreviewDetailItems(items)
	require.Len(t, filtered, 1)
	require.Equal(t, "id", filtered[0].Label)
}

func TestIsEmptyCollection(t *testing.T) {
	require.True(t, isEmptyCollection([]string{}))
	require.True(t, isEmptyCollection([]sampleRecord{}))
	require.True(t, isEmptyCollection([0]int{}))

	ptr := []string{}
	require.True(t, isEmptyCollection(&ptr))

	// typed nil slice (len==0) is considered empty
	var typedNil []string
	require.True(t, isEmptyCollection(typedNil))

	require.False(t, isEmptyCollection(nil))
	require.False(t, isEmptyCollection([]string{"a"}))
	require.False(t, isEmptyCollection(sampleRecord{}))
	require.False(t, isEmptyCollection("not a slice"))

	var nilPtr *[]string
	require.False(t, isEmptyCollection(nilPtr))
}

// stubPrinter is a minimal cli.PrintFlusher that records what was passed to Print.
type stubPrinter struct {
	printed []any
}

func (s *stubPrinter) Print(v any) { s.printed = append(s.printed, v) }
func (s *stubPrinter) Flush()      {}

func TestRenderForFormat_EmptyTextOutputWritesNoResourcesFound(t *testing.T) {
	streams, _, outBuf, _ := iostreams.NewTestIOStreams()

	err := RenderForFormat(
		nil,
		false,
		cmdCommon.TEXT,
		nil,
		streams,
		[]sampleRecord{},
		nil,
		"",
	)
	require.NoError(t, err)
	require.Contains(t, outBuf.String(), "No resources found.")
}

func TestRenderForFormat_NonEmptyTextOutputUsesCompactTable(t *testing.T) {
	streams, _, outBuf, _ := iostreams.NewTestIOStreams()
	printer := &stubPrinter{}

	record := []sampleRecord{{ID: "abc", DisplayName: "Test", LocalUpdatedTime: "2025-01-01"}}
	err := RenderForFormat(
		nil,
		false,
		cmdCommon.TEXT,
		printer,
		streams,
		record,
		nil,
		"",
	)
	require.NoError(t, err)
	require.NotContains(t, outBuf.String(), "No resources found.")
	require.Empty(t, printer.printed)
	require.Contains(t, outBuf.String(), "DISPLAY NAME")
	require.Contains(t, outBuf.String(), "Test")
	require.NotContains(t, outBuf.String(), "LOCAL UPDATED TIME")
}

func TestRenderForFormat_DefaultTextOmitsWideMetadataAndPutsIDLast(t *testing.T) {
	streams, _, outBuf, _ := iostreams.NewTestIOStreams()
	record := labeledRecord{
		Name:        "payments",
		Description: "a long description",
		Labels:      map[string]string{"team": "platform"},
		Endpoint:    "https://example.com/a/long/path",
		ID:          "12345678-1234-1234-1234-123456789012",
	}

	err := RenderForFormat(nil, false, cmdCommon.TEXT, nil, streams, record, record, "")
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(outBuf.String()), "\n")
	require.Len(t, lines, 2)
	require.Equal(t, "NAME      ID", lines[0])
	require.Contains(t, lines[1], "payments")
	require.NotContains(t, outBuf.String(), "LABELS")
	require.NotContains(t, outBuf.String(), "DESCRIPTION")
	require.NotContains(t, outBuf.String(), "ENDPOINT")
}

func TestRenderForFormat_ExplicitDefaultDescriptionIsTruncated(t *testing.T) {
	streams, _, outBuf, _ := iostreams.NewTestIOStreams()
	description := strings.Repeat("description ", 8)
	record := labeledRecord{
		Name:        "payments",
		Description: description,
		ID:          "12345678-1234-1234-1234-123456789012",
	}

	err := RenderForFormat(
		nil,
		false,
		cmdCommon.TEXT,
		nil,
		streams,
		record,
		record,
		"",
		WithCustomTable(
			[]string{"NAME", "DESCRIPTION", "ID"},
			[]table.Row{{record.Name, record.Description, record.ID}},
		),
		WithDefaultDescription(),
	)
	require.NoError(t, err)
	require.Contains(t, outBuf.String(), "DESCRIPTION")
	require.Contains(t, outBuf.String(), "…")
	require.NotContains(t, outBuf.String(), description)
}

func newMinimalBubbleModel(t *testing.T) *bubbleModel {
	t.Helper()
	columns := []table.Column{{Title: "NAME", Width: 12}}
	rows := []table.Row{{"alpha"}, {"beta"}}
	tbl := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
	)
	palette := theme.Current()
	return newBubbleModel(
		tbl,
		config{rootLabel: "Root"},
		lipgloss.NewStyle(),
		lipgloss.NewStyle(),
		lipgloss.NewStyle(),
		lipgloss.NewStyle(),
		nil,
		palette,
		80,
		24,
		len(rows),
		[]string{"NAME"},
	)
}

func sendKey(t *testing.T, model *bubbleModel, key string) *bubbleModel {
	t.Helper()
	msg := tea.KeyPressMsg{Text: key, Code: []rune(key)[0]}
	updated, _ := model.Update(msg)
	bm, ok := updated.(*bubbleModel)
	require.True(t, ok)
	return bm
}

func sendSpecialKey(t *testing.T, model *bubbleModel, code rune, text string) (*bubbleModel, tea.Cmd) {
	t.Helper()
	msg := tea.KeyPressMsg{Text: text, Code: code}
	updated, cmd := model.Update(msg)
	bm, ok := updated.(*bubbleModel)
	require.True(t, ok)
	return bm, cmd
}

func TestSelectionActionDialogRunsWithCollectedValues(t *testing.T) {
	model := newMinimalBubbleModel(t)

	var captured SelectionContext
	var ranWith SelectionActionValues
	model.selectionActions = []SelectionAction{
		{
			Key:  "d",
			Help: "dump selected resource",
			Resolve: func(selection SelectionContext) (SelectionActionCommand, error) {
				captured = selection
				return SelectionActionCommand{
					Title:               "Dump alpha",
					Label:               "Dump alpha",
					DefaultOutputFile:   "alpha.yaml",
					DefaultNamespace:    "team-alpha",
					IncludeChildrenText: "include child resources",
					Run: func(values SelectionActionValues) error {
						ranWith = values
						return nil
					},
				}, nil
			},
		},
	}

	model = sendKey(t, model, "d")
	require.NotNil(t, model.actionDialog)
	require.Equal(t, "alpha", captured.Label)
	require.Equal(t, table.Row{"alpha"}, captured.Row)
	dialogView := ansi.Strip(model.renderSelectionActionDialog())
	require.Regexp(t, `Output file:[ \t]+alpha.yaml`, dialogView)
	require.Regexp(t, `Default namespace:[ \t]+team-alpha`, dialogView)

	model, _ = sendSpecialKey(t, model, tea.KeyTab, "")
	require.Equal(t, selectionActionFocusNamespace, model.actionDialog.focus)

	model, _ = sendSpecialKey(t, model, tea.KeyTab, "")
	require.Equal(t, selectionActionFocusIncludeChildren, model.actionDialog.focus)

	model, _ = sendSpecialKey(t, model, tea.KeySpace, " ")
	require.True(t, model.actionDialog.includeChildren)

	var cmd tea.Cmd
	model, cmd = sendSpecialKey(t, model, tea.KeyEnter, "")
	require.NotNil(t, cmd)
	model = executeCmd(t, model, cmd)

	require.Nil(t, model.actionDialog)
	require.Equal(t, "alpha.yaml", ranWith.OutputFile)
	require.Equal(t, "team-alpha", ranWith.DefaultNamespace)
	require.True(t, ranWith.IncludeChildren)
	require.Contains(t, model.statusMessage, "Dump alpha")
	require.Contains(t, model.statusMessage, "alpha.yaml")
}

func TestThemeCycling_AdvancesPaletteAndSetsStatus(t *testing.T) {
	model := newMinimalBubbleModel(t)

	available := theme.Available()
	require.NotEmpty(t, available, "at least one theme must be registered")

	initialPaletteName := model.palette.Name
	initialIndex := model.themeIndex

	model = sendKey(t, model, "t")

	expectedIndex := (initialIndex + 1) % len(available)
	expectedName := available[expectedIndex]

	require.Equal(t, expectedIndex, model.themeIndex)
	require.Equal(t, expectedName, model.palette.Name)
	if len(available) > 1 {
		require.NotEqual(t, initialPaletteName, model.palette.Name)
	}
	require.Contains(t, model.statusMessage, expectedName)
	require.Contains(t, model.statusMessage, "color-theme")
}

func TestThemeCycling_ReversesWithShiftT(t *testing.T) {
	model := newMinimalBubbleModel(t)

	available := theme.Available()
	require.NotEmpty(t, available, "at least one theme must be registered")

	initialIndex := model.themeIndex

	model = sendKey(t, model, "T")

	expectedIndex := (initialIndex - 1 + len(available)) % len(available)
	expectedName := available[expectedIndex]

	require.Equal(t, expectedIndex, model.themeIndex)
	require.Equal(t, expectedName, model.palette.Name)
	require.Contains(t, model.statusMessage, expectedName)
	require.Contains(t, model.statusMessage, "color-theme")
}

func TestThemeCycling_IgnoredWhenSearchActive(t *testing.T) {
	model := newMinimalBubbleModel(t)

	model.searchActive = true
	initialIndex := model.themeIndex
	initialPaletteName := model.palette.Name

	model = sendKey(t, model, "t")
	require.Equal(t, initialIndex, model.themeIndex)
	require.Equal(t, initialPaletteName, model.palette.Name)

	model = sendKey(t, model, "T")
	require.Equal(t, initialIndex, model.themeIndex)
	require.Equal(t, initialPaletteName, model.palette.Name)
}

func TestFormatDetailValue_DereferencesPointers(t *testing.T) {
	// string pointer
	str := "hello"
	require.Equal(t, "hello", formatDetailValue(&str))

	// int pointer
	num := 42
	require.Equal(t, "42", formatDetailValue(&num))

	// Double pointer
	pstr := &str
	require.Equal(t, "hello", formatDetailValue(&pstr))

	// Direct values unchanged
	require.Equal(t, "world", formatDetailValue("world"))
	require.Equal(t, "123", formatDetailValue(123))

	// Nil cases
	require.Equal(t, "nil", formatDetailValue(nil))
	var nilPtr *string
	require.Equal(t, "nil", formatDetailValue(nilPtr))
}
