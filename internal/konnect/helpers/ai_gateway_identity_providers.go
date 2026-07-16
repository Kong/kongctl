package helpers

import (
	"context"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

// AIGatewayIdentityProvidersAPI defines the interface for AI Gateway Identity Provider operations needed by kongctl.
type AIGatewayIdentityProvidersAPI interface {
	ListAiGatewayIdentityProviders(
		ctx context.Context,
		request kkOps.ListAiGatewayIdentityProvidersRequest,
		opts ...kkOps.Option,
	) (*kkOps.ListAiGatewayIdentityProvidersResponse, error)
	CreateAiGatewayIdentityProvider(
		ctx context.Context,
		gatewayID string,
		request kkComps.CreateAIGatewayIdentityProviderRequest,
		opts ...kkOps.Option,
	) (*kkOps.CreateAiGatewayIdentityProviderResponse, error)
	GetAiGatewayIdentityProvider(
		ctx context.Context,
		gatewayID string,
		identityProviderID string,
		opts ...kkOps.Option,
	) (*kkOps.GetAiGatewayIdentityProviderResponse, error)
	UpdateAiGatewayIdentityProvider(
		ctx context.Context,
		request kkOps.UpdateAiGatewayIdentityProviderRequest,
		opts ...kkOps.Option,
	) (*kkOps.UpdateAiGatewayIdentityProviderResponse, error)
	DeleteAiGatewayIdentityProvider(
		ctx context.Context,
		gatewayID string,
		identityProviderID string,
		opts ...kkOps.Option,
	) (*kkOps.DeleteAiGatewayIdentityProviderResponse, error)
}

// AIGatewayIdentityProvidersAPIImpl provides the real SDK implementation.
type AIGatewayIdentityProvidersAPIImpl struct {
	SDK *kkSDK.SDK
}

func (a *AIGatewayIdentityProvidersAPIImpl) ListAiGatewayIdentityProviders(
	ctx context.Context,
	request kkOps.ListAiGatewayIdentityProvidersRequest,
	opts ...kkOps.Option,
) (*kkOps.ListAiGatewayIdentityProvidersResponse, error) {
	return a.SDK.AIGatewayIdentityProviders.ListAiGatewayIdentityProviders(ctx, request, opts...)
}

func (a *AIGatewayIdentityProvidersAPIImpl) CreateAiGatewayIdentityProvider(
	ctx context.Context,
	gatewayID string,
	request kkComps.CreateAIGatewayIdentityProviderRequest,
	opts ...kkOps.Option,
) (*kkOps.CreateAiGatewayIdentityProviderResponse, error) {
	return a.SDK.AIGatewayIdentityProviders.CreateAiGatewayIdentityProvider(ctx, gatewayID, request, opts...)
}

func (a *AIGatewayIdentityProvidersAPIImpl) GetAiGatewayIdentityProvider(
	ctx context.Context,
	gatewayID string,
	identityProviderID string,
	opts ...kkOps.Option,
) (*kkOps.GetAiGatewayIdentityProviderResponse, error) {
	return a.SDK.AIGatewayIdentityProviders.GetAiGatewayIdentityProvider(ctx, gatewayID, identityProviderID, opts...)
}

func (a *AIGatewayIdentityProvidersAPIImpl) UpdateAiGatewayIdentityProvider(
	ctx context.Context,
	request kkOps.UpdateAiGatewayIdentityProviderRequest,
	opts ...kkOps.Option,
) (*kkOps.UpdateAiGatewayIdentityProviderResponse, error) {
	return a.SDK.AIGatewayIdentityProviders.UpdateAiGatewayIdentityProvider(ctx, request, opts...)
}

func (a *AIGatewayIdentityProvidersAPIImpl) DeleteAiGatewayIdentityProvider(
	ctx context.Context,
	gatewayID string,
	identityProviderID string,
	opts ...kkOps.Option,
) (*kkOps.DeleteAiGatewayIdentityProviderResponse, error) {
	return a.SDK.AIGatewayIdentityProviders.DeleteAiGatewayIdentityProvider(ctx, gatewayID, identityProviderID, opts...)
}
