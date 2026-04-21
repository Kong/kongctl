# CLI Extension Design Research Report

Reviewed on 2026-04-21.

## Summary

This document recommends a concrete extension design for `kongctl`, explains
the reasoning for that design, and then records the supporting peer CLI
research and previously considered alternatives.

The document is intentionally front-loaded:

1. summary and design decision first
2. detailed design and defense of the decision second
3. peer research and earlier design explorations last

The goal is that a reader can understand the proposed plan from the first page
without reading the full research section.

## TL;DR

`kongctl` should add an extension system that lets installed extensions
contribute new `kongctl` command paths. An extension should be able to add
commands such as `kongctl get foo`, `kongctl list foo`, or a new verb such as
`kongctl promote foo`, while preserving the normal `kongctl <verb> ...`
command shape.

For users, this means they can install an extension from a local path or a
GitHub repository and then use the new command path as if it were part of the
CLI. The extension should show up in help and inspection output, follow the
same command grammar as the rest of `kongctl`, and be managed with normal
verb-first lifecycle commands such as `install`, `upgrade`, `list`, and
`uninstall`.

Technically, each extension is a separately executed script or binary described
by `extension.yaml`. During install or link, `kongctl` runs the extension in a
reserved `__kongctl describe` mode to collect command metadata, validate
conflicts, and cache help data. During normal execution, `kongctl` launches the
extension as a child process and passes invocation context through
`KONGCTL_EXTENSION_SESSION_DIR` and `context.json`, so nested `kongctl`
subprocesses can reuse the same profile, base URL, and other resolved settings
without in-process plugins.

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
| Manifest | Simple `extension.yaml` for package metadata |
| Command metadata | Runtime descriptor via `__kongctl describe` |
| Runtime model | Managed external child process |
| Runtime context transport | `KONGCTL_EXTENSION_SESSION_DIR` |
| Runtime context file | `context.json` |
| Nested host callbacks | Re-enter `kongctl` as a subprocess |
| v1 Go library | Thin wrapper, not a large host SDK |
| Secrets in context | Never include them |
| Cleanup | Best-effort immediate cleanup plus stale-session reaping |
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

An extension should describe its contributed commands as `command_paths`.

Examples:

- `[get, foo]`
- `[list, foo]`
- `[promote, foo]`

This is simpler than splitting the model into separate `commands` and `verbs`.
Whether a contribution lands under an existing verb or defines a new verb is
determined entirely by the first segment of the command path.

### 3. Open Only A Narrow Set Of Existing Verbs In v1

The initial extension surface should be intentionally selective.

Recommended v1 policy:

- open existing verbs for command contributions: `get`, `list`
- allow custom verbs

All other existing verbs should be treated as closed to extension in v1 unless
explicitly revisited later. This preserves room for future hooks without
committing to them early.

### 4. Treat One Extension As A Bundle Of Command Paths

One extension should be able to contribute many command paths. The extension
should be the installation unit, while the extension runtime should describe
the set of command paths it owns.

This lets one extension support a full resource family rather than forcing
many small install units.

### 5. Use A Simple YAML Manifest For Package Metadata

The manifest should be a plain `extension.yaml` file, and it should describe
package, install, trust, and compatibility metadata only.

It should not be the source of truth for command help, usage text, args, or
flags.

Recommended shape:

```yaml
schema_version: 1

name: foo
publisher: kong
version: 0.1.0
summary: Foo resource support for kongctl

runtime:
  type: executable
  command: kongctl-ext-foo

compatibility:
  min_version: 0.20.0
  max_version: 0.x

capabilities:
  - config_read
  - api
  - structured_output

distribution:
  source: github
  repository: kong/kongctl-ext-foo

trust:
  tier: official
```

### 6. Use A Runtime Describe Contract For Command Metadata

All command metadata should come from the extension runtime itself.

That includes:

- `command_paths`
- summaries
- descriptions
- usage text
- examples
- args
- flags

This avoids splitting command metadata between the manifest and the extension
implementation.

The host should invoke the extension using a reserved internal argument
contract, for example:

```text
kongctl-ext-foo __kongctl describe
```

The extension should return structured JSON on stdout. Example:

