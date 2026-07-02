package resources

import (
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func namespaceMeta(namespace string) *KongctlMeta {
	return &KongctlMeta{Namespace: &namespace}
}

func TestGetPortalsByNamespaceIncludesExternalsWhenRequested(t *testing.T) {
	team := "team-one"
	rs := &ResourceSet{
		Portals: []PortalResource{
			{
				BaseResource: BaseResource{
					Ref: "managed",
					Kongctl: &KongctlMeta{
						Namespace: &team,
					},
				},
			},
			{
				BaseResource: BaseResource{Ref: "external"},
				External: &ExternalBlock{
					Selector: &ExternalSelector{MatchFields: map[string]string{"name": "ext"}},
				},
			},
		},
		PortalPages: []PortalPageResource{
			{Ref: "page-1", Portal: "managed"},
			{Ref: "page-2", Portal: "external"},
		},
	}

	managed := rs.GetPortalsByNamespace(team)
	require.Len(t, managed, 1)
	require.Equal(t, "managed", managed[0].Ref)

	external := rs.GetPortalsByNamespace(NamespaceExternal)
	require.Len(t, external, 1)
	require.Equal(t, "external", external[0].Ref)

	managedPages := rs.GetPortalPagesByNamespace(team)
	require.Len(t, managedPages, 1)
	require.Equal(t, "page-1", managedPages[0].Ref)

	externalPages := rs.GetPortalPagesByNamespace(NamespaceExternal)
	require.Len(t, externalPages, 1)
	require.Equal(t, "page-2", externalPages[0].Ref)
}

func TestGetEventGatewayControlPlanesByNamespaceIncludesExternalsWhenRequested(t *testing.T) {
	team := "team-one"
	rs := &ResourceSet{
		EventGatewayControlPlanes: []EventGatewayControlPlaneResource{
			{
				BaseResource: BaseResource{
					Ref:     "managed",
					Kongctl: namespaceMeta(team),
				},
				CreateGatewayRequest: kkComps.CreateGatewayRequest{Name: "managed"},
			},
			{
				BaseResource:         BaseResource{Ref: "external"},
				CreateGatewayRequest: kkComps.CreateGatewayRequest{Name: "external"},
				External:             &ExternalBlock{ID: "gateway-123"},
				VirtualClusters: []EventGatewayVirtualClusterResource{
					{
						Ref:                         "external-vc",
						CreateVirtualClusterRequest: kkComps.CreateVirtualClusterRequest{Name: "external-vc"},
						External:                    &ExternalBlock{ID: "vc-123"},
					},
				},
			},
		},
	}

	managed := rs.GetEventGatewayControlPlanesByNamespace(team)
	require.Len(t, managed, 1)
	require.Equal(t, "managed", managed[0].Ref)

	external := rs.GetEventGatewayControlPlanesByNamespace(NamespaceExternal)
	require.Len(t, external, 1)
	require.Equal(t, "external", external[0].Ref)

	require.Equal(t, "external", rs.GetEventGatewayControlPlaneByRef("external").Ref)
	require.Equal(t, "external-vc", rs.GetVirtualClusterByRef("external-vc").Ref)
	require.Equal(t, "vc-123", rs.GetVirtualClusterByRef("external-vc").External.ID)
}

func TestGetOrganizationUserTeamMembershipsByNamespaceUsesTeamNamespace(t *testing.T) {
	teamNamespace := "team-namespace"
	rs := &ResourceSet{
		Organization: &OrganizationResource{
			Users: []OrganizationUserResource{
				{Ref: "existing-user", Email: "existing-user@example.com"},
			},
		},
		OrganizationTeams: []OrganizationTeamResource{
			{BaseResource: BaseResource{Ref: "managed-team", Kongctl: namespaceMeta(teamNamespace)}},
		},
		OrganizationUserTeamMemberships: []OrganizationUserTeamMembershipResource{
			{Ref: "existing-user-managed-team", User: "existing-user", Team: "managed-team"},
		},
	}

	memberships := rs.GetOrganizationUserTeamMembershipsByNamespace(teamNamespace)
	require.Len(t, memberships, 1)
	require.Equal(t, "existing-user-managed-team", memberships[0].Ref)

	require.Empty(t, rs.GetOrganizationUserTeamMembershipsByNamespace("default"))
}

func TestGetOrganizationUserTeamMembershipsByNamespacePartitionsByTeamNamespace(t *testing.T) {
	alphaNamespace := "alpha-namespace"
	betaNamespace := "beta-namespace"
	rs := &ResourceSet{
		Organization: &OrganizationResource{
			Users: []OrganizationUserResource{
				{Ref: "existing-user", Email: "existing-user@example.com"},
			},
		},
		OrganizationTeams: []OrganizationTeamResource{
			{BaseResource: BaseResource{Ref: "alpha-team", Kongctl: namespaceMeta(alphaNamespace)}},
			{BaseResource: BaseResource{Ref: "beta-team", Kongctl: namespaceMeta(betaNamespace)}},
		},
		OrganizationUserTeamMemberships: []OrganizationUserTeamMembershipResource{
			{Ref: "existing-user-alpha-team", User: "existing-user", Team: "alpha-team"},
			{Ref: "existing-user-beta-team", User: "existing-user", Team: "beta-team"},
		},
	}

	alphaMemberships := rs.GetOrganizationUserTeamMembershipsByNamespace(alphaNamespace)
	require.Len(t, alphaMemberships, 1)
	require.Equal(t, "existing-user-alpha-team", alphaMemberships[0].Ref)

	betaMemberships := rs.GetOrganizationUserTeamMembershipsByNamespace(betaNamespace)
	require.Len(t, betaMemberships, 1)
	require.Equal(t, "existing-user-beta-team", betaMemberships[0].Ref)

	require.Empty(t, rs.GetOrganizationUserTeamMembershipsByNamespace("default"))
}

func TestGetOrganizationSystemAccountTeamMembershipsByNamespaceUsesTeamNamespace(t *testing.T) {
	teamNamespace := "team-namespace"
	rs := &ResourceSet{
		Organization: &OrganizationResource{
			SystemAccounts: []OrganizationSystemAccountResource{
				{Ref: "existing-system-account", Name: "existing-system-account"},
			},
		},
		OrganizationTeams: []OrganizationTeamResource{
			{BaseResource: BaseResource{Ref: "managed-team", Kongctl: namespaceMeta(teamNamespace)}},
		},
		OrganizationSystemAccountTeamMemberships: []OrganizationSystemAccountTeamMembershipResource{
			{
				Ref:           "existing-system-account-managed-team",
				SystemAccount: "existing-system-account",
				Team:          "managed-team",
			},
		},
	}

	memberships := rs.GetOrganizationSystemAccountTeamMembershipsByNamespace(teamNamespace)
	require.Len(t, memberships, 1)
	require.Equal(t, "existing-system-account-managed-team", memberships[0].Ref)

	require.Empty(t, rs.GetOrganizationSystemAccountTeamMembershipsByNamespace("default"))
}

func TestResourceSetMarshalOmitsEmptyAuditLogs(t *testing.T) {
	rs := &ResourceSet{}

	require.True(t, rs.IsEmpty())
	require.Empty(t, rs.AllResourcesByType(ResourceTypeAuditLogWebhookDestination))
	require.Nil(t, rs.AuditLogs)

	yamlBytes, err := yaml.Marshal(rs)
	require.NoError(t, err)
	require.NotContains(t, string(yamlBytes), "audit-logs:")
	require.Nil(t, rs.AuditLogs)
}
