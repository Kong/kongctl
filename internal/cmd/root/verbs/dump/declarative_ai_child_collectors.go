package dump

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	declresources "github.com/kong/kongctl/internal/declarative/resources"
	declstate "github.com/kong/kongctl/internal/declarative/state"
)

type aiGatewayChildDumpContext struct {
	logger      *slog.Logger
	client      *declstate.Client
	gatewayID   string
	gatewayName string
}

type aiGatewayChildCollector struct {
	kind    declresources.ResourceType
	warning string
	collect func(context.Context, *aiGatewayChildDumpContext, *declresources.AIGatewayResource) error
	// These kinds are exported inside this collector, not by separate gateway calls.
	nested []declresources.ResourceType
}

var aiGatewayChildCollectors = buildAIGatewayChildCollectors()

func buildAIGatewayChildCollectors() []aiGatewayChildCollector {
	// Preserve API request order, including reads performed by nested exporters.
	collectors := []aiGatewayChildCollector{
		aiGatewayChild(
			"failed to load AI Gateway Model Providers",
			func(ctx context.Context, d *aiGatewayChildDumpContext) ([]declresources.AIGatewayProviderResource, error) {
				return buildAIGatewayProviders(ctx, d.logger, d.client, d.gatewayID, d.gatewayName)
			},
			func(g *declresources.AIGatewayResource) *[]declresources.AIGatewayProviderResource {
				return &g.Providers
			},
		),
		aiGatewayChild(
			"failed to load AI Gateway Auth Strategies",
			func(ctx context.Context, d *aiGatewayChildDumpContext) ([]declresources.AIGatewayAuthStrategyResource, error) {
				return buildAIGatewayAuthStrategies(ctx, d.logger, d.client, d.gatewayID, d.gatewayName)
			},
			func(g *declresources.AIGatewayResource) *[]declresources.AIGatewayAuthStrategyResource {
				return &g.AuthStrategies
			},
		),
		aiGatewayChild(
			"failed to load AI Gateway Policies",
			func(ctx context.Context, d *aiGatewayChildDumpContext) ([]declresources.AIGatewayPolicyResource, error) {
				return buildAIGatewayPolicies(ctx, d.client, d.gatewayID, d.gatewayName, "")
			},
			func(g *declresources.AIGatewayResource) *[]declresources.AIGatewayPolicyResource { return &g.Policies },
		),
		aiGatewayChild(
			"failed to load AI Gateway Agents",
			func(ctx context.Context, d *aiGatewayChildDumpContext) ([]declresources.AIGatewayAgentResource, error) {
				return buildAIGatewayAgents(ctx, d.client, d.gatewayID, d.gatewayName, "")
			},
			func(g *declresources.AIGatewayResource) *[]declresources.AIGatewayAgentResource { return &g.Agents },
		),
		aiGatewayChild(
			"failed to load AI Gateway Consumers",
			func(ctx context.Context, d *aiGatewayChildDumpContext) ([]declresources.AIGatewayConsumerResource, error) {
				return buildAIGatewayConsumers(ctx, d.client, d.gatewayID, d.gatewayName, "", true)
			},
			func(g *declresources.AIGatewayResource) *[]declresources.AIGatewayConsumerResource {
				return &g.Consumers
			},
			declresources.ResourceTypeAIGatewayConsumerCredential,
		),
		aiGatewayChild(
			"failed to load AI Gateway Consumer Groups",
			func(ctx context.Context, d *aiGatewayChildDumpContext) ([]declresources.AIGatewayConsumerGroupResource, error) {
				return buildAIGatewayConsumerGroups(ctx, d.client, d.gatewayID, d.gatewayName, "")
			},
			func(g *declresources.AIGatewayResource) *[]declresources.AIGatewayConsumerGroupResource {
				return &g.ConsumerGroups
			},
		),
		aiGatewayChild(
			"failed to load AI Gateway models",
			func(ctx context.Context, d *aiGatewayChildDumpContext) ([]declresources.AIGatewayModelResource, error) {
				return buildAIGatewayModels(ctx, d.client, d.gatewayID, d.gatewayName, "")
			},
			func(g *declresources.AIGatewayResource) *[]declresources.AIGatewayModelResource { return &g.Models },
		),
		aiGatewayChild(
			"failed to load AI Gateway MCP Servers",
			func(ctx context.Context, d *aiGatewayChildDumpContext) ([]declresources.AIGatewayMCPServerResource, error) {
				return buildAIGatewayMCPServers(ctx, d.client, d.gatewayID, d.gatewayName, "")
			},
			func(g *declresources.AIGatewayResource) *[]declresources.AIGatewayMCPServerResource {
				return &g.MCPServers
			},
		),
		aiGatewayChild(
			"failed to load AI Gateway Config Stores",
			func(ctx context.Context, d *aiGatewayChildDumpContext) ([]declresources.AIGatewayConfigStoreResource, error) {
				return buildAIGatewayConfigStores(ctx, d.client, d.gatewayID, "")
			},
			func(g *declresources.AIGatewayResource) *[]declresources.AIGatewayConfigStoreResource {
				return &g.ConfigStores
			},
			declresources.ResourceTypeAIGatewayConfigStoreSecret,
		),
		aiGatewayChild(
			"failed to load AI Gateway Vaults",
			func(ctx context.Context, d *aiGatewayChildDumpContext) ([]declresources.AIGatewayVaultResource, error) {
				return buildAIGatewayVaults(ctx, d.client, d.gatewayID, d.gatewayName, "")
			},
			func(g *declresources.AIGatewayResource) *[]declresources.AIGatewayVaultResource { return &g.Vaults },
		),
		aiGatewayChild(
			"failed to load AI Gateway data plane certificates",
			func(
				ctx context.Context, d *aiGatewayChildDumpContext,
			) ([]declresources.AIGatewayDataPlaneCertificateResource, error) {
				return buildAIGatewayDataPlaneCertificates(ctx, d.logger, d.client, d.gatewayID, d.gatewayName, "")
			},
			func(g *declresources.AIGatewayResource) *[]declresources.AIGatewayDataPlaneCertificateResource {
				return &g.DataPlaneCertificates
			},
		),
		aiGatewayChild(
			"failed to load AI Gateway certificates",
			func(ctx context.Context, d *aiGatewayChildDumpContext) ([]declresources.AIGatewayCertificateResource, error) {
				return buildAIGatewayCertificates(ctx, d.client, d.gatewayID, "")
			},
			func(g *declresources.AIGatewayResource) *[]declresources.AIGatewayCertificateResource {
				return &g.Certificates
			},
		),
		aiGatewayChild(
			"failed to load AI Gateway CA certificates",
			func(ctx context.Context, d *aiGatewayChildDumpContext) ([]declresources.AIGatewayCACertificateResource, error) {
				return buildAIGatewayCACertificates(ctx, d.client, d.gatewayID, "")
			},
			func(g *declresources.AIGatewayResource) *[]declresources.AIGatewayCACertificateResource {
				return &g.CACertificates
			},
		),
		aiGatewayChild(
			"failed to load AI Gateway SNIs",
			func(ctx context.Context, d *aiGatewayChildDumpContext) ([]declresources.AIGatewaySNIResource, error) {
				return buildAIGatewaySNIs(ctx, d.client, d.gatewayID, "")
			},
			func(g *declresources.AIGatewayResource) *[]declresources.AIGatewaySNIResource { return &g.SNIs },
		),
	}
	if err := validateAIGatewayChildCollectors(collectors); err != nil {
		panic(err)
	}
	return collectors
}

