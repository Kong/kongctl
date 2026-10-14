package executor

import (
	"testing"

	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/stretchr/testify/require"
)

func TestFeatureSavedPlanRejectsUnsupportedUpdates(t *testing.T) {
	adapter := NewFeatureAdapter(nil)
	for _, field := range []string{
		planner.FieldName, planner.FieldKey, planner.FieldMeter, planner.FieldLabels, planner.FieldDescription,
	} {
		t.Run(field, func(t *testing.T) {
			var request helpers.FeatureUpdateRequest
			require.ErrorContains(
				t,
				adapter.MapUpdateFields(t.Context(), nil, map[string]any{field: "changed"}, &request, nil),
				"only unit_cost",
			)
		})
	}
	var request helpers.FeatureUpdateRequest
	require.NoError(
		t,
		adapter.MapUpdateFields(t.Context(), nil, map[string]any{planner.FieldUnitCost: nil}, &request, nil),
	)
	require.True(t, request.UnitCostSet)
	require.Nil(t, request.UnitCost)
}
