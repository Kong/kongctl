package helpers

import (
	"context"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

// AIGatewayModelAPI defines the interface for AI Gateway model operations needed by kongctl.
type AIGatewayModelAPI interface {
	ListAiGatewayModels(
		ctx context.Context,
		request kkOps.ListAiGatewayModelsRequest,
		opts ...kkOps.Option,
	) (*kkOps.ListAiGatewayModelsResponse, error)
	CreateAiGatewayModel(
		ctx context.Context,
		gatewayID string,
		request kkComps.CreateAIGatewayModelRequest,
		opts ...kkOps.Option,
	) (*kkOps.CreateAiGatewayModelResponse, error)
	GetAiGatewayModel(
		ctx context.Context,
		gatewayID string,
		modelID string,
		opts ...kkOps.Option,
	) (*kkOps.GetAiGatewayModelResponse, error)
	UpdateAiGatewayModel(
		ctx context.Context,
		request kkOps.UpdateAiGatewayModelRequest,
		opts ...kkOps.Option,
	) (*kkOps.UpdateAiGatewayModelResponse, error)
	DeleteAiGatewayModel(
		ctx context.Context,
		gatewayID string,
		modelID string,
		opts ...kkOps.Option,
	) (*kkOps.DeleteAiGatewayModelResponse, error)
}

// AIGatewayModelAPIImpl provides the real SDK implementation.
type AIGatewayModelAPIImpl struct {
	SDK *kkSDK.SDK
}

func (a *AIGatewayModelAPIImpl) ListAiGatewayModels(
	ctx context.Context,
	request kkOps.ListAiGatewayModelsRequest,
	opts ...kkOps.Option,
) (*kkOps.ListAiGatewayModelsResponse, error) {
	return a.SDK.AIGatewayModels.ListAiGatewayModels(ctx, request, opts...)
}

func (a *AIGatewayModelAPIImpl) CreateAiGatewayModel(
	ctx context.Context,
	gatewayID string,
	request kkComps.CreateAIGatewayModelRequest,
	opts ...kkOps.Option,
) (*kkOps.CreateAiGatewayModelResponse, error) {
	return a.SDK.AIGatewayModels.CreateAiGatewayModel(ctx, gatewayID, request, opts...)
}

func (a *AIGatewayModelAPIImpl) GetAiGatewayModel(
	ctx context.Context,
	gatewayID string,
	modelID string,
	opts ...kkOps.Option,
) (*kkOps.GetAiGatewayModelResponse, error) {
	return a.SDK.AIGatewayModels.GetAiGatewayModel(ctx, gatewayID, modelID, opts...)
}

func (a *AIGatewayModelAPIImpl) UpdateAiGatewayModel(
	ctx context.Context,
	request kkOps.UpdateAiGatewayModelRequest,
	opts ...kkOps.Option,
) (*kkOps.UpdateAiGatewayModelResponse, error) {
	return a.SDK.AIGatewayModels.UpdateAiGatewayModel(ctx, request, opts...)
}

func (a *AIGatewayModelAPIImpl) DeleteAiGatewayModel(
	ctx context.Context,
	gatewayID string,
	modelID string,
	opts ...kkOps.Option,
) (*kkOps.DeleteAiGatewayModelResponse, error) {
	return a.SDK.AIGatewayModels.DeleteAiGatewayModel(ctx, gatewayID, modelID, opts...)
}
