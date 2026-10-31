//go:build e2e

package scenario

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPrivateDirectoryIsRemovedAfterEarlyStopAndDoesNotReplaceVariables(t *testing.T) {
	var path string
	t.Run("interrupted", func(t *testing.T) {
		vars := map[string]any{"taken": "preserve"}
		require.NoError(t, registerPrivateTempDirectory(t, "", vars))
		require.Error(t, registerPrivateTempDirectory(t, "bad.name", vars))
		require.Error(t, registerPrivateTempDirectory(t, "taken", vars))
		require.Equal(t, "preserve", vars["taken"])
		require.NoError(t, registerPrivateTempDirectory(t, "secret_dir", vars))
		path = vars["secret_dir"].(string)
		info, err := os.Stat(path)
		require.NoError(t, err)
		require.Zero(t, info.Mode().Perm()&0o077)
		require.NoError(t, os.WriteFile(filepath.Join(path, "issued.json"), []byte("private"), 0o600))
	})
	_, err := os.Stat(path)
	require.True(t, os.IsNotExist(err))
}
