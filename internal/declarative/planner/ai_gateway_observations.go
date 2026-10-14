package planner

import (
	"context"

	"github.com/kong/kongctl/internal/declarative/state"
)

// Reuse observations within one GeneratePlan call, including empty collections.
// Unscoped collections are still fetched when dependency validation needs them.
func listCachedAIGatewayChildren[T any](
	ctx context.Context, gatewayID string, cache map[string][]T,
	fetch func(context.Context, string) ([]T, error),
) ([]T, error) {
	if values, found := cache[gatewayID]; found {
		return values, nil
	}
	values, err := fetch(ctx, gatewayID)
	if err != nil {
		return nil, err
	}
	if cache != nil {
		cache[gatewayID] = values
	}
	return values, nil
}

func (p *Planner) listAIGatewayAgents(ctx context.Context, gatewayID string) ([]state.AIGatewayAgent, error) {
	var cache map[string][]state.AIGatewayAgent
	if p.resourceCache != nil {
		cache = p.resourceCache.aiGatewayAgents
	}
	return listCachedAIGatewayChildren(ctx, gatewayID, cache, p.client.ListAIGatewayAgents)
}

func (p *Planner) listAIGatewayConsumers(ctx context.Context, gatewayID string) ([]state.AIGatewayConsumer, error) {
	var cache map[string][]state.AIGatewayConsumer
	if p.resourceCache != nil {
		cache = p.resourceCache.aiGatewayConsumers
	}
	return listCachedAIGatewayChildren(ctx, gatewayID, cache, p.client.ListAIGatewayConsumers)
}

func (p *Planner) listAIGatewayConsumerGroups(
	ctx context.Context, gatewayID string,
) ([]state.AIGatewayConsumerGroup, error) {
	var cache map[string][]state.AIGatewayConsumerGroup
	if p.resourceCache != nil {
		cache = p.resourceCache.aiGatewayConsumerGroups
	}
	return listCachedAIGatewayChildren(ctx, gatewayID, cache, p.client.ListAIGatewayConsumerGroups)
}

func (p *Planner) listAIGatewayModels(ctx context.Context, gatewayID string) ([]state.AIGatewayModel, error) {
	var cache map[string][]state.AIGatewayModel
	if p.resourceCache != nil {
		cache = p.resourceCache.aiGatewayModels
	}
	return listCachedAIGatewayChildren(ctx, gatewayID, cache, p.client.ListAIGatewayModels)
}

func (p *Planner) listAIGatewayMCPServers(ctx context.Context, gatewayID string) ([]state.AIGatewayMCPServer, error) {
	var cache map[string][]state.AIGatewayMCPServer
	if p.resourceCache != nil {
		cache = p.resourceCache.aiGatewayMCPServers
	}
	return listCachedAIGatewayChildren(ctx, gatewayID, cache, p.client.ListAIGatewayMCPServers)
}

func (p *Planner) listAIGatewayPolicies(ctx context.Context, gatewayID string) ([]state.AIGatewayPolicy, error) {
	var cache map[string][]state.AIGatewayPolicy
	if p.resourceCache != nil {
		cache = p.resourceCache.aiGatewayPolicies
	}
	return listCachedAIGatewayChildren(ctx, gatewayID, cache, p.client.ListAIGatewayPolicies)
}

func (p *Planner) listAIGatewayProviders(ctx context.Context, gatewayID string) ([]state.AIGatewayProvider, error) {
	var cache map[string][]state.AIGatewayProvider
	if p.resourceCache != nil {
		cache = p.resourceCache.aiGatewayProviders
	}
	return listCachedAIGatewayChildren(ctx, gatewayID, cache, p.client.ListAIGatewayProviders)
}

func (p *Planner) listAIGatewayAuthStrategies(ctx context.Context,
	gatewayID string) ([]state.AIGatewayAuthStrategy,
	error,
) {
	var cache map[string][]state.AIGatewayAuthStrategy
	if p.resourceCache != nil {
		cache = p.resourceCache.aiGatewayAuthStrategies
	}
	return listCachedAIGatewayChildren(ctx, gatewayID, cache, p.client.ListAIGatewayAuthStrategies)
}

func (p *Planner) listAIGatewayCustomPolicies(ctx context.Context,
	gatewayID string) ([]state.AIGatewayCustomPolicy,
	error,
) {
	var cache map[string][]state.AIGatewayCustomPolicy
	if p.resourceCache != nil {
		cache = p.resourceCache.aiGatewayCustomPolicies
	}
	return listCachedAIGatewayChildren(ctx, gatewayID, cache, p.client.ListAIGatewayCustomPolicies)
}

func (p *Planner) listAIGatewayVaults(ctx context.Context, gatewayID string) ([]state.AIGatewayVault, error) {
	var cache map[string][]state.AIGatewayVault
	if p.resourceCache != nil {
		cache = p.resourceCache.aiGatewayVaults
	}
	return listCachedAIGatewayChildren(ctx, gatewayID, cache, p.client.ListAIGatewayVaults)
}

func (p *Planner) listAIGatewayCertificates(ctx context.Context,
	gatewayID string) ([]state.AIGatewayCertificate,
	error,
) {
	var cache map[string][]state.AIGatewayCertificate
	if p.resourceCache != nil {
		cache = p.resourceCache.aiGatewayCertificates
	}
	return listCachedAIGatewayChildren(ctx, gatewayID, cache, p.client.ListAIGatewayCertificates)
}

func (p *Planner) listAIGatewayCACertificates(ctx context.Context,
	gatewayID string) ([]state.AIGatewayCACertificate,
	error,
) {
	var cache map[string][]state.AIGatewayCACertificate
	if p.resourceCache != nil {
		cache = p.resourceCache.aiGatewayCACertificates
	}
	return listCachedAIGatewayChildren(ctx, gatewayID, cache, p.client.ListAIGatewayCACertificates)
}

func (p *Planner) listAIGatewaySNIs(ctx context.Context, gatewayID string) ([]state.AIGatewaySNI, error) {
	var cache map[string][]state.AIGatewaySNI
	if p.resourceCache != nil {
		cache = p.resourceCache.aiGatewaySNIs
	}
	return listCachedAIGatewayChildren(ctx, gatewayID, cache, p.client.ListAIGatewaySNIs)
}
