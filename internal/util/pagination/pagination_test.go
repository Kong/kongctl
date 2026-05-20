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
