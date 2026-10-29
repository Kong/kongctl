package loader

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"testing"

	testdecl "github.com/kong/kongctl/test/declarative"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func TestManagedByIsReservedAtResourceBoundaries(t *testing.T) {
	cases := testdecl.AIGatewayOwnedChildren(t)
	cases = append(
		cases,
		testdecl.AIGatewayChildCase{Collection: "certificates", Body: map[string]any{"cert": "public-cert"}},
		testdecl.AIGatewayChildCase{Collection: "consumer_credentials", Body: map[string]any{
			"type": "api-key", "ai_gateway_consumer": "consumer",
		}},
	)
	for _, tc := range cases {
		for _, nested := range []bool{false, true} {
			if nested && tc.Collection == "consumer_credentials" {
				continue
			}
			for _, value := range []any{nil, map[string]any{}, map[string]any{"arbitrary": "value"}} {
				for _, encoding := range []string{"yaml", "json"} {
					t.Run(tc.Collection+"/"+encoding, func(t *testing.T) {
						child := maps.Clone(tc.Body)
						child["ref"], child["name"] = "child", "child"
						child["managed_by"] = value
						if tc.Collection != "vaults" && tc.Collection != "ca_certificates" && tc.Collection != "certificates" {
							child["display_name"] = "Child"
						}
						gateway := map[string]any{"ref": "shared", "_external": map[string]any{"id": "gateway-id"}}
						manifest := map[string]any{"ai_gateways": []any{gateway}}
						if nested {
							gateway[tc.Collection] = []any{child}
						} else {
							if tc.Collection != "consumer_credentials" {
								child["ai_gateway"] = "shared"
							}
							manifest["ai_gateway_"+tc.Collection] = []any{child}
						}
						data, err := json.Marshal(manifest)
						require.NoError(t, err)
						if encoding == "yaml" {
							data, err = yaml.JSONToYAML(data)
							require.NoError(t, err)
						}
						path := filepath.Join(t.TempDir(), "input.yaml")
						require.NoError(t, os.WriteFile(path, data, 0o600))
						_, err = New().LoadFile(path)
						require.ErrorContains(t, err, "managed_by is reserved for Kong tools")
					})
				}
			}
		}
	}
}

func TestManagedByLabelIsPublicUserMetadata(t *testing.T) {
	path := filepath.Join(t.TempDir(), "input.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`ai_gateways:
- ref: shared
  _external: {id: gateway-id}
ai_gateway_model_providers:
- ref: provider
  ai_gateway: shared
  name: provider
  display_name: Provider
  type: openai
  labels: {managed_by: public-label}
  config: {auth: {type: basic}}
`), 0o600))
	rs, err := New().LoadFile(path)
	require.NoError(t, err)
	require.Equal(t, "public-label", rs.AIGatewayProviders[0].Labels["managed_by"])
}

func TestManagedByIsReservedOnGatewayAndNestedCredential(t *testing.T) {
	for _, content := range []string{
		"ai_gateways: [{ref: shared, _external: {id: gateway-id}, managed_by: null}]",
		`ai_gateways:
- ref: shared
  _external: {id: gateway-id}
  consumers:
  - ref: consumer
    name: consumer
    display_name: Consumer
    type: api-key
    credentials:
    - ref: credential
      name: credential
      display_name: Credential
      type: api-key
      managed_by: {}
`,
	} {
		path := filepath.Join(t.TempDir(), "input.yaml")
		require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
		_, err := New().LoadFile(path)
		require.ErrorContains(t, err, "managed_by is reserved for Kong tools")
	}
}
