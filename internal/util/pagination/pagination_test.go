package pagination

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractPageAfterCursor_DecodesFallbackQuerySnippet(t *testing.T) {
	t.Run("plain key encoded value", func(t *testing.T) {
		next := "page[after]=cursor%2Fvalue"
		require.Equal(t, "cursor/value", ExtractPageAfterCursor(&next))
	})

	t.Run("encoded key and value", func(t *testing.T) {
		next := "page%5Bafter%5D=cursor%2Fvalue"
		require.Equal(t, "cursor/value", ExtractPageAfterCursor(&next))
	})
}

func TestExtractPageAfterCursor(t *testing.T) {
	const issueCursor = "KVpEDAMKQ0FFXRZTFA19EFoXMWYNDVpODUBQABEQTUFcShZUFQ"
	for _, tc := range []struct{ name, next, want string }{
		{"empty", "", ""},
		{"whitespace", " \n ", ""},
		{"bare cursor", issueCursor, issueCursor},
		{"opaque cursor", "abc+def/ghi==", "abc+def/ghi=="},
		{"leading slash in opaque cursor", "/abc+def==", "/abc+def=="},
		{"opaque percent", "abc%2Fdef", "abc%2Fdef"},
		{"absolute URL", "https://example.test/items?page%5Bafter%5D=abc%2Bdef%2Fghi%3D%3D", "abc+def/ghi=="},
		{"relative URL", "/items?page[after]=next", "next"},
		{"query", "?page[after]=next&page[size]=100", "next"},
		{"URL without cursor", "https://example.test/items?page[size]=100", ""},
		{"relative URL without cursor", "/items?page[size]=100", ""},
		{"empty cursor", "?page[after]=", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, ExtractPageAfterCursor(&tc.next))
		})
	}
	require.Empty(t, ExtractPageAfterCursor(nil))
}
