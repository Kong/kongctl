package loader

import (
	"strings"
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"

	"github.com/stretchr/testify/require"
)

func TestRelationshipReferencesValidateBeforeDeferring(t *testing.T) {
	const manifest = `
portals:
  - ref: portal
    name: portal
    display_name: Portal
    default_application_auth_strategy_id: !ref TARGET
application_auth_strategies:
  - ref: auth
    name: auth
    display_name: Auth
    strategy_type: key_auth
    configs:
      key_auth:
        key_names: [apikey]
`
	for _, tc := range []struct{ expression, wantErr string }{
		{"missing-auth#id", "resource not found: missing-auth"},
		{"auth#missing", "unknown or unavailable selector"},
		{"auth#id", ""},
		{"auth", ""},
	} {
		t.Run(tc.expression, func(t *testing.T) {
			rs, err := loadPayloadReferences(t, strings.Replace(manifest, "TARGET", tc.expression, 1))
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				require.ErrorContains(t, err, "/default_application_auth_strategy_id")
				return
			}
			require.NoError(t, err)
			require.Equal(t, "__REF__:auth#id", *rs.Portals[0].DefaultApplicationAuthStrategyID)
			require.Empty(t, rs.ApplicationAuthStrategies[0].GetKonnectID())
		})
	}
}

func TestPayloadReferencesLoadMapsListsAndSDKFields(t *testing.T) {
	t.Setenv("PAYLOAD_REF_DESCRIPTION", "do-not-persist-this-value")
	rs, err := loadPayloadReferences(t, `
ai_gateways:
  - ref: gateway
    name: gateway-name
    display_name: Gateway
    description: !env PAYLOAD_REF_DESCRIPTION
    deployment_type: hybrid
    policies:
      - ref: policy
        name: policy
        display_name: Policy
        type: opentelemetry
        config:
          headers:
            X-Control-Plane-Id: !ref gateway
            dot.key: !ref gateway#name
            "0": !ref gateway#description
          items:
            - !ref gateway#id
            - nested: !ref gateway#display_name
          literal: "gateway#id"
          encoded_literal: "__REF__:ordinary#id"
`)
	require.NoError(t, err)
	config := rs.AIGatewayPolicies[0].Config
	headers := config["headers"].(map[string]any)
	require.Equal(t, "__REF__:gateway#id", headers["X-Control-Plane-Id"])
	require.Equal(t, "gateway-name", headers["dot.key"])
	require.Equal(t, "do-not-persist-this-value", headers["0"])
	require.Contains(t, rs.GetEnvSources("policy"), "/config/headers/0")
	require.Equal(t, []any{"__REF__:gateway#id", map[string]any{"nested": "Gateway"}}, config["items"])
	require.Equal(t, "gateway#id", config["literal"])
	require.Equal(t, "__REF__:ordinary#id", config["encoded_literal"])
	require.Contains(t, rs.LiteralSources["policy"], "/config/encoded_literal")
}

func TestPayloadReferenceErrorsIncludeDestination(t *testing.T) {
	for _, expression := range []string{"missing", "gateway#missing", "gateway#proxy_urls", "policy#config.loop"} {
		t.Run(expression, func(t *testing.T) {
			_, err := loadPayloadReferences(t, `
ai_gateways:
  - ref: gateway
    name: gateway
    display_name: Gateway
    deployment_type: hybrid
    policies:
      - ref: policy
        name: policy
        display_name: Policy
        type: opentelemetry
        config:
          loop: !ref `+expression+`
`)
			require.Error(t, err)
			require.ErrorContains(t, err, "policy")
			require.ErrorContains(t, err, "/config/loop")
		})
	}
}

func loadPayloadReferences(t *testing.T, content string) (*resources.ResourceSet, error) {
	t.Helper()
	return New().LoadFromSources([]Source{{Path: writeLoaderTestFile(t, content), Type: SourceTypeFile}}, false)
}

func TestPayloadReferencesFindNestedEventGatewayTargets(t *testing.T) {
	rs, err := loadPayloadReferences(t, `
event_gateways:
  - ref: gateway
    name: gateway
    virtual_clusters:
      - ref: virtual
        name: virtual-name
        destination:
          name: remote-backend
        authentication:
          - type: anonymous
        acl_mode: passthrough
        dns_label: virtual
    listeners:
      - ref: listener
        name: listener
        ports: [9092]
        policies:
          - ref: forward
            name: forward
            type: forward_to_virtual_cluster
            config:
              type: port_mapping
              advertised_host: example.com
              destination:
                id: !ref virtual
ai_gateways:
  - ref: ai
    name: ai
    display_name: AI
    deployment_type: hybrid
    policies:
      - ref: policy
        name: policy
        display_name: Policy
        type: opentelemetry
        config:
          nested_target_name: !ref virtual#name
          nested_target_id: !ref virtual
`)
	require.NoError(t, err)
	require.Equal(t, "virtual-name", rs.AIGatewayPolicies[0].Config["nested_target_name"])
	require.Equal(t, "__REF__:virtual#id", rs.AIGatewayPolicies[0].Config["nested_target_id"])
	target, err := rs.ReferenceTarget("virtual")
	require.NoError(t, err)
	require.Same(t, &rs.EventGatewayControlPlanes[0].VirtualClusters[0], target)
	policy, err := rs.ReferenceTarget("forward")
	require.NoError(t, err)
	require.Equal(t, resources.ResourceTypeEventGatewayListenerPolicy, policy.GetType())
}
