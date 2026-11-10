package adopt

import (
	"fmt"

	"github.com/kong/kongctl/internal/declarative/resources"
)

type declarativeAdoptionShape struct {
	collection  []string
	parentField string
	resource    resources.Resource
}

// Project each supported adoption API onto its declaration, never its response
// schema. This excludes read-only metadata and independently owned descendants.
func adoptionExportShape(kind resources.ResourceType) (declarativeAdoptionShape, error) {
	//exhaustive:ignore
	switch kind {
	case resources.ResourceTypeAIGatewayModel:
		return declarativeAdoptionShape{
			collection:  []string{"ai_gateway_models"},
			parentField: resources.SchemaFieldAIGateway,
			resource:    &resources.AIGatewayModelResource{},
		}, nil
	case resources.ResourceTypeAIGatewayProvider:
		return declarativeAdoptionShape{
			collection:  []string{"ai_gateway_model_providers"},
			parentField: resources.SchemaFieldAIGateway,
			resource:    &resources.AIGatewayProviderResource{},
		}, nil
	case resources.ResourceTypeAIGatewayAuthStrategy:
		return declarativeAdoptionShape{
			collection:  []string{"ai_gateway_auth_strategies"},
			parentField: resources.SchemaFieldAIGateway,
			resource:    &resources.AIGatewayAuthStrategyResource{},
		}, nil
	case resources.ResourceTypeAIGatewayPolicy:
		return declarativeAdoptionShape{
			collection:  []string{"ai_gateway_policies"},
			parentField: resources.SchemaFieldAIGateway,
			resource:    &resources.AIGatewayPolicyResource{},
		}, nil
	case resources.ResourceTypeAIGatewayCustomPolicy:
		return declarativeAdoptionShape{
			collection:  []string{"ai_gateway_custom_policies"},
			parentField: resources.SchemaFieldAIGateway,
			resource:    &resources.AIGatewayCustomPolicyResource{},
		}, nil
	case resources.ResourceTypeAIGatewayAgent:
		return declarativeAdoptionShape{
			collection:  []string{"ai_gateway_agents"},
			parentField: resources.SchemaFieldAIGateway,
			resource:    &resources.AIGatewayAgentResource{},
		}, nil
	case resources.ResourceTypeAIGatewayMCPServer:
		return declarativeAdoptionShape{
			collection:  []string{"ai_gateway_mcp_servers"},
			parentField: resources.SchemaFieldAIGateway,
			resource:    &resources.AIGatewayMCPServerResource{},
		}, nil
	case resources.ResourceTypeAIGatewayVault:
		return declarativeAdoptionShape{
			collection:  []string{"ai_gateway_vaults"},
			parentField: resources.SchemaFieldAIGateway,
			resource:    &resources.AIGatewayVaultResource{},
		}, nil
	case resources.ResourceTypeAIGatewayConsumer:
		return declarativeAdoptionShape{
			collection:  []string{"ai_gateway_consumers"},
			parentField: resources.SchemaFieldAIGateway,
			resource:    &resources.AIGatewayConsumerResource{},
		}, nil
	case resources.ResourceTypeAIGatewayConsumerGroup:
		return declarativeAdoptionShape{
			collection:  []string{"ai_gateway_consumer_groups"},
			parentField: resources.SchemaFieldAIGateway,
			resource:    &resources.AIGatewayConsumerGroupResource{},
		}, nil
	case resources.ResourceTypeAIGatewayCACertificate:
		return declarativeAdoptionShape{
			collection:  []string{"ai_gateway_ca_certificates"},
			parentField: resources.SchemaFieldAIGateway,
			resource:    &resources.AIGatewayCACertificateResource{},
		}, nil
	case resources.ResourceTypeAIGatewaySNI:
		return declarativeAdoptionShape{
			collection:  []string{"ai_gateway_snis"},
			parentField: resources.SchemaFieldAIGateway,
			resource:    &resources.AIGatewaySNIResource{},
		}, nil
	case resources.ResourceTypeIdentityGroup:
		return declarativeAdoptionShape{
			collection:  []string{"identity", "groups"},
			parentField: "directory",
			resource:    &resources.IdentityGroupResource{},
		}, nil
	case resources.ResourceTypeIdentityPrincipal:
		return declarativeAdoptionShape{
			collection:  []string{"identity", "principals"},
			parentField: "directory",
			resource:    &resources.IdentityPrincipalResource{},
		}, nil
	case resources.ResourceTypeIdentityPrincipalIdentity:
		return declarativeAdoptionShape{
			collection:  []string{"identity", "identities"},
			parentField: "principal",
			resource:    &resources.IdentityPrincipalIdentityResource{},
		}, nil
	case resources.ResourceTypeIdentityBasicAuth:
		return declarativeAdoptionShape{
			collection:  []string{"identity", "basic_auths"},
			parentField: "principal",
			resource:    &resources.IdentityBasicAuthResource{},
		}, nil
	case resources.ResourceTypeIdentityAuthServerClient:
		return declarativeAdoptionShape{
			collection:  []string{"identity", "clients"},
			parentField: "auth_server",
			resource:    &resources.IdentityAuthServerClientResource{},
		}, nil
	default:
		return declarativeAdoptionShape{}, fmt.Errorf("declarative export is unavailable for %s", kind)
	}
}
