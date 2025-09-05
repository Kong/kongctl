## E2E Test Harness

This repository includes an end-to-end (E2E) testing harness for `kongctl` that builds the CLI once per test run, executes commands against real Konnect, and captures detailed artifacts for triage.

- Default profile: `e2e`
- Default output: JSON (tests can override)
- Isolation: Each test uses its own `XDG_CONFIG_HOME` under a per-run artifacts directory
- No mocks: Tests call real Konnect; they will skip if required tokens are missing

### Quick Start

- Run all E2E tests (skips auth tests if PATs not set):

```
make test-e2e
```

- Run with verbose logs and a user PAT (opt-in test):

```
KONGCTL_E2E_LOG_LEVEL=debug KONGCTL_E2E_RUN_USER_ME=1 KONGCTL_E2E_KONNECT_PAT=$(cat ~/.konnect/your-user-pat) make test-e2e
```

- Reuse a prebuilt binary instead of building during tests:

```
make build
KONGCTL_E2E_BIN=./kongctl make test-e2e
```

### Environment Variables

- KONGCTL_E2E_LOG_LEVEL: Harness and CLI log level (trace|debug|info|warn|error). Default: `warn`.
- KONGCTL_E2E_OUTPUT: Default CLI output (json|yaml|text). Default: `json`.
- KONGCTL_E2E_CAPTURE: Per-command artifact capture. `0` disables. Default: enabled.
- KONGCTL_E2E_JSON_STRICT: When `1`, RunJSON fails on unknown fields. Default: lenient.
- KONGCTL_E2E_ARTIFACTS_DIR: Root folder to store artifacts for this run. Default: a temp dir.
- KONGCTL_E2E_BIN: Path to an existing `kongctl` binary to skip building (copied into artifacts/bin when possible).
- KONGCTL_E2E_RESET: Reset the Konnect org before tests (destructive). Defaults to enabled; set to `0`/`false` to disable.
- KONGCTL_E2E_KONNECT_BASE_URL: Base URL for Konnect API (default `https://us.api.konghq.com`).

Authentication token:

- KONGCTL_E2E_KONNECT_PAT: PAT used by the `e2e` profile for authenticated tests (e.g., `get me`, declarative apply).

### Test Selection

- `Test_VersionFull_JSON`: Always runs. Validates JSON output of `version --full`.
- `Test_GetMe_JSON_UserPAT`: Opt-in. Requires both `KONGCTL_E2E_RUN_USER_ME=1` and `KONGCTL_E2E_KONNECT_PAT`.
- `Test_Declarative_Apply_Portal_Basic_JSON`: Runs when `KONGCTL_E2E_KONNECT_PAT` is set; applies the basic portal example and verifies it via `get portals`.

Run only "get me" tests:

```
go test -v -tags=e2e ./test/e2e -run GetMe
```

### Artifacts Layout

Each test run creates a single artifacts directory (printed by the Makefile after the run and recorded in `run.log`). Example structure:

```
<artifacts_dir>/
  bin/
    kongctl                 # built or copied binary
  run.log                   # harness logs (also emitted to STDERR when log level allows)
  tests/
    Test_VersionFull_JSON/
      config/
        kongctl/
          config.yaml       # profile config written by the harness
      commands/
        000-version/
          command.txt
          stdout.txt
          stderr.txt
          env.json          # sanitized environment snapshot
          meta.json         # includes config_dir and config_file
    Test_GetMe_JSON_UserPAT/
      config/
      commands/
        000-get_me/
          ...
    Test_Declarative_Apply_Portal_Basic_JSON/
      config/
      inputs/
        portal.yaml         # manifest written by the test for reproducibility
      commands/
        000-apply/
          command.txt
          stdout.txt
          stderr.txt
          env.json
          meta.json
```

The harness keeps artifacts by default for easy triage and CI upload.

### Behavior & Conventions

- Build once: The binary is built (or copied from `KONGCTL_E2E_BIN`) once and reused.
- Default JSON: Harness injects `-o json` unless you pass `--output/-o`.
- Log level: Harness injects `--log-level <KONGCTL_E2E_LOG_LEVEL>` unless you pass one.
- Profile config: The harness writes `<profile>:{ output:<...>, log-level:<...> }` into `config.yaml` to mirror defaults.
- Sanitization: Token-like env vars (`PAT`, `TOKEN`, `SECRET`, `PASSWORD`) are redacted in `env.json` and logs.

### CI Notes (GitHub Actions)

- Provide `KONGCTL_E2E_KONNECT_PAT` as a secret to enable authenticated tests.
- Optionally set `KONGCTL_E2E_RUN_USER_ME=1` if you want to include the user-profile `get me` test.
- Upload `<artifacts_dir>` as a workflow artifact for post-run analysis.
- You can set `KONGCTL_E2E_ARTIFACTS_DIR=$RUNNER_TEMP/kongctl-e2e` to make artifact paths predictable.

### Troubleshooting

- Enable verbose logs: `KONGCTL_E2E_LOG_LEVEL=debug`.
- Inspect `<artifacts_dir>/run.log` for created paths, command lines, and durations.
- Check per-command `command.txt`, `stderr.txt`, and `meta.json` for the invoked command, exit codes, and context.
- If JSON parsing fails due to extra fields, either add the fields to your test struct, or keep default lenient mode (do not set `KONGCTL_E2E_JSON_STRICT`).