```json
{
  "command_paths": [
    {
      "id": "get_foo",
      "path": ["get", "foo"],
      "summary": "Get Foo resources",
      "description": "Retrieves Foo resources from Konnect.",
      "usage": "kongctl get foo [name] [flags]",
      "examples": [
        "kongctl get foo",
        "kongctl get foo my-foo --output json"
      ],
      "args": [
        {
          "name": "name",
          "required": false,
          "repeatable": false,
          "description": "Optional Foo resource name."
        }
      ],
      "flags": [
        {
          "name": "filter",
          "type": "string",
          "description": "Filter Foo resources by label."
        }
      ]
    },
    {
      "id": "promote_foo",
      "path": ["promote", "foo"],
      "summary": "Promote Foo resources"
    }
  ]
}
```

`kongctl` should validate this descriptor at install or link time, then cache
it for help, completion, inspection, and collision checks.

This design cleanly separates concerns:

- argv selects the extension control operation such as `describe`
- environment variables carry ambient runtime context such as
  `KONGCTL_EXTENSION_SESSION_DIR`

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

1. the repository must contain `extension.yaml`
2. if the repo publishes a compatible release asset for the current platform,
   prefer downloading that asset
3. if the extension is a script-based extension without release assets, clone
   the repository at the selected ref into the managed extension home
4. record the source and installed version so `upgrade` can repeat the same
   strategy later

This gives `kongctl` a clear v1 install story:

- explicit local path installs
- explicit GitHub repo installs
- no ambient PATH discovery
- no broad marketplace search requirement in v1

### 8. Pass Runtime Context Through An Inherited Environment Variable

The parent `kongctl` process should resolve invocation-bound state, write a
machine-generated `context.json` into a temporary session directory, and pass
that directory to the child through:

```text
KONGCTL_EXTENSION_SESSION_DIR=/path/to/session-dir
```

This is preferable to:

- positional JSON arguments
- hidden bootstrap flags
- raw JSON embedded directly in environment variables

The session directory should hold at least:

- `context.json`

Future transport upgrades can add additional files such as `host.sock` without
breaking the bootstrap contract.

### 9. Keep Secrets Out Of The Runtime Context

The runtime context should include resolved invocation state such as:

- matched command path
- selected profile
- resolved base URL
- output mode
- log level
- config file path
- remaining args
- active session metadata

It should not include:

- tokens
- refresh credentials
- copied secrets from the host environment

### 10. Make Nested `kongctl` Calls Session-Aware

When an extension runs `kongctl api ...` or `kongctl get config <field>`, the
nested `kongctl` subprocess should inherit
`KONGCTL_EXTENSION_SESSION_DIR`, reload `context.json`, and bootstrap itself
using the same resolved invocation state.

That means the child does not need to replay:

- `--profile`
- base URL overrides
- config file selection
- other session-bound settings

This is the key design point that makes CLI-first callbacks workable.

### 11. Use A CLI-First Host Callback Model In v1

For v1, the main host callback surface should be the `kongctl` CLI itself.

The most important existing host callback is:

- `kongctl api`

This is a strong foundation because it already supports:

- arbitrary Konnect API calls
- structured JSON output
- built-in jq filtering

Additional machine-friendly helper commands should be added where necessary,
especially:

- `kongctl get config <field>`
- `kongctl version --json`

### 12. Keep The v1 Go Library Thin

The v1 Go support should be a small helper library that lives in this
repository and wraps:

- loading `context.json`
- locating the `kongctl` binary
- running session-aware `kongctl` subprocesses
- decoding JSON output

It should not be a large in-process host SDK in v1.

### 13. Add Cleanup And Recursion Protection From The Start

Because the runtime model writes temporary session files, the implementation
must be disciplined:

- remove the session directory on normal exit
- perform opportunistic stale-session cleanup on future runs
- keep session files in a runtime or temp location, not the permanent config
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
2. `kongctl api` already provides a useful host callback surface
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

This suggests that the safest first extension implementation is:

- resolve built-in commands first
- then fall back to extension lookup
- only then return an unknown-command error

### The current `skills/` mechanism is not a CLI extension model

The repository already contains a `skills/` directory, but it is clearly aimed
at AI coding agents rather than end-user CLI extensibility.

- [`skills/README.md`](../skills/README.md) describes these as human-maintained
  skills for agent tooling
- [`skills/embed.go`](../skills/embed.go) embeds built-in skills as assets

