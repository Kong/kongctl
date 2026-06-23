package helpers

import (
	"context"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

// AIGatewayAPI defines the interface for AI Gateway operations needed by kongctl.
type AIGatewayAPI interface {
	ListAiGateways(
		ctx context.Context,
		pageSize *int64,
		pageNumber *int64,
		opts ...kkOps.Option,
	) (*kkOps.ListAiGatewaysResponse, error)
	CreateAiGateway(
		ctx context.Context,
		request kkComps.CreateAIGatewayRequest,
		opts ...kkOps.Option,
	) (*kkOps.CreateAiGatewayResponse, error)
	GetAiGateway(
		ctx context.Context,
		gatewayID string,
		opts ...kkOps.Option,
	) (*kkOps.GetAiGatewayResponse, error)
	UpdateAiGateway(
		ctx context.Context,
		gatewayID string,
		request kkComps.UpdateAIGatewayRequest,
		opts ...kkOps.Option,
	) (*kkOps.UpdateAiGatewayResponse, error)
	DeleteAiGateway(
		ctx context.Context,
		gatewayID string,
		opts ...kkOps.Option,
	) (*kkOps.DeleteAiGatewayResponse, error)
}

// AIGatewayAPIImpl provides the real SDK implementation.
type AIGatewayAPIImpl struct {
	SDK *kkSDK.SDK
}

func (a *AIGatewayAPIImpl) ListAiGateways(
	ctx context.Context,
	pageSize *int64,
	pageNumber *int64,
	opts ...kkOps.Option,
) (*kkOps.ListAiGatewaysResponse, error) {
	return a.SDK.AIGateways.ListAiGateways(ctx, pageSize, pageNumber, opts...)
}

func (a *AIGatewayAPIImpl) CreateAiGateway(
	ctx context.Context,
	request kkComps.CreateAIGatewayRequest,
	opts ...kkOps.Option,
) (*kkOps.CreateAiGatewayResponse, error) {
	return a.SDK.AIGateways.CreateAiGateway(ctx, request, opts...)
}

func (a *AIGatewayAPIImpl) GetAiGateway(
	ctx context.Context,
	gatewayID string,
	opts ...kkOps.Option,
) (*kkOps.GetAiGatewayResponse, error) {
	return a.SDK.AIGateways.GetAiGateway(ctx, gatewayID, opts...)
}

func (a *AIGatewayAPIImpl) UpdateAiGateway(
	ctx context.Context,
	gatewayID string,
	request kkComps.UpdateAIGatewayRequest,
	opts ...kkOps.Option,
) (*kkOps.UpdateAiGatewayResponse, error) {
	return a.SDK.AIGateways.UpdateAiGateway(ctx, gatewayID, request, opts...)
}

func (a *AIGatewayAPIImpl) DeleteAiGateway(
	ctx context.Context,
	gatewayID string,
	opts ...kkOps.Option,
) (*kkOps.DeleteAiGatewayResponse, error) {
	return a.SDK.AIGateways.DeleteAiGateway(ctx, gatewayID, opts...)
}
