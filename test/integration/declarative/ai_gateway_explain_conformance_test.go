//go:build integration

package declarative_test

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/kong/kongctl/internal/declarative/loader"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/stretchr/testify/require"
)

func TestAIGatewayScenarioResourcesConformToExplainSchemas(t *testing.T) {
	t.Setenv("KONGCTL_E2E_WEATHERAPI_API_KEY", "fake-weather-api-key")

	for _, path := range aiGatewayScenarioConfigPaths(t) {
		t.Run(filepath.Base(filepath.Dir(filepath.Dir(path)))+"/"+filepath.Base(path), func(t *testing.T) {
			resourceSet, err := loader.New().LoadFile(path)
			require.NoError(t, err)
			validateAIGatewayScenarioExplainSchemas(t, resourceSet)
		})
	}
}

func validateAIGatewayScenarioExplainSchemas(t *testing.T, resourceSet *resources.ResourceSet) {
	t.Helper()

	for i := range resourceSet.AIGateways {
		validateResourceAgainstExplainSchema(t, string(resources.ResourceTypeAIGateway), &resourceSet.AIGateways[i])
	}
	for i := range resourceSet.AIGatewayProviders {
		validateResourceAgainstExplainSchema(
			t,
			string(resources.ResourceTypeAIGatewayProvider),
			&resourceSet.AIGatewayProviders[i],
		)
	}
	for i := range resourceSet.AIGatewayIdentityProviders {
		validateResourceAgainstExplainSchema(
			t,
			string(resources.ResourceTypeAIGatewayIdentityProvider),
			&resourceSet.AIGatewayIdentityProviders[i],
		)
	}
	for i := range resourceSet.AIGatewayPolicies {
		validateResourceAgainstExplainSchema(
			t,
			string(resources.ResourceTypeAIGatewayPolicy),
			&resourceSet.AIGatewayPolicies[i],
		)
	}
	for i := range resourceSet.AIGatewayAgents {
		validateResourceAgainstExplainSchema(t, string(resources.ResourceTypeAIGatewayAgent), &resourceSet.AIGatewayAgents[i])
	}
	for i := range resourceSet.AIGatewayConsumers {
		validateResourceAgainstExplainSchema(
			t,
			string(resources.ResourceTypeAIGatewayConsumer),
			&resourceSet.AIGatewayConsumers[i],
		)
	}
	for i := range resourceSet.AIGatewayConsumerCredentials {
		validateResourceAgainstExplainSchema(
			t,
			string(resources.ResourceTypeAIGatewayConsumerCredential),
			&resourceSet.AIGatewayConsumerCredentials[i],
		)
	}
	for i := range resourceSet.AIGatewayConsumerGroups {
		validateResourceAgainstExplainSchema(
			t,
			string(resources.ResourceTypeAIGatewayConsumerGroup),
			&resourceSet.AIGatewayConsumerGroups[i],
		)
	}
	for i := range resourceSet.AIGatewayModels {
		validateResourceAgainstExplainSchema(t, string(resources.ResourceTypeAIGatewayModel), &resourceSet.AIGatewayModels[i])
	}
	for i := range resourceSet.AIGatewayMCPServers {
		validateResourceAgainstExplainSchema(
			t,
			string(resources.ResourceTypeAIGatewayMCPServer),
			&resourceSet.AIGatewayMCPServers[i],
		)
	}
	for i := range resourceSet.AIGatewayVaults {
		validateResourceAgainstExplainSchema(t, string(resources.ResourceTypeAIGatewayVault), &resourceSet.AIGatewayVaults[i])
	}
	for i := range resourceSet.AIGatewayDataPlaneCertificates {
		validateResourceAgainstExplainSchema(
			t,
			string(resources.ResourceTypeAIGatewayDataPlaneCertificate),
			&resourceSet.AIGatewayDataPlaneCertificates[i],
		)
	}
}

func validateResourceAgainstExplainSchema(t *testing.T, resourcePath string, resource any) {
	t.Helper()

	subject, err := resources.ResolveExplainSubject(resourcePath)
	require.NoError(t, err)
	schemaData, err := json.Marshal(resources.RenderExplainSchema(subject))
	require.NoError(t, err)
	var schemaDocument any
	require.NoError(t, json.Unmarshal(schemaData, &schemaDocument))

	const schemaURL = "https://konghq.com/kongctl/explain-schema.json"
	compiler := jsonschema.NewCompiler()
	require.NoError(t, compiler.AddResource(schemaURL, schemaDocument))
	compiled, err := compiler.Compile(schemaURL)
	require.NoError(t, err)

	resourceData, err := json.Marshal(resource)
	require.NoError(t, err)
	var resourceDocument any
	require.NoError(t, json.Unmarshal(resourceData, &resourceDocument))
	require.NoErrorf(t, compiled.Validate(resourceDocument), "%s does not conform to its explain schema", resourcePath)
}
