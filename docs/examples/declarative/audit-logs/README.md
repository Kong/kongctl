# Portal Audit Log Webhook Example

This example shows how to configure a portal audit-log webhook with an
organization audit-log destination that is managed outside declarative
configuration.

The destination is declared under `audit-logs.destinations` with `_external`.
kongctl resolves the destination by name, then uses its Konnect ID when it
updates the portal `audit_log_webhook`.

## Files

- `portal-audit-log-webhook.yaml` - creates a portal and enables the portal
  audit-log webhook.

## Prerequisites

Create the audit-log destination before applying this example. The destination
name in Konnect must match the selector in the YAML file:

```yaml
audit-logs:
  destinations:
    - ref: audit-log-destination
      _external:
        selector:
          matchFields:
            name: production-audit-log-webhook
```

## Usage

Preview the changes:

```bash
kongctl plan -f portal-audit-log-webhook.yaml --mode apply
```

Apply the portal and webhook configuration:

```bash
kongctl apply -f portal-audit-log-webhook.yaml --auto-approve
```

Remove the example when you are done:

```bash
kongctl delete -f portal-audit-log-webhook.yaml --auto-approve
```
