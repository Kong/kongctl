package executor

import (
	"context"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDashboardAdapterMapCreateFieldsDecodesPlanJSONDefinition(t *testing.T) {
	adapter := NewDashboardAdapter(nil)
	execCtx := NewExecutionContext(&planner.PlannedChange{
		ResourceRef: "traffic-summary",
		Namespace:   "analytics",
		Protection:  true,
	})
	fields := map[string]any{
		planner.FieldName: "Traffic Summary",
		planner.FieldDefinition: map[string]any{
			"tiles": []any{},
			"preset_filters": []any{
				map[string]any{
					"field":    "control_plane",
					"operator": "in",
					"value":    []any{"cp-id"},
				},
			},
		},
		planner.FieldLabels: map[string]any{
			"team": "platform",
		},
	}

	var req kkComps.DashboardUpdateRequest
	require.NoError(t, adapter.MapCreateFields(context.Background(), execCtx, fields, &req))

	assert.Equal(t, "Traffic Summary", req.Name)
	assert.NotNil(t, req.Definition.Tiles)
	require.Len(t, req.Definition.PresetFilters, 1)
	assert.Equal(t, kkComps.AllFilterItemsFieldControlPlane, req.Definition.PresetFilters[0].Field)
	assert.Equal(t, kkComps.AllFilterItemsOperatorIn, req.Definition.PresetFilters[0].Operator)
	assert.Equal(t, "analytics", req.Labels[labels.NamespaceKey])
	assert.Equal(t, labels.TrueValue, req.Labels[labels.ProtectedKey])
	assert.Equal(t, "platform", req.Labels["team"])
}

func TestDashboardAdapterMapUpdateFieldsRequiresDefinition(t *testing.T) {
	adapter := NewDashboardAdapter(nil)
	execCtx := NewExecutionContext(&planner.PlannedChange{
		ResourceRef: "traffic-summary",
		Namespace:   "analytics",
	})

	var req kkComps.DashboardUpdateRequest
	err := adapter.MapUpdateFields(context.Background(), execCtx, map[string]any{
		planner.FieldName: "Traffic Summary",
	}, &req, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "definition is required")
}
