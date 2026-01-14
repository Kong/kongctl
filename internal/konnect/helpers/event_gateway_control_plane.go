package helpers

import (
	"context"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

type EGWControlPlaneAPI interface {
	// Event Gateway Control Plane operations
	ListEGWControlPlanes(ctx context.Context, request kkOps.ListEventGatewaysRequest,
		opts ...kkOps.Option) (*kkOps.ListEventGatewaysResponse, error)
	FetchEGWControlPlane(ctx context.Context, gatewayID string,
		opts ...kkOps.Option) (*kkOps.GetEventGatewayResponse, error)
	CreateEGWControlPlane(ctx context.Context, request kkComps.CreateGatewayRequest,
		opts ...kkOps.Option) (*kkOps.CreateEventGatewayResponse, error)
	UpdateEGWControlPlane(ctx context.Context, gatewayID string, request kkComps.UpdateGatewayRequest,
		opts ...kkOps.Option) (*kkOps.UpdateEventGatewayResponse, error)
	DeleteEGWControlPlane(ctx context.Context, gatewayID string,
		opts ...kkOps.Option) (*kkOps.DeleteEventGatewayResponse, error)
}

// EGWControlPlaneAPIImpl provides an implementation of the EGWControlPlaneAPI interface.
// It implements all Event Gateway Control Plane operations defined by EGWControlPlaneAPI.
type EGWControlPlaneAPIImpl struct {
	SDK *kkSDK.SDK
}

func (a *EGWControlPlaneAPIImpl) ListEGWControlPlanes(ctx context.Context, request kkOps.ListEventGatewaysRequest,
	opts ...kkOps.Option,
) (*kkOps.ListEventGatewaysResponse, error) {
	return a.SDK.EventGateways.ListEventGateways(ctx, request, opts...)
}

func (a *EGWControlPlaneAPIImpl) FetchEGWControlPlane(ctx context.Context, gatewayID string,
	opts ...kkOps.Option,
) (*kkOps.GetEventGatewayResponse, error) {
	return a.SDK.EventGateways.GetEventGateway(ctx, gatewayID, opts...)
}

func (a *EGWControlPlaneAPIImpl) CreateEGWControlPlane(ctx context.Context, request kkComps.CreateGatewayRequest,
	opts ...kkOps.Option,
) (*kkOps.CreateEventGatewayResponse, error) {
	return a.SDK.EventGateways.CreateEventGateway(ctx, request, opts...)
}

func (a *EGWControlPlaneAPIImpl) UpdateEGWControlPlane(
	ctx context.Context,
	gatewayID string,
	request kkComps.UpdateGatewayRequest,
	opts ...kkOps.Option,
) (*kkOps.UpdateEventGatewayResponse, error) {
	return a.SDK.EventGateways.UpdateEventGateway(ctx, gatewayID, request, opts...)
}

func (a *EGWControlPlaneAPIImpl) DeleteEGWControlPlane(ctx context.Context, gatewayID string,
	opts ...kkOps.Option,
) (*kkOps.DeleteEventGatewayResponse, error) {
	return a.SDK.EventGateways.DeleteEventGateway(ctx, gatewayID, opts...)
}
