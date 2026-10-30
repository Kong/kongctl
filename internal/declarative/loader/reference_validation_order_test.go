package loader

import (
	"fmt"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/stretchr/testify/require"
)

func TestCrossReferenceValidationDiagnosticOrder(t *testing.T) {
	publication := func(ref string) resources.APIPublicationResource {
		return resources.APIPublicationResource{Ref: ref, PortalID: "missing"}
	}
	implementation := func(ref string) resources.APIImplementationResource {
		return resources.APIImplementationResource{
			Ref: ref,
			APIImplementation: kkComps.APIImplementation{
				Type: kkComps.APIImplementationTypeServiceReference,
				ServiceReference: &kkComps.ServiceReference{
					Service: &kkComps.APIImplementationService{ControlPlaneID: "11111111-1111-4111-8111-111111111111"},
				},
			},
		}
	}
	rs := &resources.ResourceSet{
		Portals: []resources.PortalResource{
			{
				BaseResource: resources.BaseResource{Ref: "portal-a"},
				CreatePortal: kkComps.CreatePortal{DefaultApplicationAuthStrategyID: new("missing")},
			},
			{
				BaseResource: resources.BaseResource{Ref: "portal-b"},
				CreatePortal: kkComps.CreatePortal{DefaultApplicationAuthStrategyID: new("missing")},
			},
		},
		APIs: []resources.APIResource{
			{
				BaseResource: resources.BaseResource{Ref: "api-a"},
				Publications: []resources.APIPublicationResource{publication("a-pub-1"), publication("a-pub-2")},
				Implementations: []resources.APIImplementationResource{
					implementation("a-impl-1"), implementation("a-impl-2"),
				},
				// Nested documents are deliberately outside this reference pass.
				Documents: []resources.APIDocumentResource{{Ref: "nested-doc", API: "missing"}},
			},
			{
				BaseResource:    resources.BaseResource{Ref: "api-b"},
				Publications:    []resources.APIPublicationResource{publication("b-pub")},
				Implementations: []resources.APIImplementationResource{implementation("b-impl")},
			},
		},
		APIPublications:    []resources.APIPublicationResource{publication("flat-pub")},
		APIImplementations: []resources.APIImplementationResource{implementation("flat-impl")},
		APIDocuments:       []resources.APIDocumentResource{{Ref: "flat-doc", API: "missing"}},
		PortalIPAllowLists: []resources.PortalIPAllowListResource{{Ref: "ip-list", Portal: "missing"}},
		PortalAuditLogWebhooks: []resources.PortalAuditLogWebhookResource{
			{Ref: "webhook", Portal: "missing"},
		},
		// ReferenceMapping does not automatically enroll a kind.
		PortalPages: []resources.PortalPageResource{{Ref: "page", Portal: "missing"}},
	}
	type failure struct {
		ref   string
		kind  resources.ResourceType
		field string
		value *string
	}
	want := []failure{
		{
			"portal-a", resources.ResourceTypeApplicationAuthStrategy, "default_application_auth_strategy_id",
			rs.Portals[0].DefaultApplicationAuthStrategyID,
		},
		{
			"portal-b", resources.ResourceTypeApplicationAuthStrategy, "default_application_auth_strategy_id",
			rs.Portals[1].DefaultApplicationAuthStrategyID,
		},
		{"a-pub-1", resources.ResourceTypePortal, "portal_id", &rs.APIs[0].Publications[0].PortalID},
		{"a-pub-2", resources.ResourceTypePortal, "portal_id", &rs.APIs[0].Publications[1].PortalID},
		{"b-pub", resources.ResourceTypePortal, "portal_id", &rs.APIs[1].Publications[0].PortalID},
		{"flat-pub", resources.ResourceTypePortal, "portal_id", &rs.APIPublications[0].PortalID},
		{"flat-doc", resources.ResourceTypeAPI, "api", &rs.APIDocuments[0].API},
		{"ip-list", resources.ResourceTypePortal, "portal", &rs.PortalIPAllowLists[0].Portal},
		{"webhook", resources.ResourceTypePortal, "portal", &rs.PortalAuditLogWebhooks[0].Portal},
	}
	loader := New()
	// Clear each first failure so the next call observes the next diagnostic.
	for _, next := range want {
		require.EqualError(t, loader.validateCrossReferences(rs), fmt.Sprintf(
			"resource %q references unknown %s: missing (field: %s)", next.ref, next.kind, next.field,
		))
		*next.value = ""
	}
	require.NoError(t, loader.validateCrossReferences(rs))
	require.NoError(t, loader.validateCrossReferences(&resources.ResourceSet{}))
}
