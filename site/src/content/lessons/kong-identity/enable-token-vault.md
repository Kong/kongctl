---
title: Enable Token Vault
summary: Understand directory ownership and enable Vault before adding its children.
order: 3
---

## Goal

Enable Token Vault on the existing `default` directory. This changes the
ownership boundary: Vault children have no `labels` or `managed_by` field, so
`kongctl` requires a managed parent directory for their declarative writes.

Coordinate this step with the directory administrator. Use a development
organization where you can take responsibility for the directory's Vault
configuration.

## Inspect ownership before adopting

```shell
kongctl get id directory default -o json
```

If `managed_by` has no ownership metadata, adopt the directory explicitly:

```shell
kongctl adopt id directory default --namespace identity-guide-directory
```

Adoption adds ownership metadata to the existing directory; it does not create
a new directory. If it is already managed, skip adoption and use its existing
`kongctl-namespace` in **both** Vault manifests in this chapter, replacing
`identity-guide-directory`. Do not overwrite another team's ownership. If the
directory is protected, coordinate a separate protection-only change with its
owner before proceeding.

## Enable Vault separately

Create this small directory configuration:

```shell label="Create directory.yaml"
cat > directory.yaml <<'YAML'
_defaults:
  kongctl:
    namespace: identity-guide-directory

identity:
  directories:
    - ref: default-directory
      name: default
      vault:
        enabled: true
YAML
```

For an existing directory, omitted settings such as TTLs and allowed control
planes are preserved. If the directory was absent in the first lesson, an
administrator can use this file to create it; review the API defaults and
access settings before approving creation, then return to the principal
lesson.

```shell
kongctl apply -f directory.yaml
kongctl get id directory default -o json
```

The response should show `vault_enabled: true`. YAML groups this setting under
`vault.enabled`; the API still returns the flat field `vault_enabled`.

Enable an existing disabled Vault before planning its children. Its API hides
child collections while it is disabled, so combining enablement with child
creation would prevent a reliable inventory.

## Understand the boundary

The next lesson adds a public key, provider, and shared credential. These
inherit the directory's namespace and protection. They cannot have separate
team ownership metadata. Keys are matched by their public-key fingerprint
(`kid`), providers by their unique directory-scoped name, and the example
credential by its default slot.

Use `apply` for this walkthrough. It preserves undeclared children. In a
managed directory, `sync` can remove resources from explicitly declared Vault
collections, including entries created by other administrative tools. Omitted
collections are preserved; an explicit empty collection requests pruning.

**Do not run `kongctl delete -f directory.yaml` or delete the next lesson's
Vault manifest as a shortcut for cleanup.** Both declare the directory itself,
so deletion selects the shared `default` directory too.
