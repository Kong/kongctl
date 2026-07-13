package planner

import (
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/stretchr/testify/assert"
)

func TestGetResourceNamespacesExternalControlPlaneContributesNamespace(t *testing.T) {
	planner := &Planner{}
	rs := &resources.ResourceSet{
		ControlPlanes: []resources.ControlPlaneResource{
			{External: &resources.ExternalBlock{ID: "ext-cp"}},
		},
	}
	// External control planes still contribute their own namespace, unlike external
	// portals/event-gateways/ai-gateways/teams which map to the external namespace
	// instead.
	assert.Equal(t, []string{"default"}, planner.getResourceNamespaces(rs))
}

func TestGetResourceNamespacesExternalAIGatewayMapsToExternal(t *testing.T) {
	planner := &Planner{}
	gateway := resources.AIGatewayResource{External: &resources.ExternalBlock{ID: "ext-gateway"}}
	gateway.Ref = "gateway-1"

	rs := &resources.ResourceSet{AIGateways: []resources.AIGatewayResource{gateway}}

	assert.Equal(t, []string{resources.NamespaceExternal}, planner.getResourceNamespaces(rs))
}
