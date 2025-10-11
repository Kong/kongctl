package tableview

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kong/kongctl/internal/iostreams"
)

type sampleRecord struct {
	ID               string
	DisplayName      string
	LocalUpdatedTime string
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
	if len(widths) == 3 {
		require.GreaterOrEqual(t, widths[2], len(data[0].LocalUpdatedTime))
	}
	if len(minWidths) == 3 {
		require.GreaterOrEqual(t, minWidths[2], len("LOCAL UPDATED TIME"))
	}

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
