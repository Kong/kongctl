package executor

import (
	"testing"

	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/stretchr/testify/require"
)

func TestResolveVirtualClusterDestinationPrefersHydratedDependencyID(t *testing.T) {
	fields := listenerPolicyFieldsWithDestination(map[string]any{planner.FieldName: "virtual-name"})
	execCtx := &ExecutionContext{PlannedChange: &planner.PlannedChange{
		References: map[string]planner.ReferenceInfo{
			planner.FieldEventGatewayVirtualClusterID: {
				Ref: "virtual-ref",
				ID:  "virtual-id",
			},
		},
	}}

	resolveVirtualClusterDestination(fields, execCtx)

	destination := listenerPolicyDestination(t, fields)
	require.Equal(t, "virtual-id", destination[planner.FieldID])
	require.NotContains(t, destination, planner.FieldName)
}

func TestResolveVirtualClusterDestinationKeepsNameWithoutPlannedDependency(t *testing.T) {
	fields := listenerPolicyFieldsWithDestination(map[string]any{planner.FieldName: "virtual-name"})

	resolveVirtualClusterDestination(fields, nil)

	destination := listenerPolicyDestination(t, fields)
	require.Equal(t, "virtual-name", destination[planner.FieldName])
	require.NotContains(t, destination, planner.FieldID)
}

func listenerPolicyFieldsWithDestination(destination map[string]any) map[string]any {
	return map[string]any{
		planner.FieldConfig: map[string]any{
			planner.FieldDestination: destination,
		},
	}
}

func listenerPolicyDestination(t *testing.T, fields map[string]any) map[string]any {
	t.Helper()
	config, ok := fields[planner.FieldConfig].(map[string]any)
	require.True(t, ok)
	destination, ok := config[planner.FieldDestination].(map[string]any)
	require.True(t, ok)
	return destination
}
