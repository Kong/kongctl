# Secrets Example

This example demonstrates how to declare, plan, and write secrets without
placing their resolved values in declarative configuration or saved plans.

It includes three representative cases:

- An updateable DCR provider API key using one environment source.
- An AI provider authorization header composed from a public `Bearer ` prefix
  and a secret token.
- A create-only AI Consumer Credential API key.

For the complete behavior reference, see
[Write-only Secret Fields](../../../declarative.md#write-only-secret-fields).

## Configuration

[config.yaml](config.yaml) uses these declarations:

```yaml
api_key: !secret {source: !env DCR_API_KEY}
```

```yaml
value: !secret
  parts:
    - "Bearer "
    - !env OPENAI_API_KEY
```

The first form defers one environment value. The second constructs the final
sensitive value during execution, so the environment contains only the token.
The public `Bearer ` prefix can appear in plan metadata; the token and completed
header cannot.

Replace the example issuer, DCR endpoint, AI Gateway proxy hostname, and other
placeholder settings with values valid for your Konnect environment before
executing the configuration.

## Create the resources

A create automatically includes each configured secret once. A write-selection
flag is not required.

Planning validates the source declarations but does not read the environment
values:

```sh
kongctl plan --mode apply -f config.yaml \
  --output-file create-plan.json
```

The execution environment must provide all three non-empty values. Supply them
through your CI system, shell environment, or secret manager, then apply the
saved plan:

```sh
# The execution environment provides and exports:
# DCR_API_KEY, OPENAI_API_KEY, and CLIENT_API_KEY
kongctl apply --plan create-plan.json --auto-approve
```

Do not put secret values directly in the command, configuration, or a committed
`.env` file.

## Inspect a saved plan

The plan records fields and source references, not resolved values. If `jq` is
available, inspect the secret-write metadata with:

```sh
jq '[.changes[] | select(.secret_writes) |
  {resource_ref, secret_writes}]' create-plan.json
```

The output can contain environment variable names and public literal parts.
It must not contain the values of those variables.

## Rotate one secret

After the resources exist, merely retaining a `!secret` declaration does not
rewrite it. Authorize one exact update while generating a plan:

```sh
kongctl plan --mode apply -f config.yaml \
  --write-secret "secrets-http-dcr#dcr_config.api_key" \
  --output-file rotate-dcr-key.json
```

Planning still does not require `DCR_API_KEY`. The execution environment
provides its new value when the reviewed plan is applied:

```sh
# The execution environment provides and exports DCR_API_KEY.
kongctl apply --plan rotate-dcr-key.json --auto-approve
```

Do not add `--write-secret` when applying a saved plan. The write authorization
is already recorded in that artifact.

To select all eligible secrets on one resource, omit the field:

```sh
kongctl plan --mode apply -f config.yaml \
  --write-secret "secrets-openai-provider" \
  --output-file rotate-provider-secrets.json
```

The exact array-backed field selector is also supported. Quote it so shell glob
processing does not interpret the brackets:

```sh
kongctl plan --mode apply -f config.yaml \
  --write-secret \
  "secrets-openai-provider#config.auth.headers[].value" \
  --output-file rotate-provider-token.json
```

Konnect currently permits at most one model-provider authentication header.
The selector uses `headers[]` because it follows the array-shaped API field and
does not depend on the configured header remaining at index `0`.

## Write every eligible secret

Use the aggregate flag to select eligible secrets throughout the configuration:

```sh
kongctl plan --mode apply -f config.yaml --write-secrets \
  --output-file rotate-all-secrets.json
```

Once the example resources exist, this plan includes the DCR API key and AI
provider header. The existing `secrets-client-key` credential is create-only,
so kongctl skips it and writes a warning to standard error. The warning is also
stored in the plan metadata for review.

`--write-secrets` is best-effort. Other eligible writes remain in the plan when
one field is skipped. If no eligible secret is found, the command succeeds and
warns that it selected no writable secret fields.

An exact selector remains strict. This command fails instead of silently
skipping the requested create-only field:

```sh
kongctl plan --mode apply -f config.yaml \
  --write-secret "secrets-client-key#api_key"
```

## Rotate a create-only credential

An AI Consumer Credential API key cannot be updated in place. To rotate it:

1. Add a new credential with a new `ref`, `name`, and environment source.
2. Apply the configuration so kongctl creates the new credential and writes
   its key once.
3. Move clients to the new credential.
4. Deliberately remove the old credential.

kongctl does not turn a secret-write request into an implicit delete and
recreate operation.

## Environment and file recommendations

`!secret` currently accepts deferred `!env` sources. kongctl does not
automatically load `.env` files. A CI runner, secret manager, or dotenv tool can
populate several environment values before execution; kongctl reads them from
the process environment during secret preflight.

Literal values and eager `!file` values are rejected on reviewed secret fields
because their contents could otherwise enter a saved plan. Do not place secret
material in a `parts` literal. A future deferred file source can extend the
source model without changing the write-selection workflow.

Every required source must resolve to a non-empty string. kongctl checks all
sources before the first API mutation, so one missing value prevents a partial
execution.

## Clean up

Delete does not write configured secrets and does not accept secret-selection
flags:

```sh
kongctl delete -f config.yaml --auto-approve
```
