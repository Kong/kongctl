package loader

import (
	"strings"
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/stretchr/testify/require"
)

func TestCustomPolicySourceNamespacesAndEmptyCollections(t *testing.T) {
	for _, tc := range []struct {
		name, collection string
		definitions      int
		scoped           bool
	}{
		{name: "omitted"},
		{name: "empty", collection: "    custom_policies: []\n", scoped: true},
		{name: "declared", definitions: 1, scoped: true, collection: `    custom_policies:
      - ref: custom-plugin
        name: custom-plugin
        type: installed
        display_name: Custom plugin
        schema: "return {}"
`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := "_defaults:\n  kongctl:\n    namespace: definitions-team\n    protected: true\nai_gateways:\n" +
				"  - ref: shared\n    _external:\n      selector:\n        matchFields:\n          name: shared\n" + tc.collection
			rs, err := New().LoadFromSources([]Source{{Path: writeLoaderTestFile(t, input), Type: SourceTypeFile}}, false)
			require.NoError(t, err)
			require.Len(t, rs.AIGatewayCustomPolicies, tc.definitions)
			require.Empty(t, rs.AIGatewayPolicies)
			scope := rs.SyncScope.OwnedChildNamespaces(resources.ResourceTypeAIGatewayCustomPolicy, "shared")
			if tc.scoped {
				require.Contains(t, scope, "definitions-team")
			} else {
				require.Empty(t, scope)
			}
			require.Empty(t, rs.SyncScope.GatewayNamespaces)
			if tc.definitions == 0 {
				return
			}
			require.True(t, rs.AIGatewayCustomPolicies[0].Protected())
			require.Equal(t, "definitions-team", resources.GetNamespace(rs.AIGatewayCustomPolicies[0].Kongctl))
			input = strings.Replace(input, "      - ref: custom-plugin",
				"      - ref: custom-plugin\n        kongctl: {namespace: explicit, protected: false}", 1)
			rs, err = New().LoadFromSources([]Source{{Path: writeLoaderTestFile(t, input), Type: SourceTypeFile}}, false)
			require.NoError(t, err)
			require.Len(t, rs.AIGatewayCustomPolicies, 1)
			require.False(t, rs.AIGatewayCustomPolicies[0].Protected())
			require.Equal(t, "explicit", resources.GetNamespace(rs.AIGatewayCustomPolicies[0].Kongctl))
			scope = rs.SyncScope.OwnedChildNamespaces(resources.ResourceTypeAIGatewayCustomPolicy, "shared")
			require.Contains(t, scope, "explicit")
			require.NotContains(t, scope, "definitions-team")
			require.Empty(t, rs.SyncScope.GatewayNamespaces)
		})
	}
}
