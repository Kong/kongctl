package identity

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestClientSecretMutationCommandsValidateSelectors(t *testing.T) {
	id := "731559d3-39bd-48c7-b33c-eb192847eb21"
	for _, verb := range []verbs.VerbValue{verbs.Create, verbs.Patch, verbs.Delete} {
		group, err := NewIdentityCmd(verb, nil, nil)
		require.NoError(t, err)
		command, _, err := group.Find([]string{"auth-server-client-secrets"})
		require.NoError(t, err)
		require.NotNil(t, command.RunE)
		args := []string{id}
		if verb == verbs.Create {
			args = nil
			require.NotNil(t, command.Flags().Lookup("secret-output-file"))
			require.NotNil(t, command.Flags().Lookup("secret-file"))
			require.NotNil(t, command.Flags().Lookup("secret-env"))
		} else {
			require.Error(t, command.Args(command, nil))
			require.Error(t, command.Args(command, []string{"not-a-uuid"}))
		}
		require.ErrorContains(t, command.Args(command, args), "exactly one")
		require.NoError(t, command.Flags().Set("client-id", "opaque-client"))
		require.NoError(t, command.Args(command, args))
		require.NoError(t, command.Flags().Set("client-name", "machine"))
		require.ErrorContains(t, command.Args(command, args), "exactly one")
	}
}

func TestClientSecretSourcesReuseDeferredResolutionWithoutLeakingErrors(t *testing.T) {
	newCommand := func() *cobra.Command {
		return newAuthServerSecretMutationCommand(verbs.Create, nil, nil)
	}
	command := newCommand()
	value, err := identitySecretSource(command)
	require.NoError(t, err)
	require.Nil(t, value, "omitting a source requests generated material")
	t.Setenv("KONGCTL_TEST_CLIENT_SECRET", "private-import")
	require.NoError(t, command.Flags().Set("secret-env", "KONGCTL_TEST_CLIENT_SECRET"))
	value, err = identitySecretSource(command)
	require.NoError(t, err)
	require.Equal(t, "private-import", *value)
	for _, test := range []struct {
		name, body string
		invalid    bool
	}{
		{"source.txt", "private-import", false},
		{"bad.json", `{"private-parser-material":`, true},
		{"empty.txt", "", true},
	} {
		command := newCommand()
		path := filepath.Join(t.TempDir(), test.name)
		require.NoError(t, os.WriteFile(path, []byte(test.body), 0o600))
		require.NoError(t, command.Flags().Set("secret-file", path))
		value, err := identitySecretSource(command)
		if test.invalid {
			require.Error(t, err)
			require.NotContains(t, err.Error(), "private-parser-material")
			require.Nil(t, value)
		} else {
			require.NoError(t, err)
			require.Equal(t, test.body, *value)
		}
	}
}
