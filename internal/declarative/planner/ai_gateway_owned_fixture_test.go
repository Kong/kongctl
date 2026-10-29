package planner

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	sdk "github.com/Kong/sdk-konnect-go"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/konnect/helpers"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/stretchr/testify/require"
)

func ownedAIGatewayObservation[T any](t *testing.T, value T, namespace string, protected bool) T {
	t.Helper()
	data, err := json.Marshal(value)
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(data, &payload))
	payload[FieldManagedBy] = resources.BuildChildManagedBy(namespace, protected, "dev")
	data, err = json.Marshal(payload)
	require.NoError(t, err)
	var result T
	require.NoError(t, json.Unmarshal(data, &result))
	return result
}

// Unrelated child collections are explicitly empty; unexpected writes fail.
type emptyAIGatewayCollectionsTransport struct{}

func (emptyAIGatewayCollectionsTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Method != http.MethodGet {
		return nil, fmt.Errorf("unexpected %s %s", req.Method, req.URL.Path)
	}
	return &http.Response{
		StatusCode: 200,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"data":[]}`)),
		Request:    req,
	}, nil
}

func emptyAIGatewayCollectionsConfig() state.ClientConfig {
	return aigatewayCollectionsConfig(emptyAIGatewayCollectionsTransport{})
}

func aigatewayCollectionsConfig(transport http.RoundTripper) state.ClientConfig {
	api := sdk.New(
		sdk.WithServerURL("https://empty.invalid"),
		sdk.WithClient(&http.Client{Transport: transport}),
	)
	return state.ClientConfig{
		AIGatewayProvidersAPI: &helpers.AIGatewayProvidersAPIImpl{
			SDK: api,
		}, AIGatewayAuthStrategiesAPI: api.AIGatewayAuthStrategies,
		AIGatewayPoliciesAPI: api.AIGatewayPolicies, AIGatewayCustomPoliciesAPI: api.AIGatewayCustomPolicies,
		AIGatewayAgentsAPI: api.AIGatewayAgents, AIGatewayModelAPI: api.AIGatewayModels,
		AIGatewayMCPServersAPI: api.AIGatewayMCPServers, AIGatewayVaultsAPI: api.AIGatewayVaults,
		AIGatewayConsumersAPI: api.AIGatewayConsumers, AIGatewayConsumerGroupsAPI: api.AIGatewayConsumerGroups,
		AIGatewayCertificatesAPI: api.AIGatewayCertificates, AIGatewayCACertificatesAPI: api.AIGatewayCACertificates,
		AIGatewaySNIsAPI: api.AIGatewaySNIs,
	}
}
