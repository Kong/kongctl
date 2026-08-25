package loader

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeclarativeLoadSchemaRejectsUnknownFieldsAtFullPaths(t *testing.T) {
	tests := []struct {
		name  string
		yaml  string
		field string
		path  string
	}{
		{
			name: "custom unmarshaller nested resource",
			yaml: `
ai_gateways:
  - ref: example-gateway
    display_name: Example Gateway
    consumers:
      - ref: eason
        name: eason
        display_name: Eason
        type: api-key
        policies: []
        credentials:
          - ref: eason-key
            name: eason-key
            display_name: Eason API Key
            type: api-key
            key: discarded-secret
`,
			field: "key",
			path:  "ai_gateways[0].consumers[0].credentials[0].key",
		},
		{
			name: "custom unmarshaller root resource",
			yaml: `
ai_gateway_consumer_credentials:
  - ref: eason-key
    ai_gateway_consumer: eason
    name: eason-key
    display_name: Eason API Key
    type: api-key
    unexpected: value
`,
			field: "unexpected",
			path:  "ai_gateway_consumer_credentials[0].unexpected",
		},
		{
			name: "ordinary unmarshaller root resource",
			yaml: `
control_plane_data_plane_certificates:
  - ref: example-cert
    control_plane: example-control-plane
    cert: certificate
    unexpected: value
`,
			field: "unexpected",
			path:  "control_plane_data_plane_certificates[0].unexpected",
		},
		{
			name: "nested SDK model",
			yaml: `
control_planes:
  - ref: example-control-plane
    name: Example Control Plane
    proxy_urls:
      - host: proxy.example.com
        port: 443
        protocol: https
        hostname: proxy.example.com
`,
			field: "hostname",
			path:  "control_planes[0].proxy_urls[0].hostname",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New().parseYAML(strings.NewReader(tt.yaml), "manifest.yaml", ".")
			require.Error(t, err)
			assert.ErrorContains(t, err, "unknown field '"+tt.field+"'")
			assert.ErrorContains(t, err, tt.path)
		})
	}
}

func TestDeclarativeLoadSchemaSelectsUnionBranch(t *testing.T) {
	input := `
application_auth_strategies:
  - ref: key-auth
    name: Key Auth
    display_name: Key Auth
    strategy_type: key_auth
    configs:
      key_auth:
        key_names: [apikey]
      openid_connect:
        issuer: https://issuer.example.com
`

	_, err := New().parseYAML(strings.NewReader(input), "manifest.yaml", ".")
	require.Error(t, err)
	assert.ErrorContains(t, err, "unknown field 'openid_connect'")
	assert.ErrorContains(t, err, "application_auth_strategies[0].configs.openid_connect")
}

func TestDeclarativeLoadSchemaAcceptsGeminiServiceAccountAuth(t *testing.T) {
	t.Setenv("GCP_SERVICE_ACCOUNT_JSON", `{"type":"service_account"}`)
	input := `
ai_gateway_model_providers:
  - ref: gemini-prod
    name: gemini-prod
    display_name: Google Gemini Prod
    ai_gateway: ai-quickstart
    type: gemini
    config:
      auth:
        type: gcp
        service_account_json: !env GCP_SERVICE_ACCOUNT_JSON
`

	_, err := New().parseYAML(strings.NewReader(input), "manifest.yaml", ".")
	require.NoError(t, err)
}

func TestDeclarativeLoadSchemaHandlesNestedAIGatewayModelNameHeaderByType(t *testing.T) {
	input := `
ai_gateways:
  - ref: support-gateway
    name: support-gateway
    display_name: Support Gateway
    models:
      - ref: support-model
        type: model
        name: support-model
        display_name: Support Model
        formats:
          - type: openai
        config:
          route: {}
          model:
            name_header: false
        capabilities:
          - generate
        targets:
          - name: gpt-4o
            provider: support-openai
            config:
              type: openai
`

	t.Run("model", func(t *testing.T) {
		resourceSet, err := New().parseYAML(strings.NewReader(input), "manifest.yaml", ".")
		require.NoError(t, err)
		require.Len(t, resourceSet.AIGatewayModels, 1)
		model := resourceSet.AIGatewayModels[0].AIGatewayModelModel
		require.NotNil(t, model)
		require.NotNil(t, model.Config.Model)
		require.NotNil(t, model.Config.Model.NameHeader)
		require.False(t, *model.Config.Model.NameHeader)
	})

	t.Run("api", func(t *testing.T) {
		apiInput := strings.Replace(input, "type: model", "type: api", 1)
		_, err := New().parseYAML(strings.NewReader(apiInput), "manifest.yaml", ".")
		require.EqualError(
			t,
			err,
			`AI Gateway model field "config.model" is only supported when type is "model"`,
		)
	})
}

