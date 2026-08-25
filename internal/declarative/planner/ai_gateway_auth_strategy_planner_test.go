package planner

import (
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/require"
)

func TestShouldUpdateAIGatewayAuthStrategyRejectsTypeChange(t *testing.T) {
	t.Parallel()

	needsUpdate, fields, _, err := shouldUpdateAIGatewayAuthStrategy(
		state.AIGatewayAuthStrategy{
			Type:        "key-auth",
			DisplayName: "Support Key Auth",
			Config:      map[string]any{"key_names": []any{"apikey"}},
		},
		resources.AIGatewayAuthStrategyResource{
			Type:        "openid-connect",
			DisplayName: "Support OIDC",
			Config:      map[string]any{"issuer": "https://issuer.example.com"},
		},
	)

	require.Error(t, err)
	require.False(t, needsUpdate)
	require.Nil(t, fields)
	require.Contains(t, err.Error(), "changing AI Gateway Auth Strategy type")
}

func TestAIGatewayAuthStrategyConfigChangedIgnoresClientSecret(t *testing.T) {
	t.Parallel()

	current := map[string]any{
		"auth_methods": []any{"bearer"},
		"client_id":    []any{"support-client"},
		"issuer":       "https://issuer.example.com",
	}
	desired := map[string]any{
		"auth_methods":  []any{"bearer"},
		"client_id":     []any{"support-client"},
		"client_secret": []any{"super-secret"},
		"issuer":        "https://issuer.example.com",
	}

	require.False(t, aiGatewayAuthStrategyConfigChanged(current, desired))
}

func TestAIGatewayAuthStrategyConfigChangedComparesPublicVaultReferences(t *testing.T) {
	t.Parallel()

	config := func(values ...any) map[string]any {
		return map[string]any{
			"auth_methods":  []any{"bearer"},
			"client_id":     []any{"primary-client", "fallback-client"},
			"client_secret": values,
			"issuer":        "https://issuer.example.com",
		}
	}

	require.False(t, aiGatewayAuthStrategyConfigChanged(
		config("{vault://support-secrets/primary}", "hidden-current"),
		config("{vault://support-secrets/primary}", "hidden-desired"),
	))
	require.True(t, aiGatewayAuthStrategyConfigChanged(
		config("{vault://support-secrets/old-primary}", "hidden-current"),
		config("{vault://support-secrets/new-primary}", "hidden-desired"),
	))
}

func TestAIGatewayAuthStrategyMatchPrefersIDOverName(t *testing.T) {
	t.Parallel()

	id := "11111111-1111-4111-8111-111111111111"
	currentByID, currentByName := indexAIGatewayAuthStrategies([]state.AIGatewayAuthStrategy{
		{ID: id, Name: "old-provider"},
		{ID: "22222222-2222-4222-8222-222222222222", Name: "new-provider"},
	})

	current, ok := matchCurrentAIGatewayAuthStrategy(
		resources.AIGatewayAuthStrategyResource{BaseResource: resources.BaseResource{Ref: id}, Name: "new-provider"},
		currentByID,
		currentByName,
	)

	require.True(t, ok)
	require.Equal(t, id, current.ID)
}

func TestAIGatewayAuthStrategyConfigChangedDetectsObservableChanges(t *testing.T) {
	t.Parallel()

	current := map[string]any{
		"auth_methods": []any{"bearer"},
		"client_id":    []any{"support-client"},
		"issuer":       "https://issuer.example.com",
	}
	desired := map[string]any{
		"auth_methods":  []any{"bearer"},
		"client_id":     []any{"support-client"},
		"client_secret": []any{"super-secret"},
		"issuer":        "https://issuer-updated.example.com",
	}

	require.True(t, aiGatewayAuthStrategyConfigChanged(current, desired))
}

func TestAIGatewayAuthStrategyConfigChangedIgnoresUndeclaredDefaults(t *testing.T) {
	t.Parallel()

	current := map[string]any{
		"hide_credentials": true,
		"key_in_body":      false,
		"key_in_header":    true,
		"key_in_query":     true,
		"key_names":        []any{"x-support-api-key"},
	}
	desired := map[string]any{
		"hide_credentials": true,
		"key_names":        []any{"x-support-api-key"},
	}

	require.False(t, aiGatewayAuthStrategyConfigChanged(current, desired))
}

