---
title: Sync Scope
summary: Control which collections sync reconciles, including desired zero.
order: 6
related:
  - label: Declarative configuration documentation
    url: https://developer.konghq.com/kongctl/declarative/
---

## Goal

You will understand how YAML key presence, namespaces, and parent identity
define what a sync-mode plan can delete.

## Scope makes sync safe

Unlike apply mode, sync mode can delete managed resources that are missing
from the desired state. `kongctl` limits that behavior to collections
explicitly present in the input.

| Input state                    | Sync behavior                             |
| ------------------------------ | ----------------------------------------- |
| `ai_gateways` is omitted       | Ignore the AI Gateway collection.         |
| `ai_gateways` contains entries | Reconcile those entries as desired state. |
| `ai_gateways: []`              | Set the desired AI Gateway count to zero. |

An omitted collection and an empty collection are intentionally different.

## Give zero resources a namespace

An empty collection has no resource entry carrying `kongctl` metadata. Use
`_defaults` to identify the namespace where the desired count is zero:

```yaml
_defaults:
  kongctl:
    namespace: aigw-learning

ai_gateways: []
```

In sync mode, this configuration plans deletion of every managed AI Gateway
in the `aigw-learning` namespace. AI Gateways in other namespaces are outside
the plan's scope. Without an explicit namespace, the `default` namespace is
used.

## Give an empty child its parent identity

Parent and child collections have separate sync scope. Omitting `models`
leaves existing models alone:

```yaml
ai_gateways:
  - ref: my-aigw
    name: my-aigw
```

An explicit empty list declares that this AI Gateway should have no models:

```yaml
ai_gateways:
  - ref: my-aigw
    name: my-aigw
    models: []
```

The empty list must be nested so `kongctl` knows which AI Gateway owns the
collection. A root-level `ai_gateway_models: []` is rejected because no model
entry provides an `ai_gateway` parent identity.

## Preview desired zero

Create a separate sync input for the learning namespace:

```shell
cat > empty-ai-gateways.yaml <<'YAML'
_defaults:
  kongctl:
    namespace: aigw-learning

ai_gateways: []
YAML
```

Generate and inspect a sync-mode plan:

```shell
kongctl plan \
  --mode sync \
  -f empty-ai-gateways.yaml \
  --output-file sync-plan.json
kongctl diff --plan sync-plan.json
```

> **Do not execute this plan unless you intend to delete the learning AI
> Gateway.** Creating and inspecting the plan does not change Konnect.
