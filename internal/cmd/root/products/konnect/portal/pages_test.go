package portal

import (
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	cmdCommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/cmd/output/tableview"
	"github.com/kong/kongctl/internal/iostreams"
	"github.com/segmentio/cli"
	"github.com/stretchr/testify/require"
)

func TestNormalizePortalPageSlugValue(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "empty", raw: "", want: ""},
		{name: "bare slug", raw: "guides", want: "guides"},
		{name: "slash prefixed", raw: "/guides", want: "guides"},
		{name: "wrapped in slashes", raw: "/guides/", want: "guides"},
		{name: "root page", raw: "/", want: "/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, normalizePortalPageSlugValue(tt.raw))
		})
	}
}

func TestNormalizePortalPageInfos(t *testing.T) {
	normalized := normalizePortalPageInfos([]kkComps.PortalPageInfo{
		{
			ID:    "parent",
			Slug:  "/guides",
			Title: "Guides",
			Children: []kkComps.PortalPageInfo{
				{
					ID:    "child",
					Slug:  "/getting-started",
					Title: "Getting Started",
				},
			},
		},
	})

	require.Len(t, normalized, 1)
	require.Equal(t, "guides", normalized[0].Slug)
	require.Len(t, normalized[0].Children, 1)
	require.Equal(t, "getting-started", normalized[0].Children[0].Slug)
}

func TestFindPageBySlugOrTitle(t *testing.T) {
	pages := []kkComps.PortalPageInfo{
		{
			ID:    "page-1",
			Slug:  "/getting-started",
			Title: "Getting Started",
		},
	}

	t.Run("matches normalized slug", func(t *testing.T) {
		match := findPageBySlugOrTitle(pages, "getting-started")
		require.NotNil(t, match)
		require.Equal(t, "page-1", match.ID)
	})

	t.Run("matches slash-prefixed slug", func(t *testing.T) {
		match := findPageBySlugOrTitle(pages, "/getting-started")
		require.NotNil(t, match)
		require.Equal(t, "page-1", match.ID)
	})

	t.Run("matches title", func(t *testing.T) {
		match := findPageBySlugOrTitle(pages, "Getting Started")
		require.NotNil(t, match)
		require.Equal(t, "page-1", match.ID)
	})
}

func TestPortalPageDetailTextOutputOmitsContentField(t *testing.T) {
	content := `---
title: "APIs"
description: "Explore a wide range of API products in our Developer Portal designed for fast, flexible development."
---

::apis-list
---
persist-page-number: true
cta-text: "View APIs"
---
::`
	record := portalPageDetailRecord{
		ID:               "b9f7...",
		Title:            "APIs",
		Slug:             "apis",
		Visibility:       "private",
		Status:           "published",
		ParentPageID:     valueNA,
		LocalCreatedTime: "2025-08-27 14:42:18",
		LocalUpdatedTime: "2025-08-27 14:42:18",
		content:          normalizePortalPageContent(content),
	}

	output := renderPortalRecordAsText(t, record)

	require.NotContains(t, output, "CONTENT")
	require.NotContains(t, output, `title: "APIs"`)
	require.NotContains(t, output, "persist-page-number")
}

func TestPortalSnippetDetailTextOutputOmitsContentField(t *testing.T) {
	record := portalSnippetDetailRecord{
		ID:               "a130...",
		Name:             "hero-snippet",
		Title:            "Hero Snippet",
		Visibility:       "public",
		Status:           "published",
		Description:      "Reusable page hero",
		LocalCreatedTime: "2025-08-27 14:42:18",
		LocalUpdatedTime: "2025-08-27 14:42:18",
		content:          "snippet-frontmatter: true\n\n::hero\nSnippet content\n::",
	}

	output := renderPortalRecordAsText(t, record)

	require.NotContains(t, output, "CONTENT")
	require.NotContains(t, output, "snippet-frontmatter")
	require.NotContains(t, output, "Snippet content")
}

func renderPortalRecordAsText(t *testing.T, record any) string {
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
