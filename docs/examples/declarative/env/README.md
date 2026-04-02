# !env Example

This example shows how to use `!env` to load an environment variable into an
API field during declarative planning and apply.

The configuration uses the `env-example` namespace so it stays isolated from
other declarative resources in the same Konnect organization.

## Files

- `api.yaml` - declares a single API and reads its `description` field from
  the `API_DESCRIPTION` environment variable

## Usage

Preview the change with an inline environment variable:

```bash
API_DESCRIPTION="API description loaded from env" \
  kongctl diff -f api.yaml
```

Apply the configuration:

```bash
API_DESCRIPTION="API description loaded from env" \
  kongctl apply -f api.yaml --auto-approve
```

Check the created API:

```bash
kongctl get api env-example-api -o yaml
```

Human-readable diff output redacts `!env` values by design. Use `get` after
apply to confirm the resolved description value.

Delete the example resource when you are done:

```bash
API_DESCRIPTION="API description loaded from env" \
  kongctl delete -f api.yaml --auto-approve
```
