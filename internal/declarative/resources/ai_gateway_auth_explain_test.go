package resources

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
