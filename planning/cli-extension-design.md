# CLI Extension Design Research Report

Reviewed on 2026-04-21.

Updated on 2026-04-26 after implementation-planning decisions.

## Summary

This document recommends a concrete extension design for `kongctl`, explains
the reasoning for that design, and then records the supporting peer CLI
research and previously considered alternatives.

The document structure follows:

1. summary and design decisions
2. detailed design and defense of the decisions
3. peer research and earlier design explorations

## TL;DR

`kongctl` will add a feature that allows users to install extensions
(plugins) and execute non-built-in functionality. Extensions enable
developers to expose new `kongctl` command paths or expand a limited set of
paths. Extensions should preserve the normal `kongctl <verb> ...` pattern.

For users, this means they can install an extension from a local path or a
GitHub repository and then use the new command path as if it were a native
feature of the CLI. The extension should show up in usage text, follow the
same command grammar as the rest of `kongctl`, and clearly identify its
source. Extensions will be managed with lifecycle commands such as
`kongctl install extension`, `kongctl upgrade extension`,
`kongctl list extensions`, and `kongctl uninstall extension`.

Technically, each extension is a separately executed script or binary
described by a YAML manifest obtained at installation (`extension.yaml`).
The manifest is the v1 source of truth for package metadata, runtime
selection, and command metadata. During execution, the parent `kongctl`
process writes a machine-generated `context.json` file, stores its full path
in `KONGCTL_EXTENSION_CONTEXT`, and launches the extension as a child process.
The child can read the selected profile and resolved non-secret configuration
values from that file.

Extensions can "re-enter" `kongctl` as another child process and it will
reload the same context file. This gives extensions a standard way to invoke
`kongctl api` or other built-in `kongctl` commands while keeping the same
resolved invocation. We will avoid building a Go SDK too early. It is not
required for the first implementation and can be added later if repeated
extension patterns justify it.

## Proposed Design

## Design Goals

The proposed design is driven by the following goals:

1. Preserve the established `kongctl <verb> <product> <resource...>` shape.
2. Allow extension authors, especially Kong contributors, to weave extensions
   into the existing command structure where appropriate.
3. Allow one extension to contribute multiple command paths.
4. Support both script-based and compiled extensions.
5. Reuse existing `kongctl` capabilities such as `kongctl api`, structured
   JSON output, jq filtering, configuration resolution, and auth selection.
6. Avoid exposing `internal/...` packages as the public extension contract.
7. Keep v1 simple enough to ship while preserving a path to richer host
   communication later.

## Design At A Glance

| Area | Recommendation |
|------|----------------|
| End-user grammar | Preserve `kongctl <verb> ...` |
| Contribution type | `command_paths` |
| Built-in precedence | Built-ins always win |
| Open existing verbs in v1 | `get`, `list` |
| Provenance in help | Extension-contributed paths must be visibly labeled |
| Manifest | `extension.yaml` for package, runtime, and command metadata |
| Command metadata | Static command metadata in `extension.yaml` |
| Dispatch integration | Synthetic Cobra commands from cached validated manifests |
| Flag boundary | Host parses global flags; extension parses `remaining_args` |
| Metadata stability | Install-stable in v1; linked extensions refresh aggressively |
| Runtime model | Managed external child process |
| Runtime context transport | `KONGCTL_EXTENSION_CONTEXT` |
| Runtime context file | `context.json` |
| Nested host callbacks | Re-enter `kongctl` as a subprocess |
| v1 Go SDK | Not required |
| Performance gate | Validate subprocess cost before locking implementation |
| Secrets in context | Never include them |
| Install integrity | Store manifest/runtime hashes; verify installed runtime before execution |
| GitHub installs | `gh`-style release artifact first, source fallback for runnable script/binary |
| Install builds | No source compilation during install |
| Storage | Existing kongctl config home (`$XDG_CONFIG_HOME/kongctl` or fallback) |
| Cleanup | Best-effort immediate cleanup plus stale-file reaping |
| Safety | Install-time conflict checks and recursion guard |

## Detailed Design

### 1. Preserve The Verb-First Command Model

The design should not force all extension behavior into:

- `kongctl install extension ...`
- `kongctl run ...`
- `kongctl foo ...`

Those command families may still exist for management and diagnostics, but the
main user-facing extension surface should preserve the existing grammar.

That means extensions should be able to contribute:

- `kongctl get foo`
- `kongctl list foo`

and, when needed:

- `kongctl promote foo`
- `kongctl validate-policy foo`

### 2. Use A Single Command Path Model

An extension should describe its contributed commands as `command_paths` in
`extension.yaml`.

Examples:

- `get foo`
- `list foo`
- `promote foo`
- `get foo bar`

This is simpler than splitting the model into separate `commands` and `verbs`.
Whether a contribution lands under an existing verb or defines a new verb is
determined entirely by the first segment of the command path.

Each command path is represented as an array of segment objects so aliases can
be attached to the segment they alias:

```yaml
command_paths:
  - id: get_foo_bar
    path:
      - name: get
      - name: foo
        aliases: [foos]
      - name: bar
        aliases: [bars]
    summary: Get Foo Bar resources
```

This allows multi-segment child-resource paths such as:

- `kongctl get foo bar`
- `kongctl get foos bar`
- `kongctl get foo bars`
- `kongctl get foos bars`

For built-in root verbs such as `get` and `list`, the root segment is reserved
and cannot declare aliases. For custom root verbs, aliases are allowed if they
do not collide with built-in command names, built-in aliases, reserved names,
or other extension paths.

### 3. Open Only A Narrow Set Of Existing Verbs In v1

The initial extension surface should be intentionally selective.

Recommended v1 policy:

- open existing verbs for command contributions: `get`, `list`
- allow custom verbs

All other existing verbs should be treated as closed to extension in v1 unless
explicitly revisited later. This preserves room for future hooks without
committing to them early.

Because these command paths can appear under built-in verbs, the trust boundary
must stay visible in the UI. `kongctl get --help`, completion output, and
inspection commands should visibly label extension-contributed entries with
their source extension. The command syntax should feel native, but provenance
should never be hidden.

Short example:

```text
$ kongctl get --help

Available Commands:
  api         Get APIs
  services    Get services
  foo         Get Foo resources  [extension: acme-foo]
```

And for inspection:

```text
$ kongctl inspect extension acme-foo

Name: acme-foo
Contributed command paths:
  - get foo
  - list foo
```

### 4. Treat One Extension As A Bundle Of Command Paths

One extension should be able to contribute many command paths. The extension
should be the installation unit, while `extension.yaml` should describe the set
of command paths it owns.

This lets one extension support a full resource family rather than forcing
many small install units.

### 5. Use A YAML Manifest For Package And Command Metadata

The manifest should be a plain `extension.yaml` file, and it should describe
the package metadata, runtime metadata, and command metadata needed for install
validation, command registration, help, completion, inspection, and execution.
In v1, this manifest is the source of truth for command help, usage text, args,
and flags.

Recommended shape:

```yaml
schema_version: 1

name: foo
publisher: kong
version: 0.1.0
summary: Foo resource support for kongctl

runtime:
  command: kongctl-ext-foo

compatibility:
  min_version: 0.20.0
  max_version: 0.x

command_paths:
  - id: get_foo
    path:
      - name: get
      - name: foo
        aliases: [foos]
    summary: Get Foo resources
    description: Retrieves Foo resources from Konnect.
    usage: kongctl get foo [name] [flags]
    examples:
      - kongctl get foo
      - kongctl get foo my-foo --output json
    args:
      - name: name
        required: false
        repeatable: false
        description: Optional Foo resource name.
    flags:
      - name: filter
        type: string
        description: Filter Foo resources by label.
```

In v1, `runtime.command` is needed immediately because it tells `kongctl`
which executable to invoke for normal dispatch. Install source, upgrade
provenance, and trust observations such as manifest and runtime hashes should
be tracked by `kongctl` itself, not required in the manifest.

`runtime.command` is a path relative to the installed extension root. It must
not be absolute, must not contain `..`, and must resolve inside the extension
root. It must point to an already-runnable script or binary; `kongctl` does
not compile extension source during install in v1. `kongctl` still needs
explicit platform resolution rules for Windows wrappers and executable
extensions.

### 6. Use Static Manifest Command Metadata In v1

All command metadata should come from `extension.yaml` in v1.

That includes:

- `command_paths`
- summaries
- descriptions
- usage text
- examples
- args
- flags

The earlier design considered a runtime metadata contract such as
`kongctl-ext-foo __kongctl describe`. That remains a possible future extension
authoring convenience, but it should not be required in v1 because it would
execute extension code during install or link just to discover metadata.

In v1, `kongctl` should validate command metadata directly from
`extension.yaml`, then cache the validated metadata for:

