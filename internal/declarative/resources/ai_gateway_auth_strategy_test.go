package resources

import (
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/stretchr/testify/require"
)

func TestAIGatewayAuthStrategyEnrollmentProtectionRequiresDatastore(t *testing.T) {
	resource := AIGatewayAuthStrategyResource{
		BaseResource: BaseResource{Ref: "oidc"},
		Name:         "oidc",
		Type:         "openid-connect",
		DisplayName:  "OIDC",
		Config:       map[string]any{"issuer": "https://example.com", "cache_tokens_salt": "salt"},
	}
	require.NoError(t, resource.Validate())
	for _, enabled := range []*bool{nil, new(false), new(true)} {
		resource.Datastores = nil
		resource.TokenVault = &kkComps.AIGatewayTokenVaultEnrollmentProtectionInput{
			EnableCredentialManager: enabled,
		}
		require.ErrorContains(t, resource.Validate(), "token_vault requires exactly one")
		resource.Datastores = []kkComps.AIGatewayDatastoreRef{{Name: "vault"}}
		require.NoError(t, resource.Validate())
	}
}

func TestAIGatewayAuthStrategyExplainRequiresOpenIDConnectAPIFields(t *testing.T) {
	node, err := aiGatewayAuthStrategyExplainNode(ExplainBuildContext{})
	require.NoError(t, err)

	openIDConnect := aiGatewayAuthStrategyExplainBranch(t, node, "openid-connect")

	configField := openIDConnect.propIndex["config"]
	require.NotNil(t, configField)
	cacheTokensSalt := configField.Node.propIndex["cache_tokens_salt"]
	require.NotNil(t, cacheTokensSalt)
	require.True(t, cacheTokensSalt.Required)
	issuer := configField.Node.propIndex["issuer"]
	require.NotNil(t, issuer)
	require.True(t, issuer.Required)
}

func TestAIGatewayAuthStrategyExplainIncludesGeneratedConfigProperties(t *testing.T) {
	node, err := aiGatewayAuthStrategyExplainNode(ExplainBuildContext{})
	require.NoError(t, err)

	for providerType, fields := range map[string][]string{
		"key-auth":       {"identity_realms", "principals"},
		"openid-connect": {"credential_claims", "upstream_headers_claims", "upstream_headers_names", "principals"},
	} {
		t.Run(providerType, func(t *testing.T) {
			branch := aiGatewayAuthStrategyExplainBranch(t, node, providerType)
			configField := branch.propIndex["config"]
			require.NotNil(t, configField)
			require.NotEmpty(t, configField.Node.Properties)
			for _, field := range fields {
				require.Contains(t, configField.Node.propIndex, field)
			}
		})
	}
}

func aiGatewayAuthStrategyExplainBranch(t *testing.T, node *ExplainNode, providerType string) *ExplainNode {
	t.Helper()

	for _, branch := range node.OneOf {
		typeField := branch.propIndex["type"]
		if typeField != nil && typeField.Node.Const == providerType {
			return branch
		}
	}

	require.Failf(t, "missing auth strategy explain branch", "provider type %q was not found", providerType)
	return nil
}
