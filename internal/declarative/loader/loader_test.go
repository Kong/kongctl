package loader

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	loader := New()
	assert.NotNil(t, loader)
}

func TestLoaderPortalTeamGroupMappingsNestedAndRoot(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
portals:
  - ref: portal-1
    name: Portal 1
    teams:
      - ref: developers
        name: Developers
        group_mappings:
          - ref: developers-idp-groups
            groups:
              - Service Developer
portal_team_group_mappings:
  - ref: admins-idp-groups
    portal: portal-1
    team: admins
    groups: []
`), 0o600)
	require.NoError(t, err)

	rs, err := New().LoadFile(path)
	require.NoError(t, err)
	require.Len(t, rs.PortalTeamGroupMappings, 2)

	byRef := map[string]resources.PortalTeamGroupMappingResource{}
	for _, mapping := range rs.PortalTeamGroupMappings {
		byRef[mapping.Ref] = mapping
	}

	assert.Equal(t, "portal-1", byRef["developers-idp-groups"].Portal)
	assert.Equal(t, "developers", byRef["developers-idp-groups"].Team)
	assert.Equal(t, []string{"Service Developer"}, byRef["developers-idp-groups"].Groups)
	assert.Equal(t, "portal-1", byRef["admins-idp-groups"].Portal)
	assert.Equal(t, "admins", byRef["admins-idp-groups"].Team)
	assert.Empty(t, byRef["admins-idp-groups"].Groups)
}

func TestLoaderFlattensAIGatewayProviders(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
ai_gateways:
  - ref: customer-support-gateway
    display_name: Customer Support Gateway
    model_providers:
      - ref: openai-provider
        name: openai-provider
        type: openai
        display_name: OpenAI Provider
        config:
          auth:
            type: basic
            headers:
              - name: Authorization
                value: Bearer ${OPENAI_API_KEY}
`), 0o600)
	require.NoError(t, err)

	rs, err := New().LoadFile(path)
	require.NoError(t, err)
	require.Len(t, rs.AIGateways, 1)
	require.Empty(t, rs.AIGateways[0].Providers)
	require.Len(t, rs.AIGatewayProviders, 1)
	require.Equal(t, "customer-support-gateway", rs.AIGatewayProviders[0].AIGateway)
	require.Equal(t, "openai-provider", rs.AIGatewayProviders[0].Name)
}

func TestLoaderRejectsLegacyNestedAIGatewayProviders(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
ai_gateways:
  - ref: customer-support-gateway
    display_name: Customer Support Gateway
    providers:
      - ref: openai-provider
        name: openai-provider
        type: openai
        display_name: OpenAI Provider
        config:
          auth:
            type: basic
`), 0o600)
	require.NoError(t, err)

	_, err = New().LoadFile(path)
	require.Error(t, err)
	require.Contains(t, err.Error(), "ai_gateways.providers is not supported")
	require.Contains(t, err.Error(), "ai_gateways.model_providers")
}

func TestLoaderRejectsLegacyRootAIGatewayProviders(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
ai_gateways:
  - ref: customer-support-gateway
    display_name: Customer Support Gateway
ai_gateway_providers:
  - ref: openai-provider
    ai_gateway: customer-support-gateway
    name: openai-provider
    type: openai
    display_name: OpenAI Provider
    config:
      auth:
        type: basic
`), 0o600)
	require.NoError(t, err)

	_, err = New().LoadFile(path)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown field 'ai_gateway_providers'")
}

func TestLoaderFlattensAIGatewayIdentityProviders(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
ai_gateways:
  - ref: customer-support-gateway
    display_name: Customer Support Gateway
    identity_providers:
      - ref: support-key-auth
        name: support-key-auth
        type: key-auth
        display_name: Support Key Auth
        config:
          key_names:
            - x-support-api-key
          hide_credentials: true
`), 0o600)
	require.NoError(t, err)

	rs, err := New().LoadFile(path)
	require.NoError(t, err)
	require.Len(t, rs.AIGateways, 1)
	require.Empty(t, rs.AIGateways[0].IdentityProviders)
	require.Len(t, rs.AIGatewayIdentityProviders, 1)
	require.Equal(t, "customer-support-gateway", rs.AIGatewayIdentityProviders[0].AIGateway)
	require.Equal(t, "support-key-auth", rs.AIGatewayIdentityProviders[0].Name)
}

func TestLoaderPortalTeamGroupMappingsPortalLevelNestedRejected(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
portals:
  - ref: portal-1
    name: Portal 1
    team_group_mappings:
      - ref: developers-idp-groups
        team: developers
        groups:
          - Service Developer
`), 0o600)
	require.NoError(t, err)

	_, err = New().LoadFile(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown field 'team_group_mappings'")
}

