package planner

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMetadataMergeIntentRequiresSupportedVersionResourceAndAction(t *testing.T) {
	change := PlannedChange{
		ID: "update", ResourceType: ResourceTypeIdentityPrincipal, ResourceRef: "worker",
		Action: ActionUpdate, MetadataMerge: true, Fields: map[string]any{FieldMetadata: map[string]any{}},
	}
	for _, version := range []string{"1.0", "1.1", CurrentPlanVersion} {
		plan := NewPlan(version, "test", PlanModeApply)
		plan.Changes = []PlannedChange{change}
		err := ValidatePlanCompatibility(plan)
		if version == CurrentPlanVersion {
			require.NoError(t, err)
		} else {
			require.Error(t, err)
		}
		plan.Changes[0].MetadataMerge = false
		require.NoError(t, ValidatePlanCompatibility(plan), "legacy replacement plans remain readable")
	}
	for _, fault := range []string{"other-resource", "create", "delete", "no-metadata", "null-metadata"} {
		plan := NewPlan(CurrentPlanVersion, "test", PlanModeApply)
		plan.Changes = []PlannedChange{change}
		invalid := &plan.Changes[0]
		switch fault {
		case "other-resource":
			invalid.ResourceType = ResourceTypeIdentityGroup
		case "create":
			invalid.Action = ActionCreate
		case "delete":
			invalid.Action = ActionDelete
		case "no-metadata":
			invalid.Fields = nil
		case "null-metadata":
			invalid.Fields = map[string]any{FieldMetadata: nil}
		}
		require.Error(t, ValidatePlanCompatibility(plan))
	}
}
