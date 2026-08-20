package resources

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAIGatewayMCPServerRejectsLegacyTopLevelAccessFields(t *testing.T) {
	input := []byte(`{
		"ref": "support-listener",
		"ai_gateway": "support-gateway",
		"type": "listener",
		"name": "support-listener",
		"display_name": "Support Listener",
		"acl_attribute_type": "oauth_access_token",
		"access_token_claim_field": "sub",
		"config": {"route": {"paths": ["/support-listener"]}}
	}`)

	var resource AIGatewayMCPServerResource
	err := json.Unmarshal(input, &resource)
	require.Error(t, err)
	require.ErrorContains(t, err, `field "acl_attribute_type" must be nested under access`)
}

func TestAIGatewayMCPServerAllowsAccessFields(t *testing.T) {
	input := []byte(`{
		"ref": "support-listener",
		"ai_gateway": "support-gateway",
		"type": "listener",
		"name": "support-listener",
		"display_name": "Support Listener",
		"sources": ["support-tools"],
		"access": {
			"acl_attribute_type": "oauth_access_token",
			"access_token_claim_field": "sub",
			"identity_providers": ["support-oidc"],
			"metadata": {
				"authorization_servers": ["https://idp.example.com"],
				"resource": "https://mcp.example.com"
			},
			"default_tool_acls": {
				"allow": ["support-subject"]
			}
		},
		"config": {"route": {"paths": ["/support-listener"]}}
	}`)

	var resource AIGatewayMCPServerResource
	require.NoError(t, json.Unmarshal(input, &resource))
	payload, err := resource.PayloadMap()
	require.NoError(t, err)
	access := payload[SchemaFieldAccess].(map[string]any)
	require.Equal(t, []any{"support-oidc"}, access[SchemaFieldIdentityProviders])
	require.Equal(t, "https://mcp.example.com", access["metadata"].(map[string]any)["resource"])
}

func TestAIGatewayMCPServerRejectsAccessForConversionOnly(t *testing.T) {
	input := []byte(`{
		"ref": "support-tools",
		"ai_gateway": "support-gateway",
		"type": "conversion-only",
		"name": "support-tools",
		"display_name": "Support Tools",
		"access": {
			"acl_attribute_type": "consumer"
		},
		"config": {
			"url": "https://support-tools.example.com",
			"route": {"paths": ["/support-tools"]}
		}
	}`)

	var resource AIGatewayMCPServerResource
	err := json.Unmarshal(input, &resource)
	require.Error(t, err)
	require.ErrorContains(t, err, `field "access" is not supported when type is "conversion-only"`)
}
