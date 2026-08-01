package resources

import (
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
