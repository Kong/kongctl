package planner

import (
	"encoding/json"
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

func TestAIGatewayProviderConfigChangedComparesPublicVaultReferences(t *testing.T) {
	t.Parallel()

	config := func(value string) map[string]any {
		return map[string]any{
			"auth": map[string]any{
				"type": "basic",
				"headers": []any{
					map[string]any{"name": "Authorization", "value": value},
				},
			},
		}
	}

	require.False(t, aiGatewayProviderConfigChanged(
		config("{vault://support-secrets/openai-token}"),
		config("{vault://support-secrets/openai-token}"),
	))
	require.True(t, aiGatewayProviderConfigChanged(
		config("{vault://support-secrets/old-openai-token}"),
		config("{vault://support-secrets/new-openai-token}"),
	))
	// Plain secret material remains excluded from comparison.
	require.False(t, aiGatewayProviderConfigChanged(
		config("old-secret-material"),
		config("new-secret-material"),
	))
}

func TestShouldUpdateAIGatewayProviderIncludesChangedPublicVaultReference(t *testing.T) {
	t.Parallel()

	oldReference := "{vault://support-secrets/old-openai-token}"
	newReference := "{vault://support-secrets/new-openai-token}"
	config := func(value string) map[string]any {
		return map[string]any{
			"auth": map[string]any{
				"type": "basic",
				"headers": []any{
					map[string]any{"name": "Authorization", "value": value},
				},
			},
		}
	}

	needsUpdate, fields, changedFields, err := shouldUpdateAIGatewayProvider(
		state.AIGatewayProvider{
			Type:        "openai",
			DisplayName: "OpenAI Provider",
			Config:      config(oldReference),
		},
		resources.AIGatewayProviderResource{
			Type:        "openai",
			DisplayName: "OpenAI Provider",
			Config:      config(newReference),
		},
	)

	require.NoError(t, err)
	require.True(t, needsUpdate)
	updateConfig := fields[FieldConfig].(map[string]any)
	updateHeader := updateConfig["auth"].(map[string]any)["headers"].([]any)[0].(map[string]any)
	require.Equal(t, newReference, updateHeader[FieldValue])
	diffConfig := changedFields[FieldConfig].New.(map[string]any)
	diffHeader := diffConfig["auth"].(map[string]any)["headers"].([]any)[0].(map[string]any)
	require.Equal(t, newReference, diffHeader[FieldValue])
}

func TestAIGatewayProviderCreatePlanKeepsPublicVaultReferenceOutOfSecretWrites(t *testing.T) {
	t.Parallel()

	const reference = "{vault://support-secrets/openai-token}"
	plan := NewPlan("1.0", "test", PlanModeApply)
	(&Planner{}).planAIGatewayProviderCreate(
		DefaultNamespace,
		"gateway",
		"Gateway",
		"gateway-id",
		resources.AIGatewayProviderResource{
			BaseResource: resources.BaseResource{Ref: "provider"},
			Name:         "provider",
			Type:         "openai",
			DisplayName:  "Provider",
			Config: map[string]any{
				"auth": map[string]any{
					"type": "basic",
					"headers": []any{
						map[string]any{"name": "Authorization", "value": reference},
					},
				},
			},
		},
		nil,
		plan,
	)

	require.Len(t, plan.Changes, 1)
	require.Empty(t, plan.Changes[0].SecretWrites)
	data, err := json.Marshal(plan)
	require.NoError(t, err)
	require.Contains(t, string(data), reference)
}

func TestAIGatewayProviderConfigChangedIgnoresAzureFoundryDomainDefault(t *testing.T) {
	t.Parallel()

	current := map[string]any{
		FieldService: "azure-foundry",
		FieldFoundry: map[string]any{
			FieldResource: "support-foundry",
			FieldDomain:   "services.ai.azure.com",
		},
	}
	desired := map[string]any{
		FieldService: "azure-foundry",
		FieldFoundry: map[string]any{
			FieldResource: "support-foundry",
		},
	}

	require.False(t, aiGatewayProviderConfigChanged(current, desired))
}

func TestAIGatewayProviderMatchPrefersIDOverName(t *testing.T) {
	t.Parallel()

	id := "11111111-1111-4111-8111-111111111111"
	currentByID, currentByName := indexAIGatewayProviders([]state.AIGatewayProvider{
		{ID: id, Name: "old-provider"},
		{ID: "22222222-2222-4222-8222-222222222222", Name: "new-provider"},
	})

	current, ok := matchCurrentAIGatewayProvider(
		resources.AIGatewayProviderResource{BaseResource: resources.BaseResource{Ref: id}, Name: "new-provider"},
		currentByID,
		currentByName,
	)

	require.True(t, ok)
	require.Equal(t, id, current.ID)
}

func TestAIGatewayProviderChangedFieldsScrubWriteOnlyValues(t *testing.T) {
	t.Parallel()

	needsUpdate, fields, changedFields, err := shouldUpdateAIGatewayProvider(
		state.AIGatewayProvider{
			Type:        "openai",
			DisplayName: "OpenAI Provider",
			Config: map[string]any{
				"auth": map[string]any{
					"type": "basic",
					"headers": []any{
						map[string]any{"name": "Authorization"},
					},
				},
			},
		},
		resources.AIGatewayProviderResource{
			Type:        "openai",
			DisplayName: "OpenAI Provider",
			Config: map[string]any{
				"auth": map[string]any{
					"type": "basic",
					"headers": []any{
						map[string]any{"name": "Authorization", "value": "Bearer token"},
					},
				},
				"endpoint": "https://api.openai.test",
			},
		},
	)

	require.NoError(t, err)
	require.True(t, needsUpdate)
	updateConfig := fields[FieldConfig].(map[string]any)
	updateHeader := updateConfig["auth"].(map[string]any)["headers"].([]any)[0].(map[string]any)
	require.Equal(t, "Bearer token", updateHeader["value"])

	configChange := changedFields[FieldConfig]
	diffConfig := configChange.New.(map[string]any)
	diffHeader := diffConfig["auth"].(map[string]any)["headers"].([]any)[0].(map[string]any)
	require.NotContains(t, diffHeader, "value")
}
