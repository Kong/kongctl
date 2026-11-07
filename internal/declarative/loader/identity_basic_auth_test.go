package loader

import (
	"strings"
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/stretchr/testify/require"
)

func TestBasicAuthDeclarationsAndIndependentScope(t *testing.T) {
	input := `_defaults:
  kongctl: {namespace: credentials}
identity:
  directories:
    - ref: people
      _external: {selector: {matchFields: {name: default}}}
      principals:
        - ref: worker
          _external: {id: 4078f559-063b-4f10-9098-9e6553b21d9b}
          basic_auths:
            - ref: login
              username: worker@example.test
              kongctl: {protected: true}
  basic_auths:
    - ref: secondary
      principal: worker
      username: other@example.test
`
	rs, err := New().LoadFile(writeLoaderTestFile(t, input))
	require.NoError(t, err)
	require.Len(t, rs.Identity.BasicAuths, 2)
	for _, b := range rs.Identity.BasicAuths {
		require.Equal(t, "worker", b.Principal)
		require.Equal(t, "credentials", resources.GetNamespace(b.Kongctl))
	}
	require.Contains(t, rs.SyncScope.ChildNamespaces[resources.ResourceTypeIdentityBasicAuth]["worker"], "credentials")
	for _, replacement := range []struct{ old, next, message string }{
		{"username: worker@example.test", "username: ''", "username is required"},
		{"username: other@example.test", "username: worker@example.test", "duplicate basic-auth username"},
		{"principal: worker", "principal: absent", "declared principal"},
		{"username: worker@example.test", "managed_by: {tool: other}", "managed_by"},
		{"username: worker@example.test", "passwords: []", "passwords"},
	} {
		_, err := New().LoadFile(writeLoaderTestFile(t, strings.Replace(input, replacement.old, replacement.next, 1)))
		require.ErrorContains(t, err, replacement.message)
	}
}

func TestBasicAuthOmittedEmptyAndExternalOnlyScopes(t *testing.T) {
	for _, collection := range []string{
		"", "      basic_auths: []\n",
		"      basic_auths:\n        - ref: existing\n          _external: {selector: {matchFields: {username: worker}}}\n",
	} {
		input := `_defaults:
  kongctl: {namespace: credentials}
identity:
  directories:
    - ref: people
      _external: {selector: {matchFields: {name: default}}}
  principals:
    - ref: worker
      directory: people
      _external: {id: 4078f559-063b-4f10-9098-9e6553b21d9b}
` + collection
		rs, err := New().LoadFile(writeLoaderTestFile(t, input))
		require.NoError(t, err)
		_, scoped := rs.SyncScope.ChildNamespaces[resources.ResourceTypeIdentityBasicAuth]["worker"]["credentials"]
		require.Equal(t, collection == "      basic_auths: []\n", scoped)
	}
}
