package resources

import (
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestControlPlaneDataPlaneCertificateResourceValidate(t *testing.T) {
	cert := ControlPlaneDataPlaneCertificateResource{
		Ref:          "dp-cert",
		ControlPlane: "cp",
		Cert:         "-----BEGIN CERTIFICATE-----\nCERT\n-----END CERTIFICATE-----",
	}

	require.NoError(t, cert.Validate())
}

func TestControlPlaneDataPlaneCertificateResourceValidateRequiresFields(t *testing.T) {
	tests := []struct {
		name    string
		cert    ControlPlaneDataPlaneCertificateResource
		wantErr string
	}{
		{
			name: "missing ref",
			cert: ControlPlaneDataPlaneCertificateResource{
				ControlPlane: "cp",
				Cert:         "cert",
			},
			wantErr: "ref cannot be empty",
		},
		{
			name: "missing control plane",
			cert: ControlPlaneDataPlaneCertificateResource{
				Ref:  "dp-cert",
				Cert: "cert",
			},
			wantErr: "control_plane is required",
		},
		{
			name: "missing cert",
			cert: ControlPlaneDataPlaneCertificateResource{
				Ref:          "dp-cert",
				ControlPlane: "cp",
			},
			wantErr: "cert is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cert.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestControlPlaneDataPlaneCertificateIdentityUsesExactCertificateContents(t *testing.T) {
	cert := "-----BEGIN CERTIFICATE-----\nCERT\n-----END CERTIFICATE-----"
	sameCert := "-----BEGIN CERTIFICATE-----\nCERT\n-----END CERTIFICATE-----"
	certWithTrailingNewline := cert + "\n"

	assert.Equal(
		t,
		ControlPlaneDataPlaneCertificateIdentity(cert),
		ControlPlaneDataPlaneCertificateIdentity(sameCert),
	)
	assert.NotEqual(
		t,
		ControlPlaneDataPlaneCertificateIdentity(cert),
		ControlPlaneDataPlaneCertificateIdentity(certWithTrailingNewline),
	)
	assert.Len(t, ShortControlPlaneDataPlaneCertificateIdentity(cert), 12)
}

func TestControlPlaneDataPlaneCertificateTryMatchKonnectResource(t *testing.T) {
	id := "cert-id"
	certValue := "-----BEGIN CERTIFICATE-----\nCERT\n-----END CERTIFICATE-----"
	resource := ControlPlaneDataPlaneCertificateResource{
		Ref:          "dp-cert",
		ControlPlane: "cp",
		Cert:         certValue,
	}

	matched := resource.TryMatchKonnectResource(kkComps.DataPlaneClientCertificate{
		ID:   &id,
		Cert: &certValue,
	})

	require.True(t, matched)
	assert.Equal(t, id, resource.GetKonnectID())
}

func TestControlPlaneDataPlaneCertificateTryMatchKonnectResourceRejectsDifferentCert(t *testing.T) {
	id := "cert-id"
	certValue := "-----BEGIN CERTIFICATE-----\nCERT\n-----END CERTIFICATE-----"
	otherCert := certValue + "\n"
	resource := ControlPlaneDataPlaneCertificateResource{
		Ref:          "dp-cert",
		ControlPlane: "cp",
		Cert:         certValue,
	}

	assert.False(t, resource.TryMatchKonnectResource(kkComps.DataPlaneClientCertificate{
		ID:   &id,
		Cert: &otherCert,
	}))
	assert.Empty(t, resource.GetKonnectID())
}
