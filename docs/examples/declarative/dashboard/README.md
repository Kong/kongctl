# Declarative Custom Dashboard

This example manages a Konnect Analytics custom dashboard from an exported JSON
dashboard definition.

```sh
kongctl plan -f dashboard.yaml
kongctl apply -f dashboard.yaml --auto-approve
```

The `definition` field can be written inline as YAML or loaded from a JSON or
YAML file with `!file`.

To bring a dashboard created in the Konnect UI into GitOps, adopt it first and
then dump the declarative definition:

```sh
kongctl adopt dashboard <dashboard-id> --namespace analytics
kongctl dump declarative --resources=dashboard \
  --default-namespace=analytics > dashboards.yaml
kongctl plan -f dashboards.yaml --mode apply
```
