package helpers

import (
	"context"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

// AIGatewayConsumerGroupsAPI defines the interface for AI Gateway Consumer Group operations needed by kongctl.
type AIGatewayConsumerGroupsAPI interface {
	ListAiGatewayConsumerGroups(
		ctx context.Context,
		request kkOps.ListAiGatewayConsumerGroupsRequest,
		opts ...kkOps.Option,
	) (*kkOps.ListAiGatewayConsumerGroupsResponse, error)
	CreateAiGatewayConsumerGroup(
		ctx context.Context,
		gatewayID string,
		request kkComps.CreateAIGatewayConsumerGroupRequest,
		opts ...kkOps.Option,
	) (*kkOps.CreateAiGatewayConsumerGroupResponse, error)
	GetAiGatewayConsumerGroup(
		ctx context.Context,
		gatewayID string,
		consumerGroupID string,
		opts ...kkOps.Option,
	) (*kkOps.GetAiGatewayConsumerGroupResponse, error)
	UpdateAiGatewayConsumerGroup(
		ctx context.Context,
		request kkOps.UpdateAiGatewayConsumerGroupRequest,
		opts ...kkOps.Option,
	) (*kkOps.UpdateAiGatewayConsumerGroupResponse, error)
	DeleteAiGatewayConsumerGroup(
		ctx context.Context,
		gatewayID string,
		consumerGroupID string,
		opts ...kkOps.Option,
	) (*kkOps.DeleteAiGatewayConsumerGroupResponse, error)
}

// AIGatewayConsumerGroupsAPIImpl provides the real SDK implementation.
type AIGatewayConsumerGroupsAPIImpl struct {
	SDK *kkSDK.SDK
}

func (a *AIGatewayConsumerGroupsAPIImpl) ListAiGatewayConsumerGroups(
	ctx context.Context,
	request kkOps.ListAiGatewayConsumerGroupsRequest,
	opts ...kkOps.Option,
) (*kkOps.ListAiGatewayConsumerGroupsResponse, error) {
	return a.SDK.AIGatewayConsumerGroups.ListAiGatewayConsumerGroups(ctx, request, opts...)
}

func (a *AIGatewayConsumerGroupsAPIImpl) CreateAiGatewayConsumerGroup(
	ctx context.Context,
	gatewayID string,
	request kkComps.CreateAIGatewayConsumerGroupRequest,
	opts ...kkOps.Option,
) (*kkOps.CreateAiGatewayConsumerGroupResponse, error) {
	return a.SDK.AIGatewayConsumerGroups.CreateAiGatewayConsumerGroup(ctx, gatewayID, request, opts...)
}

func (a *AIGatewayConsumerGroupsAPIImpl) GetAiGatewayConsumerGroup(
	ctx context.Context,
	gatewayID string,
	consumerGroupID string,
	opts ...kkOps.Option,
) (*kkOps.GetAiGatewayConsumerGroupResponse, error) {
	return a.SDK.AIGatewayConsumerGroups.GetAiGatewayConsumerGroup(ctx, gatewayID, consumerGroupID, opts...)
}

func (a *AIGatewayConsumerGroupsAPIImpl) UpdateAiGatewayConsumerGroup(
	ctx context.Context,
	request kkOps.UpdateAiGatewayConsumerGroupRequest,
	opts ...kkOps.Option,
) (*kkOps.UpdateAiGatewayConsumerGroupResponse, error) {
	return a.SDK.AIGatewayConsumerGroups.UpdateAiGatewayConsumerGroup(ctx, request, opts...)
}

func (a *AIGatewayConsumerGroupsAPIImpl) DeleteAiGatewayConsumerGroup(
	ctx context.Context,
	gatewayID string,
	consumerGroupID string,
	opts ...kkOps.Option,
) (*kkOps.DeleteAiGatewayConsumerGroupResponse, error) {
	return a.SDK.AIGatewayConsumerGroups.DeleteAiGatewayConsumerGroup(ctx, gatewayID, consumerGroupID, opts...)
}
