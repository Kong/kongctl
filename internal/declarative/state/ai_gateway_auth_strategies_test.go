package state

import (
	"context"
	"testing"
	"time"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/stretchr/testify/require"
)

func TestGetAIGatewayAuthStrategyPreservesAccessControlConfig(t *testing.T) {
	t.Parallel()

	optional := false
	provider := kkComps.CreateAIGatewayAuthStrategyOpenidConnect(
		kkComps.AIGatewayAuthStrategyOpenIDConnectResponse{
			ID:          "provider-id",
			Name:        "support-oidc",
			DisplayName: "Support OIDC",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
			Config: &kkComps.AIGatewayAuthStrategyOpenIDConnectResponseConfig{
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
		AIGatewayAuthStrategiesAPI: &testAIGatewayAuthStrategiesAPI{provider: provider},
	})

	actual, err := client.GetAIGatewayAuthStrategy(t.Context(), "gateway-id", "support-oidc")
	require.NoError(t, err)
	require.NotNil(t, actual)
	require.Equal(t, []any{"groups"}, actual.Config["consumer_groups_claim"])
	require.Equal(t, false, actual.Config["consumer_groups_optional"])
	require.Equal(t, []any{"sub"}, actual.Config["upstream_headers_claims"])
	require.Equal(t, []any{"x-consumer-subject"}, actual.Config["upstream_headers_names"])
}

type testAIGatewayAuthStrategiesAPI struct {
	provider kkComps.AIGatewayAuthStrategy
}

func (a *testAIGatewayAuthStrategiesAPI) ListAiGatewayAuthStrategies(
	context.Context,
	kkOps.ListAiGatewayAuthStrategiesRequest,
	...kkOps.Option,
) (*kkOps.ListAiGatewayAuthStrategiesResponse, error) {
	return &kkOps.ListAiGatewayAuthStrategiesResponse{}, nil
}

func (a *testAIGatewayAuthStrategiesAPI) CreateAiGatewayAuthStrategy(
	context.Context,
	string,
	kkComps.CreateAIGatewayAuthStrategyRequest,
	...kkOps.Option,
) (*kkOps.CreateAiGatewayAuthStrategyResponse, error) {
	return &kkOps.CreateAiGatewayAuthStrategyResponse{}, nil
}

func (a *testAIGatewayAuthStrategiesAPI) GetAiGatewayAuthStrategy(
	context.Context,
	string,
	string,
	...kkOps.Option,
) (*kkOps.GetAiGatewayAuthStrategyResponse, error) {
	return &kkOps.GetAiGatewayAuthStrategyResponse{AIGatewayAuthStrategy: &a.provider}, nil
}

func (a *testAIGatewayAuthStrategiesAPI) UpdateAiGatewayAuthStrategy(
	context.Context,
	kkOps.UpdateAiGatewayAuthStrategyRequest,
	...kkOps.Option,
) (*kkOps.UpdateAiGatewayAuthStrategyResponse, error) {
	return &kkOps.UpdateAiGatewayAuthStrategyResponse{}, nil
}

func (a *testAIGatewayAuthStrategiesAPI) DeleteAiGatewayAuthStrategy(
	context.Context,
	string,
	string,
	...kkOps.Option,
) (*kkOps.DeleteAiGatewayAuthStrategyResponse, error) {
	return &kkOps.DeleteAiGatewayAuthStrategyResponse{}, nil
}
