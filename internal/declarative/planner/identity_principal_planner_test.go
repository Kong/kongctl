package planner

import (
	"fmt"
	"log/slog"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	fixture "github.com/kong/kongctl/test/konnect"
	"github.com/stretchr/testify/require"
)

func principalPlan(t *testing.T, api *fixture.PrincipalAPI, desired []resources.IdentityPrincipalResource,
	mode PlanMode, scoped bool,
	identities ...*fixture.PrincipalIdentitiesAPI,
) (*Plan, error) {
	t.Helper()
	identityAPI := &fixture.PrincipalIdentitiesAPI{}
	if len(identities) != 0 {
		identityAPI = identities[0]
	}
	directory := fixture.Directory("people", "directory-team", false)
	rs := &resources.ResourceSet{DefaultNamespace: "principal-team", Identity: &resources.IdentityResource{
		Directories: []resources.IdentityDirectoryResource{
			{BaseResource: resources.BaseResource{Ref: "people"}, External: &resources.ExternalBlock{ID: directory.ID}},
		}, Principals: desired,
	}, SyncScope: resources.NewSyncScope()}
	rs.SyncScope.AddRoot(resources.ResourceTypeIdentityDirectory)
	rs.SyncScope.AddChild(resources.ResourceTypeIdentityDirectory, "people", resources.ResourceTypeIdentityPrincipal)
	if scoped {
		rs.SyncScope.AddChildNamespace(resources.ResourceTypeIdentityPrincipal, "people", "principal-team")
	}
	return NewPlanner(
		state.NewClient(
			state.ClientConfig{
				IdentityDirectoriesAPI:         &fixture.DirectoryAPI{Directories: []kkComps.KongDirectory{directory}},
				IdentityPrincipalsAPI:          api,
				IdentityPrincipalIdentitiesAPI: identityAPI,
			},
		),
		slog.Default(),
	).GeneratePlan(t.Context(), rs, Options{Mode: mode})
}

func principalDesired() resources.IdentityPrincipalResource {
	return resources.IdentityPrincipalResource{
		BaseResource: resources.BaseResource{
			Ref:     "alice",
			Kongctl: &resources.KongctlMeta{Namespace: new("principal-team")},
		},
		Directory:   "people",
		DisplayName: new("Shared name"),
		Type:        new("user"),
	}
}

func principalObserved(ref, namespace string) kkComps.KongPrincipal {
	return kkComps.KongPrincipal{
		ID:          "1248f559-063b-4f10-9098-9e6553b21d9b",
		DisplayName: new("Shared name"),
		Type:        new(kkComps.KongPrincipalTypeUser),
		ManagedBy:   resources.BuildPrincipalManagedBy(namespace, ref, false, "old"),
	}
}

func TestPrincipalMatchingAndSyncAuthority(t *testing.T) {
	id := fixture.Directory("people", "directory-team", false).ID
	api := &fixture.PrincipalAPI{
		Directories: map[string][]kkComps.KongPrincipal{id: {principalObserved("alice", "principal-team")}},
	}
	d := principalDesired()
	plan, err := principalPlan(t, api, []resources.IdentityPrincipalResource{d}, PlanModeApply, true)
	require.NoError(t, err)
	require.Empty(t, plan.Changes)
	d.DisplayName = new("Renamed")
	plan, err = principalPlan(t, api, []resources.IdentityPrincipalResource{d}, PlanModeApply, true)
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	require.Equal(t, ActionUpdate, plan.Changes[0].Action)
	require.Equal(t, id, plan.Changes[0].Parent.ID)
	for _, scoped := range []bool{false, true} {
		plan, err = principalPlan(t, api, nil, PlanModeSync, scoped)
		require.NoError(t, err)
		if scoped {
			require.Len(t, plan.Changes, 1)
			require.Equal(t, ActionDelete, plan.Changes[0].Action)
		} else {
			require.Empty(t, plan.Changes)
		}
	}
	foreign := principalObserved("other", "other-team")
	foreign.ID = "other-id"
	unowned := principalObserved("jit", "principal-team")
	unowned.ID = "jit-id"
	unowned.ManagedBy = nil
	api.Directories[id] = append(api.Directories[id], foreign, unowned)
	plan, err = principalPlan(t, api, nil, PlanModeSync, true)
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	require.Equal(t, "alice", plan.Changes[0].ResourceRef)
	duplicate := api.Directories[id][0]
	duplicate.ID = "duplicate"
	api.Directories[id] = append(api.Directories[id], duplicate)
	_, err = principalPlan(t, api, []resources.IdentityPrincipalResource{principalDesired()}, PlanModeApply, true)
	require.ErrorContains(t, err, "multiple principals match")
}

func TestPrincipalTypeCompatibilityAndProtection(t *testing.T) {
	id := fixture.Directory("people", "directory-team", false).ID
	api := &fixture.PrincipalAPI{
		Directories: map[string][]kkComps.KongPrincipal{id: {principalObserved("alice", "principal-team")}},
	}
	for _, next := range []*string{new("agent"), nil} {
		d := principalDesired()
		d.Type = next
		if next == nil {
			d.DeclareUnclassified()
		}
		_, err := principalPlan(t, api, []resources.IdentityPrincipalResource{d}, PlanModeApply, true)
		require.ErrorContains(t, err, "Q9")
	}
	d := principalDesired()
	d.Type = nil
	plan, err := principalPlan(t, api, []resources.IdentityPrincipalResource{d}, PlanModeApply, true)
	require.NoError(t, err)
	require.Empty(t, plan.Changes)
	api.Directories[id][0].ManagedBy[resources.ChildProtectedKey] = "true"
	_, err = principalPlan(t, api, nil, PlanModeSync, true)
	require.ErrorContains(t, err, "protected")
	plan, err = principalPlan(t, api, []resources.IdentityPrincipalResource{d}, PlanModeApply, true)
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	require.Contains(t, plan.Changes[0].ChangedFields, FieldManagedBy)
}

