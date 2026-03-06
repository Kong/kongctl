package helpers

import (
	"context"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

// EventGatewayClusterPolicyAPI defines the interface for Event Gateway Virtual Cluster Cluster-level Policy operations
type EventGatewayClusterPolicyAPI interface {
	ListEventGatewayVirtualClusterClusterLevelPolicies(ctx context.Context,
		request kkOps.ListEventGatewayVirtualClusterClusterLevelPoliciesRequest,
		opts ...kkOps.Option) (*kkOps.ListEventGatewayVirtualClusterClusterLevelPoliciesResponse, error)
	GetEventGatewayVirtualClusterClusterLevelPolicy(ctx context.Context,
		request kkOps.GetEventGatewayVirtualClusterClusterLevelPolicyRequest,
		opts ...kkOps.Option) (*kkOps.GetEventGatewayVirtualClusterClusterLevelPolicyResponse, error)
	CreateEventGatewayVirtualClusterClusterLevelPolicy(ctx context.Context,
		request kkOps.CreateEventGatewayVirtualClusterClusterLevelPolicyRequest,
		opts ...kkOps.Option) (*kkOps.CreateEventGatewayVirtualClusterClusterLevelPolicyResponse, error)
	UpdateEventGatewayVirtualClusterClusterLevelPolicy(ctx context.Context,
		request kkOps.UpdateEventGatewayVirtualClusterClusterLevelPolicyRequest,
		opts ...kkOps.Option) (*kkOps.UpdateEventGatewayVirtualClusterClusterLevelPolicyResponse, error)
	DeleteEventGatewayVirtualClusterClusterLevelPolicy(ctx context.Context,
		request kkOps.DeleteEventGatewayVirtualClusterClusterLevelPolicyRequest,
		opts ...kkOps.Option) (*kkOps.DeleteEventGatewayVirtualClusterClusterLevelPolicyResponse, error)
}

// EventGatewayClusterPolicyAPIImpl provides an implementation of the EventGatewayClusterPolicyAPI interface.
type EventGatewayClusterPolicyAPIImpl struct {
	SDK *kkSDK.SDK
}

func (a *EventGatewayClusterPolicyAPIImpl) ListEventGatewayVirtualClusterClusterLevelPolicies(
	ctx context.Context,
	request kkOps.ListEventGatewayVirtualClusterClusterLevelPoliciesRequest,
	opts ...kkOps.Option,
) (*kkOps.ListEventGatewayVirtualClusterClusterLevelPoliciesResponse, error) {
	return a.SDK.EventGatewayVirtualClusterPolicies.ListEventGatewayVirtualClusterClusterLevelPolicies(
		ctx, request, opts...)
}

func (a *EventGatewayClusterPolicyAPIImpl) GetEventGatewayVirtualClusterClusterLevelPolicy(
	ctx context.Context,
	request kkOps.GetEventGatewayVirtualClusterClusterLevelPolicyRequest,
	opts ...kkOps.Option,
) (*kkOps.GetEventGatewayVirtualClusterClusterLevelPolicyResponse, error) {
	return a.SDK.EventGatewayVirtualClusterPolicies.GetEventGatewayVirtualClusterClusterLevelPolicy(
		ctx, request, opts...)
}

func (a *EventGatewayClusterPolicyAPIImpl) CreateEventGatewayVirtualClusterClusterLevelPolicy(
	ctx context.Context,
	request kkOps.CreateEventGatewayVirtualClusterClusterLevelPolicyRequest,
	opts ...kkOps.Option,
) (*kkOps.CreateEventGatewayVirtualClusterClusterLevelPolicyResponse, error) {
	return a.SDK.EventGatewayVirtualClusterPolicies.CreateEventGatewayVirtualClusterClusterLevelPolicy(
		ctx, request, opts...)
}

func (a *EventGatewayClusterPolicyAPIImpl) UpdateEventGatewayVirtualClusterClusterLevelPolicy(
	ctx context.Context,
	request kkOps.UpdateEventGatewayVirtualClusterClusterLevelPolicyRequest,
	opts ...kkOps.Option,
) (*kkOps.UpdateEventGatewayVirtualClusterClusterLevelPolicyResponse, error) {
	return a.SDK.EventGatewayVirtualClusterPolicies.UpdateEventGatewayVirtualClusterClusterLevelPolicy(
		ctx, request, opts...)
}

func (a *EventGatewayClusterPolicyAPIImpl) DeleteEventGatewayVirtualClusterClusterLevelPolicy(
	ctx context.Context,
	request kkOps.DeleteEventGatewayVirtualClusterClusterLevelPolicyRequest,
	opts ...kkOps.Option,
) (*kkOps.DeleteEventGatewayVirtualClusterClusterLevelPolicyResponse, error) {
	return a.SDK.EventGatewayVirtualClusterPolicies.DeleteEventGatewayVirtualClusterClusterLevelPolicy(
		ctx, request, opts...)
}

// Compile-time interface assertion
var _ EventGatewayClusterPolicyAPI = &EventGatewayClusterPolicyAPIImpl{}
