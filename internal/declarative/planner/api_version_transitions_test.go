package planner

import (
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/stretchr/testify/require"
)

func TestAPIVersionTransitionDependencies(t *testing.T) {
	for _, targetAction := range []ActionType{ActionCreate, ActionUpdate} {
		t.Run(string(targetAction), func(t *testing.T) {
			changes := []PlannedChange{
				{
					ID: "switch", ResourceType: ResourceTypeAPI, ResourceRef: "bookings", ResourceID: "api-id",
					Action: ActionUpdate, Fields: map[string]any{FieldVersion: "beta.2"},
					ChangedFields: map[string]FieldChange{FieldVersion: {Old: "beta.1", New: "beta.2"}},
				},
				{
					ID: "target", ResourceType: ResourceTypeAPIVersion, Action: targetAction,
					Parent: &ParentInfo{Ref: "bookings", ID: "api-id"}, Fields: map[string]any{FieldVersion: "beta.2"},
				},
				{
					ID: "old", ResourceType: ResourceTypeAPIVersion, Action: ActionDelete,
					Parent: &ParentInfo{Ref: "bookings", ID: "api-id"}, Fields: map[string]any{FieldVersion: "beta.1"},
				},
				{
					ID: "other-api", ResourceType: ResourceTypeAPIVersion, Action: ActionDelete,
					Parent: &ParentInfo{Ref: "bookings", ID: "different-api"}, Fields: map[string]any{FieldVersion: "beta.1"},
				},
				{
					ID: "other-version", ResourceType: ResourceTypeAPIVersion, Action: ActionDelete,
					Parent: &ParentInfo{ID: "api-id"}, Fields: map[string]any{FieldVersion: "beta.0"},
				},
			}
			adjustAPIVersionTransitionDependencies(changes)
			adjustAPIVersionTransitionDependencies(changes)
			require.Equal(t, []string{"target"}, changes[0].DependsOn)
			require.Equal(t, []string{"switch"}, changes[2].DependsOn)
			require.Empty(t, changes[3].DependsOn)
			require.Empty(t, changes[4].DependsOn)
			resolved, err := NewDependencyResolver().ResolveDependenciesWithGroups(changes)
			require.NoError(t, err)
			require.Len(t, resolved.ExecutionGroups, 3)
			require.Contains(t, resolved.ExecutionGroups[0], "target")
			require.Equal(t, []string{"switch"}, resolved.ExecutionGroups[1])
			require.Equal(t, []string{"old"}, resolved.ExecutionGroups[2])
		})
	}
}

func TestAPIVersionTransitionRequiresVersionSwitch(t *testing.T) {
	for _, action := range []ActionType{ActionCreate, ActionUpdate, ActionDelete} {
		changes := []PlannedChange{
			{
				ID: "api", ResourceType: ResourceTypeAPI, ResourceRef: "api", Action: action,
				Fields: map[string]any{FieldName: "renamed"},
			},
			{
				ID: "version", ResourceType: ResourceTypeAPIVersion, Action: ActionDelete,
				Parent: &ParentInfo{Ref: "api"}, Fields: map[string]any{FieldVersion: "beta.1"},
			},
		}
		adjustAPIVersionTransitionDependencies(changes)
		require.Empty(t, changes[0].DependsOn)
		require.Empty(t, changes[1].DependsOn)
	}
}

func TestAPIOperationRemovalAuthorization(t *testing.T) {
	for _, allowed := range []*bool{nil, new(false), new(true)} {
		rs := &resources.ResourceSet{APIs: []resources.APIResource{{
			BaseResource: resources.BaseResource{
				Ref: "api",
			},
			CreateAPIRequest: kkComps.CreateAPIRequest{Name: "API"},
			ForceDelete:      allowed,
		}}}
		changes := []PlannedChange{
			{ResourceType: ResourceTypeAPI, ResourceRef: "api", Action: ActionUpdate},
			{ResourceType: ResourceTypeAPIVersion, Action: ActionUpdate, Parent: &ParentInfo{Ref: "api"}},
			{ResourceType: ResourceTypeAPIVersion, Action: ActionCreate, Parent: &ParentInfo{Ref: "api"}},
			{ResourceType: ResourceTypeAPIVersion, Action: ActionDelete, Parent: &ParentInfo{Ref: "api"}},
			{ResourceType: ResourceTypeAPIVersion, Action: ActionUpdate, Parent: &ParentInfo{Ref: "other-api"}},
		}
		applyAPIOperationRemovalAuthorization(changes, rs)
		require.Equal(t, allowed != nil && *allowed, changes[0].ForceDelete)
		require.Equal(t, changes[0].ForceDelete, changes[1].ForceDelete)
		for _, change := range changes[2:] {
			require.False(t, change.ForceDelete)
		}
	}
}
