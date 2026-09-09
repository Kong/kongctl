# One-scenario replay experiment

Related: [#2058](https://github.com/Kong/kongctl/issues/2058).

This directory does **not** enable replay in ordinary PR or main E2E runs.
Both still execute the entire scenario against live Konnect. The separate
`E2E replay experiment` workflow exercises only `control-plane/get`.

## Current evidence and limitation

`bootstrap.json` is a manually reconstructed **transport fixture**, not a
recorded HTTP cassette. Its request sequence and response shape were informed
by the successful live run linked in `source`. That run's debug artifacts did
not retain full HTTP bodies. Inventing a recording provenance would be wrong.
The fixture uses synthetic IDs, timestamps and control-plane hostnames.

It exercises the unchanged scenario: declarative create, list, name/ID lookup,
not-found behavior, Helm output, and declarative delete. It proves that the
existing CLI and scenario can execute through trusted local TLS without
product-code changes. It does **not** establish parity with a real recording,
recording sanitizer completeness, or a measured live-to-replay speedup.

Before expanding eligibility or replacing any PR live validation, create and
review a real cassette using the manual workflow below. The initial candidate
must also pass the workflow's three isolated replay executions.

## Local transport evaluation

Requirements: Go, Python 3, OpenSSL and the repository's normal build tools.

```sh
make test-e2e-metrics
make check-e2e-replay
make test-e2e-replay
```

The last command explicitly opts into the bootstrap fixture. It builds once
and runs the existing scenario test binary, not a replacement mock CLI.
No personal Konnect credentials or profiles are inherited by the subprocess.

Local execution restricts the proxy to two known Konnect destinations but is
not an OS network sandbox. On Linux, after building, use the CI-equivalent
network isolation (requires sudo and util-linux):

```sh
bash scripts/e2e-replay-isolated.sh --allow-bootstrap \
  --cassette test/e2e/scenarios/control-plane/get/replay/bootstrap.json
```

This creates a fresh network namespace with only loopback, drops root and all
capabilities, and then starts the wrapper, proxy and kongctl subprocesses.
Builds and artifact downloads happen outside that namespace. There is no
silent fallback if isolation is unavailable. Normal local mode is useful in
restricted development environments where namespace creation is prohibited.

## Recording a real cassette

Once GitHub makes the manual workflow available on the default branch:

```sh
gh workflow run e2e-replay.yaml --repo Kong/kongctl \
  --ref YOUR_REVIEWED_BRANCH -f mode=record
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
instructions to re-record/review. Bootstrap updates must remain labeled as
such; do not change only the fingerprint without reviewing expectations.

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

The fixture currently has eleven interactions, all in the regional control
plane API. Keep the initial experiment this narrow. Before expansion:

1. Obtain the first real recording and establish repeated live/replay parity.
2. Compare `scenario_seconds` (excludes certificate setup and live resets) and
   `elapsed_seconds` (includes wrapper/setup/reset costs) in summaries. Report
   replay match failures and cassette maintenance effort as well as speed.
3. Add explicit area/eligibility metadata and a deterministic manifest router.
4. Integrate separate PR live/replay jobs with complete coverage verification.
5. Add the agent's live-area selection with all-live fallback and overrides.

Do not add unordered interaction groups until an actual eligible scenario
needs them. Do not add resource-specific emulation handlers. Do not change
main's full live validation or refresh weighted-sharding data with replay
timings. The experiment deliberately publishes no `e2e-metrics-*` artifacts.
