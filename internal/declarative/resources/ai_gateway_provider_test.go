package resources

import (
	"reflect"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/stretchr/testify/require"
)

func TestAIGatewayProviderResourceValidateRequiresName(t *testing.T) {
	t.Parallel()

	provider := AIGatewayProviderResource{
		BaseResource: BaseResource{Ref: "openai-provider"},
		Type:         "openai",
		DisplayName:  "OpenAI Provider",
		Config: map[string]any{
			"auth": map[string]any{"type": "basic"},
		},
	}

	err := provider.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "name is required")
}

func TestAIGatewayProviderResourceValidateRejectsLegacyBasicAuthFields(t *testing.T) {
	t.Parallel()

	provider := AIGatewayProviderResource{
		BaseResource: BaseResource{Ref: "openai-provider"},
		Name:         "openai-provider",
		Type:         "openai",
		DisplayName:  "OpenAI Provider",
		Config: map[string]any{
			"auth": map[string]any{
				"type":         "basic",
				"header_name":  "Authorization",
				"header_value": "Bearer token",
			},
		},
	}

	err := provider.Validate()
	require.Error(t, err)
	require.ErrorContains(t, err, "config.auth.header_name and config.auth.header_value are not supported")
	require.ErrorContains(t, err, "use config.auth.headers[].name and config.auth.headers[].value")
}

func TestAIGatewayProviderResourceValidateAcceptsBasicAuthHeaders(t *testing.T) {
	t.Parallel()

	provider := AIGatewayProviderResource{
		BaseResource: BaseResource{Ref: "openai-provider"},
		Name:         "openai-provider",
		Type:         "openai",
		DisplayName:  "OpenAI Provider",
		Config: map[string]any{
			"auth": map[string]any{
				"type": "basic",
				"headers": []any{
					map[string]any{"name": "Authorization", "value": "Bearer token"},
				},
			},
		},
	}

	require.NoError(t, provider.Validate())
}

func TestAIGatewayProviderExplainNodeUsesBasicAuthHeaders(t *testing.T) {
	t.Parallel()

	node, err := aiGatewayProviderExplainNode(ExplainBuildContext{})
	require.NoError(t, err)

	var openAI *ExplainNode
	for _, branch := range node.OneOf {
		typeField, ok := branch.property("type")
		if ok && typeField.Node.Const == "openai" {
			openAI = branch
			break
		}
	}
	require.NotNil(t, openAI)

	config, ok := openAI.property("config")
	require.True(t, ok)
	auth, ok := config.Node.property("auth")
	require.True(t, ok)
	headers, ok := auth.Node.property("headers")
	require.True(t, ok)
	require.True(t, headers.Recommended)
	require.Equal(t, explainKindArray, headers.Node.Kind)
	require.NotNil(t, headers.Node.Items)
	require.True(t, headers.Node.Items.propertyExists("name"))
	require.True(t, headers.Node.Items.propertyExists("value"))
	require.False(t, auth.Node.propertyExists("header_name"))
	require.False(t, auth.Node.propertyExists("header_value"))
}

func TestAIGatewayProviderExplainNodeCoversSDKProviderUnion(t *testing.T) {
	t.Parallel()

	node, err := aiGatewayProviderExplainNode(ExplainBuildContext{})
	require.NoError(t, err)

	providerTypes := make([]string, 0, len(node.OneOf))
	for _, branch := range node.OneOf {
		typeField, ok := branch.property("type")
		require.True(t, ok)
		providerType, ok := typeField.Node.Const.(string)
		require.True(t, ok)
		providerTypes = append(providerTypes, providerType)
	}

	expected := []string{
		"anthropic", "azure", "bedrock", "cerebras", "cohere", "dashscope", "databricks", "deepseek",
		"gemini", "huggingface", "kimi", "llama2", "mistral", "ollama", "openai", "vercel", "vertex",
		"vllm", "xai",
	}
	require.ElementsMatch(t, expected, providerTypes)

	sdkUnionMembers := 0
	for field := range reflect.TypeFor[kkComps.CreateAIGatewayModelProviderRequest]().Fields() {
		if field.Tag.Get("union") == "member" {
			sdkUnionMembers++
		}
	}
	require.Len(t, providerTypes, sdkUnionMembers)
}

func TestAIGatewayProviderResourceParentRef(t *testing.T) {
	t.Parallel()

	provider := AIGatewayProviderResource{
		BaseResource: BaseResource{Ref: "openai-provider"},
		AIGateway:    "ai-gateway",
		Name:         "openai-provider",
	}

	parent := provider.GetParentRef()
	require.NotNil(t, parent)
	require.Equal(t, ResourceTypeAIGateway, parent.Kind)
	require.Equal(t, "ai-gateway", parent.Ref)
}

func TestAIGatewayProviderResourceParentRefNormalizesDeferredRef(t *testing.T) {
	t.Parallel()

	provider := AIGatewayProviderResource{
		BaseResource: BaseResource{Ref: "openai-provider"},
		AIGateway:    tags.RefPlaceholderPrefix + "ai-gateway#id",
		Name:         "openai-provider",
	}

	parent := provider.GetParentRef()
	require.NotNil(t, parent)
	require.Equal(t, ResourceTypeAIGateway, parent.Kind)
	require.Equal(t, "ai-gateway", parent.Ref)
}
