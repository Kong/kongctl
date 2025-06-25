package loader

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	loader := New("/test/path")
	assert.NotNil(t, loader)
	assert.Equal(t, "/test/path", loader.rootPath)
}

func TestLoader_LoadFile_ValidConfigs(t *testing.T) {
	tests := []struct {
		name                 string
		file                 string
		expectedPortals      int
		expectedAuthStrats   int
		expectedControlPlanes int
		expectedAPIs         int
	}{
		{
			name:            "simple portal",
			file:            "valid/simple-portal.yaml",
			expectedPortals: 1,
		},
		{
			name:               "auth strategy",
			file:               "valid/auth-strategy.yaml",
			expectedAuthStrats: 1,
		},
		{
			name:                  "control plane",
			file:                  "valid/control-plane.yaml",
			expectedControlPlanes: 1,
		},
		{
			name:                  "api with children",
			file:                  "valid/api-with-children.yaml",
			expectedPortals:       1,
			expectedAuthStrats:    1,
			expectedControlPlanes: 1,
			expectedAPIs:          1,
		},
		{
			name:                  "multi resource",
			file:                  "complex/multi-resource.yaml",
			expectedPortals:       1,
			expectedAuthStrats:    1,
			expectedControlPlanes: 1,
			expectedAPIs:          1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loader := New(".")
			filePath := filepath.Join("testdata", tt.file)
			
			rs, err := loader.LoadFile(filePath)
			assert.NoError(t, err, "LoadFile should not return an error for valid config")
			assert.NotNil(t, rs, "ResourceSet should not be nil")
			
			assert.Len(t, rs.Portals, tt.expectedPortals, "Portal count mismatch")
			assert.Len(t, rs.ApplicationAuthStrategies, tt.expectedAuthStrats, "Auth strategy count mismatch")
			assert.Len(t, rs.ControlPlanes, tt.expectedControlPlanes, "Control plane count mismatch")
			assert.Len(t, rs.APIs, tt.expectedAPIs, "API count mismatch")
		})
	}
}

func TestLoader_LoadFile_InvalidConfigs(t *testing.T) {
	tests := []struct {
		name        string
		file        string
		expectedErr string
	}{
		{
			name:        "missing portal ref",
			file:        "invalid/missing-portal-ref.yaml",
			expectedErr: "portal ref is required",
		},
		{
			name:        "duplicate refs",
			file:        "invalid/duplicate-refs.yaml",
			expectedErr: "duplicate portal ref: duplicate-portal",
		},
		{
			name:        "missing reference",
			file:        "invalid/missing-reference.yaml",
			expectedErr: "references unknown application_auth_strategy: nonexistent-strategy",
		},
		{
			name:        "malformed yaml",
			file:        "invalid/malformed-yaml.yaml",
			expectedErr: "failed to parse YAML",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loader := New(".")
			filePath := filepath.Join("testdata", tt.file)
			
			rs, err := loader.LoadFile(filePath)
			assert.Error(t, err, "LoadFile should return an error for invalid config")
			assert.Nil(t, rs, "ResourceSet should be nil for invalid config")
			assert.Contains(t, err.Error(), tt.expectedErr, "Error message should contain expected text")
		})
	}
}

func TestLoader_LoadFile_FileNotFound(t *testing.T) {
	loader := New(".")
	
	rs, err := loader.LoadFile("nonexistent-file.yaml")
	assert.Error(t, err)
	assert.Nil(t, rs)
	assert.Contains(t, err.Error(), "failed to open file")
}

func TestLoader_LoadFile_DefaultValues(t *testing.T) {
	loader := New(".")
	filePath := filepath.Join("testdata", "valid", "simple-portal.yaml")
	
	rs, err := loader.LoadFile(filePath)
	assert.NoError(t, err)
	assert.NotNil(t, rs)
	
	// Test that defaults were applied - portal name should default to ref
	portal := rs.Portals[0]
	assert.Equal(t, "test-portal", portal.GetRef())
	assert.Equal(t, "Test Portal", portal.Name, "Portal name should be preserved when provided")
}

func TestLoader_LoadFile_APIWithChildren(t *testing.T) {
	loader := New(".")
	filePath := filepath.Join("testdata", "valid", "api-with-children.yaml")
	
	rs, err := loader.LoadFile(filePath)
	assert.NoError(t, err)
	assert.NotNil(t, rs)
	assert.Len(t, rs.APIs, 1)
	
	api := rs.APIs[0]
	assert.Equal(t, "test-api", api.GetRef())
	assert.Len(t, api.Versions, 1, "Should have 1 version")
	assert.Len(t, api.Publications, 1, "Should have 1 publication")
	assert.Len(t, api.Implementations, 1, "Should have 1 implementation")
	
	// Check child resources
	version := api.Versions[0]
	assert.Equal(t, "test-api-v1", version.GetRef())
	
	publication := api.Publications[0]
	assert.Equal(t, "test-api-pub", publication.GetRef())
	
	implementation := api.Implementations[0]
	assert.Equal(t, "test-api-impl", implementation.GetRef())
}

func TestLoader_Load_FileVsDirectory(t *testing.T) {
	loader := New(".")
	
	// Test loading a single file
	filePath := filepath.Join("testdata", "valid", "simple-portal.yaml")
	rs, err := loader.LoadFile(filePath)
	assert.NoError(t, err)
	assert.Len(t, rs.Portals, 1)
	
	// Test Load method with single file path
	loader = New(filePath)
	rs2, err := loader.Load()
	assert.NoError(t, err)
	assert.Len(t, rs2.Portals, 1)
	
	// Test Load method with directory path
	// Note: The valid directory has files with duplicate refs, so it should fail
	loader = New(filepath.Join("testdata", "valid"))
	rs3, err := loader.Load()
	assert.Error(t, err, "Should fail due to duplicate refs across files")
	assert.Nil(t, rs3)
	assert.Contains(t, err.Error(), "duplicate")
}

