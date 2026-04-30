package helpers

import (
	"context"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

type DataPlaneCertificateAPI interface {
	ListDpClientCertificates(
		ctx context.Context,
		controlPlaneID string,
		opts ...kkOps.Option,
	) (*kkOps.ListDpClientCertificatesResponse, error)
	CreateDataplaneCertificate(
		ctx context.Context,
		controlPlaneID string,
		request *kkComps.DataPlaneClientCertificateRequest,
		opts ...kkOps.Option,
	) (*kkOps.CreateDataplaneCertificateResponse, error)
	GetDataplaneCertificate(
		ctx context.Context,
		controlPlaneID string,
		certificateID string,
		opts ...kkOps.Option,
	) (*kkOps.GetDataplaneCertificateResponse, error)
	DeleteDataplaneCertificate(
		ctx context.Context,
		controlPlaneID string,
		certificateID string,
		opts ...kkOps.Option,
	) (*kkOps.DeleteDataplaneCertificateResponse, error)
}

type DataPlaneCertificateAPIImpl struct {
	SDK *kkSDK.SDK
}

func (a *DataPlaneCertificateAPIImpl) ListDpClientCertificates(
	ctx context.Context,
	controlPlaneID string,
	opts ...kkOps.Option,
) (*kkOps.ListDpClientCertificatesResponse, error) {
	return a.SDK.DPCertificates.ListDpClientCertificates(ctx, controlPlaneID, opts...)
}

func (a *DataPlaneCertificateAPIImpl) CreateDataplaneCertificate(
	ctx context.Context,
	controlPlaneID string,
	request *kkComps.DataPlaneClientCertificateRequest,
	opts ...kkOps.Option,
) (*kkOps.CreateDataplaneCertificateResponse, error) {
	return a.SDK.DPCertificates.CreateDataplaneCertificate(ctx, controlPlaneID, request, opts...)
}

func (a *DataPlaneCertificateAPIImpl) GetDataplaneCertificate(
	ctx context.Context,
	controlPlaneID string,
	certificateID string,
	opts ...kkOps.Option,
) (*kkOps.GetDataplaneCertificateResponse, error) {
	return a.SDK.DPCertificates.GetDataplaneCertificate(ctx, controlPlaneID, certificateID, opts...)
}

func (a *DataPlaneCertificateAPIImpl) DeleteDataplaneCertificate(
	ctx context.Context,
	controlPlaneID string,
	certificateID string,
	opts ...kkOps.Option,
) (*kkOps.DeleteDataplaneCertificateResponse, error) {
	return a.SDK.DPCertificates.DeleteDataplaneCertificate(ctx, controlPlaneID, certificateID, opts...)
}
