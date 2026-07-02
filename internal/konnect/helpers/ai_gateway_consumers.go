package helpers

import (
	"context"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

// AIGatewayConsumersAPI defines the interface for AI Gateway Consumer operations needed by kongctl.
type AIGatewayConsumersAPI interface {
	ListAiGatewayConsumers(
		ctx context.Context,
		request kkOps.ListAiGatewayConsumersRequest,
		opts ...kkOps.Option,
	) (*kkOps.ListAiGatewayConsumersResponse, error)
	CreateAiGatewayConsumer(
		ctx context.Context,
		gatewayID string,
		request kkComps.CreateAIGatewayConsumerRequest,
		opts ...kkOps.Option,
	) (*kkOps.CreateAiGatewayConsumerResponse, error)
	GetAiGatewayConsumer(
		ctx context.Context,
		gatewayID string,
		consumerID string,
		opts ...kkOps.Option,
	) (*kkOps.GetAiGatewayConsumerResponse, error)
	UpdateAiGatewayConsumer(
		ctx context.Context,
		request kkOps.UpdateAiGatewayConsumerRequest,
		opts ...kkOps.Option,
	) (*kkOps.UpdateAiGatewayConsumerResponse, error)
	DeleteAiGatewayConsumer(
		ctx context.Context,
		gatewayID string,
		consumerID string,
		opts ...kkOps.Option,
	) (*kkOps.DeleteAiGatewayConsumerResponse, error)
	ListAiGatewayConsumerCredentials(
		ctx context.Context,
		request kkOps.ListAiGatewayConsumerCredentialsRequest,
		opts ...kkOps.Option,
	) (*kkOps.ListAiGatewayConsumerCredentialsResponse, error)
	CreateAiGatewayConsumerCredential(
		ctx context.Context,
		request kkOps.CreateAiGatewayConsumerCredentialRequest,
		opts ...kkOps.Option,
	) (*kkOps.CreateAiGatewayConsumerCredentialResponse, error)
	GetAiGatewayConsumerCredential(
		ctx context.Context,
		request kkOps.GetAiGatewayConsumerCredentialRequest,
		opts ...kkOps.Option,
	) (*kkOps.GetAiGatewayConsumerCredentialResponse, error)
	DeleteAiGatewayConsumerCredential(
		ctx context.Context,
		request kkOps.DeleteAiGatewayConsumerCredentialRequest,
		opts ...kkOps.Option,
	) (*kkOps.DeleteAiGatewayConsumerCredentialResponse, error)
}

// AIGatewayConsumersAPIImpl provides the real SDK implementation.
type AIGatewayConsumersAPIImpl struct {
	SDK *kkSDK.SDK
}

func (a *AIGatewayConsumersAPIImpl) ListAiGatewayConsumers(
	ctx context.Context,
	request kkOps.ListAiGatewayConsumersRequest,
	opts ...kkOps.Option,
) (*kkOps.ListAiGatewayConsumersResponse, error) {
	return a.SDK.AIGatewayConsumers.ListAiGatewayConsumers(ctx, request, opts...)
}

func (a *AIGatewayConsumersAPIImpl) CreateAiGatewayConsumer(
	ctx context.Context,
	gatewayID string,
	request kkComps.CreateAIGatewayConsumerRequest,
	opts ...kkOps.Option,
) (*kkOps.CreateAiGatewayConsumerResponse, error) {
	return a.SDK.AIGatewayConsumers.CreateAiGatewayConsumer(ctx, gatewayID, request, opts...)
}

func (a *AIGatewayConsumersAPIImpl) GetAiGatewayConsumer(
	ctx context.Context,
	gatewayID string,
	consumerID string,
	opts ...kkOps.Option,
) (*kkOps.GetAiGatewayConsumerResponse, error) {
	return a.SDK.AIGatewayConsumers.GetAiGatewayConsumer(ctx, gatewayID, consumerID, opts...)
}

func (a *AIGatewayConsumersAPIImpl) UpdateAiGatewayConsumer(
	ctx context.Context,
	request kkOps.UpdateAiGatewayConsumerRequest,
	opts ...kkOps.Option,
) (*kkOps.UpdateAiGatewayConsumerResponse, error) {
	return a.SDK.AIGatewayConsumers.UpdateAiGatewayConsumer(ctx, request, opts...)
}

func (a *AIGatewayConsumersAPIImpl) DeleteAiGatewayConsumer(
	ctx context.Context,
	gatewayID string,
	consumerID string,
	opts ...kkOps.Option,
) (*kkOps.DeleteAiGatewayConsumerResponse, error) {
	return a.SDK.AIGatewayConsumers.DeleteAiGatewayConsumer(ctx, gatewayID, consumerID, opts...)
}

func (a *AIGatewayConsumersAPIImpl) ListAiGatewayConsumerCredentials(
	ctx context.Context,
	request kkOps.ListAiGatewayConsumerCredentialsRequest,
	opts ...kkOps.Option,
) (*kkOps.ListAiGatewayConsumerCredentialsResponse, error) {
	return a.SDK.AIGatewayConsumers.ListAiGatewayConsumerCredentials(ctx, request, opts...)
}

func (a *AIGatewayConsumersAPIImpl) CreateAiGatewayConsumerCredential(
	ctx context.Context,
	request kkOps.CreateAiGatewayConsumerCredentialRequest,
	opts ...kkOps.Option,
) (*kkOps.CreateAiGatewayConsumerCredentialResponse, error) {
	return a.SDK.AIGatewayConsumers.CreateAiGatewayConsumerCredential(ctx, request, opts...)
}

func (a *AIGatewayConsumersAPIImpl) GetAiGatewayConsumerCredential(
	ctx context.Context,
	request kkOps.GetAiGatewayConsumerCredentialRequest,
	opts ...kkOps.Option,
) (*kkOps.GetAiGatewayConsumerCredentialResponse, error) {
	return a.SDK.AIGatewayConsumers.GetAiGatewayConsumerCredential(ctx, request, opts...)
}

func (a *AIGatewayConsumersAPIImpl) DeleteAiGatewayConsumerCredential(
	ctx context.Context,
	request kkOps.DeleteAiGatewayConsumerCredentialRequest,
	opts ...kkOps.Option,
) (*kkOps.DeleteAiGatewayConsumerCredentialResponse, error) {
	return a.SDK.AIGatewayConsumers.DeleteAiGatewayConsumerCredential(ctx, request, opts...)
}
