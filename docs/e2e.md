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
- `KONGCTL_E2E_CAPTURE`: Per-command artifact capture. `0` disables it.
- `KONGCTL_E2E_JSON_STRICT`: When `1`, JSON parsing fails on unknown fields.
  Default: lenient.
- `KONGCTL_E2E_ARTIFACTS_DIR`: Root folder for artifacts for this run.
  Default: a temp dir.
- `KONGCTL_E2E_BIN`: Path to an existing `kongctl` binary to skip building.
- `KONGCTL_E2E_RESET`: Reset the Konnect org before tests. Destructive.
  Defaults to enabled; set to `0` or `false` to disable.
- `KONGCTL_E2E_KONNECT_BASE_URL`: Base URL for Konnect API. Default:
  `https://us.api.konghq.com`.
- `KONGCTL_E2E_HTTP_TIMEOUT`: Per-request timeout for raw Konnect HTTP helpers
  used by scenario create/delete flows. The harness also writes the same
  value into the generated `e2e.http-timeout` profile setting so
  SDK-backed CLI commands share the same default. Default: `15s`.
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
  reset API calls. Default: `15s`.
- `KONGCTL_E2E_RESET_TIMEOUT`: Total time budget for a single org reset before
  the harness aborts the remaining reset steps. Default: `3m`.
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

Scenario selection and sharding:

- `KONGCTL_E2E_SCENARIO`: Exact scenario selector. Examples:
  `portal/edit`, `scenarios/portal/edit`, or a full `scenario.yaml` path.
- `KONGCTL_E2E_SHARD_INDEX`: Zero-based shard index for this test process.
- `KONGCTL_E2E_SHARD_TOTAL`: Total number of shards in the run.
- `KONGCTL_E2E_MATRIX_ORG`: Optional diagnostic label for the current CI job.

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
- Sanitization: token-like env vars are redacted in `env.json` and logs.
- HTTP dumps: when Konnect SDK dump env vars are enabled, the harness stores
  each exchange under the command’s `http-dumps/` directory.
- Observations: `observation.json` is attached to captured commands for apply
  summaries and read observations.

### CI Notes

The E2E GitHub Actions workflow scales wall-clock time by running one matrix
job per Konnect org. Each job gets:

- one environment-scoped PAT
- the shared Konnect base URL from the repository variable
- one shard index
- the common shard total from the matrix size

The workflow derives sharding directly from GitHub Actions strategy context:

- `KONGCTL_E2E_SHARD_INDEX=${{ strategy.job-index }}`
- `KONGCTL_E2E_SHARD_TOTAL=${{ strategy.job-total }}`

The workflow also exposes the HTTP timeout and retry knobs above as repository
or organization variables of the same names, so CI can tune reset and HTTP
behavior without changing Go code.

For transport debugging, the workflow currently defaults to:

- `KONGCTL_E2E_HTTP_TCP_USER_TIMEOUT=60s`
- `KONGCTL_E2E_HTTP_RECYCLE_CONNECTIONS_ON_ERROR=1`

Override either one with a repository or organization variable if you want to
disable or change the experiment for CI runs.

Each matrix leg writes an `assigned-scenarios.txt` manifest into its artifact
directory. A final `E2E Verify` job downloads those manifests and fails unless:

- every shard index from `0` through `KONGCTL_E2E_SHARD_TOTAL-1` appears once
- no scenario appears in more than one shard manifest
- the combined manifest set matches the full discovered scenario list

That verification step is the guardrail that keeps sharding regressions from
silently dropping or duplicating scenario coverage.

The workflow summary also includes aggregated execution results from all shard
jobs, including assigned scenario count, pass/fail/skip totals, per-shard
durations, exit codes, and a failed-scenarios table when applicable.

For temporary GitHub-runner network debugging, the workflow can also capture
packet traces for Konnect endpoints:

- set the `workflow_dispatch` input `capture_tcpdump=true`, or
- set the repository or organization variable `KONGCTL_E2E_CAPTURE_TCPDUMP=1`

When enabled, each matrix job records a `tcpdump/` directory in the normal E2E
artifact bundle containing:

- `konnect.pcap`: packet capture filtered to the regional Konnect host and
  `global.api.konghq.com` on port `443`
- `tcpdump.log`: tcpdump startup and shutdown output
- `context.txt`: runner host, DNS resolution, interfaces, and routes

The org pool is defined as JSON in the repository or organization variable
`KONGCTL_E2E_ORGS_JSON`. Example:

```json
[
  { "org_name": "kongctl-e2e-us-1" },
  { "org_name": "kongctl-e2e-us-2" }
]
```

Each `org_name` must match a GitHub Actions environment that defines the
secret `KONGCTL_E2E_KONNECT_PAT`.

Set the shared Konnect API endpoint as the repository or organization variable
`KONGCTL_E2E_KONNECT_BASE_URL`.

If the org-pool variable is unset, the workflow falls back to a single-org
matrix entry named `default`, which is useful during migration if you still
have a `default` environment or a temporary repository-level
`KONGCTL_E2E_KONNECT_PAT` secret in place.

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
