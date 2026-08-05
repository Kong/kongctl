---
title: Shared Declarative Loads
summary: Connect team resources when their files are planned together.
order: 3
related:
  - label: Federated AI Gateway example
    url: https://github.com/Kong/kongctl/tree/main/docs/examples/declarative/ai-gateway/federated
---

## Goal

You will connect resources owned by different teams in one declarative load.

## Reference the shared resource

The central configuration gives the shared gateway a local `ref`:

```yaml
ai_gateways:
  - ref: shared-ai-gateway
    name: shared-ai-gateway
    display_name: Shared Federated AI Gateway
```

A peer-owned model can refer to that gateway:

```yaml
ai_gateway_models:
  - ref: peer-support-assistant
    ai_gateway: !ref shared-ai-gateway#id
    name: peer-support-assistant
```

`!ref` works here because `kongctl` loads both declarations before resolving
the relationship.

## Load the directories together

Preview the central and peer configurations as one recursive load:

```shell
kongctl plan \
  -f docs/examples/declarative/ai-gateway/federated \
  --recursive \
  --mode apply
```

This model fits a workflow where separate team files are assembled into one
reviewed plan.