func TestPrincipalParentCreationAcrossNamespaces(t *testing.T) {
	d := directoryResource()
	d.Kongctl.Namespace = new("z-directory-team")
	p := principalDesired()
	p.Directory = d.Ref
	p.Kongctl.Namespace = new("a-principal-team")
	rs := &resources.ResourceSet{Identity: &resources.IdentityResource{
		Directories: []resources.IdentityDirectoryResource{d}, Principals: []resources.IdentityPrincipalResource{p},
	}}
	plan, err := NewPlanner(state.NewClient(state.ClientConfig{
		IdentityDirectoriesAPI: &fixture.DirectoryAPI{},
		IdentityPrincipalsAPI:  &fixture.PrincipalAPI{Directories: make(map[string][]kkComps.KongPrincipal)},
	}), slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 2)
	var parent, child PlannedChange
	for _, change := range plan.Changes {
		if change.ResourceType == ResourceTypeIdentityDirectory {
			parent = change
		} else {
			child = change
		}
	}
	require.Contains(t, child.DependsOn, parent.ID)
	require.Equal(t, d.Ref, child.Parent.Ref)
	require.Empty(t, child.Parent.ID)
}

func TestPrincipalEmptyScopeSelectsNamespaceWithoutPruningDirectories(t *testing.T) {
	directory := fixture.Directory("people", "principal-team", false)
	other := fixture.Directory("unrelated", "principal-team", false)
	other.ID = "2248f559-063b-4f10-9098-9e6553b21d9b"
	rs := &resources.ResourceSet{Identity: &resources.IdentityResource{
		Directories: []resources.IdentityDirectoryResource{{
			BaseResource: resources.BaseResource{Ref: "people"},
			External:     &resources.ExternalBlock{ID: directory.ID},
		}},
	}, SyncScope: resources.NewSyncScope()}
	rs.SyncScope.AddRoot(resources.ResourceTypeIdentityDirectory)
	rs.SyncScope.AddChild(resources.ResourceTypeIdentityDirectory, "people", resources.ResourceTypeIdentityPrincipal)
	rs.SyncScope.AddChildNamespace(resources.ResourceTypeIdentityPrincipal, "people", "principal-team")
	api := &fixture.PrincipalAPI{Directories: map[string][]kkComps.KongPrincipal{
		directory.ID: {principalObserved("alice", "principal-team")},
	}}
	plan, err := NewPlanner(state.NewClient(state.ClientConfig{
		IdentityDirectoriesAPI:         &fixture.DirectoryAPI{Directories: []kkComps.KongDirectory{directory, other}},
		IdentityPrincipalsAPI:          api,
		IdentityPrincipalIdentitiesAPI: &fixture.PrincipalIdentitiesAPI{},
	}), slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeSync})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	require.Equal(t, ResourceTypeIdentityPrincipal, plan.Changes[0].ResourceType)
	require.Equal(t, ActionDelete, plan.Changes[0].Action)
}

func TestPrincipalDeletionRequiresEmptyIdentityCollection(t *testing.T) {
	for _, mode := range []string{"empty", "owned", "foreign", "unowned", "failed"} {
		t.Run(mode, func(t *testing.T) {
			directory := fixture.Directory("people", "directory-team", false)
			principal := principalObserved("alice", "principal-team")
			api := &fixture.PrincipalAPI{Directories: map[string][]kkComps.KongPrincipal{directory.ID: {principal}}}
			identities := &fixture.PrincipalIdentitiesAPI{}
			if mode == "failed" {
				identities.Error = fmt.Errorf("identity observation failed")
			} else if mode != "empty" {
				metadata := resources.BuildPrincipalManagedBy("principal-team", "employee", true, "test")
				switch mode {
				case "foreign":
					metadata[resources.ChildNamespaceKey] = "different-team"
				case "unowned":
					metadata = nil
				}
				identities.Data = []kkComps.KongPrincipalIdentity{
					kkComps.CreateKongPrincipalIdentityCustom(kkComps.KongPrincipalIdentityCustomResponse{
						ID: "1248f559-063b-4f10-9098-9e6553b21d9c", Key: "employee", Value: "123", ManagedBy: metadata,
					}),
				}
			}
			for _, planMode := range []PlanMode{PlanModeSync, PlanModeDelete} {
				var desired []resources.IdentityPrincipalResource
				// Sync must omit the principal to request pruning.
				if planMode == PlanModeDelete {
					desired = []resources.IdentityPrincipalResource{principalDesired()}
				}
				plan, err := principalPlan(t, api, desired, planMode, true, identities)
				switch mode {
				case "empty":
					require.NoError(t, err)
					require.Len(t, plan.Changes, 1)
				case "failed":
					require.ErrorContains(t, err, "identity observation failed")
				default:
					require.ErrorContains(t, err, "still has 1 identities")
				}
			}
			for _, request := range identities.Requests {
				require.Equal(t, directory.ID, request.DirectoryID)
				require.Equal(t, principal.ID, request.PrincipalID)
			}
			require.Zero(t, api.Deletes)
		})
	}
}
