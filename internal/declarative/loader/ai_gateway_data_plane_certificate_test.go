package loader

import (
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/stretchr/testify/require"
)

const aiGatewayDataPlaneCertificateYAML = `
ai_gateways:
  - ref: support-gateway
    display_name: Support Gateway
    data_plane_certificates:
      - ref: support-data-plane-cert
        title: support-data-plane-cert
        description: Support data plane certificate
        cert: |
          -----BEGIN CERTIFICATE-----
          test
          -----END CERTIFICATE-----
`

func TestLoaderExtractsNestedAIGatewayDataPlaneCertificates(t *testing.T) {
	path := writeLoaderTestFile(t, aiGatewayDataPlaneCertificateYAML)

	rs, err := New().LoadFromSources([]Source{{Path: path, Type: SourceTypeFile}}, false)
	require.NoError(t, err)
	require.Len(t, rs.AIGateways, 1)
	require.Empty(t, rs.AIGateways[0].DataPlaneCertificates)
	require.Len(t, rs.AIGatewayDataPlaneCertificates, 1)
	require.Equal(t, "support-gateway", rs.AIGatewayDataPlaneCertificates[0].AIGateway)
	require.Equal(t, "support-data-plane-cert", rs.AIGatewayDataPlaneCertificates[0].Title)
	require.True(t, rs.SyncScope.ChildInScope(
		resources.ResourceTypeAIGateway,
		"support-gateway",
		resources.ResourceTypeAIGatewayDataPlaneCertificate,
	))
}

func TestLoaderValidatesAIGatewayDataPlaneCertificateParentAndDuplicateTitles(t *testing.T) {
	rootOnly := `
ai_gateway_data_plane_certificates:
  - ref: support-data-plane-cert
    ai_gateway: missing-gateway
    title: support-data-plane-cert
    cert: test-cert
`
	_, err := New().LoadFromSources([]Source{{Path: writeLoaderTestFile(t, rootOnly), Type: SourceTypeFile}}, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "references unknown ai_gateway")

	duplicates := `
ai_gateways:
  - ref: support-gateway
    display_name: Support Gateway
    data_plane_certificates:
      - ref: support-data-plane-cert
        title: support-data-plane-cert
        cert: test-cert
      - ref: support-data-plane-cert-2
        title: support-data-plane-cert
        cert: test-cert-2
`
	_, err = New().LoadFromSources([]Source{{Path: writeLoaderTestFile(t, duplicates), Type: SourceTypeFile}}, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicate ai_gateway_data_plane_certificate title")
}

func TestLoaderRejectsRootLevelEmptyAIGatewayDataPlaneCertificates(t *testing.T) {
	input := `ai_gateway_data_plane_certificates: []`

	_, err := New().LoadFromSources([]Source{{Path: writeLoaderTestFile(t, input), Type: SourceTypeFile}}, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "ai_gateway_data_plane_certificates cannot be empty")
}

func TestLoaderAcceptsAIGatewayDataPlaneCertificateDeferredExternalParentRef(t *testing.T) {
	input := `
ai_gateways:
  - ref: external-shared-gateway
    _external:
      selector:
        matchFields:
          display_name: Shared Gateway
ai_gateway_data_plane_certificates:
  - ref: support-data-plane-cert
    ai_gateway: !ref external-shared-gateway#id
    title: support-data-plane-cert
    cert: test-cert
`

	rs, err := New().LoadFromSources([]Source{{Path: writeLoaderTestFile(t, input), Type: SourceTypeFile}}, false)
	require.NoError(t, err)
	require.Len(t, rs.AIGatewayDataPlaneCertificates, 1)
	require.Equal(
		t,
		tags.RefPlaceholderPrefix+"external-shared-gateway#id",
		rs.AIGatewayDataPlaneCertificates[0].AIGateway,
	)
	require.True(t, rs.SyncScope.ChildInScope(
		resources.ResourceTypeAIGateway,
		"external-shared-gateway",
		resources.ResourceTypeAIGatewayDataPlaneCertificate,
	))
}
