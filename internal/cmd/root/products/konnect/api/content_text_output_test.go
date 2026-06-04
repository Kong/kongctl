package api

import (
	"testing"

	cmdCommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/cmd/output/tableview"
	"github.com/kong/kongctl/internal/iostreams"
	"github.com/segmentio/cli"
	"github.com/stretchr/testify/require"
)

func TestAPIDocumentDetailTextOutputOmitsContentField(t *testing.T) {
	record := apiDocumentDetailRecord{
		ID:               "b130...",
		RawID:            "b130f7c0-0000-4000-8000-000000000000",
		Title:            "Getting Started",
		Slug:             "getting-started",
		Status:           "published",
		ParentDocumentID: valueNA,
		LocalCreatedTime: "2025-08-27 14:42:18",
		LocalUpdatedTime: "2025-08-27 14:42:18",
		content:          normalizeAPIDocumentContent("# Getting Started\n\nDocument body content"),
	}

	output := renderAPIRecordAsText(t, record)

	require.NotContains(t, output, "CONTENT")
	require.NotContains(t, output, "# Getting Started")
	require.NotContains(t, output, "Document body content")
}

func TestAPIVersionDetailTextOutputOmitsSpecContentField(t *testing.T) {
	record := apiVersionDetailRecord{
		ID:               "c130...",
		RawID:            "c130f7c0-0000-4000-8000-000000000000",
		Version:          "1.0.0",
		SpecType:         "oas3",
		LocalCreatedTime: "2025-08-27 14:42:18",
		LocalUpdatedTime: "2025-08-27 14:42:18",
		specContent:      normalizeAPIVersionContent("openapi: 3.0.0\ninfo:\n  title: Example API"),
	}

	output := renderAPIRecordAsText(t, record)

	require.NotContains(t, output, "SPEC CONTENT")
	require.NotContains(t, output, "openapi: 3.0.0")
	require.NotContains(t, output, "title: Example API")
}

func renderAPIRecordAsText(t *testing.T, record any) string {
	t.Helper()

	streams, _, outBuf, _ := iostreams.NewTestIOStreams()
	printer, err := cli.Format("text", streams.Out)
	require.NoError(t, err)

	err = tableview.RenderForFormat(
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
	printer.Flush()

	return outBuf.String()
}
