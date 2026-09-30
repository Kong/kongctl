package planner

import (
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/stretchr/testify/require"
)

func TestAIGatewayAuthStrategyRenameWithStableRef(t *testing.T) {
	rs := &resources.ResourceSet{AIGatewayAuthStrategies: []resources.AIGatewayAuthStrategyResource{{
		BaseResource: resources.BaseResource{Ref: "old-name"}, Name: "new-name",
	}}}
	for _, selector := range []string{"id", "name"} {
		t.Run(selector, func(t *testing.T) {
			desired := authStrategyAccessFields([]string{"__REF__:old-name#" + selector})
			current := authStrategyAccessFields([]string{"old-name"})
			before, after := normalizeAIGatewayAuthStrategyReferencesForComparison(current, desired, rs)
			require.NotEqual(t, before, after, "renaming must update the user's attachment")
			require.Equal(t, []string{"old-name"}, aiGatewayAuthStrategies(before))
			require.Equal(t, []string{"new-name"}, aiGatewayAuthStrategies(after))
			current = authStrategyAccessFields([]string{"new-name"})
			before, after = normalizeAIGatewayAuthStrategyReferencesForComparison(current, desired, rs)
			require.Equal(t, before, after, "the renamed reference must converge")
			require.Equal(t, []string{"__REF__:old-name#" + selector}, aiGatewayAuthStrategies(desired))
		})
	}
	current := authStrategyAccessFields([]string{"old-name"})
	before, after := normalizeAIGatewayAuthStrategyReferencesForComparison(current, current, rs)
	// A literal old name must not acquire the meaning of a ref with that name.
	for _, payload := range []map[string]any{before, after} {
		require.Equal(t, []string{"old-name"}, aiGatewayAuthStrategies(payload))
	}
}
