package portal

import (
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/stretchr/testify/require"
)

func TestBuildPortalChildViewIncludesDescription(t *testing.T) {
	description := "A portal for application developers"
	view := buildPortalChildView([]kkComps.ListPortalsResponsePortal{
		{
			ID:          "12345678-1234-1234-1234-123456789012",
			Name:        "developers",
			Description: &description,
		},
	})

	require.Equal(t, []string{"NAME", "DESCRIPTION", "ID"}, view.Headers)
	require.Len(t, view.Rows, 1)
	require.Equal(t, "developers", view.Rows[0][0])
	require.Equal(t, description, view.Rows[0][1])
}
