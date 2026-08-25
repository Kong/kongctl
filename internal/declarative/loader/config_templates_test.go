package loader

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoaderExpandsSameFileResourceTemplate(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	require.NoError(t, os.WriteFile(configFile, []byte(`
_templates:
  standard-portal:
    authentication_enabled: true
    rbac_enabled: true
    default_api_visibility: private
    default_page_visibility: private
    labels:
      managed-by: kongctl

portals:
  - ref: payments-portal
    name: Payments Developer Portal
    labels:
      business-unit: payments
    _extends: standard-portal
`), 0o600))

	rs, err := NewWithBaseDir(tmpDir).LoadFile(configFile)
	require.NoError(t, err)
	require.Len(t, rs.Portals, 1)

	portal := rs.Portals[0]
	require.NotNil(t, portal.AuthenticationEnabled)
	assert.True(t, *portal.AuthenticationEnabled)
	require.NotNil(t, portal.RbacEnabled)
	assert.True(t, *portal.RbacEnabled)
	require.NotNil(t, portal.DefaultAPIVisibility)
	assert.Equal(t, "private", string(*portal.DefaultAPIVisibility))
	require.NotNil(t, portal.DefaultPageVisibility)
	assert.Equal(t, "private", string(*portal.DefaultPageVisibility))
	require.NotNil(t, portal.Labels["managed-by"])
	assert.Equal(t, "kongctl", *portal.Labels["managed-by"])
	require.NotNil(t, portal.Labels["business-unit"])
	assert.Equal(t, "payments", *portal.Labels["business-unit"])
}

func TestLoaderExpandsTemplateAcrossSourceFiles(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	templatesFile := filepath.Join(tmpDir, "templates.yaml")
	require.NoError(t, os.WriteFile(templatesFile, []byte(`
_templates:
  standard-portal:
    authentication_enabled: true
    default_api_visibility: private
    default_page_visibility: private
`), 0o600))
	portalsFile := filepath.Join(tmpDir, "portals.yaml")
	require.NoError(t, os.WriteFile(portalsFile, []byte(`
portals:
  - _extends: standard-portal
    ref: payments-portal
    name: Payments Developer Portal
`), 0o600))

	rs, err := NewWithBaseDir(tmpDir).LoadFromSources([]Source{
		{Path: templatesFile, Type: SourceTypeFile},
		{Path: portalsFile, Type: SourceTypeFile},
	}, false)
	require.NoError(t, err)
	require.Len(t, rs.Portals, 1)
	require.NotNil(t, rs.Portals[0].AuthenticationEnabled)
	assert.True(t, *rs.Portals[0].AuthenticationEnabled)
}

func TestExpandConfigTemplateDocumentsReleasesParsedTrees(t *testing.T) {
	t.Parallel()

	documents := []*configTemplateDocument{
		{
			content: []byte(`
_templates:
  standard:
    authentication_enabled: true
`),
			sourcePath: "templates.yaml",
		},
		{
			content: []byte(`
portals:
  - _extends: standard
    ref: docs
    name: Docs
`),
			sourcePath: "portals.yaml",
		},
	}

	require.NoError(t, expandConfigTemplateDocuments(documents))
	for _, document := range documents {
		assert.Zero(t, document.document.Kind)
		assert.NotContains(t, string(document.content), templatesKey)
		assert.NotContains(t, string(document.content), extendsKey)
	}
}

func TestLoaderDiscoversTemplatesRecursivelyInDirectorySource(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	sharedDir := filepath.Join(tmpDir, "shared")
	require.NoError(t, os.MkdirAll(sharedDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(sharedDir, "templates.yaml"), []byte(`
_templates:
  standard-portal:
    rbac_enabled: true
`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "portals.yaml"), []byte(`
portals:
  - _extends: standard-portal
    ref: docs
    name: Docs
`), 0o600))

	rs, err := NewWithBaseDir(tmpDir).LoadFromSources([]Source{
		{Path: tmpDir, Type: SourceTypeDirectory},
	}, true)
	require.NoError(t, err)
	require.Len(t, rs.Portals, 1)
	require.NotNil(t, rs.Portals[0].RbacEnabled)
	assert.True(t, *rs.Portals[0].RbacEnabled)
}

