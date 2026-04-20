# CLI Extension Design Research Report

Reviewed on 2026-04-20.

## Purpose

This document summarizes research into how peer command line tools support
plugins, extensions, and related ecosystem mechanisms. It then maps those
findings onto `kongctl` and recommends a direction for a future extension
system.

This is a design report, not an implementation plan. The goal is to answer
these questions:

1. How do peer CLIs approach extensibility?
2. Which approaches appear to work well for end users?
3. Which approaches appear to work well for extension authors?
4. What security and governance patterns are available?
5. What should `kongctl` do first, and what should it avoid?

## Executive Summary

The strongest first-generation extension model for `kongctl` is a managed,
additive, external-command system with a stable extension manifest and an
extension-facing capability surface.

In practical terms, that means:

1. `kongctl` should support user-installable extensions that add new commands,
   such as `kongctl foo` or `kongctl foo bar`.
2. These extensions should run as separate executables or scripts, not as
   in-process dynamic libraries.
3. `kongctl` should install and manage them in its own extension directory
   instead of relying only on raw `PATH` scanning.
4. `kongctl` should resolve configuration, profiles, auth, output mode, and
   log level before invoking the extension, then expose those values through a
   stable contract.
5. `kongctl` should publish an extension manifest format with compatibility,
   checksums, signatures, publisher identity, and declared capabilities.
6. `kongctl` should not allow extensions to override core commands in v1.
7. `kongctl` should defer more invasive models such as lifecycle hooks into
   core commands, RPC plug-ins, or WASM sandboxes until real use cases justify
   the extra complexity.

The main reason for this recommendation is that the most successful and
practical peer systems, especially GitHub CLI, `kubectl` plus Krew, and Helm,
all start from additive command extensions with explicit install tooling and
metadata. The deeper plugin platforms, such as Heroku and Salesforce CLI, are
powerful, but they require more framework investment, tighter API discipline,
and a more stable host runtime than `kongctl` likely wants for a first step.

## Important Caveat About "Community Sentiment"

This research emphasizes primary documentation and source-level design details.
It does not attempt a broad social-media sentiment study, issue-mining
exercise, or ecosystem survey of forum posts and conference talks.

As a result:

- Claims in this document about how a system works are evidence-based.
- Claims about whether something is "considered good" are inferential.

Those inferences are based on factors such as:

- longevity of the extension model
- scope of first-party investment
- size and maturity of the visible plugin ecosystem
- whether the host project itself recommends the model
- whether the docs emphasize safety, compatibility, and authoring support

Where the evidence is inferential, this document says so explicitly.

## `kongctl` Goals And Constraints

The intended `kongctl` extension system should satisfy several goals:

1. Extension authors should be able to add CLI behavior without forking
   `kongctl`.
2. Extension authors should have access to core `kongctl` concepts, including
   effective configuration, profile selection, output preferences, logging,
   auth context, and Konnect access.
3. End users should have a simple install and upgrade experience.
4. Users should be able to distinguish official or supported extensions from
   community or unverified ones.
5. The system should not freeze `kongctl` internals or force external code to
   depend on `internal/...` packages.
6. The system should be realistic for a Go-based CLI with a Cobra command tree.

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

That is much safer than trying to let extensions mutate the core command tree
or inject behavior into existing command paths immediately.

### The current `skills/` mechanism is not a CLI extension model

The repository already contains a `skills/` directory, but it is clearly aimed
at AI coding agents rather than end-user CLI extensibility.

- [`skills/README.md`](../skills/README.md) describes these as human-maintained
  skills for agent tooling.
- [`skills/embed.go`](../skills/embed.go) embeds built-in skills as assets.

That matters because issue #826 should not conflate the two concepts:

- AI agent skills are documentation and prompt assets.
- CLI extensions are runtime command extensions for end users.

The report therefore treats them as separate layers.

## Evaluation Criteria

The peer systems were evaluated against the following dimensions.

### End-user experience

- How easy is installation?
- How easy is discovery?
- Can users update and remove extensions predictably?
- Is the command naming intuitive?

