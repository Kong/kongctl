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
	const reference = "{vault://support-secrets/openai-auth-header}"
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
