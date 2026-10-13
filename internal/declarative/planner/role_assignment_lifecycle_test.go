package planner

import (
	"context"
	"fmt"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/require"
)

func TestRoleAssignmentLifecycleAdapters(t *testing.T) {
	for _, kind := range []resources.ResourceType{
		resources.ResourceTypeOrganizationUserRole,
		resources.ResourceTypeOrganizationSystemAccountRole,
		resources.ResourceTypeOrganizationTeamRole,
		resources.ResourceTypePortalTeamRole,
	} {
		for _, scenario := range []struct {
			name, duplicate string
			mode            PlanMode
		}{
			{"apply", "", PlanModeApply},
			{"sync", "", PlanModeSync},
			{"duplicate existing", "Viewer", PlanModeSync},
			{"duplicate new", "Editor", PlanModeSync},
		} {
			t.Run(string(kind)+"/"+scenario.name, func(t *testing.T) {
				names := []string{"Editor", "Viewer"}
				if scenario.duplicate != "" {
					names = append(names, scenario.duplicate)
				}
				names = append(names, "Owner")
				p, run := roleAssignmentLifecycleFixture(t, kind, names)
				plan := NewPlan(CurrentPlanVersion, "test", scenario.mode)
				err := run(plan)
				if scenario.duplicate != "" {
					prefix := map[resources.ResourceType]string{
						resources.ResourceTypeOrganizationUserRole:          "organization user role assignment",
						resources.ResourceTypeOrganizationSystemAccountRole: "organization system account role assignment",
						resources.ResourceTypeOrganizationTeamRole:          "organization team role assignment",
						resources.ResourceTypePortalTeamRole:                "portal team role assignment",
					}[kind]
					owner := map[resources.ResourceType]string{
						resources.ResourceTypeOrganizationUserRole:          "user",
						resources.ResourceTypeOrganizationSystemAccountRole: "system account",
						resources.ResourceTypeOrganizationTeamRole:          "team",
						resources.ResourceTypePortalTeamRole:                "team",
					}[kind]
					require.EqualError(t, err, fmt.Sprintf("duplicate %s %q for %s %q",
						prefix, scenario.duplicate+"|*|APIs|us", owner, "owner"))
					require.Len(t, plan.Changes, 1, "retain earlier creates; stop before later creates and pruning")
				} else {
					require.NoError(t, err)
					count := 2
					if scenario.mode == PlanModeSync {
						count++
					}
					require.Len(t, plan.Changes, count)
					require.Equal(t, ActionCreate, plan.Changes[1].Action)
					require.Equal(t, "Owner", plan.Changes[1].Fields[FieldRoleName])
					if scenario.mode == PlanModeSync {
						require.Equal(t, ActionDelete, plan.Changes[2].Action)
						require.Equal(t, "stale-last", plan.Changes[2].ResourceID,
							"prune the last indexed duplicate, not every observation")
					}
				}
				require.Equal(t, ActionCreate, plan.Changes[0].Action)
				require.Equal(t, "Editor", plan.Changes[0].Fields[FieldRoleName])
				for _, change := range plan.Changes {
					require.Equal(t, string(kind), change.ResourceType)
				}
				require.Empty(t, p.matchedIdentities, "set reconciliation must not bind payload identities")
			})
		}
	}
}

