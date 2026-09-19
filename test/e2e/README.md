## E2E Test Harness

This repository includes an end-to-end (E2E) testing harness for `kongctl`
that builds the CLI once per test run, executes commands against real Konnect,
and captures detailed artifacts for triage.

- Default profile: `e2e`
- Default output: JSON, unless a command overrides it
- Isolation: each test uses its own `XDG_CONFIG_HOME` under the artifacts dir
- No mocks: tests call real Konnect and skip when required auth is missing
- Test model: E2E coverage is scenario-first under `test/e2e/scenarios`

### Quick Start

Run the full E2E suite:

```bash
make test-e2e
```

Run only scenarios:

```bash
make test-e2e-scenarios
```

Run a single scenario by exact scenario path:

```bash
make test-e2e-scenarios SCENARIO=portal/edit
```

Run one shard locally:

```bash
KONGCTL_E2E_KONNECT_PAT=$(cat ~/.konnect/your-pat) \
KONGCTL_E2E_SHARD_TOTAL=4 \
KONGCTL_E2E_SHARD_INDEX=1 \
make test-e2e-scenarios
```

Run against the Kong Konnect `.tech` environment:

```bash
KONGCTL_E2E_KONNECT_ENV=tech \
KONGCTL_E2E_KONNECT_PAT=$(cat ~/.konnect/your-tech-pat) \
make test-e2e-scenarios
```

Include the opt-in user profile smoke scenario:

```bash
KONGCTL_E2E_KONNECT_PAT=$(cat ~/.konnect/your-user-pat) \
KONGCTL_E2E_RUN_USER_ME=1 \
make test-e2e-scenarios
```

Reuse a prebuilt binary instead of building during tests:

```bash
make build
KONGCTL_E2E_BIN=./kongctl make test-e2e
```

### Environment Variables

Core harness settings:

- `KONGCTL_E2E_LOG_LEVEL`: Harness and CLI log level
  (`trace|debug|info|warn|error`). Default: `warn`.
- `KONGCTL_E2E_CONSOLE_LOG_LEVEL`: Console log level while preserving richer
  logs in artifacts. Default: same as harness log level.
- `KONGCTL_E2E_OUTPUT`: Default CLI output (`json|yaml|text`). Default:
  `json`.
- `KONGCTL_E2E_TEST_TIMEOUT`: Go test timeout for local Make targets and the
  CI scenario test binary. Default: `55m`.
- `KONGCTL_E2E_KONNECT_DECLARATIVE_MAX_CONCURRENCY`: Standard `e2e`
  profile env var for declarative execution concurrency. It maps to
  `konnect.declarative.max-concurrency` and applies to all declarative
  commands unless a command passes `--max-concurrency` or a scenario override
  is set. Valid range: `1..200`.
- `KONGCTL_E2E_MAX_CONCURRENCY_VALUES`: Optional suite-wide concurrency sweep.
  When set, the harness hashes each scenario path and picks one value from the
  comma-separated list, such as `1,2,5,10`, then injects that value as the
  scenario's declarative concurrency default. Use this to run the same suite
  with mixed concurrency settings without editing each scenario. Explicit
  scenario YAML overrides and `KONGCTL_E2E_KONNECT_DECLARATIVE_MAX_CONCURRENCY`
  take precedence.
- `KONGCTL_E2E_CAPTURE`: Per-command artifact capture. `0` disables it.
- `KONGCTL_E2E_JSON_STRICT`: When `1`, JSON parsing fails on unknown fields.
  Default: lenient.
- `KONGCTL_E2E_ARTIFACTS_DIR`: Root folder for artifacts for this run.
  Default: a temp dir.
- `KONGCTL_E2E_BIN`: Path to an existing `kongctl` binary to skip building.
- `KONGCTL_E2E_RESET`: Reset the Konnect org before tests. Destructive.
  Defaults to enabled; set to `0` or `false` to disable.
- `KONGCTL_E2E_KONNECT_ENV`: Konnect environment selector. Supported values
  are `com` (default) and `tech`; `production` remains accepted as a legacy
  alias for `com`. The harness writes this into the generated CLI profile,
  and raw harness HTTP helpers use it to select the matching regional and
  global Konnect defaults.
- `KONGCTL_E2E_KONNECT_BASE_URL`: Optional regional Konnect API override.
  When unset, the harness uses the selected `KONGCTL_E2E_KONNECT_ENV`
  default. If this points at `konghq.tech`, the harness also infers the
  `.tech` global URL and machine client ID for raw harness calls unless
  explicitly overridden. The generated CLI profile includes
  `konnect.base-url` only when this variable is set.
- `KONGCTL_E2E_KONNECT_BASE_AUTH_URL`: Optional global/auth Konnect API
  override. The harness uses this for global Identity APIs, org reset, and
  generated CLI profile `konnect.base-auth-url` when set.
- `KONGCTL_E2E_KONNECT_MACHINE_CLIENT_ID`: Optional machine client ID
  override for generated CLI profile `konnect.machine-client-id` when set.
- `KONGCTL_E2E_HTTP_TIMEOUT`: Per-request timeout for raw Konnect HTTP helpers
  used by scenario create/delete flows. The harness also writes the same
  value into the generated `e2e.http-timeout` profile setting so
  SDK-backed CLI commands share the same default. Default locally and in CI:
  `15s`. Unset/empty selects that default. `0s` (also `0`, `off`, `none`,
  `disable`, `disabled`, `default`, `defaults`, `platform`, or `system`)
  explicitly disables the request timeout in both clients. Previously zero
  omitted the CLI profile setting and retained the CLI's 60-second default.
  It does not disable the subprocess deadline.
- `KONGCTL_E2E_HTTP_TCP_USER_TIMEOUT`: Linux-only `TCP_USER_TIMEOUT` applied
  to raw harness HTTP sockets. The harness also writes the same value into
  the generated `e2e.http-tcp-user-timeout` profile setting so
  SDK-backed CLI commands use the same socket setting. Default: unset.
