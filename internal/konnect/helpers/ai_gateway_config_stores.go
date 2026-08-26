package helpers

import (
	"context"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

// AIGatewayConfigStoresAPI defines the Config Store operations needed by kongctl.
type AIGatewayConfigStoresAPI interface {
	ListAiGatewayConfigStores(
		context.Context,
		kkOps.ListAiGatewayConfigStoresRequest,
		...kkOps.Option,
	) (*kkOps.ListAiGatewayConfigStoresResponse, error)
	CreateAiGatewayConfigStore(
		context.Context,
		string,
		kkComps.CreateAIGatewayConfigStoreRequest,
		...kkOps.Option,
	) (*kkOps.CreateAiGatewayConfigStoreResponse, error)
	GetAiGatewayConfigStore(
		context.Context,
		string,
		string,
		...kkOps.Option,
	) (*kkOps.GetAiGatewayConfigStoreResponse, error)
	UpdateAiGatewayConfigStore(
		context.Context,
		kkOps.UpdateAiGatewayConfigStoreRequest,
		...kkOps.Option,
	) (*kkOps.UpdateAiGatewayConfigStoreResponse, error)
	DeleteAiGatewayConfigStore(
		context.Context,
		kkOps.DeleteAiGatewayConfigStoreRequest,
		...kkOps.Option,
	) (*kkOps.DeleteAiGatewayConfigStoreResponse, error)
	ListAiGatewayConfigStoreSecrets(
		context.Context,
		kkOps.ListAiGatewayConfigStoreSecretsRequest,
		...kkOps.Option,
	) (*kkOps.ListAiGatewayConfigStoreSecretsResponse, error)
	CreateAiGatewayConfigStoreSecret(
		context.Context,
		kkOps.CreateAiGatewayConfigStoreSecretRequest,
		...kkOps.Option,
	) (*kkOps.CreateAiGatewayConfigStoreSecretResponse, error)
	GetAiGatewayConfigStoreSecret(
		context.Context,
		kkOps.GetAiGatewayConfigStoreSecretRequest,
		...kkOps.Option,
	) (*kkOps.GetAiGatewayConfigStoreSecretResponse, error)
	UpdateAiGatewayConfigStoreSecret(
		context.Context,
		kkOps.UpdateAiGatewayConfigStoreSecretRequest,
		...kkOps.Option,
	) (*kkOps.UpdateAiGatewayConfigStoreSecretResponse, error)
	DeleteAiGatewayConfigStoreSecret(
		context.Context,
		kkOps.DeleteAiGatewayConfigStoreSecretRequest,
		...kkOps.Option,
	) (*kkOps.DeleteAiGatewayConfigStoreSecretResponse, error)
}

// AIGatewayConfigStoresAPIImpl provides the SDK implementation.
type AIGatewayConfigStoresAPIImpl struct {
	SDK *kkSDK.SDK
}

func (a *AIGatewayConfigStoresAPIImpl) ListAiGatewayConfigStores(
	ctx context.Context,
	request kkOps.ListAiGatewayConfigStoresRequest,
	opts ...kkOps.Option,
) (*kkOps.ListAiGatewayConfigStoresResponse, error) {
	return a.SDK.AIGatewayConfigStores.ListAiGatewayConfigStores(ctx, request, opts...)
}

func (a *AIGatewayConfigStoresAPIImpl) CreateAiGatewayConfigStore(
	ctx context.Context,
	gatewayID string,
	request kkComps.CreateAIGatewayConfigStoreRequest,
	opts ...kkOps.Option,
) (*kkOps.CreateAiGatewayConfigStoreResponse, error) {
	return a.SDK.AIGatewayConfigStores.CreateAiGatewayConfigStore(ctx, gatewayID, request, opts...)
}

func (a *AIGatewayConfigStoresAPIImpl) GetAiGatewayConfigStore(
	ctx context.Context,
	gatewayID string,
	configStoreIDOrName string,
	opts ...kkOps.Option,
) (*kkOps.GetAiGatewayConfigStoreResponse, error) {
	return a.SDK.AIGatewayConfigStores.GetAiGatewayConfigStore(ctx, gatewayID, configStoreIDOrName, opts...)
}

func (a *AIGatewayConfigStoresAPIImpl) UpdateAiGatewayConfigStore(
	ctx context.Context,
	request kkOps.UpdateAiGatewayConfigStoreRequest,
	opts ...kkOps.Option,
) (*kkOps.UpdateAiGatewayConfigStoreResponse, error) {
	return a.SDK.AIGatewayConfigStores.UpdateAiGatewayConfigStore(ctx, request, opts...)
}

func (a *AIGatewayConfigStoresAPIImpl) DeleteAiGatewayConfigStore(
	ctx context.Context,
	request kkOps.DeleteAiGatewayConfigStoreRequest,
	opts ...kkOps.Option,
) (*kkOps.DeleteAiGatewayConfigStoreResponse, error) {
	return a.SDK.AIGatewayConfigStores.DeleteAiGatewayConfigStore(ctx, request, opts...)
}

func (a *AIGatewayConfigStoresAPIImpl) ListAiGatewayConfigStoreSecrets(
	ctx context.Context,
	request kkOps.ListAiGatewayConfigStoreSecretsRequest,
	opts ...kkOps.Option,
) (*kkOps.ListAiGatewayConfigStoreSecretsResponse, error) {
	return a.SDK.AIGatewayConfigStores.ListAiGatewayConfigStoreSecrets(ctx, request, opts...)
}

func (a *AIGatewayConfigStoresAPIImpl) CreateAiGatewayConfigStoreSecret(
	ctx context.Context,
	request kkOps.CreateAiGatewayConfigStoreSecretRequest,
	opts ...kkOps.Option,
) (*kkOps.CreateAiGatewayConfigStoreSecretResponse, error) {
	return a.SDK.AIGatewayConfigStores.CreateAiGatewayConfigStoreSecret(ctx, request, opts...)
}

func (a *AIGatewayConfigStoresAPIImpl) GetAiGatewayConfigStoreSecret(
	ctx context.Context,
	request kkOps.GetAiGatewayConfigStoreSecretRequest,
	opts ...kkOps.Option,
) (*kkOps.GetAiGatewayConfigStoreSecretResponse, error) {
	return a.SDK.AIGatewayConfigStores.GetAiGatewayConfigStoreSecret(ctx, request, opts...)
}

func (a *AIGatewayConfigStoresAPIImpl) UpdateAiGatewayConfigStoreSecret(
	ctx context.Context,
	request kkOps.UpdateAiGatewayConfigStoreSecretRequest,
	opts ...kkOps.Option,
) (*kkOps.UpdateAiGatewayConfigStoreSecretResponse, error) {
	return a.SDK.AIGatewayConfigStores.UpdateAiGatewayConfigStoreSecret(ctx, request, opts...)
}

func (a *AIGatewayConfigStoresAPIImpl) DeleteAiGatewayConfigStoreSecret(
	ctx context.Context,
	request kkOps.DeleteAiGatewayConfigStoreSecretRequest,
	opts ...kkOps.Option,
) (*kkOps.DeleteAiGatewayConfigStoreSecretResponse, error) {
	return a.SDK.AIGatewayConfigStores.DeleteAiGatewayConfigStoreSecret(ctx, request, opts...)
}
