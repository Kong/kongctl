package loader

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoaderAIGatewayTokenExchangePreservesAdditionalFields(t *testing.T) {
	for _, extension := range []string{
		"", // Known fields alone remain valid.
		`          extension: {enabled: true}
          subject_token_issuers:
            - issuer: https://subject.example.com
              issuer_extension: value
              conditions:
                has_scopes: [read]
                condition_extension: value
          request:
            scopes: [read]
            empty_audience: true
          cache:
            enabled: true
            ttl: 60
            ttl_seconds: 60
`,
	} {
		name := "known fields"
		if extension != "" {
			name = "additional fields"
		}
		t.Run(name, func(t *testing.T) {
			var manifest strings.Builder
			manifest.WriteString(`
_defaults:
  kongctl:
    namespace: token-exchange-repro
ai_gateways:
  - ref: repro-gateway
    name: repro-gateway
    display_name: Token Exchange Repro
    deployment_type: hybrid
    auth_strategies:
      - ref: repro-oidc
        name: repro-oidc
        display_name: Repro OIDC
        type: openid-connect
        config:
          issuer: https://target.example.com
          cache_tokens_salt: repro-cache-salt
          token_exchange:
`)
			if extension == "" {
				manifest.WriteString(`            subject_token_issuers:
              - issuer: https://subject.example.com
`)
			} else {
				// Indent the supplied properties under token_exchange.
				for _, line := range strings.SplitAfter(extension, "\n") {
					if line != "" && line != "\n" {
						manifest.WriteString("  " + line)
					}
				}
			}
			rs, err := New().LoadFile(writeLoaderTestFile(t, manifest.String()))
			require.NoError(t, err)
			require.Len(t, rs.AIGatewayAuthStrategies, 1)
			exchange := rs.AIGatewayAuthStrategies[0].Config["token_exchange"].(map[string]any)
			issuers := exchange["subject_token_issuers"].([]any)
			issuer := issuers[0].(map[string]any)
			assert.Equal(t, "https://subject.example.com", issuer["issuer"])
			if extension != "" {
				assert.Equal(t, map[string]any{"enabled": true}, exchange["extension"])
				assert.Equal(t, "value", issuer["issuer_extension"])
				conditions := issuer["conditions"].(map[string]any)
				assert.Equal(t, "value", conditions["condition_extension"])
				request := exchange["request"].(map[string]any)
				assert.Equal(t, true, request["empty_audience"])
				cache := exchange["cache"].(map[string]any)
				assert.Equal(t, cache["ttl"], cache["ttl_seconds"])
			}
		})
	}
}