func TestLoaderUsesTemplatesAcrossFileAndSTDINSources(t *testing.T) {
	tmpDir := t.TempDir()
	templatesFile := filepath.Join(tmpDir, "templates.yaml")
	require.NoError(t, os.WriteFile(templatesFile, []byte(`
_templates:
  standard-portal:
    authentication_enabled: true
`), 0o600))

	reader, writer, err := os.Pipe()
	require.NoError(t, err)
	originalStdin := os.Stdin
	os.Stdin = reader
	t.Cleanup(func() {
		os.Stdin = originalStdin
		require.NoError(t, reader.Close())
	})
	_, err = writer.Write([]byte(`
portals:
  - _extends: standard-portal
    ref: docs
    name: Docs
`))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	rs, err := NewWithBaseDir(tmpDir).LoadFromSources([]Source{
		{Path: templatesFile, Type: SourceTypeFile},
		{Path: "-", Type: SourceTypeSTDIN},
	}, false)
	require.NoError(t, err)
	require.Len(t, rs.Portals, 1)
	require.NotNil(t, rs.Portals[0].AuthenticationEnabled)
	assert.True(t, *rs.Portals[0].AuthenticationEnabled)
}

func TestLoaderDeepMergesAIGatewayPolicyTemplates(t *testing.T) {
	t.Parallel()

	path := writeLoaderTestFile(t, `
_templates:
  corporate-oidc-config:
    issuer: https://auth.example.com
    auth_methods: [session]
    claims:
      email: email
      subject: sub
    optional_setting: inherited
    type_conflict:
      nested: inherited
  standard-oidc-policy:
    type: openid-connect
    enabled: true
    global: false
    config:
      _extends: corporate-oidc-config

ai_gateways:
  - ref: shared-gateway
    display_name: Shared Gateway
    policies:
      - _extends: standard-oidc-policy
        ref: payments-oidc
        name: payments-oidc
        display_name: Payments OIDC
        config:
          auth_methods: [bearer]
          claims:
            groups: groups
          optional_setting: null
          type_conflict: replaced
          groups_required: [payments-api-users]
      - _extends: standard-oidc-policy
        ref: reporting-oidc
        name: reporting-oidc
        display_name: Reporting OIDC
        config:
          groups_required: [reporting-api-users]
`)

	rs, err := New().LoadFromSources([]Source{{Path: path, Type: SourceTypeFile}}, false)
	require.NoError(t, err)
	require.Len(t, rs.AIGatewayPolicies, 2)

	payments := rs.AIGatewayPolicies[0].Config
	assert.Equal(t, "https://auth.example.com", payments["issuer"])
	assert.Equal(t, []any{"bearer"}, payments["auth_methods"])
	assert.Equal(t, map[string]any{
		"email": "email", "subject": "sub", "groups": "groups",
	}, payments["claims"])
	assert.Nil(t, payments["optional_setting"])
	assert.Equal(t, "replaced", payments["type_conflict"])
	assert.Equal(t, []any{"payments-api-users"}, payments["groups_required"])

	reporting := rs.AIGatewayPolicies[1].Config
	assert.Equal(t, []any{"session"}, reporting["auth_methods"])
	assert.Equal(t, "inherited", reporting["optional_setting"])
	assert.Equal(t, map[string]any{"nested": "inherited"}, reporting["type_conflict"])
	assert.Equal(t, []any{"reporting-api-users"}, reporting["groups_required"])
}

func TestLoaderRejectsInvalidTemplateReferences(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{
			name: "unknown template",
			input: `
portals:
  - _extends: missing-portal
    ref: docs
    name: Docs
`,
			wantErr: `unknown template "missing-portal" referenced at portals[0]`,
		},
		{
			name: "non-configuration-block template",
			input: `
_templates:
  invalid: [one, two]
portals:
  - _extends: invalid
    ref: docs
    name: Docs
`,
			wantErr: `template "invalid" must be a configuration block`,
		},
		{
			name: "non-string reference",
			input: `
_templates:
  standard: {authentication_enabled: true}
portals:
  - _extends: [standard]
    ref: docs
    name: Docs
`,
			wantErr: `_extends at portals[0] must name one template`,
		},
		{
			name: "extends in defaults",
			input: `
_templates:
  standard: {namespace: shared}
_defaults:
  _extends: standard
`,
			wantErr: `_extends is not supported inside _defaults at _defaults._extends`,
		},
		{
			name: "extends in nested defaults",
			input: `
_templates:
  standard: {namespace: shared}
_defaults:
  kongctl:
    _extends: standard
`,
			wantErr: `_extends is not supported inside _defaults at _defaults.kongctl._extends`,
		},
		{
			name: "nested template registry",
			input: `
ai_gateways:
  - ref: gateway
    name: gateway
    display_name: Gateway
    config:
      _templates:
        nested: {enabled: true}
`,
			wantErr: `_templates is only supported at the document root, not at ai_gateways[0].config`,
		},
		{
			name: "duplicate template registry",
			input: `
_templates:
  first: {authentication_enabled: true}
_templates:
  second: {rbac_enabled: true}
`,
			wantErr: `duplicate _templates key`,
		},
		{
			name: "transitive cycle",
			input: `
_templates:
  first:
    _extends: second
  second:
    nested:
      _extends: third
  third:
    _extends: first
portals:
  - _extends: first
    ref: docs
    name: Docs
`,
			wantErr: "template inheritance cycle detected: first -> second -> third -> first",
		},
		{
			name: "unused template with unknown base",
			input: `
_templates:
  unused:
    _extends: missing
`,
			wantErr: `unknown template "missing" referenced at _templates.unused`,
		},
		{
			name: "unused template cycle",
			input: `
_templates:
  first:
    _extends: second
  second:
    _extends: first
`,
			wantErr: "template inheritance cycle detected: first -> second -> first",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			path := writeLoaderTestFile(t, tt.input)
			_, err := New().LoadFromSources([]Source{{Path: path, Type: SourceTypeFile}}, false)
			require.ErrorContains(t, err, tt.wantErr)
			require.ErrorContains(t, err, path)
		})
	}
}

