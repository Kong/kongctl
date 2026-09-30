package resources

import (
	"fmt"
	"maps"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCollectionValidationStepOrder(t *testing.T) {
	// Pin diagnostic order without freezing the numeric gaps between phases.
	want := []ResourceType{
		ResourceTypePortal,
		ResourceTypeApplicationAuthStrategy,
		ResourceTypeDCRProvider,
		ResourceTypeControlPlane,
		ResourceTypeCatalogService,
		ResourceTypeAIGateway,
		ResourceTypeAIGatewayProvider,
		ResourceTypeAIGatewayAuthStrategy,
		ResourceTypeAIGatewayCustomPolicy,
		ResourceTypeAIGatewayPolicy,
		ResourceTypeAIGatewayAgent,
		ResourceTypeAIGatewayConsumer,
		ResourceTypeAIGatewayConsumerCredential,
		ResourceTypeAIGatewayConsumerGroup,
		ResourceTypeAIGatewayModel,
		ResourceTypeAIGatewayMCPServer,
		ResourceTypeAIGatewayConfigStore,
		ResourceTypeAIGatewayConfigStoreSecret,
		ResourceTypeAIGatewayVault,
		ResourceTypeAIGatewayDataPlaneCertificate,
		ResourceTypeAIGatewayCertificate,
		ResourceTypeAIGatewayCACertificate,
		ResourceTypeAIGatewaySNI,
		ResourceTypeDashboard,
		ResourceTypeGatewayService,
		ResourceTypeAuditLogWebhookDestination,
		ResourceTypeControlPlaneDataPlaneCertificate,
		ResourceTypeAPI,
		ResourceTypeAPIVersion,
		ResourceTypeAPIPublication,
		ResourceTypeAPIImplementation,
		ResourceTypeAPIDocument,
		ResourceTypePortalPage,
		ResourceTypePortalSnippet,
		ResourceTypePortalCustomization,
		ResourceTypePortalAuthSettings,
		ResourceTypePortalIPAllowList,
		ResourceTypePortalIntegration,
		ResourceTypePortalIdentityProvider,
		ResourceTypePortalTeamGroupMapping,
		ResourceTypePortalCustomDomain,
		ResourceTypePortalEmailConfig,
		ResourceTypePortalAuditLogWebhook,
		ResourceTypePortalEmailTemplate,
		ResourceTypeOrganizationTeam,
		ResourceTypeOrganizationTeamRole,
		ResourceTypeOrganizationUser,
		ResourceTypeOrganizationUserTeamMembership,
		ResourceTypeOrganizationUserRole,
		ResourceTypeOrganizationSystemAccount,
		ResourceTypeOrganizationSystemAccountTeamMembership,
		ResourceTypeOrganizationSystemAccountRole,
	}
	var got []ResourceType
	for _, step := range buildCollectionValidationSteps() {
		got = append(got, step.kind)
	}
	require.Equal(t, want, got)
}

func TestCollectionValidationRejectsInvalidDispatch(t *testing.T) {
	tests := []struct {
		name      string
		configure func()
		wantPanic string
	}{
		{
			"duplicate position",
			func() {
				registry[ResourceTypeAPI].collectionValidation.phase = registry[ResourceTypePortal].collectionValidation.phase
			},
			"duplicate collection validation position for ",
		},
		{
			"missing inherited phase",
			func() {
				ops := registry[ResourceTypeControlPlane]
				ops.childValidationPhase = 0
				registry[ResourceTypeControlPlane] = ops
			},
			"child validation family has no registered phase: " + string(ResourceTypeControlPlane),
		},
		{
			"missing selector phase with explicit assignment phases",
			func() {
				selector := selectorLoaders[ResourceTypeOrganizationUser]
				selector.validationPhase = 0
				selectorLoaders[ResourceTypeOrganizationUser] = selector
				registry[ResourceTypeOrganizationUserTeamMembership].load.validatePhase = 170
				registry[ResourceTypeOrganizationUserRole].load.validatePhase = 170
			},
			"selector validation has no registered phase: " + string(ResourceTypeOrganizationUser),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isolateCollectionValidationRegistries(t)
			tt.configure()
			var caught any
			func() {
				defer func() { caught = recover() }()
				buildCollectionValidationSteps()
			}()
			require.NotNil(t, caught)
			require.Contains(t, fmt.Sprint(caught), tt.wantPanic)
		})
	}
}

