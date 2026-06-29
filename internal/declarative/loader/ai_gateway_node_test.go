package loader

import (
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/stretchr/testify/require"
)

const aiGatewayNodeYAML = `
ai_gateways:
  - ref: support-gateway
    display_name: Support Gateway
    nodes:
      - ref: support-node
        id: 3f2b1c9a-8d7e-4f6a-9b0c-1d2e3f4a5b6c
        version: "3.11.0"
        hostname: support-node-1
        type: data-plane
        config_version: "2024.06.01"
`

func TestLoaderExtractsNestedAIGatewayNodes(t *testing.T) {
	path := writeLoaderTestFile(t, aiGatewayNodeYAML)

	rs, err := New().LoadFromSources([]Source{{Path: path, Type: SourceTypeFile}}, false)
	require.NoError(t, err)
	require.Len(t, rs.AIGateways, 1)
	require.Empty(t, rs.AIGateways[0].Nodes)
	require.Len(t, rs.AIGatewayNodes, 1)
	require.Equal(t, "support-gateway", rs.AIGatewayNodes[0].AIGateway)
	require.Equal(t, "support-node", rs.AIGatewayNodes[0].Ref)
	require.Equal(t, "3f2b1c9a-8d7e-4f6a-9b0c-1d2e3f4a5b6c", rs.AIGatewayNodes[0].ID)
	require.True(t, rs.SyncScope.ChildInScope(
		resources.ResourceTypeAIGateway,
		"support-gateway",
		resources.ResourceTypeAIGatewayNode,
	))
}

func TestLoaderValidatesAIGatewayNodeParentAndDuplicates(t *testing.T) {
	rootOnly := `
ai_gateway_nodes:
  - ref: support-node
    ai_gateway: missing-gateway
    id: 3f2b1c9a-8d7e-4f6a-9b0c-1d2e3f4a5b6c
    version: "3.11.0"
    hostname: support-node-1
    type: data-plane
`
	_, err := New().LoadFromSources([]Source{{Path: writeLoaderTestFile(t, rootOnly), Type: SourceTypeFile}}, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "references unknown ai_gateway")

	duplicates := `
ai_gateways:
  - ref: support-gateway
    display_name: Support Gateway
    nodes:
      - ref: support-node
        id: 3f2b1c9a-8d7e-4f6a-9b0c-1d2e3f4a5b6c
        version: "3.11.0"
        hostname: support-node-1
        type: data-plane
      - ref: support-node-2
        id: 3f2b1c9a-8d7e-4f6a-9b0c-1d2e3f4a5b6c
        version: "3.11.0"
        hostname: support-node-2
        type: data-plane
`
	_, err = New().LoadFromSources([]Source{{Path: writeLoaderTestFile(t, duplicates), Type: SourceTypeFile}}, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicate ai_gateway_node id")
}

func TestLoaderRejectsRootLevelEmptyAIGatewayNodes(t *testing.T) {
	input := `ai_gateway_nodes: []`

	_, err := New().LoadFromSources([]Source{{Path: writeLoaderTestFile(t, input), Type: SourceTypeFile}}, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "ai_gateway_nodes cannot be empty")
}

func TestLoaderAcceptsAIGatewayNodeDeferredExternalParentRef(t *testing.T) {
	input := `
ai_gateways:
  - ref: external-shared-gateway
    _external:
      selector:
        matchFields:
          display_name: Shared Gateway
ai_gateway_nodes:
  - ref: support-node
    ai_gateway: !ref external-shared-gateway#id
    id: 3f2b1c9a-8d7e-4f6a-9b0c-1d2e3f4a5b6c
    version: "3.11.0"
    hostname: support-node-1
    type: data-plane
`

	rs, err := New().LoadFromSources([]Source{{Path: writeLoaderTestFile(t, input), Type: SourceTypeFile}}, false)
	require.NoError(t, err)
	require.Len(t, rs.AIGatewayNodes, 1)
	require.Equal(t, tags.RefPlaceholderPrefix+"external-shared-gateway#id", rs.AIGatewayNodes[0].AIGateway)
	require.True(t, rs.SyncScope.ChildInScope(
		resources.ResourceTypeAIGateway,
		"external-shared-gateway",
		resources.ResourceTypeAIGatewayNode,
	))
}