- help
- completion
- `inspect extension`
- conflict checks
- startup command registration

This creates a possible metadata drift risk: a runtime can behave differently
than its manifest declares. `kongctl` should reduce accidental drift by
installing the manifest and runtime together, recording manifest and runtime
hashes, and verifying the installed runtime hash before execution. These hashes
prove package integrity, not behavioral truth. A malicious executable can still
ignore or misrepresent its declared metadata, so extension installation remains
arbitrary local code execution.

#### Command Metadata Stability In v1

In v1, the command metadata should be treated as install-stable metadata.

That means:

- command paths should not change based on the current profile
- command paths should not change based on the connected organization
- flags and help text should remain stable for the installed extension version

Dynamic runtime behavior is still fine, but dynamic command registration is not
part of the v1 design. If an extension needs a different command surface after
an upgrade or local edit, the metadata should be refreshed through install,
upgrade, link, or an explicit refresh path.

Linked extensions are the exception for developer experience. They should
re-read the manifest on each invocation or have very aggressive cache
invalidation so local edits show up immediately. Linked extensions should not
use strict runtime hash verification because local drift is expected during
development.

#### Flag and argument boundary in v1

The host and extension need a strict parsing boundary.

Recommended v1 behavior:

- `kongctl` parses root-level global flags before extension dispatch
- once a command path is matched, the remaining tokens are passed to the
  extension verbatim as `remaining_args`
- extension-specific flags and arg validation belong to the extension, not the
  host
- `--help` for an extension command path should be rendered from the cached
  manifest metadata so help stays fast and consistent

This keeps the host/extension contract simple and avoids trying to make Cobra
authoritatively parse two different flag surfaces at once.

### 7. Discover And Install Extensions Explicitly

The v1 discovery model should be explicit-source installation, not broad
catalog search.

That means users install an extension by naming where it comes from:

1. a local filesystem path
2. a GitHub repository reference such as `owner/repo`

Recommended install behavior:

- `kongctl install extension ./my-extension`
- `kongctl install extension kong/kongctl-ext-foo`

For local path installs:

1. the target path must contain `extension.yaml`
2. `kongctl link extension` should be preferred for local development
3. `kongctl install extension <path>` should copy the extension into the
   managed extension home for normal use

For GitHub repo installs:

1. follow the GitHub CLI model: prefer a compatible release artifact for the
   current platform
2. if no compatible release artifact exists, clone the repository only when
   it contains an `extension.yaml` and an already-runnable root-level script or
   binary referenced by `runtime.command`
3. do not compile extension source during install
4. record the source, selected ref, resolved commit, manifest hash, runtime
   hash, and package hash where available so `upgrade` can repeat the same
   strategy later

For release artifacts, the archive should extract to an extension root that
contains `extension.yaml` and the runtime referenced by `runtime.command`.
Example:

```text
extension.yaml
bin/kongctl-ext-foo
README.md
```

With:

```yaml
runtime:
  command: bin/kongctl-ext-foo
```

Release asset names should follow a strict platform convention:

```text
kongctl-ext-foo_0.1.0_linux_amd64.tar.gz
kongctl-ext-foo_0.1.0_darwin_arm64.tar.gz
kongctl-ext-foo_0.1.0_windows_amd64.zip
```

For source fallback and local path installs, the canonical root-level runtime
name should be `kongctl-ext-<name>`, for example:

```text
extension.yaml
kongctl-ext-foo
```

With:

```yaml
runtime:
  command: kongctl-ext-foo
```

This gives `kongctl` a clear v1 install story:

- explicit local path installs
- explicit GitHub repo installs
- no ambient PATH discovery
- no broad marketplace search requirement in v1

`kongctl` should also store install provenance in its own local extension state
so it can later answer questions such as:

- what source was this extension installed from
- is this extension linked or installed
- what version or ref should `upgrade` compare against

### 8. Store Extensions Under The Existing Kongctl Config Home

Persistent extension files should live under the same kongctl config home used
for config, logs, and process-management state:

- `$XDG_CONFIG_HOME/kongctl`
- or the existing user-home fallback used by `config.GetDefaultConfigPath()`

Recommended v1 layout:

The examples below use `$KONGCTL_CONFIG_HOME` as shorthand for the resolved
kongctl config directory.

```text
$KONGCTL_CONFIG_HOME/
  extensions/
    installed/
      foo/
        extension.yaml
        bin/kongctl-ext-foo
    linked/
      foo.json
    state/
      foo.json
    cache/
      foo.json
    runtime/
      <session-id>/
        context.json
```

The exact file names can change during implementation, but the separation
should remain:

- `installed/` contains copied release or source fallback packages.
- `linked/` records local development links to working trees.
- `state/` records source, selected ref, resolved commit, package hash,
  manifest hash, runtime hash, install mode, and timestamps.
- `cache/` records validated manifest command metadata for startup command
  registration, help, completion, inspection, and collision checks.
- `runtime/` contains ephemeral context files and related session artifacts.

Runtime files should be best-effort cleaned up after execution and reaped
opportunistically on later runs. A conservative initial stale threshold such as
24 hours is appropriate and should be easy to adjust.

### 9. Pass Runtime Context Through An Inherited Environment Variable

The parent `kongctl` process should resolve invocation-bound state, write a
machine-generated `context.json` to a temporary runtime location, and pass the
full file path to the child through:

```text
KONGCTL_EXTENSION_CONTEXT=/path/to/context.json
```

This is preferable to:

- positional JSON arguments
- hidden bootstrap flags
- raw JSON embedded directly in environment variables

Future transport upgrades can add additional runtime artifacts alongside this
file or through additional environment variables without changing the core
bootstrap contract: the child gets a direct path to `context.json`.

### 10. Keep Secrets Out Of The Runtime Context

The runtime context should include resolved invocation state such as:

- matched command path
- selected profile
- resolved base URL
- output mode
- log level
- config file path
- remaining args
- auth mode and auth source metadata
- active session metadata

It should not include:

- tokens
- refresh credentials
- copied secrets from the host environment

Transient secrets that are already part of the invocation, such as a PAT passed
with `--pat`, may still be propagated to the extension process and nested
`kongctl` subprocesses through the normal process environment or existing
configuration mechanisms. The important boundary is that `context.json` never
serializes those secrets. It should record non-secret metadata such as
`auth_mode: pat` or `auth_mode: device`.

### 11. Make Nested `kongctl` Calls Session-Aware

When an extension runs `kongctl api ...` or `kongctl get config <field>`, the
nested `kongctl` subprocess should inherit
`KONGCTL_EXTENSION_CONTEXT`, reload `context.json`, and bootstrap itself
using the same resolved invocation state.

That means the child does not need to replay:

- `--profile`
- base URL overrides
- config file selection
- other session-bound settings

Nested calls should inherit the effective auth context from the parent
invocation. If the parent used a transient PAT, nested `kongctl` helpers should
continue to work without writing that PAT into `context.json`.

This is the key design point that makes CLI-first callbacks workable.

### 12. Use A CLI-First Host Callback Model In v1

For v1, the main host callback surface should be the `kongctl` CLI itself.

The most important existing host callback is:

- `kongctl api`

This is a useful low-level foundation because it already supports:

- arbitrary Konnect API calls
- structured JSON output
- built-in jq filtering

Additional machine-friendly helper commands should be added where necessary,
especially:

- `kongctl get config <field>`
- `kongctl version --json`

`kongctl api` is not a full extension API by itself. It still requires
extension authors to understand Konnect API paths, pagination, and response
shapes. That is acceptable in v1 if `kongctl` is explicit about the tradeoff:

- scripts and binaries can use it directly as a normal extension path
- future targeted helpers can raise the abstraction level where the raw API
  proves too painful

This is especially relevant for Go-based extensions. A child extension process
cannot reuse the parent `kongctl` process's in-memory authenticated HTTP
client. If a Go extension imports `sdk-konnect-go` directly, it can inherit
resolved values like `profile` and `base_url` from `context.json`, but it still
needs some way to obtain the effective authenticated client behavior. Without a
host bridge, the extension would need to reproduce `kongctl`'s token
resolution, refresh handling, timeout settings, transport options, and client
construction itself.

### 13. Defer A Go SDK Until It Is Clearly Needed

The design does not need to require a Go SDK in the first implementation.

Go-based extensions can still be supported in v1 without a host-owned SDK:

- they can read `context.json` directly
- they can invoke `kongctl api` and other helper commands directly
- they can import `sdk-konnect-go` themselves when they want richer typed API
  access

However, the third option currently has a real gap. Importing
`sdk-konnect-go` directly does not automatically give the extension the same
authorization, profile, refresh-token handling, timeout settings, transport
options, or logging behavior that `kongctl` uses internally. If the extension
does not re-enter `kongctl`, it would need to recreate that client setup
itself.

