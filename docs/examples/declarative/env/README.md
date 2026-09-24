# !env Example

## Stored typed values

`stored.yaml` demonstrates `store: true`, the equivalent `!env_store`
shorthand, inferred boolean conversion, and structured extraction. Stored
values appear as plaintext in saved plans and may appear in plan/diff output.
Use them for non-sensitive configuration.

```bash
PORTAL_DESCRIPTION="Approved description" \
PORTAL_AUTO_APPROVE=true \
PORTAL_METADATA='{"labels":{"owner":"42"}}' \
  kongctl plan -f stored.yaml > stored-plan.json

kongctl diff --plan stored-plan.json

# These variables were scoped to the plan command. Apply needs none of them.
kongctl apply --plan stored-plan.json --auto-approve
kongctl get portal env-stored-portal -o yaml
```

Alternatively, run apply with `PORTAL_AUTO_APPROVE=false`; the saved plan
still applies `true`. Generate a new plan to approve a different value.

```bash
PORTAL_DESCRIPTION="Approved description" \
PORTAL_AUTO_APPROVE=true \
PORTAL_METADATA='{"labels":{"owner":"42"}}' \
  kongctl delete -f stored.yaml --auto-approve
```

Known SDK types are inferred. Unknown or ambiguous destinations require a
`type`: `string`, `boolean`, `integer`, `number`, `array`, `object`, or
`"null"`. For a rate-limiting policy's dynamic configuration:

```yaml
config:
  redis:
    ssl: !env_store {var: REDIS_SSL, type: boolean}
    ssl_verify: !env_store {var: REDIS_SSL_VERIFY, type: boolean}
  sync_rate: !env_store {var: SYNC_RATE, type: number}
```

See the [conversion contract][stored-env]
for numeric limits, empty/unset values, parsing rules, and restrictions.

[stored-env]:
  ../../../declarative.md#storing-typed-environment-values-in-plans

## Deferred strings

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