func TestCollectionValidationExplicitPhaseWithoutFamilyDefault(t *testing.T) {
	isolateCollectionValidationRegistries(t)
	ops := registry[ResourceTypeControlPlane]
	ops.childValidationPhase = 0
	registry[ResourceTypeControlPlane] = ops
	registry[ResourceTypeGatewayService].load.validatePhase = 90
	registry[ResourceTypeControlPlaneDataPlaneCertificate].load.validatePhase = 111

	got := make(map[ResourceType]int)
	for _, step := range buildCollectionValidationSteps() {
		if step.kind == ResourceTypeGatewayService || step.kind == ResourceTypeControlPlaneDataPlaneCertificate {
			got[step.kind] = step.phase
		}
	}
	require.Equal(t, map[ResourceType]int{
		ResourceTypeGatewayService:                   90,
		ResourceTypeControlPlaneDataPlaneCertificate: 111,
	}, got)
}

func TestCollectionValidationRejectsInvalidRegistration(t *testing.T) {
	validate := namedCollectionValidation(func(r *PortalResource) string { return r.Name })
	tests := []struct {
		name      string
		options   []ResourceRegistrationOption
		wantPanic string
	}{
		{
			"missing root disposition",
			[]ResourceRegistrationOption{WithRootSyncScope()},
			"managed root requires a collection validation disposition: " + string(ResourceTypePortal),
		},
		{
			"conflicting child disposition",
			[]ResourceRegistrationOption{
				withChildLoadRegistration(registry[ResourceTypePortalPage].load),
				withCollectionValidationOmitted("child validation already supplied"),
			},
			"child loader already supplies the collection validation disposition: " + string(ResourceTypePortal),
		},
		{
			"nonpositive phase",
			[]ResourceRegistrationOption{withCollectionValidation(0, validate)},
			"register resource type " + string(ResourceTypePortal) +
				": collection validation requires a positive phase and matching typed validator",
		},
		{
			"mismatched validator type",
			[]ResourceRegistrationOption{
				withCollectionValidation(10, namedCollectionValidation(func(r *APIResource) string { return r.Name })),
			},
			"register resource type " + string(ResourceTypePortal) +
				": collection validation requires a positive phase and matching typed validator",
		},
		{
			"conflicting root disposition",
			[]ResourceRegistrationOption{
				withCollectionValidation(10, validate), withCollectionValidationOmitted("already validated"),
			},
			"register resource type " + string(ResourceTypePortal) + ": collection validation is already registered",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isolateCollectionValidationRegistries(t)
			require.PanicsWithValue(t, tt.wantPanic, func() {
				registerResourceType(
					ResourceTypePortal,
					func(rs *ResourceSet) *[]PortalResource { return &rs.Portals },
					AutoExplain[PortalResource](),
					tt.options...,
				)
			})
		})
	}
}

func isolateCollectionValidationRegistries(t *testing.T) {
	t.Helper()
	// Exercise the uncached builder without changing the shared OnceValue or
	// mutating registrations that other tests and cached visitors still use.
	original, selectors := registry, selectorLoaders
	registry, selectorLoaders = maps.Clone(registry), maps.Clone(selectorLoaders)
	for kind, ops := range registry {
		if ops.load != nil {
			load := *ops.load
			ops.load = &load
		}
		if ops.collectionValidation != nil {
			validation := *ops.collectionValidation
			ops.collectionValidation = &validation
		}
		registry[kind] = ops
	}
	t.Cleanup(func() { registry, selectorLoaders = original, selectors })
}
