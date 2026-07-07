package planner

import (
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/require"
)

func TestShouldUpdateAIGatewayProviderRejectsTypeChange(t *testing.T) {
	t.Parallel()

	needsUpdate, fields, _, err := shouldUpdateAIGatewayProvider(
		state.AIGatewayProvider{
			Type:        "openai",
			DisplayName: "OpenAI Provider",
			Config:      map[string]any{"auth": map[string]any{"type": "basic"}},
		},
		resources.AIGatewayProviderResource{
			Type:        "anthropic",
			DisplayName: "Anthropic Provider",
			Config:      map[string]any{"auth": map[string]any{"type": "basic"}},
		},
	)

	require.Error(t, err)
	require.False(t, needsUpdate)
	require.Nil(t, fields)
	require.Contains(t, err.Error(), "changing AI Gateway Model Provider type")
}

func TestAIGatewayProviderConfigChangedIgnoresWriteOnlyValues(t *testing.T) {
	t.Parallel()

	current := map[string]any{
		"auth": map[string]any{
			"type": "basic",
			"headers": []any{
				map[string]any{"name": "Authorization"},
			},
		},
	}
	desired := map[string]any{
		"auth": map[string]any{
			"type": "basic",
			"headers": []any{
				map[string]any{"name": "Authorization", "value": "Bearer token"},
			},
		},
	}

	require.False(t, aiGatewayProviderConfigChanged(current, desired))
}
