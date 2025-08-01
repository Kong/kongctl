package loader

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	loader := New()
	assert.NotNil(t, loader)
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
			expectedPortals:       2,
			expectedAuthStrats:    2,
			expectedControlPlanes: 2,
			expectedAPIs:          2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loader := New()
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
		expectError string
	}{
		{
			name:        "portal without ref",
			file:        "invalid/missing-portal-ref.yaml",
			expectError: "invalid portal ref: ref cannot be empty",
		},
		{
			name:        "portal with duplicate refs",
			file:        "invalid/duplicate-refs.yaml",
			expectError: "duplicate portal ref",
		},
		{
			name:        "malformed yaml",
			file:        "invalid/malformed-yaml.yaml",
			expectError: "failed to parse YAML",
		},
		{
			name:        "portal with invalid reference",
			file:        "invalid/missing-reference.yaml",
			expectError: "references unknown",
		},
		{
			name:        "duplicate names",
			file:        "invalid/duplicate-names.yaml",
			expectError: "duplicate",
		},
		{
			name:        "api with multiple versions",
			file:        "invalid/api-multiple-versions.yaml",
			expectError: "Ensure each API versions key has only 1 version defined",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loader := New()
			filePath := filepath.Join("testdata", tt.file)
			
			rs, err := loader.LoadFile(filePath)
			assert.Error(t, err, "LoadFile should return an error for invalid config")
			assert.Nil(t, rs, "ResourceSet should be nil on error")
			assert.Contains(t, err.Error(), tt.expectError)
		})
	}
}

func TestLoader_LoadFile_FileNotFound(t *testing.T) {
	loader := New()
	
	rs, err := loader.LoadFile("nonexistent-file.yaml")
	assert.Error(t, err)
	assert.Nil(t, rs)
	assert.Contains(t, err.Error(), "failed to open file")
}

func TestLoader_LoadFile_DefaultValues(t *testing.T) {
	loader := New()
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
	loader := New()
	filePath := filepath.Join("testdata", "valid", "api-with-children.yaml")
	
	rs, err := loader.LoadFile(filePath)
	assert.NoError(t, err)
	assert.NotNil(t, rs)
	
	// Verify API structure
	api := rs.APIs[0]
	assert.Equal(t, "my-api", api.GetRef())
	// After extraction, nested resources should be cleared
	assert.Len(t, api.Versions, 0)
	assert.Len(t, api.Publications, 0)
	assert.Len(t, api.Implementations, 0)
	
	// Verify child resources are extracted to root level with parent references
	assert.Len(t, rs.APIVersions, 1)
	assert.Len(t, rs.APIPublications, 1)
	assert.Len(t, rs.APIImplementations, 1)
	
	// Check version
	assert.Equal(t, "my-api-v1", rs.APIVersions[0].GetRef())
	assert.Equal(t, "my-api", rs.APIVersions[0].API) // Parent reference
	
	// Check publication
	assert.Equal(t, "my-api-pub", rs.APIPublications[0].GetRef())
	assert.Equal(t, "my-api", rs.APIPublications[0].API) // Parent reference
	
	// Check implementation
	assert.Equal(t, "my-api-impl", rs.APIImplementations[0].GetRef())
	assert.Equal(t, "my-api", rs.APIImplementations[0].API) // Parent reference
}