func aiGatewayChild[R any, RPtr interface {
	*R
	declresources.Resource
}](
	warning string,
	collect func(context.Context, *aiGatewayChildDumpContext) ([]R, error),
	destination func(*declresources.AIGatewayResource) *[]R,
	nested ...declresources.ResourceType,
) aiGatewayChildCollector {
	if collect == nil || destination == nil || strings.TrimSpace(warning) == "" {
		panic("AI Gateway child dump requires collection, storage, and a warning")
	}
	return aiGatewayChildCollector{
		kind:    RPtr(new(R)).GetType(),
		warning: warning,
		nested:  nested,
		collect: func(ctx context.Context, d *aiGatewayChildDumpContext, gateway *declresources.AIGatewayResource) error {
			values, err := collect(ctx, d)
			if err != nil {
				return err
			}
			if len(values) > 0 {
				*destination(gateway) = values
			}
			return nil
		},
	}
}

// Derive completeness from registered ownership, including grandchildren.
// SyncCollections validates owner chains before returning this metadata.
func validateAIGatewayChildCollectors(collectors []aiGatewayChildCollector) error {
	parents := make(map[declresources.ResourceType]declresources.ResourceType)
	for _, collection := range declresources.SyncCollections() {
		parents[collection.ResourceType] = collection.ParentType
	}
	seen := make(map[declresources.ResourceType]bool)
	record := func(kind, owner declresources.ResourceType) error {
		if parents[kind] != owner {
			return fmt.Errorf("AI Gateway child dump: %s is not a managed child of %s", kind, owner)
		}
		if seen[kind] {
			return fmt.Errorf("AI Gateway child dump registers %s more than once", kind)
		}
		seen[kind] = true
		return nil
	}
	for _, collector := range collectors {
		if collector.collect == nil || strings.TrimSpace(collector.warning) == "" {
			return fmt.Errorf("AI Gateway child dump requires a collector and warning for %s", collector.kind)
		}
		if err := record(collector.kind, declresources.ResourceTypeAIGateway); err != nil {
			return err
		}
		for _, kind := range collector.nested {
			if err := record(kind, collector.kind); err != nil {
				return err
			}
		}
	}
	var missing []declresources.ResourceType
	for kind := range parents {
		for owner := parents[kind]; owner != ""; owner = parents[owner] {
			if owner == declresources.ResourceTypeAIGateway {
				if !seen[kind] {
					missing = append(missing, kind)
				}
				break
			}
		}
	}
	slices.Sort(missing)
	if len(missing) != 0 {
		return fmt.Errorf("AI Gateway child dump is missing resource types %v", missing)
	}
	return nil
}
