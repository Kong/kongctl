package planner

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/require"
)

type organizationTeamAPIStub struct {
	listOrganizationTeams func(context.Context, kkOps.ListTeamsRequest) (*kkOps.ListTeamsResponse, error)
}

type organizationTeamRolesAPIStub struct {
	listTeamRoles func(
		context.Context,
		string,
		*kkOps.ListTeamRolesQueryParamFilter,
		...kkOps.Option,
	) (*kkOps.ListTeamRolesResponse, error)
	listUserRoles func(
		context.Context,
		string,
		*kkOps.ListUserRolesQueryParamFilter,
		...kkOps.Option,
	) (*kkOps.ListUserRolesResponse, error)
}

type systemAccountRolesAPIStub struct {
	listSystemAccountRoles func(
		context.Context,
		string,
		*kkOps.GetSystemAccountsAccountIDAssignedRolesQueryParamFilter,
		...kkOps.Option,
	) (*kkOps.GetSystemAccountsAccountIDAssignedRolesResponse, error)
}

func (s *organizationTeamAPIStub) ListOrganizationTeams(
	ctx context.Context,
	req kkOps.ListTeamsRequest,
) (*kkOps.ListTeamsResponse, error) {
	if s.listOrganizationTeams != nil {
		return s.listOrganizationTeams(ctx, req)
	}
	return &kkOps.ListTeamsResponse{
		TeamCollection: &kkComps.TeamCollection{
			Data: []kkComps.Team{},
			Meta: &kkComps.PaginatedMeta{Page: kkComps.PageMeta{Total: 0}},
		},
	}, nil
}

func (s *organizationTeamAPIStub) GetOrganizationTeam(
	_ context.Context,
	_ string,
) (*kkOps.GetTeamResponse, error) {
	return nil, fmt.Errorf("GetOrganizationTeam not implemented")
}

func (s *organizationTeamAPIStub) CreateOrganizationTeam(
	_ context.Context,
	_ *kkComps.CreateTeam,
) (*kkOps.CreateTeamResponse, error) {
	return nil, fmt.Errorf("CreateOrganizationTeam not implemented")
}

func (s *organizationTeamAPIStub) UpdateOrganizationTeam(
	_ context.Context,
	_ string,
	_ *kkComps.UpdateTeam,
) (*kkOps.UpdateTeamResponse, error) {
	return nil, fmt.Errorf("UpdateOrganizationTeam not implemented")
}

func (s *organizationTeamAPIStub) DeleteOrganizationTeam(
	_ context.Context,
	_ string,
) (*kkOps.DeleteTeamResponse, error) {
	return nil, fmt.Errorf("DeleteOrganizationTeam not implemented")
}

func (s *organizationTeamRolesAPIStub) ListTeamRoles(
	ctx context.Context,
	teamID string,
	filter *kkOps.ListTeamRolesQueryParamFilter,
	opts ...kkOps.Option,
) (*kkOps.ListTeamRolesResponse, error) {
	if s.listTeamRoles != nil {
		return s.listTeamRoles(ctx, teamID, filter, opts...)
	}
	return &kkOps.ListTeamRolesResponse{AssignedRoleCollection: assignedRoleCollection()}, nil
}

func (s *organizationTeamRolesAPIStub) ListUserRoles(
	ctx context.Context,
	userID string,
	filter *kkOps.ListUserRolesQueryParamFilter,
	opts ...kkOps.Option,
) (*kkOps.ListUserRolesResponse, error) {
	if s.listUserRoles != nil {
		return s.listUserRoles(ctx, userID, filter, opts...)
	}
	return &kkOps.ListUserRolesResponse{AssignedRoleCollection: assignedRoleCollection()}, nil
}

func (s *organizationTeamRolesAPIStub) TeamsAssignRole(
	_ context.Context,
	_ string,
	_ *kkComps.AssignRole,
	_ ...kkOps.Option,
) (*kkOps.TeamsAssignRoleResponse, error) {
	return nil, fmt.Errorf("TeamsAssignRole not implemented")
}

func (s *organizationTeamRolesAPIStub) TeamsRemoveRole(
	_ context.Context,
	_ string,
	_ string,
	_ ...kkOps.Option,
) (*kkOps.TeamsRemoveRoleResponse, error) {
	return nil, fmt.Errorf("TeamsRemoveRole not implemented")
}

func (s *organizationTeamRolesAPIStub) UsersAssignRole(
	_ context.Context,
	_ string,
	_ *kkComps.AssignRole,
	_ ...kkOps.Option,
) (*kkOps.UsersAssignRoleResponse, error) {
	return nil, fmt.Errorf("UsersAssignRole not implemented")
}

