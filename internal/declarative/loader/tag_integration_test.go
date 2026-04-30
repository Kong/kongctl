package loader

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
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

func TestLoader_EnvTagIntegration(t *testing.T) {
	t.Setenv("PORTAL_DESCRIPTION", "loaded-from-env")

	tmpDir, err := os.MkdirTemp("", "loader-env-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	mainContent := `
portals:
  - ref: env-portal
    name: env-portal
    description: !env PORTAL_DESCRIPTION`

	mainFile := filepath.Join(tmpDir, "main.yaml")
	require.NoError(t, os.WriteFile(mainFile, []byte(mainContent), 0o600))

	loader := NewWithBaseDir(tmpDir)
	rs, err := loader.LoadFile(mainFile)
	require.NoError(t, err)
	require.NotNil(t, rs)
	require.Len(t, rs.Portals, 1)
	require.NotNil(t, rs.Portals[0].Description)

	assert.Equal(t, "loaded-from-env", *rs.Portals[0].Description)
	assert.Equal(t, "__ENV__:PORTAL_DESCRIPTION", rs.GetEnvSources("env-portal")["/description"])
}

func TestLoader_EnvTagStringOnlyFields(t *testing.T) {
	t.Setenv("PORTAL_AUTH_ENABLED", "true")

	tmpDir, err := os.MkdirTemp("", "loader-env-bool-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	mainContent := `
portals:
  - ref: env-portal
    name: env-portal
    authentication_enabled: !env PORTAL_AUTH_ENABLED`

	mainFile := filepath.Join(tmpDir, "main.yaml")
	require.NoError(t, os.WriteFile(mainFile, []byte(mainContent), 0o600))

	loader := NewWithBaseDir(tmpDir)
	_, err = loader.LoadFile(mainFile)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "!env currently supports string-typed fields only")
}

func TestLoader_EnvTagDoesNotMaskUnknownFieldErrors(t *testing.T) {
	t.Setenv("PORTAL_DESCRIPTION", "loaded-from-env")

	tmpDir, err := os.MkdirTemp("", "loader-env-unknown-field-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	mainContent := `
portals:
  - ref: env-portal
    name: env-portal
    description: !env PORTAL_DESCRIPTION
    lables:
      team: docs`

	mainFile := filepath.Join(tmpDir, "main.yaml")
	require.NoError(t, os.WriteFile(mainFile, []byte(mainContent), 0o600))

	loader := NewWithBaseDir(tmpDir)
	_, err = loader.LoadFile(mainFile)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown field 'lables'")
	assert.NotContains(t, err.Error(), "!env currently supports string-typed fields only")
}

func TestLoader_EnvTagIntegration_PortalCustomDomainSSLUnion(t *testing.T) {
	t.Setenv("CUSTOM_CERT", "-----BEGIN CERTIFICATE-----\nCERT\n-----END CERTIFICATE-----")
	t.Setenv("CUSTOM_KEY", "-----BEGIN PRIVATE KEY-----\nKEY\n-----END PRIVATE KEY-----")

	tmpDir, err := os.MkdirTemp("", "loader-env-portal-domain-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	mainContent := `
portals:
  - ref: env-portal
    name: env-portal
    custom_domain:
      ref: env-portal-domain
      hostname: env-portal.example.com
      enabled: true
      ssl:
        domain_verification_method: custom_certificate
        custom_certificate: !env CUSTOM_CERT
        custom_private_key: !env CUSTOM_KEY
`

	mainFile := filepath.Join(tmpDir, "main.yaml")
	require.NoError(t, os.WriteFile(mainFile, []byte(mainContent), 0o600))

	loader := NewWithBaseDir(tmpDir)
	rs, err := loader.LoadFile(mainFile)
	require.NoError(t, err)
	require.NotNil(t, rs)
	require.Len(t, rs.PortalCustomDomains, 1)

	envSources := rs.GetEnvSources("env-portal-domain")
	assert.Equal(t, "__ENV__:CUSTOM_CERT", envSources["/ssl/custom_certificate"])
	assert.Equal(t, "__ENV__:CUSTOM_KEY", envSources["/ssl/custom_private_key"])
}

func TestLoader_EnvTagIntegration_PortalIdentityProviderConfig(t *testing.T) {
	t.Setenv("IDP_ISSUER_URL", "https://example.okta.test/oauth2/default")
	t.Setenv("IDP_CLIENT_ID", "client-id-from-env")
	t.Setenv("IDP_CLIENT_SECRET", "client-secret-from-env")

	tmpDir, err := os.MkdirTemp("", "loader-env-portal-idp-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	mainContent := `
portals:
  - ref: env-portal
    name: env-portal
    identity_providers:
      - ref: env-portal-idp
        type: oidc
        config:
          issuer_url: !env IDP_ISSUER_URL
          client_id: !env IDP_CLIENT_ID
          client_secret: !env IDP_CLIENT_SECRET
`

	mainFile := filepath.Join(tmpDir, "main.yaml")
	require.NoError(t, os.WriteFile(mainFile, []byte(mainContent), 0o600))

	loader := NewWithBaseDir(tmpDir)
	rs, err := loader.LoadFile(mainFile)
	require.NoError(t, err)
	require.NotNil(t, rs)
	require.Len(t, rs.PortalIdentityProviders, 1)

	config := rs.PortalIdentityProviders[0].Config
	require.NotNil(t, config)
	require.NotNil(t, config.OIDCIdentityProviderConfig)
	assert.Equal(t, "https://example.okta.test/oauth2/default", config.OIDCIdentityProviderConfig.IssuerURL)
	assert.Equal(t, "client-id-from-env", config.OIDCIdentityProviderConfig.ClientID)
	require.NotNil(t, config.OIDCIdentityProviderConfig.ClientSecret)
	assert.Equal(t, "client-secret-from-env", *config.OIDCIdentityProviderConfig.ClientSecret)

	envSources := rs.GetEnvSources("env-portal-idp")
	assert.Equal(t, "__ENV__:IDP_ISSUER_URL", envSources["/config/issuer_url"])
	assert.Equal(t, "__ENV__:IDP_CLIENT_ID", envSources["/config/client_id"])
	assert.Equal(t, "__ENV__:IDP_CLIENT_SECRET", envSources["/config/client_secret"])
}

func TestLoader_ControlPlaneDataPlaneCertificateTags(t *testing.T) {
	t.Setenv("DP_CERT", "-----BEGIN CERTIFICATE-----\nENV\n-----END CERTIFICATE-----")

	tmpDir := t.TempDir()
	rootCertPath := filepath.Join(tmpDir, "root.pem")
	require.NoError(t, os.WriteFile(
		rootCertPath,
		[]byte("-----BEGIN CERTIFICATE-----\nFILE\n-----END CERTIFICATE-----"),
		0o600,
	))

	mainContent := `
control_planes:
  - ref: cp
    name: cp
    data_plane_certificates:
      - ref: nested-dp-cert
        cert: !env DP_CERT

control_plane_data_plane_certificates:
  - ref: root-dp-cert
    control_plane: cp
    cert: !file ./root.pem
`

	mainFile := filepath.Join(tmpDir, "main.yaml")
	require.NoError(t, os.WriteFile(mainFile, []byte(mainContent), 0o600))

	loader := NewWithBaseDir(tmpDir)
	rs, err := loader.LoadFile(mainFile)
	require.NoError(t, err)
	require.NotNil(t, rs)

	require.Len(t, rs.ControlPlanes, 1)
	assert.Empty(t, rs.ControlPlanes[0].DataPlaneCertificates)
	require.Len(t, rs.ControlPlaneDataPlaneCertificates, 2)

	certsByRef := make(map[string]resources.ControlPlaneDataPlaneCertificateResource)
	for _, cert := range rs.ControlPlaneDataPlaneCertificates {
		certsByRef[cert.Ref] = cert
	}

	nested := certsByRef["nested-dp-cert"]
	assert.Equal(t, "nested-dp-cert", nested.Ref)
	assert.Equal(t, "cp", nested.ControlPlane)
	assert.Equal(t, "-----BEGIN CERTIFICATE-----\nENV\n-----END CERTIFICATE-----", nested.Cert)
	assert.Equal(t, "__ENV__:DP_CERT", rs.GetEnvSources("nested-dp-cert")["/cert"])

	root := certsByRef["root-dp-cert"]
	assert.Equal(t, "root-dp-cert", root.Ref)
	assert.Equal(t, "cp", root.ControlPlane)
	assert.Equal(t, "-----BEGIN CERTIFICATE-----\nFILE\n-----END CERTIFICATE-----", root.Cert)
}
