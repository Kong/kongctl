---
title: Independent Team Changes
summary: Add a child resource without changing the Platform configuration.
order: 8
related:
  - label: Expanded federated AI Gateway example
    url: https://github.com/Kong/kongctl/tree/main/docs/examples/declarative/ai-gateway/federated
---

## Goal

You will add a second Engineering model to the Platform-owned AI Gateway using
only the Engineering configuration.

## Add an Engineering model

Append another model to `engineering/model.yaml`. It independently looks up
the shared gateway and uses the provider managed by the Platform team:

```shell rows=8 label="Add an Engineering model"
cat >> engineering/model.yaml <<'YAML'

  - ref: engineering-code-reviewer
    ai_gateway: !lookup {name: platform-aigw}
    name: engineering-code-reviewer
    display_name: "Engineering Code Reviewer"
    type: model
    labels:
      team: engineering
    formats:
      - type: openai
    config:
      route:
        paths:
          - /engineering/code-review
        model:
          body_param: model
          values:
            - engineering-code-reviewer
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

The `/engineering/code-review` path gives the new model a distinct route from
the general `/engineering` assistant.

## Preview and apply the change

Preview only the Engineering configuration:

```shell label="Run this..."
kongctl diff --mode apply -f engineering/model.yaml
```

The plan should contain one `CREATE` for `engineering-code-reviewer`. Apply it:

```shell label="Run this..."
kongctl apply -f engineering/model.yaml
```

The apply creates the new model beneath the existing AI Gateway. It does not
recreate the gateway or its Platform-owned provider.

List the models attached to the shared gateway:

```shell label="Run this..."
kongctl get ai-gateway models \
  --gateway-name "Platform AI Gateway" \
  -o text
```

The output should now include `Engineering Assistant`, `Engineering Code
Reviewer`, and `Product Assistant`. The Platform configuration was neither
loaded nor changed; Engineering added a child resource through its own file.

## Important: `sync` scope

The `apply` commands is create and update only. If both teams manage models
in the same parent collection, running `sync` with only one team's file
could plan deletion of the other team's models. Run `sync` only from a
configuration that intentionally describes the complete desired collection.