func TestLoaderRejectsEnvironmentTemplateReferenceWithoutLeakingPlaceholder(t *testing.T) {
	t.Parallel()

	path := writeLoaderTestFile(t, `
portals:
  - _extends: !env TEMPLATE_NAME
    ref: docs
    name: Docs
`)

	_, err := New().LoadFromSources([]Source{{Path: path, Type: SourceTypeFile}}, false)
	require.ErrorContains(t, err, `_extends at portals[0] must name one template`)
	assert.NotContains(t, err.Error(), tags.EnvPlaceholderPrefix)
}

func TestLoaderRejectsDuplicateTemplatesAcrossSourceFiles(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	first := filepath.Join(tmpDir, "first.yaml")
	second := filepath.Join(tmpDir, "second.yaml")
	for _, path := range []string{first, second} {
		require.NoError(t, os.WriteFile(path, []byte(`
_templates:
  standard-portal:
    authentication_enabled: true
`), 0o600))
	}

	_, err := NewWithBaseDir(tmpDir).LoadFromSources([]Source{
		{Path: first, Type: SourceTypeFile},
		{Path: second, Type: SourceTypeFile},
	}, false)
	require.ErrorContains(t, err, `duplicate template "standard-portal"`)
	require.ErrorContains(t, err, first)
	require.ErrorContains(t, err, second)
}

func TestLoaderRejectsDuplicateTemplatesWithinSourceFile(t *testing.T) {
	t.Parallel()

	path := writeLoaderTestFile(t, `
_templates:
  standard-portal:
    authentication_enabled: true
  standard-portal:
    rbac_enabled: true
`)

	_, err := New().LoadFromSources([]Source{{Path: path, Type: SourceTypeFile}}, false)
	require.ErrorContains(t, err, `duplicate template "standard-portal"`)
	require.ErrorContains(t, err, path)
}

func TestLoaderReportsTemplateDefinitionForInheritedSchemaError(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	templatesFile := filepath.Join(tmpDir, "templates.yaml")
	require.NoError(t, os.WriteFile(templatesFile, []byte(`
_templates:
  standard-portal:
    authentication_enabledd: true
`), 0o600))
	portalsFile := filepath.Join(tmpDir, "portals.yaml")
	require.NoError(t, os.WriteFile(portalsFile, []byte(`
portals:
  - _extends: standard-portal
    ref: docs
    name: Docs
`), 0o600))

	_, err := NewWithBaseDir(tmpDir).LoadFromSources([]Source{
		{Path: templatesFile, Type: SourceTypeFile},
		{Path: portalsFile, Type: SourceTypeFile},
	}, false)
	require.ErrorContains(t, err, portalsFile)
	require.ErrorContains(t, err, `template "standard-portal" defined in `+templatesFile)
	require.ErrorContains(t, err, "authentication_enabledd")
}

func TestLoaderReportsTemplateDefinitionForInvalidInheritedEnvField(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	templatesFile := filepath.Join(tmpDir, "templates.yaml")
	require.NoError(t, os.WriteFile(templatesFile, []byte(`
_templates:
  standard-portal:
    authentication_enabled: !env PORTAL_AUTHENTICATION_ENABLED
`), 0o600))
	portalsFile := filepath.Join(tmpDir, "portals.yaml")
	require.NoError(t, os.WriteFile(portalsFile, []byte(`
portals:
  - _extends: standard-portal
    ref: docs
    name: Docs
`), 0o600))

	_, err := NewWithBaseDir(tmpDir).LoadFromSources([]Source{
		{Path: templatesFile, Type: SourceTypeFile},
		{Path: portalsFile, Type: SourceTypeFile},
	}, false)
	require.ErrorContains(t, err, "!env currently supports string-typed fields only")
	require.ErrorContains(t, err, `template "standard-portal" defined in `+templatesFile)
}