func (s *organizationTeamRolesAPIStub) UsersRemoveRole(
	_ context.Context,
	_ string,
	_ string,
	_ ...kkOps.Option,
) (*kkOps.UsersRemoveRoleResponse, error) {
	return nil, fmt.Errorf("UsersRemoveRole not implemented")
}

func (s *systemAccountRolesAPIStub) ListSystemAccountRoles(
	ctx context.Context,
	accountID string,
	filter *kkOps.GetSystemAccountsAccountIDAssignedRolesQueryParamFilter,
	opts ...kkOps.Option,
) (*kkOps.GetSystemAccountsAccountIDAssignedRolesResponse, error) {
	if s.listSystemAccountRoles != nil {
		return s.listSystemAccountRoles(ctx, accountID, filter, opts...)
	}
	return &kkOps.GetSystemAccountsAccountIDAssignedRolesResponse{
		AssignedRoleCollection: assignedRoleCollection(),
	}, nil
}

func (s *systemAccountRolesAPIStub) AssignSystemAccountRole(
	_ context.Context,
	_ string,
	_ *kkComps.AssignRole,
	_ ...kkOps.Option,
) (*kkOps.PostSystemAccountsAccountIDAssignedRolesResponse, error) {
	return nil, fmt.Errorf("AssignSystemAccountRole not implemented")
}

func (s *systemAccountRolesAPIStub) RemoveSystemAccountRole(
	_ context.Context,
	_ string,
	_ string,
	_ ...kkOps.Option,
) (*kkOps.DeleteSystemAccountsAccountIDAssignedRolesRoleIDResponse, error) {
	return nil, fmt.Errorf("RemoveSystemAccountRole not implemented")
}

func discardPlannerLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func assignedRoleCollection(roles ...kkComps.AssignedRole) *kkComps.AssignedRoleCollection {
	return &kkComps.AssignedRoleCollection{
		Data: roles,
		Meta: &kkComps.PaginatedMeta{Page: kkComps.PageMeta{Total: float64(len(roles))}},
	}
}

func assignedRole(id, roleName, entityID, entityTypeName, entityRegion string) kkComps.AssignedRole {
	region := kkComps.EntityRegion(entityRegion)
	return kkComps.AssignedRole{
		ID:             &id,
		RoleName:       &roleName,
		EntityID:       &entityID,
		EntityTypeName: &entityTypeName,
		EntityRegion:   &region,
	}
}

func TestOrganizationTeamExternalConfigContributesExternalNamespace(t *testing.T) {
	planner := &Planner{}
	rs := &resources.ResourceSet{
		OrganizationTeams: []resources.OrganizationTeamResource{
			{
				BaseResource: resources.BaseResource{Ref: "external-team"},
				CreateTeam:   kkComps.CreateTeam{Name: "External Team"},
				External:     &resources.ExternalBlock{ID: "team-123"},
			},
		},
		OrganizationTeamRoles: []resources.OrganizationTeamRoleResource{
			{
				Ref:            "external-team-admin",
				Team:           "external-team",
				RoleName:       "Admin",
				EntityID:       "*",
				EntityTypeName: "APIs",
				EntityRegion:   "us",
			},
		},
	}

	namespaces := planner.getResourceNamespaces(rs)
	require.Equal(t, []string{resources.NamespaceExternal}, namespaces)
}

func TestOrganizationTeamRoleDeleteUsesUniqueCompositeRef(t *testing.T) {
	planner := NewPlanner(nil, nil)
	teamPlanner := NewOrganizationTeamPlanner(NewBasePlanner(planner)).(*OrganizationTeamPlannerImpl)
	plan := NewPlan("1.0", "test", PlanModeSync)

	teamPlanner.planOrganizationTeamRoleDelete(
		"default",
		"api-admins",
		"API Admins",
		"team-123",
		state.OrganizationTeamRole{
			ID:             "role-1",
			RoleName:       "Admin",
			EntityID:       "api-1",
			EntityTypeName: "APIs",
			EntityRegion:   "us",
		},
		plan,
	)
	teamPlanner.planOrganizationTeamRoleDelete(
		"default",
		"api-admins",
		"API Admins",
		"team-123",
		state.OrganizationTeamRole{
			ID:             "role-2",
			RoleName:       "Admin",
			EntityID:       "api-2",
			EntityTypeName: "APIs",
			EntityRegion:   "us",
		},
		plan,
	)

	require.Len(t, plan.Changes, 2)
	require.NotEqual(t, plan.Changes[0].ResourceRef, plan.Changes[1].ResourceRef)
	require.NotEqual(t, plan.Changes[0].ID, plan.Changes[1].ID)
	require.Equal(t, "api-admins|Admin|api-1|APIs|us", plan.Changes[0].ResourceRef)
	require.Equal(t, "api-admins|Admin|api-2|APIs|us", plan.Changes[1].ResourceRef)
}