- `KONGCTL_E2E_HTTP_DISABLE_KEEPALIVES`: Disable raw harness HTTP keepalive
  reuse. The harness also writes the same value into the generated
  `e2e.http-disable-keepalives` profile setting so SDK-backed CLI
  commands share the same setting. Default: `false`.
- `KONGCTL_E2E_HTTP_RECYCLE_CONNECTIONS_ON_ERROR`: Close idle pooled harness
  HTTP connections after a raw HTTP error before retrying. The harness also
  writes the same value into the generated
  `e2e.http-recycle-connections-on-error` profile setting so
  SDK-backed CLI commands share the same setting. Default: `false`.
- `KONGCTL_E2E_HTTP_RETRY_ATTEMPTS`: Default retry attempts for raw Konnect
  HTTP helpers. Default: `4`.
- `KONGCTL_E2E_HTTP_RETRY_INTERVAL`: Base retry interval for raw Konnect HTTP
  helpers. Default: `1s`.
- `KONGCTL_E2E_HTTP_RETRY_MAX_INTERVAL`: Max retry interval for raw Konnect
  HTTP helpers. Default: `5s`.
- `KONGCTL_E2E_HTTP_RETRY_BACKOFF_FACTOR`: Backoff multiplier for raw Konnect
  HTTP helpers. Default: `2`.
- `KONGCTL_E2E_HTTP_RETRY_JITTER`: Jitter applied to raw Konnect HTTP helper
  retries. Default: `250ms`.
- `KONGCTL_E2E_RESET_HTTP_TIMEOUT`: Per-request timeout for destructive org
  reset API calls. Local and CI default: `15s`.
- `KONGCTL_E2E_RESET_TIMEOUT`: Total time budget for a single org reset before
  the harness aborts the remaining reset steps. Local and CI default: `3m`.
- `KONGCTL_E2E_RESET_RETRY_ATTEMPTS`: Retry attempts for reset API calls.
  Default: `3`.
- `KONGCTL_E2E_RESET_RETRY_INTERVAL`: Base retry interval for reset API calls.
  Default: `1s`.
- `KONGCTL_E2E_RESET_RETRY_MAX_INTERVAL`: Max retry interval for reset API
  calls. Default: `5s`.
- `KONGCTL_E2E_RESET_RETRY_BACKOFF_FACTOR`: Backoff multiplier for reset API
  calls. Default: `2`.
- `KONGCTL_E2E_RESET_RETRY_JITTER`: Jitter applied to reset API call retries.
  Default: `250ms`.
- `KONGCTL_E2E_SKIP_STEPS`: Comma-separated glob patterns to skip scenario
  steps by name.
- `KONGCTL_E2E_STOP_AFTER`: Stop after a matching step or command.

Request recovery and command budgets:

- Every CLI subprocess attempt has its own 60-second deadline, including all
  requests, planning, mutations, and output. It is not a shared scenario
  budget. Scenario commands can set `timeout: 120s` when measured healthy
  execution plus recovery needs more time. A full subprocess timeout still
  stops normal command recovery; it is not automatically retried.
- E2E profiles enable `konnect.http-retry-on-read-errors`, with two total
  HTTP attempts and backoff capped at one second. A stalled request therefore
  has a nominal maximum of `2 * 15s + 1s = 31s`, leaving room in the command
  budget for other work. Several slow requests can still exhaust the command
  budget. Imperative commands retry only GET/HEAD transport failures;
  declarative commands retain their existing retryable HTTP status codes,
  but share the generated profile's two-attempt limit and one-second backoff
  cap. These intentionally replace the CLI defaults of three attempts and a
  60-second backoff cap in E2E to leave room within the command deadline.
- Read recovery is opt-in outside E2E through the profile setting above (or
  `KONGCTL_<PROFILE>_KONNECT_HTTP_RETRY_ON_READ_ERRORS`). It shares the existing
  `konnect.http-retry-max-attempts`, `http-retry-initial-interval`, and
  `http-retry-max-interval` settings; intervals are milliseconds. Setting
  max attempts to one disables recovery. The broader connection-error retry
  option remains disabled in generated profiles, so mutations do not gain
  transport retries. A request whose parent context has ended is not retried.
- HTTP recovery retries the failed request while the CLI remains alive.
  Failures while a caller consumes a response body can still surface to the
  command. Harness command retries re-execute the whole command and replan;
  stateful assertions must follow the pattern in the scenario authoring guide.
  The harness still allows up to six command attempts by default, with its
  existing bounded backoff. HTTP and command attempt counts are separate:
  two HTTP attempts per request per command attempt, not an extra SDK loop.
- Direct harness create/delete helpers and org reset retain their own retry
  policies listed above. The new CLI read policy does not replace them.
- HTTP request/error logs include the effective `http_timeout`; request
  failures classify timeouts separately from parent-context cancellation.
  Retry logs preserve attempts and terminal/recovered outcomes. Scenario
  diagnostics report `profile_http_timeout_ms` and each subprocess's
  `timeout_ms`. CLI flags/environment overrides can change the profile value;
  the HTTP log reflects the actual client timeout. Go test and Actions job
  deadlines remain separate outer limits.

Scenario selection and sharding:

- `KONGCTL_E2E_SCENARIO`: Scenario selector. Examples: `portal/edit`,
  `scenarios/portal/edit`, a full `scenario.yaml` path, or a directory prefix
  such as `ai-gateway`.
- `KONGCTL_E2E_SHARD_INDEX`: Zero-based shard index for this test process.
- `KONGCTL_E2E_SHARD_TOTAL`: Total number of shards in the run.
- `KONGCTL_E2E_MATRIX_ORG`: Optional diagnostic label for the current CI job.
- `KONGCTL_E2E_ORGS_JSON`: JSON array of matrix org entries. Used in CI to
  validate scenario environment assignments. Each entry must include
  `org_name`. In GitHub Actions, this is also the default org matrix when an
  environment-specific org matrix is not configured.
- `KONGCTL_E2E_COM_ORGS_JSON`: Optional GitHub Actions org matrix for `.com`
  Konnect runs.
