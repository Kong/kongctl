# One-scenario replay experiment

Related: [#2058](https://github.com/Kong/kongctl/issues/2058).

This cassette is enabled for ordinary same-repository `.com` PR runs through
`test/e2e/replay-scenarios.json`. Main remains fully live. See
[PR replay policy](../../../../replay-policy.md) for routing and force-live rules.
The separate `E2E replay experiment` workflow records and validates candidates.

## Current evidence and limitation

`cassette.json` is a sanitized live HTTP recording from
[run 34377519108](https://github.com/Kong/kongctl/actions/runs/34377519108).
The recorder acquired the existing acceptance-3 lock, reset the organization,
ran the unchanged scenario successfully, and completed its final reset. Its
candidate then passed three replay executions in a loopback-only namespace.

The eleven interactions cover declarative create, list, name/ID lookup,
not-found behavior, Helm output, and declarative delete. IDs and generated
control-plane hostnames are sanitized; request fields remain strict.
The bootstrap transport fixture used during initial development has been
removed in favor of this genuine recording.

| Measurement | Seconds |
| --- | ---: |
| Live scenario through recorder, excluding before/after reset | 5.770 |
| Replay scenario, executions 1 / 2 / 3 | 1.567 / 1.471 / 1.480 |
| Live wrapper including TLS setup and before/after reset | 13.161 |
| Replay wrapper, executions 1 / 2 / 3 | 2.107 / 2.142 / 2.034 |

The replay scenario median is about 74% lower than this one recorded live
execution. This is an initial feasibility result, not a statistically robust
speedup or an estimate for the full suite. Recording itself adds proxy and
upstream-connection overhead. CI build/download/queue costs are outside these
measurements. These measurements predate ordinary PR replay routing.

## Local replay

Requirements: Go, Python 3, OpenSSL and the repository's normal build tools.

```sh
make test-e2e-metrics
make check-e2e-replay
make test-e2e-replay
```

The last command uses the reviewed live cassette. It builds once and runs
the existing scenario test binary, not a replacement mock CLI.
No personal Konnect credentials or profiles are inherited by the subprocess.

Local execution restricts the proxy to two known Konnect destinations but is
not an OS network sandbox. On Linux, after building, use the CI-equivalent
network isolation (requires sudo and util-linux):

```sh
bash scripts/e2e-replay-isolated.sh
```

This creates a fresh network namespace with only loopback, drops root and all
capabilities, and then starts the wrapper, proxy and kongctl subprocesses.
Builds and artifact downloads happen outside that namespace. There is no
silent fallback if isolation is unavailable. Normal local mode is useful in
restricted development environments where namespace creation is prohibited.

## Recording a real cassette

Dispatch the registered workflow against a trusted repository branch:

```sh
gh workflow run e2e-replay.yaml --repo Kong/kongctl \
  --ref YOUR_REVIEWED_BRANCH -f mode=record -f scenario=control-plane/get
```

Only dispatch trusted repository code. The recording job has access to the
existing `kongctl-acceptance-3` environment's E2E PAT. It holds precisely
`konnect-e2e-kongctl-acceptance-3`, the same concurrency group as that live
shard, with queuing enabled and cancellation disabled. No new organization is
provisioned. Recording waits for live work and blocks other work on that org
through before-reset, scenario execution and after-reset. Existing live runs
continue to reset before use if a recorder is forcibly terminated.

The recorder uses the existing reset executable before and after the scenario.
The scenario's reset command is disabled inside the recorded execution: each
recording/replay starts from a fresh scenario session instead. Its actual
create/get/delete operations remain mandatory interactions.

The forwarding proxy alone receives the real PAT; kongctl receives a dummy
PAT. Upstream traffic is HTTPS to the fixed regional/global Konnect allowlist.
The proxy records HTTP requests and responses in memory, discarding headers
other than the explicit JSON response contract. It never redirects upstream
requests or forwards arbitrary CONNECT destinations. Raw logs, certificates,
keys and bodies are temporary and are **not** workflow upload artifacts.

Only after scenario success, successful cleanup, sanitization and schema
validation is `replay-candidate/candidate-cassette.json` uploaded. A separate
job replays that candidate three times without credentials or external
networking. Do not promote the candidate unless that job also passes.

Download the candidate:

```sh
gh run download RUN_ID --repo Kong/kongctl --name replay-candidate \
  --dir .e2e-artifacts/replay-review
```

Review the interaction diff and sensitive-data checks before installing it as
`cassette.json` beside this README. Do not just accept a new recording because
live assertions passed: a request-construction regression can pass incomplete
assertions. Check it and replay locally with:

```sh
python3 scripts/e2e-replay.py check
python3 scripts/e2e-replay.py replay \
  --test-binary .e2e-artifacts/replay-bin/e2e.test
```

Commit the reviewed cassette and scenario changes together. No command in
the recorder automatically overwrites a reviewed cassette or commits files.
Artifacts expire after ten days; download and commit approved data promptly.

## Drift detection and matching contract

Every cassette/fixture contains a version, scenario name, source kind,
source commit/run and fingerprint. The fingerprint covers sorted relative
filenames and bytes of every file in the scenario directory except `replay/`.
Additions, deletions and assertion/fixture edits invalidate it. Symlinks are
rejected. The prototype rejects external paths/URLs, custom commands, custom
environment dependencies and org pins; expanding those needs a reviewed
dependency model, not just a new hash.

`make test-e2e-metrics` validates every cassette in this directory regardless
of live/replay routing. This runs in ordinary CI. A stale fixture fails with
instructions to re-record/review. Do not change only the fingerprint without
reviewing expectations. Bootstrap fixtures are not accepted as recordings.

Source-code changes do not invalidate the fingerprint: exercising changed
kongctl against unchanged expectations is the purpose of replay.

Matching is strict and ordered by interaction. Method, endpoint role, path,
query values and JSON body must match; only query ordering and JSON formatting
are ignored. Extra requests, mismatches and unused interactions fail the run
even if the CLI treats an error as an expected scenario failure. JSON duplicate
keys, non-finite numbers, streaming bodies, redirects and transient responses
are unsupported. No wildcard matching or live fallback exists.

Recording replaces UUIDs consistently across request/response references and
replaces generated control-plane/telemetry hostnames. The replay matcher does
**not** normalize arbitrary request IDs: a wrong ID must fail. Sensitive field
names, credential-like strings, emails and PEM material block publication.
Those conservative checks are not a universal PII detector. Review remains
mandatory, especially before allowing another resource type or normalizer.

## Architecture and next expansion

The Python wrapper owns an HTTP CONNECT proxy on loopback. The inner connection
uses a run-local TLS certificate trusted only by the subprocess through
`SSL_CERT_FILE`. Existing regional/global base URLs remain valid Kong HTTPS
URLs; `HTTPS_PROXY` routes them locally without DNS or hosts-file changes.
Replay cannot forward externally. Record mode uses a separate, explicit
upstream exchange path. The cassette engine knows HTTP, not control-plane
CRUD semantics or a simulated database.

The cassette currently has eleven interactions, all in the regional control
plane API. PR routing is deterministic, with no agent or changed-code mapping.
Compare `scenario_seconds` (excludes certificate setup and live resets) and
`elapsed_seconds` (includes wrapper/setup/reset costs) in summaries. Report
replay match failures and cassette maintenance effort as well as speed.

Do not add unordered interaction groups until an actual eligible scenario
needs them. Do not add resource-specific emulation handlers. Do not change
main's full live validation or refresh weighted-sharding data with replay
timings. The experiment deliberately publishes no `e2e-metrics-*` artifacts.
