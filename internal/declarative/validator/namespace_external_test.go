package validator

import (
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/stretchr/testify/require"
)

func TestValidateNamespaceRequirementSkipsExternalResources(t *testing.T) {
	ns := "team-a"
	api := resources.APIResource{}
	api.Ref = "api-1"
	api.Kongctl = &resources.KongctlMeta{Namespace: &ns, NamespaceOrigin: resources.NamespaceOriginExplicit}

	rs := &resources.ResourceSet{
		Portals: []resources.PortalResource{{External: &resources.ExternalBlock{ID: "ext-portal"}}},
		APIs:    []resources.APIResource{api},
	}

	// The external portal carries no metadata and must be skipped; only the managed
	// API (explicit namespace) is checked, so enforcement passes.
	err := NewNamespaceValidator().ValidateNamespaceRequirement(rs, NamespaceRequirement{Mode: NamespaceRequirementAny})
	require.NoError(t, err)
}

// External AI gateways are skipped like every other external resource. An
// external resource cannot carry kongctl metadata, so checking it would raise a
// violation no configuration can satisfy.
func TestValidateNamespaceRequirementSkipsExternalAIGateways(t *testing.T) {
	ns := "team-a"
	api := resources.APIResource{}
	api.Ref = "api-1"
	api.Kongctl = &resources.KongctlMeta{Namespace: &ns, NamespaceOrigin: resources.NamespaceOriginExplicit}

	gateway := resources.AIGatewayResource{External: &resources.ExternalBlock{ID: "ext-gateway"}}
	gateway.Ref = "gateway-1"

	rs := &resources.ResourceSet{
		AIGateways: []resources.AIGatewayResource{gateway},
		APIs:       []resources.APIResource{api},
	}

	err := NewNamespaceValidator().ValidateNamespaceRequirement(rs, NamespaceRequirement{Mode: NamespaceRequirementAny})
	require.NoError(t, err)
}

// A managed AI gateway without an explicit namespace is still a violation.
func TestValidateNamespaceRequirementChecksManagedAIGateways(t *testing.T) {
	gateway := resources.AIGatewayResource{}
	gateway.Ref = "gateway-1"

	rs := &resources.ResourceSet{AIGateways: []resources.AIGatewayResource{gateway}}

	err := NewNamespaceValidator().ValidateNamespaceRequirement(rs, NamespaceRequirement{Mode: NamespaceRequirementAny})
	require.ErrorContains(t, err, "ai_gateway 'gateway-1': missing explicit namespace")
}
