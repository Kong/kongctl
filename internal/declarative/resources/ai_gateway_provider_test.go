package resources

import (
	"testing"

	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/stretchr/testify/require"
)

func TestAIGatewayProviderResourceValidateRequiresName(t *testing.T) {
	t.Parallel()

	provider := AIGatewayProviderResource{
		Ref:         "openai-provider",
		Type:        "openai",
		DisplayName: "OpenAI Provider",
		Config: map[string]any{
			"auth": map[string]any{"type": "basic"},
		},
	}

	err := provider.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "name is required")
}

func TestAIGatewayProviderResourceParentRef(t *testing.T) {
	t.Parallel()

	provider := AIGatewayProviderResource{
		Ref:       "openai-provider",
		AIGateway: "ai-gateway",
		Name:      "openai-provider",
	}

	parent := provider.GetParentRef()
	require.NotNil(t, parent)
	require.Equal(t, ResourceTypeAIGateway, parent.Kind)
	require.Equal(t, "ai-gateway", parent.Ref)
}

func TestAIGatewayProviderResourceParentRefNormalizesDeferredRef(t *testing.T) {
	t.Parallel()

	provider := AIGatewayProviderResource{
		Ref:       "openai-provider",
		AIGateway: tags.RefPlaceholderPrefix + "ai-gateway#id",
		Name:      "openai-provider",
	}

	parent := provider.GetParentRef()
	require.NotNil(t, parent)
	require.Equal(t, ResourceTypeAIGateway, parent.Kind)
	require.Equal(t, "ai-gateway", parent.Ref)
}
