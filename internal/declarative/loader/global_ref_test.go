package loader

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGlobalRefUniqueness(t *testing.T) {
	tests := []struct {
		name        string
		yaml        string
		expectError bool
		errorMsg    string
	}{
		{
			name: "duplicate ref across portal and api should fail",
			yaml: `
portals:
  - ref: shared-ref
    name: "Test Portal"
apis:
  - ref: shared-ref
    name: "Test API"
`,
			expectError: true,
			errorMsg:    "duplicate ref 'shared-ref': already used by portal resource, cannot use for api resource",
		},
		{
			name: "duplicate ref across portal and control_plane should fail",
			yaml: `
portals:
  - ref: common
    name: "Portal"
control_planes:
  - ref: common
    name: "Control Plane"
`,
			expectError: true,
			errorMsg:    "duplicate ref 'common': already used by portal resource, cannot use for control_plane resource",
		},
		{
			name: "duplicate ref within same resource type should fail",
			yaml: `
portals:
  - ref: portal-1
    name: "Portal One"
  - ref: portal-1
    name: "Portal Two"
`,
			expectError: true,
			errorMsg:    "duplicate ref 'portal-1': already used by another portal resource",
		},
		{
			name: "unique refs across all resource types should pass",
			yaml: `
portals:
  - ref: portal-ref
    name: "Test Portal"
control_planes:
  - ref: cp-ref
    name: "Control Plane"
apis:
  - ref: api-ref
    name: "Test API"
`,
			expectError: false,
		},
		{
			name: "empty configuration should pass",
			yaml: `
# Empty config
`,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loader := New()
			
			// Create a temporary source from the YAML string
			source := Source{
				Type: SourceTypeSTDIN,
			}
			
			// Mock stdin with our test YAML
			oldStdin := mockStdin(t, tt.yaml)
			defer restoreStdin(oldStdin)
			
			// Load and validate
			_, err := loader.LoadFromSources([]Source{source}, false)
			
			if tt.expectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestGlobalRefUniquenessAcrossMultipleFiles(t *testing.T) {
	// This test simulates loading from multiple files where refs conflict
	loader := New()
	
	// First file content
	file1 := `
portals:
  - ref: shared-ref
    name: "Portal in File 1"
`
	
	// Second file content (conflicts with first)
	file2 := `
apis:
  - ref: shared-ref
    name: "API in File 2"
`
	
	// Create temporary files
	tmpDir := t.TempDir()
	file1Path := tmpDir + "/file1.yaml"
	file2Path := tmpDir + "/file2.yaml"
	
	require.NoError(t, createTestFile(file1Path, file1))
	require.NoError(t, createTestFile(file2Path, file2))
	
	// Load both files
	sources := []Source{
		{Path: file1Path, Type: SourceTypeFile},
		{Path: file2Path, Type: SourceTypeFile},
	}
	
	_, err := loader.LoadFromSources(sources, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicate ref 'shared-ref'")
}

func TestGlobalRefUniquenessWithNestedResources(t *testing.T) {
	// Test that nested resources (e.g., versions inside APIs) also respect global uniqueness
	yaml := `
apis:
  - ref: api-1
    name: "Test API"
    versions:
      - ref: shared-ref
        name: "v1"
portals:
  - ref: shared-ref  # This should conflict with the nested version
    name: "Test Portal"
`
	
	loader := New()
	oldStdin := mockStdin(t, yaml)
	defer restoreStdin(oldStdin)
	
	source := Source{Type: SourceTypeSTDIN}
	_, err := loader.LoadFromSources([]Source{source}, false)
	
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicate ref 'shared-ref'")
}

// Helper functions for testing

func mockStdin(t *testing.T, content string) *os.File {
	t.Helper()
	
	// Create a pipe
	r, w, err := os.Pipe()
	require.NoError(t, err)
	
	// Write content to the pipe
	go func() {
		defer w.Close()
		_, err := w.WriteString(content)
		require.NoError(t, err)
	}()
	
	// Save original stdin
	oldStdin := os.Stdin
	
	// Replace stdin with our pipe
	os.Stdin = r
	
	return oldStdin
}

func restoreStdin(oldStdin *os.File) {
	os.Stdin = oldStdin
}

func createTestFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0600)
}