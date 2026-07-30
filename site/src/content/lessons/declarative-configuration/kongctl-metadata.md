---
title: kongctl Metadata
summary: Scope managed resources and protect them from accidental deletion.
order: 3
related:
  - label: Declarative configuration documentation
    url: https://developer.konghq.com/kongctl/declarative/
---

## Goal

You will understand how namespaces and resource protection affect
declarative plans.

## Scope resources with namespaces

Each managed parent resource belongs to one `kongctl` namespace. Plans consider
managed resources in the selected namespaces and ignore resources in other
namespaces. Resources use the `default` namespace when none is specified.

Set metadata defaults for a file:

```yaml
_defaults:
  kongctl:
    namespace: aigw-learning
    protected: false
```

Namespaces are reconciliation boundaries, not access controls. Konnect
authorization determines what a user can change.

## Protect important resources

Set `protected: true` on a parent resource to prevent `kongctl` from deleting
it accidentally:

```yaml
ai_gateways:
  - ref: production-aigw
    name: production-aigw
    kongctl:
      namespace: production
      protected: true
```

Resource-level metadata overrides `_defaults`. Protection must be removed
before a declarative operation can delete the resource.

For stricter automation, planning flags such as
`--require-namespace=aigw-learning` can reject configurations outside an
expected namespace.