func TestAIGatewayAuthStrategyConfigChangedComparesDeclaredDefaults(t *testing.T) {
	t.Parallel()

	current := map[string]any{
		"hide_credentials": true,
		"key_in_body":      false,
		"key_in_header":    true,
		"key_in_query":     true,
		"key_names":        []any{"x-support-api-key"},
	}
	desired := map[string]any{
		"hide_credentials": true,
		"key_in_body":      true,
		"key_names":        []any{"x-support-api-key"},
	}

	require.True(t, aiGatewayAuthStrategyConfigChanged(current, desired))
}

func TestAIGatewayAuthStrategyChangedFieldsScrubClientSecret(t *testing.T) {
	t.Parallel()

	needsUpdate, _, changedFields, err := shouldUpdateAIGatewayAuthStrategy(
		state.AIGatewayAuthStrategy{
			Type:        "openid-connect",
			DisplayName: "Support OIDC",
			Config: map[string]any{
				"auth_methods": []any{"bearer"},
				"client_id":    []any{"support-client"},
				"issuer":       "https://issuer.example.com",
			},
		},
		resources.AIGatewayAuthStrategyResource{
			Type:        "openid-connect",
			DisplayName: "Support OIDC",
			Config: map[string]any{
				"auth_methods":  []any{"bearer"},
				"client_id":     []any{"support-client"},
				"client_secret": []any{"super-secret"},
				"issuer":        "https://issuer-updated.example.com",
			},
		},
	)

	require.NoError(t, err)
	require.True(t, needsUpdate)
	require.NotContains(t, changedFields[FieldConfig].New, "client_secret")
}

func TestAIGatewayAuthStrategyUpdatePreservesUndeclaredSecurityConfig(t *testing.T) {
	t.Parallel()

	current := state.AIGatewayAuthStrategy{
		Name:        "support-oidc",
		Type:        "openid-connect",
		DisplayName: "Support OIDC",
		Labels:      map[string]string{"owner": "security"},
		ManagedBy:   map[string]string{"terraform": "legacy"},
		Config: map[string]any{
			"auth_methods":             []any{"bearer"},
			"cache_tokens_salt":        "support-cache-salt",
			"consumer_groups_claim":    []any{"groups"},
			"consumer_groups_optional": false,
			"upstream_headers_claims":  []any{"sub"},
			"upstream_headers_names":   []any{"x-consumer-subject"},
		},
	}
	desired := resources.AIGatewayAuthStrategyResource{
		Name:        "support-oidc",
		Type:        "openid-connect",
		DisplayName: "Updated Support OIDC",
		Config: map[string]any{
			"auth_methods":      []any{"bearer"},
			"cache_tokens_salt": "support-cache-salt",
		},
	}

	needsUpdate, fields, changedFields, err := shouldUpdateAIGatewayAuthStrategy(current, desired)
	require.NoError(t, err)
	require.True(t, needsUpdate)
	require.Contains(t, changedFields, FieldDisplayName)
	require.NotContains(t, changedFields, FieldConfig)
	require.Equal(t, current.Config, fields[FieldConfig])
	require.Equal(t, current.Labels, fields[FieldLabels])
	require.Equal(t, current.ManagedBy, fields[FieldManagedBy])
}

func TestAIGatewayAuthStrategyUpdateOverlaysDeclaredSecurityConfig(t *testing.T) {
	t.Parallel()

	current := state.AIGatewayAuthStrategy{
		Name:        "support-oidc",
		Type:        "openid-connect",
		DisplayName: "Support OIDC",
		Config: map[string]any{
			"cache_tokens_salt":       "support-cache-salt",
			"consumer_groups_claim":   []any{"groups"},
			"upstream_headers_claims": []any{"sub"},
			"nested_extension": map[string]any{
				"preserved": true,
				"managed":   "old",
			},
		},
	}
	desired := resources.AIGatewayAuthStrategyResource{
		Name:        "support-oidc",
		Type:        "openid-connect",
		DisplayName: "Support OIDC",
		Config: map[string]any{
			"cache_tokens_salt":     "support-cache-salt",
			"consumer_groups_claim": []any{"roles"},
			"nested_extension": map[string]any{
				"managed": "new",
			},
		},
	}

	needsUpdate, fields, _, err := shouldUpdateAIGatewayAuthStrategy(current, desired)
	require.NoError(t, err)
	require.True(t, needsUpdate)
	config := fields[FieldConfig].(map[string]any)
	require.Equal(t, []any{"roles"}, config["consumer_groups_claim"])
	require.Equal(t, []any{"sub"}, config["upstream_headers_claims"])
	require.Equal(t, map[string]any{"preserved": true, "managed": "new"}, config["nested_extension"])
}
