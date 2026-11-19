# AI Cost Management examples

Choose one workflow and run the commands from this directory. Replace the
example email addresses before applying.

## Manage a budget with alerts and exports

[budget-and-alerts.yaml](budget-and-alerts.yaml) declares a monthly USD limit
budget, an alert at 80%, an email channel, and a paused monthly CSV export
schedule. The rule uses `!ref finance-email#id` to reference the channel.

```sh
kongctl plan --mode apply -f budget-and-alerts.yaml
kongctl apply -f budget-and-alerts.yaml
kongctl get ai cost-management budgets
kongctl get ai cost-management export-schedules
```

To remove the resources, run `kongctl delete -f budget-and-alerts.yaml`.

## Let another team manage the notification rule

Finance applies [budget.yaml](federated/budget.yaml). Operations independently
applies [notification-rule.yaml](federated/notification-rule.yaml), which
references the existing budget by UUID and owns its rule and email channel.

```sh
kongctl apply -f federated/budget.yaml
kongctl get ai cost-management budgets -o json
export AI_COST_EXAMPLE_BUDGET_ID='<UUID of Example shared monthly AI spend>'
kongctl plan --mode apply -f federated/notification-rule.yaml
kongctl apply -f federated/notification-rule.yaml
kongctl get ai cost-management notification-rules "$AI_COST_EXAMPLE_BUDGET_ID"
```

To clean up, delete the rule and channel before deleting the budget.

```sh
kongctl delete -f federated/notification-rule.yaml
kongctl delete -f federated/budget.yaml
```

See the [AI Cost Management guide](../../../ai-cost-management.md) for details
on ownership, sync, and Slack webhook secrets.
