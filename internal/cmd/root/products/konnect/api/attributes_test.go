package api

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAttributeDetailView_UsesSnakeCaseLabels(t *testing.T) {
	detail := attributeDetailView("category", []string{"internal", "public"})

	require.Equal(t, "key: category\nvalue_count: 2\n\nvalues:\n  - internal\n  - public\n", detail)
}
