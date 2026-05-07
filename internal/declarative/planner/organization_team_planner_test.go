package planner

import (
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/resources"
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
