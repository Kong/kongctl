package loader

import (
	"fmt"
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/stretchr/testify/require"
)

func TestRemainingChildSourceScopeDoesNotBroadenGatewayPruning(t *testing.T) {
	for _, tc := range []struct {
		kind       resources.ResourceType
		collection string
	}{
		{resources.ResourceTypeAIGatewayMCPServer, "mcp_servers"},
		{resources.ResourceTypeAIGatewayVault, "vaults"},
		{resources.ResourceTypeAIGatewayConsumer, "consumers"},
		{resources.ResourceTypeAIGatewayConsumerGroup, "consumer_groups"},
		{resources.ResourceTypeAIGatewayCACertificate, "ca_certificates"},
		{resources.ResourceTypeAIGatewaySNI, "snis"},
	} {
		t.Run(tc.collection, func(t *testing.T) {
			for _, explicit := range []bool{false, true} {
				input := `_defaults:
  kongctl: {namespace: team-a}
ai_gateways:
  - ref: shared
    _external: {id: gateway-id}
`
				if explicit {
					input += fmt.Sprintf("    %s: []\n", tc.collection)
				}
				rs, err := New().LoadFromSources([]Source{{Path: writeLoaderTestFile(t, input), Type: SourceTypeFile}}, false)
				require.NoError(t, err)
				namespaces := rs.SyncScope.OwnedChildNamespaces(tc.kind, "shared")
				if explicit {
					require.Contains(t, namespaces, "team-a")
				} else {
					require.Empty(t, namespaces)
				}
				require.Empty(t, rs.SyncScope.GatewayNamespaces)
			}
		})
	}
}
