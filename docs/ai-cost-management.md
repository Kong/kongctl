# AI Cost Management

Use `kongctl get ai cost-management` to inspect budgets, alert channels,
export schedules, usage events, and spend. The explicit Konnect form also works.

```sh
kongctl get ai cost-management budget
kongctl get ai cost-management budget <budget-id>
kongctl get ai cost-management budget balance --budget-id <budget-id>
kongctl get ai cost-management budget notification-rule --budget-id <budget-id>
kongctl get ai cost-management budget notification-rule firing --budget-id <budget-id>
kongctl get ai cost-management notifications channels
kongctl get ai cost-management notifications events
kongctl get ai cost-management attribution-plugins
kongctl get ai cost-management attribution-plugins <ai-gateway-id>
kongctl get ai cost-management forecast
```

Manage budgets, notification channels, notification rules, and export schedules
under `ai.cost_management` in a declarative manifest. See the
[runnable example](examples/declarative/ai/cost-management/README.md).

```yaml
_defaults:
  kongctl:
    namespace: finance
ai:
  cost_management:
    budgets:
      - ref: monthly-ai
        name: Monthly AI spend
        effect: limit
        scope: {filters: []}
        limit: {value: "5000", currency: USD}
        window:
          type: fixed
          interval: P1M
          anchored_at: "2026-11-01T00:00:00+00:00[UTC]"
        notification_rule:
          thresholds: [{percent: 80}]
          channels:
            - id: !ref finance-email#id
    notification_channels:
      - ref: finance-email
        name: Finance alerts
        type: email
        email:
          recipients: [finance@example.com]
    export_schedules:
      - ref: monthly-usage
        name: Monthly AI usage
        cadence: monthly
        dataset: usage_events
        format: csv
        compression: gzip
```

Budget, channel, and schedule identity is the declared `ref` within its
namespace and resource type. Changing a name retains the resource ID.
Changing a ref creates a different resource. Use `_external: {id: "..."}` to
reference an existing budget or channel by UUID.

A limit budget can have one notification rule. To manage the rule from a
separate namespace, declare the budget as external and use
`ai.cost_management.notification_rules` with a `budget` reference. For example:

```yaml
_defaults:
  kongctl:
    namespace: operations
ai:
  cost_management:
    budgets:
      - ref: shared-budget
        _external: {id: "<budget UUID>"}
    notification_rules:
      - budget: shared-budget
        thresholds: [{percent: 90}]
```

Omitting a rule preserves it. To remove it, use `kongctl delete -f` with a
manifest selecting the rule. A budget with a rule requires explicit deletion
of the rule. Detach a channel from its rules before deleting the channel.
Deleting a notification rule also removes its firing and delivery history.

Sync scopes budget, channel, and schedule pruning by collection and namespace.
An explicit empty collection scopes that collection to the file's default
namespace. An omitted collection leaves it alone. Changing the budget effect,
channel type, or schedule cadence or time zone requires explicit replacement.
Omitting the schedule status or time zone preserves its current value. Omitted
labels and schedule filters reset to an empty map. Omitted budget priority and
limit posture reset to `100` and `advisory`, respectively.

On a Slack channel, use `slack.webhook_url: !secret {source: !env SLACK_WEBHOOK}`.
Creating the channel requires this source. To rotate a webhook on an existing
channel, use `--write-secret '<channel-ref>#slack.webhook_url'`. An ordinary
update preserves the stored webhook. Dumps exclude the webhook.

Dump managed resources by specifying one or more collection paths.

```sh
kongctl dump declarative --resources ai.cost_management.budgets
```
Add `--include-child-resources` to include budget notification rules.

## Attribution plugins

The attribution plugin turns request headers and token claims into the tags and
groups that budgets filter on. Declare one installation per AI Gateway under
`ai.cost_management.attribution_plugins`. `ai_gateway` references a declared
AI Gateway, which can be `_external`.

```yaml
ai_gateways:
  - ref: engineering
    _external:
      selector:
        matchFields: {name: Engineering}
ai:
  cost_management:
    attribution_plugins:
      - ref: engineering-attribution
        ai_gateway: engineering
        integrations: [claude-code, codex]
        headers:
          - {name: x-acme-tier, tag_key: pricing-tier}
        operator_tags: {cost-center: platform}
        install_version: "1.4.0"
```

`integrations` is required; use `[]` for none. Omitted `headers`, `prefixes`,
`operator_tags`, and `group_patterns` reset to empty. Without
`install_version`, each write installs the latest plugin version. Set
`install_version` to the latest version to pin it:
the plan then upgrades only when you change that value, and an apply fails
when a newer version was installed outside the manifest until you set it.
`kongctl get ai cost-management attribution-plugins` shows each installation's
`version` and `latest_version`.

kongctl refuses to take over an installation it doesn't manage, such as one
made in Konnect. Remove that installation first. Moving a declaration to
another AI Gateway requires a new ref. Sync prunes installations by namespace
like budgets. Removing an installation while budget blocking is on for its
gateway fails until blocking is turned off.
