package resources

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAICostScaffoldUsesManifestGrouping(t *testing.T) {
	for _, path := range []string{"ai.cost_management.budgets", "ai.cost_management.notification_rules"} {
		subject, err := ResolveExplainSubject(path)
		require.NoError(t, err)
		yaml, err := RenderScaffoldYAML(subject)
		require.NoError(t, err)
		require.Contains(t, yaml, "ai:\n  cost_management:\n")
		require.NotContains(t, yaml, "managed_by:")
	}
}
