package planner

import (
	"context"
	"errors"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/require"
)

func TestOrganizationTeamRootLifecycleIdentityAndSync(t *testing.T) {
	for _, scoped := range []bool{false, true} {
		t.Run(map[bool]string{false: "assignments only", true: "roots scoped"}[scoped], func(t *testing.T) {
			p := teamRootLifecyclePlanner(
				nil,
				teamRootObservation("first", "team", false),
				teamRootObservation("last", "team", false),
				teamRootObservation("unnamed", "", false),
				teamRootObservation("stale", "stale", false),
			)
			desired := teamRootDesired("team", false)
			desired.SetKonnectID("stale")
			desired.Description = new("changed")
			scope := resources.NewSyncScope()
			if scoped {
				scope.AddRoot(resources.ResourceTypeOrganizationTeam)
			} else {
				scope.OrganizationUsersScoped = true
			}
			p.resources = &resources.ResourceSet{
				OrganizationTeams: []resources.OrganizationTeamResource{desired}, SyncScope: scope,
			}
			plan := NewPlan(CurrentPlanVersion, "test", PlanModeSync)
			err := NewOrganizationTeamPlanner(NewBasePlanner(p)).PlanChanges(t.Context(), NewConfig("default"), plan)
			require.NoError(t, err)
			wantCount := 1
			if scoped {
				wantCount++
			}
			require.Len(t, plan.Changes, wantCount)
			require.Equal(t, ActionUpdate, plan.Changes[0].Action)
			require.Equal(t, "last", plan.Changes[0].ResourceID, "last nonempty name match wins over bound ID")
			if scoped {
				require.Equal(t, ActionDelete, plan.Changes[1].Action)
				require.Equal(t, "stale", plan.Changes[1].ResourceID)
			}
			require.Empty(t, p.matchedIdentities, "team lifecycle must not register payload identities")
			require.Equal(t, "stale", p.resources.OrganizationTeams[0].GetKonnectID())
		})
	}
}

func TestOrganizationTeamRootLifecycleRolesAfterBlockedUpdate(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "continue and prune", true: "assignment error wins"}[fail], func(t *testing.T) {
			plan := NewPlan(CurrentPlanVersion, "test", PlanModeSync)
			failure := errors.New("role observation failed")
			calls := 0
			roles := &organizationTeamRolesAPIStub{listTeamRoles: func(
				_ context.Context, id string, _ *kkOps.ListTeamRolesQueryParamFilter, _ ...kkOps.Option,
			) (*kkOps.ListTeamRolesResponse, error) {
				calls++
				require.Equal(t, "protected", id)
				require.Len(t, plan.Changes, 1, "later root creates precede role reads despite blocked updates")
				require.Equal(t, ActionCreate, plan.Changes[0].Action)
				if fail {
					return nil, failure
				}
				return &kkOps.ListTeamRolesResponse{AssignedRoleCollection: assignedRoleCollection()}, nil
			}}
			p := teamRootLifecyclePlanner(roles,
				teamRootObservation("protected", "team", true), teamRootObservation("stale", "stale", false))
			desired := teamRootDesired("team", true)
			desired.Description = new("blocked")
			p.resources = &resources.ResourceSet{
				OrganizationTeams:     []resources.OrganizationTeamResource{desired, teamRootDesired("new", false)},
				OrganizationTeamRoles: []resources.OrganizationTeamRoleResource{teamRootRole()},
			}
			err := NewOrganizationTeamPlanner(NewBasePlanner(p)).PlanChanges(t.Context(), NewConfig("default"), plan)
			require.Equal(t, 1, calls)
			if fail {
				require.ErrorIs(t, err, failure)
				require.Len(t, plan.Changes, 1, "assignment failure stops pruning")
			} else {
				require.ErrorContains(t, err, `organization_team "team" is protected and cannot be updated`)
				require.Len(t, plan.Changes, 3)
				require.Equal(t, ResourceTypeOrganizationTeamRole, plan.Changes[1].ResourceType)
				require.Equal(t, ActionCreate, plan.Changes[1].Action)
				require.Equal(t, "stale", plan.Changes[2].ResourceID)
				require.Equal(t, ActionDelete, plan.Changes[2].Action)
			}
		})
	}
}

