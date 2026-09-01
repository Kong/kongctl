package helpers

import (
	"context"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

type AIGatewayCertificatesAPI interface {
	ListAiGatewayCertificates(
		context.Context, kkOps.ListAiGatewayCertificatesRequest, ...kkOps.Option,
	) (*kkOps.ListAiGatewayCertificatesResponse, error)
	CreateAiGatewayCertificate(
		context.Context, string, kkComps.CreateAIGatewayCertificateRequest, ...kkOps.Option,
	) (*kkOps.CreateAiGatewayCertificateResponse, error)
	GetAiGatewayCertificate(
		context.Context, string, string, ...kkOps.Option,
	) (*kkOps.GetAiGatewayCertificateResponse, error)
	UpdateAiGatewayCertificate(
		context.Context, kkOps.UpdateAiGatewayCertificateRequest, ...kkOps.Option,
	) (*kkOps.UpdateAiGatewayCertificateResponse, error)
	DeleteAiGatewayCertificate(
		context.Context, string, string, ...kkOps.Option,
	) (*kkOps.DeleteAiGatewayCertificateResponse, error)
}

type AIGatewayCACertificatesAPI interface {
	ListAiGatewayCaCertificates(
		context.Context, kkOps.ListAiGatewayCaCertificatesRequest, ...kkOps.Option,
	) (*kkOps.ListAiGatewayCaCertificatesResponse, error)
	CreateAiGatewayCaCertificate(
		context.Context, string, kkComps.CreateAIGatewayCACertificateRequest, ...kkOps.Option,
	) (*kkOps.CreateAiGatewayCaCertificateResponse, error)
	GetAiGatewayCaCertificate(
		context.Context, string, string, ...kkOps.Option,
	) (*kkOps.GetAiGatewayCaCertificateResponse, error)
	UpdateAiGatewayCaCertificate(
		context.Context, kkOps.UpdateAiGatewayCaCertificateRequest, ...kkOps.Option,
	) (*kkOps.UpdateAiGatewayCaCertificateResponse, error)
	DeleteAiGatewayCaCertificate(
		context.Context, string, string, ...kkOps.Option,
	) (*kkOps.DeleteAiGatewayCaCertificateResponse, error)
}

type AIGatewaySNIsAPI interface {
	ListAiGatewaySnis(
		context.Context, kkOps.ListAiGatewaySnisRequest, ...kkOps.Option,
	) (*kkOps.ListAiGatewaySnisResponse, error)
	CreateAiGatewaySni(
		context.Context, string, kkComps.CreateAIGatewaySNIRequest, ...kkOps.Option,
	) (*kkOps.CreateAiGatewaySniResponse, error)
	GetAiGatewaySni(context.Context, string, string, ...kkOps.Option) (*kkOps.GetAiGatewaySniResponse, error)
	UpdateAiGatewaySni(
		context.Context, kkOps.UpdateAiGatewaySniRequest, ...kkOps.Option,
	) (*kkOps.UpdateAiGatewaySniResponse, error)
	DeleteAiGatewaySni(context.Context, string, string, ...kkOps.Option) (*kkOps.DeleteAiGatewaySniResponse, error)
}

type AIGatewayCertificatesAPIImpl struct{ SDK *kkSDK.SDK }

func (a *AIGatewayCertificatesAPIImpl) ListAiGatewayCertificates(
	ctx context.Context, req kkOps.ListAiGatewayCertificatesRequest, opts ...kkOps.Option,
) (*kkOps.ListAiGatewayCertificatesResponse, error) {
	return a.SDK.AIGatewayCertificates.ListAiGatewayCertificates(ctx, req, opts...)
}

func (a *AIGatewayCertificatesAPIImpl) CreateAiGatewayCertificate(
	ctx context.Context, gatewayID string, req kkComps.CreateAIGatewayCertificateRequest, opts ...kkOps.Option,
) (*kkOps.CreateAiGatewayCertificateResponse, error) {
	return a.SDK.AIGatewayCertificates.CreateAiGatewayCertificate(ctx, gatewayID, req, opts...)
}

func (a *AIGatewayCertificatesAPIImpl) GetAiGatewayCertificate(
	ctx context.Context, gatewayID, id string, opts ...kkOps.Option,
) (*kkOps.GetAiGatewayCertificateResponse, error) {
	return a.SDK.AIGatewayCertificates.GetAiGatewayCertificate(ctx, gatewayID, id, opts...)
}

func (a *AIGatewayCertificatesAPIImpl) UpdateAiGatewayCertificate(
	ctx context.Context, req kkOps.UpdateAiGatewayCertificateRequest, opts ...kkOps.Option,
) (*kkOps.UpdateAiGatewayCertificateResponse, error) {
	return a.SDK.AIGatewayCertificates.UpdateAiGatewayCertificate(ctx, req, opts...)
}

func (a *AIGatewayCertificatesAPIImpl) DeleteAiGatewayCertificate(
	ctx context.Context, gatewayID, id string, opts ...kkOps.Option,
) (*kkOps.DeleteAiGatewayCertificateResponse, error) {
	return a.SDK.AIGatewayCertificates.DeleteAiGatewayCertificate(ctx, gatewayID, id, opts...)
}

type AIGatewayCACertificatesAPIImpl struct{ SDK *kkSDK.SDK }

func (a *AIGatewayCACertificatesAPIImpl) ListAiGatewayCaCertificates(
	ctx context.Context, req kkOps.ListAiGatewayCaCertificatesRequest, opts ...kkOps.Option,
) (*kkOps.ListAiGatewayCaCertificatesResponse, error) {
	return a.SDK.AIGatewayCACertificates.ListAiGatewayCaCertificates(ctx, req, opts...)
}

func (a *AIGatewayCACertificatesAPIImpl) CreateAiGatewayCaCertificate(
	ctx context.Context, gatewayID string, req kkComps.CreateAIGatewayCACertificateRequest, opts ...kkOps.Option,
) (*kkOps.CreateAiGatewayCaCertificateResponse, error) {
	return a.SDK.AIGatewayCACertificates.CreateAiGatewayCaCertificate(ctx, gatewayID, req, opts...)
}

func (a *AIGatewayCACertificatesAPIImpl) GetAiGatewayCaCertificate(
	ctx context.Context, gatewayID, id string, opts ...kkOps.Option,
) (*kkOps.GetAiGatewayCaCertificateResponse, error) {
	return a.SDK.AIGatewayCACertificates.GetAiGatewayCaCertificate(ctx, gatewayID, id, opts...)
}

func (a *AIGatewayCACertificatesAPIImpl) UpdateAiGatewayCaCertificate(
	ctx context.Context, req kkOps.UpdateAiGatewayCaCertificateRequest, opts ...kkOps.Option,
) (*kkOps.UpdateAiGatewayCaCertificateResponse, error) {
	return a.SDK.AIGatewayCACertificates.UpdateAiGatewayCaCertificate(ctx, req, opts...)
}

func (a *AIGatewayCACertificatesAPIImpl) DeleteAiGatewayCaCertificate(
	ctx context.Context, gatewayID, id string, opts ...kkOps.Option,
) (*kkOps.DeleteAiGatewayCaCertificateResponse, error) {
	return a.SDK.AIGatewayCACertificates.DeleteAiGatewayCaCertificate(ctx, gatewayID, id, opts...)
}

type AIGatewaySNIsAPIImpl struct{ SDK *kkSDK.SDK }

func (a *AIGatewaySNIsAPIImpl) ListAiGatewaySnis(
	ctx context.Context, req kkOps.ListAiGatewaySnisRequest, opts ...kkOps.Option,
) (*kkOps.ListAiGatewaySnisResponse, error) {
	return a.SDK.AIGatewaySNIs.ListAiGatewaySnis(ctx, req, opts...)
}

func (a *AIGatewaySNIsAPIImpl) CreateAiGatewaySni(
	ctx context.Context, gatewayID string, req kkComps.CreateAIGatewaySNIRequest, opts ...kkOps.Option,
) (*kkOps.CreateAiGatewaySniResponse, error) {
	return a.SDK.AIGatewaySNIs.CreateAiGatewaySni(ctx, gatewayID, req, opts...)
}

func (a *AIGatewaySNIsAPIImpl) GetAiGatewaySni(
	ctx context.Context, gatewayID, id string, opts ...kkOps.Option,
) (*kkOps.GetAiGatewaySniResponse, error) {
	return a.SDK.AIGatewaySNIs.GetAiGatewaySni(ctx, gatewayID, id, opts...)
}

func (a *AIGatewaySNIsAPIImpl) UpdateAiGatewaySni(
	ctx context.Context, req kkOps.UpdateAiGatewaySniRequest, opts ...kkOps.Option,
) (*kkOps.UpdateAiGatewaySniResponse, error) {
	return a.SDK.AIGatewaySNIs.UpdateAiGatewaySni(ctx, req, opts...)
}

func (a *AIGatewaySNIsAPIImpl) DeleteAiGatewaySni(
	ctx context.Context, gatewayID, id string, opts ...kkOps.Option,
) (*kkOps.DeleteAiGatewaySniResponse, error) {
	return a.SDK.AIGatewaySNIs.DeleteAiGatewaySni(ctx, gatewayID, id, opts...)
}
