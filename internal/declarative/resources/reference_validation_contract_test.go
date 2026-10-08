package resources

import (
	"errors"
	"maps"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReferenceValidationEnrollment(t *testing.T) {
	type position struct{ kind, parent ResourceType }
	var got []position
	for _, step := range buildReferenceValidationSteps() {
		got = append(got, position{step.kind, step.parent})
	}
	require.Equal(t, []position{
		{ResourceTypePortal, ""},
		{ResourceTypeAPIPublication, ResourceTypeAPI},
		{ResourceTypeAPIImplementation, ResourceTypeAPI},
		{ResourceTypeAPIPublication, ""},
		{ResourceTypeAPIImplementation, ""},
		{ResourceTypeAPIDocument, ""},
		{ResourceTypePortalIPAllowList, ""},
		{ResourceTypePortalAuditLogWebhook, ""},
	}, got)

	// These mapping implementations were not enrolled in the loader's reference
	// pass. Keep that compatibility disposition explicit; new kinds must choose
	// enrollment or a reviewed exclusion instead of being silently missed.
	excluded := []ResourceType{
		ResourceTypeAPI, ResourceTypeAPIVersion,
		ResourceTypeApplicationAuthStrategy, ResourceTypeDCRProvider,
		ResourceTypeControlPlane, ResourceTypeControlPlaneDataPlaneCertificate,
		ResourceTypeGatewayService, ResourceTypeOrganizationTeam,
		ResourceTypePortalPage, ResourceTypePortalSnippet, ResourceTypePortalIntegration,
		ResourceTypePortalEmailConfig, ResourceTypePortalEmailTemplate,
		ResourceTypeAIGatewayAgent, ResourceTypeAIGatewayAuthStrategy,
		ResourceTypeAIGatewayConfigStore, ResourceTypeAIGatewayConfigStoreSecret,
		ResourceTypeAIGatewayConsumer, ResourceTypeAIGatewayConsumerCredential,
		ResourceTypeAIGatewayConsumerGroup, ResourceTypeAIGatewayCustomPolicy,
		ResourceTypeAIGatewayDataPlaneCertificate, ResourceTypeAIGatewayMCPServer,
		ResourceTypeAIGatewayModel, ResourceTypeAIGatewayPolicy, ResourceTypeAIGatewayProvider,
		ResourceTypeAIGatewayCertificate, ResourceTypeAIGatewayCACertificate, ResourceTypeAIGatewaySNI,
		ResourceTypeAIGatewayVault,
	}
	var actualExcluded []ResourceType
	for _, kind := range RegisteredTypes() {
		ops := registry[kind]
		if reflect.PointerTo(ops.explain.typ).Implements(reflect.TypeFor[ReferenceMapping]()) &&
			ops.referenceValidation == nil && ops.nestedReferenceValidation == nil {
			actualExcluded = append(actualExcluded, kind)
		}
	}
	require.ElementsMatch(t, excluded, actualExcluded)
}

func TestReferenceValidationRegistrationGuards(t *testing.T) {
	nested := func(phase, order int) ResourceRegistrationOption {
		return withNestedReferenceValidation(phase, order, func(api *APIResource) *[]APIPublicationResource {
			return &api.Publications
		})
	}
	tests := []struct {
		name    string
		kind    ResourceType
		options []ResourceRegistrationOption
		message string
	}{
		{
			"zero flat phase", ResourceTypeAPIPublication,
			[]ResourceRegistrationOption{withReferenceValidation(0)},
			"reference validation requires a positive phase and reference mappings",
		},
		{
			"negative flat phase", ResourceTypeAPIPublication,
			[]ResourceRegistrationOption{withReferenceValidation(-1)},
			"reference validation requires a positive phase and reference mappings",
		},
		{
			"no mappings", ResourceTypeEventGatewayControlPlane,
			[]ResourceRegistrationOption{withReferenceValidation(10)},
			"reference validation requires a positive phase and reference mappings",
		},
		{
			"duplicate flat", ResourceTypeAPIPublication,
			[]ResourceRegistrationOption{withReferenceValidation(10), withReferenceValidation(20)},
			"reference validation is already registered",
		},
		{
			"zero nested phase", ResourceTypeAPIPublication,
			[]ResourceRegistrationOption{nested(0, 10)},
			"nested reference validation requires positive phase/order and matching typed accessor",
		},
		{
			"negative nested order", ResourceTypeAPIPublication,
			[]ResourceRegistrationOption{nested(10, -1)},
			"nested reference validation requires positive phase/order and matching typed accessor",
		},
		{
			"wrong child type", ResourceTypeAPIImplementation,
			[]ResourceRegistrationOption{nested(10, 10)},
			"nested reference validation requires positive phase/order and matching typed accessor",
		},
		{
			"nil accessor", ResourceTypeAPIPublication,
			[]ResourceRegistrationOption{withNestedReferenceValidation[APIResource, APIPublicationResource](10, 10, nil)},
			"nested reference validation requires positive phase/order and matching typed accessor",
		},
		{
			"duplicate nested", ResourceTypeAPIPublication,
			[]ResourceRegistrationOption{nested(10, 10), nested(20, 20)},
			"nested reference validation is already registered",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ops := registry[tt.kind]
			ops.referenceValidation, ops.nestedReferenceValidation = nil, nil
			for _, option := range tt.options[:len(tt.options)-1] {
				require.NoError(t, option(&ops))
			}
			require.EqualError(t, tt.options[len(tt.options)-1](&ops), tt.message)
		})
	}
}

