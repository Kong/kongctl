package helpers

import (
	"context"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

// AIGatewayDatastoresAPI defines the interface for AI Gateway Datastore operations needed by kongctl.
type AIGatewayDatastoresAPI interface {
	ListAiGatewayDatastoreUsage(
		context.Context,
		kkOps.ListAiGatewayDatastoreUsageRequest,
		...kkOps.Option,
	) (*kkOps.ListAiGatewayDatastoreUsageResponse, error)
	ListAiGatewayDatastores(
		ctx context.Context,
		request kkOps.ListAiGatewayDatastoresRequest,
		opts ...kkOps.Option,
	) (*kkOps.ListAiGatewayDatastoresResponse, error)
	CreateAiGatewayDatastore(
		ctx context.Context,
		gatewayID string,
		request kkComps.CreateAIGatewayDatastoreRequest,
		opts ...kkOps.Option,
	) (*kkOps.CreateAiGatewayDatastoreResponse, error)
	GetAiGatewayDatastore(
		ctx context.Context,
		gatewayID string,
		datastoreID string,
		opts ...kkOps.Option,
	) (*kkOps.GetAiGatewayDatastoreResponse, error)
	UpdateAiGatewayDatastore(
		ctx context.Context,
		request kkOps.UpdateAiGatewayDatastoreRequest,
		opts ...kkOps.Option,
	) (*kkOps.UpdateAiGatewayDatastoreResponse, error)
	DeleteAiGatewayDatastore(
		ctx context.Context,
		gatewayID string,
		datastoreID string,
		opts ...kkOps.Option,
	) (*kkOps.DeleteAiGatewayDatastoreResponse, error)
}

// AIGatewayDatastoresAPIImpl provides the real SDK implementation.
type AIGatewayDatastoresAPIImpl struct {
	SDK *kkSDK.SDK
}

func (a *AIGatewayDatastoresAPIImpl) ListAiGatewayDatastores(
	ctx context.Context,
	request kkOps.ListAiGatewayDatastoresRequest,
	opts ...kkOps.Option,
) (*kkOps.ListAiGatewayDatastoresResponse, error) {
	return a.SDK.AIGatewayDatastores.ListAiGatewayDatastores(ctx, request, opts...)
}

func (a *AIGatewayDatastoresAPIImpl) CreateAiGatewayDatastore(
	ctx context.Context,
	gatewayID string,
	request kkComps.CreateAIGatewayDatastoreRequest,
	opts ...kkOps.Option,
) (*kkOps.CreateAiGatewayDatastoreResponse, error) {
	return a.SDK.AIGatewayDatastores.CreateAiGatewayDatastore(ctx, gatewayID, request, opts...)
}

func (a *AIGatewayDatastoresAPIImpl) GetAiGatewayDatastore(
	ctx context.Context,
	gatewayID string,
	datastoreID string,
	opts ...kkOps.Option,
) (*kkOps.GetAiGatewayDatastoreResponse, error) {
	return a.SDK.AIGatewayDatastores.GetAiGatewayDatastore(ctx, gatewayID, datastoreID, opts...)
}

func (a *AIGatewayDatastoresAPIImpl) UpdateAiGatewayDatastore(
	ctx context.Context,
	request kkOps.UpdateAiGatewayDatastoreRequest,
	opts ...kkOps.Option,
) (*kkOps.UpdateAiGatewayDatastoreResponse, error) {
	return a.SDK.AIGatewayDatastores.UpdateAiGatewayDatastore(ctx, request, opts...)
}

func (a *AIGatewayDatastoresAPIImpl) DeleteAiGatewayDatastore(
	ctx context.Context,
	gatewayID string,
	datastoreID string,
	opts ...kkOps.Option,
) (*kkOps.DeleteAiGatewayDatastoreResponse, error) {
	return a.SDK.AIGatewayDatastores.DeleteAiGatewayDatastore(ctx, gatewayID, datastoreID, opts...)
}

func (a *AIGatewayDatastoresAPIImpl) ListAiGatewayDatastoreUsage(
	ctx context.Context,
	request kkOps.ListAiGatewayDatastoreUsageRequest,
	opts ...kkOps.Option,
) (*kkOps.ListAiGatewayDatastoreUsageResponse, error) {
	return a.SDK.AIGatewayDatastores.ListAiGatewayDatastoreUsage(ctx, request, opts...)
}
