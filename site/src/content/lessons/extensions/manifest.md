---
title: Extension Manifest
summary: Define an extension identity, executable, and command paths.
order: 2
related:
  - label: Complete extension developer guide
    url: https://github.com/Kong/kongctl/blob/main/docs/extensions.md
---

## Goal

You will recognize the required extension files and read a minimal manifest.

## Package shape

An extension directory or release archive contains:

- `kongctl-extension.yaml`
- a runnable executable, such as `bin/kongctl-ext-foo`
- a `README.md`

The manifest must be named `kongctl-extension.yaml`. Its `runtime.command`
points to the executable using a path relative to the extension root.

## Read a minimal manifest

```yaml
schema_version: 1

publisher: kong
name: foo

runtime:
  command: bin/kongctl-ext-foo

command_paths:
  - id: get_foo
    path:
      - name: get
      - name: foo
        aliases: [foos]
    summary: Get Foo resources
```

This manifest contributes `kongctl get foo` and the `foos` alias. Version 1
extensions can contribute paths under `get`, `list`, or a custom root verb
that does not collide with a built-in command.

## Declare persistent flags

Use `persistent_flags` when an override must work before or after a child
command. Existing `flags` entries provide help metadata only.

```yaml
command_paths:
  - path: [{ name: ai }]
    persistent_flags:
      - name: context
        type: string
        description: Override the context for this invocation
      - name: verbose
        type: bool
        description: Enable verbose output
  - path: [{ name: ai }, { name: status }]
```

Both `kongctl ai --context example status` and
`kongctl ai status --context example` forward `--context example`. Parent
and descendant help show the inherited declarations.

Types must be `string` or `bool`. Strings consume their next token or accept
`--context=example`. Booleans use `--verbose` or `--verbose=false`; explicit
boolean values require `=`. Repeated flags retain their original order and
syntax. The extension decides which value wins.

Inheritance stays inside the owning extension subtree. Under `get` and
`list`, declare flags on an extension child, never the shared built-in root.
Host flag collisions, duplicates, and descendant redeclarations are errors.

## Declare compatibility

An extension can restrict the supported `kongctl` versions:

```yaml
compatibility:
  min_version: 0.20.0
  max_version: 0.x
```

`kongctl` checks this range when installing, linking, upgrading, and running
the extension.

Persistent flags use schema version 1, but require a host that implements
this feature. Older hosts reject the unknown manifest field. Set the minimum
version to a released host version that supports it and that you have tested.
