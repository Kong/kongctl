package loader

import (
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/stretchr/testify/require"
)

const aiGatewayVaultYAML = `
ai_gateways:
  - ref: support-gateway
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
