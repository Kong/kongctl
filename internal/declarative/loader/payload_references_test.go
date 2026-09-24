package loader

import (
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"

	"github.com/stretchr/testify/require"
)

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
