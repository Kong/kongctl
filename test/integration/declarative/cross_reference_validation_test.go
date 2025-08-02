//go:build integration
// +build integration

package declarative_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kong/kongctl/internal/declarative/loader"
)

// TestCrossReferenceValidation_APIResources tests cross-resource reference validation for API resources
func TestCrossReferenceValidation_APIResources(t *testing.T) {
	tests := []struct {
		name        string
		config      string
		wantErr     bool
		expectedErr string
	}{
		{
			name: "valid API publication with portal reference",
			config: `
portals:
  - ref: main-portal
    name: "Main Portal"
    description: "Main developer portal"

apis:
  - ref: users-api
    name: "Users API"
    description: "User management API"
    publications:
      - ref: main-pub
        portal_id: main-portal
        publish_status: published
`,
			wantErr: false,
		},
		{
			name: "invalid API publication with nonexistent portal reference",
			config: `
portals:
  - ref: main-portal
    name: "Main Portal"
    description: "Main developer portal"

apis:
  - ref: users-api
    name: "Users API"
    description: "User management API"
    publications:
      - ref: main-pub
        portal_id: nonexistent-portal
        publish_status: published
`,
			wantErr:     true,
			expectedErr: `references unknown portal: nonexistent-portal`,
		},
		{
			name: "valid API implementation with external UUID control plane",
			config: `
apis:
  - ref: users-api
    name: "Users API"
    description: "User management API"
    implementations:
      - ref: users-impl
        implementation_url: "https://api.example.com"
        service:
          id: "12345678-1234-1234-1234-123456789012"
          control_plane_id: "87654321-4321-4321-4321-210987654321"
`,
			wantErr: false,
		},
		{
			name: "valid API implementation with declarative control plane reference",
			config: `
control_planes:
  - ref: prod-cp
    name: "Production Control Plane"
    description: "Production environment"

apis:
  - ref: users-api
    name: "Users API"
    description: "User management API"
    implementations:
      - ref: users-impl
        implementation_url: "https://api.example.com"
        service:
          id: "12345678-1234-1234-1234-123456789012"
          control_plane_id: prod-cp
`,
			wantErr: false,
		},
		{
			name: "invalid API implementation with nonexistent control plane reference",
			config: `
control_planes:
  - ref: prod-cp
    name: "Production Control Plane"
    description: "Production environment"

apis:
  - ref: users-api
    name: "Users API"
    description: "User management API"
    implementations:
      - ref: users-impl
        implementation_url: "https://api.example.com"
        service:
          id: "12345678-1234-1234-1234-123456789012"
          control_plane_id: nonexistent-cp
`,
			wantErr:     true,
			expectedErr: `references unknown control_plane: nonexistent-cp`,
		},
		{
			name: "multiple API publications with valid and invalid references",
			config: `
portals:
  - ref: main-portal
    name: "Main Portal"
    description: "Main developer portal"

apis:
  - ref: users-api
    name: "Users API"
    description: "User management API"
    publications:
      - ref: main-pub
        portal_id: main-portal
        publish_status: published
      - ref: invalid-pub
        portal_id: nonexistent-portal
        publish_status: published
`,
			wantErr:     true,
			expectedErr: `references unknown portal: nonexistent-portal`,
		},
		{
			name: "complex multi-resource scenario with valid references",
			config: `
portals:
  - ref: main-portal
    name: "Main Portal"
    description: "Main developer portal"
  - ref: dev-portal
    name: "Developer Portal"
    description: "Development portal"

control_planes:
  - ref: prod-cp
    name: "Production Control Plane"
    description: "Production environment"

apis:
  - ref: users-api
    name: "Users API"
    description: "User management API"
    versions:
      - ref: v1
        version: "1.0.0"
    publications:
      - ref: main-pub
        portal_id: main-portal
        publish_status: published
      - ref: dev-pub
        portal_id: dev-portal
        publish_status: unpublished
    implementations:
      - ref: users-impl-external
        implementation_url: "https://api.example.com"
        service:
          id: "12345678-1234-1234-1234-123456789012"
          control_plane_id: "87654321-4321-4321-4321-210987654321"
      - ref: users-impl-declarative
        implementation_url: "https://api-internal.example.com"
        service:
          id: "87654321-1234-1234-1234-123456789012"
          control_plane_id: prod-cp
`,
			wantErr: false,
		},
		{
			name: "separate API child resources with valid references",
			config: `
portals:
  - ref: main-portal
    name: "Main Portal"
    description: "Main developer portal"

control_planes:
  - ref: prod-cp
    name: "Production Control Plane"
    description: "Production environment"

apis:
  - ref: users-api
    name: "Users API"
    description: "User management API"

api_versions:
  - ref: v1
    api: users-api
    version: "1.0.0"

api_publications:
  - ref: main-pub
    api: users-api
    portal_id: main-portal
    publish_status: published

api_implementations:
  - ref: users-impl
    api: users-api
    implementation_url: "https://api.example.com"
    service:
      id: "12345678-1234-1234-1234-123456789012"
      control_plane_id: prod-cp
`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary config file
			configDir := t.TempDir()
			configFile := filepath.Join(configDir, "config.yaml")
			require.NoError(t, os.WriteFile(configFile, []byte(tt.config), 0600))

			// Load configuration
			l := loader.New()
			_, err := l.LoadFile(configFile)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestExternalIDValidation_APIImplementations tests that external UUIDs bypass validation
func TestExternalIDValidation_APIImplementations(t *testing.T) {
	tests := []struct {
		name           string
		controlPlaneID string
		serviceID      string
		wantErr        bool
		expectedErr    string
	}{
		{
			name:           "valid external UUIDs",
			controlPlaneID: "12345678-1234-1234-1234-123456789012",
			serviceID:      "87654321-4321-4321-4321-210987654321",
			wantErr:        false,
		},
		{
			name:           "invalid service ID format",
			controlPlaneID: "12345678-1234-1234-1234-123456789012",
			serviceID:      "not-a-uuid",
			wantErr:        true,
			expectedErr:    "service.id must be a valid UUID (external service managed by decK)",
		},
		{
			name:           "external control plane ID should not be validated as reference",
			controlPlaneID: "12345678-1234-1234-1234-123456789012",
			serviceID:      "87654321-4321-4321-4321-210987654321",
			wantErr:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := `
apis:
  - ref: users-api
    name: "Users API"
    description: "User management API"
    implementations:
      - ref: users-impl
        implementation_url: "https://api.example.com"
        service:
          id: "` + tt.serviceID + `"
          control_plane_id: "` + tt.controlPlaneID + `"
`

			// Create temporary config file
			configDir := t.TempDir()
			configFile := filepath.Join(configDir, "config.yaml")
			require.NoError(t, os.WriteFile(configFile, []byte(config), 0600))

			// Load configuration
			l := loader.New()
			_, err := l.LoadFile(configFile)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestReferenceValidation_ErrorMessages tests that error messages are clear and helpful
func TestReferenceValidation_ErrorMessages(t *testing.T) {
	tests := []struct {
		name        string
		config      string
		expectedErr string
	}{
		{
			name: "missing portal reference shows clear error",
			config: `
apis:
  - ref: users-api
    name: "Users API"
    publications:
      - ref: main-pub
        portal_id: missing-portal
`,
			expectedErr: `references unknown portal: missing-portal`,
		},
		{
			name: "missing control plane reference shows clear error",
			config: `
apis:
  - ref: users-api
    name: "Users API"
    implementations:
      - ref: users-impl
        service:
          id: "12345678-1234-1234-1234-123456789012"
          control_plane_id: missing-cp
`,
			expectedErr: `references unknown control_plane: missing-cp`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary config file
			configDir := t.TempDir()
			configFile := filepath.Join(configDir, "config.yaml")
			require.NoError(t, os.WriteFile(configFile, []byte(tt.config), 0600))

			// Load configuration
			l := loader.New()
			_, err := l.LoadFile(configFile)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedErr)
		})
	}
}