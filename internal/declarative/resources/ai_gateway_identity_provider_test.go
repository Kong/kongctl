package resources

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAIGatewayIdentityProviderExplainRequiresOpenIDConnectCacheTokensSalt(t *testing.T) {
	node, err := aiGatewayIdentityProviderExplainNode(ExplainBuildContext{})
	require.NoError(t, err)

	var openIDConnect *ExplainNode
	for _, branch := range node.OneOf {
		typeField := branch.propIndex["type"]
		if typeField != nil && typeField.Node.Const == "openid-connect" {
			openIDConnect = branch
			break
		}
	}
	require.NotNil(t, openIDConnect)

	configField := openIDConnect.propIndex["config"]
	require.NotNil(t, configField)
	cacheTokensSalt := configField.Node.propIndex["cache_tokens_salt"]
	require.NotNil(t, cacheTokensSalt)
	require.True(t, cacheTokensSalt.Required)
}
