package loader

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoaderLoadsSecretsDocumentationExample(t *testing.T) {
	path := filepath.Join("..", "..", "..", "docs", "examples", "declarative", "secrets", "config.yaml")

	rs, err := New().LoadFile(path)
	require.NoError(t, err)
	require.Len(t, rs.DCRProviders, 1)
	require.Len(t, rs.AIGateways, 1)
	require.Len(t, rs.AIGatewayProviders, 1)
	require.Len(t, rs.AIGatewayConsumers, 1)
	require.Len(t, rs.AIGatewayConsumerCredentials, 1)

	dcrSecret := rs.GetSecretSources("secrets-http-dcr")["/dcr_config/api_key"]
	require.Len(t, dcrSecret.Expression.Parts, 1)
	assert.Equal(t, "DCR_API_KEY", dcrSecret.Expression.Parts[0].Source.Reference)

	providerSecret := rs.GetSecretSources("secrets-openai-provider")["/config/auth/headers/0/value"]
	require.Len(t, providerSecret.Expression.Parts, 2)
	assert.Equal(t, "Bearer ", *providerSecret.Expression.Parts[0].Literal)
	assert.Equal(t, "OPENAI_API_KEY", providerSecret.Expression.Parts[1].Source.Reference)

	credentialSecret := rs.GetSecretSources("secrets-client-key")["/api_key"]
	require.Len(t, credentialSecret.Expression.Parts, 1)
	assert.Equal(t, "CLIENT_API_KEY", credentialSecret.Expression.Parts[0].Source.Reference)
}
