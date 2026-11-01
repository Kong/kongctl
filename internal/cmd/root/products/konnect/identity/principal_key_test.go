package identity

import (
	"testing"

	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/stretchr/testify/require"
)

func TestAPIKeyCommandsRequireExplicitScopeAndIntent(t *testing.T) {
	for _, group := range []bool{false, true} {
		parent := "principal"
		if group {
			parent = "group"
		}
		for _, verb := range []verbs.VerbValue{verbs.Get, verbs.List, verbs.Create, verbs.Patch, verbs.Delete} {
			t.Run(parent+"/"+string(verb), func(t *testing.T) {
				root, err := NewIdentityCmd(verb, nil, nil)
				require.NoError(t, err)
				cmd, _, err := root.Find([]string{parent + "-api-keys"})
				require.NoError(t, err)
				require.NotNil(t, cmd.RunE)
				require.ErrorContains(t, cmd.Args(cmd, nil), "directory")
				require.NoError(t, cmd.Flags().Set("directory-name", "default"))
				require.ErrorContains(t, cmd.Args(cmd, nil), parent+"-id")
				require.NoError(t, cmd.Flags().Set(parent+"-id", fixtureCommandID))
				args := []string{fixtureCommandID}
				if verb == verbs.Create {
					args = nil
					require.ErrorContains(t, cmd.ValidateRequiredFlags(), "secret-output-file")
					require.NoError(t, cmd.Flags().Set("secret-output-file", "private.json"))
				}
				if verb == verbs.Patch {
					require.ErrorContains(t, cmd.Args(cmd, args), "provide --expires-at")
					require.NoError(t, cmd.Flags().Set("no-expiration", "false"))
					require.ErrorContains(t, cmd.Args(cmd, args), "must be true")
					require.NoError(t, cmd.Flags().Set("no-expiration", "true"))
					if group {
						require.Nil(t, cmd.Flags().Lookup("name"))
					}
				}
				require.NoError(t, cmd.Args(cmd, args))
				require.Error(t, cmd.Args(cmd, []string{"bad-id", "another"}))
				if verb == verbs.Create {
					require.NoError(t, cmd.Flags().Set("secret-env", ""))
					require.ErrorContains(t, cmd.Args(cmd, args), "must not be empty")
					require.NoError(t, cmd.Flags().Set("secret-env", "KEY_SOURCE"))
					if group {
						require.NoError(t, cmd.Flags().Set("name", "ignored"))
						require.ErrorContains(t, cmd.Args(cmd, args), "imported group")
					}
				}
			})
		}
	}
}
