package loader

import (
	"strings"
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/stretchr/testify/require"
)

const identityClientManifest = `_defaults:
  kongctl: {namespace: clients}
identity:
  auth_servers:
    - ref: issuer
      name: issuer
      audience: https://api.example.test
      kongctl: {namespace: platform}
      scopes:
        - {ref: read, name: read:data}
      clients:
        - ref: machine
          name: machine
          grant_types: [client_credentials]
          response_types: []
          redirect_uris: [https://client.example.test/callback]
          login_uri: https://client.example.test/login
          access_token_duration: 600
          id_token_duration: 900
          allow_all_scopes: false
          allow_scopes: [!ref read#id]
          token_endpoint_auth_method: client_secret_basic
          labels: {team: platform}
          kongctl: {protected: true}
`

func TestIdentityClientsHaveExplicitFieldsAndIndependentLabelOwnership(t *testing.T) {
	rs, err := New().LoadFile(writeLoaderTestFile(t, identityClientManifest))
	require.NoError(t, err)
	require.Len(t, rs.Identity.Clients, 1)
	c := rs.Identity.Clients[0]
	require.Equal(t, "issuer", c.AuthServer)
	require.Equal(t, "clients", resources.GetNamespace(c.Kongctl))
	require.True(t, c.Protected())
	require.False(t, *c.AllowAllScopes)
	require.NotNil(t, c.ResponseTypes)
	require.Empty(t, c.ResponseTypes)
	require.True(t, resources.HasIndependentChildOwnership(c.GetType()))
	require.False(t, resources.UsesManagedByOwnership(c.GetType()))
	require.False(t, resources.SupportsManagedBy(c.GetType()))
	require.Contains(t, rs.SyncScope.ChildNamespaces[c.GetType()]["issuer"], "clients")
	for _, test := range []struct{ old, next, message string }{
		{"          name: machine\n", "", "name is required"},
		{"          grant_types: [client_credentials]\n", "", "grant_types must be explicitly"},
		{"          response_types: []\n", "", "response_types must be explicitly"},
		{"          grant_types: [client_credentials]", "          grant_types: [unknown]", "grant_types"},
		{"          access_token_duration: 600", "          access_token_duration: 1", "token durations"},
		{"!ref read#id", "!ref issuer#id", "auth-server scope"},
		{"!ref read#id", "read:data", "scope UUIDs"},
		{"          labels: {team: platform}", "          client_secret: private", "client_secret"},
		{"https://client.example.test/callback", "http://client.example.test/callback", "HTTPS"},
	} {
		t.Run(test.message, func(t *testing.T) {
			_, err := New().LoadFile(writeLoaderTestFile(t, strings.Replace(identityClientManifest, test.old, test.next, 1)))
			require.ErrorContains(t, err, test.message, "input change %q", test.old)
		})
	}
}

func TestIdentityClientsCanUseAnExternalParentAndExplicitEmptyScope(t *testing.T) {
	input := `_defaults:
  kongctl: {namespace: app-team}
identity:
  auth_servers:
    - ref: shared
      _external: {selector: {matchFields: {name: shared}}}
      clients:
        - ref: machine
          name: machine
          grant_types: []
          response_types: []
`
	rs, err := New().LoadFile(writeLoaderTestFile(t, input))
	require.NoError(t, err)
	require.True(t, rs.Identity.AuthServers[0].IsExternal())
	require.Equal(t, "app-team", resources.GetNamespace(rs.Identity.Clients[0].Kongctl))
	input = strings.Replace(input, `      clients:
        - ref: machine
          name: machine
          grant_types: []
          response_types: []`, "      clients: []", 1)
	rs, err = New().LoadFile(writeLoaderTestFile(t, input))
	require.NoError(t, err)
	require.Contains(
		t,
		rs.SyncScope.ChildNamespaces[resources.ResourceTypeIdentityAuthServerClient]["shared"],
		"app-team",
	)
}
