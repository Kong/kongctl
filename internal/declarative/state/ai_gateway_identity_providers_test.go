package state

import (
	"context"
	"testing"
	"time"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/stretchr/testify/require"
)

func TestGetAIGatewayIdentityProviderPreservesAccessControlConfig(t *testing.T) {
	t.Parallel()

	optional := false
	provider := kkComps.CreateAIGatewayIdentityProviderOpenidConnect(
		kkComps.AIGatewayIdentityProviderOpenIDConnectResponse{
			ID:          "provider-id",
			Name:        "support-oidc",
			DisplayName: "Support OIDC",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
			Config: &kkComps.AIGatewayIdentityProviderOpenIDConnectResponseConfig{
				CacheTokensSalt:        "support-cache-salt",
				ConsumerGroupsClaim:    []string{"groups"},
				ConsumerGroupsOptional: &optional,
				AdditionalProperties: map[string]any{
					"upstream_headers_claims": []string{"sub"},
					"upstream_headers_names":  []string{"x-consumer-subject"},
				},
			},
		},
	)
	client := NewClient(ClientConfig{
		AIGatewayIdentityProvidersAPI: &testAIGatewayIdentityProvidersAPI{provider: provider},
	})

	actual, err := client.GetAIGatewayIdentityProvider(t.Context(), "gateway-id", "support-oidc")
	require.NoError(t, err)
	require.NotNil(t, actual)
	require.Equal(t, []any{"groups"}, actual.Config["consumer_groups_claim"])
	require.Equal(t, false, actual.Config["consumer_groups_optional"])
	require.Equal(t, []any{"sub"}, actual.Config["upstream_headers_claims"])
	require.Equal(t, []any{"x-consumer-subject"}, actual.Config["upstream_headers_names"])
}

type testAIGatewayIdentityProvidersAPI struct {
	provider kkComps.AIGatewayIdentityProvider
}

func (a *testAIGatewayIdentityProvidersAPI) ListAiGatewayIdentityProviders(
	context.Context,
	kkOps.ListAiGatewayIdentityProvidersRequest,
	...kkOps.Option,
) (*kkOps.ListAiGatewayIdentityProvidersResponse, error) {
	return &kkOps.ListAiGatewayIdentityProvidersResponse{}, nil
}

func (a *testAIGatewayIdentityProvidersAPI) CreateAiGatewayIdentityProvider(
	context.Context,
	string,
	kkComps.CreateAIGatewayIdentityProviderRequest,
	...kkOps.Option,
) (*kkOps.CreateAiGatewayIdentityProviderResponse, error) {
	return &kkOps.CreateAiGatewayIdentityProviderResponse{}, nil
}

func (a *testAIGatewayIdentityProvidersAPI) GetAiGatewayIdentityProvider(
	context.Context,
	string,
	string,
	...kkOps.Option,
) (*kkOps.GetAiGatewayIdentityProviderResponse, error) {
	return &kkOps.GetAiGatewayIdentityProviderResponse{AIGatewayIdentityProvider: &a.provider}, nil
}

func (a *testAIGatewayIdentityProvidersAPI) UpdateAiGatewayIdentityProvider(
	context.Context,
	kkOps.UpdateAiGatewayIdentityProviderRequest,
	...kkOps.Option,
) (*kkOps.UpdateAiGatewayIdentityProviderResponse, error) {
	return &kkOps.UpdateAiGatewayIdentityProviderResponse{}, nil
}

func (a *testAIGatewayIdentityProvidersAPI) DeleteAiGatewayIdentityProvider(
	context.Context,
	string,
	string,
	...kkOps.Option,
) (*kkOps.DeleteAiGatewayIdentityProviderResponse, error) {
	return &kkOps.DeleteAiGatewayIdentityProviderResponse{}, nil
}