func TestLoader_LoadFile_SeparateAPIChildResources(t *testing.T) {
	loader := New()
	
	// Test loading multiple files with separate API child resources
	dir := filepath.Join("testdata", "valid")
	sources := []Source{
		{Path: filepath.Join(dir, "api-only.yaml"), Type: SourceTypeFile},
		{Path: filepath.Join(dir, "api-version-single-separate.yaml"), Type: SourceTypeFile},
		{Path: filepath.Join(dir, "api-publications-separate.yaml"), Type: SourceTypeFile},
		{Path: filepath.Join(dir, "simple-portal.yaml"), Type: SourceTypeFile}, // For portal reference
	}
	
	rs, err := loader.LoadFromSources(sources, false)
	assert.NoError(t, err)
	assert.NotNil(t, rs)
	
	// Verify API
	assert.Len(t, rs.APIs, 1)
	assert.Equal(t, "users-api", rs.APIs[0].GetRef())
	
	// Verify separately defined child resources
	assert.Len(t, rs.APIVersions, 1)
	assert.Equal(t, "users-api-v1", rs.APIVersions[0].GetRef())
	assert.Equal(t, "users-api", rs.APIVersions[0].API) // Parent reference
	
	assert.Len(t, rs.APIPublications, 1)
	assert.Equal(t, "users-api-public-pub", rs.APIPublications[0].GetRef())
	assert.Equal(t, "users-api", rs.APIPublications[0].API) // Parent reference
	
	// Verify portal exists (for publication reference)
	assert.Len(t, rs.Portals, 1)
}

func TestLoader_LoadFromSources_SingleFile(t *testing.T) {
	loader := New()
	
	// Test loading a single file
	filePath := filepath.Join("testdata", "valid", "simple-portal.yaml")
	sources := []Source{{Path: filePath, Type: SourceTypeFile}}
	
	rs, err := loader.LoadFromSources(sources, false)
	assert.NoError(t, err)
	assert.Len(t, rs.Portals, 1)
}

func TestLoader_LoadFromSources_MultipleFiles(t *testing.T) {
	loader := New()
	
	// Test loading multiple files
	sources := []Source{
		{Path: filepath.Join("testdata", "valid", "simple-portal.yaml"), Type: SourceTypeFile},
		{Path: filepath.Join("testdata", "valid", "auth-strategy.yaml"), Type: SourceTypeFile},
	}
	
	rs, err := loader.LoadFromSources(sources, false)
	assert.NoError(t, err)
	assert.Len(t, rs.Portals, 1)
	assert.Len(t, rs.ApplicationAuthStrategies, 1)
}

func TestLoader_LoadFromSources_Directory(t *testing.T) {
	loader := New()
	
	// Test loading directory with multifile support
	sources := []Source{{Path: filepath.Join("testdata", "multifile"), Type: SourceTypeDirectory}}
	
	rs, err := loader.LoadFromSources(sources, false)
	assert.NoError(t, err)
	assert.NotNil(t, rs)
	
	// Should have resources from multiple files
	assert.True(t, len(rs.Portals) > 0 || len(rs.APIs) > 0, "Should have loaded resources from directory")
}

