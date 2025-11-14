package tableview

import (
	"context"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/require"

	cmd "github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/iostreams"
	"github.com/kong/kongctl/internal/theme"
)

type sampleRecord struct {
	ID               string
	DisplayName      string
	LocalUpdatedTime string
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
