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

func TestDeclarativeLoadSchemaAcceptsVertexServiceAccountAuth(t *testing.T) {
	t.Setenv("GCP_SERVICE_ACCOUNT_JSON", `{"type":"service_account"}`)
	input := `
ai_gateway_model_providers:
  - ref: vertex-prod
    name: vertex-prod
    display_name: Google Vertex Prod
    ai_gateway: ai-quickstart
    type: vertex
    config:
      auth:
        type: vertex
        service_account_json: !env GCP_SERVICE_ACCOUNT_JSON
`

	_, err := New().parseYAML(strings.NewReader(input), "manifest.yaml", ".")
	require.NoError(t, err)
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
	assert.EqualError(
		t,
		err,
		`AI Gateway MCP Server field "access" is not supported when type is "conversion-only"`,
	)
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
