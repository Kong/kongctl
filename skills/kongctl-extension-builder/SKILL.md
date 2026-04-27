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
`extension.yaml`, an executable runtime, and local and remote test workflows.

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
- Put extension diagnostics on stderr when stdout needs to preserve structured
  output from a reentrant `kongctl` command.

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
7. Prepare remote repositories so install can consume already-runnable files:
   - script repos commit the executable runtime directly
   - Go repos must include an already-built runtime until release artifacts are
     supported
   - `extension.yaml` lives at the repository root
   - `runtime.command` points to a file inside the repository
   ```sh
   kongctl install extension <owner>/<repo>
   kongctl install extension <owner>/<repo> --ref <branch-or-tag>
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

## Script Runtime Pattern

Use shell for lightweight wrappers and debugging helpers.

```sh
#!/bin/sh
set -eu

echo "context_path=${KONGCTL_EXTENSION_CONTEXT:-}" >&2

kongctl_bin="kongctl"
if [ -n "${KONGCTL_EXTENSION_CONTEXT:-}" ] &&
  [ -r "$KONGCTL_EXTENSION_CONTEXT" ]; then
  kongctl_bin=$(sed -n \
    's/^[[:space:]]*"kongctl_path"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' \
    "$KONGCTL_EXTENSION_CONTEXT")
fi

"$kongctl_bin" api get /v2/control-planes
```

## Go Runtime Pattern

Use Go when argument parsing, JSON handling, or richer errors matter. Keep the
runtime independent from `kongctl/internal/...`; read `context.json` into a
local struct and call `kongctl` as a subprocess for authenticated host access.

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

## Good Educational Extension Ideas

- Debug context: print `context.json`, remaining args, and reentrant command
  behavior.
- Who am I: call `kongctl get me` and render the current user in the parent
  output format.
- Control plane summary: call `kongctl api` to list control planes and print a
  compact operational summary.
- Portal/API report: combine a few `kongctl api` calls into a read-only report
  that would be awkward as a one-liner.
- Declarative helper: wrap existing `kongctl plan`, `diff`, or `apply` command
  sequences for a team-specific workflow.

Prefer read-only examples for public educational repositories. They are easier
to try safely and demonstrate auth/context reuse without destructive setup.

## Examples

- Script extension: `docs/examples/extensions/script`
- Go extension: `docs/examples/extensions/go`

The Go example must be built before linking because install and link expect
`runtime.command` to point to an existing executable.