- `KONGCTL_E2E_PRODUCTION_ORGS_JSON`: Legacy optional GitHub Actions org
  matrix for `.com` Konnect runs.
- `KONGCTL_E2E_TECH_ORGS_JSON`: Optional GitHub Actions org matrix for
  `.tech` Konnect runs.

Authentication and opt-in scenarios:

- `KONGCTL_E2E_KONNECT_PAT`: PAT used by the `e2e` profile for authenticated
  scenarios. Most scenarios require this.
- `KONGCTL_E2E_RUN_USER_ME`: Opt in to the `auth/get-me` scenario.
- `KONGCTL_E2E_RUN_PORTAL_APPLICATIONS`: Opt in to the Gmail-backed portal
  applications scenario.

Gmail automation for portal developer scenarios:

- `KONGCTL_E2E_GMAIL_ADDRESS`: Base Gmail inbox. Tests append `+<uuid>` so
  mail lands in unique sub-inboxes.
- `KONGCTL_E2E_GMAIL_CLIENT_ID`
- `KONGCTL_E2E_GMAIL_CLIENT_SECRET`
- `KONGCTL_E2E_GMAIL_REFRESH_TOKEN`
- `KONGCTL_E2E_GMAIL_ACCESS_TOKEN`
- `KONGCTL_E2E_GMAIL_SUBJECT` (optional): override the Gmail subject filter.
- `KONGCTL_E2E_AUTH_STRATEGY_ID` (optional): override the auth strategy ID
  used when creating developer applications.

### Scenario Execution Model

`Test_Scenarios` discovers all `scenario.yaml` files under
`test/e2e/scenarios`, sorts them, and runs them as subtests.

Most scenarios are authenticated and will skip unless
`KONGCTL_E2E_KONNECT_PAT` is set. A scenario can opt out of that preflight
check with:

```yaml
test:
  requiresPAT: false
```

That is how the `smoke/version` scenario runs without Konnect credentials.

If both `KONGCTL_E2E_SHARD_INDEX` and `KONGCTL_E2E_SHARD_TOTAL` are set, the
runner assigns scenarios to shards by sorted position:

```text
scenario i belongs to shard (i % shard_total)
```

Example with 10 scenarios and 4 shards:

- shard `0` runs scenarios `0, 4, 8`
- shard `1` runs scenarios `1, 5, 9`
- shard `2` runs scenarios `2, 6`
- shard `3` runs scenarios `3, 7`

If `KONGCTL_E2E_SCENARIO` is set, sharding is bypassed so local single-scenario
iteration stays predictable.

A scenario can be pinned to a specific GitHub Actions environment by setting
`test.assignedEnvironment` to the matrix `org_name`:

```yaml
test:
  assignedEnvironment: kongctl-e2e-users
```

Pinned scenarios run only in the matrix job whose `KONGCTL_E2E_MATRIX_ORG`
matches that value. They are excluded from normal modulo sharding so they do
not run in any other org. Unpinned scenarios continue to be sharded by sorted
position. In CI, the harness validates pinned environments against
`KONGCTL_E2E_ORGS_JSON` and fails if a scenario names an environment that is
not present in the `.com` org pool. Non-`.com` runs, such as `.tech`, exclude
scenarios pinned to missing org names and upload an `excluded-scenarios.txt`
artifact so coverage verification still checks the scenarios expected to run
for that target.

For local full-suite runs without sharding, assignments are not enforced by
default because the run targets a single developer-selected org. To emulate a
CI environment locally, set `KONGCTL_E2E_MATRIX_ORG`:

```bash
KONGCTL_E2E_MATRIX_ORG=kongctl-e2e-users \
KONGCTL_E2E_SCENARIO=org/teams/roles \
make test-e2e-scenarios
```

### Skipping Steps

Use `KONGCTL_E2E_SKIP_STEPS` to selectively skip scenario steps. This is
useful for preserving resources for manual CLI verification.

Skip all deletion steps:

```bash
KONGCTL_E2E_SKIP_STEPS="*delete*" \
KONGCTL_E2E_SCENARIO=portal/applications \
KONGCTL_E2E_KONNECT_PAT=$(cat ~/.konnect/token) \
make test-e2e-scenarios
```

Skip specific numbered steps:

```bash
KONGCTL_E2E_SKIP_STEPS="006-*,007-*,008-*" \
KONGCTL_E2E_SCENARIO=portal/applications \
make test-e2e-scenarios
```

Skip multiple pattern types:

```bash
KONGCTL_E2E_SKIP_STEPS="*reset*,*delete*,*cleanup*" \
make test-e2e-scenarios
```

### Scenario diagnostics

Each scenario that initializes its harness writes
`scenario-diagnostics.json` in its test artifact directory when it returns.
This is observation only: it does not change retries or request reruns.
The E2E shard job summary lists terminal failures and the 15 slowest commands,
including successful commands. All command timings remain in the JSON file.

The versioned record includes scenario, organization, workflow run/attempt,
workflow SHA, and checkout SHA (when supplied by CI). Each command includes
step/name, outcome, total duration, individual execution attempts, configured
timeouts, and the execution retry stop reason. Total duration includes command
setup, retries, backoff, output processing, and assertions. Attempt durations
measure subprocess execution or a direct HTTP operation. A timeout of zero
means no configured application deadline.

Terminal failures identify the phase separately from the cause. In particular,
`subprocess_deadline` means the harness killed kongctl or an external command
such as deck; it does not establish that Konnect timed out. An assertion or
output-processing failure is not attributed to earlier recovered errors.
Unknown causes stay `unknown`; no retry eligibility is inferred.

Direct scenario create/delete HTTP operations include method, hostname,
status, timing, and typed transport error metadata, even on failure. Arguments,
URL paths, headers, bodies, and raw error messages are excluded. Requests made
inside kongctl/deck and reset helpers are not individually captured in this
record; consult their existing logs for endpoint and request/trace IDs. The
attempt list covers command execution, not assertion reads or reset internals.

