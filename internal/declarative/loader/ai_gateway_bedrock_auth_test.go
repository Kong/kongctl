package loader

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func bedrockAuthConfig(auth string) string {
	return `
_defaults:
  kongctl:
    namespace: bedrock-repro
ai_gateways:
  - ref: repro-gw
    name: repro-bedrock-gw
    display_name: Bedrock Repro GW
    deployment_type: hybrid
    model_providers:
      - ref: repro-bedrock
        name: repro-bedrock
        type: bedrock
        display_name: AWS Bedrock Provider
        config:
          auth:
            type: aws
` + auth
}

func TestLoaderBedrockAuthRejectsNestedAWS(t *testing.T) {
	_, err := New().LoadFile(writeLoaderTestFile(t, bedrockAuthConfig(`
            aws:
              access_key_id: AKIAEXAMPLE
              secret_access_key: examplesecret
`)))
	require.ErrorContains(t, err, "unknown field 'aws'")
	assert.Contains(t, err.Error(), "ai_gateways[0].model_providers[0].config.auth.aws")
}

func TestLoaderBedrockAuthLiteralErrorNamesAcceptedForms(t *testing.T) {
	_, err := New().LoadFile(writeLoaderTestFile(t, bedrockAuthConfig(`
            access_key_id: AKIAEXAMPLE
            secret_access_key: examplesecret
`)))
	require.ErrorContains(t, err, "field /config/auth/secret_access_key")
	assert.Contains(t, err.Error(), "!secret with a deferred source")
	assert.Contains(t, err.Error(), "{vault://<store>/<key>}")
	assert.NotContains(t, err.Error(), "examplesecret")
}

func TestLoaderBedrockAuthPreservesVaultReferences(t *testing.T) {
	rs, err := New().LoadFile(writeLoaderTestFile(t, bedrockAuthConfig(`
            access_key_id: '{vault://poc-aigw-secrets/aws-access-key-id}'
            secret_access_key: '{vault://poc-aigw-secrets/aws-secret-access-key}'
`)))
	require.NoError(t, err)
	require.Len(t, rs.AIGatewayProviders, 1)
	auth := rs.AIGatewayProviders[0].Config["auth"].(map[string]any)
	assert.Equal(t, "{vault://poc-aigw-secrets/aws-access-key-id}", auth["access_key_id"])
	assert.Equal(t, "{vault://poc-aigw-secrets/aws-secret-access-key}", auth["secret_access_key"])
	assert.Empty(t, rs.GetSecretSources("repro-bedrock"))
}

func TestLoaderBedrockAuthPreservesDeferredSecret(t *testing.T) {
	rs, err := New().LoadFile(writeLoaderTestFile(t, bedrockAuthConfig(`
            access_key_id: AKIAEXAMPLE
            secret_access_key: !secret {source: !env BEDROCK_TEST_SECRET}
`)))
	require.NoError(t, err)
	declaration := rs.GetSecretSources("repro-bedrock")["/config/auth/secret_access_key"]
	require.Len(t, declaration.Expression.Parts, 1)
	assert.Equal(t, "BEDROCK_TEST_SECRET", declaration.Expression.Parts[0].Source.Reference)
	assert.False(t, declaration.DeprecatedBareEnv)
}
