package helpers

import (
	"context"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

// AIGatewayDataPlaneCertificatesAPI defines the interface for AI Gateway data plane certificate operations.
type AIGatewayDataPlaneCertificatesAPI interface {
	ListAiGatewayDataPlaneCertificates(
		ctx context.Context,
		request kkOps.ListAiGatewayDataPlaneCertificatesRequest,
		opts ...kkOps.Option,
	) (*kkOps.ListAiGatewayDataPlaneCertificatesResponse, error)
	CreateAiGatewayDataPlaneCertificate(
		ctx context.Context,
		gatewayID string,
		request kkComps.CreateAIGatewayDataPlaneCertificateRequest,
		opts ...kkOps.Option,
	) (*kkOps.CreateAiGatewayDataPlaneCertificateResponse, error)
	GetAiGatewayDataPlaneCertificate(
		ctx context.Context,
		gatewayID string,
		certificateID string,
		opts ...kkOps.Option,
	) (*kkOps.GetAiGatewayDataPlaneCertificateResponse, error)
	DeleteAiGatewayDataPlaneCertificate(
		ctx context.Context,
		gatewayID string,
		certificateID string,
		opts ...kkOps.Option,
	) (*kkOps.DeleteAiGatewayDataPlaneCertificateResponse, error)
}

// AIGatewayDataPlaneCertificatesAPIImpl provides the real SDK implementation.
type AIGatewayDataPlaneCertificatesAPIImpl struct {
	SDK *kkSDK.SDK
}

func (a *AIGatewayDataPlaneCertificatesAPIImpl) ListAiGatewayDataPlaneCertificates(
	ctx context.Context,
	request kkOps.ListAiGatewayDataPlaneCertificatesRequest,
	opts ...kkOps.Option,
) (*kkOps.ListAiGatewayDataPlaneCertificatesResponse, error) {
	return a.SDK.AIGatewayDataPlaneCertificates.ListAiGatewayDataPlaneCertificates(ctx, request, opts...)
}

func (a *AIGatewayDataPlaneCertificatesAPIImpl) CreateAiGatewayDataPlaneCertificate(
	ctx context.Context,
	gatewayID string,
	request kkComps.CreateAIGatewayDataPlaneCertificateRequest,
	opts ...kkOps.Option,
) (*kkOps.CreateAiGatewayDataPlaneCertificateResponse, error) {
	return a.SDK.AIGatewayDataPlaneCertificates.CreateAiGatewayDataPlaneCertificate(ctx, gatewayID, &request, opts...)
}

func (a *AIGatewayDataPlaneCertificatesAPIImpl) GetAiGatewayDataPlaneCertificate(
	ctx context.Context,
	gatewayID string,
	certificateID string,
	opts ...kkOps.Option,
) (*kkOps.GetAiGatewayDataPlaneCertificateResponse, error) {
	return a.SDK.AIGatewayDataPlaneCertificates.GetAiGatewayDataPlaneCertificate(
		ctx,
		gatewayID,
		certificateID,
		opts...,
	)
}

func (a *AIGatewayDataPlaneCertificatesAPIImpl) DeleteAiGatewayDataPlaneCertificate(
	ctx context.Context,
	gatewayID string,
	certificateID string,
	opts ...kkOps.Option,
) (*kkOps.DeleteAiGatewayDataPlaneCertificateResponse, error) {
	return a.SDK.AIGatewayDataPlaneCertificates.DeleteAiGatewayDataPlaneCertificate(ctx, gatewayID, certificateID, opts...)
}
