package helpers

import (
	"context"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

// EventGatewayConsumePolicyAPI defines the interface for Event Gateway Virtual Cluster Consume Policy operations.
type EventGatewayConsumePolicyAPI interface {
	ListEventGatewayVirtualClusterConsumePolicies(
		ctx context.Context,
		request kkOps.ListEventGatewayVirtualClusterConsumePoliciesRequest,
		opts ...kkOps.Option,
	) (*kkOps.ListEventGatewayVirtualClusterConsumePoliciesResponse, error)
	GetEventGatewayVirtualClusterConsumePolicy(
		ctx context.Context,
		request kkOps.GetEventGatewayVirtualClusterConsumePolicyRequest,
		opts ...kkOps.Option,
	) (*kkOps.GetEventGatewayVirtualClusterConsumePolicyResponse, error)
	CreateEventGatewayVirtualClusterConsumePolicy(
		ctx context.Context,
		request kkOps.CreateEventGatewayVirtualClusterConsumePolicyRequest,
		opts ...kkOps.Option,
	) (*kkOps.CreateEventGatewayVirtualClusterConsumePolicyResponse, error)
	UpdateEventGatewayVirtualClusterConsumePolicy(
		ctx context.Context,
		request kkOps.UpdateEventGatewayVirtualClusterConsumePolicyRequest,
		opts ...kkOps.Option,
	) (*kkOps.UpdateEventGatewayVirtualClusterConsumePolicyResponse, error)
	DeleteEventGatewayVirtualClusterConsumePolicy(
		ctx context.Context,
		request kkOps.DeleteEventGatewayVirtualClusterConsumePolicyRequest,
		opts ...kkOps.Option,
	) (*kkOps.DeleteEventGatewayVirtualClusterConsumePolicyResponse, error)
}

// EventGatewayConsumePolicyAPIImpl provides an implementation of the EventGatewayConsumePolicyAPI interface.
type EventGatewayConsumePolicyAPIImpl struct {
	SDK *kkSDK.SDK
}

func (a *EventGatewayConsumePolicyAPIImpl) ListEventGatewayVirtualClusterConsumePolicies(
	ctx context.Context,
	request kkOps.ListEventGatewayVirtualClusterConsumePoliciesRequest,
	opts ...kkOps.Option,
) (*kkOps.ListEventGatewayVirtualClusterConsumePoliciesResponse, error) {
	return a.SDK.EventGatewayVirtualClusterConsumePolicies.ListEventGatewayVirtualClusterConsumePolicies(
		ctx, request, opts...)
}

func (a *EventGatewayConsumePolicyAPIImpl) GetEventGatewayVirtualClusterConsumePolicy(
	ctx context.Context,
	request kkOps.GetEventGatewayVirtualClusterConsumePolicyRequest,
	opts ...kkOps.Option,
) (*kkOps.GetEventGatewayVirtualClusterConsumePolicyResponse, error) {
	return a.SDK.EventGatewayVirtualClusterConsumePolicies.GetEventGatewayVirtualClusterConsumePolicy(
		ctx, request, opts...)
}

func (a *EventGatewayConsumePolicyAPIImpl) CreateEventGatewayVirtualClusterConsumePolicy(
	ctx context.Context,
	request kkOps.CreateEventGatewayVirtualClusterConsumePolicyRequest,
	opts ...kkOps.Option,
) (*kkOps.CreateEventGatewayVirtualClusterConsumePolicyResponse, error) {
	return a.SDK.EventGatewayVirtualClusterConsumePolicies.CreateEventGatewayVirtualClusterConsumePolicy(
		ctx, request, opts...)
}

func (a *EventGatewayConsumePolicyAPIImpl) UpdateEventGatewayVirtualClusterConsumePolicy(
	ctx context.Context,
	request kkOps.UpdateEventGatewayVirtualClusterConsumePolicyRequest,
	opts ...kkOps.Option,
) (*kkOps.UpdateEventGatewayVirtualClusterConsumePolicyResponse, error) {
	return a.SDK.EventGatewayVirtualClusterConsumePolicies.UpdateEventGatewayVirtualClusterConsumePolicy(
		ctx, request, opts...)
}

func (a *EventGatewayConsumePolicyAPIImpl) DeleteEventGatewayVirtualClusterConsumePolicy(
	ctx context.Context,
	request kkOps.DeleteEventGatewayVirtualClusterConsumePolicyRequest,
	opts ...kkOps.Option,
) (*kkOps.DeleteEventGatewayVirtualClusterConsumePolicyResponse, error) {
	return a.SDK.EventGatewayVirtualClusterConsumePolicies.DeleteEventGatewayVirtualClusterConsumePolicy(
		ctx, request, opts...)
}

// Compile-time interface assertion
var _ EventGatewayConsumePolicyAPI = &EventGatewayConsumePolicyAPIImpl{}
