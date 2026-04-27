---
name: kongctl-extension-builder
description: Scaffold and maintain kongctl CLI extensions. Use when a user
  wants to create a script or Go extension, define extension.yaml command
  paths, test install/link workflows, or debug extension context handling.
license: Apache-2.0
metadata:
  product: kongctl
  category: extensions
---

# kongctl extension builder

## Goal

Help users create a runnable `kongctl` CLI extension with a valid
`extension.yaml`, an executable runtime, and a local test workflow.

## Core Rules

- Extensions run as separate processes; do not import `internal/...` packages
  from `kongctl`.
- The manifest file must be named `extension.yaml`.
- Required manifest fields are:
  - `schema_version: 1`
  - `publisher`
  - `name`
  - `runtime.command`
  - at least one `command_paths[].path`
- Extension IDs use `publisher/name`, and both segments should be lowercase
  path-safe identifiers.
- v1 command paths may contribute below `get` or `list`, or define a custom
  root verb that does not collide with built-ins.
- Built-in root segments such as `get` and `list` cannot declare aliases.
- `runtime.command` must be relative to the extension root and already
  executable. `kongctl` does not compile source during install.
- Extension-specific flags are parsed by the extension runtime. Use `--` when
  a user needs to pass a token that looks like a host flag.

## Workflow

1. Choose a runtime style:
   - shell script for small wrappers
   - Go binary for richer parsing or reusable logic
2. Create `extension.yaml` with one or more command paths.
3. Add the runtime file named by `runtime.command`.
4. Make script runtimes executable with `chmod +x`.
5. Test with a development link:
   ```sh
   kongctl link extension <extension-dir>
   kongctl get extension <publisher>/<name>
   kongctl get <resource>
   ```
6. Use local install when testing managed package copying:
   ```sh
   kongctl install extension <extension-dir>
   kongctl list extensions
   kongctl uninstall extension <publisher>/<name>
   ```

## Minimal Manifest

```yaml
schema_version: 1

publisher: kong
name: foo

runtime:
  command: kongctl-ext-foo

command_paths:
  - path:
      - name: get
      - name: foo
        aliases: [foos]
```

## Runtime Context

When an extension runs, `kongctl` sets `KONGCTL_EXTENSION_CONTEXT` to a
generated `context.json` file. Read this file for:

- matched command path
- remaining extension arguments
- selected profile
- resolved base URL
- output and log settings
- extension data directory
- host `kongctl` path and version

Never expect secrets in `context.json`. Extensions can invoke `kongctl api`
or other built-in commands as subprocesses when they need host-authenticated
Konnect calls. Child `kongctl` commands inherit the parent extension context
unless they explicitly override flags such as `--profile`, `--output`, or
`--base-url`.

## Examples

- Script extension: `docs/examples/extensions/script`
- Go extension: `docs/examples/extensions/go`

The Go example must be built before linking because install and link expect
`runtime.command` to point to an existing executable.
