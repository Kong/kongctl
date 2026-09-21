E2E Declarative Scenarios (MVP)

Overview

- Author end-to-end tests as text-only scenarios: a base input config, step overlays, a sequence of kongctl commands per step, and simple assertions against expected files.
- Assertions use the JSON stdout of a command (or a shorthand get) plus a selector and masking to compare against an expected file (optionally merged with small overlays).
- This document specifies the minimal schema and includes a concrete example
  mirrored from the scenario-first portal visibility flow.

Quickstart

- Place a scenario at `test/e2e/scenarios/<suite>/<name>/scenario.yaml`.
- Run all scenarios only: `make test-e2e-scenarios`.
- Run all AI Gateway scenarios: `make test-e2e-ai-gateway`.
- Run a single scenario or directory prefix:
  - `make test-e2e-scenarios SCENARIO=portal/visibility`
  - `make test-e2e-scenarios SCENARIO=ai-gateway`
  - or `KONGCTL_E2E_SCENARIO=test/e2e/scenarios/portal/visibility/scenario.yaml make test-e2e-scenarios`
- Save observed output to expected files (bootstrap/update):
  - `KONGCTL_E2E_UPDATE_EXPECT=1 make test-e2e-scenarios SCENARIO=portal/visibility`
- Artifacts: set `KONGCTL_E2E_ARTIFACTS_DIR=/tmp/kongctl-e2e` to choose a folder; otherwise a temp dir is created and printed at the end.

Replay-enabled scenarios

