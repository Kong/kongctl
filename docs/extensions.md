# kongctl CLI Extension Developer Guide

`kongctl` extensions are external executable programs that contribute command
paths to the `kongctl` command tree. They are installed, linked, upgraded, and
removed by `kongctl`, but they run as separate child processes.

Extensions are executable code. Install and run only extensions whose source
and release artifacts you trust.

## Extension Shape

An extension directory or release archive must contain:

```text
kongctl-extension.yaml
bin/kongctl-ext-foo
README.md
```

The manifest must be named `kongctl-extension.yaml`. The executable can live
anywhere inside the extension root, but `runtime.command` must point to it with
a relative path.

Minimal manifest:

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

Required fields:

- `schema_version: 1`
- `publisher`
- `name`
- `runtime.command`
- at least one `command_paths[].path`

`version` is optional. For GitHub release installs, prefer release tags as the
package version source. When the manifest omits `version`, `kongctl` displays
the release tag, source ref, or short commit where available.

## Compatibility

Extensions can declare the range of `kongctl` versions they support:

```yaml
compatibility:
  min_version: 0.20.0
  max_version: 0.x
```

`kongctl` enforces this range when installing, linking, upgrading, and running
extensions. If the current `kongctl` version is outside the declared range, the
command fails before installing the package or starting the extension process.

`min_version` is inclusive. `max_version` is inclusive for exact versions and
can use a wildcard such as `0.x` to express a supported major-version lane.

Development builds report their version as `dev`. Because that is not a
released semantic version, compatibility is shown as unknown and execution is
allowed. Release builds use their actual version for enforcement.

## Command Paths

For v1, an extension can contribute command paths under:

- `get`
- `list`
- a custom root verb that does not collide with a built-in command

Built-in root segments such as `get` and `list` cannot define aliases. Child
segments and custom root verbs may define aliases when they do not collide with
existing commands or other extensions.

Examples:

```yaml
command_paths:
  - path:
      - name: get
      - name: debug-info
  - path:
      - name: print-debug-info
```

These paths produce:

```sh
kongctl get debug-info
kongctl print-debug-info
```

## Executable Rules

`kongctl` does not compile extension source during install. The file referenced
by `runtime.command` must already exist and be runnable.

Rules:

- use a relative path such as `bin/kongctl-ext-foo`
- do not use absolute paths
- do not use `..`
- keep the executable inside the extension root
- on Unix-like systems, make the executable bit part of the package

For Go extensions, build the binary before linking, installing, or packaging.
For script extensions, mark the script executable.

```sh
chmod +x bin/kongctl-ext-foo
```

## Arguments And Flags

Extension-specific arguments and flags are normally passed directly:

```sh
kongctl get foo --limit 10
```

Use `--` only as an escape hatch when an extension needs to receive a token
that collides with a host `kongctl` flag, such as `--output`, `--profile`, or
`--help`.

```sh
kongctl get foo -- --output raw
```

Inside the extension, read `remaining_args` from the runtime context, or parse
the process arguments passed to the executable.

## Runtime Context

When `kongctl` runs an extension, it writes a `context.json` file and sets:

```sh
KONGCTL_EXTENSION_CONTEXT=/path/to/context.json
```

The context contains:

- matched command path
- original and remaining arguments
- selected profile
- resolved Konnect base URL
- output, jq, color theme, and log settings
- extension data directory
- host `kongctl` path and version
- recursion guard session data

The context file does not contain secrets. Script extensions that need
authenticated Konnect access should invoke `kongctl` as a subprocess, usually
through `kongctl api` or another built-in command. Go extensions can use
`github.com/kong/kongctl/pkg/sdk` to create an authenticated `sdk-konnect-go`
client from the same runtime context. Child `kongctl` commands inherit the
parent extension context.

Keep stdout clean when the parent command is expected to emit structured
output. Send diagnostics to stderr.

## Go Extension SDK

Go extensions can import the kongctl extension SDK:

```go
import "github.com/kong/kongctl/pkg/sdk"
```

The package loads the runtime context, creates a configured Konnect SDK client,
runs reentrant `kongctl` commands, and renders output with the parent command's
`--output`, `--jq`, `--jq-raw-output`, and jq color settings.

Small example:

```go
runtimeCtx, err := sdk.LoadRuntimeContextFromEnv()
if err != nil {
    return err
}

konnect, err := runtimeCtx.KonnectSDK(context.Background())
if err != nil {
    return err
}

res, err := konnect.Me.GetUsersMe(context.Background())
if err != nil {
    return err
}

return runtimeCtx.Output().Render(displayUser(res.GetUser()), res.GetUser())
```

During local development against a kongctl checkout, add a temporary replace
directive in the extension's `go.mod`:

```go
replace github.com/kong/kongctl => /path/to/kongctl
```

Remove the replace directive before publishing, and pin a released kongctl
module version once the SDK is available from a release tag.

