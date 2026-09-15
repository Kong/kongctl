package loader

import (
	"encoding/json"
	"fmt"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/tags"
	testdecl "github.com/kong/kongctl/test/declarative"
	"github.com/stretchr/testify/require"
)

func TestLoaderAIGatewayVaultAPIConfigurations(t *testing.T) {
	for _, tc := range testdecl.VaultCases(t) {
		for _, nested := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/nested=%t", tc.Name, nested), func(t *testing.T) {
				vault := map[string]any{"ref": tc.Name, "name": tc.Name, "type": tc.Type, "config": tc.Config}
				gateway := map[string]any{"ref": "gateway", "name": "gateway", "display_name": "Gateway"}
				document := map[string]any{"ai_gateways": []any{gateway}}
				if nested {
					gateway["vaults"] = []any{vault}
				} else {
					vault["ai_gateway"] = "gateway"
					document["ai_gateway_vaults"] = []any{vault}
				}
				data, err := json.Marshal(document)
				require.NoError(t, err)
				rs, err := New().LoadFile(writeLoaderTestFile(t, string(data)))
				require.NoError(t, err)
				require.Len(t, rs.AIGatewayVaults, 1)
				payload, err := rs.AIGatewayVaults[0].PayloadMap()
				require.NoError(t, err)
				require.Equal(t, tc.Config, payload["config"])

				// Exercise the response conversion used by dump, then reload its output.
				payload["id"] = "77426bee-2bca-4005-81af-284868fd3038"
				payload["created_at"] = "2026-01-01T00:00:00Z"
				payload["updated_at"] = "2026-01-01T00:00:00Z"
				data, err = json.Marshal(payload)
				require.NoError(t, err)
				var response kkComps.AIGatewayVault
				require.NoError(t, json.Unmarshal(data, &response))
				dumped, err := resources.AIGatewayVaultResourceFromResponse("gateway", response)
				require.NoError(t, err)
				delete(gateway, "vaults")
				document["ai_gateway_vaults"] = []any{dumped}
				data, err = json.Marshal(document)
				require.NoError(t, err)
				reloaded, err := New().LoadFile(writeLoaderTestFile(t, string(data)))
				require.NoError(t, err)
				actual, err := reloaded.AIGatewayVaults[0].PayloadMap()
				require.NoError(t, err)
				expected, err := resources.AIGatewayVaultMutablePayloadMap(response)
				require.NoError(t, err)
				require.Equal(t, expected["config"], actual["config"])
			})
		}
	}
}

func TestLoaderRejectsUnknownAIGatewayVaultConfigFields(t *testing.T) {
	for _, tc := range testdecl.VaultCases(t) {
		t.Run(tc.Name, func(t *testing.T) {
			tc.Config["unknown_field"] = "value"
			config, err := json.Marshal(tc.Config)
			require.NoError(t, err)
			_, err = New().LoadFile(writeLoaderTestFile(t, vaultConfigYAML(tc.Type, string(config))))
			require.ErrorContains(t, err, "unknown field 'unknown_field'")
		})
	}
}

