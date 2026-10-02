package planner

import "context"

type aiGatewayDeleteResolver struct {
	targetType string
	resolve    func(*Planner, context.Context, string, string, string, *Plan) error
}

// Keep adapter dispatch ordered to preserve validation diagnostics. Custom
// policies also supply creation edges when there are no deletions.
var aiGatewayDeleteResolvers = []aiGatewayDeleteResolver{
	{ResourceTypeAIGatewayProvider, (*Planner).resolveAIGatewayProviderDeletes},
	{ResourceTypeAIGatewayPolicy, (*Planner).resolveAIGatewayPolicyDeletes},
	{ResourceTypeAIGatewayAuthStrategy, (*Planner).resolveAIGatewayAuthStrategyDeletes},
	{ResourceTypeAIGatewayCustomPolicy, (*Planner).resolveAIGatewayCustomPolicyDependencies},
	{ResourceTypeAIGatewayMCPServer, (*Planner).resolveAIGatewayMCPSourceDeletes},
}

func (p *Planner) resolveAIGatewayReferenceDeletes(
	ctx context.Context, namespace, gatewayRef, gatewayID string, plan *Plan,
) error {
	for _, policy := range aiGatewayDeleteResolvers {
		if err := policy.resolve(p, ctx, namespace, gatewayRef, gatewayID, plan); err != nil {
			return err
		}
	}
	return nil
}
