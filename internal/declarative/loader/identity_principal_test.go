package loader

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/stretchr/testify/require"
)

func TestIdentityPrincipalDeclarationsAndScopes(t *testing.T) {
	input := `_defaults:
  kongctl: {namespace: principal-team}
identity:
  directories:
    - ref: people
      _external: {id: 4078f559-063b-4f10-9098-9e6553b21d9b}
      principals:
        - ref: alice
          display_name: Shared name
          type: user
          metadata:
            department: engineering
            enabled: true
            count: 3
            large: 9007199254740993
            strings: [a, b]
            numbers: [1, 2]
          kongctl: {protected: true}
        - ref: unclassified
          display_name: Shared name
          type: null
  principals:
    - ref: agent
      directory: people
      type: agent
`
	rs, err := New().LoadFile(writeLoaderTestFile(t, input))
	require.NoError(t, err)
	require.Len(t, rs.Identity.Principals, 3)
	encoded, err := json.Marshal(rs)
	require.NoError(t, err)
	require.Contains(t, string(encoded), `"large":9007199254740993`)
	for _, p := range rs.Identity.Principals {
		require.Equal(t, "people", p.Directory)
		require.Equal(t, "principal-team", resources.GetNamespace(p.Kongctl))
		require.True(t, p.TypeDeclared())
	}
	require.Contains(
		t,
		rs.SyncScope.ChildNamespaces[resources.ResourceTypeIdentityPrincipal]["people"],
		"principal-team",
	)
	for _, replacement := range []struct{ old, next, message string }{
		{"type: user", "type: group", "type must be"},
		{"type: user", "active: false", "Q10"},
		{"type: user", "managed_by: {tool: other}", "managed_by"},
		{"type: user", "metadata: {nested: {a: b}}", "metadata"},
		{"directory: people", "directory: absent", "directory"},
	} {
		_, err := New().LoadFile(writeLoaderTestFile(t, strings.Replace(input, replacement.old, replacement.next, 1)))
		require.ErrorContains(t, err, replacement.message)
	}
}

func TestIdentityPrincipalOmittedAndEmptyScopes(t *testing.T) {
	for _, collection := range []string{"", "      principals: []\n"} {
		input := `_defaults:
  kongctl: {namespace: principal-team}
identity:
  directories:
    - ref: people
      _external: {id: 4078f559-063b-4f10-9098-9e6553b21d9b}
` + collection
		rs, err := New().LoadFile(writeLoaderTestFile(t, input))
		require.NoError(t, err)
		_, scoped := rs.SyncScope.ChildNamespaces[resources.ResourceTypeIdentityPrincipal]["people"]["principal-team"]
		require.Equal(t, collection != "", scoped)
	}
}
