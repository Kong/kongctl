package list

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewListCmdIncludesDirectIdentityDirectory(t *testing.T) {
	cmd, err := NewListCmd()
	require.NoError(t, err)

	directoryCmd, _, err := cmd.Find([]string{"identity", "directories"})
	require.NoError(t, err)
	require.NotNil(t, directoryCmd)
	require.Equal(t, "directory", directoryCmd.Name())
}
