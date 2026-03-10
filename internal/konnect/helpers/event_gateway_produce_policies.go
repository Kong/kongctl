package helpers

import (
	"context"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

// EventGatewayProducePolicyAPI defines the interface for Event Gateway Virtual Cluster Produce Policy operations
type EventGatewayProducePolicyAPI interface {
	ListEventGatewayVirtualClusterProducePolicies(ctx context.Context,
		request kkOps.ListEventGatewayVirtualClusterProducePoliciesRequest,
		opts ...kkOps.Option) (*kkOps.ListEventGatewayVirtualClusterProducePoliciesResponse, error)
	GetEventGatewayVirtualClusterProducePolicy(ctx context.Context,
		request kkOps.GetEventGatewayVirtualClusterProducePolicyRequest,
		opts ...kkOps.Option) (*kkOps.GetEventGatewayVirtualClusterProducePolicyResponse, error)
	CreateEventGatewayVirtualClusterProducePolicy(ctx context.Context,
		request kkOps.CreateEventGatewayVirtualClusterProducePolicyRequest,
		opts ...kkOps.Option) (*kkOps.CreateEventGatewayVirtualClusterProducePolicyResponse, error)
	UpdateEventGatewayVirtualClusterProducePolicy(ctx context.Context,
		request kkOps.UpdateEventGatewayVirtualClusterProducePolicyRequest,
		opts ...kkOps.Option) (*kkOps.UpdateEventGatewayVirtualClusterProducePolicyResponse, error)
	DeleteEventGatewayVirtualClusterProducePolicy(ctx context.Context,
		request kkOps.DeleteEventGatewayVirtualClusterProducePolicyRequest,
		opts ...kkOps.Option) (*kkOps.DeleteEventGatewayVirtualClusterProducePolicyResponse, error)
}

// EventGatewayProducePolicyAPIImpl provides an implementation of the EventGatewayProducePolicyAPI interface.
type EventGatewayProducePolicyAPIImpl struct {
	SDK *kkSDK.SDK
}

func (a *EventGatewayProducePolicyAPIImpl) ListEventGatewayVirtualClusterProducePolicies(
	ctx context.Context,
	request kkOps.ListEventGatewayVirtualClusterProducePoliciesRequest,
	opts ...kkOps.Option,
) (*kkOps.ListEventGatewayVirtualClusterProducePoliciesResponse, error) {
	return a.SDK.EventGatewayVirtualClusterProducePolicies.ListEventGatewayVirtualClusterProducePolicies(
		ctx, request, opts...)
}

func (a *EventGatewayProducePolicyAPIImpl) GetEventGatewayVirtualClusterProducePolicy(
	ctx context.Context,
	request kkOps.GetEventGatewayVirtualClusterProducePolicyRequest,
	opts ...kkOps.Option,
) (*kkOps.GetEventGatewayVirtualClusterProducePolicyResponse, error) {
	return a.SDK.EventGatewayVirtualClusterProducePolicies.GetEventGatewayVirtualClusterProducePolicy(
		ctx, request, opts...)
}

func (a *EventGatewayProducePolicyAPIImpl) CreateEventGatewayVirtualClusterProducePolicy(
	ctx context.Context,
	request kkOps.CreateEventGatewayVirtualClusterProducePolicyRequest,
	opts ...kkOps.Option,
) (*kkOps.CreateEventGatewayVirtualClusterProducePolicyResponse, error) {
	return a.SDK.EventGatewayVirtualClusterProducePolicies.CreateEventGatewayVirtualClusterProducePolicy(
		ctx, request, opts...)
}

func (a *EventGatewayProducePolicyAPIImpl) UpdateEventGatewayVirtualClusterProducePolicy(
	ctx context.Context,
	request kkOps.UpdateEventGatewayVirtualClusterProducePolicyRequest,
	opts ...kkOps.Option,
) (*kkOps.UpdateEventGatewayVirtualClusterProducePolicyResponse, error) {
	return a.SDK.EventGatewayVirtualClusterProducePolicies.UpdateEventGatewayVirtualClusterProducePolicy(
		ctx, request, opts...)
}

func (a *EventGatewayProducePolicyAPIImpl) DeleteEventGatewayVirtualClusterProducePolicy(
	ctx context.Context,
	request kkOps.DeleteEventGatewayVirtualClusterProducePolicyRequest,
	opts ...kkOps.Option,
) (*kkOps.DeleteEventGatewayVirtualClusterProducePolicyResponse, error) {
	return a.SDK.EventGatewayVirtualClusterProducePolicies.DeleteEventGatewayVirtualClusterProducePolicy(
		ctx, request, opts...)
}

// Compile-time interface assertion
var _ EventGatewayProducePolicyAPI = &EventGatewayProducePolicyAPIImpl{}
