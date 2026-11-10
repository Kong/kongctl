package identity

import (
	"bytes"
	"strings"
	"testing"

	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestNestedGroupMembershipCommands(t *testing.T) {
	for _, verb := range []verbs.VerbValue{verbs.Get, verbs.List} {
		for _, path := range [][]string{
			{"identity", "group", "memberships"},
			{"id", "group", "membership"},
			{"id", "groups", "memberships"},
			{"identity", "group-memberships"},
		} {
			t.Run(string(verb)+"/"+strings.Join(path, "/"), func(t *testing.T) {
				identity, err := NewIdentityCmd(verb, nil, nil)
				require.NoError(t, err)
				root := &cobra.Command{Use: string(verb), SilenceUsage: true, SilenceErrors: true}
				root.AddCommand(identity)
				membership, _, err := root.Find(path)
				require.NoError(t, err)
				called := false
				membership.RunE = func(cmd *cobra.Command, args []string) error {
					called = true
					selector, err := groupParentSelector(cmd, "directory")
					require.NoError(t, err)
					require.Equal(t, "default", selector)
					require.Empty(t, args)
					return nil
				}
				root.SetArgs(append(path, "--group-name", "DemoSupportOperators"))
				require.NoError(t, root.Execute())
				require.True(t, called)
			})
		}
	}
}

func TestGroupReadCommandsRequireExplicitScope(t *testing.T) {
	for _, verb := range []verbs.VerbValue{verbs.Get, verbs.List} {
		for _, name := range []string{"groups", "group-memberships"} {
			root, err := NewIdentityCmd(verb, nil, nil)
			require.NoError(t, err)
			command, _, err := root.Find([]string{name})
			require.NoError(t, err)
			require.NotNil(t, command.RunE)
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
