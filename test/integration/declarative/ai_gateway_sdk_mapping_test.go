//go:build integration

package declarative_test

import (
	"os"
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
			resourceSet, err := loader.New().LoadFile(prepareAIGatewayScenarioConfig(t, path))
			require.NoError(t, err)
			mapAIGatewayScenarioCreatePayloads(t, resourceSet)
		})
	}
}

func prepareAIGatewayScenarioConfig(t *testing.T, path string) string {
	t.Helper()
	if filepath.Base(filepath.Dir(filepath.Dir(path))) != "vault-matrix" {
		return path
	}
	// The live scenario generates its certificate before planning. Reuse the
	// existing public test certificate for offline schema and SDK mapping checks.
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	cert, err := os.ReadFile(filepath.Join(
		filepath.Dir(path), "..", "..", "runtime-tls", "testdata", "certs", "runtime.pem",
	))
	require.NoError(t, err)
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "certs"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "certs", "runtime.pem"), cert, 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "certs", "runtime.key"), []byte("fake-private-key"), 0o600))
	t.Setenv("KONGCTL_E2E_VAULT_SECRET", "fake-vault-secret")
	output := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(output, data, 0o600))
	return output
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
		if resource.DeploymentType != nil {
			fields[planner.FieldDeploymentType] = *resource.DeploymentType
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
	for _, resource := range resourceSet.AIGatewayAuthStrategies {
		fields, err := resource.MutablePayloadMap()
		require.NoError(t, err)
		fields[planner.FieldAIGatewayID] = "gateway-id"
		var request kkComps.CreateAIGatewayAuthStrategyRequest
		require.NoError(
			t,
			executor.NewAIGatewayAuthStrategyAdapter(nil).MapCreateFields(t.Context(), nil, fields, &request),
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
	for _, resource := range resourceSet.AIGatewayConfigStores {
		fields, err := resource.MutablePayloadMap()
		require.NoError(t, err)
		var request kkComps.CreateAIGatewayConfigStoreRequest
		require.NoError(
			t,
			executor.NewAIGatewayConfigStoreAdapter(nil).MapCreateFields(t.Context(), nil, fields, &request),
		)
	}
	for _, resource := range resourceSet.AIGatewayConfigStoreSecrets {
		fields, err := resource.MutablePayloadMap()
		require.NoError(t, err)
		var request kkComps.CreateAIGatewayConfigStoreSecretRequest
		require.NoError(
			t,
			executor.NewAIGatewayConfigStoreSecretAdapter(nil).MapCreateFields(t.Context(), nil, fields, &request),
		)
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
