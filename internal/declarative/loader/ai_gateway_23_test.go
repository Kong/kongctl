package loader

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAIGateway23ManifestFields(t *testing.T) {
	manifest := `ai_gateways:
- ref: gateway
  name: gateway
  display_name: Gateway
  model_providers:
  - ref: baseten
    name: baseten
    display_name: Baseten
    type: baseten
    config: {auth: {type: basic, headers: []}}
  auth_strategies:
  - ref: oidc
    name: oidc
    display_name: OIDC
    type: openid-connect
    config:
      issuer: https://issuer.test
      cache_tokens_salt: example-cache-salt
      credential_claims: [[upstream, credential]]
  agents:
  - ref: agent
    name: agent
    display_name: Agent
    type: a2a
    config:
      url: https://agent.test
      certificates: [client-cert]
      ca_certificates: [trusted-ca]
  models:
  - ref: model
    name: model
    display_name: Model
    type: model
    datastores: [{name: existing-vector-store}]
    formats: [{type: '*'}]
    capabilities: ['*']
    config:
      route: {}
      model: {}
      balancer: {algorithm: round-robin, failover_criteria: [http_400, http_429]}
    targets:
    - name: baseten-model
      provider: baseten
      config: {type: baseten}
    - name: bedrock-model
      provider: existing-bedrock
      config: {type: bedrock, region: us-east-1, vpc_endpoint: private-bedrock.test, endpoint_type: mantle}
  mcp_servers:
  - ref: mcp
    name: mcp
    display_name: MCP
    type: passthrough-listener
    tools:
    - name: search
      access:
        acls:
          allow_when: ["http.headers.x_end_user_id != null"]
    access:
      acl_attribute_type: consumer
      acls:
        allow_when: ["http.headers.x_end_user_id != null"]
      default_tool_acls:
        deny_when: ["http.headers.x_end_user_id == null"]
    config:
      url: https://mcp.test
      certificates: [client-cert]
      ca_certificates: [trusted-ca]
      route: {paths: [/mcp], methods: [POST]}
    datastores: [{name: credential-store}]
    token_vault: {provider: upstream-credentials}
    rate_limiting:
      limits: [{method: tools/list, limit: 10, window_size: 60}]
`
	rs, err := New().LoadFile(writeLoaderTestFile(t, manifest))
	require.NoError(t, err)
	provider, err := rs.AIGatewayProviders[0].MutablePayloadMap()
	require.NoError(t, err)
	require.Equal(t, "baseten", provider["type"])
	auth, err := rs.AIGatewayAuthStrategies[0].MutablePayloadMap()
	require.NoError(t, err)
	require.Equal(
		t,
		[]any{[]any{"upstream", "credential"}},
		auth["config"].(map[string]any)["credential_claims"],
	)
	agent, err := rs.AIGatewayAgents[0].MutablePayloadMap()
	require.NoError(t, err)
	require.Equal(t, []any{"client-cert"}, agent["config"].(map[string]any)["certificates"])
	model, err := rs.AIGatewayModels[0].MutablePayloadMap()
	require.NoError(t, err)
	require.Equal(t, []any{"*"}, model["capabilities"])
	require.Equal(t, "*", model["formats"].([]any)[0].(map[string]any)["type"])
	targets := model["targets"].([]any)
	require.Equal(t, "baseten", targets[0].(map[string]any)["config"].(map[string]any)["type"])
	bedrock := targets[1].(map[string]any)["config"].(map[string]any)
	require.Equal(t, "private-bedrock.test", bedrock["vpc_endpoint"])
	require.Equal(t, "mantle", bedrock["endpoint_type"])
	mcp, err := rs.AIGatewayMCPServers[0].MutablePayloadMap()
	require.NoError(t, err)
	require.Equal(t, []any{map[string]any{"name": "credential-store"}}, mcp["datastores"])
	require.Equal(t, "upstream-credentials", mcp["token_vault"].(map[string]any)["provider"])
	require.NotEmpty(t, mcp["rate_limiting"])
	access := mcp["access"].(map[string]any)
	require.Equal(t, []any{"http.headers.x_end_user_id != null"}, access["acls"].(map[string]any)["allow_when"])
	require.Equal(
		t,
		[]any{"http.headers.x_end_user_id == null"},
		access["default_tool_acls"].(map[string]any)["deny_when"],
	)
	tool := mcp["tools"].([]any)[0].(map[string]any)
	toolAccess := tool["access"].(map[string]any)
	require.Equal(t, []any{"http.headers.x_end_user_id != null"}, toolAccess["acls"].(map[string]any)["allow_when"])
}

func TestAIGatewayDatastoreLoadAndDump(t *testing.T) {
	for _, tc := range []struct{ kind, config string }{
		{"redis-ce", `{host: redis.test, cloud_authentication: {auth_provider: aws, aws_cache_name: cache}}`},
		{"redis-ee", `{host: redis.test}`},
		{"vectordb", `{host: database.test, database: kong}`},
		{"token-vault", `{directory: credentials, current_version: v1, keys: [{version: v1}]}`},
	} {
		for _, nested := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/nested=%t", tc.kind, nested), func(t *testing.T) {
				parent := "ai_gateways:\n- ref: gateway\n  name: gateway\n  display_name: Gateway\n"
				child := fmt.Sprintf(
					"- ref: store\n  name: store\n  display_name: Store\n  type: %s\n  config: %s\n",
					tc.kind,
					tc.config,
				)
				manifest := parent + "ai_gateway_datastores:\n" + child + "  ai_gateway: gateway\n"
				if nested {
					manifest = parent + "  datastores:\n" + "  " + strings.ReplaceAll(
						strings.TrimSuffix(child, "\n"),
						"\n",
						"\n  ",
					) + "\n"
				}
				rs, err := New().LoadFile(writeLoaderTestFile(t, manifest))
				require.NoError(t, err)
				require.Len(t, rs.AIGatewayDatastores, 1)
				require.Empty(t, rs.AIGateways[0].Datastores)
				encoded, err := json.Marshal(rs)
				require.NoError(t, err)
				reloaded, err := New().LoadFile(writeLoaderTestFile(t, string(encoded)))
				require.NoError(t, err)
				payload, err := reloaded.AIGatewayDatastores[0].PayloadMap()
				require.NoError(t, err)
				require.Equal(t, tc.kind, payload["type"])
			})
		}
	}
}

func TestAIGatewayDatastoreEncryptionKeyMustBeDeferred(t *testing.T) {
	manifest := `ai_gateways:
- ref: gateway
  name: gateway
  display_name: Gateway
  datastores:
  - ref: storage
    name: storage
    display_name: Storage
    type: token-vault
    config: {directory: credentials, current_version: v1, keys: [{version: v1, key: plaintext-sentinel}]}
`
	_, err := New().LoadFile(writeLoaderTestFile(t, manifest))
	require.ErrorContains(t, err, "requires !secret")
	require.NotContains(t, err.Error(), "plaintext-sentinel")
}
