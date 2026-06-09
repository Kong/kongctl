package executor

import (
	"context"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIAdapter_MapUpdateFieldsPreservesNullAttributeValues(t *testing.T) {
	t.Parallel()

	adapter := NewAPIAdapter(nil)
	execCtx := NewExecutionContext(&planner.PlannedChange{
		Namespace: "default",
	})

	fields := map[string]any{
		"attributes": map[string]any{
			"owner":     nil,
			"lifecycle": nil,
		},
	}

	var update kkComps.UpdateAPIRequest
	require.NoError(t, adapter.MapUpdateFields(context.Background(), execCtx, fields, &update, nil))
	assert.Equal(t, map[string]any{
		"owner":     nil,
		"lifecycle": nil,
	}, update.Attributes)
}