If a clear repeated pattern emerges across real extensions, `kongctl` can add a
small helper library later. That library should be justified by actual author
pain, not added speculatively.

### 14. Add Cleanup And Recursion Protection From The Start

Because the runtime model writes temporary context files, the implementation
must be disciplined:

- remove the temporary context file and any related runtime artifacts on
  normal exit
- perform opportunistic stale-file cleanup on future runs
- keep runtime files in a temp or runtime location, not the permanent config
  tree

Because extensions can contribute command paths such as `kongctl get foo`, the
design must also include a recursion guard. The session context should track:

- active contribution id
- depth

Nested calls that would redispatch to the same contribution should be rejected
by default.

## Why This Design Is Recommended

This design is recommended because it satisfies the strongest product
constraint, keeps the runtime simple enough for v1, and stays compatible with
both script authors and Go authors.

It is especially well suited to `kongctl` because:

1. the current CLI grammar is already verb-first
2. `kongctl api` already provides a useful standard low-level path for host
   callbacks
3. the root command tree is currently static and easier to extend with managed
   fallback dispatch than with deep in-process plugins
4. the design can evolve toward richer IPC later without throwing away the v1
   authoring model

## Local `kongctl` Observations

The current codebase matters because it constrains how an extension system can
fit in without destabilizing the CLI.

### The current root command tree is static

Today the root command wiring is done through explicit command construction in
[`internal/cmd/root/root.go`](../internal/cmd/root/root.go). The function
`addCommands()` calls `rootCmd.AddCommand(...)` repeatedly for each built-in
verb and topic. That is a conventional Cobra structure and it is easy to
reason about, but it means `kongctl` does not already have a dynamic command
loading model.

This suggests that the safest first extension implementation keeps built-ins
authoritative and adds extensions through managed registration rather than
through ad hoc command-not-found interception.

However, Cobra does not offer a particularly clean "unknown command" hook for a
design like this, especially with `TraverseChildren = true` already enabled.
That makes a purely reactive fallback fragile.

The safer v1 approach is:

- load cached validated manifest metadata during startup
- register synthetic Cobra commands for extension command paths
- keep built-ins authoritative by rejecting install-time collisions
- use those synthetic commands for dispatch, help, and completion

This is one of the most important implementation details in the design. The
manifest metadata cache is not just an optimization. It is the mechanism that
makes Cobra integration practical.

This also implies a completion strategy: shell completion should be driven by
runtime manifest metadata loading, not by asking users to regenerate completion
scripts after every install or uninstall. The generated shell script can
remain static while still calling back into `kongctl` dynamically at completion
time.

### The current `skills/` mechanism is not a CLI extension model

The repository already contains a `skills/` directory, but it is clearly aimed
at AI coding agents rather than end-user CLI extensibility.

- [`skills/README.md`](../skills/README.md) describes these as human-maintained
  skills for agent tooling
- [`skills/embed.go`](../skills/embed.go) embeds built-in skills as assets

That matters because issue #826 should not conflate the two concepts:

- AI agent skills are documentation and prompt assets
- CLI extensions are runtime command extensions for end users

That said, there is still a practical overlap in command structure. The current
CLI already has an `install` verb with an `install skills` subcommand. The
extension design should reuse that existing verb-object pattern rather than
inventing a separate management namespace.

Recommended coexistence model:

- keep `install skills` for agent skills
- add `install extension` for CLI extensions
- mirror that pattern for `list`, `inspect`, `upgrade`, and `uninstall` where
  it makes sense
- share lifecycle UX conventions where useful, but do not force skills and
  extensions into the same runtime model

## Evaluation Criteria

The peer systems were evaluated against these dimensions:

- end-user experience
- extension author experience
- host integration depth
- security and governance
- compatibility and maintenance
- fit for `kongctl`

## Peer System Survey

## GitHub CLI (`gh`)

### How it works

