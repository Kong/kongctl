---
title: Relationship Lookups
summary: Resolve an existing resource directly inside a relationship.
order: 5
related:
  - label: Declarative configuration documentation
    url: https://developer.konghq.com/kongctl/declarative/
---

## Goal

You will use `!lookup` when a relationship needs an existing resource but no
reusable external declaration.

## Look up the relationship target

`!lookup` selects the existing resource directly in the relationship field:

```yaml
ai_gateway_models:
  - ref: peer-support-assistant
    ai_gateway: !lookup { name: shared-ai-gateway }
    name: peer-support-assistant
```

The relationship field tells `kongctl` which resource type to query. The
selector must match exactly one resource.

Use `!lookup` for a direct relationship. Use `_external` when you need a local
`ref` that can be reused or an external parent with managed children.
`!external` is an alias for `!lookup`.

Lookups are resolved while planning with the active profile. A saved plan
contains the resolved Konnect ID rather than the lookup tag.
