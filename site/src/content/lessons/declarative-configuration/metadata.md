---
title: Metadata
summary: Preserve management scope and protection on Konnect resources.
order: 2
related:
  - label: Declarative configuration documentation
    url: https://developer.konghq.com/kongctl/declarative/
---

## Goal

You will understand what `kongctl` _metadata_ is, how it changes declarative
configuration behavior, and how it is stored on Konnect resources.

## Resource Metadata

Currently the following metadata items can be specified:

- `namespace: <string>`: Groups declaratively managed resources into a
  reconciliation scope, determining which resources `kongctl` considers
  together when planning changes.
- `protected: <bool>`: Prevents `kongctl` from deleting a resource through
  declarative operations until protection is removed.

The `kongctl` block allows you to specify the metadata for a
given resource:

```yaml label="kongctl metadata"
ai_gateways:
  - ref: my-aigw
    name: my-aigw
    kongctl:
      namespace: aigw-learning
      protected: true
```

## File-level metadata defaults

You don't have to specify metadata on every resource. _File_ level defaults
can be specified in the root `_defaults` key. This applies the metadata
specified to all resources _in the file_:

```yaml
_defaults:
  kongctl:
    namespace: aigw-learning
    protected: false
```

> _Note:_ Metadata on an individual resource overrides values specified in
> the file-level `_defaults`.

### Metadata storage

`kongctl` stores resource metadata in Konnect labels.

| Configuration metadata key | Konnect label name  |
| -------------------------- | ------------------- |
| `namespace`                | `KONGCTL-namespace` |
| `protected: true`          | `KONGCTL-protected` |

Konnect _generally_ only supports labels on root-level resources.

## Scope resources with namespaces

Each managed parent resource belongs to one namespace. When generating a
plan, `kongctl` considers managed resources in the input configuration's
namespaces and ignores resources in other namespaces.

For the example above, the AI Gateway is stored with:

```yaml
labels:
  KONGCTL-namespace: aigw-learning
  KONGCTL-protected: "true"
```

This label preserves the namespace on the remote resource so a later plan can
identify its management scope. The presence of `KONGCTL-namespace` is also how
the current implementation recognizes a declaratively managed resource.
Resources use the `default` namespace when no namespace is specified.

Namespaces are reconciliation boundaries, not access controls. Konnect
authorization determines what a user can change.

## Protect important resources

Set `protected: true` on a parent resource to prevent `kongctl` from deleting
it accidentally. Protected resources carry the `KONGCTL-protected: "true"`
label. Set `protected: false` and apply the change before a declarative
operation can delete that resource.

For stricter automation, planning flags such as
`--require-namespace=aigw-learning` can reject configurations outside an
expected namespace.