func TestDeclarativeLoadSchemaAcceptsAdditionalAuthStrategyConfigProperties(t *testing.T) {
	tests := []struct {
		name         string
		providerType string
		configYAML   string
		field        string
	}{
		{
			name:         "key auth",
			providerType: "key-auth",
			configYAML: `
      hide_credentials: true
      realm: Support API
`,
			field: "realm",
		},
		{
			name:         "OpenID Connect",
			providerType: "openid-connect",
			configYAML: `
      auth_methods: [bearer]
      cache_tokens_salt: support-cache-salt
      credential_claim: [sub]
      issuer: https://issuer.example.com
`,
			field: "credential_claim",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := fmt.Sprintf(`
ai_gateway_auth_strategies:
  - ref: support-idp
    ai_gateway: ai-quickstart
    name: support-idp
    display_name: Support Auth Strategy
    type: %s
    config:%s
`, tt.providerType, tt.configYAML)

			resourceSet, err := New().parseYAML(strings.NewReader(input), "manifest.yaml", ".")
			require.NoError(t, err)
			require.Len(t, resourceSet.AIGatewayAuthStrategies, 1)
			require.Contains(t, resourceSet.AIGatewayAuthStrategies[0].Config, tt.field)
		})
	}
}

func TestDeclarativeLoadSchemaAcceptsAuthStrategyAccessControlFields(t *testing.T) {
	input := `
ai_gateway_auth_strategies:
  - ref: support-oidc
    ai_gateway: ai-quickstart
    name: support-oidc
    display_name: Support OIDC
    type: openid-connect
    config:
      auth_methods: [bearer]
      cache_tokens_salt: support-cache-salt
      issuer: https://issuer.example.com
      consumer_groups_claim: [groups]
      consumer_groups_optional: false
      upstream_headers_claims: [sub]
      upstream_headers_names: [x-consumer-subject]
`

	resourceSet, err := New().parseYAML(strings.NewReader(input), "manifest.yaml", ".")
	require.NoError(t, err)
	require.Len(t, resourceSet.AIGatewayAuthStrategies, 1)
	config := resourceSet.AIGatewayAuthStrategies[0].Config
	require.Equal(t, []any{"groups"}, config["consumer_groups_claim"])
	require.Equal(t, false, config["consumer_groups_optional"])
	require.Equal(t, []any{"sub"}, config["upstream_headers_claims"])
	require.Equal(t, []any{"x-consumer-subject"}, config["upstream_headers_names"])
}