### Extension author experience

- Can authors write extensions in many languages?
- Is there a scaffold or generator?
- Is local development easy?
- Are there helper libraries for host concepts?

### Host integration depth

- Can extensions read effective config?
- Can they reuse auth and API clients?
- Can they integrate with host output and logging?
- Can they hook into command lifecycle events?

### Security and governance

- Is there signature or checksum verification?
- Is there an official registry or curated index?
- Can users understand what they are installing?
- Can organizations restrict or approve extension sources?

### Compatibility and maintenance

- Does the host need to freeze internal APIs?
- Is the extension contract versioned?
- Can the host evolve without breaking all extensions?

### Fit for `kongctl`

- Does the model fit a Go CLI?
- Does it fit a Cobra command structure?
- Does it support configuration, auth, output, and Konnect usage well?
- Is it realistic to build incrementally?

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

## Recommended Direction For `kongctl`

## Summary Conclusion

`kongctl` should implement a managed external-command extension system first,
with:

- additive command dispatch
- manifest-driven installation
- strong compatibility metadata
- checksums and signatures
- declared capabilities
- a stable extension-facing context contract
- an official Go helper SDK for richer integration

This recommendation is supported by the following facts:

1. GitHub CLI proves that additive external commands can be easy to author and
   easy to install.
2. `kubectl` and Krew prove that executable plugins need an install and
   metadata layer.
3. Helm proves that signatures and manifests substantially improve the model.
4. Salesforce proves that extension authors need stable host-facing libraries.
5. Terraform proves that deeper contracts are possible, but are heavier and
   better saved for later.
6. The current `kongctl` command tree is static, which makes additive fallback
   dispatch much safer than deep runtime mutation.

## Recommended v1 Scope

The first extension release should include only these capabilities.

### 1. Additive command extensions only

Extensions may add new command roots or subcommands under their own owned root.

Allowed examples:

- `kongctl foo`
- `kongctl foo list`
- `kongctl foo create`

Disallowed examples in v1:

- `kongctl get foo`
- `kongctl delete foo`
- overriding `kongctl login`
- injecting hooks into `kongctl apply`

This mirrors the safest peer pattern.

### 2. Managed install and link workflow

`kongctl` should own these commands:

- `kongctl extension install`
- `kongctl extension remove`
- `kongctl extension list`
- `kongctl extension inspect`
- `kongctl extension upgrade`
- `kongctl extension link`

Optional later additions:

- `kongctl extension search`
- `kongctl extension doctor`

### 3. Extension manifest

Every installable extension should include a manifest.

Recommended fields:

```yaml
apiVersion: kongctl.konghq.com/v1alpha1
kind: Extension
metadata:
  name: foo
  publisher: example-inc
  version: 0.1.0
  summary: Manage Foo resources with kongctl
spec:
  commands:
    - path: ["foo"]
      description: Manage Foo resources
  runtime:
    type: executable
    entrypoint: kongctl-foo
    platforms:
      - os: linux
        arch: amd64
        uri: https://example.invalid/foo-linux-amd64.tar.gz
        sha256: "..."
  compatibility:
    extensionApiVersion: v1alpha1
    minKongctlVersion: 0.20.0
    maxKongctlVersion: 0.x
  capabilities:
    - config.read
    - profile.read
    - konnect.read
    - konnect.write
    - output.structured
    - log.write
  trust:
    signature:
      provider: sigstore
      identity: example-inc
```

Important design notes:

- the manifest should describe command paths, not just binary names
- compatibility should version the extension API separately from the CLI
- capabilities should be explicit even if v1 enforcement is mostly policy and
  disclosure rather than hard sandboxing

### 4. Resolved context contract

The host should resolve effective configuration before executing the extension.
That includes:

- selected profile
- effective config after file, flag, env, and default precedence
- output format
- log level
- CLI version
- current working directory
- active config file path
- Konnect auth material needed to create clients

This should be exposed through a stable contract such as:

