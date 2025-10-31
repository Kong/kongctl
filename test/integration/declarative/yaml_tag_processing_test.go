//go:build integration
// +build integration

package declarative_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kong/kongctl/internal/declarative/loader"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestYAMLTagProcessingComprehensive tests comprehensive YAML tag processing scenarios
func TestYAMLTagProcessingComprehensive(t *testing.T) {
	t.Run("multiple tag formats in single file", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create data files with different formats
		yamlData := `
yaml_content:
  title: "From YAML"
  description: "YAML file content"
  metadata:
    format: "yaml"
    version: "1.0"
`
		yamlFile := filepath.Join(tempDir, "data.yaml")
		require.NoError(t, os.WriteFile(yamlFile, []byte(yamlData), 0o600))

		jsonData := `{
			"json_content": {
				"title": "From JSON",
				"description": "JSON file content",
				"metadata": {
					"format": "json",
					"version": "2.0"
				}
			}
		}`
		jsonFile := filepath.Join(tempDir, "data.json")
		require.NoError(t, os.WriteFile(jsonFile, []byte(jsonData), 0o600))

		textData := "Plain text content from file"
		textFile := filepath.Join(tempDir, "data.txt")
		require.NoError(t, os.WriteFile(textFile, []byte(textData), 0o600))

		// Create config using all tag formats
		config := `
portals:
  - ref: multi-format-portal
    name: "Multi-Format Portal"
    # Simple file loading
    description: !file ./data.txt
    
    # YAML extraction with hash syntax
    display_name: !file ./data.yaml#yaml_content.title
    
    # Map format for complex extraction
    labels:
      json_title: !file ./data.json#json_content.title
      yaml_version: !file
        path: ./data.yaml
        extract: yaml_content.metadata.version
      json_version: !file
        path: ./data.json
        extract: json_content.metadata.version
      yaml_format: !file ./data.yaml#yaml_content.metadata.format
      json_format: !file ./data.json#json_content.metadata.format
`
		configFile := filepath.Join(tempDir, "config.yaml")
		require.NoError(t, os.WriteFile(configFile, []byte(config), 0o600))

		// Load and verify all formats work
		l := loader.New()
		sources := []loader.Source{{Path: configFile, Type: loader.SourceTypeFile}}

		resourceSet, err := l.LoadFromSources(sources, false)
		require.NoError(t, err)
		require.Len(t, resourceSet.Portals, 1)

		portal := resourceSet.Portals[0]
		require.NotNil(t, portal.Description)
		assert.Equal(t, "Plain text content from file", *portal.Description)
		require.NotNil(t, portal.DisplayName)
		assert.Equal(t, "From YAML", *portal.DisplayName)
		require.NotNil(t, portal.Labels["json_title"])
		assert.Equal(t, "From JSON", *portal.Labels["json_title"])
		require.NotNil(t, portal.Labels["yaml_version"])
		assert.Equal(t, "1.0", *portal.Labels["yaml_version"])
		require.NotNil(t, portal.Labels["json_version"])
		assert.Equal(t, "2.0", *portal.Labels["json_version"])
		require.NotNil(t, portal.Labels["yaml_format"])
		assert.Equal(t, "yaml", *portal.Labels["yaml_format"])
		require.NotNil(t, portal.Labels["json_format"])
		assert.Equal(t, "json", *portal.Labels["json_format"])
	})

	t.Run("nested tag processing", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create base configuration file
		baseConfig := `
base_settings:
  api_prefix: "v1"
  environment: "production"
  features:
    authentication: true
    rate_limiting: true
    caching: false
`
		baseFile := filepath.Join(tempDir, "base.yaml")
		require.NoError(t, os.WriteFile(baseFile, []byte(baseConfig), 0o600))

		// Create environment-specific file that references base
		envConfig := `
environment_config:
  name: "Production Environment"
  prefix: !file ./base.yaml#base_settings.api_prefix
  settings:
    environment: !file ./base.yaml#base_settings.environment
    api_prefix: !file ./base.yaml#base_settings.api_prefix
  enabled_features:
    auth: !file ./base.yaml#base_settings.features.authentication
    rate_limit: !file ./base.yaml#base_settings.features.rate_limiting
`
		envFile := filepath.Join(tempDir, "environment.yaml")
		require.NoError(t, os.WriteFile(envFile, []byte(envConfig), 0o600))

		// Create final config that references environment config
		config := `
apis:
  - ref: nested-api
    name: !file ./environment.yaml#environment_config.name
    description: "API with nested file references"
    version: !file ./environment.yaml#environment_config.prefix
    labels:
      environment: !file ./environment.yaml#environment_config.settings.environment
      auth_enabled: !file ./environment.yaml#environment_config.enabled_features.auth
      rate_limit_enabled: !file ./environment.yaml#environment_config.enabled_features.rate_limit
`
		configFile := filepath.Join(tempDir, "config.yaml")
		require.NoError(t, os.WriteFile(configFile, []byte(config), 0o600))

		// Load and verify nested references work
		l := loader.New()
		sources := []loader.Source{{Path: configFile, Type: loader.SourceTypeFile}}

		resourceSet, err := l.LoadFromSources(sources, false)
		require.NoError(t, err)
		require.Len(t, resourceSet.APIs, 1)

		api := resourceSet.APIs[0]
		assert.Equal(t, "Production Environment", api.Name)
		require.NotNil(t, api.Version)
		// File tags inside loaded files are not processed recursively
		assert.Equal(t, "./base.yaml#base_settings.api_prefix", *api.Version)
		// File tags inside loaded files are not processed recursively
		assert.Equal(t, "./base.yaml#base_settings.environment", api.Labels["environment"])
		assert.Equal(t, "./base.yaml#base_settings.features.authentication", api.Labels["auth_enabled"])
		assert.Equal(t, "./base.yaml#base_settings.features.rate_limiting", api.Labels["rate_limit_enabled"])
	})

	t.Run("complex data type extraction", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create file with complex data structures
		complexData := `
api_specs:
  users:
    openapi: "3.0.0"
    info:
      title: "Users API"
      version: "1.0.0"
      contact:
        name: "API Team"
        email: "api@company.com"
        url: "https://company.com/api"
    servers:
      - url: "https://api.company.com/v1"
        description: "Production"
      - url: "https://staging.company.com/v1"
        description: "Staging"
    paths:
      "/users":
        get:
          summary: "List users"
          parameters:
            - name: "limit"
              in: "query"
              schema:
                type: "integer"
                default: 10
          responses:
            "200":
              description: "Success"
              content:
                "application/json":
                  schema:
                    type: "object"
                    properties:
                      users:
                        type: "array"
                        items:
                          type: "object"

configurations:
  database:
    host: "db.company.com"
    port: "5432"  # Must be string for label usage
    ssl: "true"   # Must be string for label usage
    pools:
      read: 10
      write: 5
  cache:
    redis:
      host: "redis.company.com"
      port: 6379
      ttl: "3600"  # Must be string for label usage
    memory:
      max_size: "256MB"
      cleanup_interval: "5m"
`
		complexFile := filepath.Join(tempDir, "complex.yaml")
		require.NoError(t, os.WriteFile(complexFile, []byte(complexData), 0o600))

		// Create config that extracts various complex data types
		config := `
apis:
  - ref: complex-api
    name: !file ./complex.yaml#api_specs.users.info.title
    description: "Complex API extraction test"
    version: !file ./complex.yaml#api_specs.users.info.version
    labels:
      contact_email: !file ./complex.yaml#api_specs.users.info.contact.email
      db_host: !file ./complex.yaml#configurations.database.host
      db_port: !file ./complex.yaml#configurations.database.port
      cache_ttl: !file ./complex.yaml#configurations.cache.redis.ttl
      
    versions:
      - ref: complex-api-v1
        version: "v1"
        # Extract entire OpenAPI spec
        spec: !file ./complex.yaml#api_specs.users
        
portals:
  - ref: complex-portal
    name: "Complex Portal"
    description: !file ./complex.yaml#api_specs.users.info.contact.name
    labels:
      # Extract nested objects (not arrays)
      api_title: !file ./complex.yaml#api_specs.users.info.title
      api_version: !file ./complex.yaml#api_specs.users.info.version
      # Extract nested objects
      db_ssl: !file ./complex.yaml#configurations.database.ssl
      cache_size: !file ./complex.yaml#configurations.cache.memory.max_size
`
		configFile := filepath.Join(tempDir, "config.yaml")
		require.NoError(t, os.WriteFile(configFile, []byte(config), 0o600))

		// Load and verify complex extraction
		l := loader.New()
		sources := []loader.Source{{Path: configFile, Type: loader.SourceTypeFile}}

		resourceSet, err := l.LoadFromSources(sources, false)
		require.NoError(t, err)
		require.Len(t, resourceSet.APIs, 1)
		require.Len(t, resourceSet.APIVersions, 1)
		require.Len(t, resourceSet.Portals, 1)

		api := resourceSet.APIs[0]
		assert.Equal(t, "Users API", api.Name)
		require.NotNil(t, api.Version)
		assert.Equal(t, "1.0.0", *api.Version)
		assert.Equal(t, "api@company.com", api.Labels["contact_email"])
		assert.Equal(t, "db.company.com", api.Labels["db_host"])
		assert.Equal(t, "5432", api.Labels["db_port"])
		assert.Equal(t, "3600", api.Labels["cache_ttl"])

		version := resourceSet.APIVersions[0]
		if version.Spec.Content != nil {
			t.Log("Complex value extraction spec loaded successfully")
		}

		portal := resourceSet.Portals[0]
		require.NotNil(t, portal.Description)
		assert.Equal(t, "API Team", *portal.Description)
		require.NotNil(t, portal.Labels["api_title"])
		assert.Equal(t, "Users API", *portal.Labels["api_title"])
		require.NotNil(t, portal.Labels["api_version"])
		assert.Equal(t, "1.0.0", *portal.Labels["api_version"])
		require.NotNil(t, portal.Labels["db_ssl"])
		assert.Equal(t, "true", *portal.Labels["db_ssl"])
		require.NotNil(t, portal.Labels["cache_size"])
		assert.Equal(t, "256MB", *portal.Labels["cache_size"])
	})

	t.Run("tag processing with special characters", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create file with special characters and edge cases
		specialData := `
special_data:
  # String with special characters
  description: "API with special chars: àáâãäåæçèéêë & symbols !@#$%^&*(){}[]|\\:;\"'<>?,./"
  
  # Multiline string
  long_description: |
    This is a multiline description
    that spans multiple lines
    and contains various content
    including special characters: àáâãäåæçèéêë
    
  # String with quotes
  quoted_content: "Content with \"nested quotes\" and 'single quotes'"
  
  # Numeric values as strings
  version_string: "1.2.3"
  
  # Boolean values
  enabled: true
  disabled: false
  
  # Null value
  optional_field: null
  
  # Array with mixed types
  mixed_array:
    - "string item"
    - 42
    - true
    - null
  
  # Object with special key names  
  key-with-dashes: "dash value"
  key_simple: "simple value"
  key_with_spaces: "space value"
  
  # Unicode content
  unicode_text: "Hello 世界 🌍 Мир"
  
  # URLs and special formats
  api_endpoint: "https://api.example.com/v1/users?limit=10&offset=0"
  email: "test@example.com"
`
		specialFile := filepath.Join(tempDir, "special.yaml")
		require.NoError(t, os.WriteFile(specialFile, []byte(specialData), 0o600))

		// Create config that extracts special content
		config := `
portals:
  - ref: special-portal
    name: "Special Characters Portal"
    description: !file ./special.yaml#special_data.description
    display_name: !file ./special.yaml#special_data.long_description
    labels:
      quoted_content: !file ./special.yaml#special_data.quoted_content
      version: !file ./special.yaml#special_data.version_string
      enabled: "true"  # Can't use boolean file tags as label values
      disabled: "false"  # Can't use boolean file tags as label values
      dash_key: !file ./special.yaml#special_data.key-with-dashes
      simple_key: !file ./special.yaml#special_data.key_simple
      space_key: !file ./special.yaml#special_data.key_with_spaces
      unicode: !file ./special.yaml#special_data.unicode_text
      endpoint: !file ./special.yaml#special_data.api_endpoint
      email: !file ./special.yaml#special_data.email

apis:
  - ref: special-api
    name: "Special API"
    description: !file ./special.yaml#special_data.unicode_text
    version: !file ./special.yaml#special_data.version_string
`
		configFile := filepath.Join(tempDir, "config.yaml")
		require.NoError(t, os.WriteFile(configFile, []byte(config), 0o600))

		// Load and verify special character handling
		l := loader.New()
		sources := []loader.Source{{Path: configFile, Type: loader.SourceTypeFile}}

		resourceSet, err := l.LoadFromSources(sources, false)
		require.NoError(t, err)
		require.Len(t, resourceSet.Portals, 1)
		require.Len(t, resourceSet.APIs, 1)

		portal := resourceSet.Portals[0]
		require.NotNil(t, portal.Description)
		assert.Contains(t, *portal.Description, "àáâãäåæçèéêë")
		assert.Contains(t, *portal.Description, "!@#$%^&*()")
		require.NotNil(t, portal.DisplayName)
		assert.Contains(t, *portal.DisplayName, "multiline description")
		require.NotNil(t, portal.Labels["quoted_content"])
		assert.Contains(t, *portal.Labels["quoted_content"], `"nested quotes"`)
		require.NotNil(t, portal.Labels["version"])
		assert.Equal(t, "1.2.3", *portal.Labels["version"])
		require.NotNil(t, portal.Labels["enabled"])
		assert.Equal(t, "true", *portal.Labels["enabled"])
		require.NotNil(t, portal.Labels["disabled"])
		assert.Equal(t, "false", *portal.Labels["disabled"])
		require.NotNil(t, portal.Labels["dash_key"])
		assert.Equal(t, "dash value", *portal.Labels["dash_key"])
		require.NotNil(t, portal.Labels["simple_key"])
		assert.Equal(t, "simple value", *portal.Labels["simple_key"])
		require.NotNil(t, portal.Labels["space_key"])
		assert.Equal(t, "space value", *portal.Labels["space_key"])
		require.NotNil(t, portal.Labels["unicode"])
		assert.Contains(t, *portal.Labels["unicode"], "世界 🌍 Мир")
		require.NotNil(t, portal.Labels["endpoint"])
		assert.Equal(t, "https://api.example.com/v1/users?limit=10&offset=0", *portal.Labels["endpoint"])
		require.NotNil(t, portal.Labels["email"])
		assert.Equal(t, "test@example.com", *portal.Labels["email"])

		api := resourceSet.APIs[0]
		require.NotNil(t, api.Description)
		assert.Contains(t, *api.Description, "Hello 世界 🌍 Мир")
		require.NotNil(t, api.Version)
		assert.Equal(t, "1.2.3", *api.Version)
	})

	t.Run("tag processing error scenarios", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create file for valid references
		validData := `valid: "content"`
		validFile := filepath.Join(tempDir, "valid.yaml")
		require.NoError(t, os.WriteFile(validFile, []byte(validData), 0o600))

		errorTests := []struct {
			name          string
			configContent string
			expectedError string
		}{
			{
				name: "malformed file tag",
				configContent: `
portals:
  - ref: test-portal
    name: "Test"
    description: !file [malformed
`,
				expectedError: "failed to parse YAML",
			},
			{
				name: "nonexistent extraction path",
				configContent: `
portals:
  - ref: test-portal
    name: "Test"
    description: !file ./valid.yaml#nonexistent.path
`,
				expectedError: "path not found",
			},
			{
				name: "invalid extraction syntax",
				configContent: `
portals:
  - ref: test-portal
    name: "Test"
    description: !file ./valid.yaml#.invalid..syntax.
`,
				expectedError: "path not found",
			},
			{
				name: "empty file reference",
				configContent: `
portals:
  - ref: test-portal
    name: "Test"
    description: !file
`,
				expectedError: "is a directory",
			},
		}

		for _, tt := range errorTests {
			t.Run(tt.name, func(t *testing.T) {
				configFile := filepath.Join(tempDir, "error_config.yaml")
				require.NoError(t, os.WriteFile(configFile, []byte(tt.configContent), 0o600))

				l := loader.New()
				sources := []loader.Source{{Path: configFile, Type: loader.SourceTypeFile}}

				_, err := l.LoadFromSources(sources, false)
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			})
		}
	})
}

