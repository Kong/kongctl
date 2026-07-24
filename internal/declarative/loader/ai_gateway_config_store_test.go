package loader

import (
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
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
