package loader

import (
	"strings"
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/stretchr/testify/require"
)

func TestIdentityAuthServerDeclarations(t *testing.T) {
	input := `_defaults:
  kongctl: {namespace: auth-team}
identity:
  auth_servers:
    - ref: issuer
      name: issuer
      audience: https://api.example.test
      description: Example
      signing_algorithm: PS256
      trusted_origins: [https://client.example.test]
      dcr: {default_access_token_duration: 600}
      labels: {owner: team}
      kongctl: {protected: true}
    - ref: existing
      _external: {selector: {matchFields: {name: shared-issuer}}}
`
	rs, err := New().LoadFile(writeLoaderTestFile(t, input))
	require.NoError(t, err)
	require.Len(t, rs.Identity.AuthServers, 2)
	require.Equal(t, "auth-team", resources.GetNamespace(rs.Identity.AuthServers[0].Kongctl))
	require.True(t, rs.Identity.AuthServers[0].Protected())
	require.True(t, rs.SyncScope.RootInScope(resources.ResourceTypeIdentityAuthServer))
	for _, tc := range []struct{ old, next, message string }{
		{"      name: issuer\n", "", "name is required"},
		{"      audience: https://api.example.test\n", "", "audience is required"},
		{"PS256", "unsupported", "signing_algorithm"},
		{"600", "59", "between 60"},
		{"[https://client.example.test]", "[relative]", "absolute URIs"},
	} {
		_, err := New().LoadFile(writeLoaderTestFile(t, strings.Replace(input, tc.old, tc.next, 1)))
		require.ErrorContains(t, err, tc.message)
	}
	for _, input := range []string{"identity: {auth_servers: []}", "identity: {}"} {
		rs, err := New().LoadFile(writeLoaderTestFile(t, input))
		require.NoError(t, err)
		require.Equal(t, strings.Contains(input, "auth_servers"),
			rs.SyncScope.RootInScope(resources.ResourceTypeIdentityAuthServer))
	}
}