Retry stop reasons are `succeeded`, `attempt_limit`, `retry_policy`,
`not_configured` (external commands), or `expected_failure`. `retry_policy`
means the existing predicate stopped retries; the recorded duration and limit
help identify timeout suppression. A successful expected-failure command can
have a nonzero subprocess exit code. Advisory beta failures still appear as
scenario failures here; their advisory handling remains unchanged.

Scenarios skipped before initialization, load/preflight failures, and processes
killed before diagnostics can be written may have no record. Missing records
are not evidence of success. Diagnostics write or summary errors are reported
without changing the scenario's result. Malformed records are counted as
unavailable without discarding summaries from valid records.

If scenario capture never starts, the shard summary reports that fact (and
identifies a failed Setup deck step), and metrics generation is skipped.
Metrics are still collected after failed scenario executions when capture
started. Collector validation remains strict; collection failures produce a
warning and a summary note without changing the scenario result. Metrics are
uploaded only when collection succeeds. No empty or zero-valued replacement
metrics are generated for unavailable data.

To generate a summary locally:

```sh
python3 scripts/e2e_diagnostics.py <artifacts_dir> <summary.md>
```

### Artifacts Layout

Each test run creates a single artifacts directory. The Makefile prints the
path at the end of the run and records logs in `run.log`.

```text
<artifacts_dir>/
  bin/
    kongctl
  run.log
  tests/
    Test_Scenarios_scenarios_portal_edit_scenario_yaml/
      config/
        kongctl/
          config.yaml
      steps/
        000-reset-org/
          commands/
            000-reset-org/
              command.txt
              stdout.txt
              stderr.txt
              env.json
              meta.json
        001-apply-initial/
          inputs/
            portal.yaml
          commands/
            000-apply-initial/
              command.txt
              stdout.txt
              stderr.txt
              env.json
              meta.json
              observation.json
            001-get-portal/
              command.txt
              stdout.txt
              stderr.txt
              env.json
              meta.json
              observation.json
```

The harness keeps artifacts by default for local debugging and CI upload.

### Behavior And Conventions

- Build once: the binary is built, or copied from `KONGCTL_E2E_BIN`, once per
  run and reused.
- Default JSON: the harness injects `-o json` unless the command already sets
  an output flag.
- Log level: the harness injects `--log-level` unless the command already sets
  one.
- Profile config: the harness writes `config.yaml` for the `e2e` profile.
- Declarative concurrency: scenarios can set `maxConcurrency` at the default,
  step, or command level. Command wins over step, step wins over defaults, and
  explicit `--max-concurrency` in `run` still has normal CLI precedence.
- Sanitization: token-like env vars are redacted in `env.json` and logs.
- HTTP dumps: when Konnect SDK dump env vars are enabled, the harness stores
  each exchange under the command’s `http-dumps/` directory.
- Observations: `observation.json` is attached to captured commands for apply
  summaries and read observations.

Example scenario concurrency overrides:

```yaml
defaults:
  maxConcurrency: 5

steps:
  - name: cleanup
    maxConcurrency: 1
    commands:
      - name: delete
        maxConcurrency: 1
        run:
          - delete
          - -f
          - "{{ .workdir }}/config.yaml"
          - --auto-approve
```

### CI Notes

The `E2E / Scenario Suite` GitHub Actions workflow scales wall-clock time by
running one matrix job per Konnect org. Each job gets:

- one environment-scoped PAT
- the Konnect environment selector from workflow dispatch or repository
  variables
- optional environment-specific Konnect URL overrides from repository
  variables
- one shard index
- the common shard total from the matrix size

The workflow derives sharding directly from GitHub Actions strategy context:

- `KONGCTL_E2E_SHARD_INDEX=${{ strategy.job-index }}`
- `KONGCTL_E2E_SHARD_TOTAL=${{ strategy.job-total }}`
- `KONGCTL_E2E_MATRIX_ORG=${{ matrix.org_name }}`
- `KONGCTL_E2E_ORGS_JSON` set to the selected org matrix

Fork pull requests do not receive secret-backed E2E automatically. When a fork
PR changes files that require E2E, the required status remains pending with a
message that trusted maintainer E2E is required. The normal fork-triggered E2E
workflow does not expand shard jobs, does not use the `default` org fallback,
and does not receive Konnect or Gmail secrets.

The status named `E2E Required` is the merge gate. It is a commit status, not
one of the workflow job names. For fork PRs, the
`E2E / Required Status` workflow initializes that status and posts an
updatable PR comment with the exact SHA and trusted workflow command. The
trusted `E2E / Scenario Suite` dispatch later updates the same commit status
and comment with the pass or fail result.

GitHub displays the E2E checks using this hierarchy:

1. The workflow name is the section on the checks screen.
2. The job name is a check row within that section.
3. The step name is an operation visible after opening a job.
4. `E2E Required` is the protected commit-status context used as the merge
   gate.

Multiple PR events can still create multiple workflow sections. The distinct
workflow, run, job, and step names clarify those sections without changing
trigger or deduplication behavior. Main-branch scheduling appears in Actions
as `E2E / Main Scheduler`.

For the maintainer runbook, see
`docs/contributor/forked-pr-e2e.md`.

The workflow also exposes the Konnect target, test timeout, HTTP timeout, and
retry knobs above as repository or organization variables of the same names,
so CI can tune runtime behavior without changing Go code. Manual dispatch
runs can choose `auto`, `com`, or `tech` with the `konnect_environment`
input. In `auto`, PRs labeled `konnect-env:tech` target `.tech`; other runs
use `KONGCTL_E2E_KONNECT_ENV` when configured, then default to `.com`.

For transport debugging, the workflow currently defaults to:

- `KONGCTL_E2E_HTTP_TCP_USER_TIMEOUT=60s`
- `KONGCTL_E2E_HTTP_RECYCLE_CONNECTIONS_ON_ERROR=1`

Override either one with a repository or organization variable if you want to
disable or change the experiment for CI runs.

Each matrix leg writes an `assigned-scenarios.txt` manifest into its artifact
directory. A final `E2E Verify` job downloads those manifests and fails unless:

- every shard index from `0` through `KONGCTL_E2E_SHARD_TOTAL-1` appears once
- no scenario appears in more than one shard manifest
- the combined manifest set matches the discovered scenario list after any
  target-specific `excluded-scenarios.txt` entries are removed

That verification step is the guardrail that keeps sharding regressions from
silently dropping or duplicating scenario coverage.

The workflow summary also includes aggregated execution results from all shard
jobs, including assigned scenario count, pass/fail/skip totals, per-shard
durations, exit codes, and a failed-scenarios table when applicable.

Shard result artifacts are named `e2e-artifacts-<run-id>-<attempt>-<org>`.
The verifier downloads all attempts and uses the highest recorded attempt
number per shard, retaining earlier results for shards that were not rerun.
Distinct names prevent the download action from choosing between attempts by
artifact ID, which does not reliably reflect upload order. A newer failed
attempt still blocks verification; an older failure does not override a
successful rerun.

For temporary GitHub-runner network debugging, the workflow can also capture
packet traces for Konnect endpoints:

- set the `workflow_dispatch` input `capture_tcpdump=true`, or
- set the repository or organization variable `KONGCTL_E2E_CAPTURE_TCPDUMP=1`

When enabled, each matrix job records a `tcpdump/` directory in the normal E2E
artifact bundle containing:

- `konnect.pcap`: packet capture filtered to the regional Konnect host and
  selected global Konnect host on port `443`
- `tcpdump.log`: tcpdump startup and shutdown output
- `context.txt`: runner host, DNS resolution, interfaces, and routes

To investigate a stalled request, enable `trace_http=true` on a manual
workflow dispatch. This keeps console logging at `warn` and captures trace
logs in the normal command artifacts. For a targeted local run, use:

```sh
KONGCTL_E2E_LOG_LEVEL=trace KONGCTL_E2E_CONSOLE_LOG_LEVEL=warn \
  make test-e2e-scenarios SCENARIO=deck/multi-file
```

The CLI also supports `--log-level trace --log-file <path>` outside E2E.
Trace verbosity includes existing redacted HTTP payload logging; connection
phase events themselves contain no headers, bodies, query values, or raw
network error text. Normal debug/info logging does not emit phase events.

Each `log_type=http_phase` record is written as progress happens, including
DNS, TCP connect, TLS, connection acquisition/reuse, request headers/write,
and first response byte. `elapsed_ms` measures time since that HTTP attempt
started. `request_done` with `outcome=response_headers` means the HTTP client
returned response headers, not that response-body consumption completed.
Phase callbacks may be concurrent; absent DNS/TLS events can indicate reuse
or an unsupported transport, and are not evidence of failure.

In `scenario-diagnostics.json`, each subprocess attempt with a captured log
includes a `log_path` relative to that file and `http_requests` with the last
observed phase, elapsed time, effective HTTP timeout, and outcome.
`terminal_elapsed_ms` separately records the completion/error event's elapsed
time when available; it is omitted for unfinished requests or missing/invalid
terminal timing. Its existing `timeout_ms` is the separate subprocess limit.
`unfinished` means no completion event was observed; it does not establish a
server timeout. `http_trace_status` reports `not_observed` when no log exists
or no phase events were captured, and `unavailable_or_incomplete` on
read/scanner errors. Missing logs have no `log_path`. Summary metadata
excludes routes, error text, headers, and bodies.

Correlate using scenario, step, command, attempt index, log path, and request
ID together: `khttp-000001` can recur in every process. Earlier retried
execution logs link to their preserved `attempts/` directories; final
execution logs remain separate from terminal assertion diagnostics. Compare
the last observed phases and budgets against one successful execution of
the same command. Preserve both artifact bundles; do not automatically
rerun failed shards until they pass. Add `capture_tcpdump=true` when packet
evidence is needed to investigate gaps left by the application trace.

The default org pool is defined as JSON in the repository or organization
variable `KONGCTL_E2E_ORGS_JSON`. Example:

```json
[
  { "org_name": "kongctl-e2e-us-1" },
  { "org_name": "kongctl-e2e-us-2" }
]
```

Each `org_name` must match a GitHub Actions environment that defines the
secret `KONGCTL_E2E_KONNECT_PAT`.

Set `KONGCTL_E2E_KONNECT_ENV=tech` as a repository or organization variable
to run scheduled or PR-triggered E2E against `.tech`. Manual dispatch can
override this with the `konnect_environment` input.

If `.com` and `.tech` need separate org pools, set
`KONGCTL_E2E_COM_ORGS_JSON` and `KONGCTL_E2E_TECH_ORGS_JSON` to environment
names backed by matching PATs. `KONGCTL_E2E_PRODUCTION_ORGS_JSON` remains a
fallback for existing `.com` configuration. The workflow selects an org
matrix with this fallback order for non-fork runs:

1. `KONGCTL_E2E_TECH_ORGS_JSON` for `.tech`, or
   `KONGCTL_E2E_COM_ORGS_JSON` for `.com`, falling back to
   `KONGCTL_E2E_PRODUCTION_ORGS_JSON` for legacy `.com` setups.
2. `KONGCTL_E2E_ORGS_JSON`.
3. A single `default` matrix entry.

Use `KONGCTL_E2E_COM_KONNECT_BASE_URL`,
`KONGCTL_E2E_COM_KONNECT_BASE_AUTH_URL`, and
`KONGCTL_E2E_COM_KONNECT_MACHINE_CLIENT_ID` for `.com`-specific endpoint
overrides. The legacy `KONGCTL_E2E_KONNECT_BASE_URL`,
`KONGCTL_E2E_KONNECT_BASE_AUTH_URL`, and
`KONGCTL_E2E_KONNECT_MACHINE_CLIENT_ID` variables remain `.com` fallbacks.

Use `KONGCTL_E2E_TECH_KONNECT_BASE_URL`,
`KONGCTL_E2E_TECH_KONNECT_BASE_AUTH_URL`, and
`KONGCTL_E2E_TECH_KONNECT_MACHINE_CLIENT_ID` for `.tech`-specific endpoint
overrides. Leave them unset when the standard `.tech` defaults are sufficient.

