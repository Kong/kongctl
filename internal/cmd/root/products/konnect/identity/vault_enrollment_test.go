package identity

import (
	"testing"

	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/stretchr/testify/require"
)

func TestVaultEnrollmentCommandsRequireScopeIDsAndPrivateOutput(t *testing.T) {
	for _, mode := range []string{"create", "enrollment", "delete"} {
		verb := verbs.Create
		if mode == "delete" {
			verb = verbs.Delete
		}
		command, err := newVaultCredentialMutationCommand(verb, mode == "enrollment", nil, nil)
		require.NoError(t, err)
		require.ErrorContains(t, command.Args(command, nil), "directory")
		require.NoError(t, command.Flags().Set("directory-name", "default"))
		require.ErrorContains(t, command.Args(command, nil), "provider")
		require.NoError(t, command.Flags().Set("provider-id", fixtureCommandID))
		var args []string
		if mode != "create" {
			args = []string{fixtureCommandID}
		}
		require.NoError(t, command.Args(command, args))
		if verb == verbs.Create {
			require.ErrorContains(t, command.ValidateRequiredFlags(), "secret-output-file")
			require.NoError(t, command.Flags().Set("secret-output-file", ""))
			require.Error(t, command.Args(command, args))
			require.NoError(t, command.Flags().Set("secret-output-file", "new-private-capture.json"))
		}
		require.NoError(t, command.Args(command, args))
		require.Error(t, command.Args(command, []string{"not-uuid"}))
		require.NoError(t, command.Flags().Set("provider-name", "provider"))
		require.Error(t, command.Args(command, args))
		root, err := NewIdentityCmd(verb, nil, nil)
		require.NoError(t, err)
		for _, name := range append([]string{command.Name()}, command.Aliases...) {
			found, _, err := root.Find([]string{name})
			require.NoError(t, err)
			require.Equal(t, command.Name(), found.Name())
		}
	}
	_, err := newVaultCredentialMutationCommand(verbs.Patch, false, nil, nil)
	require.Error(t, err)
	_, err = newVaultCredentialMutationCommand(verbs.Delete, true, nil, nil)
	require.Error(t, err)
}

func TestVaultEnrollmentRoutingRequiresExplicitPairAndPreservesZero(t *testing.T) {
	command, err := newVaultCredentialMutationCommand(verbs.Create, false, nil, nil)
	require.NoError(t, err)
	routing, err := vaultCredentialRoutingFromFlags(command)
	require.NoError(t, err)
	require.Nil(t, routing.Selector)
	require.Nil(t, routing.Priority)
	require.NoError(t, command.Flags().Set("selector", "true"))
	_, err = vaultCredentialRoutingFromFlags(command)
	require.Error(t, err)
	require.NoError(t, command.Flags().Set("priority", "0"))
	routing, err = vaultCredentialRoutingFromFlags(command)
	require.NoError(t, err)
	require.EqualValues(t, 0, *routing.Priority)
	require.Equal(t, "true", *routing.Selector)
	require.NoError(t, command.Flags().Set("priority", "-1"))
	_, err = vaultCredentialRoutingFromFlags(command)
	require.Error(t, err)
	require.NoError(t, command.Flags().Set("selector", ""))
	_, err = vaultCredentialRoutingFromFlags(command)
	require.Error(t, err)
}
