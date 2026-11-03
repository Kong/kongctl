package resources

import (
	"encoding/json"
	"fmt"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/stretchr/testify/require"
)

func TestAIGatewayMCPServerACLRules(t *testing.T) {
	for _, location := range []string{"server", "tool"} {
		for _, rule := range []struct {
			name  string
			acls  string
			valid bool
		}{
			{"allow", `{"allow":["support"]}`, true},
			{"deny", `{"deny":["blocked"]}`, true},
			{"allow_when", `{"allow_when":["true"]}`, true},
			{"deny_when", `{"deny_when":["false"]}`, true},
			{"empty_allow", `{"allow":[]}`, true},
			{"combined", `{"allow":["support"],"deny":["blocked"]}`, false},
			{"combined_expressions", `{"allow_when":["true"],"deny_when":["false"]}`, false},
			{"mixed", `{"allow":["support"],"deny_when":["false"]}`, false},
			{"empty", `{}`, false},
			{"unknown", `{"unknown":[]}`, false},
		} {
			t.Run(location+"/"+rule.name, func(t *testing.T) {
				access := fmt.Sprintf(`"access":{"acl_attribute_type":"consumer","acls":%s}`, rule.acls)
				if location == "tool" {
					access = fmt.Sprintf(`"tools":[{"name":"lookup","access":{"acls":%s}}]`, rule.acls)
				}
				input := fmt.Sprintf(`{
					"ref":"listener","ai_gateway":"gateway","type":"passthrough-listener",
					"name":"listener","display_name":"Listener",
					"config":{"url":"https://example.com","route":{"paths":["/mcp"]}},%s
				}`, access)
				var resource AIGatewayMCPServerResource
				err := json.Unmarshal([]byte(input), &resource)
				if !rule.valid {
					require.ErrorContains(t, err, "acls must specify exactly one")
					return
				}
				require.NoError(t, err)
				resource.SetDefaults()
				payload, err := resource.MutablePayloadMap()
				require.NoError(t, err)
				update, err := marshalObjectToMap(resource.UpdateRequest(), "update")
				require.NoError(t, err)
				response, err := resource.PayloadMap()
				require.NoError(t, err)
				response["id"] = "server-id"
				response["created_at"] = "2026-11-03T00:00:00Z"
				response["updated_at"] = "2026-11-03T00:00:00Z"
				data, err := json.Marshal(response)
				require.NoError(t, err)
				var server kkComps.AIGatewayMCPServer
				require.NoError(t, json.Unmarshal(data, &server))
				observed, err := AIGatewayMCPServerMutablePayloadMap(server)
				require.NoError(t, err)
				dumped, err := AIGatewayMCPServerResourceFromResponse("gateway", server)
				require.NoError(t, err)
				dumpPayload, err := dumped.MutablePayloadMap()
				require.NoError(t, err)
				for _, result := range []map[string]any{payload, update, observed, dumpPayload} {
					if location == "tool" {
						result = result["tools"].([]any)[0].(map[string]any)
					}
					got, err := json.Marshal(result["access"].(map[string]any)["acls"])
					require.NoError(t, err)
					require.JSONEq(t, rule.acls, string(got))
				}
			})
		}
	}
}

func TestAIGatewayMCPServerDefaultToolACLsAllowCombinedRules(t *testing.T) {
	input := []byte(`{
		"type":"listener","name":"listener","display_name":"Listener",
		"sources":["tools"],
		"access":{"acl_attribute_type":"consumer","default_tool_acls":{
			"allow":["support"],"deny":["blocked"],"allow_when":["true"],"deny_when":["false"]
		}},
		"config":{"route":{"paths":["/mcp"]}}
	}`)
	var resource AIGatewayMCPServerResource
	require.NoError(t, json.Unmarshal(input, &resource))
	payload, err := resource.PayloadMap()
	require.NoError(t, err)
	acls := payload["access"].(map[string]any)["default_tool_acls"].(map[string]any)
	require.Len(t, acls, 4)
}

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
			"auth_strategies": ["support-oidc"],
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
	require.Equal(t, []any{"support-oidc"}, access[SchemaFieldAuthStrategies])
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
