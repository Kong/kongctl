---
title: Federate Product Team
summary: Make the Product configuration independently deployable.
order: 7
related:
  - label: YAML tags lesson
    url: https://kong.github.io/kongctl/declarative-configuration/yaml-tags/
---

## Goal

You will give Product the same independent workflow while it continues to use
the Platform team's AI Gateway.

## Replace the local reference

Open `product/model.yaml` in your editor and find its local relationship:

```yaml label="Find this in product/model.yaml"
ai_gateway_models:
  - ref: product-assistant
    ai_gateway: !ref platform-aigw#id
    type: model
```

Change the relationship to look up the existing Platform AI Gateway:

```yaml label="Change it to this"
ai_gateway_models:
  - ref: product-assistant
    ai_gateway: !lookup { name: platform-aigw }
    type: model
```

The `ai_gateway` field tells `kongctl` which resource type to query. The
selector must match exactly one AI Gateway.

## Apply Product independently

Apply only the Product configuration:

```shell label="Run this..."
kongctl apply -f product/model.yaml
```

This apply should also be a no-op:

```text label="Example output"
No changes needed. Resources match configuration.
```

Engineering and Product can now plan and apply their files independently. The
Platform configuration is no longer required as an input, but the Platform AI
Gateway must already exist for each lookup to resolve.