That matters because issue #826 should not conflate the two concepts:

- AI agent skills are documentation and prompt assets
- CLI extensions are runtime command extensions for end users

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

GitHub CLI is the best single precedent for a first-generation `kongctl`
command-extension model.

`kongctl` should borrow these specific ideas:

- additive commands, not core overrides
- script or compiled binary support
- local link/install flow for development
- scaffolding for authors
- stable helper APIs rather than internal package exposure

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

Docker's privilege prompt is also a good reminder that capability disclosure is
useful even when it is not a perfect sandbox.

## Vercel

### How it works

Vercel has an `integration` CLI command, but this is not a general local
plugin system in the same sense as `gh`, `kubectl`, or Helm
([docs](https://vercel.com/docs/cli/integration)).

Vercel integrations are marketplace and provider constructs:

- they provision and manage provider resources
- they can expose SSO flows
- they involve product definitions, provider APIs, and review processes
- provider integrations are created through Vercel's integration platform
  ([docs](https://vercel.com/docs/integrations/create-integration))

### Strengths

1. Strong governance.
2. Good marketplace and provider workflow.
3. Better fit for platform partnerships than local arbitrary code execution.

### Weaknesses

1. Not a general model for end-user command extensions.
2. Higher approval and provider burden.
3. Solves a different problem from local CLI extensibility.

### Lessons for `kongctl`

Vercel is useful mainly as evidence that "integration" can mean marketplace
provisioning, not local CLI plugin execution.

That matters because `kongctl` may eventually want:

- extension metadata
- support tiers
- official provider partnerships

But those are likely later ecosystem concerns, not the first local extension
mechanism.

## Fly.io

### How it works

Fly.io documents `fly extensions`, but this is a first-party or partner-facing
platform extension surface, not a general local plugin runtime
([CLI docs](https://fly.io/docs/flyctl/extensions/),
[program docs](https://fly.io/docs/about/extensions/)).

Fly's provider requirements focus on:

- resource provisioning
- account and organization mapping
- SSO
- billing detail exchange

That is much closer to a marketplace integration program than a local command
plugin system.

### Strengths

1. Strong governance and provider alignment.
2. Good fit for managed service partnerships.
3. Clear product-level requirements.

### Weaknesses

1. Not a model for arbitrary local user-authored CLI extensions.
2. More closed and partnership-oriented.

### Lessons for `kongctl`

Like Vercel, Fly shows that many SaaS CLIs prefer tightly governed
provider-extension programs over open local plugin execution.

That is useful as a strategic reminder:

- open plugin ecosystems increase support and security burden
- not every product chooses to expose one

## Railway

### How it works

Railway's official CLI documentation exposes commands, global options, upgrade
flows, and project interactions, but it does not document a general local
plugin or extension model
([docs](https://docs.railway.com/cli)).

### Strengths

1. Simpler product surface.
2. Lower governance burden.

### Weaknesses

1. No obvious extensibility path for third-party command authors.

### Lessons for `kongctl`

Railway is useful primarily as evidence that many modern SaaS CLIs choose not
to offer general local extensibility at all.

## Supabase

### How it works

Supabase's CLI docs cover local development, deployment, configuration, and
project management, but they do not document a general local plugin system
([docs](https://supabase.com/docs/guides/local-development/cli/getting-started)).

### Strengths

1. Product simplicity.
2. Lower extension-support burden.

### Weaknesses

1. No first-class extension ecosystem for local CLI behavior.

### Lessons for `kongctl`

Supabase reinforces the point that a general plugin model is optional, not
inevitable. If `kongctl` opens this surface, it should do so intentionally and
with governance.

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
top-level extension commands. The core design choice is:

1. allow extension-contributed `command_paths`
2. allow those command paths to land under the open existing verbs `get` and
   `list`, or to define a new verb naturally through the first path segment
3. use a simple `extension.yaml` manifest for package metadata only
4. require the extension runtime to provide command metadata through a reserved
   argv contract such as `__kongctl describe`
5. pass runtime context through `KONGCTL_EXTENSION_SESSION_DIR`
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

The first extension release should include only these capabilities.

### 1. Command Paths

Allowed in v1:

- `command_paths` whose first segment is `get` or `list`
- `command_paths` whose first segment is a new verb not claimed by a built-in
  command

Disallowed in v1:

- overriding built-in commands
- collisions with built-in resources or other extensions
- `command_paths` under any existing verb other than `get` or `list`
- general host lifecycle hooks

This lets `kongctl` preserve its existing grammar without opening the most
dangerous integration points too early.

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
3. after install or link, `kongctl` should invoke the extension with
   `__kongctl describe`
4. the returned command descriptor should be validated and cached
5. GitHub repo installs require `extension.yaml`
6. for binary extensions, prefer compatible release assets when present
7. for script extensions, clone the repo into the managed extension home
8. store the install source and ref so `upgrade` can reuse the same strategy

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
  type: executable
  command: kongctl-ext-foo

compatibility:
  min_version: 0.20.0
  max_version: 0.x

capabilities:
  - config_read
  - api
  - structured_output

distribution:
  source: github
  repository: kong/kongctl-ext-foo

trust:
  tier: official
```

Important design notes:

- `schema_version` is simpler than Kubernetes-style object metadata
- compatibility should express the supported `kongctl` version range without
  repeating `kongctl` in every field name
- capabilities should be explicit even if enforcement is mostly governance in
  v1
- command metadata should not be duplicated here

### 5. Runtime Command Descriptor Contract

All command metadata should come from the extension runtime, not the manifest.

That includes:

- `command_paths`
- summaries
- descriptions
- usage text
- examples
- args
- flags

The host should invoke the extension using a reserved internal argument
contract:

```text
kongctl-ext-foo __kongctl describe
```

The extension should return structured JSON on stdout. Example:

```json
{
  "command_paths": [
    {
      "id": "get_foo",
      "path": ["get", "foo"],
      "summary": "Get Foo resources",
      "description": "Retrieves Foo resources from Konnect.",
      "usage": "kongctl get foo [name] [flags]",
      "examples": [
        "kongctl get foo",
        "kongctl get foo my-foo --output json"
      ],
      "args": [
        {
          "name": "name",
          "required": false,
          "repeatable": false,
          "description": "Optional Foo resource name."
        }
      ],
      "flags": [
        {
          "name": "filter",
          "type": "string",
          "description": "Filter Foo resources by label."
        }
      ]
    },
    {
      "id": "promote_foo",
      "path": ["promote", "foo"],
      "summary": "Promote Foo resources"
    }
  ]
}
```

`kongctl` should validate this descriptor at install or link time, reject any
restricted or colliding command paths, and cache the validated descriptor for:

- `help`
- completion
- `inspect extension`
- conflict checks

This avoids splitting command metadata between the manifest and the extension
implementation.

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
- active extension session metadata

The runtime contract should be:

- an inherited `KONGCTL_EXTENSION_SESSION_DIR`
- pointing at a session directory
- containing a machine-generated `context.json`

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
    "config_file": "/home/me/.config/kongctl/config.yaml"
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

### 7. Session-Aware Nested `kongctl` Calls

Nested `kongctl` subprocesses should detect `KONGCTL_EXTENSION_SESSION_DIR`
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

### 8. CLI-First Host Callback Surface

The v1 host callback model should be the `kongctl` CLI itself.

Existing and proposed host callbacks:

- `kongctl api ...`
- `kongctl get config <field>`
- `kongctl version --json`

This is intentionally small. `kongctl api` already gives extensions a useful
authenticated Konnect API surface with structured output and jq filtering.

### 9. Thin Go SDK Library

The repository should include a thin Go SDK library for extension authors.

This is now an explicit v1 deliverable.

The SDK should:

- load the runtime context
- expose typed accessors for context fields
- wrap session-aware `kongctl` subprocess execution
- decode JSON results

It should not be a large in-process host API in v1.

### 10. Example Extensions

The repository should include at least two example extensions:

1. one script-based extension
2. one Go-based extension using the thin SDK

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
- wire in the thin Go SDK when appropriate
- test local install and link workflows

This will make the extension system much easier to use in practice, especially
for internal Kong contributors and users working with coding agents.

## Security And Trust Model

This is where `kongctl` should be more opinionated than many peers.

### What can be improved in v1

1. Checksums should be required for remote binary assets.
2. Signatures should be supported for official and verified publishers.
3. Extensions should declare capabilities.
4. The install command should show publisher, source, version, checksum, and
   trust state before proceeding.
5. Organizations should be able to restrict install sources.
6. `kongctl` should support an `official only` policy mode.

### What cannot be fully enforced in v1

If extensions run as arbitrary scripts or binaries on the user's machine, then
capability declarations are not a true sandbox. They are:

- disclosure
- policy metadata
- install-time risk communication
- enterprise policy hooks

They are not strong technical isolation.

That means `kongctl` must be explicit with users:

- executable extensions are trusted-code installation
- signatures verify identity and integrity, not safety

### Recommended trust tiers

- `official`
- `verified`
- `community`
- `unsigned`

### Recommended policy controls

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
7. thin Go SDK
8. `kongctl-extension-builder` skill

### Why shell support matters

Many peer systems succeed because the easiest path is not blocked. GitHub CLI
and `kubectl` both benefit from the fact that small automation tasks can start
as shell scripts.

That means `kongctl` should not force authors into a Go-only model.

### Why a thin Go SDK still matters

Once extensions need to:

- authenticate to Konnect
- reuse `kongctl api` and configuration helpers
- decode structured output
- standardize host re-entry

a thin helper library becomes meaningfully better than ad hoc shelling out.

The right v1 design is therefore:

- script-first if you want
- thin Go SDK when you need it

## End-User Experience Recommendations

The user experience should be boring and predictable.

Recommended rules:

1. `kongctl` owns installation, removal, upgrade, and inspection.
2. Built-ins always win over extensions.
3. Extension command path collisions are rejected.
4. Upgrades are explicit, not silent.
5. Local development links are clearly marked in `list` and `inspect`.

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
6. No promise that executable extension capabilities are strongly sandboxed.
7. No large in-process host SDK.

Writing these non-goals down will prevent the first implementation from growing
into a framework before the basic product loop is proven.

## Suggested Phased Roadmap

## Phase 1: Core Runtime And Install Flows

- finalize `extension.yaml` schema
- define command-path matching and collision rules
- define the runtime context contract
- implement `KONGCTL_EXTENSION_SESSION_DIR` bootstrap
- implement recursion guard
- implement `install`, `uninstall`, `list`, `inspect`, `upgrade`, and `link`
- support local path install and GitHub repo install

## Phase 2: Authoring Deliverables

- add the thin Go SDK library to this repository
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

- evaluate whether subprocess re-entry is fast enough in practice
- add richer structured output helpers if needed
- evaluate socket or RPC transport if the thin SDK needs a faster backend
- only add lifecycle hooks if concrete use cases justify them

## Open Questions

These questions should be resolved before implementation begins.

1. Should custom verbs be generally allowed, or should policy default them to
   `official` and `verified` extensions only?
2. Should GitHub repo installation always use a hybrid rule of release assets
   first and repo clone for script extensions, or should users choose
   explicitly?
3. Should there be an official extension index in v1, or should that wait
   until after direct GitHub and local installs are working?
4. What exact precedence rules should apply when nested session-aware helper
   commands specify explicit output or log flags?
5. What stale-session cleanup threshold is appropriate?
6. Should `kongctl get config <field>` return machine-oriented output by
   default when called within an extension session?
7. Which top-level command paths should be reserved so custom verbs cannot
   overlap with extension management, help, or other CLI-owned namespaces?

## Final Recommendation

`kongctl` should preserve its verb-first command model and implement extensions
as managed external command contributions.

The concrete v1 recommendation is:

1. preserve `kongctl <verb> ...`
2. support extension-contributed `command_paths`
3. allow those command paths to land under `get` or `list`, or to define a new
   verb through the first path segment
4. use a simple `extension.yaml` manifest with `schema_version` for package
   metadata only
5. require the extension runtime to provide command metadata through
   `__kongctl describe`
6. invoke extensions as child processes
7. pass runtime context through `KONGCTL_EXTENSION_SESSION_DIR`
8. store `context.json` in a temporary session directory
9. keep secrets out of the runtime context
10. make nested `kongctl` subprocesses session-aware
11. use `kongctl api` and small machine-friendly helpers as the v1 host
    callback surface
12. ship a thin Go SDK library in this repository
13. ship one script example and one Go example
14. ship extension install, remove, list, inspect, upgrade, and link flows
15. add a `kongctl-extension-builder` skill for coding-agent-assisted
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
