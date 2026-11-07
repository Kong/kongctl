package loader

import (
	"strings"
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/stretchr/testify/require"
)

func TestMembershipNestedAndGroupedAuthority(t *testing.T) {
	input := `_defaults:
  kongctl: {namespace: directory-team}
identity:
  directories:
    - ref: people
      _external: {id: 4078f559-063b-4f10-9098-9e6553b21d9b}
      principals:
        - ref: alice
          type: user
          kongctl: {namespace: principal-team}
      groups:
        - ref: operators
          name: Operators
          kongctl: {namespace: group-team}
          memberships:
            - ref: alice-member
              member: {type: principal, id: !ref alice#id}
  memberships:
    - ref: existing-member
      group: operators
      member: {type: principal, id: 5078f559-063b-4f10-9098-9e6553b21d9b}
`
	rs, err := New().LoadFile(writeLoaderTestFile(t, input))
	require.NoError(t, err)
	require.Len(t, rs.Identity.Memberships, 2)
	selected := resources.GetResourcesByNamespace[resources.IdentityGroupMembershipResource](rs, "group-team")
	require.Len(t, selected, 2)
	require.Empty(t, resources.GetResourcesByNamespace[resources.IdentityGroupMembershipResource](rs, "principal-team"))
	require.True(
		t,
		rs.SyncScope.ChildInScope(
			resources.ResourceTypeIdentityGroup,
			"operators",
			resources.ResourceTypeIdentityGroupMembership,
		),
	)
	for _, replacement := range []struct{ old, next, message string }{
		{"name: Operators", "_external: {id: 6078f559-063b-4f10-9098-9e6553b21d9b}", "managed ordinary group"},
		{"type: principal, id: !ref alice#id", "type: group, id: !ref operators#id", "Q32"},
		{"type: principal, id: !ref alice#id", "type: principal, id: !ref operators#id", "declared principal"},
		{"type: principal, id: !ref alice#id", "type: principal, id: not-a-uuid", "principal UUID"},
		{"ref: alice-member", "ref: alice-member\n              kongctl: {namespace: other}", "kongctl"},
	} {
		candidate := strings.Replace(input, replacement.old, replacement.next, 1)
		if replacement.old == "name: Operators" {
			candidate = strings.Replace(candidate, "          kongctl: {namespace: group-team}\n", "", 1)
		}
		_, err := New().LoadFile(writeLoaderTestFile(t, candidate))
		require.ErrorContains(t, err, replacement.message)
	}
}

func TestMembershipOmittedAndEmptyCollections(t *testing.T) {
	for _, collection := range []string{"", "          memberships: []\n"} {
		input := `identity:
  directories:
    - ref: people
      _external: {id: 4078f559-063b-4f10-9098-9e6553b21d9b}
      groups:
        - ref: operators
          name: Operators
` + collection
		rs, err := New().LoadFile(writeLoaderTestFile(t, input))
		require.NoError(t, err)
		require.Equal(
			t,
			collection != "",
			rs.SyncScope.ChildInScope(
				resources.ResourceTypeIdentityGroup,
				"operators",
				resources.ResourceTypeIdentityGroupMembership,
			),
		)
	}
}
