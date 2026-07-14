package executor

import (
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