func TestOrganizationUserAssignmentPlansCreateChanges(t *testing.T) {
	planner := NewPlanner(nil, nil)
	teamPlanner := NewOrganizationTeamPlanner(NewBasePlanner(planner)).(*OrganizationTeamPlannerImpl)
	plan := NewPlan("1.0", "test", PlanModeApply)

	teamPlanner.planOrganizationUserTeamMembershipCreate(
		"default",
		"alice",
		"user-123",
		"alice-platform-team",
		"platform-team",
		"team-123",
		"Platform Engineering",
		plan,
	)
	teamPlanner.planOrganizationUserRoleCreate(
		"default",
		"alice",
		"user-123",
		resources.OrganizationUserRoleResource{
			Ref:            "alice-products-viewer",
			User:           "alice",
			RoleName:       "Viewer",
			EntityID:       "__REF__:products-api#id",
			EntityTypeName: "APIs",
			EntityRegion:   "us",
		},
		plan,
	)

	require.Len(t, plan.Changes, 2)
	require.Equal(t, ResourceTypeOrganizationUserTeamMembership, plan.Changes[0].ResourceType)
	require.Equal(t, "user-123", plan.Changes[0].References[FieldUserID].ID)
	require.Equal(t, "team-123", plan.Changes[0].References[FieldTeamID].ID)
	require.Equal(t, ResourceTypeOrganizationUserRole, plan.Changes[1].ResourceType)
	require.Equal(t, "alice-products-viewer", plan.Changes[1].ResourceRef)
	require.Equal(t, "__REF__:products-api#id", plan.Changes[1].References[FieldEntityID].Ref)
}

func TestOrganizationTeamRolePortalEntityRefMatchesExistingRole(t *testing.T) {
	const (
		portalID = "portal-123"
		teamID   = "team-123"
	)

	portal := resources.PortalResource{
		BaseResource: resources.BaseResource{Ref: "developer-portal"},
		CreatePortal: kkComps.CreatePortal{
			Name: "Developer Portal",
		},
	}
	portal.SetKonnectID(portalID)
	team := resources.OrganizationTeamResource{
		BaseResource: resources.BaseResource{Ref: "platform-team"},
		CreateTeam:   kkComps.CreateTeam{Name: "Platform Team"},
	}
	team.SetKonnectID(teamID)

	resourceSet := &resources.ResourceSet{
		Portals:           []resources.PortalResource{portal},
		OrganizationTeams: []resources.OrganizationTeamResource{team},
		OrganizationTeamRoles: []resources.OrganizationTeamRoleResource{
			{
				Ref:            "platform-team-portal-viewer",
				Team:           "platform-team",
				RoleName:       "Viewer",
				EntityID:       "__REF__:developer-portal#id",
				EntityTypeName: "Portals",
				EntityRegion:   "us",
			},
		},
	}
	client := state.NewClient(state.ClientConfig{
		OrganizationTeamRolesAPI: &organizationTeamRolesAPIStub{
			listTeamRoles: func(
				_ context.Context,
				gotTeamID string,
				_ *kkOps.ListTeamRolesQueryParamFilter,
				_ ...kkOps.Option,
			) (*kkOps.ListTeamRolesResponse, error) {
				require.Equal(t, teamID, gotTeamID)
				return &kkOps.ListTeamRolesResponse{
					AssignedRoleCollection: assignedRoleCollection(
						assignedRole("role-123", "Viewer", portalID, "Portals", "us"),
					),
				}, nil
			},
		},
	})
	planner := NewPlanner(client, discardPlannerLogger())
	planner.resources = resourceSet
	teamPlanner := NewOrganizationTeamPlanner(NewBasePlanner(planner)).(*OrganizationTeamPlannerImpl)
	plan := NewPlan("1.0", "test", PlanModeApply)

	err := teamPlanner.planOrganizationTeamRoleChanges(
		t.Context(),
		"default",
		resourceSet.OrganizationTeams,
		map[string]state.OrganizationTeam{},
		plan,
	)

	require.NoError(t, err)
	require.Empty(t, plan.Changes)
}