func TestLoaderRejectsInvalidAIGatewayVaultConfigurations(t *testing.T) {
	for _, tc := range []struct{ name, kind, config string }{
		{"unknown type", "invalid", `{}`},
		{"missing config store", "konnect", `{}`},
		{"missing project", "gcp", `{}`},
		{"missing location", "azure", `{"vault_uri":"https://example.vault.azure.net"}`},
		{"missing vault URI", "azure", `{"location":"eastus"}`},
		{"missing account", "conjur", `{"login":"workload","endpoint_url":"https://conjur.example.net"}`},
		{"wrong ARN type", "aws", `{"assume_role_arn":42}`},
		{"wrong vault branch", "env", `{"region":"us-east-1"}`},
		{"missing auth method", "hcv", `{"host":"vault.example.net","port":8200}`},
		{"invalid auth method", "hcv", `{"auth_method":"invalid","host":"vault.example.net","port":8200}`},
		{"missing host", "hcv", `{"auth_method":"token","port":8200}`},
		{"wrong auth branch", "hcv", `{"auth_method":"token","host":"vault.example.net","port":8200,"nonce":"n"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := New().LoadFile(writeLoaderTestFile(t, vaultConfigYAML(tc.kind, tc.config)))
			require.Error(t, err)
		})
	}
}

func TestLoaderAIGatewayVaultDeferredSecrets(t *testing.T) {
	for _, tc := range []struct{ kind, config, field string }{
		{"conjur", "{account: org, endpoint_url: 'https://conjur.example.net', login: workload, api_key: %s}", "api_key"},
		{"hcv", "{auth_method: cert, host: vault.example.net, port: 8200, cert: public-cert, key: %s}", "key"},
		{"hcv", "{auth_method: token, host: vault.example.net, port: 8200, token: %s}", "token"},
		{"hcv", "{auth_method: jwt, host: vault.example.net, port: 8200, role: role, " +
			"token_endpoint: 'https://idp.example.net', client_id: client, client_secret: %s}", "client_secret"},
		{"hcv", "{auth_method: aws_iam, host: vault.example.net, port: 8200, role: role, " +
			"region: us-east-1, secret_access_key: %s}", "secret_access_key"},
		{"hcv", "{auth_method: approle, host: vault.example.net, port: 8200, secret_id: %s}", "secret_id"},
	} {
		t.Run(tc.field, func(t *testing.T) {
			config := fmt.Sprintf(tc.config, "!secret {source: !env UNAVAILABLE_VAULT_SECRET}")
			rs, err := New().LoadFile(writeLoaderTestFile(t, vaultConfigYAML(tc.kind, config)))
			require.NoError(t, err)
			source := rs.GetSecretSources("vault")["/config/"+tc.field]
			require.Len(t, source.Expression.Parts, 1)
			require.Equal(t, "UNAVAILABLE_VAULT_SECRET", source.Expression.Parts[0].Source.Reference)
			config = fmt.Sprintf(tc.config, "must-not-appear-in-error")
			_, err = New().LoadFile(writeLoaderTestFile(t, vaultConfigYAML(tc.kind, config)))
			require.ErrorContains(t, err, "requires !secret")
			require.NotContains(t, err.Error(), "must-not-appear-in-error")
		})
	}
}

func vaultConfigYAML(kind, config string) string {
	return fmt.Sprintf(`
ai_gateways:
  - ref: gateway
    name: gateway
    display_name: Gateway
ai_gateway_vaults:
  - ref: vault
    ai_gateway: gateway
    type: %s
    name: vault
    config: %s
`, kind, config)
}

const aiGatewayVaultYAML = `
ai_gateways:
  - ref: support-gateway
    name: support-gateway
    display_name: Support Gateway
    vaults:
      - ref: support-env
        type: env
        name: support-env
        description: Support environment variables
        config:
          prefix: SUPPORT_
          base64_decode: false
`

func TestLoaderExtractsNestedAIGatewayVaults(t *testing.T) {
	path := writeLoaderTestFile(t, aiGatewayVaultYAML)

	rs, err := New().LoadFromSources([]Source{{Path: path, Type: SourceTypeFile}}, false)
	require.NoError(t, err)
	require.Len(t, rs.AIGateways, 1)
	require.Empty(t, rs.AIGateways[0].Vaults)
	require.Len(t, rs.AIGatewayVaults, 1)
	require.Equal(t, "support-gateway", rs.AIGatewayVaults[0].AIGateway)
	require.Equal(t, "support-env", rs.AIGatewayVaults[0].Name())
	require.True(t, rs.SyncScope.ChildInScope(
		resources.ResourceTypeAIGateway,
		"support-gateway",
		resources.ResourceTypeAIGatewayVault,
	))
}

func TestLoaderValidatesAIGatewayVaultParentAndDuplicates(t *testing.T) {
	rootOnly := `
ai_gateway_vaults:
  - ref: support-env
    ai_gateway: missing-gateway
    type: env
    name: support-env
    config: {prefix: SUPPORT_}
`
	_, err := New().LoadFromSources([]Source{{Path: writeLoaderTestFile(t, rootOnly), Type: SourceTypeFile}}, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "references unknown ai_gateway")

	duplicates := `
ai_gateways:
  - ref: support-gateway
    name: support-gateway
    display_name: Support Gateway
    vaults:
      - ref: support-env
        type: env
        name: support-env
        config: {prefix: SUPPORT_}
      - ref: support-env-2
        type: env
        name: support-env
        config: {prefix: SUPPORT_}
`
	_, err = New().LoadFromSources([]Source{{Path: writeLoaderTestFile(t, duplicates), Type: SourceTypeFile}}, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicate ai_gateway_vault name")
}

func TestLoaderRejectsRootLevelEmptyAIGatewayVaults(t *testing.T) {
	input := `ai_gateway_vaults: []`

	_, err := New().LoadFromSources([]Source{{Path: writeLoaderTestFile(t, input), Type: SourceTypeFile}}, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "ai_gateway_vaults cannot be empty")
}

func TestLoaderAcceptsAIGatewayVaultDeferredExternalParentRef(t *testing.T) {
	input := `
ai_gateways:
  - ref: external-shared-gateway
    _external:
      selector:
        matchFields:
          display_name: Shared Gateway
ai_gateway_vaults:
  - ref: support-env
    ai_gateway: !ref external-shared-gateway#id
    type: env
    name: support-env
    config: {prefix: SUPPORT_}
`

	rs, err := New().LoadFromSources([]Source{{Path: writeLoaderTestFile(t, input), Type: SourceTypeFile}}, false)
	require.NoError(t, err)
	require.Len(t, rs.AIGatewayVaults, 1)
	require.Equal(t, tags.RefPlaceholderPrefix+"external-shared-gateway#id", rs.AIGatewayVaults[0].AIGateway)
	require.True(t, rs.SyncScope.ChildInScope(
		resources.ResourceTypeAIGateway,
		"external-shared-gateway",
		resources.ResourceTypeAIGatewayVault,
	))
}
