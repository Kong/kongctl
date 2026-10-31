package identity

import (
	"testing"

	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/stretchr/testify/require"
)

func TestIdentityDirectoryCommands(t *testing.T) {
	for _, verb := range []verbs.VerbValue{verbs.Get, verbs.List, verbs.Adopt} {
		command, err := NewIdentityCmd(verb, nil, nil)
		require.NoError(t, err)
		child, _, err := command.Find([]string{"directories"})
		require.NoError(t, err)
		require.Equal(t, "directory", child.Name())
		require.NotNil(t, child.RunE)
		require.Error(t, child.Args(child, []string{"one", "two"}))
		if verb == verbs.Adopt {
			require.Error(t, child.Args(child, nil))
		}
	}
}
