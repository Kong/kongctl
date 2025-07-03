//go:build integration
// +build integration

package declarative_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kong/kongctl/internal/declarative/loader"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileTagLoader_BasicFileLoading(t *testing.T) {
	// Create test configuration directory
	configDir := t.TempDir()
	
	// Create external content file
	externalContent := `
description: "This content was loaded from an external file"
version: "1.0.0"
metadata:
  environment: production
  team: platform
`
	externalFile := filepath.Join(configDir, "external.yaml")
	require.NoError(t, os.WriteFile(externalFile, []byte(externalContent), 0600))
	
	// Create main configuration file with file tags
	config := `
portals:
  - ref: test-portal
    name: "Test Portal"
    description: !file ./external.yaml#description
    display_name: !file ./external.yaml#version
`
	configFile := filepath.Join(configDir, "portal.yaml")
	require.NoError(t, os.WriteFile(configFile, []byte(config), 0600))
	
	// Load configuration using the loader
	ldr := loader.New()
	sources, err := loader.ParseSources([]string{configFile})
	require.NoError(t, err)
	
	resourceSet, err := ldr.LoadFromSources(sources, false)
	require.NoError(t, err)
	
	// Verify the portal was loaded with file tag values resolved
	require.Len(t, resourceSet.Portals, 1)
	portal := resourceSet.Portals[0]
	
	assert.Equal(t, "test-portal", portal.Ref)
	assert.Equal(t, "Test Portal", portal.Name)
	assert.Equal(t, "This content was loaded from an external file", *portal.Description)
	assert.Equal(t, "1.0.0", *portal.DisplayName)
}

func TestFileTagLoader_NestedDirectoryLoading(t *testing.T) {
	// Create test configuration directory structure
	configDir := t.TempDir()
	subDir := filepath.Join(configDir, "subdir")
	require.NoError(t, os.MkdirAll(subDir, 0755))
	
	// Create external content file in subdirectory
	externalContent := `
api_spec: |
  openapi: 3.0.0
  info:
    title: Test API
    version: 1.0.0
  paths: {}
metadata:
  environment: production
  team: backend
`
	externalFile := filepath.Join(subDir, "api-spec.yaml")
	require.NoError(t, os.WriteFile(externalFile, []byte(externalContent), 0600))
	
	// Create API configuration file in subdirectory with relative file reference
	config := `
apis:
  - ref: test-api
    name: "Test API"
    description: !file ./api-spec.yaml#metadata.environment
`
	configFile := filepath.Join(subDir, "api.yaml")
	require.NoError(t, os.WriteFile(configFile, []byte(config), 0600))
	
	// Load configuration using the loader
	ldr := loader.New()
	sources, err := loader.ParseSources([]string{configFile})
	require.NoError(t, err)
	
	resourceSet, err := ldr.LoadFromSources(sources, false)
	require.NoError(t, err)
	
	// Verify the API was loaded with file tag values resolved from correct relative path
	require.Len(t, resourceSet.APIs, 1)
	api := resourceSet.APIs[0]
	
	assert.Equal(t, "test-api", api.Ref)
	assert.Equal(t, "Test API", api.Name)
	assert.Equal(t, "production", *api.Description)
}