- an environment variable pointing to a context JSON file
- a small set of stable environment variables for common fields

Example:

```json
{
  "extensionApiVersion": "v1alpha1",
  "kongctlVersion": "0.20.0",
  "profile": "default",
  "output": "json",
  "logLevel": "debug",
  "configFile": "/home/me/.config/kongctl/config.yaml",
  "effectiveConfig": {
    "konnect": {
      "base_url": "https://us.api.konghq.com"
    }
  },
  "auth": {
    "type": "pat",
    "tokenEnv": "KONGCTL_EXTENSION_TOKEN"
  }
}
```

This is an important design choice. The extension should receive effective
values, not parse `kongctl` configuration sources for itself.

That is how `kongctl` can give authors access to config, flags, and env-var
resolution without exposing internal implementation packages.

### 5. Official Go helper SDK

`kongctl` should publish an extension-facing Go SDK or helper module.

This SDK should:

- load the extension context
- construct a preconfigured Konnect client
- construct a preconfigured `http.Client`
- expose a logger aligned with `kongctl` expectations
- provide helpers for writing structured output

This should be a public, stable surface. It should not require importing
`internal/...`.

This recommendation is directly supported by peer systems:

- GitHub has `go-gh`
- Kubernetes has `cli-runtime`
- Salesforce has `@salesforce/core`

### 6. Output contract

`kongctl` should support two output modes for extensions.

#### Plain output mode

Extensions print their own human text.

Pros:

- simplest possible script experience

Cons:

- inconsistent JSON, YAML, and table behavior

#### Structured output mode

Extensions can return a machine-readable output object and let `kongctl`
render it according to `--output`.

Pros:

- consistent host rendering
- better integration with scripting

Cons:

- requires a small schema

Recommended approach:

- allow plain output immediately
- add structured output support in the official SDK

### 7. Logging contract

Recommended rules:

- stdout is reserved for command output
- stderr is reserved for logs and diagnostics
- the context carries the active log level
- the official SDK emits logs in a style compatible with `kongctl`

This keeps shell scripts simple while still allowing rich logging for compiled
extensions.

## Security And Trust Model

This is where `kongctl` should be more opinionated than many peers.

### What can be improved in v1

1. Checksums should be required for remote binary assets.
2. Signatures should be supported for official and verified publishers.
3. Extensions should declare capabilities.
4. The install command should show publisher, source, version, checksum, and
   trust state before proceeding.
5. Organizations should be able to restrict install sources.
6. `kongctl` should support an "official only" policy mode.

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

At minimum:

- `official`
- `verified`
- `community`
- `unsigned`

These are product signals, not security absolutes, but they help users make
better choices.

### Recommended policy controls

Possible config and policy features:

- disable all extensions
- allow only official extensions
- allow only signed extensions
- allow only extensions from a configured index
- allowlist specific extension IDs
- denylist specific publishers

Terraform and Salesforce are the strongest inspiration here.

## Developer Experience Recommendations

`kongctl` should make extension development feel deliberate, not accidental.

### Minimum author tooling

1. `kongctl extension create foo`
2. `kongctl extension link .`
3. `kongctl extension inspect foo`
4. sample manifest templates
5. sample shell extension
6. sample Go extension using the helper SDK

### Why shell support matters

Many peer systems succeed because the easiest path is not blocked. GitHub CLI
and `kubectl` both benefit from the fact that small automation tasks can start
as shell scripts.

That means `kongctl` should not force authors into a Go-only model.

### Why a Go helper SDK still matters

Once extensions need to:

- authenticate to Konnect
- reuse retry behavior
- emit structured output
- integrate with host logging

compiled extensions with a helper SDK become much better than shell scripts.

So the best design is not "shell or SDK". It is "shell first if you want, SDK
when you need it".

## End-User Experience Recommendations

The user experience should be boring and predictable.

Recommended rules:

1. `kongctl` owns extension installation.
2. Core commands always win over extensions.
3. Extension command collisions are rejected.
4. Upgrades are explicit, not silent.
5. Local development links are clearly marked in `list` and `inspect`.