func TestLoader_ParseYAML_EmptyFile(t *testing.T) {
	// Create empty temp file
	tmpfile, err := os.CreateTemp("", "empty-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())
	tmpfile.Close()
	
	loader := New(".")
	rs, err := loader.LoadFile(tmpfile.Name())
	
	// Empty file should parse successfully with empty ResourceSet
	assert.NoError(t, err)
	assert.NotNil(t, rs)
	assert.Len(t, rs.Portals, 0)
	assert.Len(t, rs.ApplicationAuthStrategies, 0)
	assert.Len(t, rs.ControlPlanes, 0)
	assert.Len(t, rs.APIs, 0)
}

func TestLoader_ParseYAML_WithComments(t *testing.T) {
	yamlContent := `
# This is a comment
portals:
  # Portal definition
  - ref: test-portal
    name: "Test Portal"  # Inline comment
    description: "A test portal"
`
	
	// Create temp file with content
	tmpfile, err := os.CreateTemp("", "comments-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())
	
	if _, err := tmpfile.Write([]byte(yamlContent)); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpfile.Close()
	
	loader := New(".")
	rs, err := loader.LoadFile(tmpfile.Name())
	
	assert.NoError(t, err)
	assert.NotNil(t, rs)
	assert.Len(t, rs.Portals, 1)
	assert.Equal(t, "test-portal", rs.Portals[0].GetRef())
}

func TestLoader_LoadFile_LongFieldPath(t *testing.T) {
	// Test that files with deeply nested structures parse correctly
	// This tests the reflection-based field access
	loader := New(".")
	filePath := filepath.Join("testdata", "valid", "api-with-children.yaml")
	
	rs, err := loader.LoadFile(filePath)
	assert.NoError(t, err)
	assert.NotNil(t, rs)
	
	// Verify that the implementation service fields are accessible
	impl := rs.APIs[0].Implementations[0]
	assert.NotNil(t, impl.Service)
	assert.Equal(t, "12345678-1234-1234-1234-123456789012", impl.Service.ID)
	assert.Equal(t, "test-cp", impl.Service.ControlPlaneID)
}

func TestLoader_Load_Directory(t *testing.T) {
	// Test loading multiple files from a directory
	loader := New(filepath.Join("testdata", "multifile"))
	
	rs, err := loader.Load()
	assert.NoError(t, err)
	assert.NotNil(t, rs)
	
	// Should have loaded resources from multiple files including subdirectory
	assert.Len(t, rs.Portals, 3, "Should have 3 portals (2 from portals.yaml, 1 from subdirectory)")
	assert.Len(t, rs.ApplicationAuthStrategies, 1, "Should have 1 auth strategy")
	assert.Len(t, rs.ControlPlanes, 1, "Should have 1 control plane")
	assert.Len(t, rs.APIs, 1, "Should have 1 API")
	
	// Verify specific resources
	portalRefs := make([]string, len(rs.Portals))
	for i, portal := range rs.Portals {
		portalRefs[i] = portal.GetRef()
	}
	assert.Contains(t, portalRefs, "multifile-portal1")
	assert.Contains(t, portalRefs, "multifile-portal2")
	assert.Contains(t, portalRefs, "subdirectory-portal")
	
	// Verify API has nested resources
	api := rs.APIs[0]
	assert.Equal(t, "multifile-api", api.GetRef())
	assert.Len(t, api.Versions, 1)
	assert.Len(t, api.Publications, 1)
	assert.Len(t, api.Implementations, 1)
}

func TestLoader_Load_DirectoryWithDuplicates(t *testing.T) {
	// Test that duplicate refs across files are detected
	loader := New(filepath.Join("testdata", "multifile-duplicates"))
	
	rs, err := loader.Load()
	assert.Error(t, err)
	assert.Nil(t, rs)
	assert.Contains(t, err.Error(), "duplicate portal ref: duplicate-portal")
}

func TestLoader_Load_DirectoryFiltering(t *testing.T) {
	// Test that non-YAML files are ignored
	// The multifile directory contains README.txt which should be ignored
	loader := New(filepath.Join("testdata", "multifile"))
	
	rs, err := loader.Load()
	assert.NoError(t, err)
	assert.NotNil(t, rs)
	
	// Should successfully load only YAML files
	// The README.txt should be ignored
}

func TestLoader_Load_SingleFile(t *testing.T) {
	// Test that Load() works with a single file path
	loader := New(filepath.Join("testdata", "valid", "simple-portal.yaml"))
	
	rs, err := loader.Load()
	assert.NoError(t, err)
	assert.NotNil(t, rs)
	assert.Len(t, rs.Portals, 1)
}

func TestLoader_Load_EmptyDirectory(t *testing.T) {
	// Create empty temp directory
	tmpDir, err := os.MkdirTemp("", "empty-loader-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	
	loader := New(tmpDir)
	rs, err := loader.Load()
	
	// Should return empty ResourceSet without error
	assert.NoError(t, err)
	assert.NotNil(t, rs)
	assert.Len(t, rs.Portals, 0)
	assert.Len(t, rs.ApplicationAuthStrategies, 0)
	assert.Len(t, rs.ControlPlanes, 0)
	assert.Len(t, rs.APIs, 0)
}

func TestLoader_Load_InvalidPath(t *testing.T) {
	loader := New("/nonexistent/path")
	
	rs, err := loader.Load()
	assert.Error(t, err)
	assert.Nil(t, rs)
	assert.Contains(t, err.Error(), "failed to stat path")
}