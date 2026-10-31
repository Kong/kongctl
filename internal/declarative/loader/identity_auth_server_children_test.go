package loader

import (
	"strings"
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/stretchr/testify/require"
)

const authChildrenManifest = `_defaults:
  kongctl: {namespace: auth-team}
identity:
  auth_servers:
    - ref: issuer
      name: issuer
      audience: https://api.example.test
      scopes:
        - ref: read
          name: read:data
          description: Read data
          default: false
          include_in_metadata: true
          enabled: false
      claims:
        - ref: department
          name: department
          value: '${ uuidv4 }'
          include_in_token: false
          include_in_all_scopes: false
          include_in_scopes: [!ref read#id]
          enabled: true
`

func TestAuthServerChildrenHaveParentAuthorityAndTypedReferences(t *testing.T) {
	rs, err := New().LoadFile(writeLoaderTestFile(t, authChildrenManifest))
	require.NoError(t, err)
	require.Len(t, rs.Identity.Scopes, 1)
	require.Len(t, rs.Identity.Claims, 1)
	require.Equal(t, "issuer", rs.Identity.Scopes[0].AuthServer)
	require.Equal(t, "${ uuidv4 }", rs.Identity.Claims[0].Value)
	require.False(t, *rs.Identity.Scopes[0].Enabled)
	require.True(t, rs.SyncScope.ChildInScope(resources.ResourceTypeIdentityAuthServer, "issuer",
		resources.ResourceTypeIdentityAuthServerScope))
	for _, tc := range []struct{ old, next, message string }{
		{"          name: read:data\n", "", "name is required"},
		{"          value: '${ uuidv4 }'\n", "", "value is required"},
		{"          enabled: false", "          kongctl: {protected: true}", "kongctl"},
		{"!ref read#id", "!ref issuer#id", "auth-server scope"},
		{"!ref read#id", "relative-name", "scope UUIDs"},
	} {
		_, err := New().LoadFile(writeLoaderTestFile(t, strings.Replace(authChildrenManifest, tc.old, tc.next, 1)))
		require.ErrorContains(t, err, tc.message)
	}
}

func TestAuthServerChildCollectionsOmittedAndExplicitlyEmpty(t *testing.T) {
	for _, children := range []string{"", "      scopes: []\n      claims: []\n"} {
		input := "identity:\n  auth_servers:\n    - ref: issuer\n      name: issuer\n      audience: api\n" + children
		rs, err := New().LoadFile(writeLoaderTestFile(t, input))
		require.NoError(t, err)
		require.Equal(t, children != "", rs.SyncScope.ChildInScope(resources.ResourceTypeIdentityAuthServer,
			"issuer", resources.ResourceTypeIdentityAuthServerScope))
	}
}

func TestClaimRejectsScopeReferenceFromAnotherServer(t *testing.T) {
	input := `identity:
  auth_servers:
    - ref: first
      name: first
      audience: api
      scopes:
        - {ref: read, name: read:data}
    - ref: second
      name: second
      audience: api
      claims:
        - ref: claim
          name: claim
          value: value
          include_in_scopes: [!ref read#id]
`
	_, err := New().LoadFile(writeLoaderTestFile(t, input))
	require.ErrorContains(t, err, "same auth server")
}
