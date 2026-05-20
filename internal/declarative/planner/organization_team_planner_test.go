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

func discardPlannerLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
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
