package resources

import (
	"slices"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDashboardQueryExplainBranchesConformToSDKShapes(t *testing.T) {
	query := dashboardQueryExplainNode()
	require.Len(t, query.OneOf, 4)

	t.Run("api_usage", func(t *testing.T) {
		assertCustomExplainSupportsSDKShape[kkComps.AdvancedQuery](t, query.OneOf[0])
	})
	t.Run("llm_usage", func(t *testing.T) {
		assertCustomExplainSupportsSDKShape[kkComps.LLMQuery](t, query.OneOf[1])
	})
	t.Run("agentic_usage", func(t *testing.T) {
		assertCustomExplainSupportsSDKShape[kkComps.AgenticQuery](t, query.OneOf[2])
	})
	t.Run("platform_usage", func(t *testing.T) {
		assertCustomExplainSupportsSDKShape[kkComps.PlatformQuery](t, query.OneOf[3])
	})
}

func TestAIGatewayCustomExplainSchemasPreserveSDKOpenness(t *testing.T) {
	tests := []struct {
		name     string
		expected func(*testing.T, *ExplainNode)
		actual   func() (*ExplainNode, error)
	}{
		{
			name: "gateway",
			expected: func(t *testing.T, actual *ExplainNode) {
				assertCustomExplainPreservesSDKOpenness[kkComps.CreateAIGatewayRequest](t, actual)
				assertCustomExplainPreservesSDKOpenness[kkComps.UpdateAIGatewayRequest](t, actual)
			},
			actual: func() (*ExplainNode, error) { return aiGatewayExplainNode(ExplainBuildContext{}) },
		},
		{
			name: "agent",
			expected: func(t *testing.T, actual *ExplainNode) {
				assertCustomExplainPreservesSDKOpenness[kkComps.CreateAIGatewayAgentRequest](t, actual)
				assertCustomExplainPreservesSDKOpenness[kkComps.UpdateAIGatewayAgentRequest](t, actual)
			},
			actual: func() (*ExplainNode, error) { return aiGatewayAgentExplainNode(ExplainBuildContext{}) },
		},
		{
			name: "consumer",
			expected: func(t *testing.T, actual *ExplainNode) {
				assertCustomExplainPreservesSDKOpenness[kkComps.CreateAIGatewayConsumerRequest](t, actual)
				assertCustomExplainPreservesSDKOpenness[kkComps.UpdateAIGatewayConsumerRequest](t, actual)
			},
			actual: func() (*ExplainNode, error) { return aiGatewayConsumerExplainNode(ExplainBuildContext{}) },
		},
		{
			name: "consumer group",
			expected: func(t *testing.T, actual *ExplainNode) {
				assertCustomExplainPreservesSDKOpenness[kkComps.CreateAIGatewayConsumerGroupRequest](t, actual)
				assertCustomExplainPreservesSDKOpenness[kkComps.UpdateAIGatewayConsumerGroupRequest](t, actual)
			},
			actual: func() (*ExplainNode, error) { return aiGatewayConsumerGroupExplainNode(ExplainBuildContext{}) },
		},
		{
			name: "identity provider",
			expected: func(t *testing.T, actual *ExplainNode) {
				assertCustomExplainPreservesSDKOpenness[kkComps.AIGatewayIdentityProviderKeyAuth](
					t,
					aiGatewayIdentityProviderExplainBranch(t, actual, "key-auth"),
				)
				assertCustomExplainPreservesSDKOpenness[kkComps.AIGatewayIdentityProviderOpenIDConnect](
					t,
					aiGatewayIdentityProviderExplainBranch(t, actual, "openid-connect"),
				)
			},
			actual: func() (*ExplainNode, error) {
				return aiGatewayIdentityProviderExplainNode(ExplainBuildContext{})
			},
		},
		{
			name: "MCP server",
			expected: func(t *testing.T, actual *ExplainNode) {
				assertCustomExplainPreservesSDKOpenness[kkComps.AIGatewayMCPServerConversionOnly](
					t,
					explainUnionBranchByType(t, actual, "conversion-only"),
				)
				assertCustomExplainPreservesSDKOpenness[kkComps.AIGatewayMCPServerConversionListener](
					t,
					explainUnionBranchByType(t, actual, "conversion-listener"),
				)
				assertCustomExplainPreservesSDKOpenness[kkComps.AIGatewayMCPServerListener](
					t,
					explainUnionBranchByType(t, actual, "listener"),
				)
				assertCustomExplainPreservesSDKOpenness[kkComps.AIGatewayMCPServerPassthroughListener](
					t,
					explainUnionBranchByType(t, actual, "passthrough-listener"),
				)
				assertCustomExplainPreservesSDKOpenness[kkComps.AIGatewayMCPServerUpstreamServer](
					t,
					explainUnionBranchByType(t, actual, "upstream-server"),
				)
			},
			actual: func() (*ExplainNode, error) { return aiGatewayMCPServerExplainNode(ExplainBuildContext{}) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := tt.actual()
			require.NoError(t, err)
			tt.expected(t, actual)
		})
	}
}

func explainUnionBranchByType(t *testing.T, node *ExplainNode, resourceType string) *ExplainNode {
	t.Helper()
	for _, branch := range node.OneOf {
		field, ok := branch.property("type")
		if ok && field.Node.Const == resourceType {
			return branch
		}
	}
	require.Failf(t, "missing explain union branch", "resource type %q was not found", resourceType)
	return nil
}

func assertCustomExplainPreservesSDKOpenness[T any](t *testing.T, actual *ExplainNode) {
	t.Helper()

	expected, err := autoExplainConcreteNode[T](nil)
	require.NoError(t, err)
	assertExplainNodePreservesSDKOpenness(t, "", expected, actual)
}

func assertExplainNodePreservesSDKOpenness(t *testing.T, path string, expected, actual *ExplainNode) {
	t.Helper()
	require.NotNilf(t, actual, "custom explain field %q has no schema", path)
	if expected.Additional != nil {
		require.NotNilf(t, actual.Additional, "custom explain field %q closes an SDK-open object", path)
	}

	for _, expectedField := range expected.Properties {
		actualField, ok := actual.property(expectedField.Name)
		if !ok {
			require.Falsef(
				t,
				explainNodeContainsSDKOpenness(expectedField.Node),
				"custom explain schema is missing SDK-open field %q",
				expectedField.Name,
			)
			continue
		}
		fieldPath := expectedField.Name
		if path != "" {
			fieldPath = path + "." + expectedField.Name
		}
		assertExplainNodePreservesSDKOpenness(t, fieldPath, expectedField.Node, actualField.Node)
	}

	if expected.Items != nil {
		if explainNodeContainsSDKOpenness(expected.Items) {
			require.NotNilf(t, actual.Items, "custom explain field %q omits SDK-open array items", path)
		}
		if actual.Items != nil {
			assertExplainNodePreservesSDKOpenness(t, path+"[]", expected.Items, actual.Items)
		}
	}
	if explainNodesContainSDKOpenness(expected.OneOf) {
		require.GreaterOrEqual(t, len(actual.OneOf), len(expected.OneOf))
	}
	for i := range min(len(expected.OneOf), len(actual.OneOf)) {
		assertExplainNodePreservesSDKOpenness(t, path, expected.OneOf[i], actual.OneOf[i])
	}
}

func explainNodeContainsSDKOpenness(node *ExplainNode) bool {
	if node == nil {
		return false
	}
	if node.Additional != nil {
		return true
	}
	for _, field := range node.Properties {
		if explainNodeContainsSDKOpenness(field.Node) {
			return true
		}
	}
	if explainNodeContainsSDKOpenness(node.Items) {
		return true
	}
	return explainNodesContainSDKOpenness(node.OneOf)
}

func explainNodesContainSDKOpenness(nodes []*ExplainNode) bool {
	return slices.ContainsFunc(nodes, explainNodeContainsSDKOpenness)
}

func assertCustomExplainSupportsSDKShape[T any](t *testing.T, actual *ExplainNode) {
	t.Helper()

	expected, err := autoExplainConcreteNode[T](nil)
	require.NoError(t, err)
	require.NotNil(t, actual)
	require.Equal(t, expected.Kind, actual.Kind)

	for _, expectedField := range expected.Properties {
		actualField, ok := actual.property(expectedField.Name)
		require.Truef(t, ok, "custom explain schema is missing SDK field %q", expectedField.Name)
		assertExplainFieldShapeCompatible(t, expectedField.Name, expectedField.Node, actualField.Node)
	}
}

func assertExplainFieldShapeCompatible(t *testing.T, path string, expected, actual *ExplainNode) {
	t.Helper()

	require.NotNilf(t, actual, "custom explain field %q has no schema", path)
	assert.Equalf(t, expected.Kind, actual.Kind, "custom explain field %q has an incompatible kind", path)
	if expected.Kind == explainKindArray {
		require.NotNilf(t, actual.Items, "custom explain array field %q has no item schema", path)
		assert.Equalf(
			t,
			expected.Items.Kind,
			actual.Items.Kind,
			"custom explain array field %q has an incompatible item kind",
			path,
		)
	}
	if len(expected.OneOf) > 0 {
		require.Lenf(t, actual.OneOf, len(expected.OneOf), "custom explain union field %q has incompatible branches", path)
		for i := range expected.OneOf {
			assert.Equalf(
				t,
				expected.OneOf[i].Kind,
				actual.OneOf[i].Kind,
				"custom explain union field %q branch %d has an incompatible kind",
				path,
				i,
			)
		}
	}
}
