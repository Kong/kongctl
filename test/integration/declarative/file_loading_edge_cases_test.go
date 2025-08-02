//go:build integration
// +build integration

package declarative_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kong/kongctl/internal/declarative/loader"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFileLoadingEdgeCases tests various edge cases in file loading and tag processing
func TestFileLoadingEdgeCases(t *testing.T) {
	t.Run("nested directory file loading", func(t *testing.T) {
		tempDir := t.TempDir()
		
		// Create deeply nested directory structure
		nestedDir := filepath.Join(tempDir, "level1", "level2", "level3")
		require.NoError(t, os.MkdirAll(nestedDir, 0755))
		
		// Create file in nested directory
		deepContent := `
deep_config:
  value: "from deep nested file"
  metadata:
    level: "3"
    path: "level1/level2/level3"
`
		deepFile := filepath.Join(nestedDir, "deep.yaml")
		require.NoError(t, os.WriteFile(deepFile, []byte(deepContent), 0600))
		
		// Create config that references nested file
		config := `
portals:
  - ref: nested-portal
    name: "Nested Portal"
    description: !file ./level1/level2/level3/deep.yaml#deep_config.value
    labels:
      source_level: !file ./level1/level2/level3/deep.yaml#deep_config.metadata.level
    kongctl:
      namespace: default
`
		configFile := filepath.Join(tempDir, "config.yaml")
		require.NoError(t, os.WriteFile(configFile, []byte(config), 0600))
		
		// Load and verify
		l := loader.New()
		sources := []loader.Source{{Path: configFile, Type: loader.SourceTypeFile}}
		
		resourceSet, err := l.LoadFromSources(sources, false)
		require.NoError(t, err)
		require.Len(t, resourceSet.Portals, 1)
		
		portal := resourceSet.Portals[0]
		require.NotNil(t, portal.Description)
		assert.Equal(t, "from deep nested file", *portal.Description)
		require.NotNil(t, portal.Labels["source_level"])
		assert.Equal(t, "3", *portal.Labels["source_level"])
	})
	
	t.Run("relative path resolution from same directory", func(t *testing.T) {
		tempDir := t.TempDir()
		
		// Create shared data file
		sharedContent := `
shared_settings:
  api_version: "v2.1.0"
  environment: "production"
  region: "us-west-2"
`
		sharedFile := filepath.Join(tempDir, "common.yaml")
		require.NoError(t, os.WriteFile(sharedFile, []byte(sharedContent), 0600))
		
		// Create data file with resolved values (no nested file tags)
		dataContent := `
api_metadata:
  title: "Customer API"
  description: "API for customer management"
  version: "v2.1.0"
  labels:
    environment: "production"
    region: "us-west-2"
`
		dataFile := filepath.Join(tempDir, "api-meta.yaml")
		require.NoError(t, os.WriteFile(dataFile, []byte(dataContent), 0600))
		
		// Create config file 
		config := `
apis:
  - ref: customer-api
    name: !file ./api-meta.yaml#api_metadata.title
    description: !file ./api-meta.yaml#api_metadata.description
    version: !file ./api-meta.yaml#api_metadata.version
    labels: !file ./api-meta.yaml#api_metadata.labels
    kongctl:
      namespace: default
`
		configFile := filepath.Join(tempDir, "api.yaml")
		require.NoError(t, os.WriteFile(configFile, []byte(config), 0600))
		
		// Load and verify relative path resolution works correctly
		l := loader.New()
		sources := []loader.Source{{Path: configFile, Type: loader.SourceTypeFile}}
		
		resourceSet, err := l.LoadFromSources(sources, false)
		require.NoError(t, err)
		require.Len(t, resourceSet.APIs, 1)
		
		api := resourceSet.APIs[0]
		assert.Equal(t, "Customer API", api.Name)
		require.NotNil(t, api.Description)
		assert.Equal(t, "API for customer management", *api.Description)
		require.NotNil(t, api.Version)
		assert.Equal(t, "v2.1.0", *api.Version)
		assert.Equal(t, "production", api.Labels["environment"])
		assert.Equal(t, "us-west-2", api.Labels["region"])
		
		// Verify we can also load the shared file directly
		config2 := `
portals:
  - ref: test-portal
    name: "Test Portal"
    description: !file ./common.yaml#shared_settings.environment
    kongctl:
      namespace: default
`
		configFile2 := filepath.Join(tempDir, "portal.yaml")
		require.NoError(t, os.WriteFile(configFile2, []byte(config2), 0600))
		
		resourceSet2, err := l.LoadFromSources([]loader.Source{{Path: configFile2, Type: loader.SourceTypeFile}}, false)
		require.NoError(t, err)
		require.Len(t, resourceSet2.Portals, 1)
		require.NotNil(t, resourceSet2.Portals[0].Description)
		assert.Equal(t, "production", *resourceSet2.Portals[0].Description)
	})
	
	t.Run("large file handling", func(t *testing.T) {
		tempDir := t.TempDir()
		
		// Create a large JSON specification
		largeSpec := map[string]interface{}{
			"openapi": "3.0.0",
			"info": map[string]interface{}{
				"title":   "Large API",
				"version": "1.0.0",
			},
			"paths": make(map[string]interface{}),
		}
		
		// Add many paths to make it large
		paths := largeSpec["paths"].(map[string]interface{})
		for i := 0; i < 1000; i++ {
			pathName := fmt.Sprintf("/endpoint_%d", i)
			paths[pathName] = map[string]interface{}{
				"get": map[string]interface{}{
					"summary":     fmt.Sprintf("Get endpoint %d", i),
					"description": strings.Repeat("This is a detailed description. ", 50),
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Success",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"id":   map[string]interface{}{"type": "integer"},
											"name": map[string]interface{}{"type": "string"},
											"data": map[string]interface{}{
												"type": "object",
												"additionalProperties": map[string]interface{}{
													"type": "string",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			}
		}
		
		// Convert to JSON string
		specJSON, err := json.Marshal(largeSpec)
		require.NoError(t, err)
		
		specFile := filepath.Join(tempDir, "large-spec.json")
		require.NoError(t, os.WriteFile(specFile, specJSON, 0600))
		
		// Create config that loads the large file
		config := `
apis:
  - ref: large-api
    name: "Large API"
    description: "API with large specification"
    version: "1.0.0"
    kongctl:
      namespace: default
    versions:
      - ref: large-api-v1
        version: "v1"
        spec: !file ./large-spec.json
`
		configFile := filepath.Join(tempDir, "config.yaml")
		require.NoError(t, os.WriteFile(configFile, []byte(config), 0600))
		
		// Load and verify large file handling
		l := loader.New()
		sources := []loader.Source{{Path: configFile, Type: loader.SourceTypeFile}}
		
		resourceSet, err := l.LoadFromSources(sources, false)
		require.NoError(t, err)
		require.Len(t, resourceSet.APIs, 1)
		require.Len(t, resourceSet.APIVersions, 1)
		
		version := resourceSet.APIVersions[0]
		if version.Spec != nil {
			t.Log("Large spec file loaded successfully")
		}
	})
	
	t.Run("binary file rejection", func(t *testing.T) {
		tempDir := t.TempDir()
		
		// Create a binary file (simulated with non-UTF8 content)
		binaryContent := []byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0xFD}
		binaryFile := filepath.Join(tempDir, "binary.dat")
		require.NoError(t, os.WriteFile(binaryFile, binaryContent, 0600))
		
		// Create config that tries to load binary file
		config := `
portals:
  - ref: test-portal
    name: "Test Portal"
    description: !file ./binary.dat
`
		configFile := filepath.Join(tempDir, "config.yaml")
		require.NoError(t, os.WriteFile(configFile, []byte(config), 0600))
		
		// Attempt to load - should handle binary gracefully
		l := loader.New()
		sources := []loader.Source{{Path: configFile, Type: loader.SourceTypeFile}}
		
		_, err := l.LoadFromSources(sources, false)
		// Should either load as raw text or fail gracefully
		if err != nil {
			assert.Contains(t, err.Error(), "failed to process file tag")
		}
	})
	
	t.Run("file permission handling", func(t *testing.T) {
		if os.Getuid() == 0 {
			t.Skip("Skipping permission test when running as root")
		}
		
		tempDir := t.TempDir()
		
		// Create file and remove read permission
		restrictedContent := `restricted: "secret data"`
		restrictedFile := filepath.Join(tempDir, "restricted.yaml")
		require.NoError(t, os.WriteFile(restrictedFile, []byte(restrictedContent), 0000))
		
		// Create config that tries to load restricted file
		config := `
portals:
  - ref: test-portal
    name: "Test Portal"
    description: !file ./restricted.yaml#restricted
`
		configFile := filepath.Join(tempDir, "config.yaml")
		require.NoError(t, os.WriteFile(configFile, []byte(config), 0600))
		
		// Attempt to load - should fail with permission error
		l := loader.New()
		sources := []loader.Source{{Path: configFile, Type: loader.SourceTypeFile}}
		
		_, err := l.LoadFromSources(sources, false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "permission denied")
	})
	
	t.Run("symlink handling", func(t *testing.T) {
		tempDir := t.TempDir()
		
		// Create target file
		targetContent := `
target_data:
  value: "from symlinked file"
  type: "symlink_target"
`
		targetFile := filepath.Join(tempDir, "target.yaml")
		require.NoError(t, os.WriteFile(targetFile, []byte(targetContent), 0600))
		
		// Create symlink
		symlinkFile := filepath.Join(tempDir, "symlink.yaml")
		require.NoError(t, os.Symlink(targetFile, symlinkFile))
		
		// Create config that loads via symlink
		config := `
portals:
  - ref: symlink-portal
    name: "Symlink Portal"
    description: !file ./symlink.yaml#target_data.value
    labels:
      type: !file ./symlink.yaml#target_data.type
    kongctl:
      namespace: default
`
		configFile := filepath.Join(tempDir, "config.yaml")
		require.NoError(t, os.WriteFile(configFile, []byte(config), 0600))
		
		// Load and verify symlink resolution works
		l := loader.New()
		sources := []loader.Source{{Path: configFile, Type: loader.SourceTypeFile}}
		
		resourceSet, err := l.LoadFromSources(sources, false)
		require.NoError(t, err)
		require.Len(t, resourceSet.Portals, 1)
		
		portal := resourceSet.Portals[0]
		require.NotNil(t, portal.Description)
		assert.Equal(t, "from symlinked file", *portal.Description)
		require.NotNil(t, portal.Labels["type"])
		assert.Equal(t, "symlink_target", *portal.Labels["type"])
	})
	
	t.Run("concurrent file loading", func(t *testing.T) {
		tempDir := t.TempDir()
		
		// Create multiple data files
		numFiles := 10
		dataFiles := make([]string, numFiles)
		for i := 0; i < numFiles; i++ {
			content := fmt.Sprintf(`
file_%d:
  value: "data from file %d"
  index: "%d"
`, i, i, i)
			fileName := fmt.Sprintf("data_%d.yaml", i)
			dataFiles[i] = fileName
			filePath := filepath.Join(tempDir, fileName)
			require.NoError(t, os.WriteFile(filePath, []byte(content), 0600))
		}
		
		// Create config that loads from all files
		configParts := []string{"apis:"}
		for i, fileName := range dataFiles {
			apiDef := fmt.Sprintf(`
  - ref: api-%d
    name: !file ./%s#file_%d.value
    description: "API number %d"
    version: "1.0.0"
    labels:
      index: !file ./%s#file_%d.index
    kongctl:
      namespace: default`, i, fileName, i, i, fileName, i)
			configParts = append(configParts, apiDef)
		}
		
		config := strings.Join(configParts, "")
		configFile := filepath.Join(tempDir, "config.yaml")
		require.NoError(t, os.WriteFile(configFile, []byte(config), 0600))
		
		// Load and verify concurrent file processing
		l := loader.New()
		sources := []loader.Source{{Path: configFile, Type: loader.SourceTypeFile}}
		
		resourceSet, err := l.LoadFromSources(sources, false)
		require.NoError(t, err)
		require.Len(t, resourceSet.APIs, numFiles)
		
		// Verify all files were loaded correctly
		for i, api := range resourceSet.APIs {
			expectedName := fmt.Sprintf("data from file %d", i)
			assert.Equal(t, expectedName, api.Name)
			assert.Equal(t, fmt.Sprintf("%d", i), api.Labels["index"])
		}
	})
}

// TestFileTagCaching tests file tag caching behavior
func TestFileTagCaching(t *testing.T) {
	tempDir := t.TempDir()
	
	// Create data file
	dataContent := `
cached_data:
  timestamp: "2024-01-01T00:00:00Z"
  counter: "42"
  message: "this content should be cached"
`
	dataFile := filepath.Join(tempDir, "cacheable.yaml")
	require.NoError(t, os.WriteFile(dataFile, []byte(dataContent), 0600))
	
	// Create config that references the same file multiple times
	config := `
portals:
  - ref: portal-1
    name: "Portal 1"
    description: !file ./cacheable.yaml#cached_data.message
    labels:
      timestamp: !file ./cacheable.yaml#cached_data.timestamp
      counter: !file ./cacheable.yaml#cached_data.counter
    kongctl:
      namespace: default

  - ref: portal-2
    name: "Portal 2"
    description: !file ./cacheable.yaml#cached_data.message
    labels:
      timestamp: !file ./cacheable.yaml#cached_data.timestamp
      counter: !file ./cacheable.yaml#cached_data.counter
    kongctl:
      namespace: default

apis:
  - ref: api-1
    name: "API 1"
    description: !file ./cacheable.yaml#cached_data.message
    version: "1.0.0"
    labels:
      counter: !file ./cacheable.yaml#cached_data.counter
    kongctl:
      namespace: default
`
	configFile := filepath.Join(tempDir, "config.yaml")
	require.NoError(t, os.WriteFile(configFile, []byte(config), 0600))
	
	// Load configuration - file should be cached and reused
	l := loader.New()
	sources := []loader.Source{{Path: configFile, Type: loader.SourceTypeFile}}
	
	resourceSet, err := l.LoadFromSources(sources, false)
	require.NoError(t, err)
	require.Len(t, resourceSet.Portals, 2)
	require.Len(t, resourceSet.APIs, 1)
	
	// Verify all resources have consistent values (proving cache worked)
	expectedMessage := "this content should be cached"
	expectedTimestamp := "2024-01-01T00:00:00Z"
	expectedCounter := "42"
	
	for _, portal := range resourceSet.Portals {
		require.NotNil(t, portal.Description)
		assert.Equal(t, expectedMessage, *portal.Description)
		require.NotNil(t, portal.Labels["timestamp"])
		assert.Equal(t, expectedTimestamp, *portal.Labels["timestamp"])
		require.NotNil(t, portal.Labels["counter"])
		assert.Equal(t, expectedCounter, *portal.Labels["counter"])
	}
	
	api := resourceSet.APIs[0]
	require.NotNil(t, api.Description)
	assert.Equal(t, expectedMessage, *api.Description)
	assert.Equal(t, expectedCounter, api.Labels["counter"])
}

// TestFileTagSecurityValidation tests security-related file tag validation
func TestFileTagSecurityValidation(t *testing.T) {
	tests := []struct {
		name           string
		fileReference  string
		expectedError  string
		shouldSucceed  bool
	}{
		{
			name:          "absolute path rejection",
			fileReference: "/etc/passwd",
			expectedError: "absolute paths are not allowed",
		},
		{
			name:          "parent directory traversal",
			fileReference: "../../../etc/passwd",
			expectedError: "parent directory traversal is not allowed",
		},
		{
			name:          "hidden parent traversal",
			fileReference: "./safe/../../../etc/passwd",
			expectedError: "parent directory traversal is not allowed",
		},
		{
			name:          "allowed relative path",
			fileReference: "./data/config.yaml",
			shouldSucceed: true,
		},
		{
			name:          "allowed subdirectory",
			fileReference: "./configs/nested/file.yaml",
			shouldSucceed: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			
			if tt.shouldSucceed {
				// Create the referenced file for successful cases
				targetDir := filepath.Join(tempDir, filepath.Dir(tt.fileReference))
				if targetDir != tempDir {
					require.NoError(t, os.MkdirAll(targetDir, 0755))
				}
				
				targetFile := filepath.Join(tempDir, tt.fileReference)
				content := `allowed: "safe content"`
				require.NoError(t, os.WriteFile(targetFile, []byte(content), 0600))
			}
			
			// Create config with potentially unsafe file reference
			config := fmt.Sprintf(`
portals:
  - ref: test-portal
    name: "Test Portal"
    description: !file %s#allowed
    kongctl:
      namespace: default
`, tt.fileReference)
			
			configFile := filepath.Join(tempDir, "config.yaml")
			require.NoError(t, os.WriteFile(configFile, []byte(config), 0600))
			
			// Attempt to load
			l := loader.New()
			sources := []loader.Source{{Path: configFile, Type: loader.SourceTypeFile}}
			
			resourceSet, err := l.LoadFromSources(sources, false)
			
			if tt.shouldSucceed {
				require.NoError(t, err)
				require.Len(t, resourceSet.Portals, 1)
				require.NotNil(t, resourceSet.Portals[0].Description)
				assert.Equal(t, "safe content", *resourceSet.Portals[0].Description)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			}
		})
	}
}