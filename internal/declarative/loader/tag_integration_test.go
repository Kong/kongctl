package loader

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3" //nolint:gomodguard // yaml.v3 required for custom tag processing
)

// mockFileResolver is a test resolver that replaces !file tags with test values
type mockFileResolver struct{}

func (m *mockFileResolver) Tag() string {
	return "!file"
}

func (m *mockFileResolver) Resolve(_ *yaml.Node) (interface{}, error) {
	// For testing, just return a fixed value
	return "test-value-from-file", nil
}

func TestLoader_TagProcessing(t *testing.T) {
	// Create a loader
	loader := New()
	
	// Register a mock file resolver
	loader.getTagRegistry().Register(&mockFileResolver{})
	
	// Test YAML with a !file tag
	yamlContent := `
portals:
  - ref: test-portal
    name: !file portal-name.txt`
	
	// Create a temporary file with the YAML content
	tmpDir, err := os.MkdirTemp("", "loader-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)
	
	tmpfile := filepath.Join(tmpDir, "test.yaml")
	err = os.WriteFile(tmpfile, []byte(yamlContent), 0600)
	require.NoError(t, err)
	
	// Load the file
	rs, err := loader.LoadFile(tmpfile)
	assert.NoError(t, err)
	assert.NotNil(t, rs)
	
	// Verify the tag was processed
	assert.Len(t, rs.Portals, 1)
	assert.Equal(t, "test-portal", rs.Portals[0].Ref)
	assert.Equal(t, "test-value-from-file", rs.Portals[0].Name)
}

func TestLoader_NoTagsRegistered(t *testing.T) {
	// Create a loader without any tag resolvers
	loader := New()
	
	// Test YAML with a tag (should be ignored since no resolvers)
	yamlContent := `
portals:
  - ref: test-portal
    name: Regular Name`
	
	tmpDir, err := os.MkdirTemp("", "loader-test2-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)
	
	tmpfile := filepath.Join(tmpDir, "test.yaml")
	err = os.WriteFile(tmpfile, []byte(yamlContent), 0600)
	require.NoError(t, err)
	
	// Load should work normally
	rs, err := loader.LoadFile(tmpfile)
	assert.NoError(t, err)
	assert.NotNil(t, rs)
	assert.Len(t, rs.Portals, 1)
	assert.Equal(t, "Regular Name", rs.Portals[0].Name)
}