package planner

import (
	"strings"
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/stretchr/testify/require"
)

// Descriptors describe reference binding, not all runtime relationships. Keep
// the name-based and membership relationships in the audit as well.
func TestAIGatewayForeignKeyDeleteCoverage(t *testing.T) {
	type relationship struct {
		user   resources.ResourceType
		field  string
		target resources.ResourceType
	}
	type disposition struct {
		adapter   string
		exception string
	}
	coverage := map[relationship]disposition{}
	for _, user := range []resources.ResourceType{
		resources.ResourceTypeAIGatewayAgent, resources.ResourceTypeAIGatewayModel,
		resources.ResourceTypeAIGatewayMCPServer,
	} {
		coverage[relationship{
			user,
			FieldAccess + "." + FieldAuthStrategies,
			resources.ResourceTypeAIGatewayAuthStrategy,
		}] = disposition{
			adapter: ResourceTypeAIGatewayAuthStrategy,
		}
	}
	coverage[relationship{
		resources.ResourceTypeAIGatewaySNI,
		FieldCertificate,
		resources.ResourceTypeAIGatewayCertificate,
	}] = disposition{
		exception: "TLS orchestration validates observed and planned SNIs before adding pending certificate deletes",
	}
	coverage[relationship{
		resources.ResourceTypeAIGatewayVault,
		FieldConfig + "." + FieldConfigStoreID,
		resources.ResourceTypeAIGatewayConfigStore,
	}] = disposition{
		exception: "Existing gap: Konnect vault config-store deletion needs variant-specific observation and detachment; " +
			"preserve its lifecycle in this migration",
	}
	for _, user := range []resources.ResourceType{
		resources.ResourceTypeAIGatewayAgent, resources.ResourceTypeAIGatewayConsumer,
		resources.ResourceTypeAIGatewayConsumerGroup, resources.ResourceTypeAIGatewayModel,
		resources.ResourceTypeAIGatewayMCPServer,
	} {
		coverage[relationship{user, FieldPolicies, resources.ResourceTypeAIGatewayPolicy}] = disposition{
			adapter: ResourceTypeAIGatewayPolicy,
		}
	}
	for _, field := range []string{
		FieldTargets + "[]." + FieldProvider,
		FieldConfig + "." + FieldBalancer + "." + FieldEmbeddings + "." + FieldProvider,
	} {
		coverage[relationship{
			resources.ResourceTypeAIGatewayModel,
			field,
			resources.ResourceTypeAIGatewayProvider,
		}] = disposition{
			adapter: ResourceTypeAIGatewayProvider,
		}
	}
	coverage[relationship{
		resources.ResourceTypeAIGatewayPolicy,
		FieldType,
		resources.ResourceTypeAIGatewayCustomPolicy,
	}] = disposition{
		adapter: ResourceTypeAIGatewayCustomPolicy,
	}
	coverage[relationship{
		resources.ResourceTypeAIGatewayConsumerGroup,
		FieldConsumers,
		resources.ResourceTypeAIGatewayConsumer,
	}] = disposition{
		exception: "Separate membership endpoints, not the group PUT payload; " +
			"consumer deletion/membership reconciliation remains outside this foreign-key migration",
	}
	adapters := map[string]bool{}
	for _, policy := range aiGatewayDeleteResolvers {
		require.NotNil(t, policy.resolve)
		require.False(t, adapters[policy.targetType], "duplicate adapter %s", policy.targetType)
		adapters[policy.targetType] = true
	}
	for key, value := range coverage {
		require.NotEqual(t, value.adapter != "", value.exception != "", "ambiguous disposition for %+v", key)
		if value.adapter != "" {
			require.True(t, adapters[value.adapter], "unregistered adapter for %+v", key)
			require.Equal(t, string(key.target), value.adapter)
		}
	}
	for _, user := range resources.RegisteredTypes() {
		if string(user) != ResourceTypeAIGateway && !strings.HasPrefix(string(user), ResourceTypeAIGateway+"_") {
			continue
		}
		for _, descriptor := range resources.RelationshipDescriptorsForType(user) {
			if descriptor.Kind != resources.RelationshipKindAPIForeignKey {
				continue
			}
			targets := descriptor.TargetTypes
			if descriptor.TargetType != "" {
				targets = append(targets, descriptor.TargetType)
			}
			for _, target := range targets {
				key := relationship{user, descriptor.FieldPath, target}
				_, covered := coverage[key]
				require.True(
					t,
					covered,
					"AI Gateway foreign key needs a delete-ordering adapter or concrete exception: %+v",
					key,
				)
			}
		}
	}
}
