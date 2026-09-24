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
