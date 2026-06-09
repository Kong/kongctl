package dump

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetDumpWriterExpandsHomeDirectory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	require.NoError(t, os.Mkdir(filepath.Join(home, "kongctl"), 0o755))

	writer, cleanup, err := getDumpWriter(nil, "~/kongctl/api.yaml")
	require.NoError(t, err)
	_, err = writer.Write([]byte("apis: []\n"))
	require.NoError(t, err)
	require.NoError(t, cleanup())

	content, err := os.ReadFile(filepath.Join(home, "kongctl", "api.yaml"))
	require.NoError(t, err)
	require.Equal(t, "apis: []\n", string(content))
}
