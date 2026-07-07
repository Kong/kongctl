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

func TestNewGetCmdIncludesDirectIdentityPrincipalChildren(t *testing.T) {
	cmd, err := NewGetCmd()
	require.NoError(t, err)

	principalsCmd, _, err := cmd.Find([]string{"identity", "directory", "principals"})
	require.NoError(t, err)
	require.NotNil(t, principalsCmd)
	require.Equal(t, "principals", principalsCmd.Name())

	identitiesCmd, _, err := cmd.Find([]string{"identity", "directory", "principals", "identities"})
	require.NoError(t, err)
	require.NotNil(t, identitiesCmd)
	require.Equal(t, "identities", identitiesCmd.Name())

	jqFlag := identitiesCmd.Flag("jq")
	require.NotNil(t, jqFlag)
}
