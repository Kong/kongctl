# Custom lint functions: evaluation and proposed plan

Status: proposal for maintainer review. This document does not authorize or
implement custom-function execution. Current behavior is documented in the
[linting guide](../lint.md).

## Scope and evidence

[Issue #2581](https://github.com/Kong/kongctl/issues/2581) asks for three
current compatibility limits to be documented and proposes evaluating functions
and declarative expansion. This change documents those limits; function
support and expansion remain separate follow-ups.

The evaluation targets vacuum **v0.30.6**, pinned in `go.mod`. Recheck these
findings when changing that dependency. Source paths below refer to the
[vacuum v0.30.6 source tree][vacuum].

- `internal/lint/lint.go` supplies no `CustomFunctions` to vacuum's motor.
  The command reads a local ruleset and raw input files, without invoking the
  declarative loader. No go-apiops change is needed to document this behavior
  or to pass custom functions to the motor.
- The original [lint PR #443](https://github.com/Kong/kongctl/pull/443) and its
  review discussion describe parity with decK and binary-size concerns. The
  reviewed discussion provides no explicit security rationale for omitting
  custom functions. That absence does not establish the original intent.
- Vacuum's motor accepts `map[string]model.RuleFunction` through
  `RuleSetExecution.CustomFunctions`; it also accepts `FetchConfig`. It does
  not expose a `CustomFunctionsDir` field in this version. The issue's
  automated triage should not be used as an API specification.

## Related implementations and recorded decisions

kongctl's lint engine calls vacuum directly; go-apiops is a dependency for
other functionality, including patching. decK likewise calls vacuum directly
in `lint/lint.go`, using go-apiops `filebasics` helpers for file I/O. Neither
client supplies `CustomFunctions` in its current motor invocation.

[decK PR #1312][deck-functions] proposed adding a custom-function directory
flag and vacuum plugin loading in May 2024. On May 31, rspurgeon requested
that it remain a draft pending discussion as a product feature, documentation
alignment, and other items. On August 21, hbagdi closed it for inactivity,
inviting reopening if needed. The PR was never merged.

This is evidence of deliberate feature deferral in decK, not an explicit
security prohibition. Its comments and inline reviews do not state that custom
functions were rejected for security reasons. The identified security risks
are findings from this evaluation, not an established historical rationale.
Describe the present behavior as unsupported or not wired up; "intentionally
disabled for security" is not supported by the reviewed evidence.

## Security and compatibility findings

### Loading functions executes code

Vacuum's `plugin/plugin_loader.go` scans a directory for both `.so` and `.js`
files. Directly reusing it would include native plugin loading. Invalid
JavaScript is printed to stdout and skipped, even with silence enabled.
kongctl needs JavaScript-only loading and returned errors so failed loading
cannot silently reduce governance coverage or corrupt JSON output.

The `plugin/javascript/js_plugin.go` implementation executes top-level code
during validation and schema discovery. Its contract uses `runRule` and
optionally `getSchema`; it is not a general Node.js/Spectral module runner.
Supporting this API would not establish full Spectral custom-function parity.

### JavaScript is not isolated from local files

`BuildVM` enables goja_nodejs's default `require` registry. Its default source
loader reads host files, including JSON modules. Restricting discovery to a
functions directory alone would not restrict reads initiated by `require`.
A local probe using `BuildVM` can load a synthetic JSON fixture by absolute
path outside a functions directory. Do not describe this VM as a sandbox for
untrusted code or run third-party functions in privileged CI jobs.

### Fetch adds outbound data access

The `plugin/javascript/eventloop/fetch` package provides `fetch()` during rule
execution, with HTTPS and private-network restrictions by default. Public
HTTPS requests remain permitted, including methods and bodies that could send
configuration data elsewhere. These defaults are not an offline policy.

The internal fetch configuration has host lists, but the public
`config.FetchConfig` passed through the motor does not expose those lists or
a disable-fetch switch. Passing that struct alone cannot enforce a host
allowlist. Ruleset inheritance and OpenAPI reference lookup use separate
network paths; a function-fetch policy would not control them.

### Timeouts do not cover all execution

The JavaScript implementation defaults to a five-second asynchronous rule
timeout. Script validation and top-level execution call `RunString` outside
the event loop. `plugin/javascript/eventloop/eventloop.go` invokes the function
before checking its context and does not interrupt the VM. An infinite loop
can therefore prevent completion. A timeout on the caller alone would not
terminate that work or impose a memory limit.

## Proposed implementation sequence

1. Confirm the trust model with maintainers before implementing support.
   Recommend explicitly supplied, reviewed local functions only. Decide
   whether trusted-code limitations are acceptable or whether execution must
   be isolated in a worker process with enforceable resource limits. Do not
   promise safe execution of untrusted repository content.
2. Add the required vacuum integration points, or a kongctl adapter
   implementing `model.RuleFunction`, before enabling the feature. Required
   controls are a restricted module loader, interruptible execution covering
   validation/schema discovery/rule calls, and an explicit fetch policy.
   The existing constructor builds its own VM, so these controls cannot be
   supplied simply by constructing `RuleSetExecution` differently.
3. Add an opt-in `--functions <local-directory>` to kongctl. Never discover
   executable code from the working directory, a remote ruleset, or a
   ruleset's `functionsDir` implicitly. Enumerate only `.js` files, reject
   native plugin files, enforce the module root across symlinks and traversal,
   and reject duplicate function names. Preserve the existing behavior when
   the flag is absent. Document the supported vacuum function contract.
4. Default function network access to disabled. If metadata lookup is approved,
   add explicit host selection with HTTPS, certificate verification, bounded
   requests/responses, and validation across redirects and DNS resolution.
   Keep private networks blocked initially; agree on a separately reviewed
   policy before enabling internal metadata services. Never pass Konnect PATs
   or profile credentials into a function's HTTP client automatically.
5. Load and validate functions once per invocation, then share the prepared
   mapping across file and stdin runs with safe runtime isolation. Return all
   load and engine execution errors through kongctl's error path. Runtime or
   fetch failures must fail lint independently of the selected violation
   severity threshold; audit `RuleSetExecutionResult.Errors`, currently not
   propagated by `Bytes`. Capture JavaScript console output and keep
   diagnostics off structured-output streams.
6. Add CLI help, user documentation, and focused regression tests with the
   implementation. Treat declarative expansion as a separate design because
   it changes input semantics, environment access, and source locations.

## Acceptance criteria for a future implementation

- Without `--functions`, no custom code is loaded and existing lint output
  and exit behavior are preserved.
- A valid local JavaScript function works for file, directory, multiple-file,
  and stdin inputs. The documented API example runs on CGO-disabled builds.
- Missing directories, unreadable files, invalid scripts, missing `runRule`,
  name collisions, and unknown functions fail clearly. No function is silently
  skipped. Load/runtime errors cannot become a successful lint run by changing
  `--fail-severity` or `--display-only-failures`.
- Native plugins and module reads outside the approved root are rejected,
  including symlink escapes. Verify with synthetic fixtures only.
- Tests demonstrate that validation, schema discovery, synchronous loops,
  asynchronous calls, and cancellation finish within enforced limits. Any
  remaining memory/resource limitations are explicit in the approved trust
  model and documentation.
- Function fetch is disabled by default. Tests cover permitted metadata
  requests, denied destinations, redirects, private addresses, timeouts,
  response limits, and credential isolation without depending on the public
  network.
- JSON and YAML output remain parseable on success and failure. Engine errors
  and function exceptions cannot silently remove checks from the result.

## Documentation coordination

The kongctl PR can ship the compatibility guide and CLI help immediately.
It should reference #2581 without closing the feature evaluation.

For the developer site, propose adding a "Spectral compatibility limits"
section before command usage in `app/kongctl/lint/index.md`, covering the same
three limits as `docs/lint.md`. Regenerate its CLI help include using the site's
normal workflow after the kongctl change is available. Keep the existing
vacuum explanation and worked example on the current upstream page.

Open a separate developer-site PR only after the maintainer confirms. No
developer-site or go-apiops files are changed by this evaluation; any future
changes there require separate PRs and confirmation before filing them.

[vacuum]: https://github.com/daveshanley/vacuum/tree/v0.30.6
[deck-functions]: https://github.com/Kong/deck/pull/1312
