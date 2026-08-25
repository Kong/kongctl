package loader

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kong/kongctl/internal/declarative/tags"
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

func TestLoaderLoadsOpenAILLMDocumentationExample(t *testing.T) {
	sourcePath := filepath.Join(
		"..", "..", "..", "docs", "examples", "declarative", "ai-gateway", "openai-llm", "ai-gateway.yaml",
	)
	contents, err := os.ReadFile(sourcePath)
	require.NoError(t, err)

	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "certs"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ai-gateway.yaml"), contents, 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "certs", "data-plane.crt"), []byte("test certificate"), 0o600))

	rs, err := New().LoadFile(filepath.Join(dir, "ai-gateway.yaml"))
	require.NoError(t, err)
	require.Len(t, rs.AIGateways, 1)
	require.Len(t, rs.AIGatewayDataPlaneCertificates, 1)
	require.Len(t, rs.AIGatewayProviders, 1)
	require.Len(t, rs.AIGatewayModels, 1)

	providerSecret := rs.GetSecretSources("openai")["/config/auth/headers/0/value"]
	require.Len(t, providerSecret.Expression.Parts, 2)
	assert.Equal(t, "Bearer ", *providerSecret.Expression.Parts[0].Literal)
	assert.Equal(t, "OPENAI_API_KEY", providerSecret.Expression.Parts[1].Source.Reference)
}

func TestLoaderLoadsAIGatewayTemplatesDocumentationExample(t *testing.T) {
	baseDir := filepath.Join("..", "..", "..", "docs", "examples", "declarative", "ai-gateway", "templates")

	rs, err := NewWithBaseDir(baseDir).LoadFromSources([]Source{
		{Path: filepath.Join(baseDir, "costs.yaml"), Type: SourceTypeFile},
		{Path: filepath.Join(baseDir, "ai-gateway.yaml"), Type: SourceTypeFile},
	}, false)
	require.NoError(t, err)
	require.Len(t, rs.AIGateways, 1)
	require.Len(t, rs.AIGatewayProviders, 1)
	require.Len(t, rs.AIGatewayModels, 2)
	require.Len(t, rs.AIGatewayPolicies, 1)

	providerSecret := rs.GetSecretSources("openai-provider")["/config/auth/headers/0/value"]
	require.Len(t, providerSecret.Expression.Parts, 2)
	assert.Equal(t, "Bearer ", *providerSecret.Expression.Parts[0].Literal)
	assert.Equal(t, "OPENAI_API_KEY", providerSecret.Expression.Parts[1].Source.Reference)

	for _, model := range rs.AIGatewayModels {
		require.NotNil(t, model.AIGatewayModelModel)
		require.Len(t, model.AIGatewayModelModel.Targets, 1)
		assert.Equal(
			t,
			[]string{tags.RefPlaceholderPrefix + "hourly-model-cost-budget#name"},
			model.AIGatewayModelModel.Policies,
		)

		config := model.AIGatewayModelModel.Targets[0].GetConfigOpenai()
		require.NotNil(t, config)
		require.NotNil(t, config.InputCost)
		assert.InDelta(t, 2.5, *config.InputCost, 0.001)
		require.NotNil(t, config.OutputCost)
		assert.InDelta(t, 10.0, *config.OutputCost, 0.001)
	}

	policy := rs.AIGatewayPolicies[0]
	assert.Equal(t, "ai-rate-limiting-advanced", policy.Type)
	assert.Equal(t, "local", policy.Config["strategy"])
	policies := policy.Config["policies"].([]any)
	limits := policies[0].(map[string]any)["limits"].([]any)
	limit := limits[0].(map[string]any)
	assert.Equal(t, "cost", limit["tokens_count_strategy"])
	assert.InDelta(t, 25, limit["limit"], 0.0001)
}