func TestReferenceValidationDispatchGuards(t *testing.T) {
	tests := []struct {
		name      string
		configure func()
		message   string
	}{
		{"duplicate flat phase", func() {
			registry[ResourceTypeAPIDocument].referenceValidation.phase = registry[ResourceTypePortal].referenceValidation.phase
		}, "conflicting reference validation position for api_document and portal"},
		{"flat and nested phase collision", func() {
			publication := registry[ResourceTypeAPIPublication].nestedReferenceValidation
			registry[ResourceTypeAPIDocument].referenceValidation.phase = publication.phase
		}, "conflicting reference validation position for api_document and api_publication"},
		{"duplicate nested position", func() {
			publication := registry[ResourceTypeAPIPublication].nestedReferenceValidation
			registry[ResourceTypeAPIImplementation].nestedReferenceValidation.order = publication.order
		}, "conflicting reference validation position for api_implementation and api_publication"},
		{"different parents at same phase", func() {
			nested := registry[ResourceTypeAPIImplementation].nestedReferenceValidation
			nested.parent, nested.parentType = ResourceTypePortal, reflect.TypeFor[PortalResource]()
		}, "conflicting reference validation position for api_publication and api_implementation"},
		{"missing parent", func() {
			delete(registry, ResourceTypeAPI)
		}, "nested reference validation requires a matching registered parent: api_implementation"},
		{"mistyped parent", func() {
			registry[ResourceTypeAPIImplementation].nestedReferenceValidation.parentType = reflect.TypeFor[PortalResource]()
		}, "nested reference validation requires a matching registered parent: api_implementation"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := registry
			registry = maps.Clone(original)
			for kind, ops := range registry {
				if ops.referenceValidation != nil {
					value := *ops.referenceValidation
					ops.referenceValidation = &value
				}
				if ops.nestedReferenceValidation != nil {
					value := *ops.nestedReferenceValidation
					ops.nestedReferenceValidation = &value
				}
				registry[kind] = ops
			}
			t.Cleanup(func() { registry = original })
			tt.configure()
			// The builder is uncached here, so guards cannot be hidden by an
			// earlier successful assembly. Diagnostics must be deterministic.
			for range 3 {
				require.PanicsWithValue(t, tt.message, func() { buildReferenceValidationSteps() })
			}
		})
	}
}

func TestReferenceValidationReturnsVisitorError(t *testing.T) {
	rs := &ResourceSet{
		Portals: []PortalResource{{BaseResource: BaseResource{Ref: "portal"}}},
		APIs: []APIResource{
			{
				Publications:    []APIPublicationResource{{Ref: "a-pub-1"}, {Ref: "a-pub-2"}},
				Implementations: []APIImplementationResource{{Ref: "a-impl"}},
				Documents:       []APIDocumentResource{{Ref: "excluded-nested-doc"}},
			},
			{
				Publications:    []APIPublicationResource{{Ref: "b-pub"}},
				Implementations: []APIImplementationResource{{Ref: "b-impl"}},
			},
		},
		APIPublications:        []APIPublicationResource{{Ref: "flat-pub"}},
		APIImplementations:     []APIImplementationResource{{Ref: "flat-impl"}},
		APIDocuments:           []APIDocumentResource{{Ref: "flat-doc"}},
		PortalIPAllowLists:     []PortalIPAllowListResource{{Ref: "ip-list"}},
		PortalAuditLogWebhooks: []PortalAuditLogWebhookResource{{Ref: "webhook"}},
		PortalPages:            []PortalPageResource{{Ref: "excluded-page"}},
	}
	want := []string{
		"portal", "a-pub-1", "a-pub-2", "a-impl", "b-pub", "b-impl",
		"flat-pub", "flat-impl", "flat-doc", "ip-list", "webhook",
	}
	var got []string
	require.NoError(t, rs.ValidateRegisteredReferences(func(resource Resource) error {
		got = append(got, resource.GetRef())
		return nil
	}))
	require.Equal(t, want, got)
	for i, stop := range want {
		sentinel := errors.New("stop reference validation")
		got = nil
		err := rs.ValidateRegisteredReferences(func(resource Resource) error {
			got = append(got, resource.GetRef())
			if resource.GetRef() == stop {
				return sentinel
			}
			return nil
		})
		require.Same(t, sentinel, err)
		require.Equal(t, want[:i+1], got)
	}
}