If the org-pool variable is unset, the workflow falls back to a single-org
matrix entry named `default` for non-fork runs, which is useful during
migration if you still have a `default` environment or a temporary
repository-level `KONGCTL_E2E_KONNECT_PAT` secret in place.
Untrusted fork-triggered PR runs do not use this fallback.

### SDK Prerelease Preview Automation

- Workflow `SDK Prerelease Preview` runs daily, and on manual dispatch, to
  fetch the latest prerelease tag from `Kong/sdk-konnect-go`, bump `go.mod`,
  and execute `make build`, `make test`, and `make test-e2e`.
- It requires repository secret `KONGCTL_E2E_KONNECT_PAT`.
- The workflow publishes harness artifacts and opens or updates a PR under
  `automation/sdk-preview/<tag>` when dependency changes are detected.

### Troubleshooting

- Enable verbose logs with `KONGCTL_E2E_LOG_LEVEL=debug`.
- Inspect `<artifacts_dir>/run.log` for created paths, command lines, and
  durations.
- Check per-command `command.txt`, `stderr.txt`, and `meta.json` for the exact
  invocation and exit codes.
- Reset and scenario raw HTTP calls use a separate backoff policy from CLI
  subprocess retries. The harness honors `Retry-After` when Konnect returns
  throttling responses and fails faster after repeated full request timeouts.
- If JSON parsing fails due to extra fields, either add those fields to the
  relevant test struct or keep the default lenient mode.

### Remote CI Failure Diagnosis

Use `make diagnose-e2e-ci` to download E2E workflow artifacts and summarize the
failed shard from `scenario-results.txt`, `run.log`, per-command `meta.json`,
`stderr.txt`, `kongctl.log`, and HTTP dump artifacts.

```sh
# Diagnose failed E2E shards for workflow run number 2254.
make diagnose-e2e-ci RUN=2254

# Diagnose failed E2E shards for the latest E2E run on PR 123.
make diagnose-e2e-ci PR=123

# Diagnose one matrix org/shard.
make diagnose-e2e-ci RUN=2254 ORG=kongctl-acceptance-5

# Analyze artifacts that have already been downloaded.
make diagnose-e2e-ci ARTIFACTS_DIR=.e2e-artifacts/ci/e2e-run-2254-attempt-1
```

The helper uses `gh`, so authenticate once with `gh auth login`. By default it
resolves `RUN` as an E2E workflow run number, or `PR` as the latest E2E run for
that pull request. It downloads only failed E2E shard artifacts into
`.e2e-artifacts/ci/`, prints a markdown report, and also writes
`e2e-ci-diagnosis.md` beside the downloaded artifacts. Pass additional script
flags through `E2E_CI_DIAGNOSE_FLAGS`, for example:

```sh
make diagnose-e2e-ci RUN=2254 E2E_CI_DIAGNOSE_FLAGS="--all-shards"
```

### CI Performance Baseline

Every `.com` scenario shard uploads a small `e2e-metrics-*` artifact. It
contains no credentials or request payloads. The record includes selected and
individual scenario durations plus reset counts, list calls, resources found,
delete calls, resources deleted, and list/delete timing.

`resources_found` counts the initial resources returned for each resource
family in a reset. List and delete durations cover each complete operation,
including response-body handling and retry attempts, but exclude retry sleep.

Live org reset clears configured user assignments first, then inventories up
to three resource categories concurrently. Pagination and HTTP retries stay
sequential within each category. Once every inventory read finishes, resource
deletion follows the existing dependency order, including portal custom-domain
cleanup. Conflict retries fetch fresh inventory. A failed inventory never
causes deletion from a partial result; other categories still get cleaned up.
The total reset deadline applies to both phases. Workflow organization locks
are unchanged, and replay jobs continue to skip org reset entirely.

Reset `duration_ms` is elapsed wall time. Per-category durations count active
listing and deletion time, including backoff, but exclude waiting for other
categories. List/delete request durations are cumulative work: concurrent list
durations can overlap and must not be summed to infer elapsed reset time.
Compare reset wall time and longest-shard duration across pre/post-change runs;
use request counts and errors to check for increased retries or rate limiting.

`make test-e2e-harness` runs the harness unit tests with the race detector and
local HTTP test servers; it does not contact Konnect or reset an organization.
It also runs in PR CI and as part of `make test-all`.

After at least 20 instrumented successful full runs, generate a reproducible
baseline with:

```sh
gh auth status
make baseline-e2e-ci
```

The command writes Markdown and JSON reports under
`.e2e-artifacts/baseline/`. It scans the 100 most recent successful workflow
runs by default, then retains only complete `.com` runs whose latest attempt
has every shard and whose build, harness, scenario, coverage-verification, and
required-status jobs succeeded. This excludes short runs where the gate did
not require scenarios.

The report includes p50, p75, and p90 values for workflow admission delay,
queue-to-required-status latency, build job and build-step duration,
longest-shard duration, shard spread, and reset cost. It also reports each
organization's admission delay, selected scenario count, execution duration,
and every scenario's measured Go subtest duration in the JSON data. Percentile
calculations use the nearest-rank method.

Increase the search window when fewer than 20 eligible runs appear:

```sh
make baseline-e2e-ci E2E_BASELINE_SCAN=200
```

GitHub Actions metrics artifacts may expire before 20 eligible runs complete.
Collect and retain partial observations in the versioned weighted snapshot:

```sh
make collect-e2e-baseline
```

The target reads and updates
`test/e2e/baselines/weighted-v1-2026-09-observations.json`, deduplicates saved and
new runs by workflow run ID, and writes the current report to
`test/e2e/baselines/weighted-v1-2026-09.md`. It succeeds while the report is still
collecting so the updated files can be committed before source artifacts
expire. Saved observations remain eligible after their source artifacts are
deleted.

Commit the updated files regularly, for example twice a week, before the
10-day artifact retention expires. Collection remains an administrator task.

### Ongoing progress after a baseline is complete

