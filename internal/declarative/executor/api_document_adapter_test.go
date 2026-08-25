package executor

import (
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/common"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/stretchr/testify/require"
)

func TestAPIDocumentAdapterRequiresEffectiveTitle(t *testing.T) {
	t.Parallel()

	adapter := NewAPIDocumentAdapter(nil)
	fields := map[string]any{
		planner.FieldContent: "API documentation",
	}

	require.Equal(t, []string{planner.FieldTitle, planner.FieldContent}, adapter.RequiredFields())
	require.ErrorContains(
		t,
		common.ValidateRequiredFields(fields, adapter.RequiredFields()),
		"required field 'title' is missing",
	)

	var request kkComps.CreateAPIDocumentRequest
	require.ErrorContains(t, adapter.MapCreateFields(t.Context(), nil, fields, &request), "title is required")
}
