package planner

import (
	"log/slog"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/konnect/helpers"
	fixture "github.com/kong/kongctl/test/konnect"
	"github.com/stretchr/testify/require"
)

func groupPlan(t *testing.T, api *fixture.GroupAPI, desired []resources.IdentityGroupResource,
	mode PlanMode, scoped bool,
) (*Plan, error) {
	t.Helper()
	directory := fixture.Directory("people", "directory-team", true)
	rs := &resources.ResourceSet{DefaultNamespace: "group-team", Identity: &resources.IdentityResource{
		Directories: []resources.IdentityDirectoryResource{{
			BaseResource: resources.BaseResource{Ref: "people"},
			External:     &resources.ExternalBlock{ID: directory.ID},
		}}, Groups: desired,
	}, SyncScope: resources.NewSyncScope()}
	rs.SyncScope.AddRoot(resources.ResourceTypeIdentityDirectory)
	if scoped {
		rs.SyncScope.AddChildNamespace(resources.ResourceTypeIdentityGroup, "people", "group-team")
	}
	return NewPlanner(state.NewClient(state.ClientConfig{
		IdentityDirectoriesAPI: &fixture.DirectoryAPI{
			Directories: []kkComps.KongDirectory{directory},
		}, IdentityGroupsAPI: api,
	}), slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: mode})
}

func groupDesired() resources.IdentityGroupResource {
	return resources.IdentityGroupResource{BaseResource: resources.BaseResource{
		Ref:     "fulfillment",
		Kongctl: &resources.KongctlMeta{Namespace: new("group-team")},
	}, Directory: "people", Name: "Fulfillment"}
}

func TestGroupMatchingIndependentOwnershipAndSync(t *testing.T) {
	id := fixture.Directory("people", "directory-team", true).ID
	current := fixture.Group("Fulfillment", "group-team", false)
	api := &fixture.GroupAPI{Directories: map[string][]helpers.IdentityGroup{id: {current}}}
	d := groupDesired()
	plan, err := groupPlan(t, api, []resources.IdentityGroupResource{d}, PlanModeApply, true)
	require.NoError(t, err)
	require.Empty(t, plan.Changes)
	d.Description = new("Changed")
	plan, err = groupPlan(t, api, []resources.IdentityGroupResource{d}, PlanModeApply, true)
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	require.Equal(t, current.ID, plan.Changes[0].ResourceID)
	require.Equal(t, id, plan.Changes[0].Parent.ID)
	require.Equal(t, "Fulfillment", plan.Changes[0].Fields[FieldName])
	require.Len(t, plan.Changes[0].Fields[FieldManagedBy], 4)
	for _, scoped := range []bool{false, true} {
		plan, err := groupPlan(t, api, nil, PlanModeSync, scoped)
		require.NoError(t, err)
		if scoped {
			require.Len(t, plan.Changes, 1)
			require.Equal(t, ActionDelete, plan.Changes[0].Action)
		} else {
			require.Empty(t, plan.Changes)
		}
	}
	api.Directories[id][0].ManagedBy = nil
	_, err = groupPlan(t, api, []resources.IdentityGroupResource{d}, PlanModeApply, true)
	require.ErrorContains(t, err, "adopt")
	plan, err = groupPlan(t, api, nil, PlanModeSync, true)
	require.NoError(t, err)
	require.Empty(t, plan.Changes)
}

func TestGroupPlanningRejectsAmbiguityProtectionAndDescendants(t *testing.T) {
	id := fixture.Directory("people", "directory-team", true).ID
	for _, fault := range []string{"duplicate", "protected", "keys", "membership", "importer", "missing-provenance"} {
		t.Run(fault, func(t *testing.T) {
			current := fixture.Group("Fulfillment", "group-team", false)
			api := &fixture.GroupAPI{Directories: map[string][]helpers.IdentityGroup{id: {current}}}
			switch fault {
			case "duplicate":
				other := current
				other.ID = "5078f559-063b-4f10-9098-9e6553b21d9b"
				api.Directories[id] = append(api.Directories[id], other)
			case "protected":
				current.ManagedBy[resources.ChildProtectedKey] = "true"
			case "keys":
				api.Keys = map[string][]string{current.ID: {"key"}}
			case "membership":
				api.Failure = "membership"
			case "importer":
				api.Directories[id][0].ProvisionedBy = new(kkComps.ProvisionedByDeveloperPortal)
			case "missing-provenance":
				api.Directories[id][0].ProvisionedBy = nil
			}
			_, err := groupPlan(t, api, []resources.IdentityGroupResource{groupDesired()}, PlanModeDelete, true)
			require.Error(t, err)
			require.Zero(t, api.Deletes)
		})
	}
}

func TestNewGroupDependsOnDirectoryAcrossNamespaces(t *testing.T) {
	d := directoryResource()
	d.Kongctl.Namespace = new("z-directory-team")
	g := groupDesired()
	g.Directory = d.Ref
	g.Kongctl.Namespace = new("a-group-team")
	rs := &resources.ResourceSet{
		Identity: &resources.IdentityResource{
			Directories: []resources.IdentityDirectoryResource{d},
			Groups:      []resources.IdentityGroupResource{g},
		},
	}
	plan, err := NewPlanner(state.NewClient(state.ClientConfig{
		IdentityDirectoriesAPI: &fixture.DirectoryAPI{},
		IdentityGroupsAPI:      &fixture.GroupAPI{},
	}), slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 2)
	var parent, child PlannedChange
	for _, c := range plan.Changes {
		if c.ResourceType == ResourceTypeIdentityDirectory {
			parent = c
		} else {
			child = c
		}
	}
	require.Equal(t, ResourceTypeIdentityGroup, child.ResourceType)
	require.Contains(t, child.DependsOn, parent.ID)
	require.Equal(t, parent.ResourceRef, child.Parent.Ref)
}

func TestExternalGroupNameLookupProvesDirectoryScopeWithoutPruning(t *testing.T) {
	id := fixture.Directory("people", "directory-team", true).ID
	current := fixture.Group("Fulfillment", "group-team", false)
	api := &fixture.GroupAPI{Directories: map[string][]helpers.IdentityGroup{id: {current}}}
	group := resources.IdentityGroupResource{
		BaseResource: resources.BaseResource{Ref: "existing"}, Directory: "people",
		External: &resources.ExternalBlock{
			Selector: &resources.ExternalSelector{MatchFields: map[string]string{FieldName: current.Name}},
		},
	}
	plan, err := groupPlan(t, api, []resources.IdentityGroupResource{group}, PlanModeSync, false)
	require.NoError(t, err)
	require.Empty(t, plan.Changes)
	group.External = &resources.ExternalBlock{ID: "5078f559-063b-4f10-9098-9e6553b21d9b"}
	_, err = groupPlan(t, api, []resources.IdentityGroupResource{group}, PlanModeSync, false)
	require.ErrorContains(t, err, "selected directory")
}
