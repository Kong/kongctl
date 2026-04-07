package executor

import (
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildVirtualClusterAuthentication_ClientCertificate(t *testing.T) {
	input := []any{
		map[string]any{
			"type": "client_certificate",
		},
	}

	result, err := buildVirtualClusterAuthentication(input)
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, kkComps.VirtualClusterAuthenticationSchemeTypeClientCertificate, result[0].Type)
	assert.NotNil(t, result[0].VirtualClusterAuthenticationClientCertificate)
}

func TestBuildVirtualClusterAuthentication_UnsupportedType(t *testing.T) {
	input := []any{
		map[string]any{
			"type": "unknown_type",
		},
	}

	_, err := buildVirtualClusterAuthentication(input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported authentication type")
}

func TestConvertToVirtualClusterSensitiveDataAwareAuth_ClientCertificate(t *testing.T) {
	auth := kkComps.CreateVirtualClusterAuthenticationSchemeClientCertificate(
		kkComps.VirtualClusterAuthenticationClientCertificate{},
	)

	result, err := convertToVirtualClusterSensitiveDataAwareAuth(auth)
	require.NoError(t, err)
	assert.Equal(t, kkComps.VirtualClusterAuthenticationSensitiveDataAwareSchemeTypeClientCertificate, result.Type)
	assert.NotNil(t, result.VirtualClusterAuthenticationClientCertificate)
}

func TestConvertToVirtualClusterSensitiveDataAwareAuth_ClientCertificate_MissingData(t *testing.T) {
	auth := kkComps.VirtualClusterAuthenticationScheme{
		Type: kkComps.VirtualClusterAuthenticationSchemeTypeClientCertificate,
		// VirtualClusterAuthenticationClientCertificate is intentionally nil
	}

	_, err := convertToVirtualClusterSensitiveDataAwareAuth(auth)
	require.Error(t, err)
	assert.Equal(t, "Client certificate authentication data is missing", err.Error())
}
