package identity

import (
	"testing"
	"time"

	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

const fixtureCommandID = "731559d3-39bd-48c7-b33c-eb192847eb20"

func passwordScopedCommand(t *testing.T, verb verbs.VerbValue) *cobra.Command {
	t.Helper()
	command, err := newPasswordMutationCommand(verb, nil, nil)
	require.NoError(t, err)
	require.NoError(t, command.Flags().Set("directory-name", "default"))
	require.NoError(t, command.Flags().Set("principal-id", fixtureCommandID))
	require.NoError(t, command.Flags().Set("basic-auth-id", fixtureCommandID))
	return command
}

func TestPasswordMutationCommandsRequireExplicitParentScope(t *testing.T) {
	for _, verb := range []verbs.VerbValue{verbs.Create, verbs.Patch, verbs.Delete} {
		t.Run(string(verb), func(t *testing.T) {
			root, err := NewIdentityCmd(verb, nil, nil)
			require.NoError(t, err)
			command, _, err := root.Find([]string{"principal-basic-auth-passwords"})
			require.NoError(t, err)
			require.NotNil(t, command.RunE)
			require.ErrorContains(t, command.Args(command, nil), "directory")
			require.NoError(t, command.Flags().Set("directory-name", "default"))
			require.ErrorContains(t, command.Args(command, nil), "principal-id")
			require.NoError(t, command.Flags().Set("principal-id", fixtureCommandID))
			require.ErrorContains(t, command.Args(command, nil), "exactly one")
			require.NoError(t, command.Flags().Set("username", fixtureCommandID))
			args := []string{fixtureCommandID}
			//exhaustive:ignore
			switch verb {
			case verbs.Create:
				args = nil
				require.ErrorContains(t, command.ValidateRequiredFlags(), "secret-output-file")
				require.NoError(t, command.Flags().Set("secret-output-file", "new-password.json"))
			case verbs.Patch:
				require.NoError(t, command.Flags().Set("no-expiration", "true"))
			}
			require.NoError(t, command.Args(command, args))
			require.Error(t, command.Args(command, []string{"one", "two"}))
			require.NoError(t, command.Flags().Set("basic-auth-id", fixtureCommandID))
			require.ErrorContains(t, command.Args(command, args), "exactly one")
		})
	}
}

func TestPasswordPatchRequiresAnExplicitChange(t *testing.T) {
	command := passwordScopedCommand(t, verbs.Patch)
	args := []string{fixtureCommandID}
	require.ErrorContains(t, command.Args(command, args), "provide --expires-at")
	require.ErrorContains(t, command.Args(command, []string{"not-uuid"}), "must be a UUID")
	require.NoError(t, command.Flags().Set("secret-env", "PASSWORD_SOURCE"))
	require.NoError(t, command.Args(command, args), "a source explicitly requests in-place replacement")
	require.NoError(t, command.Flags().Set("secret-file", "password.txt"))
	require.ErrorContains(t, command.Args(command, args), "not both")

	command = passwordScopedCommand(t, verbs.Patch)
	require.NoError(t, command.Flags().Set("secret-file", ""))
	require.ErrorContains(t, command.Args(command, args), "must not be empty")
}

func TestPasswordExpirationDistinguishesOmissionValueAndClear(t *testing.T) {
	command := passwordScopedCommand(t, verbs.Create)
	value, supplied, err := passwordExpirationFromFlags(command)
	require.NoError(t, err)
	require.False(t, supplied)
	require.Nil(t, value)
	require.NoError(t, command.Flags().Set("expires-at", "2031-01-01T02:00:00.123456+02:00"))
	value, supplied, err = passwordExpirationFromFlags(command)
	require.NoError(t, err)
	require.True(t, supplied)
	require.Equal(t, "2031-01-01T00:00:00.123456Z", value.UTC().Format(time.RFC3339Nano))
	require.NoError(t, command.Flags().Set("no-expiration", "true"))
	_, _, err = passwordExpirationFromFlags(command)
	require.ErrorContains(t, err, "not both")
	for _, raw := range []string{"", "tomorrow", "2031-01-01"} {
		command = passwordScopedCommand(t, verbs.Patch)
		require.NoError(t, command.Flags().Set("expires-at", raw))
		require.Error(t, command.Args(command, []string{fixtureCommandID}))
	}
	command = passwordScopedCommand(t, verbs.Patch)
	require.NoError(t, command.Flags().Set("no-expiration", "false"))
	require.ErrorContains(t, command.Args(command, []string{fixtureCommandID}), "must be true")
	command = passwordScopedCommand(t, verbs.Patch)
	require.NoError(t, command.Flags().Set("no-expiration", "true"))
	value, supplied, err = passwordExpirationFromFlags(command)
	require.NoError(t, err)
	require.True(t, supplied)
	require.Nil(t, value)
}

func TestPasswordDeleteAllRequiresExplicitWholeCollectionSelection(t *testing.T) {
	command := passwordScopedCommand(t, verbs.Delete)
	require.Error(t, command.Args(command, nil))
	require.NoError(t, command.Args(command, []string{fixtureCommandID}))
	require.NoError(t, command.Flags().Set("all", "true"))
	require.NoError(t, command.Args(command, nil))
	require.ErrorContains(t, command.Args(command, []string{fixtureCommandID}), "not both")
}