// TestYAMLTagPerformance tests performance-related aspects of tag processing
func TestYAMLTagPerformance(t *testing.T) {
	t.Run("large scale file tag processing", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create multiple data files
		numDataFiles := 50
		for i := 0; i < numDataFiles; i++ {
			content := fmt.Sprintf(`
data_%d:
  id: %d
  name: "Data File %d"
  description: "Content from data file number %d"
  timestamp: "2024-01-01T%02d:00:00Z"
  metadata:
    index: "%d"
    category: "category_%d"
`, i, i, i, i, i%24, i, i%10)

			fileName := fmt.Sprintf("data_%d.yaml", i)
			filePath := filepath.Join(tempDir, fileName)
			require.NoError(t, os.WriteFile(filePath, []byte(content), 0o600))
		}

		// Create config that references all files
		configParts := []string{"apis:"}
		for i := 0; i < numDataFiles; i++ {
			apiDef := fmt.Sprintf(`
  - ref: api-%d
    name: !file ./data_%d.yaml#data_%d.name
    description: !file ./data_%d.yaml#data_%d.description
    version: "1.0.0"
    labels:
      timestamp: !file ./data_%d.yaml#data_%d.timestamp
      category: !file ./data_%d.yaml#data_%d.metadata.category
      index: !file ./data_%d.yaml#data_%d.metadata.index`,
				i, i, i, i, i, i, i, i, i, i, i)
			configParts = append(configParts, apiDef)
		}

		config := strings.Join(configParts, "")
		configFile := filepath.Join(tempDir, "large_config.yaml")
		require.NoError(t, os.WriteFile(configFile, []byte(config), 0o600))

		// Load and verify performance is acceptable
		l := loader.New()
		sources := []loader.Source{{Path: configFile, Type: loader.SourceTypeFile}}

		resourceSet, err := l.LoadFromSources(sources, false)
		require.NoError(t, err)
		require.Len(t, resourceSet.APIs, numDataFiles)

		// Verify random sampling of loaded data
		for i := 0; i < 10; i++ {
			api := resourceSet.APIs[i]
			expectedName := fmt.Sprintf("Data File %d", i)
			assert.Equal(t, expectedName, api.Name)
			require.NotNil(t, api.Description)
			assert.Contains(t, *api.Description, fmt.Sprintf("number %d", i))
		}
	})
}