Example flow:

```text
kongctl extension install kong/konnect-foo
kongctl foo list
kongctl extension inspect foo
kongctl extension upgrade foo
kongctl extension remove foo
```

## Should `kongctl` Support Multiple Extension Points?

Yes, but in phases.

The recommended model is:

### Lane 1: Command extensions

This is the default and should satisfy most early use cases.

Good for:

- new resource workflows
- organization-specific utilities
- automation helpers
- internal team tooling

### Lane 2: Rich helper SDKs

This is still the same command-extension lane, but with richer tooling for
compiled extensions.

Good for:

- deeper Konnect integrations
- better output and diagnostics
- shared auth and retry behavior

### Lane 3: Advanced isolated or typed plugin systems

Only add this if there is clear demand for:

- deeper hook points
- stronger isolation
- versioned service-style contracts

Candidates here are:

- RPC/gRPC plugin contracts
- WASM with `wazero`

This phased model addresses both user needs described in issue #826:

- people who want to extend with shell scripts
- people who want to write real code against a host-supported API

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
extension API surface instead of accidentally exporting internals.

## Proposed Non-Goals For v1

The first release should explicitly not attempt all of the following.

1. No extension overrides of core commands.
2. No host lifecycle hooks such as "run before every command".
3. No mutation of existing core verbs such as `get`, `delete`, `apply`, or
   `plan`.
4. No install hooks.
5. No background daemons started by extensions during installation.
6. No implicit marketplace or registry dependence for local development.
7. No promise that executable extension capabilities are strongly sandboxed.

Writing these non-goals down will prevent the first implementation from growing
into a framework before the basic product loop is proven.

## Suggested Phased Roadmap

## Phase 1: Research-to-prototype

- define manifest schema
- define install location and naming rules
- define extension context contract
- build fallback dispatch from root command
- ship `install`, `list`, `remove`, `link`
- support local path install and signed remote release install

## Phase 2: Author experience

- ship `create` scaffolding
- publish example shell extension
- publish example Go extension
- publish official Go helper SDK
- add `inspect`

## Phase 3: Trust and policy

- add signatures and trust tiers
- add org policy controls
- add allowlist and official-only modes
- add richer compatibility checks

## Phase 4: Richer integration

- add structured output helpers
- add `doctor`-style diagnostics for extensions
- consider a machine API for host-assisted operations if context-only proves
  insufficient

## Phase 5: Advanced extension lanes

- evaluate RPC plugins for deeper service contracts
- evaluate WASM for stronger isolation
- only add lifecycle hooks if concrete use cases justify them

## Open Questions

These questions should be resolved before implementation begins.

1. Should extension commands be limited to a single top-level root in v1?
2. Should the official install source be a GitHub release convention, an
   index file, or both?
3. Which signature mechanism should be preferred?
4. Should `kongctl` expose a host machine API immediately, or wait to see
   whether context files and the Go SDK are sufficient?
5. Should output rendering be host-owned from day one, or should extensions
   own all rendering until a later phase?
6. How much of the Konnect API should be exposed as typed helpers versus
   generic REST calls?
7. Should `kongctl` support extension execution in CI when policy requires
   "official only" or "signed only" modes?

## Final Recommendation

If `kongctl` wants an extension system soon, it should not try to match the
full power of Salesforce CLI or Terraform on day one.

Instead, it should adopt the most successful common pattern in the ecosystem:

- additive command extensions
- managed installation
- manifest-driven metadata
- trust signals
- stable extension-facing helper APIs

That path is:

- easier to explain
- easier to secure incrementally
- easier to support
- easier to evolve
- compatible with both shell-based and compiled extension authors

The deeper lesson from the peer systems is not that there is one perfect plugin
architecture. It is that the host should begin with the smallest extension
surface that is genuinely useful, then expand only when real use cases demand
it.

For `kongctl`, that smallest useful surface is a managed command-extension
system with a strong manifest and a stable capability contract.

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
