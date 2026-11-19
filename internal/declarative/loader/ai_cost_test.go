package loader

import (
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/stretchr/testify/require"
)

func TestAICostRulesAndCollectionScopes(t *testing.T) {
	const manifest = `
_defaults:
  kongctl: {namespace: operations}
ai:
  cost_management:
    budgets:
      - ref: external-budget
        _external: {id: 4078f559-063b-4f10-9098-9e6553b21d9b}
        notification_rule:
          thresholds: [{percent: 80}]
          kongctl: {namespace: alerts}
    notification_channels: []
`
	rs, err := New().LoadFile(writeLoaderTestFile(t, manifest))
	require.NoError(t, err)
	cost := rs.CostManagement()
	require.Nil(t, cost.Budgets[0].NotificationRule)
	require.Len(t, cost.NotificationRules, 1)
	rule := cost.NotificationRules[0]
	require.Equal(t, "external-budget", rule.Budget)
	require.Equal(t, "alerts", resources.GetNamespace(rule.Kongctl))
	require.Empty(t, rs.SyncScope.AICostNamespaces[resources.ResourceTypeAICostBudget])
	require.Contains(t, rs.SyncScope.AICostNamespaces[resources.ResourceTypeAICostChannel], "operations")
	require.Empty(t, rs.SyncScope.AICostNamespaces[resources.ResourceTypeAICostSchedule])
}

func TestAICostSlackRequiresDeferredSecret(t *testing.T) {
	const manifest = `
ai:
  cost_management:
    notification_channels:
      - ref: slack-alerts
        name: Slack alerts
        type: slack
        slack:
          webhook_url: `
	rs, err := New().LoadFile(writeLoaderTestFile(t, manifest+"!secret {source: !env TEST_SLACK_WEBHOOK}\n"))
	require.NoError(t, err)
	require.Contains(t, rs.SecretSources["slack-alerts"], "/slack/webhook_url")
	_, err = New().LoadFile(writeLoaderTestFile(t, manifest+"literal-webhook\n"))
	require.ErrorContains(t, err, "!secret")
}
