package helpers

import (
	"context"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

// EventGatewayStaticKeyAPI defines the interface for Event Gateway Static Key operations.
type EventGatewayStaticKeyAPI interface {
	ListEventGatewayStaticKeys(ctx context.Context,
		request kkOps.ListEventGatewayStaticKeysRequest,
		opts ...kkOps.Option) (*kkOps.ListEventGatewayStaticKeysResponse, error)
	GetEventGatewayStaticKey(ctx context.Context,
		request kkOps.GetEventGatewayStaticKeyRequest,
		opts ...kkOps.Option) (*kkOps.GetEventGatewayStaticKeyResponse, error)
	CreateEventGatewayStaticKey(ctx context.Context,
		request kkOps.CreateEventGatewayStaticKeyRequest,
		opts ...kkOps.Option) (*kkOps.CreateEventGatewayStaticKeyResponse, error)
	DeleteEventGatewayStaticKey(ctx context.Context,
		request kkOps.DeleteEventGatewayStaticKeyRequest,
		opts ...kkOps.Option) (*kkOps.DeleteEventGatewayStaticKeyResponse, error)
}

// EventGatewayStaticKeyAPIImpl provides an implementation of the EventGatewayStaticKeyAPI interface.
type EventGatewayStaticKeyAPIImpl struct {
	SDK *kkSDK.SDK
}

func (a *EventGatewayStaticKeyAPIImpl) ListEventGatewayStaticKeys(
	ctx context.Context,
	request kkOps.ListEventGatewayStaticKeysRequest,
	opts ...kkOps.Option,
) (*kkOps.ListEventGatewayStaticKeysResponse, error) {
	return a.SDK.EventGatewayStaticKeys.ListEventGatewayStaticKeys(ctx, request, opts...)
}

func (a *EventGatewayStaticKeyAPIImpl) GetEventGatewayStaticKey(
	ctx context.Context,
	request kkOps.GetEventGatewayStaticKeyRequest,
	opts ...kkOps.Option,
) (*kkOps.GetEventGatewayStaticKeyResponse, error) {
	return a.SDK.EventGatewayStaticKeys.GetEventGatewayStaticKey(ctx, request.GatewayID, request.StaticKeyID, opts...)
}

func (a *EventGatewayStaticKeyAPIImpl) CreateEventGatewayStaticKey(
	ctx context.Context,
	request kkOps.CreateEventGatewayStaticKeyRequest,
	opts ...kkOps.Option,
) (*kkOps.CreateEventGatewayStaticKeyResponse, error) {
	return a.SDK.EventGatewayStaticKeys.CreateEventGatewayStaticKey(
		ctx, request.GatewayID, request.EventGatewayStaticKeyCreate, opts...)
}

func (a *EventGatewayStaticKeyAPIImpl) DeleteEventGatewayStaticKey(
	ctx context.Context,
	request kkOps.DeleteEventGatewayStaticKeyRequest,
	opts ...kkOps.Option,
) (*kkOps.DeleteEventGatewayStaticKeyResponse, error) {
	return a.SDK.EventGatewayStaticKeys.DeleteEventGatewayStaticKey(
		ctx, request.GatewayID, request.StaticKeyID, opts...)
}
