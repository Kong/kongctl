# Declarative Analytics Dashboard

This example manages a Konnect Analytics custom dashboard from an exported JSON
dashboard definition. The sample dashboard is based on a Quick summary dashboard
created in the Konnect UI.

```sh
kongctl plan -f dashboard.yaml
kongctl apply -f dashboard.yaml --auto-approve
```

The `definition` field can be written inline as YAML or loaded from a JSON or
YAML file with `!file`. When using an exported API response, keep the dashboard
`definition` object and omit API-managed fields such as `id`, `created_at`,
`created_by`, and `updated_at`.

To bring a dashboard created in the Konnect UI into GitOps, adopt it first and
then dump the declarative definition:

```sh
kongctl adopt analytics dashboard <dashboard-id> --namespace analytics
kongctl dump declarative --resources=analytics.dashboards \
  --default-namespace=analytics > dashboards.yaml
kongctl plan -f dashboards.yaml --mode apply
```
