package resources

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEventGatewaySchemaRegistryResourceAllowsOmittedReadOnlyPassword(t *testing.T) {
	input := []byte(`{
		"ref": "schema-registry",
		"name": "schema-registry",
		"type": "confluent",
		"config": {
			"schema_type": "json",
			"endpoint": "https://schema-registry.example.com",
			"authentication": {
				"type": "basic",
				"username": "testuser"
			}
		}
	}`)

	var resource EventGatewaySchemaRegistryResource
	require.NoError(t, json.Unmarshal(input, &resource))
	require.NotNil(t, resource.SchemaRegistryConfluent)
	require.NotNil(t, resource.SchemaRegistryConfluent.Config.Authentication)
	require.NotNil(t, resource.SchemaRegistryConfluent.Config.Authentication.SchemaRegistryAuthenticationBasic)
	assert.Empty(t, resource.SchemaRegistryConfluent.Config.Authentication.SchemaRegistryAuthenticationBasic.Password)

	output, err := json.Marshal(resource)
	require.NoError(t, err)
	assert.NotContains(t, string(output), `"password"`)
}

func TestEventGatewaySchemaRegistryResourcePreservesPublicVaultReference(t *testing.T) {
	input := []byte(`{
		"ref": "schema-registry",
		"name": "schema-registry",
		"type": "confluent",
		"config": {
			"schema_type": "json",
			"endpoint": "https://schema-registry.example.com",
			"authentication": {
				"type": "basic",
				"username": "testuser",
				"password": "${vault['env']['SCHEMA_REGISTRY_PASSWORD']}"
			}
		}
	}`)

	var resource EventGatewaySchemaRegistryResource
	require.NoError(t, json.Unmarshal(input, &resource))

	output, err := json.Marshal(resource)
	require.NoError(t, err)
	assert.Contains(t, string(output), `"password":"${vault['env']['SCHEMA_REGISTRY_PASSWORD']}"`)
}
