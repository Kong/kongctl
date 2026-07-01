package get

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewGetCmdIncludesDirectIdentityDirectory(t *testing.T) {
	cmd, err := NewGetCmd()
	require.NoError(t, err)

	directoryCmd, _, err := cmd.Find([]string{"identity", "directory"})
	require.NoError(t, err)
	require.NotNil(t, directoryCmd)
	require.Equal(t, "directory", directoryCmd.Name())
}