Keep the completed historical baselines frozen. For ongoing collection use:

```sh
make collect-e2e-progress E2E_BASELINE_SCAN=150
make collect-e2e-progress E2E_PROGRESS_MODE=pr E2E_BASELINE_SCAN=150
```

The first command selects the current full-live weighted allocation (main,
plus any fully live PR/manual runs with the same allocation). The second
selects the current reduced-live PR allocation, including the exact replay
membership hash. A changed weight snapshot or replay list creates a separate
snapshot, never a mixed comparison. This target assumes weighted sharding;
use the baseline script's explicit allocation option for rollback data.

Files default to `.e2e-artifacts/progress/<allocation>-observations.json` and
`<allocation>.md`, with colons replaced by hyphens. These local files are
ignored, not automatically committed or backed up. Set `E2E_PROGRESS_DIR` to
a reviewed versioned directory when retaining observations in the repository;
collect and commit regularly before GitHub artifacts expire.

Unlike target-limited `collect-e2e-baseline`, this target uses `--refresh`:
every invocation scans for new eligible runs even after the target is full.
The cumulative JSON retains all saved runs; the report uses only the latest
`E2E_BASELINE_COUNT` runs (20 by default). Refresh does not prune history or
replace a saved successful run with a later failed/partial attempt. Full
same-attempt eligibility and allocation/cohort checks remain unchanged.

New observations include `cache_result`: `exact`, `fallback`, `cold`,
`uncached`, or `unknown`. The report groups build times by those categories.
Missing/expired logs and old observations without a category stay `unknown`;
duration is never used to guess a hit. This is a successful-full-run report,
not a cache hit-rate audit of all builds, and replay execution durations stay
in the separate replay artifacts. Main and PR wall-clock comparisons remain
observational, affected by queueing, source changes, and Konnect latency.

Collection defaults to the exact weighted allocation embedded in the current
checkout. `E2E_BASELINE_ALLOCATION` combines the algorithm version and the
weight snapshot SHA-256. A different strategy or weight hash is excluded from
collection; a mismatched saved file is rejected before it is overwritten.
Keep the 20-run `post-cache-2026-09-*` files as the pre-activation baseline.
Do not overwrite them with weighted or rollback observations.

Metrics schema 2 records `allocation_id` from the harness allocation sidecar,
not from requested environment settings. Missing or inconsistent metadata fails
metrics generation. Historical schema 1 metrics and saved observations without
allocation identity are explicitly treated as `modulo-v1`; mixed-shard runs
are rejected. Observation schema 2 is retained with an additive allocation ID
on newly saved documents and runs.

`E2E_BASELINE_COHORT` defaults to `cache-enabled`. This includes cache hits and
misses in builds containing the `Report Go cache status` step added by #2069.
Keep that step as the identification marker when changing the cache policy.
The `uncached` cohort contains builds without that step. A dependency change
may produce a cold build and still belongs to `cache-enabled`.

E2E builds retain setup-go's dependency-specific restore and successful-job
save. On an exact-key miss, a best-effort restore can reuse an older dependency
cache for the same Linux runner image, architecture, and exact Go version.
There is no broader cross-platform or cross-toolchain fallback. The fallback
uses only `go env GOMODCACHE` and `go env GOCACHE`, in setup-go's path order,
and its existing key namespace; no checkout, credentials, or built executables
are added to the cache. GitHub's existing branch access restrictions remain.
The offline cache test pins the reviewed setup-go revision. When updating the
action, review its namespace/path compatibility before updating that test pin.

Both binaries are always built and all tests still run. Go validates cached
compilation against its inputs; a fallback does not reuse a previous kongctl
executable. Missing caches or a failed/timed-out fallback restore simply leave
the normal build to run. The fallback restore has a two-minute limit, and a
successful job still saves its cache under the current dependency-specific
key through setup-go. Exact hits do not perform the additional restore.

The `Report Go cache status` log and build details artifact preserve the original
primary-key hit field and add `dependency-fallback-v1` with `exact`, `fallback`,
or `cold`, plus the fallback outcome and matched key. Compare these categories
separately when measuring build time; all still belong to `cache-enabled`.
This policy does not change scenario routing, resets, sharding, or org locks.

The collector rejects a saved file belonging to a different cohort and rejects
mixed records. Set `E2E_BASELINE_COHORT`, `E2E_BASELINE_OBSERVATIONS`, and
`E2E_BASELINE_REPORT` together when collecting a different cohort. Schema 2
adds cohort, source revision, attempt timestamps, and build/harness setup and
harness test timing. Older private copies must be recollected or explicitly
migrated from verified job metadata; their cohort is never guessed on load.

The Stage 0 files preserve the preliminary uncached baseline. They are frozen
below the original 20-run target, rather than filling that target with cached
runs. Regenerate their report offline with:

```sh
python3 scripts/e2e_baseline.py --cohort uncached --frozen \
  --observations test/e2e/baselines/stage0-2026-09-observations.json \
  --output test/e2e/baselines/stage0-2026-09.md
```

Latency uses the selected attempt's creation timestamp from GitHub's attempt
API. The original workflow creation timestamp is retained for provenance but
does not charge time between reruns as queue delay. Each workflow run ID
contributes one saved successful attempt; collection keeps saved observations
even when a later rerun occurs. Controlled same-commit reruns are useful cache
experiments, but should not be counted as independent baseline samples.

Gather the post-cache baseline before changing concurrency, assignment, or
reset policy. Those future changes require a new, explicitly identified
measurement period; the cache cohort alone does not distinguish them.

### Weighted sharding allocation and rollback

Full, sharded `.com` runs use weighted allocation by default. This is the
activation following #2094, which only reported proposals. Filtered, unsharded,
and `.tech` execution retains the original selector. No scenarios are removed,
no replay is introduced, and reset policy and concurrency are unchanged.

The scheduler reserves pinned load first, then assigns unpinned
scenarios in descending estimated-duration order to the least-loaded org.
Equal weights use scenario path order; equal loads use configured organization
order. Weighted execution lists are sorted by path. Counts need not be equal:
the goal is balanced estimated time, not balanced scenario counts.

