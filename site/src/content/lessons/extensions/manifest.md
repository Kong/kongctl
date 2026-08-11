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

## Declare compatibility

An extension can restrict the supported `kongctl` versions:

```yaml
compatibility:
  min_version: 0.20.0
  max_version: 0.x
```

`kongctl` checks this range when installing, linking, upgrading, and running
the extension.
