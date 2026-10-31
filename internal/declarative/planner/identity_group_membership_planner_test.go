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

func membershipPlanFixture() (*Planner, *resources.ResourceSet, *fixture.GroupAPI) {
	directory := fixture.Directory("people", "directory-team", true)
	group := fixture.Group("Operators", "group-team", false)
	api := &fixture.GroupAPI{Directories: map[string][]helpers.IdentityGroup{directory.ID: {group}}}
	principal := kkComps.KongPrincipal{
		ID:        "5078f559-063b-4f10-9098-9e6553b21d9b",
		ManagedBy: resources.BuildPrincipalManagedBy("principal-team", "alice", false, "old"),
	}
	client := state.NewClient(state.ClientConfig{
		IdentityDirectoriesAPI: &fixture.DirectoryAPI{Directories: []kkComps.KongDirectory{directory}},
		IdentityGroupsAPI:      api,
		IdentityPrincipalsAPI: &fixture.PrincipalAPI{
			Directories: map[string][]kkComps.KongPrincipal{directory.ID: {principal}},
		},
		IdentityPrincipalIdentitiesAPI: &fixture.PrincipalIdentitiesAPI{},
	})
	rs := &resources.ResourceSet{
		DefaultNamespace: "group-team",
		SyncScope:        resources.NewSyncScope(),
		Identity: &resources.IdentityResource{
			Directories: []resources.IdentityDirectoryResource{
				{
					BaseResource: resources.BaseResource{Ref: "people"},
					External:     &resources.ExternalBlock{ID: directory.ID},
				},
			},
			Groups: []resources.IdentityGroupResource{
				{
					BaseResource: resources.BaseResource{
						Ref:     "operators",
						Kongctl: &resources.KongctlMeta{Namespace: new("group-team")},
					},
					Directory: "people",
					Name:      "Operators",
				},
			},
			Principals: []resources.IdentityPrincipalResource{
				{
					BaseResource: resources.BaseResource{
						Ref:     "alice",
						Kongctl: &resources.KongctlMeta{Namespace: new("principal-team")},
					},
					Directory: "people",
				},
			},
			Memberships: []resources.IdentityGroupMembershipResource{
				{
					Ref:    "alice-member",
					Group:  "operators",
					Member: resources.IdentityGroupMemberResource{Type: "principal", ID: "__REF__:alice#id"},
				},
			},
		},
	}
	rs.SyncScope.AddRoot(resources.ResourceTypeIdentityDirectory)
	return NewPlanner(client, slog.Default()), rs, api
}

func TestMembershipLogicalIdentityAndSyncScope(t *testing.T) {
	p, rs, api := membershipPlanFixture()
	plan, err := p.GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	change := plan.Changes[0]
	require.Equal(t, ResourceTypeIdentityGroupMembership, change.ResourceType)
	require.Equal(t, "group-team", change.Namespace)
	require.Equal(t, "Operators", change.ProtectingParent.ResourceName)
	require.Equal(t, ResourceTypeIdentityGroup, change.ProtectingParent.ResourceType)
	groupID := change.Parent.ID
	entry := kkComps.KongGroupMembershipEntry{
		ID: "6078f559-063b-4f10-9098-9e6553b21d9b",
		Member: kkComps.KongGroupMember{
			Type: kkComps.KongGroupMembershipTypePrincipal, ID: "5078f559-063b-4f10-9098-9e6553b21d9b",
		},
	}
	api.Memberships = map[string][]kkComps.KongGroupMembershipEntry{groupID: {entry}}
	plan, err = p.GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
	require.NoError(t, err)
	require.Empty(t, plan.Changes)
	rs.Identity.Memberships = nil
	plan, err = p.GeneratePlan(t.Context(), rs, Options{Mode: PlanModeSync})
	require.NoError(t, err)
	require.Empty(t, plan.Changes)
	rs.SyncScope.AddChild(
		resources.ResourceTypeIdentityGroup,
		"operators",
		resources.ResourceTypeIdentityGroupMembership,
	)
	plan, err = p.GeneratePlan(t.Context(), rs, Options{Mode: PlanModeSync})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 1)
	require.Equal(t, ActionDelete, plan.Changes[0].Action)
	require.Equal(t, entry.ID, plan.Changes[0].ResourceID)
}