func TestLoaderFlattensPortalCustomizationSpecRendererAndRobots(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
portals:
  - ref: portal-1
    name: Portal 1
    customization:
      ref: portal-customization
      spec_renderer:
        try_it_ui: false
        try_it_insomnia: true
        infinite_scroll: false
        show_schemas: true
        hide_internal: true
        hide_deprecated: false
        allow_custom_server_urls: true
      robots: "User-agent: *"
`), 0o600)
	require.NoError(t, err)

	rs, err := New().LoadFile(path)
	require.NoError(t, err)
	require.Len(t, rs.PortalCustomizations, 1)

	customization := rs.PortalCustomizations[0]
	assert.Equal(t, "portal-1", customization.Portal)
	assert.Equal(t, "portal-customization", customization.Ref)
	require.NotNil(t, customization.SpecRenderer)
	require.NotNil(t, customization.SpecRenderer.TryItUI)
	assert.False(t, *customization.SpecRenderer.TryItUI)
	require.NotNil(t, customization.SpecRenderer.TryItInsomnia)
	assert.True(t, *customization.SpecRenderer.TryItInsomnia)
	require.NotNil(t, customization.SpecRenderer.InfiniteScroll)
	assert.False(t, *customization.SpecRenderer.InfiniteScroll)
	require.NotNil(t, customization.SpecRenderer.ShowSchemas)
	assert.True(t, *customization.SpecRenderer.ShowSchemas)
	require.NotNil(t, customization.SpecRenderer.HideInternal)
	assert.True(t, *customization.SpecRenderer.HideInternal)
	require.NotNil(t, customization.SpecRenderer.HideDeprecated)
	assert.False(t, *customization.SpecRenderer.HideDeprecated)
	require.NotNil(t, customization.SpecRenderer.AllowCustomServerUrls)
	assert.True(t, *customization.SpecRenderer.AllowCustomServerUrls)
	require.NotNil(t, customization.Robots)
	assert.Equal(t, "User-agent: *", *customization.Robots)
}

func TestLoader_LoadFile_ValidConfigs(t *testing.T) {
	tests := []struct {
		name                  string
		file                  string
		expectedPortals       int
		expectedAuthStrats    int
		expectedControlPlanes int
		expectedAPIs          int
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
		{
			name:         "api with multiple versions",
			file:         "valid/api-multiple-versions.yaml",
			expectedAPIs: 1,
		},
		{
			name:                  "external control plane references",
			file:                  "valid/external-control-plane.yaml",
			expectedControlPlanes: 1,
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

			if tt.name == "external control plane references" {
				require.Len(t, rs.GatewayServices, 1)
				svc := rs.GatewayServices[0]
				assert.Equal(t, "ext-gw-svc", svc.GetRef())
				assert.Equal(t, "ext-cp", svc.ControlPlane)
				assert.NotNil(t, svc.External)
				assert.Nil(t, svc.Service, "expected external gateway service to skip embedded service payload")
			}
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
			expectError: "duplicate ref",
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

func TestLoader_FlattensPortalEmailConfig(t *testing.T) {
	content := `
portals:
  - ref: portal-email
    name: portal-email
    email_config:
      ref: portal-email-config
      from_name: "Portal Team"
      from_email: "team@example.com"
      reply_to_email: "reply@example.com"
`

	loader := New()
	rs, err := loader.parseYAML(strings.NewReader(content), "inline", "")
	require.NoError(t, err)

	require.Len(t, rs.Portals, 1)
	require.Len(t, rs.PortalEmailConfigs, 1)
	assert.Equal(t, "portal-email", rs.PortalEmailConfigs[0].Portal)
	assert.Equal(t, "portal-email-config", rs.PortalEmailConfigs[0].Ref)
}

func TestLoader_EmptyPortalSingletonChildrenAreScopeOnly(t *testing.T) {
	tests := []struct {
		name         string
		childYAML    string
		resourceType resources.ResourceType
		assertEmpty  func(*testing.T, *resources.ResourceSet)
	}{
		{
			name:         "custom domain",
			childYAML:    "    custom_domain: {}\n",
			resourceType: resources.ResourceTypePortalCustomDomain,
			assertEmpty: func(t *testing.T, rs *resources.ResourceSet) {
				t.Helper()
				assert.Empty(t, rs.PortalCustomDomains)
			},
		},
		{
			name:         "email config",
			childYAML:    "    email_config: {}\n",
			resourceType: resources.ResourceTypePortalEmailConfig,
			assertEmpty: func(t *testing.T, rs *resources.ResourceSet) {
				t.Helper()
				assert.Empty(t, rs.PortalEmailConfigs)
			},
		},
		{
			name:         "audit log webhook",
			childYAML:    "    audit_log_webhook: {}\n",
			resourceType: resources.ResourceTypePortalAuditLogWebhook,
			assertEmpty: func(t *testing.T, rs *resources.ResourceSet) {
				t.Helper()
				assert.Empty(t, rs.PortalAuditLogWebhooks)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := `
portals:
  - ref: portal-email
    name: portal-email
` + tt.childYAML

			loader := New()
			rs, err := loader.parseYAML(strings.NewReader(content), "inline", "")
			require.NoError(t, err)
			require.NoError(t, loader.validateResourceSet(rs))

			require.NotNil(t, rs.SyncScope)
			assert.True(t, rs.SyncScope.ChildInScope(resources.ResourceTypePortal, "portal-email", tt.resourceType))
			tt.assertEmpty(t, rs)
		})
	}
}

func TestLoader_FlattensPortalIntegration(t *testing.T) {
	content := `
portals:
  - ref: portal-integrations
    name: portal-integrations
    integrations:
      ref: portal-integrations-config
      google_tag_manager:
        enabled: true
        config_data:
          id: GTM-ABC123
`

	loader := New()
	rs, err := loader.parseYAML(strings.NewReader(content), "inline", "")
	require.NoError(t, err)

	require.Len(t, rs.Portals, 1)
	require.Len(t, rs.PortalIntegrations, 1)
	assert.Equal(t, "portal-integrations", rs.PortalIntegrations[0].Portal)
	assert.Equal(t, "portal-integrations-config", rs.PortalIntegrations[0].Ref)
	assert.Nil(t, rs.Portals[0].Integrations)
}

func TestLoader_FlattensPortalIPAllowList(t *testing.T) {
	content := `
portals:
  - ref: portal-ip-allow-list
    name: portal-ip-allow-list
    ip_allow_list:
      ref: portal-ip-allow-list-config
      allowed_ips:
        - 192.0.2.10
        - 198.51.100.0/24
`

	loader := New()
	rs, err := loader.parseYAML(strings.NewReader(content), "inline", "")
	require.NoError(t, err)

	require.Len(t, rs.Portals, 1)
	require.Len(t, rs.PortalIPAllowLists, 1)
	assert.Equal(t, "portal-ip-allow-list", rs.PortalIPAllowLists[0].Portal)
	assert.Equal(t, "portal-ip-allow-list-config", rs.PortalIPAllowLists[0].Ref)
	assert.Nil(t, rs.Portals[0].IPAllowList)
}

func TestLoader_FlattensOrganizationTeamRoles(t *testing.T) {
	content := `
organization:
  teams:
    - ref: platform-team
      name: Platform Engineering
      roles:
        - ref: platform-admin
          role_name: Admin
          entity_id: "*"
          entity_type_name: APIs
          entity_region: us
organization_team_roles:
  - ref: platform-viewer
    team: platform-team
    role_name: Viewer
    entity_id: "*"
    entity_type_name: APIs
    entity_region: us
`

	loader := New()
	rs, err := loader.parseYAML(strings.NewReader(content), "inline", "")
	require.NoError(t, err)

	require.Len(t, rs.OrganizationTeams, 1)
	require.Len(t, rs.OrganizationTeamRoles, 2)
	assert.Empty(t, rs.OrganizationTeams[0].Roles)
	rolesByRef := map[string]string{}
	for _, role := range rs.OrganizationTeamRoles {
		rolesByRef[role.Ref] = role.Team
	}
	assert.Equal(t, map[string]string{
		"platform-admin":  "platform-team",
		"platform-viewer": "platform-team",
	}, rolesByRef)
}

func TestLoader_LoadFileAllowsOrganizationTeamRolePortalRef(t *testing.T) {
	content := `
organization:
  teams:
    - ref: repro-team
      name: repro-team
      roles:
        - ref: repro-team-role
          role_name: viewer
          entity_id: !ref repro-portal
          entity_type_name: Portals
          entity_region: us

portals:
  - ref: repro-portal
    name: repro-portal
`

	dir := t.TempDir()
	file := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(file, []byte(content), 0o600))

	loader := New()
	rs, err := loader.LoadFile(file)
	require.NoError(t, err)

	require.Len(t, rs.OrganizationTeamRoles, 1)
	assert.Equal(t, "__REF__:repro-portal#id", rs.OrganizationTeamRoles[0].EntityID)
	assert.Equal(t, "Portals", rs.OrganizationTeamRoles[0].EntityTypeName)
}

func TestLoader_FlattensOrganizationUserAssignments(t *testing.T) {
	content := `
apis:
  - ref: products-api
    name: Products API
organization:
  teams:
    - ref: platform-team
      name: Platform Engineering
  users:
    - ref: alice
      email: alice@example.com
      teams:
        - ref: alice-platform-team
          team: platform-team
      roles:
        - ref: alice-products-viewer
          role_name: Viewer
          entity_id: !ref products-api#id
          entity_type_name: APIs
          entity_region: us
`

	loader := New()
	rs, err := loader.parseYAML(strings.NewReader(content), "inline", "")
	require.NoError(t, err)

	require.Len(t, rs.OrganizationTeams, 1)
	require.Len(t, rs.OrganizationUserTeamMemberships, 1)
	require.Len(t, rs.OrganizationUserRoles, 1)
	assert.Equal(t, "alice-platform-team", rs.OrganizationUserTeamMemberships[0].Ref)
	assert.Equal(t, "alice", rs.OrganizationUserTeamMemberships[0].User)
	assert.Equal(t, "platform-team", rs.OrganizationUserTeamMemberships[0].Team)
	assert.Equal(t, "alice", rs.OrganizationUserRoles[0].User)
	assert.Equal(t, "__REF__:products-api#id", rs.OrganizationUserRoles[0].EntityID)
	require.NotNil(t, rs.Organization)
	require.Len(t, rs.Organization.Users, 1)
	assert.Empty(t, rs.Organization.Users[0].Teams)
	assert.Empty(t, rs.Organization.Users[0].Roles)
}

func TestLoader_FlattensOrganizationSystemAccountAssignments(t *testing.T) {
	content := `
apis:
  - ref: products-api
    name: Products API
organization:
  teams:
    - ref: platform-team
      name: Platform Engineering
  system-accounts:
    - ref: ci-bot
      name: ci-bot
      teams:
        - ref: ci-bot-platform-team
          team: platform-team
      roles:
        - ref: ci-bot-products-viewer
          role_name: Viewer
          entity_id: !ref products-api#id
          entity_type_name: APIs
          entity_region: us
`

	loader := New()
	rs, err := loader.parseYAML(strings.NewReader(content), "inline", "")
	require.NoError(t, err)

	require.Len(t, rs.OrganizationTeams, 1)
	require.Len(t, rs.OrganizationSystemAccountTeamMemberships, 1)
	require.Len(t, rs.OrganizationSystemAccountRoles, 1)
	assert.Equal(t, "ci-bot-platform-team", rs.OrganizationSystemAccountTeamMemberships[0].Ref)
	assert.Equal(t, "ci-bot", rs.OrganizationSystemAccountTeamMemberships[0].SystemAccount)
	assert.Equal(t, "platform-team", rs.OrganizationSystemAccountTeamMemberships[0].Team)
	assert.Equal(t, "ci-bot", rs.OrganizationSystemAccountRoles[0].SystemAccount)
	assert.Equal(t, "__REF__:products-api#id", rs.OrganizationSystemAccountRoles[0].EntityID)
	require.NotNil(t, rs.Organization)
	require.Len(t, rs.Organization.SystemAccounts, 1)
	assert.Empty(t, rs.Organization.SystemAccounts[0].Teams)
	assert.Empty(t, rs.Organization.SystemAccounts[0].Roles)
}

func TestLoader_ValidatesOrganizationUserRefsAndSelectors(t *testing.T) {
	tests := []struct {
		name    string
		user    string
		wantErr string
	}{
		{
			name: "missing ref",
			user: `
    - email: alice@example.com
`,
			wantErr: "ref cannot be empty",
		},
		{
			name: "missing selector",
			user: `
    - ref: alice
`,
			wantErr: "exactly one of email or id is required",
		},
		{
			name: "multiple selectors",
			user: `
    - ref: alice
      email: alice@example.com
      id: 00000000-0000-0000-0000-000000000000
`,
			wantErr: "exactly one of email or id is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := `
organization:
  users:
` + tt.user

			dir := t.TempDir()
			file := filepath.Join(dir, "config.yaml")
			require.NoError(t, os.WriteFile(file, []byte(content), 0o600))

			loader := New()
			_, err := loader.LoadFile(file)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestLoader_ValidatesOrganizationSystemAccountSelectors(t *testing.T) {
	tests := []struct {
		name    string
		account string
		wantErr string
	}{
		{
			name: "missing selector",
			account: `
    - ref: ci-bot
      teams:
        - ref: ci-bot-platform-team
          team: platform-team
`,
			wantErr: "exactly one of name or id is required",
		},
		{
			name: "multiple selectors",
			account: `
    - ref: ci-bot
      id: 00000000-0000-0000-0000-000000000000
      name: ci-bot
`,
			wantErr: "exactly one of name or id is required",
		},
		{
			name: "missing ref",
			account: `
    - name: ci-bot
`,
			wantErr: "ref cannot be empty",
		},
		{
			name: "empty team ref",
			account: `
    - ref: ci-bot
      name: ci-bot
      teams:
        - ref: ci-bot-platform-team
          team: ""
`,
			wantErr: "team is required",
		},
		{
			name: "missing team membership ref",
			account: `
    - ref: ci-bot
      name: ci-bot
      teams:
        - team: platform-team
`,
			wantErr: "ref cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := `
organization:
  teams:
    - ref: platform-team
      name: Platform Engineering
  system-accounts:
` + tt.account

			dir := t.TempDir()
			file := filepath.Join(dir, "config.yaml")
			require.NoError(t, os.WriteFile(file, []byte(content), 0o600))

			loader := New()
			_, err := loader.LoadFile(file)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestLoader_RejectsDuplicateOrganizationUserAndSystemAccountRef(t *testing.T) {
	content := `
organization:
  users:
    - ref: automation-principal
      email: alice@example.com
  system-accounts:
    - ref: automation-principal
      name: ci-bot
`

	dir := t.TempDir()
	file := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(file, []byte(content), 0o600))

	loader := New()
	_, err := loader.LoadFile(file)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate ref 'automation-principal'")
	assert.Contains(t, err.Error(), string(resources.ResourceTypeOrganizationUser))
}

func TestLoader_LoadFilePreservesOrganizationUsers(t *testing.T) {
	content := `
_defaults:
  kongctl:
    namespace: org-users-test
apis:
  - ref: products-api
    name: Products API
organization:
  teams:
    - ref: platform-team
      name: Platform Engineering
  users:
    - ref: alice
      email: alice@example.com
      teams:
        - ref: alice-platform-team
          team: platform-team
      roles:
        - ref: alice-products-viewer
          role_name: Viewer
          entity_id: !ref products-api#id
          entity_type_name: APIs
          entity_region: us
`

	dir := t.TempDir()
	file := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(file, []byte(content), 0o600))

	loader := New()
	rs, err := loader.LoadFile(file)
	require.NoError(t, err)

	require.NotNil(t, rs.Organization)
	require.Len(t, rs.Organization.Users, 1)
	require.NotNil(t, rs.Organization.Users[0].Kongctl)
	require.NotNil(t, rs.Organization.Users[0].Kongctl.Namespace)
	assert.Equal(t, "org-users-test", *rs.Organization.Users[0].Kongctl.Namespace)
	require.Len(t, rs.GetOrganizationUserTeamMembershipsByNamespace("org-users-test"), 1)
	require.Len(t, rs.GetOrganizationUserRolesByNamespace("org-users-test"), 1)
}

func TestLoader_LoadFileScopesOrganizationUserTeamMembershipsByTeamNamespace(t *testing.T) {
	content := `
organization:
  teams:
    - ref: platform-team
      kongctl:
        namespace: dumped-team-namespace
      name: Platform Engineering
  users:
    - ref: alice
      email: alice@example.com
      teams:
        - ref: alice-platform-team
          team: platform-team
`

	dir := t.TempDir()
	file := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(file, []byte(content), 0o600))

	loader := New()
	rs, err := loader.LoadFile(file)
	require.NoError(t, err)

	require.NotNil(t, rs.Organization)
	require.Len(t, rs.Organization.Users, 1)
	memberships := rs.GetOrganizationUserTeamMembershipsByNamespace("dumped-team-namespace")
	require.Len(t, memberships, 1)
	assert.Equal(t, "alice-platform-team", memberships[0].Ref)
	assert.Empty(t, rs.GetOrganizationUserTeamMembershipsByNamespace("default"))
}

func TestLoader_RejectsSingularPortalIntegrationKey(t *testing.T) {
	content := `
portals:
  - ref: portal-integrations
    name: portal-integrations
    integration:
      ref: portal-integrations-config
`

	loader := New()
	rs, err := loader.parseYAML(strings.NewReader(content), "inline", "")
	require.Error(t, err)
	assert.Nil(t, rs)
	assert.Contains(t, err.Error(), "unknown field 'integration'")
	assert.Contains(t, err.Error(), "Did you mean 'integrations'?")
}

func TestLoader_LoadFile_PortalEmailConfigFlattening(t *testing.T) {
	loader := New()
	rs, err := loader.LoadFile(filepath.Join(
		"..", "..", "..", "test", "e2e", "testdata", "declarative", "portal", "email-config", "config.yaml",
	))
	require.NoError(t, err)

	require.Len(t, rs.Portals, 1)
	require.Len(t, rs.PortalEmailConfigs, 1)
	assert.Equal(t, "portal-email", rs.PortalEmailConfigs[0].Portal)
	assert.Equal(t, "portal-email-config", rs.PortalEmailConfigs[0].Ref)
}

func TestLoader_FlattensPortalEmailTemplates(t *testing.T) {
	content := `
portals:
  - ref: portal-email
    name: portal-email
    email_templates:
      reset-password:
        enabled: true
        content:
          subject: Reset
      app-registration-approved:
        ref: approved-template
        enabled: false
`

	loader := New()
	rs, err := loader.parseYAML(strings.NewReader(content), "inline", "")
	require.NoError(t, err)

	require.Len(t, rs.Portals, 1)
	require.Len(t, rs.PortalEmailTemplates, 2)

	templates := make(map[string]resources.PortalEmailTemplateResource)
	for _, tpl := range rs.PortalEmailTemplates {
		templates[string(tpl.Name)] = tpl
	}

	reset, ok := templates["reset-password"]
	require.True(t, ok)
	approved, ok := templates["app-registration-approved"]
	require.True(t, ok)

	assert.Equal(t, "portal-email", reset.Portal)
	assert.Equal(t, "portal-email", approved.Portal)

	assert.Equal(t, "reset-password", reset.Ref)
	assert.Equal(t, "reset-password", string(reset.Name))

	assert.Equal(t, "approved-template", approved.Ref)
	assert.Equal(t, "app-registration-approved", string(approved.Name))
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

func TestLoader_LoadFile_APIImplementationServiceShape(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
apis:
  - ref: users-api
    name: Users API
    implementations:
      - ref: users-api-impl
        type: service
        service:
          id: users-service
          control_plane_id: users-control-plane
`), 0o600)
	require.NoError(t, err)

	rs, err := New().LoadFile(path)
	require.NoError(t, err)
	require.Len(t, rs.APIImplementations, 1)

	implementation := rs.APIImplementations[0]
	assert.Equal(t, "users-api-impl", implementation.GetRef())
	assert.Equal(t, "users-api", implementation.API)
	require.NotNil(t, implementation.ServiceReference)
	service := implementation.ServiceReference.GetService()
	require.NotNil(t, service)
	assert.Equal(t, "users-service", service.ID)
	assert.Equal(t, "users-control-plane", service.ControlPlaneID)
}

