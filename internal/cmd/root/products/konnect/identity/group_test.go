package identity

import (
	"bytes"
	"testing"

	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/stretchr/testify/require"
)

func TestGroupReadCommandsRequireExplicitScope(t *testing.T) {
	for _, verb := range []verbs.VerbValue{verbs.Get, verbs.List} {
		for _, name := range []string{"groups", "group-memberships"} {
			root, err := NewIdentityCmd(verb, nil, nil)
			require.NoError(t, err)
			command, _, err := root.Find([]string{name})
			require.NoError(t, err)
			require.NotNil(t, command.RunE)
			require.ErrorContains(t, command.Args(command, nil), "directory")
			require.NoError(t, command.Flags().Set("directory-id", "invalid"))
			require.ErrorContains(t, command.Args(command, nil), "UUID")
			require.NoError(t, command.Flags().Set("directory-id", "731559d3-39bd-48c7-b33c-eb192847eb20"))
			if name == "group-memberships" {
				require.ErrorContains(t, command.Args(command, nil), "group")
				require.NoError(t, command.Flags().Set("group-name", "Platform"))
				require.ErrorContains(t, command.Args(command, []string{"Alice"}), "entry ID must be a UUID")
			}
			require.NoError(t, command.Args(command, nil))
			require.NoError(t, command.Args(command, []string{"731559d3-39bd-48c7-b33c-eb192847eb21"}))
			require.Error(t, command.Args(command, []string{"one", "two"}))
			require.NoError(t, command.Flags().Set("directory-name", "default"))
			require.ErrorContains(t, command.Args(command, nil), "exactly one")
		}
	}
}

func TestGroupMembershipTableIdentifiesEntryAndMember(t *testing.T) {
	var output bytes.Buffer
	require.NoError(t, printGroupReadRows(&output, []groupReadRow{{
		ID: "entry-id", MemberID: "member-id", Type: "principal", Name: "Alice",
	}}, true))
	require.Contains(t, output.String(), "MEMBER_ID")
	require.Contains(t, output.String(), "entry-id")
	require.Contains(t, output.String(), "member-id")
	require.Contains(t, output.String(), "Alice")
}
