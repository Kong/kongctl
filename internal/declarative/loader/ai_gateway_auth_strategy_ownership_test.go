package loader

import (
	"strings"
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/stretchr/testify/require"
)

func TestAuthStrategySourceNamespacesAndEmptyCollections(t *testing.T) {
	for _, collection := range []string{"", "    auth_strategies: []\n", `    auth_strategies:
      - ref: openai
        name: openai
        type: key-auth
        display_name: OpenAI
        config: {key_names: [apikey]}
`} {
		input := "_defaults:\n  kongctl:\n    namespace: auth-team\n    protected: true\nai_gateways:\n" +
			"  - ref: shared\n    _external:\n      selector:\n        matchFields:\n          name: shared\n" + collection
		rs, err := New().LoadFromSources([]Source{{Path: writeLoaderTestFile(t, input), Type: SourceTypeFile}}, false)
		require.NoError(t, err)
		scope := rs.SyncScope.OwnedChildNamespaces(resources.ResourceTypeAIGatewayAuthStrategy, "shared")
		if collection == "" {
			require.Empty(t, scope)
		} else {
			require.Contains(t, scope, "auth-team")
		}
		require.Empty(t, rs.SyncScope.GatewayNamespaces)
		if len(rs.AIGatewayAuthStrategies) > 0 {
			require.True(t, rs.AIGatewayAuthStrategies[0].Protected())
			require.Equal(t, "auth-team", resources.GetNamespace(rs.AIGatewayAuthStrategies[0].Kongctl))
			input = strings.Replace(
				input,
				"      - ref: openai",
				"      - ref: openai\n        kongctl: {namespace: explicit, protected: false}",
				1,
			)
			rs, err = New().LoadFromSources([]Source{{Path: writeLoaderTestFile(t, input), Type: SourceTypeFile}}, false)
			require.NoError(t, err)
			require.False(t, rs.AIGatewayAuthStrategies[0].Protected())
			require.Equal(t, "explicit", resources.GetNamespace(rs.AIGatewayAuthStrategies[0].Kongctl))
		}
	}
}
