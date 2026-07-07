package helpers

import (
	"context"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

// AIGatewayProvidersAPI defines the interface for AI Gateway Model Provider operations needed by kongctl.
type AIGatewayProvidersAPI interface {
	ListAiGatewayProviders(
		ctx context.Context,
		request kkOps.ListAiGatewayModelProvidersRequest,
		opts ...kkOps.Option,
	) (*kkOps.ListAiGatewayModelProvidersResponse, error)
	CreateAiGatewayProvider(
		ctx context.Context,
		gatewayID string,
		request kkComps.CreateAIGatewayModelProviderRequest,
		opts ...kkOps.Option,
	) (*kkOps.CreateAiGatewayModelProviderResponse, error)
	GetAiGatewayProvider(
		ctx context.Context,
		gatewayID string,
		providerID string,
		opts ...kkOps.Option,
	) (*kkOps.GetAiGatewayModelProviderResponse, error)
	UpdateAiGatewayProvider(
		ctx context.Context,
		request kkOps.UpdateAiGatewayModelProviderRequest,
		opts ...kkOps.Option,
	) (*kkOps.UpdateAiGatewayModelProviderResponse, error)
	DeleteAiGatewayProvider(
		ctx context.Context,
		gatewayID string,
		providerID string,
		opts ...kkOps.Option,
	) (*kkOps.DeleteAiGatewayModelProviderResponse, error)
}

// AIGatewayProvidersAPIImpl provides the real SDK implementation.
type AIGatewayProvidersAPIImpl struct {
	SDK *kkSDK.SDK
}

func (a *AIGatewayProvidersAPIImpl) ListAiGatewayProviders(
	ctx context.Context,
	request kkOps.ListAiGatewayModelProvidersRequest,
	opts ...kkOps.Option,
) (*kkOps.ListAiGatewayModelProvidersResponse, error) {
	return a.SDK.AIGatewayModelProviders.ListAiGatewayModelProviders(ctx, request, opts...)
}

func (a *AIGatewayProvidersAPIImpl) CreateAiGatewayProvider(
	ctx context.Context,
	gatewayID string,
	request kkComps.CreateAIGatewayModelProviderRequest,
	opts ...kkOps.Option,
) (*kkOps.CreateAiGatewayModelProviderResponse, error) {
	return a.SDK.AIGatewayModelProviders.CreateAiGatewayModelProvider(ctx, gatewayID, request, opts...)
}

func (a *AIGatewayProvidersAPIImpl) GetAiGatewayProvider(
	ctx context.Context,
	gatewayID string,
	providerID string,
	opts ...kkOps.Option,
) (*kkOps.GetAiGatewayModelProviderResponse, error) {
	return a.SDK.AIGatewayModelProviders.GetAiGatewayModelProvider(ctx, gatewayID, providerID, opts...)
}

func (a *AIGatewayProvidersAPIImpl) UpdateAiGatewayProvider(
	ctx context.Context,
	request kkOps.UpdateAiGatewayModelProviderRequest,
	opts ...kkOps.Option,
) (*kkOps.UpdateAiGatewayModelProviderResponse, error) {
	return a.SDK.AIGatewayModelProviders.UpdateAiGatewayModelProvider(ctx, request, opts...)
}

func (a *AIGatewayProvidersAPIImpl) DeleteAiGatewayProvider(
	ctx context.Context,
	gatewayID string,
	providerID string,
	opts ...kkOps.Option,
) (*kkOps.DeleteAiGatewayModelProviderResponse, error) {
	return a.SDK.AIGatewayModelProviders.DeleteAiGatewayModelProvider(ctx, gatewayID, providerID, opts...)
}
