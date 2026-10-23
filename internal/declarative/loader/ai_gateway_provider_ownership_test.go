package loader

import (
	"strings"
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/stretchr/testify/require"
)

func TestProviderSourceNamespacesAndEmptyCollections(t *testing.T) {
	for _, collection := range []string{"", "    model_providers: []\n", `    model_providers:
      - ref: openai
        name: openai
        type: openai
        display_name: OpenAI
        config: {auth: {type: basic}}
`} {
		input := "_defaults:\n  kongctl:\n    namespace: providers-team\n    protected: true\nai_gateways:\n" +
			"  - ref: shared\n    _external:\n      selector:\n        matchFields:\n          name: shared\n" + collection
		rs, err := New().LoadFromSources([]Source{{Path: writeLoaderTestFile(t, input), Type: SourceTypeFile}}, false)
		require.NoError(t, err)
		scope := rs.SyncScope.OwnedChildNamespaces(resources.ResourceTypeAIGatewayProvider, "shared")
		if collection == "" {
			require.Empty(t, scope)
		} else {
			require.Contains(t, scope, "providers-team")
		}
		require.Empty(t, rs.SyncScope.GatewayNamespaces)
		if len(rs.AIGatewayProviders) > 0 {
			require.True(t, rs.AIGatewayProviders[0].Protected())
			require.Equal(t, "providers-team", resources.GetNamespace(rs.AIGatewayProviders[0].Kongctl))
			input = strings.Replace(
				input,
				"      - ref: openai",
				"      - ref: openai\n        kongctl: {namespace: explicit, protected: false}",
				1,
			)
			rs, err = New().LoadFromSources([]Source{{Path: writeLoaderTestFile(t, input), Type: SourceTypeFile}}, false)
			require.NoError(t, err)
			require.False(t, rs.AIGatewayProviders[0].Protected())
			require.Equal(t, "explicit", resources.GetNamespace(rs.AIGatewayProviders[0].Kongctl))
		}
	}
}
