package resources

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAIGatewayUnionDefaultsRequireExplicitName(t *testing.T) {
	type namedResource interface {
		Resource
		Name() string
	}
	for _, tc := range []struct {
		name        string
		payload     string
		newResource func() namedResource
	}{
		{
			"MCP listener", `{
				"type":"listener","display_name":"Listener","sources":[],"config":{"route":{"paths":["/mcp"]}}
			}`,
			func() namedResource { return &AIGatewayMCPServerResource{} },
		},
		{
			"environment vault", `{"type":"env","config":{"prefix":"EXAMPLE_"}}`,
			func() namedResource { return &AIGatewayVaultResource{} },
		},
		{
			"API model", aiGatewayAPIModelJSON,
			func() namedResource { return &AIGatewayModelResource{} },
		},
		{
			"model", aiGatewayModelJSON,
			func() namedResource { return &AIGatewayModelResource{} },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, name := range []struct {
				label, value string
				present      bool
			}{
				{label: "omitted"},
				{label: "empty", present: true},
				{label: "explicit", present: true, value: "api-name"},
			} {
				t.Run(name.label, func(t *testing.T) {
					var payload map[string]any
					require.NoError(t, json.Unmarshal([]byte(tc.payload), &payload))
					payload[SchemaFieldRef] = "local-ref"
					payload[SchemaFieldAIGateway] = "gateway-ref"
					if name.present {
						payload[SchemaFieldName] = name.value
					} else {
						delete(payload, SchemaFieldName)
					}
					data, err := json.Marshal(payload)
					require.NoError(t, err)
					resource := tc.newResource()
					err = json.Unmarshal(data, resource)
					if !name.present {
						// SDK union decoding rejects absent names before defaulting.
						require.ErrorContains(t, err, "missing required fields: name")
						return
					}
					require.NoError(t, err)

					resource.SetDefaults()

					require.Equal(t, name.value, resource.Name())
					require.Equal(t, "local-ref", resource.GetRef())
					if name.value == "" {
						require.ErrorContains(t, resource.Validate(), "name is required")
					} else {
						require.NoError(t, resource.Validate())
					}
				})
			}
		})
	}
}
