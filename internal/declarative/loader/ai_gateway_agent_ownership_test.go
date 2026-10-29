package loader

import (
	"strings"
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/stretchr/testify/require"
)

func TestAgentSourceNamespacesAndEmptyCollections(t *testing.T) {
	for _, tc := range []struct {
		name, collection string
		agents           int
		scoped           bool
	}{
		{name: "omitted"},
		{name: "empty", collection: "    agents: []\n", scoped: true},
		{name: "declared", agents: 1, scoped: true, collection: `    agents:
      - ref: booking-agent
        name: booking-agent
        type: a2a
        display_name: Booking Agent
        config: {url: "https://example.com"}
`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := "_defaults:\n  kongctl:\n    namespace: agents-team\n    protected: true\nai_gateways:\n" +
				"  - ref: shared\n    _external:\n      selector:\n        matchFields:\n          name: shared\n" + tc.collection
			rs, err := New().LoadFromSources([]Source{{Path: writeLoaderTestFile(t, input), Type: SourceTypeFile}}, false)
			require.NoError(t, err)
			require.Len(t, rs.AIGatewayAgents, tc.agents)
			require.Empty(t, rs.AIGatewayPolicies)
			scope := rs.SyncScope.OwnedChildNamespaces(resources.ResourceTypeAIGatewayAgent, "shared")
			if tc.scoped {
				require.Contains(t, scope, "agents-team")
			} else {
				require.Empty(t, scope)
			}
			require.Empty(t, rs.SyncScope.GatewayNamespaces)
			if tc.agents == 0 {
				return
			}
			require.True(t, rs.AIGatewayAgents[0].Protected())
			require.Equal(t, "agents-team", resources.GetNamespace(rs.AIGatewayAgents[0].Kongctl))
			input = strings.Replace(input, "      - ref: booking-agent",
				"      - ref: booking-agent\n        kongctl: {namespace: explicit, protected: false}", 1)
			rs, err = New().LoadFromSources([]Source{{Path: writeLoaderTestFile(t, input), Type: SourceTypeFile}}, false)
			require.NoError(t, err)
			require.Len(t, rs.AIGatewayAgents, 1)
			require.False(t, rs.AIGatewayAgents[0].Protected())
			require.Equal(t, "explicit", resources.GetNamespace(rs.AIGatewayAgents[0].Kongctl))
			scope = rs.SyncScope.OwnedChildNamespaces(resources.ResourceTypeAIGatewayAgent, "shared")
			require.Contains(t, scope, "explicit")
			require.NotContains(t, scope, "agents-team")
			require.Empty(t, rs.SyncScope.GatewayNamespaces)
		})
	}
}
