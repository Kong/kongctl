package loader

import (
	"strings"
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/stretchr/testify/require"
)

func TestPolicySourceNamespacesAndEmptyCollections(t *testing.T) {
	for _, collection := range []string{"", "    policies: []\n", `    policies:
      - ref: openai
        name: openai
        type: ai-sanitizer
        display_name: OpenAI
        config: {anonymize: [email]}
`} {
		input := "_defaults:\n  kongctl:\n    namespace: auth-team\n    protected: true\nai_gateways:\n" +
			"  - ref: shared\n    _external:\n      selector:\n        matchFields:\n          name: shared\n" + collection
		rs, err := New().LoadFromSources([]Source{{Path: writeLoaderTestFile(t, input), Type: SourceTypeFile}}, false)
		require.NoError(t, err)
		scope := rs.SyncScope.OwnedChildNamespaces(resources.ResourceTypeAIGatewayPolicy, "shared")
		if collection == "" {
			require.Empty(t, scope)
		} else {
			require.Contains(t, scope, "auth-team")
		}
		require.Empty(t, rs.SyncScope.GatewayNamespaces)
		if len(rs.AIGatewayPolicies) > 0 {
			require.True(t, rs.AIGatewayPolicies[0].Protected())
			require.Equal(t, "auth-team", resources.GetNamespace(rs.AIGatewayPolicies[0].Kongctl))
			input = strings.Replace(
				input,
				"      - ref: openai",
				"      - ref: openai\n        kongctl: {namespace: explicit, protected: false}",
				1,
			)
			rs, err = New().LoadFromSources([]Source{{Path: writeLoaderTestFile(t, input), Type: SourceTypeFile}}, false)
			require.NoError(t, err)
			require.False(t, rs.AIGatewayPolicies[0].Protected())
			require.Equal(t, "explicit", resources.GetNamespace(rs.AIGatewayPolicies[0].Kongctl))
		}
	}
}