func TestMembershipDetachBeforeGroupAndPrincipalDeletion(t *testing.T) {
	p, rs, api := membershipPlanFixture()
	group := api.Directories[rs.Identity.Directories[0].External.ID][0]
	api.Memberships = map[string][]kkComps.KongGroupMembershipEntry{group.ID: {{
		ID: "6078f559-063b-4f10-9098-9e6553b21d9b",
		Member: kkComps.KongGroupMember{
			Type: kkComps.KongGroupMembershipTypePrincipal, ID: "5078f559-063b-4f10-9098-9e6553b21d9b",
		},
	}}}
	plan, err := p.GeneratePlan(t.Context(), rs, Options{Mode: PlanModeDelete})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 3)
	var detachment PlannedChange
	for _, c := range plan.Changes {
		if c.ResourceType == ResourceTypeIdentityGroupMembership {
			detachment = c
		}
	}
	require.NotEmpty(t, detachment.ID)
	for _, c := range plan.Changes {
		if c.ResourceType == ResourceTypeIdentityGroup || c.ResourceType == ResourceTypeIdentityPrincipal {
			require.Contains(t, c.DependsOn, detachment.ID)
		}
	}
	rs.Identity.Memberships = nil
	_, err = p.GeneratePlan(t.Context(), rs, Options{Mode: PlanModeDelete})
	require.ErrorContains(t, err, "memberships")
}

func TestMembershipRejectsForeignProtectedAndCrossDirectoryTargets(t *testing.T) {
	for _, fault := range []string{
		"foreign-group", "protected-removal", "missing-principal", "duplicate", "failed-observation",
	} {
		t.Run(fault, func(t *testing.T) {
			p, rs, api := membershipPlanFixture()
			directoryID := rs.Identity.Directories[0].External.ID
			current := &api.Directories[directoryID][0]
			mode := PlanModeApply
			switch fault {
			case "foreign-group":
				current.ManagedBy[resources.ChildNamespaceKey] = "foreign"
			case "protected-removal":
				current.ManagedBy[resources.ChildProtectedKey] = "true"
				api.Memberships = map[string][]kkComps.KongGroupMembershipEntry{
					current.ID: {
						{
							ID: "6078f559-063b-4f10-9098-9e6553b21d9b",
							Member: kkComps.KongGroupMember{
								Type: kkComps.KongGroupMembershipTypePrincipal,
								ID:   "5078f559-063b-4f10-9098-9e6553b21d9b",
							},
						},
					},
				}
				rs.Identity.Groups[0].Kongctl.Protected = new(true)
				rs.Identity.Memberships = nil
				rs.SyncScope.AddChild(
					resources.ResourceTypeIdentityGroup,
					"operators",
					resources.ResourceTypeIdentityGroupMembership,
				)
				mode = PlanModeSync
			case "missing-principal":
				rs.Identity.Memberships[0].Member.ID = "7078f559-063b-4f10-9098-9e6553b21d9b"
			case "duplicate":
				rs.Identity.Memberships = append(
					rs.Identity.Memberships,
					resources.IdentityGroupMembershipResource{
						Ref:    "duplicate",
						Group:  "operators",
						Member: rs.Identity.Memberships[0].Member,
					},
				)
			case "failed-observation":
				api.Failure = "membership"
			}
			_, err := p.GeneratePlan(t.Context(), rs, Options{Mode: mode})
			require.Error(t, err)
		})
	}
}

func TestMembershipCreationDependsOnNewParentsAcrossNamespaces(t *testing.T) {
	_, rs, _ := membershipPlanFixture()
	directory := directoryResource()
	directory.Ref = "people"
	directory.Kongctl.Namespace = new("z-directory")
	rs.Identity.Directories = []resources.IdentityDirectoryResource{directory}
	rs.Identity.Groups[0].Kongctl.Namespace = new("a-group")
	rs.Identity.Principals[0].Kongctl.Namespace = new("y-principal")
	client := state.NewClient(state.ClientConfig{
		IdentityDirectoriesAPI: &fixture.DirectoryAPI{}, IdentityGroupsAPI: &fixture.GroupAPI{},
		IdentityPrincipalsAPI: &fixture.PrincipalAPI{},
	})
	plan, err := NewPlanner(client, slog.Default()).GeneratePlan(t.Context(), rs, Options{Mode: PlanModeApply})
	require.NoError(t, err)
	require.Len(t, plan.Changes, 4)
	var membership PlannedChange
	for _, change := range plan.Changes {
		if change.ResourceType == ResourceTypeIdentityGroupMembership {
			membership = change
		}
	}
	require.Len(t, membership.DependsOn, 3)
	require.Len(t, membership.PayloadReferences, 1)
	require.Equal(t, "/member/id", membership.PayloadReferences[0].Path)
	require.Equal(t, ResourceTypeIdentityPrincipal, membership.PayloadReferences[0].ResourceType)
}
