package portal

import (
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
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
