package helpers

import (
	"context"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

type EventGatewayDataPlaneCertificateAPI interface {
	// Event Gateway Data Plane Certificate operations
	ListEventGatewayDataPlaneCertificates(ctx context.Context,
		request kkOps.ListEventGatewayDataPlaneCertificatesRequest,
		opts ...kkOps.Option) (*kkOps.ListEventGatewayDataPlaneCertificatesResponse, error)
	FetchEventGatewayDataPlaneCertificate(ctx context.Context, gatewayID string, certificateID string,
		opts ...kkOps.Option) (*kkOps.GetEventGatewayDataPlaneCertificateResponse, error)
	CreateEventGatewayDataPlaneCertificate(ctx context.Context, gatewayID string,
		request kkComps.CreateEventGatewayDataPlaneCertificateRequest,
		opts ...kkOps.Option) (*kkOps.CreateEventGatewayDataPlaneCertificateResponse, error)
	UpdateEventGatewayDataPlaneCertificate(
		ctx context.Context,
		gatewayID string,
		certificateID string,
		request kkComps.UpdateEventGatewayDataPlaneCertificateRequest,
		opts ...kkOps.Option,
	) (*kkOps.UpdateEventGatewayDataPlaneCertificateResponse, error)
	DeleteEventGatewayDataPlaneCertificate(ctx context.Context, gatewayID string, certificateID string,
		opts ...kkOps.Option) (*kkOps.DeleteEventGatewayDataPlaneCertificateResponse, error)
}

// EventGatewayDataPlaneCertificateAPIImpl provides an implementation of the
// EventGatewayDataPlaneCertificateAPI interface.
// It implements all Event Gateway Data Plane Certificate operations.
type EventGatewayDataPlaneCertificateAPIImpl struct {
	SDK *kkSDK.SDK
}

func (a *EventGatewayDataPlaneCertificateAPIImpl) ListEventGatewayDataPlaneCertificates(
	ctx context.Context,
	request kkOps.ListEventGatewayDataPlaneCertificatesRequest,
	opts ...kkOps.Option,
) (*kkOps.ListEventGatewayDataPlaneCertificatesResponse, error) {
	return a.SDK.EventGatewayDataPlaneCertificates.ListEventGatewayDataPlaneCertificates(ctx, request, opts...)
}

func (a *EventGatewayDataPlaneCertificateAPIImpl) FetchEventGatewayDataPlaneCertificate(
	ctx context.Context,
	gatewayID string,
	certificateID string,
	opts ...kkOps.Option,
) (*kkOps.GetEventGatewayDataPlaneCertificateResponse, error) {
	return a.SDK.EventGatewayDataPlaneCertificates.GetEventGatewayDataPlaneCertificate(
		ctx, gatewayID, certificateID, opts...)
}

func (a *EventGatewayDataPlaneCertificateAPIImpl) CreateEventGatewayDataPlaneCertificate(
	ctx context.Context,
	gatewayID string,
	request kkComps.CreateEventGatewayDataPlaneCertificateRequest,
	opts ...kkOps.Option,
) (*kkOps.CreateEventGatewayDataPlaneCertificateResponse, error) {
	return a.SDK.EventGatewayDataPlaneCertificates.CreateEventGatewayDataPlaneCertificate(
		ctx, gatewayID, &request, opts...)
}

func (a *EventGatewayDataPlaneCertificateAPIImpl) UpdateEventGatewayDataPlaneCertificate(
	ctx context.Context,
	gatewayID string,
	certificateID string,
	request kkComps.UpdateEventGatewayDataPlaneCertificateRequest,
	opts ...kkOps.Option,
) (*kkOps.UpdateEventGatewayDataPlaneCertificateResponse, error) {
	putRequest := kkOps.UpdateEventGatewayDataPlaneCertificateRequest{
		CertificateID: certificateID,
		GatewayID:     gatewayID,
		UpdateEventGatewayDataPlaneCertificateRequest: &request,
	}

	return a.SDK.EventGatewayDataPlaneCertificates.UpdateEventGatewayDataPlaneCertificate(ctx, putRequest, opts...)
}

func (a *EventGatewayDataPlaneCertificateAPIImpl) DeleteEventGatewayDataPlaneCertificate(
	ctx context.Context,
	gatewayID string,
	certificateID string,
	opts ...kkOps.Option,
) (*kkOps.DeleteEventGatewayDataPlaneCertificateResponse, error) {
	return a.SDK.EventGatewayDataPlaneCertificates.DeleteEventGatewayDataPlaneCertificate(
		ctx, gatewayID, certificateID, opts...)
}
