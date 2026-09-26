package dump

import (
	"context"

	declresources "github.com/kong/kongctl/internal/declarative/resources"
)

type aiGatewayChildCollector = childCollector[declresources.AIGatewayResource]

var aiGatewayChildCollectors = buildAIGatewayChildCollectors()

func buildAIGatewayChildCollectors() []aiGatewayChildCollector {
	// Preserve API request order, including reads performed by nested exporters.
	collectors := []aiGatewayChildCollector{
		childCollection(
			"failed to load AI Gateway Model Providers",
			func(ctx context.Context, d *childDumpContext) ([]declresources.AIGatewayProviderResource, error) {
				return buildAIGatewayProviders(ctx, d.logger, d.client, d.parentID, d.parentName)
			},
			func(g *declresources.AIGatewayResource) *[]declresources.AIGatewayProviderResource {
				return &g.Providers
			},
		),
		childCollection(
			"failed to load AI Gateway Auth Strategies",
			func(ctx context.Context, d *childDumpContext) ([]declresources.AIGatewayAuthStrategyResource, error) {
				return buildAIGatewayAuthStrategies(ctx, d.logger, d.client, d.parentID, d.parentName)
			},
			func(g *declresources.AIGatewayResource) *[]declresources.AIGatewayAuthStrategyResource {
				return &g.AuthStrategies
			},
		),
		childCollection(
			"failed to load AI Gateway Custom Policies",
			func(ctx context.Context, d *childDumpContext) ([]declresources.AIGatewayCustomPolicyResource, error) {
				return buildAIGatewayCustomPolicies(ctx, d.client, d.parentID, d.parentName, "")
			},
			func(g *declresources.AIGatewayResource) *[]declresources.AIGatewayCustomPolicyResource {
				return &g.CustomPolicies
			},
		),

		childCollection(
			"failed to load AI Gateway Policies",
			func(ctx context.Context, d *childDumpContext) ([]declresources.AIGatewayPolicyResource, error) {
				return buildAIGatewayPolicies(ctx, d.client, d.parentID, d.parentName, "")
			},
			func(g *declresources.AIGatewayResource) *[]declresources.AIGatewayPolicyResource { return &g.Policies },
		),
		childCollection(
			"failed to load AI Gateway Agents",
			func(ctx context.Context, d *childDumpContext) ([]declresources.AIGatewayAgentResource, error) {
				return buildAIGatewayAgents(ctx, d.client, d.parentID, d.parentName, "")
			},
			func(g *declresources.AIGatewayResource) *[]declresources.AIGatewayAgentResource { return &g.Agents },
		),
		childCollection(
			"failed to load AI Gateway Consumers",
			func(ctx context.Context, d *childDumpContext) ([]declresources.AIGatewayConsumerResource, error) {
				return buildAIGatewayConsumers(ctx, d.client, d.parentID, d.parentName, "", true)
			},
			func(g *declresources.AIGatewayResource) *[]declresources.AIGatewayConsumerResource {
				return &g.Consumers
			},
			declresources.ResourceTypeAIGatewayConsumerCredential,
		),
		childCollection(
			"failed to load AI Gateway Consumer Groups",
			func(ctx context.Context, d *childDumpContext) ([]declresources.AIGatewayConsumerGroupResource, error) {
				return buildAIGatewayConsumerGroups(ctx, d.client, d.parentID, d.parentName, "")
			},
			func(g *declresources.AIGatewayResource) *[]declresources.AIGatewayConsumerGroupResource {
				return &g.ConsumerGroups
			},
		),
		childCollection(
			"failed to load AI Gateway models",
			func(ctx context.Context, d *childDumpContext) ([]declresources.AIGatewayModelResource, error) {
				return buildAIGatewayModels(ctx, d.client, d.parentID, d.parentName, "")
			},
			func(g *declresources.AIGatewayResource) *[]declresources.AIGatewayModelResource { return &g.Models },
		),
		childCollection(
			"failed to load AI Gateway MCP Servers",
			func(ctx context.Context, d *childDumpContext) ([]declresources.AIGatewayMCPServerResource, error) {
				return buildAIGatewayMCPServers(ctx, d.client, d.parentID, d.parentName, "")
			},
			func(g *declresources.AIGatewayResource) *[]declresources.AIGatewayMCPServerResource {
				return &g.MCPServers
			},
		),
		childCollection(
			"failed to load AI Gateway Config Stores",
			func(ctx context.Context, d *childDumpContext) ([]declresources.AIGatewayConfigStoreResource, error) {
				return buildAIGatewayConfigStores(ctx, d.client, d.parentID, "")
			},
			func(g *declresources.AIGatewayResource) *[]declresources.AIGatewayConfigStoreResource {
				return &g.ConfigStores
			},
			declresources.ResourceTypeAIGatewayConfigStoreSecret,
		),
		childCollection(
			"failed to load AI Gateway Vaults",
			func(ctx context.Context, d *childDumpContext) ([]declresources.AIGatewayVaultResource, error) {
				return buildAIGatewayVaults(ctx, d.client, d.parentID, d.parentName, "")
			},
			func(g *declresources.AIGatewayResource) *[]declresources.AIGatewayVaultResource { return &g.Vaults },
		),
		childCollection(
			"failed to load AI Gateway data plane certificates",
			func(
				ctx context.Context, d *childDumpContext,
			) ([]declresources.AIGatewayDataPlaneCertificateResource, error) {
				return buildAIGatewayDataPlaneCertificates(ctx, d.logger, d.client, d.parentID, d.parentName, "")
			},
			func(g *declresources.AIGatewayResource) *[]declresources.AIGatewayDataPlaneCertificateResource {
				return &g.DataPlaneCertificates
			},
		),
		childCollection(
			"failed to load AI Gateway certificates",
			func(ctx context.Context, d *childDumpContext) ([]declresources.AIGatewayCertificateResource, error) {
				return buildAIGatewayCertificates(ctx, d.client, d.parentID, "")
			},
			func(g *declresources.AIGatewayResource) *[]declresources.AIGatewayCertificateResource {
				return &g.Certificates
			},
		),
		childCollection(
			"failed to load AI Gateway CA certificates",
			func(ctx context.Context, d *childDumpContext) ([]declresources.AIGatewayCACertificateResource, error) {
				return buildAIGatewayCACertificates(ctx, d.client, d.parentID, "")
			},
			func(g *declresources.AIGatewayResource) *[]declresources.AIGatewayCACertificateResource {
				return &g.CACertificates
			},
		),
		childCollection(
			"failed to load AI Gateway SNIs",
			func(ctx context.Context, d *childDumpContext) ([]declresources.AIGatewaySNIResource, error) {
				return buildAIGatewaySNIs(ctx, d.client, d.parentID, "")
			},
			func(g *declresources.AIGatewayResource) *[]declresources.AIGatewaySNIResource { return &g.SNIs },
		),
	}
	if err := validateAIGatewayChildCollectors(collectors); err != nil {
		panic(err)
	}
	return collectors
}

func validateAIGatewayChildCollectors(collectors []aiGatewayChildCollector) error {
	return validateChildCollectors("AI Gateway child dump", collectors)
}
