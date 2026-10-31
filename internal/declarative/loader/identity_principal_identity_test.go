package loader

import (
	"strings"
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/stretchr/testify/require"
)

func TestPrincipalIdentityDeclarationsAndOwnershipScopes(t *testing.T) {
	const input = `_defaults:
  kongctl: {namespace: identity-team}
identity:
  directories:
    - ref: people
      name: people
      allow_all_control_planes: true
      kongctl: {namespace: directory-team}
      principals:
        - ref: person
          display_name: Shared name
          kongctl: {namespace: principal-team}
          identities:
            - ref: employee
              type: custom
              key: employee
              value: '123'
              kongctl: {protected: true}
            - ref: existing-link
              _external: {id: 731559d3-39bd-48c7-b33c-eb192847eb22}
        - ref: existing-person
          _external: {id: 731559d3-39bd-48c7-b33c-eb192847eb21}
          identities: []
  principals:
    - ref: grouped-person
      directory: people
      _external: {id: 731559d3-39bd-48c7-b33c-eb192847eb23}
      identities:
        - ref: employee-oidc
          type: oidc
          issuer: https://issuer.example.test
          claim: {name: sub, value: '123'}
  identities:
    - ref: client-link
      principal: person
      type: auth_server_client
      auth_server_id: 731559d3-39bd-48c7-b33c-eb192847eb24
      client_id: opaque-external-client
    - ref: consumer-link
      principal: person
      type: control_plane_consumer
      control_plane_id: 731559d3-39bd-48c7-b33c-eb192847eb25
      consumer_id: 731559d3-39bd-48c7-b33c-eb192847eb26
`
	rs, err := New().LoadFile(writeLoaderTestFile(t, input))
	require.NoError(t, err)
	require.Len(t, rs.Identity.Identities, 5)
	require.Len(t, rs.Identity.Principals, 3)
	require.Equal(t, map[string]struct{}{"directory-team": {}}, rs.SyncScope.IdentityDirectoryNamespaces)
	require.Equal(
		t,
		map[string]struct{}{"principal-team": {}},
		rs.SyncScope.ChildNamespaces[resources.ResourceTypeIdentityPrincipal]["people"],
	)
	for _, parent := range []string{"person", "existing-person", "grouped-person"} {
		require.Contains(
			t,
			rs.SyncScope.ChildNamespaces[resources.ResourceTypeIdentityPrincipalIdentity][parent],
			"identity-team",
		)
	}
	for _, identity := range rs.Identity.Identities {
		if identity.IsExternal() {
			require.Nil(t, identity.Kongctl)
			continue
		}
		require.Equal(t, "identity-team", resources.GetNamespace(identity.Kongctl))
	}
	for _, replacement := range []struct{ old, next, error string }{
		{"type: custom\n              key: employee\n              value: '123'", "type: oidc", "requires explicit issuer"},
		{"type: custom", "type: custom\n              issuer: invalid", "issuer"},
		{"principal: person", "principal: absent", "declared principal"},
		{"client_id: opaque-external-client", "client_id: !ref people#id", "matching resource#id"},
		{
			"consumer_id: 731559d3-39bd-48c7-b33c-eb192847eb26",
			"consumer_id: 731559d3-39bd-48c7-b33c-eb192847eb26\n      workspace_id: workspace-supplied", "Q25",
		},
		{"value: '123'", "managed_by: {tool: other}", "managed_by"},
		{
			"_external: {id: 731559d3-39bd-48c7-b33c-eb192847eb21}",
			"_external: {selector: {matchFields: {display_name: Shared}}}", "explicit UUID",
		},
	} {
		_, err := New().LoadFile(writeLoaderTestFile(t, strings.Replace(input, replacement.old, replacement.next, 1)))
		require.ErrorContains(t, err, replacement.error)
	}
}
