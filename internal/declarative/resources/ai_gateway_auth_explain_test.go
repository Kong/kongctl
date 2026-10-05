package resources

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAIGatewayTokenExchangeExplainPaths(t *testing.T) {
	const base = "ai_gateway.auth_strategies.config.token_exchange"
	tests := []struct {
		path     string
		kind     string
		required bool
	}{
		{"", explainKindObject, false},
		{"subject_token_issuers", explainKindObject, true},
		{"subject_token_issuers.issuer", explainKindString, true},
		{"subject_token_issuers.conditions", explainKindObject, false},
		{"subject_token_issuers.conditions.has_audience", explainKindArray, false},
		{"subject_token_issuers.conditions.missing_audience", explainKindArray, false},
		{"subject_token_issuers.conditions.has_scopes", explainKindArray, false},
		{"subject_token_issuers.conditions.missing_scopes", explainKindArray, false},
		{"request", explainKindObject, false},
		{"request.scopes", explainKindArray, false},
		{"request.audience", explainKindArray, false},
		{"request.empty_scopes", explainKindBoolean, false},
		{"cache", explainKindObject, false},
		{"cache.enabled", explainKindBoolean, false},
		{"cache.ttl", explainKindInteger, false},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			path := base
			if tt.path != "" {
				path += "." + tt.path
			}
			subject, err := ResolveExplainSubject(path)
			require.NoError(t, err)
			assert.Equal(t, tt.kind, subject.Node.Kind)
			assert.Equal(t, tt.required, subject.FieldRequired)
			if tt.kind == explainKindArray && tt.path != "subject_token_issuers" {
				assert.Equal(t, explainKindString, subject.Node.Items.Kind)
			}
		})
	}
	subject, err := ResolveExplainSubject("ai_gateway.auth_strategies.config")
	require.NoError(t, err)
	text := RenderExplainText(subject, true)
	assert.Contains(t, text, "- token_exchange: map[string] optional")
	assert.Contains(t, text, "- subject_token_issuers:")
	conditions, err := ResolveExplainSubject(base + ".subject_token_issuers.conditions")
	require.NoError(t, err)
	assert.Contains(t, RenderExplainSchema(conditions).Description,
		"Required and non-empty when exchanging a token from the same issuer.")
	_, err = ResolveExplainSubject(base + ".grant_type")
	require.Error(t, err)

	node, err := aiGatewayAuthStrategyExplainNode(ExplainBuildContext{})
	require.NoError(t, err)
	keyAuth := aiGatewayAuthStrategyExplainBranch(t, node, "key-auth")
	_, ok := keyAuth.lookup([]string{"config", "token_exchange"})
	assert.False(t, ok, "token exchange applies only to OpenID Connect")
}

func TestAIGatewayProviderAuthShapesDistinguishBedrockAndSagemaker(t *testing.T) {
	node, err := aiGatewayProviderExplainNode(ExplainBuildContext{})
	require.NoError(t, err)
	bedrock := explainUnionBranchByType(t, node, "bedrock")
	auth, ok := bedrock.lookup([]string{"config", "auth"})
	require.True(t, ok)
	aws := explainUnionBranchByType(t, auth, "aws")
	assert.False(t, aws.propertyExists("aws"))
	for _, field := range []string{"access_key_id", "secret_access_key", "session_token"} {
		assert.True(t, aws.propertyExists(field), field)
	}
	sagemaker := explainUnionBranchByType(t, node, "sagemaker")
	auth, ok = sagemaker.lookup([]string{"config", "auth"})
	require.True(t, ok)
	assert.True(t, explainUnionBranchByType(t, auth, "sagemaker").propertyExists("aws"))
}

func TestRenderExplainText_AIGatewayProviderAuthVariantContext(t *testing.T) {
	subject, err := ResolveExplainSubject("ai_gateway.model_providers")
	require.NoError(t, err)
	text := RenderExplainText(subject, true)

	// Use the existing scaffold convention to identify each provider's fields.
	_, bedrock, found := strings.Cut(text, "oneOf option: type=bedrock\n")
	require.True(t, found, "extended explain must identify the Bedrock variant")
	bedrock, _, _ = strings.Cut(bedrock, "\noneOf option:")
	assert.Contains(t, bedrock, "oneOf option: type=aws\n")
	assert.Contains(t, bedrock, "- access_key_id:")
	assert.Contains(t, bedrock, "- secret_access_key:")
	assert.NotContains(t, bedrock, "- aws:")

	_, sagemaker, found := strings.Cut(text, "oneOf option: type=sagemaker\n")
	require.True(t, found, "extended explain must identify the SageMaker variant")
	sagemaker, _, _ = strings.Cut(sagemaker, "\noneOf option:")
	assert.Contains(t, sagemaker, "- aws:")
}
