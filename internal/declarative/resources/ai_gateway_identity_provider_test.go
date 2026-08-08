package resources

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAIGatewayIdentityProviderExplainRequiresOpenIDConnectAPIFields(t *testing.T) {
	node, err := aiGatewayIdentityProviderExplainNode(ExplainBuildContext{})
	require.NoError(t, err)

	openIDConnect := aiGatewayIdentityProviderExplainBranch(t, node, "openid-connect")

	configField := openIDConnect.propIndex["config"]
	require.NotNil(t, configField)
	cacheTokensSalt := configField.Node.propIndex["cache_tokens_salt"]
	require.NotNil(t, cacheTokensSalt)
	require.True(t, cacheTokensSalt.Required)
	issuer := configField.Node.propIndex["issuer"]
	require.NotNil(t, issuer)
	require.True(t, issuer.Required)
}

func TestAIGatewayIdentityProviderExplainAllowsAdditionalConfigProperties(t *testing.T) {
	node, err := aiGatewayIdentityProviderExplainNode(ExplainBuildContext{})
	require.NoError(t, err)

	for _, providerType := range []string{"key-auth", "openid-connect"} {
		t.Run(providerType, func(t *testing.T) {
			branch := aiGatewayIdentityProviderExplainBranch(t, node, providerType)
			configField := branch.propIndex["config"]
			require.NotNil(t, configField)
			require.NotNil(t, configField.Node.Additional)
		})
	}
}

func aiGatewayIdentityProviderExplainBranch(t *testing.T, node *ExplainNode, providerType string) *ExplainNode {
	t.Helper()

	for _, branch := range node.OneOf {
		typeField := branch.propIndex["type"]
		if typeField != nil && typeField.Node.Const == providerType {
			return branch
		}
	}

	require.Failf(t, "missing identity provider explain branch", "provider type %q was not found", providerType)
	return nil
}
