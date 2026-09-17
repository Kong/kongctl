package loader

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewCatalogFieldsDeferSources(t *testing.T) {
	for _, tc := range []struct{ name, path, manifest string }{
		{"domain", "/ssl/custom_private_key", `
portals:
  - ref: portal
    name: portal
    custom_domain:
      ref: target
      hostname: developer.example.com
      enabled: true
      ssl:
        domain_verification_method: custom_certificate
        custom_certificate: public-certificate
        custom_private_key: %s
`},
		{"backend", "/authentication/password", `
event_gateways:
  - ref: gateway
    name: gateway
    backend_clusters:
      - ref: target
        name: backend
        bootstrap_servers: [broker.example.com:9092]
        tls: {enabled: false}
        authentication:
          type: sasl_plain
          username: user
          password: %s
`},
		{"http proxy", "/config/http_proxy_authorization", proxySecretManifest("http")},
		{"https proxy", "/config/https_proxy_authorization", proxySecretManifest("https")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, source := range []string{
				"!secret {source: !env UNAVAILABLE_CATALOG_SECRET}",
				"!env UNAVAILABLE_CATALOG_SECRET",
			} {
				rs, err := New().LoadFile(writeLoaderTestFile(t, fmt.Sprintf(tc.manifest, source)))
				require.NoError(t, err)
				declaration, ok := rs.GetSecretSources("target")[tc.path]
				require.True(t, ok)
				require.Len(t, declaration.Expression.Parts, 1)
				require.Equal(t, "UNAVAILABLE_CATALOG_SECRET", declaration.Expression.Parts[0].Source.Reference)
			}
			_, err := New().LoadFile(writeLoaderTestFile(t, fmt.Sprintf(tc.manifest, "literal-private-value")))
			require.ErrorContains(t, err, "requires !secret with a deferred source")
			require.NotContains(t, err.Error(), "literal-private-value")
		})
	}
}

func proxySecretManifest(scheme string) string {
	return `
ai_gateways:
  - ref: gateway
    name: gateway
    display_name: Gateway
    auth_strategies:
      - ref: target
        name: target
        type: openid-connect
        display_name: Target
        config:
          issuer: https://issuer.example.com
          cache_tokens_salt: public-salt
          ` + scheme + `_proxy_authorization: %s
`
}