The activation snapshot uses only the completed 20-run cache-enabled baseline.
Keep it fixed during the first 20 successful weighted runs. An intentional
refresh starts a new allocation identity and requires a separate observation
file; it is not routine maintenance during that experiment. To regenerate the
activation snapshot from its frozen source:

```sh
make refresh-e2e-weights
```

This writes `test/e2e/baselines/scenario-weights.json`. The generator uses one
attempt per workflow run (the highest supplied attempt),
rejects conflicting duplicate observations, and requires at least 10 finite,
positive, passing durations per scenario. Weights are conventional medians
rounded to milliseconds. Failed, skipped, and nonpositive durations do not
contribute. Removed scenarios are omitted on refresh.

New or insufficiently sampled scenarios use the median of established scenario
weights. If no scenario qualifies, all scenarios receive a uniform 1000ms
weight. Such a report is a balancing illustration, not a calibrated time
prediction. The snapshot records sample counts and source paths/SHA-256 hashes
and is embedded in the scenario test binary; selection never queries GitHub.
Refresh is explicit, not part of builds or baseline collection.

Weights do not learn from each run: unchanged manifests, organizations, and
weights produce the same assignments. Malformed activation weights fail before
execution instead of silently reverting to a different allocation. Malformed
observation records are rejected before replacing the snapshot.

For rollback, set the GitHub repository variable `KONGCTL_E2E_SHARD_STRATEGY`
to `modulo`. The target-selection job captures it once for the entire matrix;
already selected runs keep their allocation, and later runs use the original
selector, including its original ordering. Rollback works even if weights are
invalid. Set it to `weighted` (or remove the variable) to resume activation.
An invalid value fails the `.com` target-selection job. Locally the equivalent
override is `KONGCTL_E2E_SHARD_STRATEGY=modulo`.

Record rollback measurements separately, for example:

```sh
make collect-e2e-baseline E2E_BASELINE_ALLOCATION=modulo-v1 \
  E2E_BASELINE_OBSERVATIONS=.e2e-artifacts/rollback-observations.json \
  E2E_BASELINE_REPORT=.e2e-artifacts/rollback.md
```

For an exact snapshot identity without changing weights:

```sh
python3 scripts/e2e_weights.py --print-allocation-id
```

The build job runs the weighted scheduler tests using its already compiled
test binary and kongctl executable. This offline check covers the full
repository corpus, pin preservation, coverage, and identical full-pool reports
across matrix organizations; it never executes scenarios or contacts Konnect.
Run it locally with:

```sh
CGO_ENABLED=0 go test -tags=e2e ./test/e2e -run '^TestWeighted' -v
```

Each shard's detailed Markdown artifact identifies the allocation and compares
modulo and weighted estimated totals,
longest-shard time, and spread. Its `e2e-artifacts-*` artifact contains:

- `scenario-allocation.json`: the actual strategy and allocation identity.
- `proposed-scenario-assignments.json`: schema 2 full-pool comparison with
  `legacy` and `weighted` assignments plus the actual allocation identity.
  Schema 1 `current`/`proposed` fields from report-only runs are historical.
- `weighted-sharding-summary.md`: the human-readable comparison.

These are separate from `assigned-scenarios.txt` and `e2e-metrics.json`, which
describe actual weighted or modulo execution. Comparison-report errors produce
warnings; allocation metadata errors fail before execution so measurements
cannot silently be assigned to the wrong experiment.

Predictions exclude job overhead and assume that scenario durations transfer
across organizations and execution ordering. They are not measured savings.
Repeated runs from the same PR are correlated samples. Compare measured
longest-shard duration, spread, and overall latency with the completed
pre-activation baseline. Also inspect failed/cancelled workflows and beta
scenario failures: this successful-run collector alone cannot establish
reliability. Roll back promptly for coverage/pinning problems or reproducible
ordering-dependent failures; do not weaken assertions to retain speedups.

### Consolidated Actions run report

The `E2E run summary` job publishes the workflow's main human-readable report:

- Live and recorded replay assignments, passes, failures, skips, and advisory
  failures, together with coverage verification results.
- Scenario execution windows, individual shard runner and job durations, build
  duration, and initial queue wait reported separately. Execution windows exclude
  build and artifact upload, but include launch skew and waits between selected
  attempts; parallel shard durations are not added together as elapsed time.
- Observed request failures and subprocess deadlines by shard and scenario, with
  evidence paths and artifact links. Reset events say `recovered` only when the
  individual operation subsequently succeeds. `command_passed` indicates command
  completion and does not prove that a particular HTTP request recovered.
- Collapsed coverage, slowest scenarios, routing, cache, and artifact details.

The report selects the latest available attempt for each shard. A newer job with
missing evidence invalidates older successful evidence. Missing logs can
undercount transient events; missing results remain incomplete. Compare runner
measurements only across compatible scenario sets, environments, and allocation
strategies. Windows combining partial reruns can include time between attempts.

Artifacts retain the detailed evidence:

- `e2e-routing-<run>`: complete routing JSON and build/cache details.
- `e2e-report-<run>-<attempt>-<org>`: compact shard JSON and detailed Markdown.
- `e2e-coverage-<run>-<attempt>`: detailed coverage verification output.
- `e2e-run-summary-<run>-<attempt>`: rendered summary, combined report JSON,
  verification job outcomes, and GitHub job timing metadata.
- Existing `e2e-artifacts-*` and `e2e-metrics-*` artifacts retain full diagnostics
  and metrics. The summary job downloads only compact evidence.

Reset event records contain classification, timing, attempt, and outcome, without
request URLs, resource IDs, raw errors, or response bodies. Counts represent
observed failed attempts, not distinct network incidents. Reporting does not
change retry eligibility, deadlines, or the required E2E gate.

GitHub controls summary card ordering. Our workflow publishes one consolidated
card instead of separate routing and shard cards. StepSecurity Harden Runner
retains its own monitoring and summaries, which may appear before this report.
