package helpers

import (
	"context"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

type EventGatewayListenerAPI interface {
	// Event Gateway Listener operations
	ListEventGatewayListeners(ctx context.Context, request kkOps.ListEventGatewayListenersRequest,
		opts ...kkOps.Option) (*kkOps.ListEventGatewayListenersResponse, error)
	FetchEventGatewayListener(ctx context.Context, gatewayID string, listenerID string,
		opts ...kkOps.Option) (*kkOps.GetEventGatewayListenerResponse, error)
	CreateEventGatewayListener(ctx context.Context, gatewayID string, request kkComps.CreateEventGatewayListenerRequest,
		opts ...kkOps.Option) (*kkOps.CreateEventGatewayListenerResponse, error)
	UpdateEventGatewayListener(
		ctx context.Context,
		gatewayID string,
		listenerID string,
		request kkComps.UpdateEventGatewayListenerRequest,
		opts ...kkOps.Option,
	) (*kkOps.UpdateEventGatewayListenerResponse, error)
	DeleteEventGatewayListener(ctx context.Context, gatewayID string, listenerID string,
		opts ...kkOps.Option) (*kkOps.DeleteEventGatewayListenerResponse, error)
}

// EventGatewayListenerAPIImpl provides an implementation of the EventGatewayListenerAPI interface.
// It implements all Event Gateway Listener operations defined by EventGatewayListenerAPI.
type EventGatewayListenerAPIImpl struct {
	SDK *kkSDK.SDK
}

func (a *EventGatewayListenerAPIImpl) ListEventGatewayListeners(
	ctx context.Context,
	request kkOps.ListEventGatewayListenersRequest,
	opts ...kkOps.Option,
) (*kkOps.ListEventGatewayListenersResponse, error) {
	return a.SDK.EventGatewayListeners.ListEventGatewayListeners(ctx, request, opts...)
}

func (a *EventGatewayListenerAPIImpl) FetchEventGatewayListener(
	ctx context.Context,
	gatewayID string,
	listenerID string,
	opts ...kkOps.Option,
) (*kkOps.GetEventGatewayListenerResponse, error) {
	return a.SDK.EventGatewayListeners.GetEventGatewayListener(ctx, gatewayID, listenerID, opts...)
}

func (a *EventGatewayListenerAPIImpl) CreateEventGatewayListener(
	ctx context.Context,
	gatewayID string,
	request kkComps.CreateEventGatewayListenerRequest,
	opts ...kkOps.Option,
) (*kkOps.CreateEventGatewayListenerResponse, error) {
	return a.SDK.EventGatewayListeners.CreateEventGatewayListener(ctx, gatewayID, &request, opts...)
}

func (a *EventGatewayListenerAPIImpl) UpdateEventGatewayListener(
	ctx context.Context,
	gatewayID string,
	listenerID string,
	request kkComps.UpdateEventGatewayListenerRequest,
	opts ...kkOps.Option,
) (*kkOps.UpdateEventGatewayListenerResponse, error) {
	putRequest := kkOps.UpdateEventGatewayListenerRequest{
		ListenerID:                         listenerID,
		GatewayID:                          gatewayID,
		UpdateEventGatewayListenerRequest1: &request,
	}

	return a.SDK.EventGatewayListeners.UpdateEventGatewayListener(ctx, putRequest, opts...)
}

func (a *EventGatewayListenerAPIImpl) DeleteEventGatewayListener(
	ctx context.Context,
	gatewayID string,
	listenerID string,
	opts ...kkOps.Option,
) (*kkOps.DeleteEventGatewayListenerResponse, error) {
	return a.SDK.EventGatewayListeners.DeleteEventGatewayListener(ctx, gatewayID, listenerID, opts...)
}
