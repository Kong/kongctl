package identity

import (
	"bytes"
	"testing"
	"time"

	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/stretchr/testify/require"
)

func TestBasicAuthReadCommandsRequireExplicitScopeAndSelectors(t *testing.T) {
	const id = "731559d3-39bd-48c7-b33c-eb192847eb20"
	for _, verb := range []verbs.VerbValue{verbs.Get, verbs.List} {
		for _, name := range []string{"principal-basic-auths", "principal-basic-auth-passwords"} {
			t.Run(string(verb)+"/"+name, func(t *testing.T) {
				root, err := NewIdentityCmd(verb, nil, nil)
				require.NoError(t, err)
				command, _, err := root.Find([]string{name})
				require.NoError(t, err)
				require.NotNil(t, command.RunE)
				require.NoError(t, command.Flags().Set("directory-name", "default"))
				require.ErrorContains(t, command.Args(command, nil), "principal-id")
				require.NoError(t, command.Flags().Set("principal-id", id))
				passwords := name == "principal-basic-auth-passwords"
				if passwords {
					require.ErrorContains(t, command.Args(command, nil), "exactly one")
				} else {
					require.NoError(t, command.Args(command, nil))
				}
				require.NoError(t, command.Flags().Set("username", id))
				require.NoError(t, command.Args(command, nil), "UUID-shaped usernames are explicit names")
				require.Error(t, command.Args(command, []string{"not-a-uuid"}))
				require.Error(t, command.Args(command, []string{"one", "two"}))
				if passwords {
					require.NoError(t, command.Args(command, []string{id}))
					require.NoError(t, command.Flags().Set("basic-auth-id", id))
					require.ErrorContains(t, command.Args(command, nil), "exactly one")
				} else {
					require.ErrorContains(t, command.Args(command, []string{id}), "not both")
				}
				require.NoError(t, command.Flags().Set("username", ""))
				require.ErrorContains(t, command.Args(command, nil), "must not be empty")
			})
		}
	}
}

func TestBasicAuthReadCommandsAcceptUUIDSelectors(t *testing.T) {
	const id = "731559d3-39bd-48c7-b33c-eb192847eb20"
	for _, passwords := range []bool{false, true} {
		command, err := newBasicAuthReadCommand(verbs.Get, passwords, nil, nil)
		require.NoError(t, err)
		require.NoError(t, command.Flags().Set("directory-id", id))
		require.NoError(t, command.Flags().Set("principal-id", id))
		if passwords {
			require.NoError(t, command.Flags().Set("basic-auth-id", "invalid"))
			require.ErrorContains(t, command.Args(command, nil), "basic-auth-id must be a UUID")
			require.NoError(t, command.Flags().Set("basic-auth-id", id))
		}
		require.NoError(t, command.Args(command, []string{id}))
		require.NoError(t, command.Flags().Set("directory-name", "default"))
		require.ErrorContains(t, command.Args(command, nil), "exactly one")
	}
}

func TestBasicAuthTextOutputUsesTablesForDetailAndLists(t *testing.T) {
	var output bytes.Buffer
	require.NoError(t, printBasicAuthReadRows(&output, [][2]string{{"id", "worker"}}, false))
	require.Contains(t, output.String(), "USERNAME")
	require.Contains(t, output.String(), "worker")
	require.NotContains(t, output.String(), "{")
	output.Reset()
	require.NoError(t, printBasicAuthReadRows(&output, [][2]string{{"password-id", "-"}}, true))
	require.Contains(t, output.String(), "EXPIRES_AT")
	require.Contains(t, output.String(), "password-id")
	require.NotContains(t, output.String(), "{")
	require.Equal(t, "-", basicAuthPasswordExpiration(nil))
	expiry, err := time.Parse(time.RFC3339, "2031-01-01T02:00:00+02:00")
	require.NoError(t, err)
	require.Equal(t, "2031-01-01T00:00:00Z", basicAuthPasswordExpiration(&expiry))
}
