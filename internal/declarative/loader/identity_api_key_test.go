package loader

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/stretchr/testify/require"
)

const generatedKeyManifest = `_defaults:
  kongctl: {namespace: directory-team}
identity:
  directories:
    - ref: people
      _external: {id: 4078f559-063b-4f10-9098-9e6553b21d9b}
      principals:
        - ref: worker
          type: user
          kongctl: {namespace: principal-team}
          api_keys:
            - ref: worker-primary
              type: v1
              name: Worker primary
              expires_at: null
      groups:
        - ref: operators
          name: Operators
          kongctl: {namespace: group-team}
          api_keys:
            - ref: operators-primary
              type: v1
              expires_at: 2030-01-01T00:00:00Z
  principal_api_keys:
    - ref: worker-secondary
      principal: worker
      type: v1
  group_api_keys:
    - ref: operators-secondary
      group: operators
      type: v1
`

func TestGeneratedKeysLoadInheritedAuthorityAndNullableExpiration(t *testing.T) {
	rs, err := New().LoadFile(writeLoaderTestFile(t, generatedKeyManifest))
	require.NoError(t, err)
	require.Len(t, rs.Identity.PrincipalAPIKeys, 2)
	require.Len(t, rs.Identity.GroupAPIKeys, 2)
	require.Len(
		t,
		resources.GetResourcesByNamespace[resources.IdentityPrincipalAPIKeyResource](rs, "principal-team"),
		2,
	)
	require.Len(t, resources.GetResourcesByNamespace[resources.IdentityGroupAPIKeyResource](rs, "group-team"), 2)
	require.Empty(t, resources.GetResourcesByNamespace[resources.IdentityPrincipalAPIKeyResource](rs, "directory-team"))
	for _, key := range rs.Identity.PrincipalAPIKeys {
		data, err := json.Marshal(key)
		require.NoError(t, err)
		var roundTrip resources.IdentityPrincipalAPIKeyResource
		require.NoError(t, json.Unmarshal(data, &roundTrip))
		require.Equal(t, key.ExpirationDeclared(), roundTrip.ExpirationDeclared())
		if key.Ref == "worker-primary" {
			require.True(t, roundTrip.ExpirationDeclared())
			require.Nil(t, roundTrip.ExpiresAt)
			require.JSONEq(
				t,
				`{"ref":"worker-primary","principal":"worker","type":"v1","name":"Worker primary","expires_at":null}`,
				string(data),
			)
		} else {
			require.False(t, roundTrip.ExpirationDeclared())
			require.NotContains(t, string(data), "expires_at")
		}
	}
	for _, pair := range []struct {
		parent, child resources.ResourceType
		ref           string
	}{
		{resources.ResourceTypeIdentityPrincipal, resources.ResourceTypeIdentityPrincipalAPIKey, "worker"},
		{resources.ResourceTypeIdentityGroup, resources.ResourceTypeIdentityGroupAPIKey, "operators"},
	} {
		require.True(t, rs.SyncScope.ChildInScope(pair.parent, pair.ref, pair.child))
	}
}

func TestGeneratedKeysRejectUnsupportedDeclarations(t *testing.T) {
	for _, change := range []struct{ name, old, next string }{
		{"missing-type", "              type: v1\n", ""},
		{"imported-type", "              type: v1", "              type: imported"},
		{"material", "name: Worker primary", "secret: private-value"},
		{"ownership", "name: Worker primary", "managed_by: {tool: kongctl}"},
		{"independent-namespace", "name: Worker primary", "kongctl: {namespace: other}"},
		{
			"external-group", "          name: Operators\n          kongctl: {namespace: group-team}",
			"          _external: {id: 6078f559-063b-4f10-9098-9e6553b21d9b}",
		},
	} {
		t.Run(change.name, func(t *testing.T) {
			candidate := strings.Replace(generatedKeyManifest, change.old, change.next, 1)
			require.NotEqual(t, generatedKeyManifest, candidate)
			_, err := New().LoadFile(writeLoaderTestFile(t, candidate))
			require.Error(t, err)
		})
	}
}

func TestGeneratedKeysUnderExternalPrincipalHaveNamespaceOnlyScope(t *testing.T) {
	const input = `_defaults:
  kongctl: {namespace: workforce}
identity:
  directories:
    - ref: directory
      _external: {selector: {matchFields: {name: default}}}
      principals:
        - ref: employee
          _external: {id: 5078f559-063b-4f10-9098-9e6553b21d9b}
          api_keys:
            - ref: primary
              type: v1
              kongctl: {namespace: application}
`
	rs, err := New().LoadFile(writeLoaderTestFile(t, input))
	require.NoError(t, err)
	require.Len(t, resources.GetResourcesByNamespace[resources.IdentityPrincipalAPIKeyResource](rs, "application"), 1)
	require.Empty(t, resources.GetResourcesByNamespace[resources.IdentityPrincipalAPIKeyResource](rs, "workforce"))
	scopes := rs.SyncScope.ChildNamespaces[resources.ResourceTypeIdentityPrincipalAPIKey]["employee"]
	require.Contains(t, scopes, "application")
	for _, protected := range []string{"true", "false"} {
		_, err := New().LoadFile(writeLoaderTestFile(t,
			strings.Replace(input, "namespace: application", "namespace: application, protected: "+protected, 1)))
		require.ErrorContains(t, err, "fixed creation identity markers")
	}
	collection := `api_keys:
            - ref: primary
              type: v1
              kongctl: {namespace: application}`
	empty := strings.Replace(input, collection, "api_keys: []", 1)
	rs, err = New().LoadFile(writeLoaderTestFile(t, empty))
	require.NoError(t, err)
	scopes = rs.SyncScope.ChildNamespaces[resources.ResourceTypeIdentityPrincipalAPIKey]["employee"]
	require.Contains(t, scopes, "workforce")
	omitted := strings.Replace(empty, "          api_keys: []\n", "", 1)
	rs, err = New().LoadFile(writeLoaderTestFile(t, omitted))
	require.NoError(t, err)
	require.Empty(t, rs.SyncScope.ChildNamespaces[resources.ResourceTypeIdentityPrincipalAPIKey]["employee"])
}

func TestGeneratedKeyCollectionsRequireExplicitSyncScope(t *testing.T) {
	for _, empty := range []bool{false, true} {
		input := `identity:
  directories:
    - ref: people
      _external: {id: 4078f559-063b-4f10-9098-9e6553b21d9b}
      principals:
        - ref: worker
          type: user
      groups:
        - ref: operators
          name: Operators
`
		if empty {
			input = strings.Replace(input, "          type: user", "          type: user\n          api_keys: []", 1)
			input = strings.Replace(
				input,
				"          name: Operators",
				"          name: Operators\n          api_keys: []",
				1,
			)
		}
		rs, err := New().LoadFile(writeLoaderTestFile(t, input))
		require.NoError(t, err)
		require.Equal(
			t,
			empty,
			rs.SyncScope.ChildInScope(
				resources.ResourceTypeIdentityPrincipal,
				"worker",
				resources.ResourceTypeIdentityPrincipalAPIKey,
			),
		)
		require.Equal(
			t,
			empty,
			rs.SyncScope.ChildInScope(
				resources.ResourceTypeIdentityGroup,
				"operators",
				resources.ResourceTypeIdentityGroupAPIKey,
			),
		)
	}
}
