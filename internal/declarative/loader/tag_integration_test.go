package loader

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoader_TagProcessing(t *testing.T) {
	// Create test directory
	tmpDir, err := os.MkdirTemp("", "loader-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create the referenced file
	portalNameContent := "test-value-from-file"
	portalFile := filepath.Join(tmpDir, "portal-name.txt")
	require.NoError(t, os.WriteFile(portalFile, []byte(portalNameContent), 0o600))

	// Test YAML with a !file tag
	yamlContent := `
portals:
  - ref: test-portal
    name: !file portal-name.txt`

	tmpfile := filepath.Join(tmpDir, "test.yaml")
	err = os.WriteFile(tmpfile, []byte(yamlContent), 0o600)
	require.NoError(t, err)

	// Create a loader
	loader := NewWithBaseDir(tmpDir)

	// Load the file
	rs, err := loader.LoadFile(tmpfile)
	assert.NoError(t, err)
	assert.NotNil(t, rs)

	// Verify the tag was processed
	assert.Len(t, rs.Portals, 1)
	assert.Equal(t, "test-portal", rs.Portals[0].Ref)
	assert.Equal(t, "test-value-from-file", rs.Portals[0].Name)
}

func TestLoader_WithoutFileTags(t *testing.T) {
	// Test YAML without any custom tags
	yamlContent := `
portals:
  - ref: test-portal
    name: Regular Name`

	tmpDir, err := os.MkdirTemp("", "loader-test2-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	tmpfile := filepath.Join(tmpDir, "test.yaml")
	err = os.WriteFile(tmpfile, []byte(yamlContent), 0o600)
	require.NoError(t, err)

	// Create a loader (file resolver is registered by default)
	loader := NewWithBaseDir(tmpDir)

	// Load should work normally
	rs, err := loader.LoadFile(tmpfile)
	assert.NoError(t, err)
	assert.NotNil(t, rs)
	assert.Len(t, rs.Portals, 1)
	assert.Equal(t, "Regular Name", rs.Portals[0].Name)
}

func TestLoader_FileTagIntegration(t *testing.T) {
	// Create test directory structure
	tmpDir, err := os.MkdirTemp("", "loader-file-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create an OpenAPI spec file
	openapiContent := `
openapi: 3.0.0
info:
  title: Test API
  version: 1.0.0
  description: A test API for file loading
  contact:
    email: test@example.com`

	specFile := filepath.Join(tmpDir, "api-spec.yaml")
	require.NoError(t, os.WriteFile(specFile, []byte(openapiContent), 0o600))

	// Create a text description file
	descContent := "This is a comprehensive API for managing resources."
	descFile := filepath.Join(tmpDir, "description.txt")
	require.NoError(t, os.WriteFile(descFile, []byte(descContent), 0o600))

	// Create main configuration with file tags
	mainContent := `
apis:
  - ref: test-api
    name: !file
      path: api-spec.yaml
      extract: info.title
    version: !file
      path: api-spec.yaml
      extract: info.version
    description: !file description.txt`

	mainFile := filepath.Join(tmpDir, "main.yaml")
	require.NoError(t, os.WriteFile(mainFile, []byte(mainContent), 0o600))

	// Load the configuration
	loader := NewWithBaseDir(tmpDir)
	rs, err := loader.LoadFile(mainFile)
	require.NoError(t, err)
	require.NotNil(t, rs)

	// Verify the file tags were resolved correctly
	require.Len(t, rs.APIs, 1)
	api := rs.APIs[0]

	assert.Equal(t, "test-api", api.Ref)
	assert.Equal(t, "Test API", api.Name)
	ptrStr := func(s string) *string { return &s }
	assert.Equal(t, ptrStr("1.0.0"), api.Version)
	assert.Equal(t, ptrStr("This is a comprehensive API for managing resources."), api.Description)
}

func TestLoader_FileTagWithNestedFiles(t *testing.T) {
	// Create test directory structure
	tmpDir, err := os.MkdirTemp("", "loader-nested-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create nested directory
	nestedDir := filepath.Join(tmpDir, "configs")
	require.NoError(t, os.MkdirAll(nestedDir, 0o755))

	// Create a config file in nested directory
	configContent := `
name: Production Portal
display_name: Production Developer Portal
authentication_enabled: true
settings:
  theme: dark
  language: en-US`

	configFile := filepath.Join(nestedDir, "portal-config.yaml")
	require.NoError(t, os.WriteFile(configFile, []byte(configContent), 0o600))

	// Create main configuration referencing nested file
	mainContent := `
portals:
  - ref: prod-portal
    name: !file
      path: configs/portal-config.yaml
      extract: name
    display_name: !file
      path: configs/portal-config.yaml
      extract: display_name
    authentication_enabled: !file
      path: configs/portal-config.yaml
      extract: authentication_enabled`

	mainFile := filepath.Join(tmpDir, "portals.yaml")
	require.NoError(t, os.WriteFile(mainFile, []byte(mainContent), 0o600))

	// Load the configuration
	loader := NewWithBaseDir(tmpDir)
	rs, err := loader.LoadFile(mainFile)
	require.NoError(t, err)
	require.NotNil(t, rs)

	// Verify the nested file tags were resolved
	require.Len(t, rs.Portals, 1)
	portal := rs.Portals[0]

	assert.Equal(t, "prod-portal", portal.Ref)
	assert.Equal(t, "Production Portal", portal.Name)
	ptrStr := func(s string) *string { return &s }
	assert.Equal(t, ptrStr("Production Developer Portal"), portal.DisplayName)
	ptrBool := func(b bool) *bool { return &b }
	assert.Equal(t, ptrBool(true), portal.AuthenticationEnabled)
}

func TestLoader_FileTagErrorHandling(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "loader-error-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name        string
		yamlContent string
		wantErr     string
	}{
		{
			name: "File not found",
			yamlContent: `
portals:
  - ref: test
    name: !file missing-file.yaml`,
			wantErr: "file not found",
		},
		{
			name: "Invalid extraction path",
			yamlContent: `
portals:
  - ref: test
    name: !file
      path: ../../../etc/passwd
      extract: whatever`,
			wantErr: "path resolves outside base dir",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testFile := filepath.Join(tmpDir, "test.yaml")
			require.NoError(t, os.WriteFile(testFile, []byte(tt.yamlContent), 0o600))

			loader := NewWithBaseDir(tmpDir)
			_, err := loader.LoadFile(testFile)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}