GitHub CLI extensions are repositories whose names start with `gh-`, and each
extension repository must contain an executable with the same name or provide
precompiled release assets. GitHub documents both script-based and precompiled
extensions, and it ships scaffolding via `gh extension create`
([docs](https://docs.github.com/en/github-cli/github-cli/creating-github-cli-extensions)).

Important properties from the official docs:

- Users install via `gh extension install`.
- Remote install first looks for release artifacts and otherwise clones the
  repository as a script extension
  ([manual](https://cli.github.com/manual/gh_extension_install)).
- Local development install can point at `.` and is managed as a symlink to an
  executable in the repository root
  ([manual](https://cli.github.com/manual/gh_extension_install)).
- Extensions cannot override core `gh` commands
  ([manual](https://cli.github.com/manual/gh_extension)).
- GitHub explicitly warns that third-party extensions are not certified by
  GitHub and tells users to audit the source before installing or updating
  ([docs](https://docs.github.com/en/github-cli/github-cli/using-github-cli-extensions)).

GitHub also documents a helper Go library, `go-gh`, for precompiled
extensions, which is a notable design choice:

- the host does not expose internal structs directly
- instead, it offers a stable helper library and host commands

### Strengths

1. Very low barrier to entry.
2. Supports both shell scripts and compiled binaries.
3. Excellent local development story through `gh extension install .`.
4. Good discoverability through naming, topic tags, and built-in extension
   commands.
5. Clear collision rule: extensions do not replace core commands.
6. Good example of shipping an authoring path without exposing host internals.

### Weaknesses

1. Trust model is intentionally light. GitHub warns users, but the system still
   executes arbitrary local code.
2. Host integration is shallow. Most extension integration happens by calling
   back into `gh` commands or APIs, not by sharing an internal runtime.
3. There is no formal permission model for extensions.

### Lessons for `kongctl`

GitHub CLI is one of the strongest precedents for the execution model and
authoring ergonomics of a first-generation `kongctl` extension system, but not
for namespace injection under existing verbs.

`kongctl` should borrow these specific ideas:

- additive commands, not core overrides
- script or compiled binary support
- local link/install flow for development
- scaffolding for authors
- stable helper APIs rather than internal package exposure

`kongctl` is intentionally deviating from `gh` in one important way: it wants
to preserve a verb-first grammar such as `kongctl get foo`. That makes visible
provenance in help and completion more important than it is in `gh`.

## `kubectl` and Krew

### How `kubectl` works

`kubectl` plugins are standalone executable files whose names begin with
`kubectl-` ([docs](https://kubernetes.io/docs/tasks/extend-kubectl/kubectl-plugins/)).

The official docs emphasize the simplicity of the model:

- place an executable on `PATH`
- invoke it as `kubectl foo`
- `kubectl plugin list` scans `PATH` and reports matches

The same docs also make several important limitations explicit:

- plugins cannot overwrite existing `kubectl` commands
- plugins cannot generally extend existing command trees except for the special
  `kubectl create something` pattern
- all args and flags after the plugin name are passed through as-is
- environment variables are inherited as-is

The docs also note that older plugin-specific environment variables are gone,
and that plugin authors must parse their own arguments. For Go plugin authors,
Kubernetes points them to `cli-runtime`, which provides helpers for kubeconfig,
API requests, flags, and printing
([docs](https://kubernetes.io/docs/tasks/extend-kubectl/kubectl-plugins/)).

### How Krew works

Krew is a plugin manager layered on top of `kubectl`. Its manifest format adds
install metadata such as download URLs, checksums, extracted files, supported
platforms, and binary paths
([docs](https://krew.sigs.k8s.io/docs/developer-guide/plugin-manifest/)).

Krew solves several problems raw `kubectl` does not solve:

- packaging
- installation
- upgrade workflows
- platform selection
- discoverability through a shared index

However, Kubernetes is explicit that Krew plugins are not audited for security
([docs](https://kubernetes.io/docs/tasks/extend-kubectl/kubectl-plugins/)).

### Strengths

1. Extremely language-agnostic.
2. Very easy to understand operationally.
3. Very easy to prototype.
4. Krew adds the metadata and distribution layer that raw `PATH` discovery
   lacks.
5. `cli-runtime` is an excellent precedent for a host-owned helper library.

### Weaknesses

1. Raw `PATH` scanning is weak as a product UX.
2. Security posture is weak unless paired with extra controls.
3. Integration with host behavior is shallow by default.
4. Plugins do not get deep lifecycle hooks.
5. The command collision rules are strict and occasionally awkward.

### Lessons for `kongctl`

Raw `kubectl`-style `PATH` discovery alone is not enough for `kongctl`.

The good ideas are:

- executables as the extension unit
- language neutrality
- a host-owned helper library
- a metadata layer similar to Krew

The weak ideas are:

- ungoverned `PATH` discovery as the main installation story
- minimal trust controls

## Helm

### How it works

Helm plugins live under `$HELM_PLUGINS` and are described by a `plugin.yaml`
file ([docs](https://helm.sh/docs/topics/plugins/)).

Helm's model is richer than raw `kubectl`:

- plugins declare metadata in `plugin.yaml`
- plugins can define platform-specific commands
- plugins can define lifecycle hooks such as install, update, and delete
- plugins can even register downloader capabilities for custom repository
  protocols ([docs](https://helm.sh/docs/topics/plugins/))

Helm also has a stronger verification story than many peers:

- for remote tarball installs, signatures are verified by default
- users can explicitly disable verification
  ([docs](https://helm.sh/docs/helm/helm_plugin_install/))

### Strengths

1. Manifest-driven installation is much more product-friendly than raw `PATH`
   discovery.
2. Platform-specific entry points are first-class.
3. The model supports both command plugins and special capabilities such as
   downloaders.
4. Signature verification is a meaningful improvement over many peers.

### Weaknesses

1. Lifecycle hooks increase complexity and attack surface.
2. The model still executes arbitrary local code.
3. Deep host integration is still limited compared with framework-style plugin
   platforms.

### Lessons for `kongctl`

Helm provides a strong precedent for:

- a manifest file
- install metadata
- compatibility metadata
- verification by default

It is also a caution that install hooks should not be added casually. For
`kongctl`, it would be better to skip install hooks in v1.

## Heroku CLI and the `oclif` ecosystem

### How it works

Heroku CLI is built on `oclif`, a Node.js CLI framework with runtime plugin
support ([docs](https://devcenter.heroku.com/articles/developing-cli-plugins),
[framework docs](https://oclif.io/docs/plugins/)).

Important facts from first-party docs:

- users install plugins with `heroku plugins:install`
- plugin developers can use `heroku plugins:link` for local development
- plugins auto-update alongside the CLI
- `oclif` plugins can export commands, hooks, and even other plugins
- user-installed plugins can override core plugins in `oclif`'s runtime plugin
  model
  ([Heroku docs](https://devcenter.heroku.com/articles/using-cli-plugins),
  [oclif docs](https://github.com/oclif/plugin-plugins))

`oclif` itself supports lifecycle hooks such as `init`, `prerun`,
`command_not_found`, and custom events
([docs](https://oclif.io/docs/hooks/)).

### Strengths

1. Deep host integration.
2. Good developer ergonomics in the Node ecosystem.
3. Excellent local development link workflow.
4. Hooks enable advanced extensibility.
5. Mature ecosystem concepts such as aliases, plugin renames, and migration of
   functionality into core.

### Weaknesses

1. The model is tightly coupled to the host runtime and package ecosystem.
2. The host must freeze more behavior and be more disciplined about plugin
   compatibility.
3. Override behavior is powerful but risky.
4. This is heavier than an additive command model.

### Lessons for `kongctl`

Heroku is useful as a "do later if needed" precedent, not as the first move.

The big ideas worth borrowing are:

- `install`, `link`, `inspect`, `update`
- structured migration and aliasing support
- good local author workflow

The big ideas to avoid initially are:

- core command overrides
- deep lifecycle hook surfaces
- extension logic that is tightly coupled to host internals

## Salesforce CLI

### How it works

Salesforce CLI is one of the richest documented plugin ecosystems among peer
CLIs. Salesforce describes the CLI as an npm package that supports custom
plugins, and states that "Salesforce CLI is itself a plugin" and that most of
its core functionality comes from plugins
([overview](https://developer.salesforce.com/docs/platform/salesforce-cli-plugin/guide/conceptual-overview.html)).

Several design details are especially relevant:

- plugins are npm packages installed with `sf plugins install`
- core functionality is built out of plugins
- Salesforce provides plugin generators and shared libraries
- `@salesforce/core` exposes org auth, configuration, connections, and
  utilities that plugin authors can reuse
  ([library docs](https://developer.salesforce.com/docs/platform/salesforce-cli-plugin/guide/use-libraries.html))
- Salesforce supports hooks through the `oclif` hook system
  ([docs](https://developer.salesforce.com/docs/platform/salesforce-cli-plugin/guide/hooks.html))

Salesforce also has an unusually explicit trust model. Salesforce's official
blog explains that plugin installs go through signature validation, and that
unsigned plugins require explicit acknowledgment or allowlisting in CI
([blog](https://developer.salesforce.com/blogs/2017/10/salesforce-dx-cli-plugin-update)).

### Strengths

1. Best-in-class example of first-party extension author tooling.
2. Best-in-class example of exposing stable host libraries instead of internals.
3. Good diagnostics and discoverability features, such as plugin listing and
   inspection.
4. More serious trust posture than most peers.
5. Demonstrates that a CLI can make plugins central to its architecture.

### Weaknesses

1. Heavyweight model.
2. Tied to Node/npm and the `oclif` runtime.
3. Demands long-term API discipline from the host team.
4. More moving parts than a simple command-extension system.

### Lessons for `kongctl`

Salesforce is the strongest precedent for the idea that extension authors need
stable host-facing libraries and diagnostics.

For `kongctl`, that implies:

- do not expose `internal/...` packages directly
- create a stable, extension-facing API or SDK
- add `inspect`, `which`, and `doctor` style tools over time
- add signatures or equivalent trust signals from the start

## Terraform and `go-plugin`

### How it works

Terraform is built on a plugin-based architecture where Terraform Core uses RPC
to talk to plugins
([docs](https://developer.hashicorp.com/terraform/plugin/how-terraform-works)).

HashiCorp's `go-plugin` library is the underlying reference point for many of
these systems. It launches subprocesses, communicates over RPC or gRPC, and
supports features such as protocol versioning, logging, stdout/stderr syncing,
checksums, and TLS
([repo](https://github.com/hashicorp/go-plugin)).

Terraform also has mature installation controls:

- plugin and provider installation behavior can be configured
- local mirrors and air-gapped installation workflows are supported
- installation behavior can be constrained deliberately
  ([docs](https://developer.hashicorp.com/terraform/cli/plugins))

### Strengths

1. Strong typed contract between host and plugin.
2. Good compatibility story through explicit protocol versioning.
3. Separate-process isolation.
4. Enterprise-friendly installation and mirroring options.
5. Good fit when plugins model durable provider-like capabilities.

### Weaknesses

1. Significantly heavier than simple command plugins.
2. Poor fit for shell-script authors.
3. Requires a more opinionated plugin API design.
4. More engineering cost for both host and authors.

### Lessons for `kongctl`

Terraform's model is not the right first answer for general `kongctl`
extensions, but it is the right precedent for any future, deeper contract such
as:

- advanced lifecycle hooks
- provider-like adapters
- long-lived background integrations
- stable typed service boundaries

It is best treated as a possible second-generation extension surface.

## Docker CLI patterns

### How it works

Docker has more than one plugin story, which is important because it shows how
easy it is to conflate separate concepts.

For CLI command extensions, Docker-compatible client plugins commonly live in a
managed directory such as `~/.docker/cli-plugins`, where a binary named
`docker-foo` becomes `docker foo`
([example docs](https://docs.docker.com/dhi/how-to/cli/)).

Docker's CLI plugin ecosystem also defines a metadata subcommand,
`docker-cli-plugin-metadata`, which plugins use to report structured metadata.
That is a useful precedent for a possible future generated-metadata mode, even
though v1 `kongctl` should use static command metadata from `extension.yaml`
([design reference](https://github.com/docker/cli/issues/1534),
[Go package](https://pkg.go.dev/github.com/docker/cli/cli-plugins/metadata)).

Docker also has daemon-side engine plugins managed with `docker plugin`, which
are a different category. Those plugins can request privileges such as network,
devices, and capabilities, and Docker prompts the user to accept those
permissions during installation
([docs](https://docs.docker.com/reference/cli/docker/plugin/install/)).

### Strengths

1. Managed plugin directories are cleaner than raw `PATH` scanning.
2. Naming conventions are predictable.
3. Docker's privilege prompt is a useful precedent for communicating risk.

### Weaknesses

1. The Docker ecosystem contains multiple plugin concepts, which can be
   confusing.
2. The client plugin pattern is mostly binary-oriented.
3. The daemon plugin security model does not translate directly to local CLI
   command extensions.

### Lessons for `kongctl`

`kongctl` should strongly prefer a managed extension directory over raw
`PATH` scanning.

Docker is also supporting evidence that a future plugin runtime could add a
hidden metadata/reporting subcommand without making the user-facing command
surface awkward. For v1, `kongctl` should prefer static command metadata from
`extension.yaml`.

## Several SaaS CLIs Avoid Local Plugin Runtimes

Vercel, Fly.io, Railway, and Supabase are useful as a collective counterpoint:
they show that many modern SaaS CLIs either prefer tightly governed provider
integration programs or avoid general local plugin execution entirely
([Vercel CLI docs](https://vercel.com/docs/cli/integration),
[Vercel integration docs](https://vercel.com/docs/integrations/create-integration),
[Fly CLI docs](https://fly.io/docs/flyctl/extensions/),
[Fly program docs](https://fly.io/docs/about/extensions/),
[Railway CLI docs](https://docs.railway.com/cli),
[Supabase CLI docs](https://supabase.com/docs/guides/local-development/cli/getting-started)).

The strategic lesson for `kongctl` is simple: a local extension ecosystem is
optional, not inevitable. If `kongctl` opens this surface, it should do so
intentionally and with clear governance, because open local execution always
expands the support and security burden.

## Comparable Technology Choices That `kongctl` Should Treat Carefully

## Go stdlib `plugin`

Go's `plugin` package supports dynamically loaded Go plugins, but the official
Go docs carry unusually strong warnings
([docs](https://go.dev/pkg/plugin/?m=old)).

The warnings include:

- limited platform portability
- poor race detector support
- difficult deployment
- harder initialization reasoning
- risks from dangerous or untrusted libraries
- likely runtime crashes unless application and plugin are built with the same
  toolchain, flags, env, and dependency sources

The Go docs go so far as to say that, in practice, the application and plugins
often need to be built together and that IPC or RPC may be more suitable.

This is a strong reason not to use stdlib `plugin` for `kongctl`.

If `kongctl` also intends to keep a `CGO_ENABLED=0` build discipline, stdlib
`plugin` becomes even less attractive.

## WASM Runtimes

WebAssembly is attractive when true sandboxing becomes important.

`wazero` is especially relevant because it is written completely in Go, has no
platform dependencies, and runs guest modules in sandboxes
([docs](https://wazero.io/docs/)).

`Extism` is also relevant because it is designed as a general plug-in system
with host SDKs for multiple languages
([docs](https://extism.org/docs/overview/)).

WASM's tradeoff is that it provides better isolation, but usually at the cost
of a more specialized authoring model and a more constrained host API.

For `kongctl`, that suggests WASM is a valuable future option for
high-trust-isolation scenarios, but likely not the best first extension model.

## Cross-Cutting Findings

Across the surveyed tools, several themes repeat.

### 1. Additive commands are the safest first extension point

The most successful extension systems generally begin by adding new commands,
not by letting plugins modify existing command flows.

Examples:

- `gh` adds `gh foo`, but extensions cannot override core commands.
- `kubectl` adds `kubectl foo`, but cannot override core commands.
- Helm adds new plugin commands.

This is the single clearest pattern in the ecosystem.

`kongctl` is intentionally bending this pattern by allowing command paths under
`get` and `list`. If it does that, it needs stronger provenance labeling and
stricter collision rules than top-level-only systems such as `gh`.

### 2. Raw discovery is not enough

Pure executable discovery on `PATH` is elegant, but incomplete. Sooner or
later, real systems add:

- manifests
- checksums
- compatibility metadata
- discovery commands
- install and update commands

Krew and Helm show this clearly.

### 3. Deep integration requires a stable host-facing API

The systems with the best extension author experience expose stable host-facing
libraries rather than forcing authors to reverse-engineer internals.

Examples:

- Kubernetes offers `cli-runtime`.
- GitHub offers `go-gh`.
- Salesforce offers `@salesforce/core`.

This is a major point for `kongctl`: extension authors need a stable interface,
but they should not depend on `internal/...`.

### 4. Trust posture varies enormously

The weakest trust models are essentially:

- install arbitrary code
- hope the user audits it

That is where `gh` and raw `kubectl` land today.

The stronger trust models add some combination of:

- checksums
- signatures
- curated indexes
- allowlists
- install prompts
- enterprise policy controls

Helm, Salesforce, and Terraform provide better precedents here.

### 5. The more powerful the plugin surface, the more expensive it is to own

Framework-style plugins and RPC plugin systems are powerful, but they also
force the host team to own:

- compatibility guarantees
- richer diagnostics
- longer deprecation windows
- more complex author tooling
- more security review

That is why a command-extension model is usually the right first move.

## What Appears To Work Best For End Users

With the caveat that this is inferential rather than a measured sentiment
study, the best end-user patterns appear to be:

### Best end-user patterns

1. A single install command owned by the host CLI.
2. Predictable command naming.
3. Safe collision rules with core commands.
4. Local development linking without publishing.
5. Upgrade and inspect commands.
6. Discoverability through an index, topic, or marketplace.

Systems that seem strongest here:

- GitHub CLI
- `kubectl` plus Krew
- Helm

### End-user pain points that repeatedly appear

1. Arbitrary code execution with weak trust signaling.
2. Confusion around script vs binary runtime requirements.
3. Poor discoverability when there is no index or install manager.
4. Conflicts when plugins can override core commands.

## What Appears To Work Best For Extension Authors

Again, this is inferential rather than a measured survey.

### Best author patterns

1. A simple local link or install flow.
2. Official scaffolding.
3. Stable helper libraries for auth, config, and output.
4. Compatibility metadata and diagnostics.
5. A stable contract that does not require depending on host internals.

Systems that seem strongest here:

- GitHub CLI for low-friction command extensions
- Salesforce CLI for rich authoring support and host libraries
- Heroku and `oclif` for framework-style plugin authoring

### Author pain points that repeatedly appear

1. Heavy runtime ecosystems can exclude some authors.
2. Raw shell-style models are easy to start but weak for deeper integration.
3. Deep hook systems require much more knowledge of host behavior.

## Candidate `kongctl` Extension Architectures

## Option A: Raw `PATH`-discovered executables

### Summary

Look for `kongctl-foo` on `PATH` and execute it when the user runs
`kongctl foo`.

### Pros

- simplest possible implementation
- any language
- shell script friendly
- easy to prototype

### Cons

- weak install UX
- weak upgrade UX
- weak trust story
- weak discoverability
- more collision and shadowing ambiguity

### Fit for `kongctl`

Poor as the main product surface. Acceptable only as an emergency compatibility
mode or a debugging fallback.

## Option B: Managed external command extensions

### Summary

`kongctl` installs extensions into its own extension home, reads a manifest,
and dispatches additive commands to the extension executable.

### Pros

- script and binary friendly
- separate-process isolation
- good install UX
- good compatibility UX
- realistic to build incrementally
- best fit for current Cobra structure

### Cons

- host integration must be designed deliberately
- still arbitrary local code unless sandboxed
- capability enforcement is mostly advisory without sandboxing

### Fit for `kongctl`

Excellent. This is the recommended first-generation model.

## Option C: Shared-runtime framework plugins

### Summary

Extensions compile against a stable Go extension framework and register
commands, hooks, or capabilities inside a host-owned runtime.

### Pros

- deep host integration
- very strong Go author experience if done well
- shared libraries for config, auth, output, logging

### Cons

- strong compatibility burden
- more fragile over time
- harder to support across host versions
- much heavier engineering investment

### Fit for `kongctl`

Possible later, but not the best first step.

## Option D: RPC plugins

### Summary

Extensions are subprocesses that communicate with `kongctl` over a typed RPC or
gRPC protocol.

### Pros

- stable versioned interface
- separate-process isolation
- best model for deep capabilities without internal imports

### Cons

- complex for casual authors
- not friendly for shell scripts
- more protocol and SDK work

### Fit for `kongctl`

Better as a future advanced extension lane than as the default first model.

## Option E: WASM plugins

### Summary

Extensions compile to WASM and run inside a host-controlled sandbox.

### Pros

- strongest isolation story
- host can enforce capability boundaries more meaningfully

### Cons

- higher authoring complexity
- more constrained programming model
- more runtime design work

### Fit for `kongctl`

Good future option for high-security or policy-driven extensions, but likely
too heavy for the first release.

## Option F: Multiple extension points

### Summary

Support more than one extension lane.

The most pragmatic version of this is:

1. command extensions for most use cases
2. optional richer SDK or RPC lane later
3. optional WASM lane later for higher isolation

### Pros

- accommodates shell, binary, and richer integrations
- avoids forcing one authoring model on everyone

### Cons

- more product surface
- more docs and support burden
- easy to over-design too early

### Fit for `kongctl`

Good as a phased strategy, not as a day-one everything model.

## Detailed Design And Delivery Plan

## Summary Conclusion

`kongctl` should implement a managed external-command extension system that
preserves the `kongctl <verb> ...` model instead of replacing it with isolated
top-level extension commands. The core design choices are:

1. allow extension-contributed `command_paths`
2. allow those command paths to land under the open existing verbs `get` and
   `list`, or to define a new verb naturally through the first path segment
3. use `extension.yaml` for package, runtime, and command metadata
4. validate and cache command metadata from the manifest without executing the
   extension during install or link
5. pass runtime context through `KONGCTL_EXTENSION_CONTEXT`
6. keep the v1 host callback model CLI-first

This direction is supported by the peer research and by local `kongctl`
constraints:

1. GitHub CLI proves that child-process extensions and a thin helper library
   can work well.
2. `kubectl` and Krew prove that executable plugins need install metadata and
   management commands.
3. Helm proves that a custom YAML manifest and verification signals are a good
   fit for CLI plugins.
4. Salesforce proves that a stable helper library becomes valuable as the
   ecosystem matures.
5. The current `kongctl` root command tree is static, which favors managed
   dispatch over deep in-process mutation.

## Recommended v1 Scope

The first extension release should include only these features.

### 1. Command Paths

Allowed in v1:

- `command_paths` whose first segment is `get` or `list`
- `command_paths` whose first segment is a new verb not claimed by a built-in
  command
- multi-segment command paths such as `get foo bar`
- per-segment aliases, except aliases on reserved built-in root segments

Disallowed in v1:

- overriding built-in commands
- collisions with built-in resources or other extensions
- `command_paths` under any existing verb other than `get` or `list`
- general host lifecycle hooks

This lets `kongctl` preserve its existing grammar without opening the most
dangerous integration points too early.

Each path segment should be represented as an object:

```yaml
command_paths:
  - id: get_foo_bar
    path:
      - name: get
      - name: foo
        aliases: [foos]
      - name: bar
        aliases: [bars]
    summary: Get Foo Bar resources
```

Built-in root segments such as `get` and `list` are reserved and cannot define
aliases. Custom root verbs can define aliases when they do not collide with
built-in command names, built-in aliases, reserved names, or other extensions.
Every canonical segment and alias is collision-checked at its parent node. If
an extension contributes `get foo bar` but not a runnable `get foo`, `kongctl`
may synthesize `get foo` as a non-runnable namespace/help node.

### 2. Managed Install, Remove, Upgrade, And Link Workflow

`kongctl` should own extension lifecycle commands.

Required v1 commands:

- `kongctl install extension <source>`
- `kongctl uninstall extension <name>`
- `kongctl upgrade extension <name>`
- `kongctl list extensions`
- `kongctl inspect extension <name>`
- `kongctl link extension <path>`

Recommended but optional for the earliest cut:

- `kongctl upgrade extension --all`
- `kongctl create extension <name>`
- `kongctl search extensions`

### 3. Installation Sources And Discovery

The v1 install model should be explicit and source-driven.

Supported v1 sources:

1. local filesystem path
2. GitHub repository reference such as `owner/repo`

Recommended install rules:

1. local path installs require `extension.yaml`
2. `link` should be used for local development workflows
3. command metadata should be read from `extension.yaml`, validated, and cached
4. GitHub installs should follow the GitHub CLI model: prefer compatible
   release artifacts and fall back to source clone only for already-runnable
   script or binary extensions
5. no source compilation should happen during install
6. store source, selected ref, resolved commit, package hash where available,
   manifest hash, and runtime hash so `upgrade` can reuse the same strategy
7. installed runtime hashes should be verified before execution

Release artifacts should use a strict archive layout. The archive root must
contain `extension.yaml`; `runtime.command` points to an already-runnable
script or binary inside that extracted root:

```text
extension.yaml
bin/kongctl-ext-foo
README.md
```

```yaml
runtime:
  command: bin/kongctl-ext-foo
```

Release asset names should follow:

```text
kongctl-ext-foo_0.1.0_linux_amd64.tar.gz
kongctl-ext-foo_0.1.0_darwin_arm64.tar.gz
kongctl-ext-foo_0.1.0_windows_amd64.zip
```

Source fallback repositories and local path installs use the same
`runtime.command` rule. The canonical source fallback layout is:

```text
extension.yaml
kongctl-ext-foo
```

```yaml
runtime:
  command: kongctl-ext-foo
```

`runtime.command` is relative to the extension root. It must not be absolute,
must not contain `..`, and must resolve inside the extension root. On Unix, it
must be executable. On Windows, the manifest should name a runnable file for
the platform, such as `.exe`, `.cmd`, or `.bat`; v1 should avoid ambiguous
PATH-style suffix guessing.

Persistent extension storage should live under the existing kongctl config
home:

The examples below use `$KONGCTL_CONFIG_HOME` as shorthand for the resolved
kongctl config directory.

```text
$KONGCTL_CONFIG_HOME/
  extensions/
    installed/
    linked/
    state/
    cache/
    runtime/
```

`installed/` contains copied packages, `linked/` records local development
links, `state/` records provenance and hashes, `cache/` records validated
manifest metadata, and `runtime/` contains ephemeral context files.

Recommended non-goal for v1:

- no broad catalog or marketplace search requirement

Recommended later addition:

- an official index or catalog for search, trust tiering, and discoverability

### 4. Extension Manifest

Every installable extension should include an `extension.yaml` manifest.

Recommended fields:

```yaml
schema_version: 1

name: foo
publisher: kong
version: 0.1.0
summary: Foo resource support for kongctl

runtime:
  command: kongctl-ext-foo

compatibility:
  min_version: 0.20.0
  max_version: 0.x

command_paths:
  - id: get_foo
    path:
      - name: get
      - name: foo
        aliases: [foos]
    summary: Get Foo resources
    description: Retrieves Foo resources from Konnect.
    usage: kongctl get foo [name] [flags]
    examples:
      - kongctl get foo
      - kongctl get foo my-foo --output json
    args:
      - name: name
        required: false
        repeatable: false
        description: Optional Foo resource name.
    flags:
      - name: filter
        type: string
        description: Filter Foo resources by label.
```

Important design notes:

- `schema_version` is simpler than Kubernetes-style object metadata
- compatibility should express the supported `kongctl` version range without
  repeating `kongctl` in every field name
- `runtime.command` is the concrete v1 execution hook and tells `kongctl`
  which executable to run
- install source, upgrade state, and hash observations should be stored by
  `kongctl`, not embedded in the extension manifest
- command metadata belongs in the manifest for v1

Deferred from the v1 manifest:

- capability declarations
- trust tiers or publisher verification markers
- distribution metadata for registries, indexes, or signed catalogs
- alternative runtime types beyond executable child processes

### 5. Static Command Metadata Contract

All command metadata should come from the manifest in v1.

That includes:

- `command_paths`
- summaries
- descriptions
- usage text
- examples
- args
- flags

`kongctl` should validate this metadata at install or link time, reject any
restricted or colliding command paths, and cache the validated metadata for:

- `help`
- completion
- `inspect extension`
- conflict checks

The earlier `__kongctl describe` concept should be deferred. It can be added
later as an optional authoring convenience for trusted or linked development
workflows, but v1 should not execute extension code during install or link just
to discover metadata.

The tradeoff is that manifest metadata can drift from executable behavior.
`kongctl` should reduce accidental drift by storing manifest/runtime hashes
for installed extensions and verifying the installed runtime hash before
execution. This is package-integrity checking, not a behavioral guarantee.

### 6. Runtime Context Contract

The host should resolve effective invocation state before executing the
extension. That includes:

- matched command path
- selected profile
- resolved base URL
- output format
- log level
- config file path
- CLI version
- original and remaining args
- auth mode and auth source metadata
- active extension session metadata

The runtime contract should be:

- an inherited `KONGCTL_EXTENSION_CONTEXT`
- pointing directly at a machine-generated `context.json`

Example:

```json
{
  "schema_version": 1,
  "matched_command_path": {
    "id": "get_foo",
    "path": ["get", "foo"]
  },
  "invocation": {
    "original_args": ["get", "foo", "--limit", "10"],
    "remaining_args": ["--limit", "10"]
  },
  "resolved": {
    "profile": "default",
    "base_url": "https://us.api.konghq.com",
    "output": "json",
    "log_level": "debug",
    "config_file": "/home/me/.config/kongctl/config.yaml",
    "auth_mode": "pat",
    "auth_source": "flag"
  },
  "host": {
    "kongctl_path": "/usr/local/bin/kongctl",
    "kongctl_version": "0.20.0"
  },
  "session": {
    "id": "9f4e2a",
    "active_contribution_id": "get_foo",
    "depth": 0
  }
}
```

This is the right place to preserve `profile`, `base_url`, and other resolved
settings that the child should not have to rediscover.

No secrets should be written into this file.

Transient secrets such as a PAT supplied by `--pat` may be propagated through
the normal process environment or existing config/auth mechanisms so nested
`kongctl` helpers can keep using the effective parent auth context. The context
file records only non-secret metadata about the selected auth mode and source.

`context.json` also needs an explicit compatibility contract:

- changes within a schema version should be additive only
- extensions should ignore unknown fields
- removing or renaming fields should require a new `schema_version`
- `kongctl` should reject unsupported schema versions rather than guessing

### 7. Session-Aware Nested `kongctl` Calls

Nested `kongctl` subprocesses should detect `KONGCTL_EXTENSION_CONTEXT`
early during startup, reload `context.json`, and use that session overlay to
preserve the parent invocation identity.

Recommended session semantics:

Locked session values:

- `profile`
- `base_url`
- selected config file
- auth selection context

Defaulted but overridable values:

- `output`
- `log_level`

This allows commands such as `kongctl api ...` to stay in the same logical
session while still allowing an extension to ask for JSON or YAML explicitly
when needed.

Context-file lifetime also needs to be safe under concurrency. The cleanup
strategy must not delete the temporary context file, or related runtime
artifacts, while nested `kongctl` children still rely on them. In practice
that means immediate cleanup should only happen when the root invocation can
prove no nested children are still active, and stale-file reaping should
remain as the safety net.

### 8. CLI-First Host Callback Surface

The v1 host callback model should be the `kongctl` CLI itself.

Existing and proposed host callbacks:

- `kongctl api ...`
- `kongctl get config <field>`
- `kongctl version --json`

This is intentionally small. `kongctl api` is a useful standard low-level
foundation,
not a full extension API. It gives extensions authenticated Konnect requests
with structured output and jq filtering, but extension authors still need to
understand API paths, pagination, and response shapes.

That is acceptable in v1 if `kongctl` is explicit about the tradeoff:

- scripts and binaries can use it directly as a normal extension path
- future targeted helpers can raise the abstraction level where the raw API
  proves too painful

This is especially relevant for Go-based extensions. A child extension process
cannot reuse the parent `kongctl` process's in-memory authenticated HTTP
client. If a Go extension imports `sdk-konnect-go` directly, it can inherit
resolved values like `profile` and `base_url` from `context.json`, but it still
needs some way to obtain the effective authenticated client behavior. Without a
host bridge, the extension would need to reproduce `kongctl`'s token
resolution, refresh handling, timeout settings, transport options, and client
construction itself.

### 9. Defer A Go SDK Until It Is Clearly Needed

The design does not need to require a Go SDK in the first implementation.

Go-based extensions can still be supported in v1 without a host-owned SDK:

- they can read `context.json` directly
- they can invoke `kongctl api` and other helper commands directly
- they can import `sdk-konnect-go` themselves when they want richer typed API
  access

However, the third option currently has a real gap. Importing
`sdk-konnect-go` directly does not automatically give the extension the same
authorization, profile, refresh-token handling, timeout settings, transport
options, or logging behavior that `kongctl` uses internally. If the extension
does not re-enter `kongctl`, it would need to recreate that client setup
itself.

If a clear repeated pattern emerges across real extensions, `kongctl` can add a
small helper library later. That library should be justified by actual author
pain, not added speculatively.

### 10. Example Extensions

The repository should include at least two example extensions:

1. one script-based extension
2. one Go-based extension without requiring a host-owned SDK

These are important deliverables because they prove:

- the manifest shape
- command-path dispatch wiring
- callback ergonomics
- install and link flows

### 11. `kongctl-extension-builder` Agent Skill

The repository should include an agent skill named
`kongctl-extension-builder`.

This is also an explicit deliverable.

The skill should help a coding agent:

- scaffold a new extension
- choose between script and Go templates
- fill in `extension.yaml`
- register command paths
- test local install and link workflows

This will make the extension system much easier to use in practice, especially
for internal Kong contributors and users working with coding agents.

## Future Plans Beyond v1

The following ideas are worth keeping in the design record, but they should not
be treated as part of the immediate v1 plan.

### Future manifest growth

Once `kongctl` has concrete install prompts, verification flows, or policy
controls, the manifest could grow to include:

- capability declarations
- trust or verification markers
- distribution metadata for indexes or catalogs
- richer runtime types beyond executable child processes

These should be added only when `kongctl` has a clear planned use for them.

### Future trust and policy work

This is where `kongctl` may eventually want to be more opinionated than many
peer CLIs.

Potential future work:

1. require checksums for remote binary assets
2. support signatures for verified publishers
3. add capability declarations for disclosure or policy controls
4. show publisher, source, version, checksum, and trust state during install
5. let organizations restrict install sources
6. add policy modes such as `official only` or `signed only`

Important limitation:

If extensions run as arbitrary scripts or binaries on the user's machine, then
capability declarations are not strong technical isolation. At best they become
disclosure, policy metadata, and install-time risk communication.

Possible future trust tiers:

- `official`
- `verified`
- `community`
- `unsigned`

Possible future policy controls:

- disable all extensions
- allow only official extensions
- allow only signed extensions
- allow only extensions from a configured index
- allowlist specific extension IDs
- denylist specific publishers

## Developer Experience Recommendations

`kongctl` should make extension development feel deliberate rather than
accidental.

### Minimum author tooling

1. `kongctl create extension <name>`
2. `kongctl link extension <path>`
3. `kongctl inspect extension <name>`
4. sample extension templates
5. sample shell extension
6. sample Go extension
7. `kongctl-extension-builder` skill

### Why shell support matters

Many peer systems succeed because the easiest path is not blocked. GitHub CLI
and `kubectl` both benefit from the fact that small automation tasks can start
as shell scripts.

That means `kongctl` should not force authors into a Go-only model.

However, script support should be described honestly. It is best suited to
simple wrappers and lightweight task automation. Once an extension owns
multiple command paths, richer help metadata, or non-trivial flag parsing, the
Go-based authoring is likely to be the more maintainable path.

### Why a Go SDK might come later

Once several Go-based extensions are repeating the same logic around:

- loading `context.json`
- invoking session-aware `kongctl` helpers
- constructing authenticated Konnect clients
- decoding repeated response shapes

then a small helper library may become worthwhile. Until that pattern is clear,
the first release does not need to commit to shipping one.

The strongest signal would be repeated extension code that is rebuilding the
same `sdk-konnect-go` client wiring from resolved config and auth state. That
would indicate `kongctl` should provide a narrow bridge for authenticated client
construction rather than forcing each extension to rediscover it.

## End-User Experience Recommendations

The user experience should be boring and predictable.

Recommended rules:

1. `kongctl` owns installation, removal, upgrade, and inspection.
2. Built-ins always win over extensions.
3. Extension command path collisions are rejected.
4. Extension-contributed commands are visibly labeled in help and completion.
5. Upgrades are explicit, not silent.
6. Local development links are clearly marked in `list` and `inspect`.

Example flow:

```text
kongctl install extension kong/kongctl-ext-foo
kongctl get foo
kongctl inspect extension foo
kongctl upgrade extension foo
kongctl uninstall extension foo
```

## Why `kongctl` Should Not Expose Internal Packages Directly

This point deserves emphasis.

If external Go extensions import `internal/...` or depend on the exact shape of
core command construction, then:

- `kongctl` loses refactoring freedom
- compatibility becomes fragile
- every internal rename becomes a plugin break risk

Peer systems handle this better:

- Kubernetes provides `cli-runtime`
- GitHub provides `go-gh`
- Salesforce provides `@salesforce/core`

`kongctl` should follow the same pattern and publish an intentionally stable
extension helper surface instead of accidentally exporting internals.

## Proposed Non-Goals For v1

The first release should explicitly not attempt all of the following.

1. No extension overrides of built-in commands.
2. No host lifecycle hooks such as `before every command`.
3. No command contributions under existing verbs other than `get` and `list`.
4. No install hooks.
5. No background daemons started by extensions during installation.
6. No promise that executable extensions are strongly sandboxed.
7. No large in-process host SDK.
8. No source compilation during extension install.
9. No required runtime metadata command such as `__kongctl describe` in v1.

Writing these non-goals down will prevent the first implementation from growing
into a framework before the basic product loop is proven.

## Suggested Phased Roadmap

## Phase 0: Validate Core Runtime Assumptions

- prototype extension dispatch end to end
- measure the cost of one extension invocation and repeated nested callbacks
- confirm that subprocess re-entry is acceptable for the expected v1 workloads
- only lock the transport after that data exists

## Phase 1: Core Runtime And Install Flows

- finalize `extension.yaml` schema
- define command-path matching and collision rules
- implement synthetic Cobra command registration from cached manifest metadata
- define the host/extension flag boundary
- define the runtime context contract
- implement `KONGCTL_EXTENSION_CONTEXT` bootstrap
- implement recursion guard
- implement `install`, `uninstall`, `list`, `inspect`, `upgrade`, and `link`
- support local path install and GitHub repo install
- support GitHub release artifacts first, with source clone fallback only for
  already-runnable script or binary extensions
- record manifest/runtime hashes and verify installed runtime hashes before
  dispatch

## Phase 2: Authoring Deliverables

- publish one script example extension
- publish one Go example extension
- add `create` scaffolding if not already shipped in phase 1
- document local development and link workflows

## Phase 3: Trust And Policy

- add checksums and signatures
- add trust tiers
- add organization policy controls
- add official-only and signed-only modes
- add clearer install-time trust prompts

## Phase 4: Agent And DX Tooling

- add the `kongctl-extension-builder` skill
- connect the skill to extension templates and examples
- add extension diagnostics and `doctor`-style guidance if needed

## Phase 5: Richer Integration

- add richer structured output helpers if needed
- evaluate JSON-RPC over stdio or a local socket if extension callback volume
  needs a faster backend
- add a small Go helper library only if repeated extension patterns justify it
- only add lifecycle hooks if concrete use cases justify them

## Resolved Implementation Decisions

These decisions should be treated as settled for v1 implementation.

1. Custom verbs are allowed in v1, and extensions may also contribute below
   `get` and `list`.
2. Existing built-in root commands other than `get` and `list` are closed to
   extension contributions.
3. Command paths are arrays of segment objects. Segments may define aliases.
   Built-in root segments such as `get` and `list` cannot define aliases.
4. Every canonical segment and alias is collision-checked at its parent node.
   Built-in command names, built-in aliases, extension lifecycle namespaces,
   help/completion internals, and other extension paths are reserved.
5. GitHub installs follow the GitHub CLI model: release artifact first, source
   clone fallback only for already-runnable script or binary extensions, and no
   source compilation during install.
6. Release artifacts are archives whose root contains `extension.yaml` and the
   runtime referenced by `runtime.command`.
7. `runtime.command` is a relative path inside the extension root. It cannot be
   absolute, cannot contain `..`, and is not resolved through `PATH`.
8. Installed extensions store install source, selected ref, resolved commit,
   package hash where available, manifest hash, and runtime hash. Installed
   runtime hashes are verified before execution. Linked extensions skip strict
   runtime hash verification.
9. Extension state is stored under the existing kongctl config home, which uses
   `$XDG_CONFIG_HOME/kongctl` or the existing user-home fallback.
10. The host parses global/root flags and passes extension-specific args and
    flags through verbatim as `remaining_args`.
11. Nested `kongctl` subprocesses inherit the effective invocation context,
    including transient auth, without writing secrets into `context.json`.
12. `profile`, `base_url`, selected config file, and auth selection context are
    locked for nested helper calls. `output` and `log_level` are defaulted from
    the parent session but can be explicitly overridden by nested commands.
13. `context.json` changes within a schema version are additive only; removing
    or renaming fields requires a new schema version.
14. `kongctl get config <field>` should be machine-oriented when used as an
    extension helper. Text output should print raw scalar values without
    decoration, while JSON/YAML output should remain structured.
15. V1 does not need `kongctl run extension <name> [args...]`; command-path
    dispatch is the primary invocation model. A direct debug runner can be
    added later if real debugging workflows need it.
16. Stale extension runtime context files should be reaped opportunistically.
    The initial implementation should use a conservative default threshold,
    such as 24 hours, and make it easy to adjust.

## Deferred Questions

These questions are intentionally not blockers for v1.

1. Should there be an official extension index for search, trust tiering, and
   discoverability?
2. How and when should `kongctl` check for extension updates without adding
   surprising latency or background network traffic?
3. Should a future trusted or linked-development mode support generated
   metadata through an optional runtime command such as `__kongctl describe`?
4. Should `kongctl` eventually provide a direct debug runner such as
   `kongctl run extension <name> [args...]`?

## Final Recommendation

`kongctl` should preserve its verb-first command model and implement extensions
as managed external command contributions.

The concrete v1 recommendation is:

1. preserve `kongctl <verb> ...`
2. support extension-contributed `command_paths`
3. allow those command paths to land under `get` or `list`, or to define a new
   verb through the first path segment
4. represent command paths as arrays of segment objects with optional
   per-segment aliases
5. visibly label extension-contributed command paths in help, completion, and
   inspection output
6. use `extension.yaml` with `schema_version` for package, runtime, and command
   metadata
7. validate command metadata from the manifest instead of requiring
   `__kongctl describe`
8. load cached manifest metadata and register synthetic Cobra commands for dispatch,
   help, and completion
9. invoke extensions as child processes
10. pass runtime context through `KONGCTL_EXTENSION_CONTEXT`
11. store `context.json` at a temporary runtime path
12. keep secrets out of the runtime context
13. make nested `kongctl` subprocesses session-aware
14. parse global flags in the host and pass `remaining_args` to the extension
15. follow the GitHub CLI install model: release artifact first, source clone
    fallback only for already-runnable script or binary extensions, and no
    source compilation during install
16. store manifest/runtime hashes and verify installed runtime hashes before
    execution
17. store installed extension state under the existing kongctl config home
18. use `kongctl api` and small machine-friendly helpers as the standard
    low-level v1 host callback surface
19. ship one script example and one Go example
20. ship extension install, remove, list, inspect, upgrade, and link flows
21. validate subprocess performance before locking the transport
22. add a `kongctl-extension-builder` skill for coding-agent-assisted
    authoring

This design keeps the CLI recognizable, supports both script and Go extension
authors, and avoids over-committing to a heavy plugin architecture before the
real workload is better understood.

## Sources

Primary sources reviewed for this report:

- GitHub CLI:
  [Using GitHub CLI extensions](https://docs.github.com/en/github-cli/github-cli/using-github-cli-extensions)
- GitHub CLI:
  [Creating GitHub CLI extensions](https://docs.github.com/en/github-cli/github-cli/creating-github-cli-extensions)
- GitHub CLI manual:
  [`gh extension`](https://cli.github.com/manual/gh_extension)
- GitHub CLI manual:
  [`gh extension install`](https://cli.github.com/manual/gh_extension_install)
- Kubernetes:
  [Extend kubectl with plugins](https://kubernetes.io/docs/tasks/extend-kubectl/kubectl-plugins/)
- Krew:
  [Writing Krew plugin manifests](https://krew.sigs.k8s.io/docs/developer-guide/plugin-manifest/)
- Helm:
  [The Helm Plugins Guide](https://helm.sh/docs/topics/plugins/)
- Helm:
  [`helm plugin install`](https://helm.sh/docs/helm/helm_plugin_install/)
- Heroku:
  [Using CLI Plugins](https://devcenter.heroku.com/articles/using-cli-plugins)
- Heroku:
  [Developing CLI Plugins](https://devcenter.heroku.com/articles/developing-cli-plugins)
- `oclif`:
  [Plugins](https://oclif.io/docs/plugins/)
- `oclif`:
  [Hooks](https://oclif.io/docs/hooks/)
- Salesforce CLI:
  [Overview of Salesforce CLI Plugins](https://developer.salesforce.com/docs/platform/salesforce-cli-plugin/guide/conceptual-overview.html)
- Salesforce CLI:
  [Use Libraries](https://developer.salesforce.com/docs/platform/salesforce-cli-plugin/guide/use-libraries.html)
- Salesforce blog:
  [New Signature Validation in Salesforce CLI Plugins](https://developer.salesforce.com/blogs/2017/10/salesforce-dx-cli-plugin-update)
- Terraform:
  [How Terraform works with plugins](https://developer.hashicorp.com/terraform/plugin/how-terraform-works)
- Terraform:
  [Manage Terraform plugins](https://developer.hashicorp.com/terraform/cli/plugins)
- HashiCorp:
  [`go-plugin`](https://github.com/hashicorp/go-plugin)
- Docker client plugin example:
  [Use the CLI](https://docs.docker.com/dhi/how-to/cli/)
- Docker CLI plugin design:
  [CLI Plugins Design](https://github.com/docker/cli/issues/1534)
- Docker CLI plugin metadata package:
  [metadata package](https://pkg.go.dev/github.com/docker/cli/cli-plugins/metadata)
- Docker engine plugin install:
  [`docker plugin install`](https://docs.docker.com/reference/cli/docker/plugin/install/)
- Vercel:
  [`vercel integration`](https://vercel.com/docs/cli/integration)
- Vercel:
  [Create an Integration](https://vercel.com/docs/integrations/create-integration)
- Fly.io:
  [`fly extensions`](https://fly.io/docs/flyctl/extensions/)
- Fly.io:
  [Extensions Program](https://fly.io/docs/about/extensions/)
- Railway:
  [CLI](https://docs.railway.com/cli)
- Supabase:
  [Supabase CLI getting started](https://supabase.com/docs/guides/local-development/cli/getting-started)
- Go stdlib:
  [`plugin` package docs](https://go.dev/pkg/plugin/?m=old)
- `wazero`:
  [Docs](https://wazero.io/docs/)
- `Extism`:
  [Overview](https://extism.org/docs/overview/)