func TestFileTagLoader_RecursiveDirectoryLoading(t *testing.T) {
	// Create test configuration directory structure
	configDir := t.TempDir()
	subDir := filepath.Join(configDir, "apis")
	require.NoError(t, os.MkdirAll(subDir, 0755))
	
	// Create external spec file
	specContent := `
title: "External API Spec"
version: "2.1.0"
description: "This is loaded from an external specification file"
owner: "platform-team"
`
	specFile := filepath.Join(subDir, "spec.yaml")
	require.NoError(t, os.WriteFile(specFile, []byte(specContent), 0600))
	
	// Create portal config in root
	portalConfig := `
portals:
  - ref: main-portal
    name: "Main Portal"
    description: "Main portal for APIs"
`
	portalFile := filepath.Join(configDir, "portal.yaml")
	require.NoError(t, os.WriteFile(portalFile, []byte(portalConfig), 0600))
	
	// Create API config in subdirectory with file tag
	apiConfig := `
apis:
  - ref: external-api
    name: !file ./spec.yaml#title
    description: !file ./spec.yaml#description
`
	apiFile := filepath.Join(subDir, "api.yaml")
	require.NoError(t, os.WriteFile(apiFile, []byte(apiConfig), 0600))
	
	// Load configuration using the loader recursively from root directory
	ldr := loader.New()
	sources, err := loader.ParseSources([]string{configDir})
	require.NoError(t, err)
	
	resourceSet, err := ldr.LoadFromSources(sources, true) // recursive = true
	require.NoError(t, err)
	
	// Should have 1 portal + 1 API
	require.Len(t, resourceSet.Portals, 1)
	require.Len(t, resourceSet.APIs, 1)
	
	// Verify portal
	portal := resourceSet.Portals[0]
	assert.Equal(t, "main-portal", portal.Ref)
	assert.Equal(t, "Main Portal", portal.Name)
	
	// Verify API with file tags resolved correctly
	api := resourceSet.APIs[0]
	assert.Equal(t, "external-api", api.Ref)
	assert.Equal(t, "External API Spec", api.Name)
	assert.Equal(t, "This is loaded from an external specification file", *api.Description)
}

func TestFileTagLoader_ComplexExtraction(t *testing.T) {
	// Create test configuration directory
	configDir := t.TempDir()
	
	// Create external data file with nested structure
	externalData := `
portal:
  metadata:
    name: "Complex Portal"
    description: "Portal with complex metadata"
    labels:
      env: staging
      team: frontend
  settings:
    auth:
      enabled: true
      provider: oauth2
    branding:
      theme: dark
      logo_url: "https://example.com/logo.png"
`
	externalFile := filepath.Join(configDir, "portal-data.yaml")
	require.NoError(t, os.WriteFile(externalFile, []byte(externalData), 0600))
	
	// Create configuration with complex extractions
	config := `
portals:
  - ref: complex-portal
    name: !file ./portal-data.yaml#portal.metadata.name
    description: !file ./portal-data.yaml#portal.metadata.description
    display_name: !file ./portal-data.yaml#portal.settings.branding.theme
`
	configFile := filepath.Join(configDir, "complex.yaml")
	require.NoError(t, os.WriteFile(configFile, []byte(config), 0600))
	
	// Load configuration
	ldr := loader.New()
	sources, err := loader.ParseSources([]string{configFile})
	require.NoError(t, err)
	
	resourceSet, err := ldr.LoadFromSources(sources, false)
	require.NoError(t, err)
	
	// Verify complex nested value extraction
	require.Len(t, resourceSet.Portals, 1)
	portal := resourceSet.Portals[0]
	
	assert.Equal(t, "complex-portal", portal.Ref)
	assert.Equal(t, "Complex Portal", portal.Name)
	assert.Equal(t, "Portal with complex metadata", *portal.Description)
	assert.Equal(t, "dark", *portal.DisplayName)
}

func TestFileTagLoader_LoadPlainContent(t *testing.T) {
	// Create test configuration directory
	configDir := t.TempDir()
	
	// Create plain text file
	textContent := "This is plain text content without YAML structure"
	textFile := filepath.Join(configDir, "plain.txt")
	require.NoError(t, os.WriteFile(textFile, []byte(textContent), 0600))
	
	// Create configuration that loads entire file content
	config := `
apis:
  - ref: text-api
    name: "Text API"
    description: !file ./plain.txt
`
	configFile := filepath.Join(configDir, "text-api.yaml")
	require.NoError(t, os.WriteFile(configFile, []byte(config), 0600))
	
	// Load configuration
	ldr := loader.New()
	sources, err := loader.ParseSources([]string{configFile})
	require.NoError(t, err)
	
	resourceSet, err := ldr.LoadFromSources(sources, false)
	require.NoError(t, err)
	
	// Verify plain text was loaded
	require.Len(t, resourceSet.APIs, 1)
	api := resourceSet.APIs[0]
	
	assert.Equal(t, "text-api", api.Ref)
	assert.Equal(t, "Text API", api.Name)
	assert.Equal(t, textContent, *api.Description)
}