func TestOrganizationUserRolePortalEntityRefMatchesExistingRoleInSyncScope(t *testing.T) {
	const (
		portalID = "portal-123"
		userID   = "user-123"
	)

	portal := resources.PortalResource{
		BaseResource: resources.BaseResource{Ref: "developer-portal"},
		CreatePortal: kkComps.CreatePortal{
			Name: "Developer Portal",
		},
	}
	portal.SetKonnectID(portalID)
	user := resources.OrganizationUserResource{
		Ref:   "alice",
		Email: "alice@example.com",
	}
	user.SetKonnectID(userID)

	resourceSet := &resources.ResourceSet{
		Portals: []resources.PortalResource{portal},
		Organization: &resources.OrganizationResource{
			Users: []resources.OrganizationUserResource{user},
		},
		OrganizationUserRoles: []resources.OrganizationUserRoleResource{
			{
				Ref:            "alice-portal-viewer",
				User:           "alice",
				RoleName:       "Viewer",
				EntityID:       "__REF__:developer-portal#id",
				EntityTypeName: "Portals",
				EntityRegion:   "us",
			},
		},
	}
	client := state.NewClient(state.ClientConfig{
		OrganizationTeamRolesAPI: &organizationTeamRolesAPIStub{
			listUserRoles: func(
				_ context.Context,
				gotUserID string,
				_ *kkOps.ListUserRolesQueryParamFilter,
				_ ...kkOps.Option,
			) (*kkOps.ListUserRolesResponse, error) {
				require.Equal(t, userID, gotUserID)
				return &kkOps.ListUserRolesResponse{
					AssignedRoleCollection: assignedRoleCollection(
						assignedRole("role-123", "Viewer", portalID, "Portals", "us"),
					),
				}, nil
			},
		},
	})
	planner := NewPlanner(client, discardPlannerLogger())
	planner.resources = resourceSet
	teamPlanner := NewOrganizationTeamPlanner(NewBasePlanner(planner)).(*OrganizationTeamPlannerImpl)
	plan := NewPlan("1.0", "test", PlanModeSync)

	err := teamPlanner.planOrganizationUserRoleChanges(t.Context(), "default", plan)

	require.NoError(t, err)
	require.Empty(t, plan.Changes)
}

func TestOrganizationSystemAccountRolePortalEntityRefMatchesExistingRoleInSyncScope(t *testing.T) {
	const (
		portalID  = "portal-123"
		accountID = "system-account-123"
	)

	portal := resources.PortalResource{
		BaseResource: resources.BaseResource{Ref: "developer-portal"},
		CreatePortal: kkComps.CreatePortal{
			Name: "Developer Portal",
		},
	}
	portal.SetKonnectID(portalID)
	account := resources.OrganizationSystemAccountResource{
		Ref:  "automation",
		Name: "Automation",
	}
	account.SetKonnectID(accountID)

	resourceSet := &resources.ResourceSet{
		Portals: []resources.PortalResource{portal},
		Organization: &resources.OrganizationResource{
			SystemAccounts: []resources.OrganizationSystemAccountResource{account},
		},
		OrganizationSystemAccountRoles: []resources.OrganizationSystemAccountRoleResource{
			{
				Ref:            "automation-portal-viewer",
				SystemAccount:  "automation",
				RoleName:       "Viewer",
				EntityID:       "__REF__:developer-portal#id",
				EntityTypeName: "Portals",
				EntityRegion:   "us",
			},
		},
	}
	client := state.NewClient(state.ClientConfig{
		SystemAccountRolesAPI: &systemAccountRolesAPIStub{
			listSystemAccountRoles: func(
				_ context.Context,
				gotAccountID string,
				_ *kkOps.GetSystemAccountsAccountIDAssignedRolesQueryParamFilter,
				_ ...kkOps.Option,
			) (*kkOps.GetSystemAccountsAccountIDAssignedRolesResponse, error) {
				require.Equal(t, accountID, gotAccountID)
				return &kkOps.GetSystemAccountsAccountIDAssignedRolesResponse{
					AssignedRoleCollection: assignedRoleCollection(
						assignedRole("role-123", "Viewer", portalID, "Portals", "us"),
					),
				}, nil
			},
		},
	})
	planner := NewPlanner(client, discardPlannerLogger())
	planner.resources = resourceSet
	teamPlanner := NewOrganizationTeamPlanner(NewBasePlanner(planner)).(*OrganizationTeamPlannerImpl)
	plan := NewPlan("1.0", "test", PlanModeSync)

	err := teamPlanner.planOrganizationSystemAccountRoleChanges(t.Context(), "default", plan)

	require.NoError(t, err)
	require.Empty(t, plan.Changes)
}

