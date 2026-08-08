package planner

import (
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/require"
)

func TestShouldUpdateAIGatewayIdentityProviderRejectsTypeChange(t *testing.T) {
	t.Parallel()

	needsUpdate, fields, _, err := shouldUpdateAIGatewayIdentityProvider(
		state.AIGatewayIdentityProvider{
			Type:        "key-auth",
			DisplayName: "Support Key Auth",
			Config:      map[string]any{"key_names": []any{"apikey"}},
		},
		resources.AIGatewayIdentityProviderResource{
			Type:        "openid-connect",
			DisplayName: "Support OIDC",
			Config:      map[string]any{"issuer": "https://issuer.example.com"},
		},
	)

	require.Error(t, err)
	require.False(t, needsUpdate)
	require.Nil(t, fields)
	require.Contains(t, err.Error(), "changing AI Gateway Identity Provider type")
}

func TestAIGatewayIdentityProviderConfigChangedIgnoresClientSecret(t *testing.T) {
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

	require.False(t, aiGatewayIdentityProviderConfigChanged(current, desired))
}

func TestAIGatewayIdentityProviderMatchPrefersIDOverName(t *testing.T) {
	t.Parallel()

	id := "11111111-1111-4111-8111-111111111111"
	currentByID, currentByName := indexAIGatewayIdentityProviders([]state.AIGatewayIdentityProvider{
		{ID: id, Name: "old-provider"},
		{ID: "22222222-2222-4222-8222-222222222222", Name: "new-provider"},
	})

	current, ok := matchCurrentAIGatewayIdentityProvider(
		resources.AIGatewayIdentityProviderResource{BaseResource: resources.BaseResource{Ref: id}, Name: "new-provider"},
		currentByID,
		currentByName,
	)

	require.True(t, ok)
	require.Equal(t, id, current.ID)
}

func TestAIGatewayIdentityProviderConfigChangedDetectsObservableChanges(t *testing.T) {
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

	require.True(t, aiGatewayIdentityProviderConfigChanged(current, desired))
}

func TestAIGatewayIdentityProviderConfigChangedIgnoresUndeclaredDefaults(t *testing.T) {
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

	require.False(t, aiGatewayIdentityProviderConfigChanged(current, desired))
}

func TestAIGatewayIdentityProviderConfigChangedComparesDeclaredDefaults(t *testing.T) {
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

	require.True(t, aiGatewayIdentityProviderConfigChanged(current, desired))
}

func TestAIGatewayIdentityProviderChangedFieldsScrubClientSecret(t *testing.T) {
	t.Parallel()

	needsUpdate, _, changedFields, err := shouldUpdateAIGatewayIdentityProvider(
		state.AIGatewayIdentityProvider{
			Type:        "openid-connect",
			DisplayName: "Support OIDC",
			Config: map[string]any{
				"auth_methods": []any{"bearer"},
				"client_id":    []any{"support-client"},
				"issuer":       "https://issuer.example.com",
			},
		},
		resources.AIGatewayIdentityProviderResource{
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

func TestAIGatewayIdentityProviderUpdatePreservesUndeclaredSecurityConfig(t *testing.T) {
	t.Parallel()

	current := state.AIGatewayIdentityProvider{
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
	desired := resources.AIGatewayIdentityProviderResource{
		Name:        "support-oidc",
		Type:        "openid-connect",
		DisplayName: "Updated Support OIDC",
		Config: map[string]any{
			"auth_methods":      []any{"bearer"},
			"cache_tokens_salt": "support-cache-salt",
		},
	}

	needsUpdate, fields, changedFields, err := shouldUpdateAIGatewayIdentityProvider(current, desired)
	require.NoError(t, err)
	require.True(t, needsUpdate)
	require.Contains(t, changedFields, FieldDisplayName)
	require.NotContains(t, changedFields, FieldConfig)
	require.Equal(t, current.Config, fields[FieldConfig])
	require.Equal(t, current.Labels, fields[FieldLabels])
	require.Equal(t, current.ManagedBy, fields[FieldManagedBy])
}

func TestAIGatewayIdentityProviderUpdateOverlaysDeclaredSecurityConfig(t *testing.T) {
	t.Parallel()

	current := state.AIGatewayIdentityProvider{
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
	desired := resources.AIGatewayIdentityProviderResource{
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

	needsUpdate, fields, _, err := shouldUpdateAIGatewayIdentityProvider(current, desired)
	require.NoError(t, err)
	require.True(t, needsUpdate)
	config := fields[FieldConfig].(map[string]any)
	require.Equal(t, []any{"roles"}, config["consumer_groups_claim"])
	require.Equal(t, []any{"sub"}, config["upstream_headers_claims"])
	require.Equal(t, map[string]any{"preserved": true, "managed": "new"}, config["nested_extension"])
}
