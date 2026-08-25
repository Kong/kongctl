package executor

import (
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/common"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIDocumentAdapterAllowsCreateWithoutTitle(t *testing.T) {
	t.Parallel()

	adapter := NewAPIDocumentAdapter(nil)
	fields := map[string]any{
		planner.FieldContent: "API documentation",
	}

	require.Equal(t, []string{planner.FieldContent}, adapter.RequiredFields())
	require.NoError(t, common.ValidateRequiredFields(fields, adapter.RequiredFields()))

	var request kkComps.CreateAPIDocumentRequest
	require.NoError(t, adapter.MapCreateFields(t.Context(), nil, fields, &request))
	assert.Nil(t, request.Title)
	assert.Equal(t, "API documentation", request.Content)
}