func TestOrganizationTeamRootLifecycleExplicitDelete(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "roles before team", true: "role failure stops team"}[fail], func(t *testing.T) {
			failure := errors.New("role observation failed")
			calls := 0
			roles := &organizationTeamRolesAPIStub{listTeamRoles: func(
				_ context.Context, id string, _ *kkOps.ListTeamRolesQueryParamFilter, _ ...kkOps.Option,
			) (*kkOps.ListTeamRolesResponse, error) {
				calls++
				require.Equal(t, "team-id", id, "protected teams must not read or delete roles")
				if fail {
					return nil, failure
				}
				return &kkOps.ListTeamRolesResponse{AssignedRoleCollection: assignedRoleCollection(
					assignedRole("role-id", "Admin", "*", "APIs"),
				)}, nil
			}}
			p := teamRootLifecyclePlanner(roles,
				teamRootObservation("protected", "protected", true), teamRootObservation("team-id", "team", false))
			protectedRole := teamRootRole()
			protectedRole.Team = "protected"
			p.resources = &resources.ResourceSet{
				OrganizationTeams: []resources.OrganizationTeamResource{
					teamRootDesired("protected", true), teamRootDesired("missing", false), teamRootDesired("team", false),
				},
				OrganizationTeamRoles: []resources.OrganizationTeamRoleResource{protectedRole, teamRootRole()},
			}
			plan := NewPlan(CurrentPlanVersion, "test", PlanModeDelete)
			err := NewOrganizationTeamPlanner(NewBasePlanner(p)).PlanChanges(t.Context(), NewConfig("default"), plan)
			require.Equal(t, 1, calls)
			require.Len(t, plan.Warnings, 1)
			require.Equal(t, `organization_team "missing" not found in Konnect, skipping delete`, plan.Warnings[0].Message)
			if fail {
				require.ErrorIs(t, err, failure)
				require.Empty(t, plan.Changes)
			} else {
				require.ErrorContains(t, err, `organization_team "protected" is protected and cannot be deleted`)
				require.Len(t, plan.Changes, 2)
				require.Equal(t, "role-id", plan.Changes[0].ResourceID)
				require.Equal(t, "team-id", plan.Changes[1].ResourceID)
				for _, change := range plan.Changes {
					require.Equal(t, ActionDelete, change.Action)
				}
			}
		})
	}
}

func TestOrganizationTeamRootLifecycleProtectionTransitions(t *testing.T) {
	for _, tt := range []struct {
		name                       string
		old, next, update, blocked bool
	}{
		{"protect", false, true, false, false},
		{"protect with update", false, true, true, false},
		{"unprotect", true, false, false, false},
		{"unprotect with update", true, false, true, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p := teamRootLifecyclePlanner(nil, teamRootObservation("id", "team", tt.old))
			desired := teamRootDesired("team", tt.next)
			if tt.update {
				desired.Description = new("changed")
			}
			p.resources = &resources.ResourceSet{OrganizationTeams: []resources.OrganizationTeamResource{desired}}
			plan := NewPlan(CurrentPlanVersion, "test", PlanModeApply)
			err := NewOrganizationTeamPlanner(NewBasePlanner(p)).PlanChanges(t.Context(), NewConfig("default"), plan)
			if tt.blocked {
				require.ErrorContains(t, err, `organization_team "team" is protected`)
				require.Empty(t, plan.Changes)
				return
			}
			require.NoError(t, err)
			require.Len(t, plan.Changes, 1)
			require.Equal(t, ActionUpdate, plan.Changes[0].Action)
			require.Equal(t, "id", plan.Changes[0].ResourceID)
			require.Equal(t, ProtectionChange{Old: tt.old, New: tt.next}, plan.Changes[0].Protection)
			if tt.update {
				// Preserve the existing builder's protection-only payload when protecting.
				require.NotContains(t, plan.Changes[0].Fields, FieldDescription)
				require.Equal(t, "changed", plan.Changes[0].ChangedFields[FieldDescription].New)
			}
		})
	}
}

func teamRootLifecyclePlanner(roles *organizationTeamRolesAPIStub, observed ...state.OrganizationTeam) *Planner {
	if roles == nil {
		roles = &organizationTeamRolesAPIStub{}
	}
	p := NewPlanner(state.NewClient(state.ClientConfig{OrganizationTeamRolesAPI: roles}), discardPlannerLogger())
	p.resourceCache.managedOrganizationTeams.byNamespaces[namespaceCacheKey([]string{"default"})] = observed
	p.matchedIdentities = make(map[string]matchedResourceIdentity)
	return p
}

func teamRootDesired(name string, protected bool) resources.OrganizationTeamResource {
	return resources.OrganizationTeamResource{
		BaseResource: resources.BaseResource{Ref: name, Kongctl: &resources.KongctlMeta{Protected: new(protected)}},
		CreateTeam:   kkComps.CreateTeam{Name: name},
	}
}

func teamRootObservation(id, name string, protected bool) state.OrganizationTeam {
	return state.OrganizationTeam{
		TeamResponse: kkComps.TeamResponse{ID: id, Name: name},
		NormalizedLabels: map[string]string{
			labels.NamespaceKey: "default",
			labels.ProtectedKey: map[bool]string{true: "true", false: "false"}[protected],
		},
	}
}

func teamRootRole() resources.OrganizationTeamRoleResource {
	return resources.OrganizationTeamRoleResource{
		Ref: "role", Team: "team", RoleName: "Admin", EntityID: "*", EntityTypeName: "APIs", EntityRegion: "us",
	}
}