Before editing a scenario, check its replay eligibility and follow
the [refresh procedure](../replay-policy.md#refresh-existing-replay-coverage).
This applies to steps, inputs, overlays, and assertion/expected files.

Scenario Maturity

Scenarios are stable by default. Use the `test.maturity` field for scenarios
covering beta product areas:

```yaml
test:
  maturity: beta
```

Supported values are `stable` and `beta`. Unknown values fail scenario
preflight.

`KONGCTL_E2E_BETA_MODE` controls beta scenario execution:

- `fail` runs beta scenarios and treats failures as fatal. This is the default
  for local and ordinary manually dispatched runs.
- `warn` runs beta scenarios but reports their failures as advisory. Stable
  failures remain fatal. Required pull request, merge queue, trusted fork, and
  main-branch runs use this mode.
- `skip` skips beta scenarios before executing their steps.

Examples:

```sh
KONGCTL_E2E_BETA_MODE=warn make test-e2e-scenarios
KONGCTL_E2E_BETA_MODE=skip make test-e2e-scenarios
```

Warned beta failures preserve normal command artifacts and add a structured
record under `beta-failures/`. The harness then performs a best-effort captured
organization reset before continuing. Cleanup failures are reported as
additional warnings without replacing the original scenario error.

CI continues to use `scenario-results.txt` as the shard result artifact. It
adds `beta_failed_count` and a `[beta_failed]` section alongside the existing
passed, failed, and skipped results.

Key Concepts

- Scenario: Top-level test definition. Sets defaults for masking and retries.
- Step: Creates a working copy of the base inputs and applies one or more overlay directories before running commands.
- Command: Any kongctl invocation (apply, sync, diff, get, …). Each command can have 0–n assertions.
- Assertion: Selects data from a JSON source (the parent command’s stdout by default) and compares it to an expected file after masking transient fields.

Names Are Optional (Indexed From 000)

- If a name is omitted, the runner auto-generates stable names starting at 000:
  - Scenario name defaults to the containing directory name.
  - Steps: step-000, step-001, …
  - Commands (per step): cmd-000, cmd-001, …
  - Assertions (per command): assert-000, assert-001, …

Schema (YAML)

- baseInputsPath: path to the base declarative config directory
- log-level: optional default `kongctl` log level for every command (trace|debug|info|warn|error)
- env: map of environment variables to pass to kongctl
- vars: free-form variables usable in templates (e.g., for selectors or overlay files)
- test:
  - maturity: optional stable|beta classification; defaults to stable
  - enabledByEnvVar: optional opt-in env gate for the scenario
  - assignedEnvironment: optional GitHub Actions environment / matrix org
    name. When set, CI runs the scenario only in the matching matrix job.
  - requiresPAT: optional boolean, defaults to true
- defaults:
  - retry:
      attempts: int
      interval: string duration (e.g., "1500ms", "2s")
  - mask:
      dropKeys: [list of key names]        # remove keys by name at any depth
- steps: [
  {
    name?: string,
    skipInputs?: bool,                      # when true, do not copy baseInputsPath into this step's workdir
    inputOverlayDirs?: [paths],            # merge these dirs into the step workdir (omit if none)
    inputOverlayOpsFiles?: [paths],        # targeted ops files (match + set/remove) applied to inputs
    inputOverlayOps?:                      # inline targeted ops (same schema as file entries)
      - file: "apis.yaml"
        match: "apis[?ref=='sms'].publications[?ref=='sms-api-to-getting-started'] | [0]"
        set: { visibility: "private" }
    env?: { KEY: value },                  # step-scoped env vars for all commands in this step
    mask?: { dropKeys: [...] },            # override/extend defaults.mask
    retry?: { attempts, interval },        # override defaults.retry
    commands: [
      {
        name?: string,
        run?: ["apply", "-f", "{{ .workdir }}/portal.yaml", …],  # arbitrary kongctl args
        env?: { KEY: value },                 # command-scoped env vars (override step env)
        resetOrg?: true,                              # synthetic harness command to reset org
        create?: { resource, payload?, expectStatus?, endpointParams? },
          # synthetic Konnect POST
        delete?: { resource, expectStatus?, endpointParams? },
          # synthetic Konnect DELETE
        mask?: { dropKeys: [...] },
        retry?: { attempts, interval },
        assertions: [
          {
            name?: string,
            # Source: by default, the parent command’s stdout is used.
            # Optionally request a fresh source:
            source?: {
              get?: "apis" | "portals" | "…",
              artifact?: {
                path?: "stderr.txt" | "meta.json" | "…",
                glob?: "http-dumps/request-*.txt",
                parseAs?: "json" | "yaml" | "text"
              }
            },
            select: "JMESPath expression",                # isolate object/array/scalar
            expect: {
              file?: path,                                  # expected file (JSON or YAML); required unless fields is used
              overlays?: [paths],                           # small merges applied to expect.file
              fields?: { dotted.path: value, ... }          # inline field expectations against the selected object
            },
            mask?: { dropKeys: [...] },
            retry?: { attempts, interval }
          }
        ]
      }
    ]
  }
]

Scoped Environment Variables

- `scenario.env` sets base environment variables for every kongctl command in the scenario.
- `step.env` extends the scenario env for all commands in that step.
- `command.env` extends (and overrides) the step env for a single command. Later scopes win on key conflicts.
- Use this layering to toggle diagnostics narrowly, e.g., enable `KONNECT_SDK_HTTP_DUMP_REQUEST`/`KONNECT_SDK_HTTP_DUMP_RESPONSE` only on a failing command.
- When those Konnect SDK dump env vars are set, the harness removes the verbose HTTP dumps from stdout (so JSON parsing still works) and writes them to `<step>/commands/<cmd>/http-dumps/request-001.txt`, `response-001.txt`, etc.

Masking (MVP)

- Purpose: remove dynamic fields (ids, timestamps, etags, links) so expected files can focus on business values.
- Applied to both observed and expected before compare.
- MVP keeps this simple: only dropKeys is supported.
- Recommended defaults at scenario.defaults.mask.dropKeys:
  - [id, uuid, created_at, createdAt, updated_at, updatedAt, etag, links, href, self, version]
- Precedence (lowest to highest): defaults.mask < step.mask < command.mask < assertion.mask.
- Merge rule: dropKeys is the union of all applicable levels.

Overlays

- Step input overlays: inputOverlayDirs is a list of directories whose file trees mirror the base inputs. Files are merged into the step workdir using a JSON Merge Patch–style deep merge:
  - Maps merge; scalars replace; arrays replace entirely.
  - Templating is allowed in overlay files via Go text/template + Sprig, with these variables: .vars (from scenario), .scenario, .step, .workdir.
  - When arrays need targeted edits, copy the base file into an overlay dir and modify it, or replace the whole array; we can add JSON Patch/JMESPath ops later if needed.
- Expected overlays: expect.overlays is a list of files merged into the expect.file to form the final expected payload.
- MVP scopes overlays to steps (inputs) and assertions (expected). There are no command-level overlays.

Overlay Ops (Targeted Edits)

- Use `inputOverlayOps` for targeted field updates without replacing arrays.
- Ops file format (YAML):

  ops:
    - file: "apis.yaml"
      match: "apis[?ref=='sms'].publications[?ref=='sms-api-to-getting-started'] | [0]"
      set:
        visibility: "private"

- Semantics:
  - `file`: target file under the step workdir.
  - `match`: JMESPath-like expression (limited subset) that chains mapping keys with optional filters `[?field=='value']`. A trailing `| [0]` is allowed to pick the first match.
  - `set`: deep-merge keys into each matched mapping node (create if absent). Scalars replace, maps merge, arrays replace.
  - Templates allowed in ops file (same context as overlays).
  - Inline ops are supported via `inputOverlayOps` with the same schema.

Optional And Empty Fields

- Omit fields that are empty or not needed. For example, if a step has no
  input overlays, you can omit inputOverlayDirs. If a command has no
  assertions, you can omit the assertions key entirely. If a command is a
  `resetOrg`, `create`, or `delete` synthetic command, omit `run`.

Stateful Commands and Retries

- After reset, use a read-only `plan --mode sync` command to check original
  CREATE actions, references, dependencies, and planned fields before mutation.
- Run `sync -f` normally so each execution retry replans against current state.
  Do not replay the original CREATE plan after a partially successful execution.
- Check successful execution, then verify remote resources and fields with
  readbacks, no-op convergence, and deletion checks. A recovered plan can omit
  resources already created; fixed/minimum change counts or CREATE-or-UPDATE
  assertions on that plan are not reliable coverage.
- Assertions on command stdout and captured artifacts run once: those sources
  are immutable. Only `source.get` assertions poll fresh remote state using the
  configured bounded retry policy. A standalone `get` command's stdout is also
  immutable; use `source.get` when eventual-consistency polling is needed.
- Failed execution artifacts remain in their attempt directories. Assertion
  artifacts belong to the final command output or the fresh readback attempt.
  Each assertion's `source.json` records its source kind and final attempt.

Retry-Safe Assertion Review

- Classify each assertion as an initial planning contract, an execution result,
  or a final-state contract. Read-only `plan`/`diff` output remains a valid
  assertion source when the starting state is controlled. Do not ban output
  assertions wholesale.
- Assert exact CREATE/UPDATE/DELETE actions and counts before mutation. After
  retried apply/sync/delete, require successful completion, but do not treat the
  last attempt's action list or applied count as the complete operation history.
  Earlier attempts may already have applied some changes. Retain those attempts
  for diagnosis; summing their output cannot prove final remote state when a
  response was lost.
- Reuse existing convergence and deletion checks. A fresh read-only no-op plan
  verifies fields the planner compares without potentially repairing drift.
  Use independent readbacks for critical relationships and fields that this
  cannot establish; query a collection once and check multiple fields from it.
  Avoid adding a separate read per field or duplicating every planner check.
- Validate deletion with a pre-deletion plan, then read back absent targets and
  surviving parents. Fixed final-attempt DELETE counts are unsafe for the same
  reason as CREATE counts. A test specifically about single-attempt execution
  counts must explicitly control its initial state and retry behavior.
- `external/api-parent` is a focused example: initial create/delete plans,
  retryable sync completion, grouped version/publication/team-role readbacks,
  read-only convergence, and existing deletion readbacks. Its scoped `get`
  commands produce immutable snapshots; they do not poll assertion mismatches.
  The current `source.get` supports a single resource argument, not a command
  with flags. Do not imply that a standalone `get` assertion polls fresh state.
- Before extending the pattern, record the command-count delta and compare
  HTTP request counts and runner time in comparable live runs. Extra plans can
  perform many inventory reads; command count is not network-request count.

Selectors and Sources

- Use JMESPath to target the object/array/scalar you want to compare.
- Default source is the parent command’s JSON stdout.
- Set `source.get: "<resource>"` to run a fresh `kongctl get <resource>`
  for the assertion.
- Set `source.artifact.path` to read a captured command artifact relative to
  the parent command directory. JSON and YAML files are auto-parsed by
  extension; other files are exposed as an object with a `text` field. Use
  `select: "text"` or `expect.fields.text` when asserting on text artifacts.
  You can override parsing with `parseAs`.
- Set `source.artifact.glob` to inspect a group of captured files relative to
  the parent command directory. Glob sources expose
  `{ count, matches }`, which is useful for request-count assertions against
  `http-dumps/request-*.txt`.
- Examples: "[?name=='My First Portal'] | [0]", "[0]", "data[?title=='SMS API']".
- Tiny example (assert under an apply command using a fresh get source):

  assertions:
    - select: "[?name=='{{ .vars.portalName }}'] | [0]"
      source: { get: "portals" }
      expect:
        file: "expect/portal.json"

  assertions:
    - source:
        artifact:
          glob: "http-dumps/request-*.txt"
      expect:
        fields:
          count: 2

  assertions:
    - source:
        artifact:
          path: "stderr.txt"
      select: "text"
      expect:
        fields:
          text: "warning: example"

JMESPath Examples (Nested Fields)

- From apply output: `plan.changes[?resource=='portal' && op=='create' && after.name=='{{ .vars.portalName }}'] | [0]`
- Nested field: `plan.changes[?op=='update'] | [0].after.visibility`

Comparison Semantics (Simplicity First)

- Deep-compare JSON after masking. Object key order is irrelevant; array order is significant.
- To avoid array-order flakiness in MVP, prefer selecting a specific element with stable filters (e.g., by name) rather than comparing whole arrays.
- If you must compare arrays, use JMESPath to sort or project them into a stable form before compare, e.g., `sort_by(@,&name)` or `[].[name, visibility]`.

Library Evaluation: go-cmp / cmpopts

- We can use `github.com/google/go-cmp/cmp` to produce clear diffs on failure.
- `cmpopts.IgnoreFields` is most effective with typed Go structs; our assertions compare dynamic JSON (`map[string]any`/`[]any`). For MVP, a simple pre-compare sanitizer that drops keys by name is clearer and requires no schema.
- `cmpopts.SortSlices` and other helpers are designed for typed slices; for generic JSON arrays we’d still need custom logic or selectors. Given we encourage selectors to pick a single element or to sort via JMESPath, we don’t need array sorting in the comparator for MVP.
- Proposal: MVP uses a lightweight sanitizer (dropKeys) + `go-cmp` for deep-compare and human-friendly diffs. We can revisit path-aware ignore/sort rules using `cmp.FilterPath` if we later move to typed models or need more advanced behavior.

Golden Files (Terminology)

- We avoid the term “golden”; the DSL uses expect.file and expect.overlays.
- Update mode (for bootstrapping): set KONGCTL_E2E_UPDATE_EXPECT=1 to write the sanitized observed value into expect.file (file-based assertions only). Overlays still apply afterward. Inline fields (expect.fields) do not write back; the diff is shown in artifacts.

Artifacts Per Assertion

- observed.json: masked selected object from the command output (or subset for fields mode)
- expected.json: expected payload (file-based or fields map)
- select.txt: the JMESPath selector used (after templating) to derive observed.json
- result.txt: first line pass|fail, separator, then a human-readable diff ("(no diff)" when equal)

Example Scenario

See: test/e2e/scenarios/portal/visibility/scenario.yaml

Notes

- Assertions attach to commands so you can validate state in-between changes.
- If a needed resource isn’t supported by kongctl get yet, add that coverage to the CLI so scenarios don’t need HTTP fallbacks.

Synthetic Create Recovery

- A synthetic `create:` POST that times out, loses its response, or returns a
  server error has an unknown outcome. The harness does not repeat it, because
  the server may already have committed the mutation. This differs from sync
  recovery, which replans against remote state.
- For organization teams, opt into `recoverTeam: true` inside `create:`. The
  harness adds a reserved `e2e-create-id` label with a unique value before POST.
  After an ambiguous failure it polls the paginated team inventory for up to
  30 seconds. Exactly one matching team with all requested fields recovers its
  ID for `recordVar`; conflicting or duplicate matches fail. Absence at the
  deadline remains unknown and never authorizes another POST.
- The label remains on the team, so opt-in scenarios must tolerate its presence
  in readbacks/dumps. Existing labels are preserved; supplying the reserved key
  is an error. Other resource types have no automatic create reconciliation.
- Failed requests retain duration/status/timeout metadata in `failure.json`.
  With `KONGCTL_E2E_LOG_LEVEL=trace` (the workflow's `trace_http` option), synthetic
  requests also write `http-phases.jsonl` as phases occur. Phase completion does
  not imply success; these records omit URLs, credentials, bodies and addresses.
- Recovery polls share one artifact directory with phase traces, the latest
  failed-read metadata, and `create-recovery.json` on success. Full team
  inventories are not saved. Disabling capture also disables these artifacts.
