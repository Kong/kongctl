package loader

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoaderPreservesExplicitSecretWithoutResolvingEnvironment(t *testing.T) {
	rs, err := New().LoadFile(writeLoaderTestFile(t, portalSecretConfig(
		"!secret {source: !env UNAVAILABLE_PORTAL_SECRET}",
	)))
	require.NoError(t, err)

	declaration := rs.GetSecretSources("portal-idp")["/config/client_secret"]
	require.Len(t, declaration.Expression.Parts, 1)
	assert.False(t, declaration.DeprecatedBareEnv)
	assert.Equal(t, "UNAVAILABLE_PORTAL_SECRET", declaration.Expression.Parts[0].Source.Reference)
	assert.True(t, tags.IsSecretPlaceholder(*rs.PortalIdentityProviders[0].Config.OIDCIdentityProviderConfig.ClientSecret))
}

func TestLoaderPreservesSecretComposition(t *testing.T) {
	config := `
ai_gateways:
  - ref: gateway
    name: gateway
    display_name: Gateway
    model_providers:
      - ref: provider
        name: provider
        type: openai
        display_name: Provider
        config:
          auth:
            type: basic
            headers:
              - name: Authorization
                value: !secret
                  parts:
                    - "Bearer "
                    - !env PROVIDER_TOKEN
`
	rs, err := New().LoadFile(writeLoaderTestFile(t, config))
	require.NoError(t, err)

	parts := rs.GetSecretSources("provider")["/config/auth/headers/0/value"].Expression.Parts
	require.Len(t, parts, 2)
	assert.Equal(t, "Bearer ", *parts[0].Literal)
	assert.Equal(t, "PROVIDER_TOKEN", parts[1].Source.Reference)
}

func TestLoaderRejectsLiteralAndEagerFileOnReviewedSecretFieldsWithoutEchoingValues(t *testing.T) {
	t.Run("literal", func(t *testing.T) {
		secret := "must-not-appear-in-error"
		_, err := New().LoadFile(writeLoaderTestFile(t, portalSecretConfig(secret)))
		require.ErrorContains(t, err, "field /config/client_secret")
		assert.NotContains(t, err.Error(), secret)
	})

	t.Run("eager file", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "secret.txt"), []byte("file-secret-value"), 0o600))
		path := filepath.Join(dir, "config.yaml")
		require.NoError(t, os.WriteFile(path, []byte(portalSecretConfig("!file secret.txt")), 0o600))
		_, err := NewWithBaseDir(dir).LoadFile(path)
		require.ErrorContains(t, err, "requires !secret with a deferred source")
		assert.NotContains(t, err.Error(), "file-secret-value")
	})
}

func TestLoaderRejectsLiteralAIGatewayProviderHeaderValue(t *testing.T) {
	config := `
ai_gateways:
  - ref: gateway
    name: gateway
    display_name: Gateway
    model_providers:
      - ref: provider
        name: provider
        type: openai
        display_name: Provider
        config:
          auth:
            type: basic
            headers:
              - name: Authorization
                value: public-looking-value
`

	_, err := New().LoadFile(writeLoaderTestFile(t, config))
	require.ErrorContains(t, err, "field /config/auth/headers/0/value")
	require.ErrorContains(t, err, "requires !secret with a deferred source")
	assert.NotContains(t, err.Error(), "public-looking-value")
}

func TestLoaderRejectsSecretOnUnreviewedField(t *testing.T) {
	config := `
portals:
  - ref: portal
    name: !secret {source: !env PORTAL_NAME}
`
	_, err := New().LoadFile(writeLoaderTestFile(t, config))
	require.ErrorContains(t, err, "not a reviewed write-only field")
}

func TestLoaderRejectsSecretOnOrganizationSelectorsOutsideResourceRegistry(t *testing.T) {
	tests := map[string]string{
		"user email": `
organization:
  users:
    - ref: user
      email: !secret {source: !env ORGANIZATION_USER_EMAIL}
`,
		"system account name": `
organization:
  system-accounts:
    - ref: automation
      name: !secret {source: !env SYSTEM_ACCOUNT_NAME}
`,
	}

	for name, config := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := New().LoadFile(writeLoaderTestFile(t, config))
			require.ErrorContains(t, err, "not a reviewed write-only field")
		})
	}
}

func TestLoaderAllowsPublicVaultReferenceOnReviewedSecretField(t *testing.T) {
	references := []string{
		"{vault://support-secrets/openai-auth-header}",
		"${vault['support-secrets']['openai-auth-header']}",
	}

	for _, reference := range references {
		t.Run(reference, func(t *testing.T) {
			config := `
ai_gateways:
  - ref: gateway
    name: gateway
    display_name: Gateway
    model_providers:
      - ref: provider
        name: provider
        type: openai
        display_name: Provider
        config:
          auth:
            type: basic
            headers:
              - name: Authorization
                value: "` + reference + `"
`

			rs, err := New().LoadFile(writeLoaderTestFile(t, config))
			require.NoError(t, err)
			assert.Empty(t, rs.GetSecretSources("provider"))
			auth := rs.AIGatewayProviders[0].Config["auth"].(map[string]any)
			headers := auth["headers"].([]any)
			header := headers[0].(map[string]any)
			assert.Equal(t, reference, header["value"])
		})
	}
}

func TestLoaderPreservesVaultReferenceAlongsideDeferredArraySecret(t *testing.T) {
	const reference = "{vault://support-secrets/primary-client-secret}"
	config := `
ai_gateways:
  - ref: gateway
    name: gateway
    display_name: Gateway
    auth_strategies:
      - ref: provider
        name: provider
        type: openid-connect
        display_name: Provider
        config:
          cache_tokens_salt: support-cache-salt
          issuer: https://issuer.example.test
          client_id:
            - primary-client
            - fallback-client
          client_secret:
            - "` + reference + `"
            - !secret {source: !env FALLBACK_CLIENT_SECRET}
`

	rs, err := New().LoadFile(writeLoaderTestFile(t, config))
	require.NoError(t, err)
	declarations := rs.GetSecretSources("provider")
	require.Len(t, declarations, 1)
	declaration := declarations["/config/client_secret/1"]
	require.Len(t, declaration.Expression.Parts, 1)
	assert.Equal(t, "FALLBACK_CLIENT_SECRET", declaration.Expression.Parts[0].Source.Reference)

	clientSecrets := rs.AIGatewayAuthStrategies[0].Config["client_secret"].([]any)
	require.Len(t, clientSecrets, 2)
	assert.Equal(t, reference, clientSecrets[0])
	secretPlaceholder, ok := clientSecrets[1].(string)
	require.True(t, ok)
	assert.True(t, tags.IsSecretPlaceholder(secretPlaceholder))
}

func TestLoaderNormalizesDeprecatedBareEnvSecretWithoutResolvingIt(t *testing.T) {
	rs, err := New().LoadFile(writeLoaderTestFile(t, portalSecretConfig("!env UNAVAILABLE_LEGACY_SECRET")))
	require.NoError(t, err)

	declaration := rs.GetSecretSources("portal-idp")["/config/client_secret"]
	assert.True(t, declaration.DeprecatedBareEnv)
	assert.Equal(t, "UNAVAILABLE_LEGACY_SECRET", declaration.Expression.Parts[0].Source.Reference)
}

func portalSecretConfig(secretValue string) string {
	return `
portals:
  - ref: portal
    name: Portal
    identity_providers:
      - ref: portal-idp
        type: oidc
        config:
          issuer_url: https://issuer.example.test
          client_id: portal-client
          client_secret: ` + secretValue + "\n"
}
