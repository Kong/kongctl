---
title: Initial Configuration Setup
summary: Create the Platform, Engineering, and Product configuration files.
order: 4
related:
  - label: YAML tags lesson
    url: https://kong.github.io/kongctl/declarative-configuration/yaml-tags/
---

## Goal

You will create the three team configurations and load them together with
_local_ `!ref` relationships.

## Create the Platform configuration

Set the provider credential used by this example. Replace the placeholder with
your OpenAI API key:

```shell label="Set the provider credential"
export OPENAI_AUTH_HEADER='Bearer <openai-api-key>'
```

Create the shared AI Gateway and model provider:

```shell label="Create platform/ai-gateway.yaml"
cat > platform/ai-gateway.yaml <<'YAML'
_defaults:
  kongctl:
    namespace: federated-aigw

ai_gateways:
  - ref: platform-aigw
    name: platform-aigw
    display_name: Platform AI Gateway
    proxy_urls:
      - host: platform-aigw.example.com
        port: 443
        protocol: https
    labels:
      team: platform
    model_providers:
      - ref: platform-openai
        name: platform-openai
        type: openai
        display_name: Platform OpenAI
        config:
          auth:
            type: basic
            headers:
              - name: Authorization
                value: !env OPENAI_AUTH_HEADER
YAML
```

## Create the satellite configurations

Engineering and Product each declare one model. Both refer to the Platform AI
Gateway by the `ref` loaded from the Platform file:

```shell label="Create engineering/model.yaml"
cat > engineering/model.yaml <<'YAML'
ai_gateway_models:
  - ref: engineering-assistant
    ai_gateway: !ref platform-aigw#id
    name: engineering-assistant
    display_name: "Engineering Assistant"
    type: model
    labels:
      team: engineering
    formats:
      - type: openai
    config:
      route:
        paths:
          - /engineering
        model:
          body_param: model
          values:
            - engineering-assistant
    targets:
      - name: gpt-4o
        provider: platform-openai
        config:
          type: openai
    policies: []
    capabilities:
      - generate
YAML
```

```shell label="Create product/model.yaml"
cat > product/model.yaml <<'YAML'
ai_gateway_models:
  - ref: product-assistant
    ai_gateway: !ref platform-aigw#id
    name: product-assistant
    display_name: "Product Assistant"
    type: model
    labels:
      team: product
    formats:
      - type: openai
    config:
      route:
        paths:
          - /product
        model:
          body_param: model
          values:
            - product-assistant
    targets:
      - name: gpt-4o
        provider: platform-openai
        config:
          type: openai
    policies: []
    capabilities:
      - generate
YAML
```

The routes make the ownership boundary visible to callers. Requests sent to
`/engineering` select the Engineering model, while `/product` selects the
Product model. Each route also restricts the request body's `model` value to
the model managed by that team.

> _Note:_ Child resources like these do not have `kongctl` metadata.
> For example they inhert the parent `namespace` and `protection` settings.

## Apply as one shared plan

Run this `apply` command which loads the all 3 files recursively. `kongctl`
will plan the entire input configuration, show you the expected changes
and prompt for confirmation to apply them:

```shell label="Run this..."
kongctl apply -f . --recursive
```

`kongctl` resolves both `!ref platform-aigw#id` values in the
engineering and product configurations because the Platform
declaration is part of the same configuration inputs.

> _Note:_ The `--recursive` flag tries to load _every_ YAML file
> in the given path recursively. This could include files not related
> to `kongctl` at all, like specs or documentaiton files. `--recursive`
> is useful in this example, but it's likely better practice to use
> more explicit repeated `-f <path-to-file>` flags.

You should see the following after confirming the `apply` command:

```text label="apply output"
...
Executing changes:
[1/4] [namespace: federated-aigw] Creating ai_gateway: platform-aigw... ✓
[2/4] [namespace: federated-aigw] Creating ai_gateway_model_provider: platform-openai... ✓
[3/4] [namespace: federated-aigw] Creating ai_gateway_model: engineering-assistant... ✓
[4/4] [namespace: federated-aigw] Creating ai_gateway_model: product-assistant... ✓

Complete.
Executed 4 changes.
```
