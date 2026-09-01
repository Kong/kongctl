package loader

import (
	"strings"
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoaderExtractsNestedAIGatewayTLSResourcesAndPreservesSecretSources(t *testing.T) {
	config := `
ai_gateways:
  - ref: gateway
    name: gateway
    display_name: Gateway
    certificates:
      - ref: runtime-cert
        name: runtime-cert
        cert: public-certificate
        key: !secret {source: !env RUNTIME_PRIVATE_KEY}
    ca_certificates:
      - ref: root-ca
        name: root-ca
        cert: public-ca
    snis:
      - ref: api-sni
        name: api-sni
        display_name: API SNI
        hostname: '*.example.test'
        certificate: !ref runtime-cert#name
`

	rs, err := New().LoadFile(writeLoaderTestFile(t, config))
	require.NoError(t, err)
	require.Len(t, rs.AIGatewayCertificates, 1)
	require.Len(t, rs.AIGatewayCACertificates, 1)
	require.Len(t, rs.AIGatewaySNIs, 1)
	assert.Equal(t, "gateway", rs.AIGatewayCertificates[0].AIGateway)
	assert.Equal(t, "gateway", rs.AIGatewayCACertificates[0].AIGateway)
	assert.Equal(t, "gateway", rs.AIGatewaySNIs[0].AIGateway)
	assert.Equal(t, tags.RefPlaceholderPrefix+"runtime-cert#name", rs.AIGatewaySNIs[0].Certificate)
	assert.True(t, tags.IsSecretPlaceholder(rs.AIGatewayCertificates[0].Key))
	secretSource := rs.GetSecretSources("runtime-cert")["/key"].Expression.Parts[0].Source
	assert.Equal(t, "RUNTIME_PRIVATE_KEY", secretSource.Reference)
}

func TestLoaderCapturesNestedAIGatewayTLSSyncScopes(t *testing.T) {
	rs, err := New().parseYAML(strings.NewReader(`
ai_gateways:
  - ref: gateway
    name: gateway
    display_name: Gateway
    certificates: []
    ca_certificates: []
    snis: []
`), "test.yaml", ".")
	require.NoError(t, err)

	for _, resourceType := range []resources.ResourceType{
		resources.ResourceTypeAIGatewayCertificate,
		resources.ResourceTypeAIGatewayCACertificate,
		resources.ResourceTypeAIGatewaySNI,
	} {
		assert.True(t, rs.SyncScope.ChildInScope(resources.ResourceTypeAIGateway, "gateway", resourceType))
	}
}

func TestLoaderRejectsRootLevelEmptyAIGatewayTLSCollections(t *testing.T) {
	for _, key := range []string{"ai_gateway_certificates", "ai_gateway_ca_certificates", "ai_gateway_snis"} {
		t.Run(key, func(t *testing.T) {
			_, err := New().LoadFromSources([]Source{{
				Path: writeLoaderTestFile(t, key+": []"), Type: SourceTypeFile,
			}}, false)
			require.ErrorContains(t, err, key+" cannot be empty")
		})
	}
}

func TestLoaderAcceptsRootLevelAIGatewayTLSResources(t *testing.T) {
	config := `
ai_gateways:
  - ref: gateway
    name: gateway
    display_name: Gateway
ai_gateway_certificates:
  - ref: runtime-cert
    ai_gateway: !ref gateway
    name: runtime-cert
    cert: public-certificate
    key: !secret {source: !env RUNTIME_PRIVATE_KEY}
ai_gateway_ca_certificates:
  - ref: root-ca
    ai_gateway: !ref gateway
    name: root-ca
    cert: public-ca
ai_gateway_snis:
  - ref: api-sni
    ai_gateway: !ref gateway
    name: api-sni
    display_name: API SNI
    hostname: example.test
    certificate: !ref runtime-cert#name
`

	rs, err := New().LoadFile(writeLoaderTestFile(t, config))
	require.NoError(t, err)
	assert.Len(t, rs.AIGatewayCertificates, 1)
	assert.Len(t, rs.AIGatewayCACertificates, 1)
	assert.Len(t, rs.AIGatewaySNIs, 1)
}
