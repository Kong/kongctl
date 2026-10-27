package loader

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoaderDeferredLabelValues(t *testing.T) {
	for _, tt := range []struct {
		name       string
		expression string
		sourceName string
		envValue   string
		wantErr    bool
	}{
		{name: "lookup", expression: `!lookup {resource_type: ai_gateway, name: external-ai-gateway}`},
		{name: "external", expression: `!external {resource_type: ai_gateway, name: external-ai-gateway}`},
		{name: "pending reference", expression: `!ref source#id`},
		{name: "known reference", expression: `!ref source#name`, sourceName: "Source-1"},
		{name: "invalid known reference", expression: `!ref source#name`, sourceName: "has space", wantErr: true},
		{name: "environment", expression: `!env LABEL_VALUE`, envValue: "Team-1"},
		{name: "invalid environment", expression: `!env LABEL_VALUE`, envValue: "has space", wantErr: true},
		{name: "literal reference encoding", expression: `"__REF__:source#id"`, wantErr: true},
		{
			name: "environment reference encoding", expression: `!env LABEL_VALUE`,
			envValue: "__REF__:source#id", wantErr: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("LABEL_VALUE", tt.envValue)
			if tt.sourceName == "" {
				tt.sourceName = "Source-1"
			}
			manifest := fmt.Sprintf(`apis:
  - ref: source
    name: %q
  - ref: consumer
    name: consumer
    labels:
      gateway_id: %s
`, tt.sourceName, tt.expression)
			_, err := New().LoadFromSources([]Source{{
				Path: writeLoaderTestFile(t, manifest), Type: SourceTypeFile,
			}}, false)
			if tt.wantErr {
				require.ErrorContains(t, err, "gateway_id")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestLoaderKonnectLabels(t *testing.T) {
	manifests := map[string]string{
		"portal pointer map": `portals:
  - ref: portal
    name: Portal
    labels: {owner: LABEL_VALUE}
`,
		"api": `apis:
  - ref: api
    name: API
    labels: {owner: LABEL_VALUE}
`,
		"grouped organization team": `organization:
  teams:
    - ref: team
      name: Team
      labels: {owner: LABEL_VALUE}
`,
		"nested ai gateway union": `ai_gateways:
  - ref: gateway
    name: gateway
    display_name: Gateway
    vaults:
      - ref: vault
        name: vault
        type: env
        config: {prefix: EXAMPLE_}
        labels: {owner: LABEL_VALUE}
`,
		"root ai gateway union": `ai_gateways:
  - ref: gateway
    name: gateway
    display_name: Gateway
ai_gateway_vaults:
  - ref: vault
    ai_gateway: gateway
    name: vault
    type: env
    config: {prefix: EXAMPLE_}
    labels: {owner: LABEL_VALUE}
`,
		"event gateway policy union": `event_gateways:
  - ref: gateway
    name: gateway
    listeners:
      - ref: listener
        name: listener
        addresses: ["example.com:9092"]
        policies:
          - ref: policy
            type: tls_server
            config: {certificates: [], allow_plaintext: true}
            labels: {owner: LABEL_VALUE}
`,
	}
	for name, manifest := range manifests {
		t.Run(name, func(t *testing.T) {
			valid := strings.ReplaceAll(manifest, "LABEL_VALUE", "Platform-1")
			_, err := New().LoadFile(writeLoaderTestFile(t, valid))
			require.NoError(t, err)
			invalid := strings.ReplaceAll(manifest, "LABEL_VALUE", `"has space"`)
			_, err = New().LoadFile(writeLoaderTestFile(t, invalid))
			require.ErrorContains(t, err, "label")
			require.ErrorContains(t, err, "owner")
			lookup := strings.ReplaceAll(manifest, "LABEL_VALUE",
				`!lookup {resource_type: ai_gateway, id: known-id}`)
			_, err = New().LoadFile(writeLoaderTestFile(t, lookup))
			require.NoError(t, err)
			literal := strings.ReplaceAll(manifest, "LABEL_VALUE", `"__REF__:source#id"`)
			_, err = New().LoadFile(writeLoaderTestFile(t, literal))
			require.ErrorContains(t, err, "owner")
		})
	}
}
