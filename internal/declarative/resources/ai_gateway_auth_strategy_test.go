package resources

import (
	"testing"

	"github.com/stretchr/testify/require"
)

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

func TestAIGatewayAuthStrategyExplainAllowsAdditionalConfigProperties(t *testing.T) {
	node, err := aiGatewayAuthStrategyExplainNode(ExplainBuildContext{})
	require.NoError(t, err)

	for _, providerType := range []string{"key-auth", "openid-connect"} {
		t.Run(providerType, func(t *testing.T) {
			branch := aiGatewayAuthStrategyExplainBranch(t, node, providerType)
			configField := branch.propIndex["config"]
			require.NotNil(t, configField)
			require.NotNil(t, configField.Node.Additional)
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
