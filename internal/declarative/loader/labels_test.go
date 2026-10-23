package loader

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

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
		})
	}
}