func TestLoaderRequiresStringTemplateIdentifiers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{
			name: "template name",
			input: `
_templates:
  123:
    authentication_enabled: true
`,
			wantErr: "template names must be non-empty strings",
		},
		{
			name: "extends value",
			input: `
_templates:
  standard:
    authentication_enabled: true
portals:
  - _extends: 123
    ref: docs
    name: Docs
`,
			wantErr: "_extends at portals[0] must name one template",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			path := writeLoaderTestFile(t, tt.input)
			_, err := New().LoadFromSources([]Source{{Path: path, Type: SourceTypeFile}}, false)
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestLoaderResolvesTemplateTagsInDefinitionContext(t *testing.T) {
	t.Setenv("PORTAL_DESCRIPTION", "Description from the environment")

	tmpDir := t.TempDir()
	templateDir := filepath.Join(tmpDir, "shared")
	consumerDir := filepath.Join(tmpDir, "portals")
	require.NoError(t, os.MkdirAll(templateDir, 0o700))
	require.NoError(t, os.MkdirAll(consumerDir, 0o700))
	require.NoError(t, os.WriteFile(
		filepath.Join(templateDir, "portal-name.txt"),
		[]byte("Shared Developer Portal"),
		0o600,
	))
	templatesFile := filepath.Join(templateDir, "templates.yaml")
	require.NoError(t, os.WriteFile(templatesFile, []byte(`
_templates:
  standard-portal:
    name: !file portal-name.txt
    description: !env PORTAL_DESCRIPTION
`), 0o600))
	portalsFile := filepath.Join(consumerDir, "portals.yaml")
	require.NoError(t, os.WriteFile(portalsFile, []byte(`
portals:
  - _extends: standard-portal
    ref: docs
`), 0o600))

	rs, err := NewWithBaseDir(tmpDir).LoadFromSources([]Source{
		{Path: templatesFile, Type: SourceTypeFile},
		{Path: portalsFile, Type: SourceTypeFile},
	}, false)
	require.NoError(t, err)
	require.Len(t, rs.Portals, 1)
	assert.Equal(t, "Shared Developer Portal", rs.Portals[0].Name)
	require.NotNil(t, rs.Portals[0].Description)
	assert.Equal(t, "Description from the environment", *rs.Portals[0].Description)
	assert.Equal(t, "__ENV__:PORTAL_DESCRIPTION", rs.GetEnvSources("docs")["/description"])
}

func TestLoaderPreservesSecretSourcesInheritedFromTemplate(t *testing.T) {
	t.Parallel()

	path := writeLoaderTestFile(t, `
_templates:
  openai-provider:
    name: openai
    type: openai
    display_name: OpenAI
    config:
      auth:
        type: basic
        headers:
          - name: Authorization
            value: !secret {source: !env OPENAI_API_KEY}
ai_gateways:
  - ref: gateway
    name: gateway
    display_name: Gateway
    model_providers:
      - _extends: openai-provider
        ref: provider
`)

	rs, err := New().LoadFromSources([]Source{{Path: path, Type: SourceTypeFile}}, false)
	require.NoError(t, err)
	declaration := rs.GetSecretSources("provider")["/config/auth/headers/0/value"]
	require.Len(t, declaration.Expression.Parts, 1)
	assert.Equal(t, "OPENAI_API_KEY", declaration.Expression.Parts[0].Source.Reference)
}

func TestLoaderCapturesChildSyncScopeInheritedFromTemplate(t *testing.T) {
	t.Parallel()

	path := writeLoaderTestFile(t, `
_templates:
  managed-portal:
    pages: []
portals:
  - _extends: managed-portal
    ref: docs
    name: Docs
`)

	rs, err := New().LoadFromSources([]Source{{Path: path, Type: SourceTypeFile}}, false)
	require.NoError(t, err)
	require.True(t, rs.SyncScope.ChildInScope(
		resources.ResourceTypePortal,
		"docs",
		resources.ResourceTypePortalPage,
	))
}

func TestLoaderExpandsTemplateForNestedAndRootPortalPages(t *testing.T) {
	t.Parallel()

	path := writeLoaderTestFile(t, `
_templates:
  published-page:
    visibility: public
    status: published
    content: Shared page content
portals:
  - ref: docs
    name: Docs
    pages:
      - _extends: published-page
        ref: getting-started
        slug: getting-started
        title: Getting Started
portal_pages:
  - _extends: published-page
    ref: authentication
    portal: docs
    slug: authentication
    title: Authentication
`)

	rs, err := New().LoadFromSources([]Source{{Path: path, Type: SourceTypeFile}}, false)
	require.NoError(t, err)
	require.Len(t, rs.PortalPages, 2)
	for _, page := range rs.PortalPages {
		assert.Equal(t, "Shared page content", page.Content)
		require.NotNil(t, page.Visibility)
		assert.Equal(t, "public", string(*page.Visibility))
		require.NotNil(t, page.Status)
		assert.Equal(t, "published", string(*page.Status))
	}
}
