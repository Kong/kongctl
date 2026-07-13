package resources

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// oneOfEveryParticipant builds a ResourceSet holding a single instance of every
// namespace-bearing resource, including the nested Analytics.Dashboards and
// Organization.Teams locations that exist before extractNestedResources runs.
func oneOfEveryParticipant() *ResourceSet {
	return &ResourceSet{
		Portals:                   []PortalResource{{}},
		APIs:                      []APIResource{{}},
		CatalogServices:           []CatalogServiceResource{{}},
		Dashboards:                []DashboardResource{{}},
		EventGatewayControlPlanes: []EventGatewayControlPlaneResource{{}},
		ApplicationAuthStrategies: []ApplicationAuthStrategyResource{{}},
		DCRProviders:              []DCRProviderResource{{}},
		ControlPlanes:             []ControlPlaneResource{{}},
		OrganizationTeams:         []OrganizationTeamResource{{}},
		Analytics:                 &AnalyticsResource{Dashboards: []DashboardResource{{}}},
		Organization: &OrganizationResource{
			Teams:          []OrganizationTeamResource{{}},
			Users:          []OrganizationUserResource{{}},
			SystemAccounts: []OrganizationSystemAccountResource{{}},
		},
	}
}

func TestForEachNamespaceParticipantVisitsEveryType(t *testing.T) {
	counts := map[ResourceType]int{}
	err := oneOfEveryParticipant().ForEachNamespaceParticipant(func(p NamespaceParticipant) error {
		counts[p.Type]++
		return nil
	})
	assert.NoError(t, err)

	assert.Equal(t, map[ResourceType]int{
		ResourceTypePortal:                    1,
		ResourceTypeAPI:                       1,
		ResourceTypeCatalogService:            1,
		ResourceTypeDashboard:                 2, // top-level + nested Analytics.Dashboards
		ResourceTypeEventGatewayControlPlane:  1,
		ResourceTypeApplicationAuthStrategy:   1,
		ResourceTypeDCRProvider:               1,
		ResourceTypeControlPlane:              1,
		ResourceTypeOrganizationTeam:          2, // top-level + nested Organization.Teams
		ResourceTypeOrganizationUser:          1,
		ResourceTypeOrganizationSystemAccount: 1,
	}, counts)
}

func TestForEachNamespaceParticipantMetadata(t *testing.T) {
	byType := map[ResourceType]NamespaceParticipant{}
	_ = oneOfEveryParticipant().ForEachNamespaceParticipant(func(p NamespaceParticipant) error {
		byType[p.Type] = p
		return nil
	})

	// Only organization users and system accounts skip protected defaulting.
	assert.False(t, byType[ResourceTypeOrganizationUser].SupportsProtected)
	assert.False(t, byType[ResourceTypeOrganizationSystemAccount].SupportsProtected)
	assert.True(t, byType[ResourceTypePortal].SupportsProtected)
	assert.True(t, byType[ResourceTypeControlPlane].SupportsProtected)

	// Meta addresses the resource field so defaulting can write in place.
	assert.NotNil(t, byType[ResourceTypePortal].Meta)
	assert.Equal(t, "portal", byType[ResourceTypePortal].Label)
	assert.Equal(t, "organization user", byType[ResourceTypeOrganizationUser].Label)
}

func TestForEachNamespaceParticipantStopsOnError(t *testing.T) {
	visited := 0
	err := oneOfEveryParticipant().ForEachNamespaceParticipant(func(NamespaceParticipant) error {
		visited++
		return assert.AnError
	})
	assert.ErrorIs(t, err, assert.AnError)
	assert.Equal(t, 1, visited)
}
