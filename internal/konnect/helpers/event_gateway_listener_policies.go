package helpers

import (
	"context"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

type EventGatewayListenerPolicyAPI interface {
	// Event Gateway Listener Policy operations
	ListEventGatewayListenerPolicies(ctx context.Context,
		request kkOps.ListEventGatewayListenerPoliciesRequest,
		opts ...kkOps.Option) (*kkOps.ListEventGatewayListenerPoliciesResponse, error)
	GetEventGatewayListenerPolicy(ctx context.Context,
		request kkOps.GetEventGatewayListenerPolicyRequest,
		opts ...kkOps.Option) (*kkOps.GetEventGatewayListenerPolicyResponse, error)
	CreateEventGatewayListenerPolicy(ctx context.Context,
		request kkOps.CreateEventGatewayListenerPolicyRequest,
		opts ...kkOps.Option) (*kkOps.CreateEventGatewayListenerPolicyResponse, error)
	UpdateEventGatewayListenerPolicy(ctx context.Context,
		request kkOps.UpdateEventGatewayListenerPolicyRequest,
		opts ...kkOps.Option) (*kkOps.UpdateEventGatewayListenerPolicyResponse, error)
	DeleteEventGatewayListenerPolicy(ctx context.Context,
		request kkOps.DeleteEventGatewayListenerPolicyRequest,
		opts ...kkOps.Option) (*kkOps.DeleteEventGatewayListenerPolicyResponse, error)
}

// EventGatewayListenerPolicyAPIImpl provides an implementation of the EventGatewayListenerPolicyAPI interface.
type EventGatewayListenerPolicyAPIImpl struct {
	SDK *kkSDK.SDK
}

func (a *EventGatewayListenerPolicyAPIImpl) ListEventGatewayListenerPolicies(
	ctx context.Context,
	request kkOps.ListEventGatewayListenerPoliciesRequest,
	opts ...kkOps.Option,
) (*kkOps.ListEventGatewayListenerPoliciesResponse, error) {
	return a.SDK.EventGatewayListenerPolicies.ListEventGatewayListenerPolicies(ctx, request, opts...)
}

func (a *EventGatewayListenerPolicyAPIImpl) GetEventGatewayListenerPolicy(
	ctx context.Context,
	request kkOps.GetEventGatewayListenerPolicyRequest,
	opts ...kkOps.Option,
) (*kkOps.GetEventGatewayListenerPolicyResponse, error) {
	return a.SDK.EventGatewayListenerPolicies.GetEventGatewayListenerPolicy(ctx, request, opts...)
}

func (a *EventGatewayListenerPolicyAPIImpl) CreateEventGatewayListenerPolicy(
	ctx context.Context,
	request kkOps.CreateEventGatewayListenerPolicyRequest,
	opts ...kkOps.Option,
) (*kkOps.CreateEventGatewayListenerPolicyResponse, error) {
	return a.SDK.EventGatewayListenerPolicies.CreateEventGatewayListenerPolicy(ctx, request, opts...)
}

func (a *EventGatewayListenerPolicyAPIImpl) UpdateEventGatewayListenerPolicy(
	ctx context.Context,
	request kkOps.UpdateEventGatewayListenerPolicyRequest,
	opts ...kkOps.Option,
) (*kkOps.UpdateEventGatewayListenerPolicyResponse, error) {
	return a.SDK.EventGatewayListenerPolicies.UpdateEventGatewayListenerPolicy(ctx, request, opts...)
}

func (a *EventGatewayListenerPolicyAPIImpl) DeleteEventGatewayListenerPolicy(
	ctx context.Context,
	request kkOps.DeleteEventGatewayListenerPolicyRequest,
	opts ...kkOps.Option,
) (*kkOps.DeleteEventGatewayListenerPolicyResponse, error) {
	return a.SDK.EventGatewayListenerPolicies.DeleteEventGatewayListenerPolicy(ctx, request, opts...)
}

// Compile-time interface assertion
var _ EventGatewayListenerPolicyAPI = &EventGatewayListenerPolicyAPIImpl{}
