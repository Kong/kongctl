package loader

import (
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/stretchr/testify/require"
)

func TestLoaderExtractsNestedAIGatewayConfigStores(t *testing.T) {
	input := `
ai_gateways:
  - ref: support-gateway
    display_name: Support Gateway
    config_stores:
      - ref: support-store
        name: support-store
        display_name: Support-Store
`
	rs, err := New().LoadFromSources([]Source{{
		Path: writeLoaderTestFile(t, input),
		Type: SourceTypeFile,
	}}, false)
	require.NoError(t, err)
	require.Len(t, rs.AIGateways, 1)
	require.Empty(t, rs.AIGateways[0].ConfigStores)
	require.Len(t, rs.AIGatewayConfigStores, 1)
	require.Equal(t, "support-gateway", rs.AIGatewayConfigStores[0].AIGateway)
	require.Equal(t, "support-store", rs.AIGatewayConfigStores[0].Name)
	require.True(t, rs.SyncScope.ChildInScope(
		resources.ResourceTypeAIGateway,
		"support-gateway",
		resources.ResourceTypeAIGatewayConfigStore,
	))
}

func TestLoaderValidatesAIGatewayConfigStoreParentAndDuplicates(t *testing.T) {
	rootOnly := `
ai_gateway_config_stores:
  - ref: support-store
    ai_gateway: missing-gateway
    name: support-store
`
	_, err := New().LoadFromSources([]Source{{
		Path: writeLoaderTestFile(t, rootOnly),
		Type: SourceTypeFile,
	}}, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "references unknown ai_gateway")

	duplicates := `
ai_gateways:
  - ref: support-gateway
    display_name: Support Gateway
    config_stores:
      - ref: support-store
        name: support-store
      - ref: support-store-2
        name: support-store
`
	_, err = New().LoadFromSources([]Source{{
		Path: writeLoaderTestFile(t, duplicates),
		Type: SourceTypeFile,
	}}, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicate ai_gateway_config_store name")
}

func TestLoaderRejectsRootLevelEmptyAIGatewayConfigStores(t *testing.T) {
	_, err := New().LoadFromSources([]Source{{
		Path: writeLoaderTestFile(t, "ai_gateway_config_stores: []"),
		Type: SourceTypeFile,
	}}, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "ai_gateway_config_stores cannot be empty")
}

func TestLoaderRejectsInvalidAIGatewayConfigStoreDisplayName(t *testing.T) {
	input := `
ai_gateways:
  - ref: support-gateway
    display_name: Support Gateway
    config_stores:
      - ref: support-store
        name: support-store
        display_name: Support Store
`
	_, err := New().LoadFromSources([]Source{{
		Path: writeLoaderTestFile(t, input),
		Type: SourceTypeFile,
	}}, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "display_name")
}

func TestLoaderAcceptsVaultConfigStoreReference(t *testing.T) {
	input := `
ai_gateways:
  - ref: support-gateway
    display_name: Support Gateway
    config_stores:
      - ref: support-store
        name: support-store
    vaults:
      - ref: support-vault
        type: konnect
        name: support-vault
        config:
          config_store_id: !ref support-store#id
`
	rs, err := New().LoadFromSources([]Source{{
		Path: writeLoaderTestFile(t, input),
		Type: SourceTypeFile,
	}}, false)
	require.NoError(t, err)
	require.Len(t, rs.AIGatewayVaults, 1)
	payload, err := rs.AIGatewayVaults[0].PayloadMap()
	require.NoError(t, err)
	config := payload[resources.SchemaFieldConfig].(map[string]any)
	require.Equal(t, "__REF__:support-store#id", config[resources.SchemaFieldConfigStoreID])
}

func TestLoaderExtractsNestedAIGatewayConfigStoreSecrets(t *testing.T) {
	input := `
ai_gateways:
  - ref: support-gateway
    display_name: Support Gateway
    config_stores:
      - ref: support-store
        name: support-store
        secrets:
          - ref: support-openai-header
            key: openai-auth-header
            value: !secret {source: !env OPENAI_AUTH_HEADER}
`
	rs, err := New().LoadFile(writeLoaderTestFile(t, input))
	require.NoError(t, err)
	require.Len(t, rs.AIGatewayConfigStoreSecrets, 1)
	require.Equal(t, "support-store", rs.AIGatewayConfigStoreSecrets[0].AIGatewayConfigStore)
	require.Equal(t, "openai-auth-header", rs.AIGatewayConfigStoreSecrets[0].Key)
	require.True(t, tags.IsSecretPlaceholder(rs.AIGatewayConfigStoreSecrets[0].Value))
	require.Contains(t, rs.GetSecretSources("support-openai-header"), "/value")
	require.True(t, rs.SyncScope.ChildInScope(
		resources.ResourceTypeAIGatewayConfigStore,
		"support-store",
		resources.ResourceTypeAIGatewayConfigStoreSecret,
	))
}

func TestLoaderAcceptsRootAIGatewayConfigStoreSecret(t *testing.T) {
	rs, err := New().LoadFile(writeLoaderTestFile(t, `
ai_gateways:
  - ref: support-gateway
    display_name: Support Gateway
ai_gateway_config_stores:
  - ref: support-store
    ai_gateway: !ref support-gateway
    name: support-store
ai_gateway_config_store_secrets:
  - ref: support-api-key
    ai_gateway_config_store: !ref support-store
    key: support-api-key
    value: !secret {source: !env SUPPORT_API_KEY}
`))
	require.NoError(t, err)
	require.Len(t, rs.AIGatewayConfigStoreSecrets, 1)
	require.Equal(t, "support-store", resources.NormalizeResourceRef(
		rs.AIGatewayConfigStoreSecrets[0].AIGatewayConfigStore,
	))
	require.Contains(t, rs.GetSecretSources("support-api-key"), "/value")
}

func TestLoaderRejectsDuplicateAIGatewayConfigStoreSecretRefsAcrossStores(t *testing.T) {
	_, err := New().LoadFile(writeLoaderTestFile(t, `
ai_gateways:
  - ref: support-gateway
    display_name: Support Gateway
    config_stores:
      - ref: primary-store
        name: primary-store
        secrets:
          - key: api-key
            value: !secret {source: !env PRIMARY_API_KEY}
      - ref: fallback-store
        name: fallback-store
        secrets:
          - key: api-key
            value: !secret {source: !env FALLBACK_API_KEY}
`))
	require.ErrorContains(t, err, "duplicate ref 'api-key'")
}

func TestValidatorRejectsDuplicateAIGatewayConfigStoreSecretRefsAcrossStores(t *testing.T) {
	rs := &resources.ResourceSet{
		AIGatewayConfigStores: []resources.AIGatewayConfigStoreResource{
			{BaseResource: resources.BaseResource{Ref: "primary-store"}},
			{BaseResource: resources.BaseResource{Ref: "fallback-store"}},
		},
		AIGatewayConfigStoreSecrets: []resources.AIGatewayConfigStoreSecretResource{
			{
				BaseResource:         resources.BaseResource{Ref: "api-key"},
				AIGatewayConfigStore: "primary-store",
				Key:                  "api-key",
			},
			{
				BaseResource:         resources.BaseResource{Ref: "api-key"},
				AIGatewayConfigStore: "fallback-store",
				Key:                  "api-key",
			},
		},
	}

	err := New().validateAIGatewayConfigStoreSecrets(rs)
	require.ErrorContains(t, err, "duplicate ref 'api-key' (already defined as ai_gateway_config_store_secret)")
}

func TestLoaderAIGatewayConfigStoreSecretSyncScope(t *testing.T) {
	t.Run("omitted is unmanaged", func(t *testing.T) {
		rs, err := New().LoadFile(writeLoaderTestFile(t, `
ai_gateways:
  - ref: support-gateway
    display_name: Support Gateway
    config_stores:
      - ref: support-store
        name: support-store
`))
		require.NoError(t, err)
		require.False(t, rs.SyncScope.ChildInScope(
			resources.ResourceTypeAIGatewayConfigStore,
			"support-store",
			resources.ResourceTypeAIGatewayConfigStoreSecret,
		))
	})

	t.Run("explicit empty is managed", func(t *testing.T) {
		rs, err := New().LoadFile(writeLoaderTestFile(t, `
ai_gateways:
  - ref: support-gateway
    display_name: Support Gateway
    config_stores:
      - ref: support-store
        name: support-store
        secrets: []
`))
		require.NoError(t, err)
		require.True(t, rs.SyncScope.ChildInScope(
			resources.ResourceTypeAIGatewayConfigStore,
			"support-store",
			resources.ResourceTypeAIGatewayConfigStoreSecret,
		))
	})

	t.Run("root empty is rejected", func(t *testing.T) {
		_, err := New().LoadFile(writeLoaderTestFile(t, "ai_gateway_config_store_secrets: []"))
		require.ErrorContains(t, err, "ai_gateway_config_store_secrets cannot be empty")
	})
}

func TestLoaderRejectsLiteralAIGatewayConfigStoreSecretWithoutEchoingValue(t *testing.T) {
	secretValue := "must-not-appear"
	_, err := New().LoadFile(writeLoaderTestFile(t, `
ai_gateways:
  - ref: support-gateway
    display_name: Support Gateway
    config_stores:
      - ref: support-store
        name: support-store
        secrets:
          - ref: support-openai-header
            key: openai-auth-header
            value: `+secretValue+`
`))
	require.ErrorContains(t, err, "field /value")
	require.NotContains(t, err.Error(), secretValue)
}

func TestLoaderAcceptsDumpedAIGatewayConfigStoreSecretWithoutValue(t *testing.T) {
	rs, err := New().LoadFile(writeLoaderTestFile(t, `
ai_gateways:
  - ref: support-gateway-id
    display_name: Support Gateway
    config_stores:
      - ref: support-store-id
        name: support-store
        secrets:
          - ref: support-api-key
            key: support-api-key
`))
	require.NoError(t, err)
	require.Len(t, rs.AIGatewayConfigStoreSecrets, 1)
	require.Equal(t, "support-api-key", rs.AIGatewayConfigStoreSecrets[0].Key)
	require.Empty(t, rs.AIGatewayConfigStoreSecrets[0].Value)
}
