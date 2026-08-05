---
title: External Resources
summary: Attach managed resources to a parent owned elsewhere.
order: 4
related:
  - label: Declarative configuration documentation
    url: https://developer.konghq.com/kongctl/declarative/
  - label: Federated AI Gateway example
    url: https://github.com/Kong/kongctl/tree/main/docs/examples/declarative/ai-gateway/federated
---

## Goal

You will use `_external` when a team needs a reusable reference to a resource
that another team manages.

## Declare the external parent

An external resource selects an existing Konnect resource instead of managing
that resource:

```yaml
ai_gateways:
  - ref: external-shared-ai-gateway
    _external:
      selector:
        matchFields:
          display_name: Shared Federated AI Gateway
```

The local `ref` can then connect team-owned resources to the selected gateway:

```yaml
ai_gateway_models:
  - ref: external-peer-support-assistant
    ai_gateway: !ref external-shared-ai-gateway#id
    name: external-peer-support-assistant
```

Use `_external` when a team needs a reusable declarative `ref` or needs to
manage resources beneath an externally owned parent. The external declaration
itself is not managed by `kongctl` and cannot have `kongctl` metadata.

The shared gateway must already exist before the peer team plans this
configuration independently.
