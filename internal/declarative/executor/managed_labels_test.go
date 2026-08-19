package executor

import (
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProtectableAdaptersMapProtectionOnlyLabelUpdates(t *testing.T) {
	covered := make(map[resources.ResourceType]bool)

	t.Run("portal", func(t *testing.T) {
		covered[resources.ResourceTypePortal] = true
		verifyPointerManagedLabels(t, NewPortalAdapter(nil), func(update *kkComps.UpdatePortal) map[string]*string {
			return update.Labels
		})
	})
	t.Run("api", func(t *testing.T) {
		covered[resources.ResourceTypeAPI] = true
		verifyPointerManagedLabels(t, NewAPIAdapter(nil), func(update *kkComps.UpdateAPIRequest) map[string]*string {
			return update.Labels
		})
	})
	t.Run("catalog_service", func(t *testing.T) {
		covered[resources.ResourceTypeCatalogService] = true
		verifyPointerManagedLabels(t, NewCatalogServiceAdapter(nil),
			func(update *kkComps.UpdateCatalogService) map[string]*string {
				return update.Labels
			})
	})
	t.Run("ai_gateway", func(t *testing.T) {
		covered[resources.ResourceTypeAIGateway] = true
		verifyStringManagedLabels(t, NewAIGatewayAdapter(nil),
			func(update *kkComps.UpdateAIGatewayRequest) map[string]string {
				return update.Labels
			})
	})
	t.Run("dashboard", func(t *testing.T) {
		covered[resources.ResourceTypeDashboard] = true
		verifyStringManagedLabels(t, NewDashboardAdapter(nil),
			func(update *kkComps.DashboardUpdateRequest) map[string]string {
				return update.Labels
			})
	})
	t.Run("event_gateway", func(t *testing.T) {
		covered[resources.ResourceTypeEventGatewayControlPlane] = true
		verifyStringManagedLabels(t, NewEventGatewayControlPlaneControlPlaneAdapter(nil),
			func(update *kkComps.UpdateGatewayRequest) map[string]string {
				return update.Labels
			})
	})
	t.Run("application_auth_strategy", func(t *testing.T) {
		covered[resources.ResourceTypeApplicationAuthStrategy] = true
		verifyPointerManagedLabels(t, NewAuthStrategyAdapter(nil),
			func(update *kkComps.UpdateAppAuthStrategyRequest) map[string]*string {
				return update.Labels
			})
	})
	t.Run("dcr_provider", func(t *testing.T) {
		covered[resources.ResourceTypeDCRProvider] = true
		verifyPointerManagedLabels(t, NewDCRProviderAdapter(nil),
			func(update *kkComps.UpdateDcrProviderRequest) map[string]*string {
				return update.Labels
			})
	})
	t.Run("control_plane", func(t *testing.T) {
		covered[resources.ResourceTypeControlPlane] = true
		verifyStringManagedLabels(t, NewControlPlaneAdapter(nil),
			func(update *kkComps.UpdateControlPlaneRequest) map[string]string {
				return update.Labels
			})
	})
	t.Run("organization_team", func(t *testing.T) {
		covered[resources.ResourceTypeOrganizationTeam] = true
		verifyPointerManagedLabels(t, NewOrganizationTeamAdapter(nil), func(update *kkComps.UpdateTeam) map[string]*string {
			return update.Labels
		})
	})

	assertAllProtectableResourcesCovered(t, covered)
}

func verifyPointerManagedLabels[T any](
	t *testing.T,
	mapper ManagedLabelOperations[T],
	getLabels func(*T) map[string]*string,
) {
	t.Helper()

	currentLabels := map[string]string{"team": "platform"}
	var unprotectUpdate T
	mapper.MapUpdateLabels(
		&ExecutionContext{
			Namespace:  "test",
			Protection: planner.ProtectionChange{Old: true, New: false},
		},
		&unprotectUpdate,
		currentLabels,
		currentLabels,
	)
	unprotectedLabels := getLabels(&unprotectUpdate)
	require.Contains(t, unprotectedLabels, labels.ProtectedKey)
	assert.Nil(t, unprotectedLabels[labels.ProtectedKey])
	require.NotNil(t, unprotectedLabels[labels.NamespaceKey])
	assert.Equal(t, "test", *unprotectedLabels[labels.NamespaceKey])
	require.NotNil(t, unprotectedLabels["team"])
	assert.Equal(t, "platform", *unprotectedLabels["team"])

	var protectUpdate T
	mapper.MapUpdateLabels(
		&ExecutionContext{
			Namespace:  "test",
			Protection: planner.ProtectionChange{Old: false, New: true},
		},
		&protectUpdate,
		currentLabels,
		currentLabels,
	)
	protectedLabels := getLabels(&protectUpdate)
	require.NotNil(t, protectedLabels[labels.ProtectedKey])
	assert.Equal(t, labels.TrueValue, *protectedLabels[labels.ProtectedKey])
}

func verifyStringManagedLabels[T any](
	t *testing.T,
	mapper ManagedLabelOperations[T],
	getLabels func(*T) map[string]string,
) {
	t.Helper()

	currentLabels := map[string]string{"team": "platform"}
	var unprotectUpdate T
	mapper.MapUpdateLabels(
		&ExecutionContext{
			Namespace:  "test",
			Protection: planner.ProtectionChange{Old: true, New: false},
		},
		&unprotectUpdate,
		currentLabels,
		currentLabels,
	)
	unprotectedLabels := getLabels(&unprotectUpdate)
	assert.NotContains(t, unprotectedLabels, labels.ProtectedKey)
	assert.Equal(t, "test", unprotectedLabels[labels.NamespaceKey])
	assert.Equal(t, "platform", unprotectedLabels["team"])

	var protectUpdate T
	mapper.MapUpdateLabels(
		&ExecutionContext{
			Namespace:  "test",
			Protection: planner.ProtectionChange{Old: false, New: true},
		},
		&protectUpdate,
		currentLabels,
		currentLabels,
	)
	protectedLabels := getLabels(&protectUpdate)
	assert.Equal(t, labels.TrueValue, protectedLabels[labels.ProtectedKey])
}

func assertAllProtectableResourcesCovered(t *testing.T, covered map[resources.ResourceType]bool) {
	t.Helper()

	resourceSet := resources.ResourceSet{
		Portals:                   []resources.PortalResource{{}},
		APIs:                      []resources.APIResource{{}},
		CatalogServices:           []resources.CatalogServiceResource{{}},
		AIGateways:                []resources.AIGatewayResource{{}},
		Dashboards:                []resources.DashboardResource{{}},
		EventGatewayControlPlanes: []resources.EventGatewayControlPlaneResource{{}},
		ApplicationAuthStrategies: []resources.ApplicationAuthStrategyResource{{}},
		DCRProviders:              []resources.DCRProviderResource{{}},
		ControlPlanes:             []resources.ControlPlaneResource{{}},
		OrganizationTeams:         []resources.OrganizationTeamResource{{}},
	}

	protectable := make(map[resources.ResourceType]bool)
	err := resourceSet.ForEachNamespaceParticipant(func(participant resources.NamespaceParticipant) error {
		if participant.SupportsProtected {
			protectable[participant.Type] = true
		}
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, protectable, covered)
}
