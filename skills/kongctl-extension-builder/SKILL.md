---
name: kongctl-extension-builder
description: Scaffold and maintain kongctl CLI extensions. Use when a user
  wants to create a script or Go extension, define kongctl-extension.yaml command
  paths, test install/link workflows, or debug extension context handling.
license: Apache-2.0
metadata:
  product: kongctl
  category: extensions
---

# kongctl extension builder

## Goal

Help users create a runnable `kongctl` CLI extension with a valid
`kongctl-extension.yaml`, an executable runtime, and local and remote test
workflows.

## Core Rules

- Extensions run as separate processes; do not import `internal/...` packages
  from `kongctl`.
- The manifest file must be named `kongctl-extension.yaml`.
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
- `version` in `kongctl-extension.yaml` is optional. For remotely installed
  extensions, prefer Git tags/releases as the package version source; kongctl
  displays the release tag, ref, or short commit when the manifest omits a
  version.
- GitHub repository names do not need a `kongctl-*` prefix. Use names that are
  clear for humans and keep the manifest ID in `publisher/name` form.
- Extension-specific args and flags should be shown directly in examples, such
  as `kongctl get foo --limit 10`. Use `--` only as an escape hatch when an
  extension needs to receive a token that collides with a host `kongctl` flag,
  such as `--output`, `--profile`, `--help`, or their shorthands.
- Put extension diagnostics on stderr when stdout needs to preserve structured
  output from a reentrant `kongctl` command.

## Workflow

1. Choose a runtime style:
   - shell script for small wrappers
   - Go binary for richer parsing or reusable logic
2. Create `kongctl-extension.yaml` with one or more command paths.
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
7. Prepare remote repositories so install can consume release archives:
   - release archives are preferred over source clone fallback
   - `kongctl-extension.yaml` must be at the archive root
   - `runtime.command` points to a runnable file inside the archive
   - publish one archive asset per release, or include the target platform in
     each platform-specific asset name
   - use `.tar.gz` for Go binary releases because it preserves executable bits
     reliably
   ```sh
   kongctl install extension <owner>/<repo>
   kongctl install extension <owner>/<repo> --ref <branch-or-tag>
   kongctl install extension <owner>/<repo>@<tag>
   kongctl install extension <owner>/<repo>@<tag> --yes
   kongctl upgrade extension <publisher>/<name>
   kongctl upgrade extension <owner>/<repo>
   kongctl upgrade extension <publisher>/<name>@<tag-or-version> --yes
   ```

When no compatible release archive exists, `kongctl install extension
<owner>/<repo>` falls back to cloning the repository. Source fallback is only
valid when the repository root already contains `kongctl-extension.yaml` and an
already-runnable script or binary referenced by `runtime.command`.

For release-artifact installs, `kongctl upgrade extension <publisher>/<name>`
or `kongctl upgrade extension <owner>/<repo>` selects the latest compatible
GitHub release asset from the originally recorded repository. Add
`@<tag-or-version>` to pin the upgrade target, for example
`kongctl upgrade extension kong/debug@0.2.0` or
`kongctl upgrade extension kong/kongctl-ext-debug@0.2.0`. A bare semantic
version tries the exact release tag first and then a `v`-prefixed tag.
Source-clone installs require an explicit `@<tag|ref|commit>` target because
there is no stable "latest" release asset to resolve.

Remote installs show a trust confirmation prompt with the selected source,
asset or ref, runtime command, command paths, and package/manifest/runtime
hashes. Use `--yes` for automated tests or release workflow verification.

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

## Release Artifact Workflow

For GitHub installs, prefer a release artifact that extracts to this shape:

```text
kongctl-extension.yaml
bin/kongctl-ext-foo
README.md
```

With:

```yaml
runtime:
  command: bin/kongctl-ext-foo
```

For a Go extension, add a workflow like this and adjust the binary package path
and artifact names for the repository:

```yaml
name: release

on:
  push:
    tags:
      - "v*"

permissions:
  contents: write

jobs:
  release:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        include:
          - goos: linux
            goarch: amd64
          - goos: darwin
            goarch: arm64
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - run: mkdir -p dist/package/bin
      - run: |
          CGO_ENABLED=0 GOOS=${{ matrix.goos }} GOARCH=${{ matrix.goarch }} \
            go build -o dist/package/bin/kongctl-ext-foo ./cmd/kongctl-ext-foo
          cp kongctl-extension.yaml README.md dist/package/
          chmod +x dist/package/bin/kongctl-ext-foo
          tar -C dist/package -czf \
            dist/kongctl-ext-foo-${{ matrix.goos }}-${{ matrix.goarch }}.tar.gz \
            kongctl-extension.yaml README.md bin/kongctl-ext-foo
      - uses: softprops/action-gh-release@v2
        with:
          files: dist/*.tar.gz
```

For a script extension, publish a single universal archive:

```sh
mkdir -p dist/package/bin
cp kongctl-extension.yaml README.md dist/package/
cp kongctl-ext-foo dist/package/bin/
chmod +x dist/package/bin/kongctl-ext-foo
tar -C dist/package -czf dist/kongctl-ext-foo-universal.tar.gz \
  kongctl-extension.yaml README.md bin/kongctl-ext-foo
```

The installer accepts `.tar.gz`, `.tgz`, and `.zip` assets. If a release has
more than one archive asset, platform-specific assets should include the current
`GOOS` and `GOARCH` in the name, such as
`kongctl-ext-foo-linux-amd64.tar.gz`.

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