## Security Model

Extensions are not isolated sandboxes. An extension runs with the same operating
system permissions as the user running `kongctl`.

When an extension invokes `kongctl` as a child process, the child command uses
the effective parent invocation context. That means an extension can make
Konnect requests with the same profile, token source, and permissions available
to the parent command.

Do not put secrets in `context.json`, command metadata, examples, release
assets, or logs. Treat installed extension packages like any other executable
software dependency.

## Local Development

Use `link` while developing an extension. Linked extensions run from the working
tree, so edits are visible immediately.

```sh
kongctl link extension ./my-extension
kongctl get extension kong/foo
kongctl list extensions
kongctl get foo --help
kongctl get foo
```

Use managed local install when you want to test copying into kongctl's extension
home.

```sh
kongctl install extension ./my-extension
kongctl list extensions
kongctl uninstall extension kong/foo
```

Linked extensions and local path installs are not upgraded. Re-link or reinstall
them from the local source path.

## GitHub Installs

GitHub repositories do not need a `kongctl-*` prefix. Use a clear repository
name, and use the manifest `publisher/name` as the extension identity.

Install from a repository:

```sh
kongctl install extension owner/repo
kongctl install extension owner/repo@v0.1.0
kongctl install extension owner/repo@0.1.0
```

`kongctl` first tries to install a compatible GitHub release archive. If no
compatible release archive exists, it falls back to cloning the repository only
when the repository root contains `kongctl-extension.yaml` and an already
runnable executable referenced by `runtime.command`.

Release artifact installs fetch GitHub release metadata and assets over HTTPS.
Source fallback uses `git clone` over HTTPS.

For the most predictable install experience, publish public release artifacts.
Private repositories may work through source-clone fallback when local Git
credentials are configured, but release artifact discovery uses GitHub HTTP
metadata and should be treated as public-repository oriented for v1.

Remote installs show a trust warning with the selected source, extension name,
release or source ref, asset, executable, command paths, and package hashes.
Use `--yes` only in automation or repeatable test environments.

## Release Archives

Prefer release archives for public extensions. The archive root must contain
`kongctl-extension.yaml`; it should not wrap the package in an extra top-level
directory.

Good archive layout:

```text
kongctl-extension.yaml
bin/kongctl-ext-foo
README.md
```

For universal script extensions:

```sh
mkdir -p dist/package/bin
cp kongctl-extension.yaml README.md dist/package/
cp kongctl-ext-foo dist/package/bin/
chmod +x dist/package/bin/kongctl-ext-foo
tar -C dist/package -czf dist/kongctl-ext-foo-universal.tar.gz \
  kongctl-extension.yaml README.md bin/kongctl-ext-foo
```

For Go extensions, publish platform-specific archives. Include the target
`GOOS` and `GOARCH` in the asset name.

```text
kongctl-ext-foo-linux-amd64.tar.gz
kongctl-ext-foo-darwin-arm64.tar.gz
kongctl-ext-foo-windows-amd64.zip
```

Example GitHub Actions release workflow:

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
          archive="dist/kongctl-ext-foo-${{ matrix.goos }}-"
          archive="${archive}${{ matrix.goarch }}.tar.gz"
          tar -C dist/package -czf "$archive" \
            kongctl-extension.yaml README.md bin/kongctl-ext-foo
      - uses: softprops/action-gh-release@v2
        with:
          files: dist/*.tar.gz
```

## Upgrade

Upgrade one extension by manifest identity or by the GitHub repository recorded
at install time:

```sh
kongctl upgrade extension kong/foo
kongctl upgrade extension owner/repo
kongctl upgrade extension kong/foo@v0.2.0
kongctl upgrade extension owner/repo@0.2.0
```

Upgrade all installed extensions:

```sh
kongctl upgrade extension
kongctl upgrade extensions
```

Batch upgrade processes installed GitHub release-asset extensions. It skips
linked extensions, local path installs, and GitHub source-clone installs that
need an explicit `@tag|ref|commit` target. It continues after individual
failures and exits non-zero if any upgrade failed.

## Safe Example Ideas

Prefer read-only examples for public educational repositories.

- print extension context and remaining args
- call `kongctl get me` and preserve the parent output format
- use `pkg/sdk` to call `sdk-konnect-go` and preserve output settings
- call `kongctl api get /v2/control-planes`
- summarize APIs, portals, or control planes
- wrap team-specific read-only reporting workflows

See `docs/examples/extensions/script` and `docs/examples/extensions/go` for
small script and Go examples.

## Release Checklist

Before publishing an extension repository:

- run the executable directly
- link the extension locally
- run every contributed command path
- verify `kongctl get extension <publisher>/<name>`
- install from the local path
- publish a tagged GitHub release archive
- install from `owner/repo@tag`
- upgrade from an older tag
- run `kongctl upgrade extensions --yes` in a test profile
- check that stdout remains structured when `--output json` is used
