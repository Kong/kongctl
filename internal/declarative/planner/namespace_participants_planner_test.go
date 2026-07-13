package planner

import (
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/stretchr/testify/assert"
)

func TestGetResourceNamespacesExternalAIGatewayMapsToExternal(t *testing.T) {
	planner := &Planner{}
	gateway := resources.AIGatewayResource{External: &resources.ExternalBlock{ID: "ext-gateway"}}
	gateway.Ref = "gateway-1"

	rs := &resources.ResourceSet{AIGateways: []resources.AIGatewayResource{gateway}}

	assert.Equal(t, []string{resources.NamespaceExternal}, planner.getResourceNamespaces(rs))
}
