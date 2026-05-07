package planner

import (
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/require"
)

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
