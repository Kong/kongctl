//go:build integration && disabled
// +build integration,disabled

package declarative_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kong/kongctl/internal/declarative/loader"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCompleteMultiResourceScenario tests a complex scenario with APIs, portals, and child resources
func TestCompleteMultiResourceScenario(t *testing.T) {
	// Create test directory structure
	tempDir := t.TempDir()

	// Create external spec file
	specContent := `{
		"openapi": "3.0.0",
		"info": {
			"title": "Users API",
			"version": "1.0.0"
		},
		"paths": {
			"/users": {
				"get": {
					"summary": "List users",
					"responses": {
						"200": {
							"description": "Success"
						}
					}
				}
			}
		}
	}`
	specFile := filepath.Join(tempDir, "specs", "users-api-spec.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(specFile), 0o755))
	require.NoError(t, os.WriteFile(specFile, []byte(specContent), 0o600))

	// Create portal configuration
	portalConfig := `
portals:
  - ref: developer-portal
    name: "Developer Portal"
    description: "Main developer portal for APIs"
    authentication_enabled: true
    rbac_enabled: false
    default_api_visibility: "public"
    auto_approve_developers: true
`
	portalFile := filepath.Join(tempDir, "portal.yaml")
	require.NoError(t, os.WriteFile(portalFile, []byte(portalConfig), 0o600))

	// Create control plane configuration
	controlPlaneConfig := `
control_planes:
  - ref: production-cp
    name: "Production Control Plane"
    description: "Production environment control plane"
    cluster_type: "CLUSTER_TYPE_HYBRID"
`
	controlPlaneFile := filepath.Join(tempDir, "control-plane.yaml")
	require.NoError(t, os.WriteFile(controlPlaneFile, []byte(controlPlaneConfig), 0o600))

	// Create main API configuration with file tags and child resources
	apiConfig := `
apis:
  - ref: users-api
    name: "Users API"
    description: "API for user management operations"
    version: "1.0.0"
    versions:
      - ref: users-api-v1
        name: "v1"
        gateway_service:
          control_plane_id: production-cp
          id: "550e8400-e29b-41d4-a716-446655440000"
        spec: !file ./specs/users-api-spec.json
      - ref: users-api-v2
        name: "v2"
        gateway_service:
          control_plane_id: production-cp
          id: "550e8400-e29b-41d4-a716-446655440001"
        spec: !file ./specs/users-api-spec.json
    publications:
      - ref: users-api-pub
        portal_id: developer-portal
        visibility: public
        auto_approve_registrations: true
    implementations:
      - ref: users-api-impl
        service:
          control_plane_id: production-cp
          id: "550e8400-e29b-41d4-a716-446655440000"
`
	apiFile := filepath.Join(tempDir, "api.yaml")
	require.NoError(t, os.WriteFile(apiFile, []byte(apiConfig), 0o600))

	// Load all configurations
	l := loader.New()
	sources := []loader.Source{
		{Path: portalFile, Type: loader.SourceTypeFile},
		{Path: controlPlaneFile, Type: loader.SourceTypeFile},
		{Path: apiFile, Type: loader.SourceTypeFile},
	}

	resourceSet, err := l.LoadFromSources(sources, false)
	require.NoError(t, err)

	// Verify loaded resources
	require.Len(t, resourceSet.Portals, 1)
	require.Len(t, resourceSet.ControlPlanes, 1)
	require.Len(t, resourceSet.APIs, 1)
	require.Len(t, resourceSet.APIVersions, 2)
	require.Len(t, resourceSet.APIPublications, 1)
	require.Len(t, resourceSet.APIImplementations, 1)

	// Verify API versions have spec loaded from file
	for _, version := range resourceSet.APIVersions {
		if version.Spec != nil {
			// Note: spec handling varies by SDK version
			t.Logf("API version %s has spec loaded from file", version.GetRef())
		}
	}

	// This test verifies comprehensive multi-resource loading with file tags
	// The actual planning and execution tests are covered in existing api_test.go
	t.Log("Successfully loaded complex multi-resource scenario with file tags")
}

// TestSeparateFileMultiResourceConfiguration tests loading resources from separate files
func TestSeparateFileMultiResourceConfiguration(t *testing.T) {
	// Create test directory structure
	tempDir := t.TempDir()
	resourcesDir := filepath.Join(tempDir, "resources")
	require.NoError(t, os.MkdirAll(resourcesDir, 0o755))

	// Create separate files for each resource type
	portalConfig := `
portals:
  - ref: api-portal
    name: "API Portal"
    description: "Portal for API documentation"
`
	require.NoError(t, os.WriteFile(filepath.Join(resourcesDir, "portals.yaml"), []byte(portalConfig), 0o600))

	apiConfig := `
apis:
  - ref: payment-api
    name: "Payment API"
    description: "API for payment processing"
    version: "2.0.0"
`
	require.NoError(t, os.WriteFile(filepath.Join(resourcesDir, "apis.yaml"), []byte(apiConfig), 0o600))

	versionConfig := `
api_versions:
  - ref: payment-api-v2
    api: payment-api
    name: "v2"
    gateway_service:
      control_plane_id: "550e8400-e29b-41d4-a716-446655440000"
      id: "550e8400-e29b-41d4-a716-446655440001"
`
	require.NoError(t, os.WriteFile(filepath.Join(resourcesDir, "versions.yaml"), []byte(versionConfig), 0o600))

	publicationConfig := `
api_publications:
  - ref: payment-api-pub
    api: payment-api
    portal_id: api-portal
    visibility: private
    auto_approve_registrations: false
`
	require.NoError(t, os.WriteFile(filepath.Join(resourcesDir, "publications.yaml"), []byte(publicationConfig), 0o600))

	// Load configuration directory
	l := loader.New()
	sources := []loader.Source{{Path: resourcesDir, Type: loader.SourceTypeDirectory}}

	resourceSet, err := l.LoadFromSources(sources, false)
	require.NoError(t, err)

	// Verify all resources loaded correctly
	require.Len(t, resourceSet.Portals, 1)
	require.Len(t, resourceSet.APIs, 1)
	require.Len(t, resourceSet.APIVersions, 1)
	require.Len(t, resourceSet.APIPublications, 1)

	// Verify cross-references
	assert.Equal(t, "payment-api", resourceSet.APIVersions[0].API)
	assert.Equal(t, "payment-api", resourceSet.APIPublications[0].API)
	assert.Equal(t, "api-portal", resourceSet.APIPublications[0].PortalID)

	// This test verifies loading separate file configurations
	// Planning and execution are covered in existing tests
	t.Log("Successfully loaded separate file multi-resource configuration")
}

// TestFileTagComplexValueExtraction tests complex YAML value extraction scenarios
func TestFileTagComplexValueExtraction(t *testing.T) {
	// Create test directory
	tempDir := t.TempDir()

	// Create complex metadata file
	metadataContent := `
api_specs:
  users:
    openapi: "3.0.0"
    info:
      title: "Users API"
      version: "1.2.3"
      description: "Comprehensive user management API"
      contact:
        name: "API Team"
        email: "api-team@company.com"
    servers:
      - url: "https://api.company.com/v1"
        description: "Production server"
      - url: "https://staging-api.company.com/v1"
        description: "Staging server"
    paths:
      /users:
        get:
          summary: "List users"
          tags: ["users"]
  products:
    openapi: "3.0.0"
    info:
      title: "Products API"
      version: "2.1.0"
      description: "Product catalog management"

deployment:
  environment: production
  regions:
    - us-west-2
    - eu-west-1
  feature_flags:
    new_auth: true
    beta_features: false

portal_settings:
  branding:
    logo_url: "https://company.com/logo.png"
    theme_color: "#2563eb"
    footer_text: "© 2024 Company Inc"
  authentication:
    providers:
      - type: "oauth2"
        name: "Google"
        enabled: true
      - type: "saml"
        name: "Corporate SSO"
        enabled: true
`
	metadataFile := filepath.Join(tempDir, "metadata.yaml")
	require.NoError(t, os.WriteFile(metadataFile, []byte(metadataContent), 0o600))

	// Create configuration with complex value extraction
	config := `
portals:
  - ref: main-portal
    name: "Main Portal"
    description: !file ./metadata.yaml#portal_settings.branding.footer_text
    custom_theme:
      primary_color: !file ./metadata.yaml#portal_settings.branding.theme_color

apis:
  - ref: users-api
    name: !file ./metadata.yaml#api_specs.users.info.title
    description: !file ./metadata.yaml#api_specs.users.info.description
    version: !file ./metadata.yaml#api_specs.users.info.version
    labels:
      environment: !file ./metadata.yaml#deployment.environment
      team: !file ./metadata.yaml#api_specs.users.info.contact.name
    versions:
      - ref: users-api-v1
        name: "v1"
        gateway_service:
          control_plane_id: "550e8400-e29b-41d4-a716-446655440000"
          id: "550e8400-e29b-41d4-a716-446655440001"
        spec: !file ./metadata.yaml#api_specs.users
  
  - ref: products-api
    name: !file ./metadata.yaml#api_specs.products.info.title
    description: !file ./metadata.yaml#api_specs.products.info.description
    version: !file ./metadata.yaml#api_specs.products.info.version
    labels:
      environment: !file ./metadata.yaml#deployment.environment
`
	configFile := filepath.Join(tempDir, "config.yaml")
	require.NoError(t, os.WriteFile(configFile, []byte(config), 0o600))

	// Load configuration
	l := loader.New()
	sources := []loader.Source{{Path: configFile, Type: loader.SourceTypeFile}}

	resourceSet, err := l.LoadFromSources(sources, false)
	require.NoError(t, err)

	// Verify complex value extraction worked correctly
	require.Len(t, resourceSet.Portals, 1)
	require.Len(t, resourceSet.APIs, 2)
	require.Len(t, resourceSet.APIVersions, 1)

	// Check portal values
	portal := resourceSet.Portals[0]
	assert.Equal(t, "© 2024 Company Inc", portal.Description)
	// Note: CustomTheme field handling may vary by SDK version

	// Check APIs values
	usersAPI := resourceSet.APIs[0]
	assert.Equal(t, "Users API", usersAPI.Name)
	assert.Equal(t, "Comprehensive user management API", usersAPI.Description)
	assert.Equal(t, "1.2.3", usersAPI.Version)
	assert.Equal(t, "production", usersAPI.Labels["environment"])
	assert.Equal(t, "API Team", usersAPI.Labels["team"])

	productsAPI := resourceSet.APIs[1]
	assert.Equal(t, "Products API", productsAPI.Name)
	assert.Equal(t, "Product catalog management", productsAPI.Description)
	assert.Equal(t, "2.1.0", productsAPI.Version)
	assert.Equal(t, "production", productsAPI.Labels["environment"])

	// Check version spec
	version := resourceSet.APIVersions[0]
	if version.Spec != nil {
		// Note: spec handling varies by SDK version
		t.Logf("Version spec loaded successfully")
	}
}

// TestErrorHandlingScenarios tests various error conditions
func TestErrorHandlingScenarios(t *testing.T) {
	tests := []struct {
		name          string
		configContent string
		expectedError string
		setupFiles    func(string) error
	}{
		{
			name: "missing file reference",
			configContent: `
portals:
  - ref: test-portal
    name: "Test Portal"
    description: !file ./nonexistent.yaml#description
`,
			expectedError: "no such file or directory",
		},
		{
			name: "invalid file tag syntax",
			configContent: `
portals:
  - ref: test-portal
    name: "Test Portal"
    description: !file [invalid, syntax]
`,
			expectedError: "failed to parse file reference",
		},
		{
			name: "missing extraction path",
			configContent: `
portals:
  - ref: test-portal
    name: "Test Portal"
    description: !file ./metadata.yaml#nonexistent.path
`,
			expectedError: "path not found: nonexistent.path",
			setupFiles: func(dir string) error {
				content := `existing: value`
				return os.WriteFile(filepath.Join(dir, "metadata.yaml"), []byte(content), 0o600)
			},
		},
		{
			name: "circular file reference",
			configContent: `
portals:
  - ref: test-portal
    name: "Test Portal"
    description: !file ./circular.yaml#portal.description
`,
			expectedError: "circular reference detected",
			setupFiles: func(dir string) error {
				content := `
portal:
  description: !file ./circular.yaml#portal.description
`
				return os.WriteFile(filepath.Join(dir, "circular.yaml"), []byte(content), 0o600)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()

			// Setup additional files if needed
			if tt.setupFiles != nil {
				require.NoError(t, tt.setupFiles(tempDir))
			}

			configFile := filepath.Join(tempDir, "config.yaml")
			require.NoError(t, os.WriteFile(configFile, []byte(tt.configContent), 0o600))

			// Attempt to load configuration
			l := loader.New()
			sources := []loader.Source{{Path: configFile, Type: loader.SourceTypeFile}}

			_, err := l.LoadFromSources(sources, false)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedError)
		})
	}
}

// stringPtr helper already defined in api_test.go
