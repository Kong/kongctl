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

func TestAICostAttributionPluginLoads(t *testing.T) {
	const manifest = `
_defaults:
  kongctl: {namespace: platform}
ai_gateways:
  - ref: engineering
    name: Engineering
ai:
  cost_management:
    attribution_plugins:
      - ref: engineering-attribution
        ai_gateway: engineering
        integrations: [claude-code, codex]
        operator_tags: {cost-center: platform}
        install_version: "1.4.0"
`
	rs, err := New().LoadFile(writeLoaderTestFile(t, manifest))
	require.NoError(t, err)
	plugins := rs.CostManagement().AttributionPlugins
	require.Len(t, plugins, 1)
	require.Equal(t, "engineering", plugins[0].AIGateway)
	require.Equal(t, "1.4.0", *plugins[0].InstallVersion)
	require.Contains(t, rs.SyncScope.AICostNamespaces[resources.ResourceTypeAICostPlugin], "platform")
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

func TestAICostCollectionsValidateDeclarations(t *testing.T) {
	tests := []struct {
		name, declarations, want string
	}{
		{"budget", `    budgets:
      - ref: exemption
        name: Exemption
        effect: exempt
        scope: {filters: []}
        validity: {ends_at: "2030-01-01T00:00:00Z"}
        limit: {value: "10", currency: USD}
`, "cannot declare limit"},
		{"channel", `    notification_channels:
      - ref: email
        name: Email
        type: email
        email: {recipients: [finance@example.com]}
        slack: {}
`, "email channel requires email.recipients"},
		{"schedule", `    export_schedules:
      - ref: export
        name: Export
        cadence: monthly
        dataset: usage_events
        format: csv
`, "export schedule requires"},
		{"rule", `    budgets:
      - ref: budget
        _external: {id: 4078f559-063b-4f10-9098-9e6553b21d9b}
        notification_rule:
          thresholds: [{percent: 80}, {percent: 80}]
`, "thresholds must be distinct"},
		{"attribution plugin", `    attribution_plugins:
      - ref: engineering-attribution
        ai_gateway: engineering
`, "requires integrations"},
		{"repeated names", `    notification_channels:
      - ref: finance
        name: Alerts
        type: email
        email: {recipients: [finance@example.com]}
      - ref: operations
        name: Alerts
        type: email
        email: {recipients: [operations@example.com]}
`, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New().LoadFile(writeLoaderTestFile(t, "ai:\n  cost_management:\n"+tt.declarations))
			if tt.want == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tt.want)
			}
		})
	}
}
