package loader

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoaderRejectsUnknownNestedOIDCConfigFields(t *testing.T) {
	for _, tc := range []struct{ name, config string }{
		{"principals", `principals: {enabeld: true}`},
		{"jit", `principals: {jit: {enabeld: true}}`},
		{"redis", `redis: {unknown_nested_field: true}`},
		{"token_exchange", `token_exchange: {subject_token_issuers: [{issuer: https://example.com}], ` +
			`unknown_nested_field: true}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := fmt.Sprintf(`ai_gateways:
- ref: gateway
  name: oidc-validation
  display_name: OIDC Validation
  auth_strategies:
  - ref: auth
    name: auth
    display_name: Auth
    type: openid-connect
    config:
      issuer: https://example.com
      cache_tokens_salt: test-salt
      %s
`, tc.config)
			_, err := New().LoadFile(writeLoaderTestFile(t, input))
			require.ErrorContains(t, err, "unknown field")
		})
	}
}
