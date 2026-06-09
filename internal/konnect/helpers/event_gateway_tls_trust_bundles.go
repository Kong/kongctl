package helpers

import (
	"context"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

// EventGatewayTLSTrustBundleAPI defines the interface for Event Gateway TLS Trust Bundle operations.
type EventGatewayTLSTrustBundleAPI interface {
	ListEventGatewayTLSTrustBundles(ctx context.Context,
		request kkOps.ListEventGatewayTLSTrustBundlesRequest,
		opts ...kkOps.Option) (*kkOps.ListEventGatewayTLSTrustBundlesResponse, error)
	GetEventGatewayTLSTrustBundle(ctx context.Context,
		request kkOps.GetEventGatewayTLSTrustBundleRequest,
		opts ...kkOps.Option) (*kkOps.GetEventGatewayTLSTrustBundleResponse, error)
	CreateEventGatewayTLSTrustBundle(ctx context.Context,
		request kkOps.CreateEventGatewayTLSTrustBundleRequest,
		opts ...kkOps.Option) (*kkOps.CreateEventGatewayTLSTrustBundleResponse, error)
	UpdateEventGatewayTLSTrustBundle(ctx context.Context,
		request kkOps.UpdateEventGatewayTLSTrustBundleRequest,
		opts ...kkOps.Option) (*kkOps.UpdateEventGatewayTLSTrustBundleResponse, error)
	DeleteEventGatewayTLSTrustBundle(ctx context.Context,
		request kkOps.DeleteEventGatewayTLSTrustBundleRequest,
		opts ...kkOps.Option) (*kkOps.DeleteEventGatewayTLSTrustBundleResponse, error)
}

// EventGatewayTLSTrustBundleAPIImpl provides an implementation of the EventGatewayTLSTrustBundleAPI interface.
type EventGatewayTLSTrustBundleAPIImpl struct {
	SDK *kkSDK.SDK
}

func (a *EventGatewayTLSTrustBundleAPIImpl) ListEventGatewayTLSTrustBundles(
	ctx context.Context,
	request kkOps.ListEventGatewayTLSTrustBundlesRequest,
	opts ...kkOps.Option,
) (*kkOps.ListEventGatewayTLSTrustBundlesResponse, error) {
	return a.SDK.EventGatewayTLSTrustBundles.ListEventGatewayTLSTrustBundles(ctx, request, opts...)
}

func (a *EventGatewayTLSTrustBundleAPIImpl) GetEventGatewayTLSTrustBundle(
	ctx context.Context,
	request kkOps.GetEventGatewayTLSTrustBundleRequest,
	opts ...kkOps.Option,
) (*kkOps.GetEventGatewayTLSTrustBundleResponse, error) {
	return a.SDK.EventGatewayTLSTrustBundles.GetEventGatewayTLSTrustBundle(
		ctx, request.GatewayID, request.TLSTrustBundleID, opts...)
}

func (a *EventGatewayTLSTrustBundleAPIImpl) CreateEventGatewayTLSTrustBundle(
	ctx context.Context,
	request kkOps.CreateEventGatewayTLSTrustBundleRequest,
	opts ...kkOps.Option,
) (*kkOps.CreateEventGatewayTLSTrustBundleResponse, error) {
	return a.SDK.EventGatewayTLSTrustBundles.CreateEventGatewayTLSTrustBundle(
		ctx, request.GatewayID, request.CreateTLSTrustBundleRequest, opts...)
}

func (a *EventGatewayTLSTrustBundleAPIImpl) UpdateEventGatewayTLSTrustBundle(
	ctx context.Context,
	request kkOps.UpdateEventGatewayTLSTrustBundleRequest,
	opts ...kkOps.Option,
) (*kkOps.UpdateEventGatewayTLSTrustBundleResponse, error) {
	return a.SDK.EventGatewayTLSTrustBundles.UpdateEventGatewayTLSTrustBundle(ctx, request, opts...)
}

func (a *EventGatewayTLSTrustBundleAPIImpl) DeleteEventGatewayTLSTrustBundle(
	ctx context.Context,
	request kkOps.DeleteEventGatewayTLSTrustBundleRequest,
	opts ...kkOps.Option,
) (*kkOps.DeleteEventGatewayTLSTrustBundleResponse, error) {
	return a.SDK.EventGatewayTLSTrustBundles.DeleteEventGatewayTLSTrustBundle(
		ctx, request.GatewayID, request.TLSTrustBundleID, opts...,
	)
}
