package loader

import (
	"strings"
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/stretchr/testify/require"
)

func TestIdentityDirectoryDeclarations(t *testing.T) {
	input := `
_defaults:
  kongctl:
    namespace: identity-team
control_planes:
  - ref: gateway
    name: Gateway
identity:
  directories:
    - ref: people
      name: people
      description: Directory description
      allowed_control_planes: [!ref gateway#id]
      allow_all_control_planes: false
      ttl_secs: 300
      negative_ttl_secs: 86400
      vault_enabled: false
      labels: {owner: identity-team}
      kongctl: {protected: true}
    - ref: existing-default
      _external:
        selector:
          matchFields: {name: default}
`
	rs, err := New().LoadFile(writeLoaderTestFile(t, input))
	require.NoError(t, err)
	require.Len(t, rs.Identity.Directories, 2)
	directory := rs.Identity.Directories[0]
	require.Equal(t, "identity-team", resources.GetNamespace(directory.Kongctl))
	require.True(t, directory.Protected())
	require.False(t, *directory.AllowAllControlPlanes)
	require.False(t, *directory.VaultEnabled)
	require.Equal(t, []string{"__REF__:gateway#id"}, directory.AllowedControlPlanes)
	require.True(t, rs.SyncScope.RootInScope(resources.ResourceTypeIdentityDirectory))
	require.True(t, rs.Identity.Directories[1].IsExternal())
	_, err = New().LoadFile(writeLoaderTestFile(t, strings.Replace(input,
		"[!ref gateway#id]", "[!lookup {name: Existing Gateway}]", 1)))
	require.NoError(t, err, "control-plane relationship fields support the standard lookup tag")

	for _, test := range []struct{ name, old, next, message string }{
		{"explicit name", "      name: people\n", "", "name is required"},
		{"TTL minimum", "ttl_secs: 300", "ttl_secs: 299", "between 300 and 86400"},
		{"name syntax", "name: people", "name: People", "lowercase"},
		{"control plane IDs", "[!ref gateway#id]", "[gateway]", "must be UUIDs"},
		{"control plane property", "gateway#id", "gateway#name", "must be UUIDs"},
		{"generated metadata", "      labels: {owner: identity-team}", "      managed_by: {tool: other}", "managed_by"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := New().LoadFile(writeLoaderTestFile(t, strings.Replace(input, test.old, test.next, 1)))
			require.ErrorContains(t, err, test.message)
		})
	}
}

func TestIdentityDirectoryEmptyCollectionsAndSyncScope(t *testing.T) {
	for _, input := range []string{"identity: {directories: []}", "identity: {}"} {
		rs, err := New().LoadFile(writeLoaderTestFile(t, input))
		require.NoError(t, err)
		require.Equal(t, strings.Contains(input, "directories"),
			rs.SyncScope.RootInScope(resources.ResourceTypeIdentityDirectory))
	}
	rs, err := New().LoadFile(writeLoaderTestFile(t, `identity:
  directories:
    - ref: explicit-empty
      name: explicit-empty
      labels: {}
      allowed_control_planes: []
`))
	require.NoError(t, err)
	directory := rs.Identity.Directories[0]
	require.NotNil(t, directory.Labels)
	require.NotNil(t, directory.AllowedControlPlanes)
	require.Nil(t, directory.AllowAllControlPlanes, "omitted settings preserve current state during update")
	require.Nil(t, directory.TTLSecs)
}
