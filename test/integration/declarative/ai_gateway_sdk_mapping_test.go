//go:build integration

package declarative_test

import (
	"path/filepath"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/executor"
	"github.com/kong/kongctl/internal/declarative/loader"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/stretchr/testify/require"
)

func TestAIGatewayScenarioCreatePayloadsSurviveSDKMapping(t *testing.T) {
	t.Setenv("KONGCTL_E2E_WEATHERAPI_API_KEY", "fake-weather-api-key")

	for _, path := range aiGatewayScenarioConfigPaths(t) {
		t.Run(filepath.Base(filepath.Dir(filepath.Dir(path)))+"/"+filepath.Base(path), func(t *testing.T) {
			resourceSet, err := loader.New().LoadFile(path)
			require.NoError(t, err)
			mapAIGatewayScenarioCreatePayloads(t, resourceSet)
		})
	}
}

func aiGatewayScenarioConfigPaths(t *testing.T) []string {
	t.Helper()

	pattern := filepath.Join("..", "..", "e2e", "scenarios", "ai-gateway", "*", "testdata", "*.yaml")
	paths, err := filepath.Glob(pattern)
	require.NoError(t, err)
	require.NotEmpty(t, paths)
	return paths
}

func mapAIGatewayScenarioCreatePayloads(t *testing.T, resourceSet *resources.ResourceSet) {
	t.Helper()

	for _, resource := range resourceSet.AIGateways {
		fields := map[string]any{
			planner.FieldName:        resource.Name,
			planner.FieldDisplayName: resource.DisplayName,
			planner.FieldProxyURLs:   resource.ProxyUrls,
			planner.FieldLabels:      resource.Labels,
		}
		if resource.Description != nil {
			fields[planner.FieldDescription] = *resource.Description
		}
		var request kkComps.CreateAIGatewayRequest
		executionContext := &executor.ExecutionContext{Namespace: "sdk-mapping-test"}
		require.NoError(
			t,
			executor.NewAIGatewayAdapter(nil).MapCreateFields(t.Context(), executionContext, fields, &request),
		)
	}
	for _, resource := range resourceSet.AIGatewayProviders {
		fields, err := resource.MutablePayloadMap()
		require.NoError(t, err)
		fields[planner.FieldAIGatewayID] = "gateway-id"
		var request kkComps.CreateAIGatewayModelProviderRequest
		require.NoError(t, executor.NewAIGatewayProviderAdapter(nil).MapCreateFields(t.Context(), nil, fields, &request))
	}
	for _, resource := range resourceSet.AIGatewayIdentityProviders {
		fields, err := resource.MutablePayloadMap()
		require.NoError(t, err)
		fields[planner.FieldAIGatewayID] = "gateway-id"
		var request kkComps.CreateAIGatewayIdentityProviderRequest
		require.NoError(
			t,
			executor.NewAIGatewayIdentityProviderAdapter(nil).MapCreateFields(t.Context(), nil, fields, &request),
		)
	}
	for _, resource := range resourceSet.AIGatewayPolicies {
		fields, err := resource.MutablePayloadMap()
		require.NoError(t, err)
		fields[planner.FieldAIGatewayID] = "gateway-id"
		var request kkComps.CreateAIGatewayPolicyRequest
		require.NoError(t, executor.NewAIGatewayPolicyAdapter(nil).MapCreateFields(t.Context(), nil, fields, &request))
	}
	for _, resource := range resourceSet.AIGatewayAgents {
		fields, err := resource.MutablePayloadMap()
		require.NoError(t, err)
		fields[planner.FieldAIGatewayID] = "gateway-id"
		var request kkComps.CreateAIGatewayAgentRequest
		require.NoError(t, executor.NewAIGatewayAgentAdapter(nil).MapCreateFields(t.Context(), nil, fields, &request))
	}
	for _, resource := range resourceSet.AIGatewayConsumers {
		fields, err := resource.MutablePayloadMap()
		require.NoError(t, err)
		fields[planner.FieldAIGatewayID] = "gateway-id"
		var request kkComps.CreateAIGatewayConsumerRequest
		require.NoError(t, executor.NewAIGatewayConsumerAdapter(nil).MapCreateFields(t.Context(), nil, fields, &request))
	}
	for _, resource := range resourceSet.AIGatewayConsumerCredentials {
		fields, err := resource.MutablePayloadMap()
		require.NoError(t, err)
		fields[planner.FieldAIGatewayID] = "gateway-id"
		fields[planner.FieldAIGatewayConsumerID] = "consumer-id"
		var request kkComps.CreateAIGatewayConsumerCredentialRequest
		require.NoError(
			t,
			executor.NewAIGatewayConsumerCredentialAdapter(nil).MapCreateFields(t.Context(), nil, fields, &request),
		)
	}
	for _, resource := range resourceSet.AIGatewayConsumerGroups {
		fields, err := resource.MutablePayloadMap()
		require.NoError(t, err)
		fields[planner.FieldAIGatewayID] = "gateway-id"
		var request kkComps.CreateAIGatewayConsumerGroupRequest
		require.NoError(
			t,
			executor.NewAIGatewayConsumerGroupAdapter(nil).MapCreateFields(t.Context(), nil, fields, &request),
		)
	}
	for _, resource := range resourceSet.AIGatewayModels {
		fields, err := resource.MutablePayloadMap()
		require.NoError(t, err)
		fields[planner.FieldAIGatewayID] = "gateway-id"
		var request kkComps.CreateAIGatewayModelRequest
		require.NoError(t, executor.NewAIGatewayModelAdapter(nil).MapCreateFields(t.Context(), nil, fields, &request))
	}
	for _, resource := range resourceSet.AIGatewayMCPServers {
		fields, err := resource.MutablePayloadMap()
		require.NoError(t, err)
		fields[planner.FieldAIGatewayID] = "gateway-id"
		var request kkComps.CreateAIGatewayMCPServerRequest
		require.NoError(t, executor.NewAIGatewayMCPServerAdapter(nil).MapCreateFields(t.Context(), nil, fields, &request))
	}
	for _, resource := range resourceSet.AIGatewayVaults {
		fields, err := resource.MutablePayloadMap()
		require.NoError(t, err)
		fields[planner.FieldAIGatewayID] = "gateway-id"
		var request kkComps.CreateAIGatewayVaultRequest
		require.NoError(t, executor.NewAIGatewayVaultAdapter(nil).MapCreateFields(t.Context(), nil, fields, &request))
	}
	for _, resource := range resourceSet.AIGatewayDataPlaneCertificates {
		var request kkComps.CreateAIGatewayDataPlaneCertificateRequest
		require.NoError(
			t,
			executor.NewAIGatewayDataPlaneCertificateAdapter(nil).MapCreateFields(
				t.Context(),
				nil,
				resource.PayloadMap(),
				&request,
			),
		)
	}
}
