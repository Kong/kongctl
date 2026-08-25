# AI Gateway Configuration Templates

This example uses configuration templates to keep OpenAI token costs and an
AI Rate Limiting Advanced policy in one place.

- `costs.yaml` defines reusable model and policy configuration blocks under
  `_templates`.
- `ai-gateway.yaml` defines an OpenAI provider, two models, and a policy that
  use those templates through `_extends`.

Both model targets inherit the same token costs while retaining their own model
configuration. Updating a price in `costs.yaml` updates every target that
extends `gpt-4o-token-costs` the next time the configuration is applied.

The OpenAI authorization header is a deferred secret. `!env` reads
`OPENAI_API_KEY`, while `!secret` combines it with the `Bearer ` prefix and
prevents the resulting value from appearing in a plan.

## Preview the configuration

Run the plan with both files so kongctl can discover the templates and the
resources that use them:

```sh
kongctl plan \
  --mode apply \
  -f costs.yaml \
  -f ai-gateway.yaml \
  --output-file template-plan.json
```

`OPENAI_API_KEY` is not required when creating the plan because secret
resolution is deferred until apply. Inspect the expanded resources:

```sh
jq '
  .changes[]
  | select(
      .resource_ref == "customer-support"
      or .resource_ref == "document-review"
      or .resource_ref == "hourly-model-cost-budget"
    )
  | {resource_ref, fields}
' template-plan.json
```

The two models contain the same inherited token costs. `_templates` and
`_extends` do not appear in the plan.

## Apply the configuration

Set the OpenAI key only when you are ready to apply the configuration:

```sh
export OPENAI_API_KEY='your-openai-api-key'

kongctl apply \
  -f costs.yaml \
  -f ai-gateway.yaml \
  --write-secrets
```

Review the plan and confirm the apply when prompted.

## Update a shared price

Change `input_cost` in `gpt-4o-token-costs`, then run a diff with the same
source set. Both model resources receive the new price without per-model edits:

```sh
sed 's/input_cost: 2.50/input_cost: 3.00/' \
  costs.yaml > costs-updated.yaml

kongctl diff \
  --mode apply \
  -f costs-updated.yaml \
  -f ai-gateway.yaml
```

Use either `costs.yaml` or `costs-updated.yaml` in a command, not both, because
template identifiers must be unique across the complete source set.