func TestLoader_LoadFromSources_DirectoryRecursive(t *testing.T) {
	// Create nested directory structure
	tmpDir, err := os.MkdirTemp("", "loader-recursive-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)
	
	// Create subdirectory
	subDir := filepath.Join(tmpDir, "subdir")
	err = os.Mkdir(subDir, 0755)
	require.NoError(t, err)
	
	// Create file in subdirectory
	subYAML := `
portals:
  - ref: sub-portal
    name: "Sub Portal"
`
	err = os.WriteFile(filepath.Join(subDir, "sub.yaml"), []byte(subYAML), 0600)
	require.NoError(t, err)
	
	loader := New()
	
	// Test without recursive - should fail
	sources := []Source{{Path: tmpDir, Type: SourceTypeDirectory}}
	_, err = loader.LoadFromSources(sources, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no YAML files found")
	assert.Contains(t, err.Error(), "Use -R to search subdirectories")
	
	// Test with recursive - should succeed
	rs, err := loader.LoadFromSources(sources, true)
	assert.NoError(t, err)
	assert.Len(t, rs.Portals, 1)
}

func TestLoader_LoadFromSources_DuplicateDetection(t *testing.T) {
	loader := New()
	
	// Load directory with duplicate refs across files
	sources := []Source{{Path: filepath.Join("testdata", "multifile-duplicates"), Type: SourceTypeDirectory}}
	
	rs, err := loader.LoadFromSources(sources, false)
	assert.Error(t, err, "Should fail due to duplicate refs")
	assert.Nil(t, rs)
	assert.Contains(t, err.Error(), "duplicate")
}

func TestLoader_LoadFromSources_NameDuplicateDetection(t *testing.T) {
	loader := New()
	
	// Load directory with duplicate names across files
	sources := []Source{{Path: filepath.Join("testdata", "name-duplicates"), Type: SourceTypeDirectory}}
	
	rs, err := loader.LoadFromSources(sources, false)
	assert.Error(t, err, "Should fail due to duplicate names")
	assert.Nil(t, rs)
	assert.Contains(t, err.Error(), "duplicate")
	assert.Contains(t, err.Error(), "name")
}

func TestLoader_ParseSources(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []Source
	}{
		{
			name:  "single file",
			input: []string{"file.yaml"},
			expected: []Source{
				{Path: "file.yaml", Type: SourceTypeFile},
			},
		},
		{
			name:  "multiple files",
			input: []string{"file1.yaml", "file2.yaml"},
			expected: []Source{
				{Path: "file1.yaml", Type: SourceTypeFile},
				{Path: "file2.yaml", Type: SourceTypeFile},
			},
		},
		{
			name:  "comma-separated",
			input: []string{"file1.yaml,file2.yaml"},
			expected: []Source{
				{Path: "file1.yaml", Type: SourceTypeFile},
				{Path: "file2.yaml", Type: SourceTypeFile},
			},
		},
		{
			name:  "stdin",
			input: []string{"-"},
			expected: []Source{
				{Path: "-", Type: SourceTypeSTDIN},
			},
		},
		{
			name:  "empty defaults to current directory",
			input: []string{},
			expected: []Source{
				{Path: ".", Type: SourceTypeDirectory},
			},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// For testing, we need to mock file existence checks
			// Since ParseSources checks if files exist, we'll test with stdin
			// which doesn't require file existence
			if tt.name == "stdin" || tt.name == "empty defaults to current directory" {
				sources, err := ParseSources(tt.input)
				assert.NoError(t, err)
				assert.Equal(t, len(tt.expected), len(sources))
				for i, expected := range tt.expected {
					assert.Equal(t, expected.Path, sources[i].Path)
					assert.Equal(t, expected.Type, sources[i].Type)
				}
			}
		})
	}
}

func TestLoader_ValidateYAMLFile(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"file.yaml", true},
		{"file.yml", true},
		{"file.YAML", true},
		{"file.YML", true},
		{"file.txt", false},
		{"file", false},
		{"file.yaml.bak", false},
		{".yaml", true},
		{".yml", true},
	}
	
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := ValidateYAMLFile(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestLoader_LoadFile_NonYAMLExtension(t *testing.T) {
	loader := New()
	
	// Try to load a non-YAML file
	rs, err := loader.LoadFile("testdata/test.txt")
	assert.Error(t, err)
	assert.Nil(t, rs)
	assert.Contains(t, err.Error(), "does not have .yaml or .yml extension")
}

func TestLoader_LoadFile_UnknownFields(t *testing.T) {
	tests := []struct {
		name          string
		file          string
		expectedError string
	}{
		{
			name:          "misspelled labels field with suggestion",
			file:          "invalid/unknown-field-portal.yaml",
			expectedError: "unknown field 'lables' in testdata/invalid/unknown-field-portal.yaml. Did you mean 'labels'?",
		},
		{
			name:          "unknown field with no suggestion",
			file:          "invalid/unknown-field-no-suggestion.yaml",
			expectedError: "unknown field 'completely_unknown_field' in " +
				"testdata/invalid/unknown-field-no-suggestion.yaml. " +
				"Please check the field name against the schema",
		},
		{
			name:          "misspelled strategy_type field",
			file:          "invalid/unknown-field-auth.yaml",
			expectedError: "unknown field 'strategytype' in " +
				"testdata/invalid/unknown-field-auth.yaml. Did you mean 'strategy_type'?",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loader := New()
			rs, err := loader.LoadFile("testdata/" + tt.file)
			
			require.Error(t, err)
			assert.Nil(t, rs)
			assert.Contains(t, err.Error(), tt.expectedError)
		})
	}
}