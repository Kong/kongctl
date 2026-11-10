package identity

import (
	"testing"

	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/stretchr/testify/require"
)

func TestAPIKeyReadCommandsRequireExplicitParents(t *testing.T) {
	for _, group := range []bool{false, true} {
		for _, verb := range []verbs.VerbValue{verbs.Get, verbs.List} {
			cmd, err := newIdentityAPIKeyCommand(verb, group, nil, nil)
			require.NoError(t, err)
			parent := "principal-id"
			if group {
				parent = "group-id"
			}
			require.ErrorContains(t, cmd.Args(cmd, nil), parent)
			require.NoError(t, cmd.Flags().Set(parent, fixtureCommandID))
			require.NoError(t, cmd.Args(cmd, nil))
			require.NoError(t, cmd.Args(cmd, []string{fixtureCommandID}))
			require.Error(t, cmd.Args(cmd, []string{"invalid"}))
			for _, flag := range []string{"name", "clear-name", "expires-at", "secret-env", "secret-output-file"} {
				require.Nil(t, cmd.Flags().Lookup(flag))
			}
		}
	}
}
