package helpers

import (
	"context"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

// AIGatewayProvidersAPI defines the interface for AI Gateway Provider operations needed by kongctl.
type AIGatewayProvidersAPI interface {
	ListAiGatewayProviders(
		ctx context.Context,
		request kkOps.ListAiGatewayProvidersRequest,
		opts ...kkOps.Option,
	) (*kkOps.ListAiGatewayProvidersResponse, error)
	CreateAiGatewayProvider(
		ctx context.Context,
		gatewayID string,
		request kkComps.CreateAIGatewayProviderRequest,
		opts ...kkOps.Option,
	) (*kkOps.CreateAiGatewayProviderResponse, error)
	GetAiGatewayProvider(
		ctx context.Context,
		gatewayID string,
		providerID string,
		opts ...kkOps.Option,
	) (*kkOps.GetAiGatewayProviderResponse, error)
	UpdateAiGatewayProvider(
		ctx context.Context,
		request kkOps.UpdateAiGatewayProviderRequest,
		opts ...kkOps.Option,
	) (*kkOps.UpdateAiGatewayProviderResponse, error)
	DeleteAiGatewayProvider(
		ctx context.Context,
		gatewayID string,
		providerID string,
		opts ...kkOps.Option,
	) (*kkOps.DeleteAiGatewayProviderResponse, error)
}

// AIGatewayProvidersAPIImpl provides the real SDK implementation.
type AIGatewayProvidersAPIImpl struct {
	SDK *kkSDK.SDK
}

func (a *AIGatewayProvidersAPIImpl) ListAiGatewayProviders(
	ctx context.Context,
	request kkOps.ListAiGatewayProvidersRequest,
	opts ...kkOps.Option,
) (*kkOps.ListAiGatewayProvidersResponse, error) {
	return a.SDK.AIGatewayProviders.ListAiGatewayProviders(ctx, request, opts...)
}

func (a *AIGatewayProvidersAPIImpl) CreateAiGatewayProvider(
	ctx context.Context,
	gatewayID string,
	request kkComps.CreateAIGatewayProviderRequest,
	opts ...kkOps.Option,
) (*kkOps.CreateAiGatewayProviderResponse, error) {
	return a.SDK.AIGatewayProviders.CreateAiGatewayProvider(ctx, gatewayID, request, opts...)
}

func (a *AIGatewayProvidersAPIImpl) GetAiGatewayProvider(
	ctx context.Context,
	gatewayID string,
	providerID string,
	opts ...kkOps.Option,
) (*kkOps.GetAiGatewayProviderResponse, error) {
	return a.SDK.AIGatewayProviders.GetAiGatewayProvider(ctx, gatewayID, providerID, opts...)
}

func (a *AIGatewayProvidersAPIImpl) UpdateAiGatewayProvider(
	ctx context.Context,
	request kkOps.UpdateAiGatewayProviderRequest,
	opts ...kkOps.Option,
) (*kkOps.UpdateAiGatewayProviderResponse, error) {
	return a.SDK.AIGatewayProviders.UpdateAiGatewayProvider(ctx, request, opts...)
}

func (a *AIGatewayProvidersAPIImpl) DeleteAiGatewayProvider(
	ctx context.Context,
	gatewayID string,
	providerID string,
	opts ...kkOps.Option,
) (*kkOps.DeleteAiGatewayProviderResponse, error) {
	return a.SDK.AIGatewayProviders.DeleteAiGatewayProvider(ctx, gatewayID, providerID, opts...)
}
