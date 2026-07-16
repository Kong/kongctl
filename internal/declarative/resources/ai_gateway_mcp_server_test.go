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
		"access": {
			"acl_attribute_type": "oauth_access_token",
			"access_token_claim_field": "sub",
			"default_tool_acls": {
				"allow": ["support-subject"]
			}
		},
		"config": {"route": {"paths": ["/support-listener"]}}
	}`)

	var resource AIGatewayMCPServerResource
	require.NoError(t, json.Unmarshal(input, &resource))
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
