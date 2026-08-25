---
title: Configuration Templates
summary: Reuse declarative configuration across resources and files.
order: 7
related:
  - label: Declarative configuration examples
    url: https://github.com/Kong/kongctl/tree/main/docs/examples/declarative
  - label: AI Rate Limiting Advanced documentation
    url: https://developer.konghq.com/plugins/ai-rate-limiting-advanced/
---

## Goal

You will define named configuration templates, extend them from resources in a
different file, and observe how inherited and local values merge. Shared
OpenAI model prices provide the working example.

## How templates work

Define reusable configuration blocks under the top-level `_templates` key. Add
`_extends` to a resource or nested configuration block to inherit one named
template. The configuration block containing `_extends` can add fields or
override inherited values.

Templates are shared across all sources supplied to one command. They can
therefore live in a dedicated file while the resources that extend them live
elsewhere.

## Prerequisites

Complete [Konnect Authentication](../../installation/authenticate/) and make
sure the active profile can create AI Gateway resources.

You can complete all planning steps without an OpenAI key. The key is only
needed if you apply the configuration.

## Why centralize model costs?

AI Rate Limiting Advanced can enforce limits based on model cost. Those limits
depend on the input and output token prices configured for each model target.
If several model resources use the same target, repeating its prices makes
updates easy to miss.

A model template provides one place to update the price. Every resource that
extends it receives the updated configuration on the next plan or apply.

## Define the templates

Create and enter a working directory:

```shell label="run this..."
mkdir -p aigw-templates && cd aigw-templates
```

Create `costs.yaml`:

```shell label="run this..."
cat > costs.yaml <<'YAML'
_templates:
  gpt-4o-token-costs:
    # Illustrative USD cost per one million tokens.
    input_cost: 2.50
    output_cost: 10.00

  hourly-model-cost-budget:
    type: ai-rate-limiting-advanced
    enabled: true
    global: false
    labels:
      managed-by: kongctl
      purpose: model-cost-budget
    config:
      strategy: local
      window_type: sliding
      policies:
        - match:
            - type: model
              partition_by: true
          limits:
            - limit: 25
              window_size: 3600
              tokens_count_strategy: cost
YAML
```

The first template contains the shared OpenAI token costs. The policy template
sets a shared hourly cost limit. Defining these templates does not create
resources by itself.

## Extend the templates

Create `ai-gateway.yaml`:

```shell label="run this..."
cat > ai-gateway.yaml <<'YAML'
_defaults:
  kongctl:
    namespace: aigw-templates-learning

ai_gateways:
  - ref: shared-cost-gateway
    name: shared-cost-gateway
    display_name: Shared Cost Gateway
    description: Centralizes model token costs for cost-based rate limiting
    proxy_urls:
      - host: shared-cost-gateway.example.com
        port: 443
        protocol: https

    model_providers:
      - ref: openai-provider
        name: openai-provider
        display_name: OpenAI
        type: openai
        config:
          auth:
            type: basic
            headers:
              - name: Authorization
                value: !secret
                  parts:
                    - "Bearer "
                    - !env OPENAI_API_KEY

    models:
      - ref: customer-support
        name: customer-support
        display_name: Customer Support
        type: model
        formats:
          - type: openai
        config:
          route:
            model:
              body_param: model
              values:
                - customer-support
        targets:
          - name: gpt-4o
            provider: openai-provider
            config:
              _extends: gpt-4o-token-costs
              type: openai
        policies:
          - !ref hourly-model-cost-budget#name
        capabilities:
          - generate

      - ref: document-review
        name: document-review
        display_name: Document Review
        type: model
        formats:
          - type: openai
        config:
          route:
            model:
              body_param: model
              values:
                - document-review
        targets:
          - name: gpt-4o
            provider: openai-provider
            config:
              type: openai
              _extends: gpt-4o-token-costs
        policies:
          - !ref hourly-model-cost-budget#name
        capabilities:
          - generate

    policies:
      - _extends: hourly-model-cost-budget
        ref: hourly-model-cost-budget
        name: hourly-model-cost-budget
        display_name: Hourly Model Cost Budget
YAML
```

The provider uses `!env` to identify the environment variable and `!secret` to
compose the full authorization header. The value is deferred until apply and
does not appear in a plan.

Both model targets inherit the shared costs while retaining their own model
configuration. The policy resource inherits its complete configuration from
`hourly-model-cost-budget`.

## Preview the expanded configuration

Supply both files to the same command. A resource can extend a template defined
in any file in the command's source set:

```shell label="run this..."
kongctl plan \
  --mode apply \
  -f costs.yaml \
  -f ai-gateway.yaml \
  --output-file template-plan.json
```

Inspect the materialized models and policy:

```shell label="run this..."
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

Both models contain the inherited input and output costs, as well as their
local route. The policy contains the inherited cost limit. `_templates` and
`_extends` do not appear in the plan.

Try planning only the resource file:

```shell label="run this..."
kongctl diff --mode apply -f ai-gateway.yaml
```

The command fails with an unknown-template error. Template discovery is
cross-file, but it is limited to files, directories, standard input, and URLs
supplied to that command.

## Understand the merge rules

The configuration block containing `_extends` is the consumer. Its values
override the selected template:

| Template and consumer values  | Effective value                         |
| ----------------------------- | --------------------------------------- |
| Both are configuration blocks | Recursively merge their keys.           |
| Consumer key is omitted       | Keep the template value.                |
| Consumer scalar               | Replace the template value.             |
| Consumer array                | Replace the whole template array.       |
| Consumer value is `null`      | Replace the template value with `null`. |
| Value types differ            | Replace with the consumer value.        |

In this example, `_extends` is used inside each target's `config` configuration
block and on the policy resource itself. Its position among the other keys in a
configuration block does not affect the result. Arrays do not append: a local
array replaces the complete inherited array at the same key.

Each configuration block can extend one template. Template identifiers must be
unique in the complete source set. Unknown names, duplicate names, and circular
inheritance are errors.

## Apply the configuration

Skip this section if you only want to inspect expansion. Otherwise, set the
OpenAI key and apply the same source set:

```shell label="run this..."
export OPENAI_API_KEY='your-openai-api-key'

kongctl apply \
  -f costs.yaml \
  -f ai-gateway.yaml \
  --write-secrets
```

Review the plan and confirm the apply when prompted.

## Update a shared price

Create a new template file with the input price changed:

```shell label="run this..."
sed 's/input_cost: 2.50/input_cost: 3.00/' \
  costs.yaml > costs-updated.yaml
```

Use the updated file instead of the original so only one definition of each
template is present:

```shell label="run this..."
kongctl diff \
  --mode apply \
  -f costs-updated.yaml \
  -f ai-gateway.yaml
```

If you applied the initial configuration, the diff updates both
`customer-support` and `document-review`. Neither resource was edited; both
receive the new price from `gpt-4o-token-costs`.

## Delete the resources

If you applied the example, delete the resources represented by the files:

```shell label="run this..."
kongctl delete \
  -f costs-updated.yaml \
  -f ai-gateway.yaml
```

The template file remains necessary because the resource file contains
`_extends`. Delete mode targets only the resources produced after expansion.
