package helpers

import (
	"context"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

// AIGatewayAuthStrategiesAPI defines the interface for AI Gateway Auth Strategy operations needed by kongctl.
type AIGatewayAuthStrategiesAPI interface {
	ListAiGatewayAuthStrategies(
		ctx context.Context,
		request kkOps.ListAiGatewayAuthStrategiesRequest,
		opts ...kkOps.Option,
	) (*kkOps.ListAiGatewayAuthStrategiesResponse, error)
	CreateAiGatewayAuthStrategy(
		ctx context.Context,
		gatewayID string,
		request kkComps.CreateAIGatewayAuthStrategyRequest,
		opts ...kkOps.Option,
	) (*kkOps.CreateAiGatewayAuthStrategyResponse, error)
	GetAiGatewayAuthStrategy(
		ctx context.Context,
		gatewayID string,
		authStrategyID string,
		opts ...kkOps.Option,
	) (*kkOps.GetAiGatewayAuthStrategyResponse, error)
	UpdateAiGatewayAuthStrategy(
		ctx context.Context,
		request kkOps.UpdateAiGatewayAuthStrategyRequest,
		opts ...kkOps.Option,
	) (*kkOps.UpdateAiGatewayAuthStrategyResponse, error)
	DeleteAiGatewayAuthStrategy(
		ctx context.Context,
		gatewayID string,
		authStrategyID string,
		opts ...kkOps.Option,
	) (*kkOps.DeleteAiGatewayAuthStrategyResponse, error)
}

// AIGatewayAuthStrategiesAPIImpl provides the real SDK implementation.
type AIGatewayAuthStrategiesAPIImpl struct {
	SDK *kkSDK.SDK
}

func (a *AIGatewayAuthStrategiesAPIImpl) ListAiGatewayAuthStrategies(
	ctx context.Context,
	request kkOps.ListAiGatewayAuthStrategiesRequest,
	opts ...kkOps.Option,
) (*kkOps.ListAiGatewayAuthStrategiesResponse, error) {
	return a.SDK.AIGatewayAuthStrategies.ListAiGatewayAuthStrategies(ctx, request, opts...)
}

func (a *AIGatewayAuthStrategiesAPIImpl) CreateAiGatewayAuthStrategy(
	ctx context.Context,
	gatewayID string,
	request kkComps.CreateAIGatewayAuthStrategyRequest,
	opts ...kkOps.Option,
) (*kkOps.CreateAiGatewayAuthStrategyResponse, error) {
	return a.SDK.AIGatewayAuthStrategies.CreateAiGatewayAuthStrategy(ctx, gatewayID, request, opts...)
}

func (a *AIGatewayAuthStrategiesAPIImpl) GetAiGatewayAuthStrategy(
	ctx context.Context,
	gatewayID string,
	authStrategyID string,
	opts ...kkOps.Option,
) (*kkOps.GetAiGatewayAuthStrategyResponse, error) {
	return a.SDK.AIGatewayAuthStrategies.GetAiGatewayAuthStrategy(ctx, gatewayID, authStrategyID, opts...)
}

func (a *AIGatewayAuthStrategiesAPIImpl) UpdateAiGatewayAuthStrategy(
	ctx context.Context,
	request kkOps.UpdateAiGatewayAuthStrategyRequest,
	opts ...kkOps.Option,
) (*kkOps.UpdateAiGatewayAuthStrategyResponse, error) {
	return a.SDK.AIGatewayAuthStrategies.UpdateAiGatewayAuthStrategy(ctx, request, opts...)
}

func (a *AIGatewayAuthStrategiesAPIImpl) DeleteAiGatewayAuthStrategy(
	ctx context.Context,
	gatewayID string,
	authStrategyID string,
	opts ...kkOps.Option,
) (*kkOps.DeleteAiGatewayAuthStrategyResponse, error) {
	return a.SDK.AIGatewayAuthStrategies.DeleteAiGatewayAuthStrategy(ctx, gatewayID, authStrategyID, opts...)
}
