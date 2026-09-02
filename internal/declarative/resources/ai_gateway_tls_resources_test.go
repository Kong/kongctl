package resources

import (
	"strings"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAIGatewayTLSResourcesValidateRequiredPublicFields(t *testing.T) {
	certificate := AIGatewayCertificateResource{
		BaseResource: BaseResource{Ref: "runtime-cert"},
		AIGateway:    tags.RefPlaceholderPrefix + "gateway#id",
		Name:         "runtime-cert",
		Cert:         "public-certificate",
	}
	require.NoError(t, certificate.Validate())

	caCertificate := AIGatewayCACertificateResource{
		BaseResource: BaseResource{Ref: "root-ca"},
		AIGateway:    tags.RefPlaceholderPrefix + "gateway#id",
		Name:         "root-ca",
		Cert:         "public-ca",
	}
	require.NoError(t, caCertificate.Validate())

	sni := AIGatewaySNIResource{
		BaseResource: BaseResource{Ref: "api-sni"},
		AIGateway:    tags.RefPlaceholderPrefix + "gateway#id",
		Name:         "api-sni",
		DisplayName:  "API SNI",
		Hostname:     "*.example.test",
		Certificate:  tags.RefPlaceholderPrefix + "runtime-cert#name",
	}
	require.NoError(t, sni.Validate())
	assert.ElementsMatch(t, []ResourceRef{
		{Kind: ResourceTypeAIGateway, Ref: "gateway"},
		{Kind: ResourceTypeAIGatewayCertificate, Ref: "runtime-cert"},
	}, sni.GetDependencies())
}

func TestAIGatewayCertificateResponseMappingOmitsPrivateKeys(t *testing.T) {
	alternativeCertificate := "alternative-public-certificate"
	resource := AIGatewayCertificateResourceFromResponse("gateway", kkComps.AIGatewayCertificate{
		ID: "certificate-id", Name: "runtime-cert", Cert: "public-certificate",
		CertAlt: &alternativeCertificate,
	})

	assert.Equal(t, "runtime-cert", resource.Ref)
	assert.Equal(t, "public-certificate", resource.Cert)
	assert.Empty(t, resource.Key)
	assert.Nil(t, resource.KeyAlt)
	require.NoError(t, resource.Validate())
	payload := resource.PayloadMap()
	assert.NotContains(t, payload, SchemaFieldKey)
	assert.NotContains(t, payload, SchemaFieldKeyAlt)
}

func TestAIGatewaySNIRejectsWildcardsAtBothEnds(t *testing.T) {
	sni := AIGatewaySNIResource{
		BaseResource: BaseResource{Ref: "api-sni"}, AIGateway: "gateway", Name: "api-sni",
		DisplayName: "API SNI", Hostname: "*.example.*", Certificate: "runtime-cert",
	}
	require.ErrorContains(t, sni.Validate(), "hostname is invalid")
}

func TestAIGatewaySNIReportsNameLengthSeparately(t *testing.T) {
	sni := AIGatewaySNIResource{
		BaseResource: BaseResource{Ref: "api-sni"}, AIGateway: "gateway", Name: strings.Repeat("a", 257),
		DisplayName: "API SNI", Hostname: "api.example.test", Certificate: "runtime-cert",
	}
	require.ErrorContains(t, sni.Validate(), "must not exceed 256 characters")
}

func TestAIGatewaySNIRejectsNonStringHostnameFromResponse(t *testing.T) {
	_, err := AIGatewaySNIResourceFromResponse("gateway", kkComps.AIGatewaySNI{
		ID: "sni-id", Name: "api-sni", DisplayName: "API SNI", Hostname: 42, Certificate: "runtime-cert",
	})
	require.ErrorContains(t, err, "hostname")
}