func TestAPIOnlyApplyDoesNotListOrganizationTeams(t *testing.T) {
	var listTeamCalls int
	apiClient := &MockAPIAPI{}
	mockEmptyAPIsList(t.Context(), apiClient)
	client := state.NewClient(state.ClientConfig{
		APIAPI: apiClient,
		OrganizationTeamAPI: &organizationTeamAPIStub{
			listOrganizationTeams: func(context.Context, kkOps.ListTeamsRequest) (*kkOps.ListTeamsResponse, error) {
				listTeamCalls++
				return nil, fmt.Errorf("forbidden team list")
			},
		},
	})
	planner := NewPlanner(client, discardPlannerLogger())

	plan, err := planner.GeneratePlan(t.Context(), &resources.ResourceSet{
		APIs: []resources.APIResource{
			{
				BaseResource: resources.BaseResource{Ref: "api-only"},
				CreateAPIRequest: kkComps.CreateAPIRequest{
					Name: "API Only",
				},
			},
		},
	}, Options{Mode: PlanModeApply})

	require.NoError(t, err)
	require.Zero(t, listTeamCalls)
	require.Len(t, plan.Changes, 1)
	require.Equal(t, ResourceTypeAPI, plan.Changes[0].ResourceType)
}

func TestOrganizationAssignmentInputsStillListOrganizationTeams(t *testing.T) {
	tests := []struct {
		name      string
		resource  *resources.ResourceSet
		namespace string
	}{
		{
			name: "user assignment",
			resource: &resources.ResourceSet{
				Organization: &resources.OrganizationResource{
					Users: []resources.OrganizationUserResource{
						{
							Ref: "alice",
							ID:  "user-123",
						},
					},
				},
				OrganizationUserRoles: []resources.OrganizationUserRoleResource{
					{
						Ref:            "alice-api-viewer",
						User:           "alice",
						RoleName:       "Viewer",
						EntityID:       "*",
						EntityTypeName: "APIs",
						EntityRegion:   "us",
					},
				},
			},
			namespace: "default",
		},
		{
			name: "system account assignment",
			resource: &resources.ResourceSet{
				Organization: &resources.OrganizationResource{
					SystemAccounts: []resources.OrganizationSystemAccountResource{
						{
							Ref: "automation",
							ID:  "system-account-123",
						},
					},
				},
				OrganizationSystemAccountRoles: []resources.OrganizationSystemAccountRoleResource{
					{
						Ref:            "automation-api-viewer",
						SystemAccount:  "automation",
						RoleName:       "Viewer",
						EntityID:       "*",
						EntityTypeName: "APIs",
						EntityRegion:   "us",
					},
				},
			},
			namespace: "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var listTeamCalls int
			client := state.NewClient(state.ClientConfig{
				OrganizationTeamAPI: &organizationTeamAPIStub{
					listOrganizationTeams: func(context.Context, kkOps.ListTeamsRequest) (*kkOps.ListTeamsResponse, error) {
						listTeamCalls++
						return nil, fmt.Errorf("team list called")
					},
				},
			})
			planner := NewPlanner(client, discardPlannerLogger())
			planner.resources = tt.resource
			teamPlanner := NewOrganizationTeamPlanner(NewBasePlanner(planner)).(*OrganizationTeamPlannerImpl)

			err := teamPlanner.PlanChanges(t.Context(), NewConfig(tt.namespace), NewPlan("1.0", "test", PlanModeApply))

			require.ErrorContains(t, err, "team list called")
			require.Equal(t, 1, listTeamCalls)
		})
	}
}

func TestOrganizationTeamSyncStillListsOrganizationTeams(t *testing.T) {
	var listTeamCalls int
	client := state.NewClient(state.ClientConfig{
		OrganizationTeamAPI: &organizationTeamAPIStub{
			listOrganizationTeams: func(context.Context, kkOps.ListTeamsRequest) (*kkOps.ListTeamsResponse, error) {
				listTeamCalls++
				return nil, fmt.Errorf("team list called")
			},
		},
	})
	planner := NewPlanner(client, discardPlannerLogger())
	planner.resources = &resources.ResourceSet{}
	teamPlanner := NewOrganizationTeamPlanner(NewBasePlanner(planner)).(*OrganizationTeamPlannerImpl)

	err := teamPlanner.PlanChanges(t.Context(), NewConfig("default"), NewPlan("1.0", "test", PlanModeSync))

	require.ErrorContains(t, err, "team list called")
	require.Equal(t, 1, listTeamCalls)
}