func roleAssignmentLifecycleFixture(
	t *testing.T, kind resources.ResourceType, names []string,
) (*Planner, func(*Plan) error) {
	t.Helper()
	observed := assignedRoleCollection(
		assignedRole("keep", "Viewer", "*", "APIs"),
		assignedRole("stale-first", "Admin", "*", "APIs"),
		assignedRole("stale-last", "Admin", "*", "APIs"),
	)
	client := state.NewClient(state.ClientConfig{
		OrganizationTeamRolesAPI: &organizationTeamRolesAPIStub{
			listTeamRoles: func(_ context.Context, id string, _ *kkOps.ListTeamRolesQueryParamFilter,
				_ ...kkOps.Option,
			) (*kkOps.ListTeamRolesResponse, error) {
				require.Equal(t, "owner-id", id)
				return &kkOps.ListTeamRolesResponse{AssignedRoleCollection: observed}, nil
			},
			listUserRoles: func(_ context.Context, id string, _ *kkOps.ListUserRolesQueryParamFilter,
				_ ...kkOps.Option,
			) (*kkOps.ListUserRolesResponse, error) {
				require.Equal(t, "owner-id", id)
				return &kkOps.ListUserRolesResponse{AssignedRoleCollection: observed}, nil
			},
		},
		SystemAccountRolesAPI: &systemAccountRolesAPIStub{
			listSystemAccountRoles: func(_ context.Context, id string,
				_ *kkOps.GetSystemAccountsAccountIDAssignedRolesQueryParamFilter,
				_ ...kkOps.Option,
			) (*kkOps.GetSystemAccountsAccountIDAssignedRolesResponse, error) {
				require.Equal(t, "owner-id", id)
				return &kkOps.GetSystemAccountsAccountIDAssignedRolesResponse{AssignedRoleCollection: observed}, nil
			},
		},
		PortalTeamRolesAPI: &stubExternalPortalTeamRolesAPI{
			listPortalTeamRolesFn: func(id string) []kkComps.PortalAssignedRoleResponse {
				require.Equal(t, "owner-id", id)
				var roles []kkComps.PortalAssignedRoleResponse
				for _, role := range observed.Data {
					roles = append(roles, kkComps.PortalAssignedRoleResponse{
						ID: *role.ID, RoleName: *role.RoleName, EntityID: *role.EntityID,
						EntityTypeName: *role.EntityTypeName, EntityRegion: kkComps.EntityRegion(*role.EntityRegion),
					})
				}
				return roles
			},
		},
	})
	p := NewPlanner(client, discardPlannerLogger())
	p.matchedIdentities = make(map[string]matchedResourceIdentity)
	user := resources.OrganizationUserResource{Ref: "owner", Email: "owner@example.com"}
	user.SetKonnectID("owner-id")
	account := resources.OrganizationSystemAccountResource{Ref: "owner", Name: "Owner"}
	account.SetKonnectID("owner-id")
	team := teamRootDesired("owner", false)
	team.SetKonnectID("owner-id")
	p.resources = &resources.ResourceSet{
		Organization: &resources.OrganizationResource{
			Users:          []resources.OrganizationUserResource{user},
			SystemAccounts: []resources.OrganizationSystemAccountResource{account},
		},
		OrganizationTeams: []resources.OrganizationTeamResource{team},
	}
	p.desiredPortalTeams = []resources.PortalTeamResource{{
		Ref: "owner", Portal: "portal", PortalCreateTeamRequest: kkComps.PortalCreateTeamRequest{Name: "owner"},
	}}
	p.resourceCache.portalTeamsByPortalID["portal-id"] = []state.PortalTeam{{
		ID: "owner-id", Name: "owner",
	}}
	for i, name := range names {
		ref := fmt.Sprintf("role-%d", i)
		switch kind { //nolint:exhaustive // This fixture covers only the four role assignment adapters.
		case resources.ResourceTypeOrganizationUserRole:
			p.resources.OrganizationUserRoles = append(p.resources.OrganizationUserRoles,
				resources.OrganizationUserRoleResource{
					Ref: ref, User: "owner", RoleName: name,
					EntityID: "*", EntityTypeName: "APIs", EntityRegion: "US",
				})
		case resources.ResourceTypeOrganizationSystemAccountRole:
			p.resources.OrganizationSystemAccountRoles = append(p.resources.OrganizationSystemAccountRoles,
				resources.OrganizationSystemAccountRoleResource{
					Ref: ref, SystemAccount: "owner", RoleName: name,
					EntityID: "*", EntityTypeName: "APIs", EntityRegion: "US",
				})
		case resources.ResourceTypeOrganizationTeamRole:
			p.resources.OrganizationTeamRoles = append(p.resources.OrganizationTeamRoles,
				resources.OrganizationTeamRoleResource{
					Ref: ref, Team: "owner", RoleName: name,
					EntityID: "*", EntityTypeName: "APIs", EntityRegion: "US",
				})
		case resources.ResourceTypePortalTeamRole:
			p.desiredPortalTeamRoles = append(p.desiredPortalTeamRoles,
				resources.PortalTeamRoleResource{
					Ref: ref, Portal: "portal", Team: "owner", RoleName: name,
					EntityID: "*", EntityTypeName: "APIs", EntityRegion: "US",
				})
		default:
			t.Fatalf("unsupported role assignment fixture %s", kind)
		}
	}
	org := NewOrganizationTeamPlanner(NewBasePlanner(p)).(*OrganizationTeamPlannerImpl)
	return p, func(plan *Plan) error {
		switch kind { //nolint:exhaustive // This fixture covers only the four role assignment adapters.
		case resources.ResourceTypeOrganizationUserRole:
			return org.planOrganizationUserRoleChanges(t.Context(), "default", plan)
		case resources.ResourceTypeOrganizationSystemAccountRole:
			return org.planOrganizationSystemAccountRoleChanges(t.Context(), "default", plan)
		case resources.ResourceTypeOrganizationTeamRole:
			return org.planOrganizationTeamRoleChanges(t.Context(), "default", p.resources.OrganizationTeams,
				map[string]state.OrganizationTeam{}, plan)
		case resources.ResourceTypePortalTeamRole:
			return p.planPortalTeamRolesChanges(t.Context(), "default", "portal-id", "portal", "Portal", plan)
		default:
			return fmt.Errorf("unsupported role assignment fixture %s", kind)
		}
	}
}
