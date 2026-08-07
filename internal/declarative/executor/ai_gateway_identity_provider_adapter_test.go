package executor

import (
	"encoding/json"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/stretchr/testify/require"
)

func TestAIGatewayIdentityProviderAdapterMapsOpenIDConnectCacheTokensSalt(t *testing.T) {
	fields := map[string]any{
		planner.FieldName:        "support-oidc",
		planner.FieldType:        "openid-connect",
		planner.FieldDisplayName: "Support OIDC",
		planner.FieldConfig: map[string]any{
			"auth_methods":      []string{"bearer"},
			"cache_tokens_salt": "support-cache-salt",
		},
	}

	var request kkComps.CreateAIGatewayIdentityProviderRequest
	err := NewAIGatewayIdentityProviderAdapter(nil).MapCreateFields(t.Context(), nil, fields, &request)
	require.NoError(t, err)
	require.NotNil(t, request.AIGatewayIdentityProviderOpenIDConnect)
	require.NotNil(t, request.AIGatewayIdentityProviderOpenIDConnect.Config)
	require.Equal(t, "support-cache-salt", request.AIGatewayIdentityProviderOpenIDConnect.Config.CacheTokensSalt)
}

func TestAIGatewayIdentityProviderAdapterRejectsMissingOpenIDConnectCacheTokensSalt(t *testing.T) {
	fields := map[string]any{
		planner.FieldName:        "support-oidc",
		planner.FieldType:        "openid-connect",
		planner.FieldDisplayName: "Support OIDC",
		planner.FieldConfig: map[string]any{
			"auth_methods": []string{"bearer"},
		},
	}

	var request kkComps.CreateAIGatewayIdentityProviderRequest
	err := NewAIGatewayIdentityProviderAdapter(nil).MapCreateFields(t.Context(), nil, fields, &request)
	require.Error(t, err)
}

func TestAIGatewayIdentityProviderAdapterPreservesAdditionalOpenIDConnectConfigProperties(t *testing.T) {
	fields := map[string]any{
		planner.FieldName:        "support-oidc",
		planner.FieldType:        "openid-connect",
		planner.FieldDisplayName: "Support OIDC",
		planner.FieldConfig: map[string]any{
			"auth_methods":      []string{"bearer"},
			"cache_tokens_salt": "support-cache-salt",
			"credential_claim":  []string{"sub"},
		},
	}

	var request kkComps.CreateAIGatewayIdentityProviderRequest
	err := NewAIGatewayIdentityProviderAdapter(nil).MapCreateFields(t.Context(), nil, fields, &request)
	require.NoError(t, err)
	require.NotNil(t, request.AIGatewayIdentityProviderOpenIDConnect)
	require.NotNil(t, request.AIGatewayIdentityProviderOpenIDConnect.Config)
	require.Contains(t, request.AIGatewayIdentityProviderOpenIDConnect.Config.AdditionalProperties, "credential_claim")

	data, err := json.Marshal(request)
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(data, &payload))
	config, ok := payload[planner.FieldConfig].(map[string]any)
	require.True(t, ok)
	require.Equal(t, []any{"sub"}, config["credential_claim"])
}
