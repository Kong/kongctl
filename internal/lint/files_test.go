package lint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollectFiles_SingleFile(t *testing.T) {
	tmpDir := t.TempDir()
	f := filepath.Join(tmpDir, "config.yaml")
	require.NoError(t, os.WriteFile(f, []byte("name: test"), 0o600))

	files, err := CollectFiles([]string{f}, false)
	require.NoError(t, err)
	assert.Equal(t, []string{f}, files)
}

func TestCollectFiles_Directory(t *testing.T) {
	tmpDir := t.TempDir()
	f1 := filepath.Join(tmpDir, "a.yaml")
	f2 := filepath.Join(tmpDir, "b.yml")
	f3 := filepath.Join(tmpDir, "c.txt")
	require.NoError(t, os.WriteFile(f1, []byte("a"), 0o600))
	require.NoError(t, os.WriteFile(f2, []byte("b"), 0o600))
	require.NoError(t, os.WriteFile(f3, []byte("c"), 0o600))

	files, err := CollectFiles([]string{tmpDir}, false)
	require.NoError(t, err)
	assert.Len(t, files, 2) // only .yaml and .yml
	for _, f := range files {
		assert.True(t, strings.HasSuffix(f, ".yaml") || strings.HasSuffix(f, ".yml"))
	}
}

func TestCollectFiles_DirectoryRecursive(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "sub")
	require.NoError(t, os.MkdirAll(subDir, 0o755))

	f1 := filepath.Join(tmpDir, "top.yaml")
	f2 := filepath.Join(subDir, "nested.yaml")
	f3 := filepath.Join(subDir, "ignored.txt")
	require.NoError(t, os.WriteFile(f1, []byte("a"), 0o600))
	require.NoError(t, os.WriteFile(f2, []byte("b"), 0o600))
	require.NoError(t, os.WriteFile(f3, []byte("c"), 0o600))

	// Non-recursive should only find top-level
	nonRecursive, err := CollectFiles([]string{tmpDir}, false)
	require.NoError(t, err)
	assert.Len(t, nonRecursive, 1)

	// Recursive should find both
	recursive, err := CollectFiles([]string{tmpDir}, true)
	require.NoError(t, err)
	assert.Len(t, recursive, 2)
}

func TestCollectFiles_NonExistent(t *testing.T) {
	_, err := CollectFiles([]string{"/nonexistent/path"}, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot access")
}

func TestReadFromStdin(t *testing.T) {
	input := "name: test\nhost: example.com\n"
	r := strings.NewReader(input)

	data, err := ReadFromStdin(r)
	require.NoError(t, err)
	assert.Equal(t, input, string(data))
}

func TestIsYAMLFile(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"config.yaml", true},
		{"config.yml", true},
		{"config.YAML", true},
		{"config.YML", true},
		{"config.json", false},
		{"config.txt", false},
		{"config", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			assert.Equal(t, tt.expected, isYAMLFile(tt.path))
		})
	}
}
