package planner

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSDK68ProviderOrderingWithRuntimeDependencies(t *testing.T) {
	for _, downgrade := range []bool{false, true} {
		t.Run(map[bool]string{false: "upgrade", true: "downgrade"}[downgrade], func(t *testing.T) {
			plan := &Plan{Changes: []PlannedChange{
				{ID: "gateway", Action: ActionUpdate, ResourceType: ResourceTypeAIGateway},
				{
					ID: "delete-provider", Action: ActionDelete, ResourceType: ResourceTypeAIGatewayProvider,
					Parent: &ParentInfo{Ref: "gateway"}, DependsOn: []string{"update-model"},
				},
				{
					ID: "create-provider", Action: ActionCreate, ResourceType: ResourceTypeAIGatewayProvider,
					Parent: &ParentInfo{Ref: "gateway"},
				},
				{
					ID: "update-model", Action: ActionUpdate, ResourceType: ResourceTypeAIGatewayModel,
					Parent: &ParentInfo{Ref: "gateway"}, DependsOn: []string{"create-provider"},
				},
			}}
			oldVersion, newVersion := "2.0", "2.1"
			if downgrade {
				oldVersion, newVersion = newVersion, oldVersion
			}
			orderAIGatewayRuntimeChange(plan, "gateway", 1, oldVersion, newVersion)
			result, err := NewDependencyResolver().ResolveDependenciesWithGroups(plan.Changes)
			require.NoError(t, err)
			expected := []string{"gateway", "create-provider", "update-model", "delete-provider"}
			if downgrade {
				expected = []string{"create-provider", "update-model", "delete-provider", "gateway"}
			}
			require.Equal(t, expected, result.ExecutionOrder)
		})
	}
}
