package state

import (
	"context"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type dataPlaneCertificateAPIMock struct {
	list   func(context.Context, string) (*kkOps.ListDpClientCertificatesResponse, error)
	create func(
		context.Context,
		string,
		*kkComps.DataPlaneClientCertificateRequest,
	) (*kkOps.CreateDataplaneCertificateResponse, error)
	get    func(context.Context, string, string) (*kkOps.GetDataplaneCertificateResponse, error)
	delete func(context.Context, string, string) (*kkOps.DeleteDataplaneCertificateResponse, error)
}

func (m *dataPlaneCertificateAPIMock) ListDpClientCertificates(
	ctx context.Context,
	controlPlaneID string,
	_ ...kkOps.Option,
) (*kkOps.ListDpClientCertificatesResponse, error) {
	if m.list == nil {
		return nil, nil
	}
	return m.list(ctx, controlPlaneID)
}

func (m *dataPlaneCertificateAPIMock) CreateDataplaneCertificate(
	ctx context.Context,
	controlPlaneID string,
	request *kkComps.DataPlaneClientCertificateRequest,
	_ ...kkOps.Option,
) (*kkOps.CreateDataplaneCertificateResponse, error) {
	if m.create == nil {
		return nil, nil
	}
	return m.create(ctx, controlPlaneID, request)
}

func (m *dataPlaneCertificateAPIMock) GetDataplaneCertificate(
	ctx context.Context,
	controlPlaneID string,
	certificateID string,
	_ ...kkOps.Option,
) (*kkOps.GetDataplaneCertificateResponse, error) {
	if m.get == nil {
		return nil, nil
	}
	return m.get(ctx, controlPlaneID, certificateID)
}

func (m *dataPlaneCertificateAPIMock) DeleteDataplaneCertificate(
	ctx context.Context,
	controlPlaneID string,
	certificateID string,
	_ ...kkOps.Option,
) (*kkOps.DeleteDataplaneCertificateResponse, error) {
	if m.delete == nil {
		return nil, nil
	}
	return m.delete(ctx, controlPlaneID, certificateID)
}

func TestClientListControlPlaneDataPlaneCertificates(t *testing.T) {
	certID := "cert-id"
	certValue := "cert"
	client := NewClient(ClientConfig{
		DataPlaneCertificateAPI: &dataPlaneCertificateAPIMock{
			list: func(_ context.Context, controlPlaneID string) (*kkOps.ListDpClientCertificatesResponse, error) {
				assert.Equal(t, "cp-id", controlPlaneID)
				return &kkOps.ListDpClientCertificatesResponse{
					ListDataPlaneCertificatesResponse: &kkComps.ListDataPlaneCertificatesResponse{
						Items: []kkComps.DataPlaneClientCertificate{
							{
								ID:   &certID,
								Cert: &certValue,
							},
						},
					},
				}, nil
			},
		},
	})

	certs, err := client.ListControlPlaneDataPlaneCertificates(t.Context(), "cp-id")
	require.NoError(t, err)
	require.Len(t, certs, 1)
	assert.Equal(t, certID, *certs[0].ID)
	assert.Equal(t, certValue, *certs[0].Cert)
}

func TestClientCreateControlPlaneDataPlaneCertificate(t *testing.T) {
	certID := "new-cert-id"
	client := NewClient(ClientConfig{
		DataPlaneCertificateAPI: &dataPlaneCertificateAPIMock{
			create: func(
				_ context.Context,
				controlPlaneID string,
				request *kkComps.DataPlaneClientCertificateRequest,
			) (*kkOps.CreateDataplaneCertificateResponse, error) {
				assert.Equal(t, "cp-id", controlPlaneID)
				require.NotNil(t, request)
				assert.Equal(t, "cert", request.Cert)
				return &kkOps.CreateDataplaneCertificateResponse{
					DataPlaneClientCertificateResponse: &kkComps.DataPlaneClientCertificateResponse{
						Item: &kkComps.DataPlaneClientCertificate{ID: &certID},
					},
				}, nil
			},
		},
	})

	id, err := client.CreateControlPlaneDataPlaneCertificate(
		t.Context(),
		"cp-id",
		kkComps.DataPlaneClientCertificateRequest{Cert: "cert"},
		"default",
	)
	require.NoError(t, err)
	assert.Equal(t, certID, id)
}

func TestClientDeleteControlPlaneDataPlaneCertificate(t *testing.T) {
	client := NewClient(ClientConfig{
		DataPlaneCertificateAPI: &dataPlaneCertificateAPIMock{
			delete: func(
				_ context.Context,
				controlPlaneID string,
				certificateID string,
			) (*kkOps.DeleteDataplaneCertificateResponse, error) {
				assert.Equal(t, "cp-id", controlPlaneID)
				assert.Equal(t, "cert-id", certificateID)
				return &kkOps.DeleteDataplaneCertificateResponse{}, nil
			},
		},
	})

	err := client.DeleteControlPlaneDataPlaneCertificate(t.Context(), "cp-id", "cert-id")
	require.NoError(t, err)
}
