package loader

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/stretchr/testify/require"
)

func TestIdentityGroupDeclarationsAndScopes(t *testing.T) {
	input := `_defaults:
  kongctl: {namespace: group-team}
identity:
  directories:
    - ref: people
      _external: {id: 4078f559-063b-4f10-9098-9e6553b21d9b}
      groups:
        - ref: fulfillment
          name: Fulfillment
          type: group
          description: Operations
          metadata: {large: 9007199254740994, count: 3, enabled: true, roles: [reader, writer]}
          labels: {team: operations}
          kongctl: {protected: true}
  groups:
    - ref: support
      directory: people
      name: Support
`
	rs, err := New().LoadFile(writeLoaderTestFile(t, input))
	require.NoError(t, err)
	require.Len(t, rs.Identity.Groups, 2)
	data, err := json.Marshal(rs)
	require.NoError(t, err)
	require.Contains(t, string(data), `"large":9007199254740994`)
	for _, group := range rs.Identity.Groups {
		require.Equal(t, "people", group.Directory)
		require.Equal(t, "group-team", resources.GetNamespace(group.Kongctl))
	}
	require.Contains(t, rs.SyncScope.ChildNamespaces[resources.ResourceTypeIdentityGroup]["people"], "group-team")
	for _, name := range []string{"A", "Support-Operations", "Support--Operations", "1-2"} {
		_, err := New().LoadFile(writeLoaderTestFile(t, strings.Replace(input, "name: Fulfillment", "name: "+name, 1)))
		require.NoError(t, err, "SDK 0.83 permits internal hyphens")
	}
	for _, replacement := range []struct{ old, next, message string }{
		{"name: Fulfillment", "name: -leading-hyphen", "ASCII"},
		{"name: Fulfillment", "name: trailing-hyphen-", "ASCII"},
		{"name: Fulfillment", "name: has_underscore", "ASCII"},
		{"name: Fulfillment", "name: has space", "ASCII"},
		{"name: Fulfillment", "name: ''", "name is required"},
		{"type: group", "type: team", "Q32"},
		{"type: group", "managed_by: {tool: other}", "managed_by"},
		{"large: 9007199254740994", "large: 9007199254740993", "Q31"},
		{"directory: people", "directory: absent", "declared directory"},
	} {
		_, err := New().LoadFile(writeLoaderTestFile(t, strings.Replace(input, replacement.old, replacement.next, 1)))
		require.ErrorContains(t, err, replacement.message)
	}
}

func TestIdentityGroupOmittedEmptyAndExternalOnlyScopes(t *testing.T) {
	for _, collection := range []string{
		"", "      groups: []\n",
		"      groups:\n        - ref: external\n          _external: {id: 5078f559-063b-4f10-9098-9e6553b21d9b}\n",
	} {
		input := `_defaults:
  kongctl: {namespace: group-team}
identity:
  directories:
    - ref: people
      _external: {id: 4078f559-063b-4f10-9098-9e6553b21d9b}
` + collection
		rs, err := New().LoadFile(writeLoaderTestFile(t, input))
		require.NoError(t, err)
		_, scoped := rs.SyncScope.ChildNamespaces[resources.ResourceTypeIdentityGroup]["people"]["group-team"]
		require.Equal(t, collection == "      groups: []\n", scoped)
	}
}