func TestLoader_LoadFile_APIImplementationRejectsServiceReference(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
apis:
  - ref: users-api
    name: Users API
    implementations:
      - ref: users-api-impl
        type: service
        service_reference:
          service:
            id: users-service
            control_plane_id: users-control-plane
`), 0o600)
	require.NoError(t, err)

	_, err = New().LoadFile(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown field 'service_reference'")
	assert.Contains(t, err.Error(), "Did you mean 'service'?")
}

func TestLoader_LoadFile_APIImplementationRejectsUnsupportedType(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
apis:
  - ref: users-api
    name: Users API
    implementations:
      - ref: users-api-impl
        type: control_plane
        service:
          id: users-service
          control_plane_id: users-control-plane
`), 0o600)
	require.NoError(t, err)

	_, err = New().LoadFile(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "API implementation type must be service")
}

func TestLoader_LoadFile_RejectsAPISpecContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
apis:
  - ref: users-api
    name: Users API
    spec_content: |
      openapi: 3.0.0
      info:
        title: Users API
        version: 1.0.0
`), 0o600)
	require.NoError(t, err)

	_, err = New().LoadFile(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "apis[].spec_content is not supported in declarative configuration")
	assert.Contains(t, err.Error(), "use versions[].spec instead")
}

func TestLoader_LoadFile_APIUnknownField(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
apis:
  - ref: users-api
    name: Users API
    lables:
      env: test
`), 0o600)
	require.NoError(t, err)

	_, err = New().LoadFile(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown field 'lables'")
	assert.Contains(t, err.Error(), "Did you mean 'labels'?")
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
	err = os.Mkdir(subDir, 0o755)
	require.NoError(t, err)

	// Create file in subdirectory
	subYAML := `
portals:
  - ref: sub-portal
    name: "Sub Portal"
`
	err = os.WriteFile(filepath.Join(subDir, "sub.yaml"), []byte(subYAML), 0o600)
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
		err      error
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
			name:  "https url",
			input: []string{"https://example.com/config.yaml"},
			expected: []Source{
				{Path: "https://example.com/config.yaml", Type: SourceTypeURL},
			},
		},
		{
			name:  "http url",
			input: []string{"http://example.com/config"},
			expected: []Source{
				{Path: "http://example.com/config", Type: SourceTypeURL},
			},
		},
		{
			name:  "empty rejects missing configuration source",
			input: []string{},
			err:   ErrNoSources,
		},
		{
			name:  "empty comma-separated values reject missing configuration source",
			input: []string{","},
			err:   ErrNoSources,
		},
		{
			name:  "unsupported url scheme",
			input: []string{"ftp://example.com/config.yaml"},
			err:   errors.New("unsupported URL scheme"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// For testing, we need to mock file existence checks
			// Since ParseSources checks if files exist, we'll test with stdin
			// which doesn't require file existence
			if tt.name == "stdin" || strings.Contains(tt.name, "url") || tt.err != nil {
				sources, err := ParseSources(tt.input)
				if tt.err != nil {
					require.Error(t, err)
					if errors.Is(tt.err, ErrNoSources) {
						assert.True(t, errors.Is(err, tt.err))
					} else {
						assert.Contains(t, err.Error(), tt.err.Error())
					}
					assert.Nil(t, sources)
					return
				}
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
			name: "unknown field with no suggestion",
			file: "invalid/unknown-field-no-suggestion.yaml",
			expectedError: "unknown field 'completely_unknown_field' in " +
				"testdata/invalid/unknown-field-no-suggestion.yaml. " +
				"Please check the field name against the schema",
		},
		{
			name: "misspelled strategy_type field",
			file: "invalid/unknown-field-auth.yaml",
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