func TestLoaderRequiresOpenIDConnectAPIFields(t *testing.T) {
	tests := []struct {
		name       string
		configYAML string
		wantError  string
	}{
		{
			name: "issuer",
			configYAML: `
      cache_tokens_salt: support-cache-salt`,
			wantError: "config.issuer is required",
		},
		{
			name: "cache tokens salt",
			configYAML: `
      issuer: https://issuer.example.com`,
			wantError: "config.cache_tokens_salt is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := fmt.Sprintf(`
ai_gateways:
  - ref: ai-quickstart
    name: ai-quickstart
    display_name: AI Quickstart
ai_gateway_auth_strategies:
  - ref: support-oidc
    ai_gateway: ai-quickstart
    name: support-oidc
    display_name: Support OIDC
    type: openid-connect
    config:%s
`, tt.configYAML)

			_, err := New().LoadFile(writeLoaderTestFile(t, input))
			require.ErrorContains(t, err, tt.wantError)
		})
	}
}

func TestDeclarativeLoadSchemaRejectsUnknownAIGatewayProperties(t *testing.T) {
	tests := []struct {
		name  string
		yaml  string
		field string
		path  string
	}{
		{"gateway", "future_gateway_field: value", "future_gateway_field", "ai_gateways[0].future_gateway_field"},
		{"agent", "agents:\n      - ref: agent\n        name: agent\n        type: a2a\n        display_name: Agent\n        config:\n          url: https://agent.example.com\n        future_agent_field: value", "future_agent_field", "ai_gateways[0].agents[0].future_agent_field"},                                                                                                                    //nolint:lll
		{"consumer", "consumers:\n      - ref: consumer\n        name: consumer\n        type: api-key\n        display_name: Consumer\n        future_consumer_field: value", "future_consumer_field", "ai_gateways[0].consumers[0].future_consumer_field"},                                                                                                                                                //nolint:lll
		{"consumer group", "consumer_groups:\n      - ref: group\n        name: group\n        display_name: Group\n        future_group_field: value", "future_group_field", "ai_gateways[0].consumer_groups[0].future_group_field"},                                                                                                                                                                       //nolint:lll
		{"MCP server", "mcp_servers:\n      - ref: tools\n        type: conversion-only\n        name: tools\n        display_name: Tools\n        config:\n          url: https://tools.example.com\n        tools:\n          - name: search\n            method: GET\n            path: /search\n        future_mcp_field: value", "future_mcp_field", "ai_gateways[0].mcp_servers[0].future_mcp_field"}, //nolint:lll
		{"listener tools", "mcp_servers:\n      - ref: listener\n        type: listener\n        name: listener\n        display_name: Listener\n        config: {}\n        sources: [tools]\n        tools: []", "tools", "ai_gateways[0].mcp_servers[0].tools"},                                                                                                                                          //nolint:lll
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := "ai_gateways:\n  - ref: support-gateway\n    display_name: Support Gateway\n    " + tt.yaml + "\n"
			_, err := New().parseYAML(strings.NewReader(input), "manifest.yaml", ".")
			require.Error(t, err)
			assert.ErrorContains(t, err, "unknown field '"+tt.field+"'")
			assert.ErrorContains(t, err, tt.path)
		})
	}
}

func TestDeclarativeLoadSchemaKeepsAuthStrategyResourceClosed(t *testing.T) {
	input := `
ai_gateway_auth_strategies:
  - ref: support-key-auth
    ai_gateway: ai-quickstart
    name: support-key-auth
    display_name: Support Key Auth
    type: key-auth
    config:
      realm: Support API
    unexpected: value
`

	_, err := New().parseYAML(strings.NewReader(input), "manifest.yaml", ".")
	require.Error(t, err)
	assert.ErrorContains(t, err, "unknown field 'unexpected'")
	assert.ErrorContains(t, err, "ai_gateway_auth_strategies[0].unexpected")
}

func TestDeclarativeLoadSchemaReportsKnownRejectedUnionField(t *testing.T) {
	input := `
ai_gateways:
  - ref: example-gateway
    name: Example Gateway
    display_name: Example Gateway
    mcp_servers:
      - ref: example-server
        type: conversion-only
        name: Example Server
        display_name: Example Server
        access:
          acl_attribute_type: consumer
        config:
          url: https://example.com
`

	_, err := New().parseYAML(strings.NewReader(input), "manifest.yaml", ".")
	require.Error(t, err)
	assert.ErrorContains(t, err, `AI Gateway MCP Server field "access" is not supported when type is "conversion-only"`)
}

func TestDeclarativeLoadSchemaReportsDeprecatedPortalAuthSettingsFields(t *testing.T) {
	tests := []struct {
		field string
		value string
	}{
		{field: "oidc_auth_enabled", value: "true"},
		{field: "saml_auth_enabled", value: "true"},
		{field: "oidc_team_mapping_enabled", value: "true"},
		{field: "oidc_issuer", value: "https://issuer.example.com"},
		{field: "oidc_client_id", value: "client-id"},
		{field: "oidc_client_secret", value: "must-not-appear-in-errors"},
		{field: "oidc_scopes", value: "[openid, profile]"},
		{field: "oidc_claim_mappings", value: "{name: name, email: email}"},
	}

	for _, tt := range tests {
		t.Run(tt.field, func(t *testing.T) {
			input := fmt.Sprintf(`
portals:
  - ref: example-portal
    name: Example Portal
    display_name: Example Portal
    auth_settings:
      ref: example-auth-settings
      basic_auth_enabled: true
      %s: %s
`, tt.field, tt.value)

			_, err := New().parseYAML(strings.NewReader(input), "manifest.yaml", ".")
			require.Error(t, err)
			assert.ErrorContains(t, err, fmt.Sprintf("uses deprecated field %q", tt.field))
			assert.ErrorContains(t, err, "move identity provider configuration to identity_providers")
			assert.NotContains(t, err.Error(), tt.value)
		})
	}
}

func TestDeclarativeLoadSchemaValidatesScalarSelectedUnionBranches(t *testing.T) {
	tests := []struct {
		name      string
		yaml      string
		errPath   string
		errDetail string
	}{
		{
			name: "listener accepts integer and string ports",
			yaml: `
event_gateways:
  - ref: example-event-gateway
    name: Example Event Gateway
    listeners:
      - ref: example-listener
        name: Example Listener
        ports:
          - 9092
          - "9093-9095"
`,
		},
		{
			name: "ACL resource names accept array and string forms",
			yaml: `
event_gateways:
  - ref: example-event-gateway
    name: Example Event Gateway
    virtual_clusters:
      - ref: example-virtual-cluster
        name: Example Virtual Cluster
        cluster_policies:
          - ref: example-cluster-policy
            name: Example Cluster Policy
            type: acls
            config:
              rules:
                - action: allow
                  operations:
                    - name: describe_configs
                  resource_names:
                    - match: "*"
                  resource_type: transactional_id
                - action: deny
                  operations:
                    - name: read
                  resource_names: 'name == "orders"'
                  resource_type: topic
`,
		},
		{
			name: "listener rejects a boolean port",
			yaml: `
event_gateways:
  - ref: example-event-gateway
    name: Example Event Gateway
    listeners:
      - ref: example-listener
        name: Example Listener
        ports:
          - true
`,
			errPath:   "event_gateways[0].listeners[0].ports[0]",
			errDetail: "expected string",
		},
		{
			name: "ACL resource names reject an object",
			yaml: `
event_gateways:
  - ref: example-event-gateway
    name: Example Event Gateway
    virtual_clusters:
      - ref: example-virtual-cluster
        name: Example Virtual Cluster
        cluster_policies:
          - ref: example-cluster-policy
            name: Example Cluster Policy
            type: acls
            config:
              rules:
                - action: allow
                  operations:
                    - name: describe_configs
                  resource_names:
                    match: "*"
                  resource_type: transactional_id
`,
			errPath:   "event_gateways[0].virtual_clusters[0].cluster_policies[0].config.rules[0].resource_names",
			errDetail: "expected array",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New().parseYAML(strings.NewReader(tt.yaml), "manifest.yaml", ".")
			if tt.errPath == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.ErrorContains(t, err, tt.errPath)
			assert.ErrorContains(t, err, tt.errDetail)
		})
	}
}

func TestDeclarativeLoadSchemaKeepsMapFieldsOpen(t *testing.T) {
	input := `
control_planes:
  - ref: example-control-plane
    name: Example Control Plane
    labels:
      owner.example.com/team: platform
      arbitrary-key: arbitrary-value
`

	_, err := New().parseYAML(strings.NewReader(input), "manifest.yaml", ".")
	require.NoError(t, err)
}

func TestDeclarativeLoadSchemaRejectsMalformedKnownShape(t *testing.T) {
	input := `
control_planes:
  - ref: example-control-plane
    name: Example Control Plane
    proxy_urls: not-an-array
`

	_, err := New().parseYAML(strings.NewReader(input), "manifest.yaml", ".")
	require.Error(t, err)
	assert.ErrorContains(t, err, "control_planes[0].proxy_urls")
	assert.ErrorContains(t, err, "expected array")
}

func TestDeclarativeLoadSchemaDoesNotExposeResolvedEnvValues(t *testing.T) {
	t.Setenv("LOAD_SCHEMA_SECRET", "must-not-appear-in-errors")
	input := `
ai_gateway_consumer_credentials:
  - ref: eason-key
    ai_gateway_consumer: eason
    name: eason-key
    display_name: Eason API Key
    type: api-key
    key: !env LOAD_SCHEMA_SECRET
`

	_, err := New().parseYAML(strings.NewReader(input), "manifest.yaml", ".")
	require.Error(t, err)
	assert.ErrorContains(t, err, "ai_gateway_consumer_credentials[0].key")
	assert.NotContains(t, err.Error(), "must-not-appear-in-errors")
}

func TestCompiledDeclarativeLoadSchemaIsCached(t *testing.T) {
	first, err := compiledDeclarativeLoadSchema()
	require.NoError(t, err)
	second, err := compiledDeclarativeLoadSchema()
	require.NoError(t, err)
	assert.Same(t, first, second)
	assert.Same(t, first.compiled, second.compiled)
}

func TestDeclarativeLoadSchemaValidatesCuratedE2EFixtures(t *testing.T) {
	fixtures := []string{
		"test/e2e/scenarios/ai-gateway/consumer/testdata/config.yaml",
		"test/e2e/scenarios/ai-gateway/model-provider/testdata/config.yaml",
		"test/e2e/scenarios/apis/comprehensive-fields/testdata/apis.yaml",
		"test/e2e/scenarios/portal/api_docs_with_children/testdata/apis.yaml",
		"test/e2e/scenarios/portal/api_docs_with_children/testdata/portal.yaml",
		"test/e2e/testdata/declarative/event-gateways/comprehensive-fields/event-gateways.yaml",
	}

	for _, fixture := range fixtures {
		t.Run(fixture, func(t *testing.T) {
			path := filepath.Join("..", "..", "..", fixture)
			_, err := New().LoadFile(path)
			require.NoError(t, err)
		})
	}
}
