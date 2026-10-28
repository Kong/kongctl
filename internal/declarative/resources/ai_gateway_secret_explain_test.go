package resources

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAgentAndMCPSecretsTraverseNestedSDKUnions(t *testing.T) {
	for _, builder := range []func(ExplainBuildContext) (*ExplainNode,
		error){
		aiGatewayAgentExplainNode,
		aiGatewayMCPServerExplainNode,
	} {
		node, err := builder(ExplainBuildContext{})
		require.NoError(t, err)
		for _, path := range [][]string{
			{
				"config",
				"upstream",
				"auth",
				"secret_access_key",
			},
			{
				"config",
				"upstream",
				"auth",
				"session_token",
			},
			{
				"config",
				"proxy",
				"auth",
				"password",
			},
		} {
			t.Run(
				path[1]+"/"+path[len(path)-1],
				func(t *testing.T) { require.Positive(t, requireSecretExplainPath(t, node, path)) },
			)
		}
	}
	node, err := aiGatewayMCPServerExplainNode(ExplainBuildContext{})
	require.NoError(t, err)
	require.Positive(
		t,
		requireSecretExplainPath(
			t,
			node,
			[]string{"config", "server", "tools_list_auth", "client_secret"},
		),
	)
}

func requireSecretExplainPath(t *testing.T, node *ExplainNode, path []string) int {
	t.Helper()
	if node == nil {
		return 0
	}
	if len(path) == 0 {
		require.Equal(t, "!secret", node.PreferredTag)
		return 1
	}
	head, tail := path[0], path[1:]
	count := 0
	for _, branch := range node.OneOf {
		count += requireSecretExplainPath(t, branch, path)
	}
	if field, ok := node.property(head); ok {
		count += requireSecretExplainPath(t, field.Node, tail)
	}
	return count
}
