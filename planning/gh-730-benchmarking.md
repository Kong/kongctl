# GH-730 Declarative Benchmarking Plan

## Goal

Issue `#730` requests a repeatable performance benchmarking capability for the
declarative engine, focused on larger declarative configuration sets against the
real Konnect SaaS API. The benchmark system should establish baselines, support
tracking over time, and help detect regressions as the GA release approaches.

The benchmark capability needs to work in two environments:

- GitHub Actions CI, where it will run most often
- local developer machines, where contributors need a supported way to run the
  same benchmark flows manually

## Issue Summary

The issue body asks for a benchmarking process that:

- creates test case configuration inputs for `small`, `medium`, `large`, and
  `XL`
- exercises both single-file and multiple-file declarative inputs
- reuses `scripts/command-analyzer.sh` for wall-clock timing and API request and
  response log auditing
- adds a GitHub workflow to run the benchmarks
- tracks benchmark results over time
- tracks regressions in an issue

The automated issue triage comment is useful as implementation guidance, but it
should be treated as advisory rather than canonical requirements.

## Interpreted Acceptance Criteria

The feature is likely complete when all of the following are true:

- benchmark fixtures exist for multiple declarative workload sizes
- the benchmark process supports both single-file and multi-file inputs
- benchmark execution captures runtime and API behavior in a structured way
- a GitHub Actions workflow can run the benchmark suite
- results can be compared over time against a known baseline
- regressions can be surfaced automatically or semi-automatically

## Repository Assessment

### Existing E2E harness is the strongest foundation

The repository already includes a real-Konnect E2E harness with strong support
for repeatable execution and artifact capture:

- builds `kongctl` once per run and reuses the binary
- creates isolated config state per run
- runs against live Konnect instead of mocks
- supports destructive org reset
- captures per-command artifacts, including:
  - `stdout.txt`
  - `stderr.txt`
  - `meta.json`
  - `kongctl.log`
  - optional HTTP dumps

Key references:

- `test/e2e/harness/builder.go`
- `test/e2e/harness/cli.go`
- `test/e2e/harness/reset.go`
- `test/e2e/harness/step.go`
- `docs/e2e.md`

This is already much closer to a benchmark runner than the integration suite.

### Scenario DSL already models useful benchmark flows

The scenario system under `test/e2e/scenarios` can already express:

- reset org
- apply declarative config
- re-apply declarative config
- mutate inputs between steps
- assert no-op behavior
- run arbitrary external commands

This is valuable because benchmark cases will likely need to cover both:

- initial apply cost
- idempotent re-apply cost

There are currently many scenario fixtures and patterns to borrow from, notably
portal and declarative lifecycle scenarios.

Key references:

- `test/e2e/scenarios_test.go`
- `test/e2e/harness/scenario/types.go`
- `test/e2e/harness/scenario/run.go`
- `docs/e2e-scenarios.md`
- `docs/e2e-scenarios-getting-started.md`

### Existing shell analyzer is useful but not sufficient by itself

The repo already has:

- `scripts/command-analyzer.sh`
- `scripts/http-log-summary.sh`

Current strengths:

- wall-clock timing for a `kongctl` invocation
- request and response log parsing
- method and route summaries
- timing summaries from log duration fields

Current limitations:

- output is human-readable rather than machine-readable
- it is a standalone shell entrypoint rather than part of the E2E artifact model
- it is not yet organized as a benchmark suite runner

The current scripts are good operator tools, but they are not yet a durable
benchmarking system.

### CI workflow patterns already exist and should be reused

The repository already has a mature E2E workflow that handles:

- build-once binary packaging
- matrix execution
- environment-scoped PAT usage
- org-scoped concurrency
- artifact upload
- final aggregation and verification

Key reference:

- `.github/workflows/e2e.yaml`

This matters because benchmarking should follow the same operational model
rather than introducing a completely separate CI pattern.

### Integration tests are not the correct substrate

The integration suite is largely mock-based. The real SDK path is not a mature,
ready-to-use basis for live performance measurement.

Key reference:

- `test/integration/declarative/sdk_helper_test.go`

Conclusion: the benchmark feature should live beside E2E infrastructure, not
inside the current integration-test framework.

## Proposed Direction

### Build a dedicated benchmark runner

Rather than forcing benchmarks into regular tests or only shell scripts, add a
dedicated benchmark runner under the repository test tooling that reuses E2E
harness internals.

Recommended shape:

- a small Go command or package under `test/benchmarks/`
- reuse:
  - binary preparation
  - Konnect PAT and base URL handling
  - org reset behavior
  - artifact directory structure
  - command execution and log capture

Why this is preferable:

- benchmark runs are destructive and external, which is a poor fit for normal
  `go test -bench`
- the repository already has reusable harness code for exactly this style of
  execution
- structured result generation is easier in Go than in large shell scripts

### Generate benchmark fixtures deterministically

The benchmark system needs workload sizes such as:

- `small`
- `medium`
- `large`
- `xl`

Those should be generated from committed templates or logical fixture
descriptions rather than committing very large static YAML payloads.

Recommended approach:

- commit seed fixture definitions and generator inputs
- generate runtime fixture material into the benchmark artifacts directory
- emit both:
  - single-file layout
  - multi-file layout

This keeps repo size manageable while making workloads reproducible.

### Start with two benchmark phases per case

For each benchmark case, the first useful flow is:

1. reset org
2. measured apply against empty state
3. measured re-apply against matching state

That gives two high-value signals:

- create/apply cost
- idempotency/no-op cost

Possible later extensions:

- `plan` benchmark runs
- `sync` benchmark runs
- delete-heavy scenarios
- specialized resource family suites

### Prefer request-count baselines over raw time baselines

Since the benchmarks run against real Konnect SaaS, raw wall-clock duration will
contain more noise than local-only benchmarks.

The most stable performance signals are likely:

- total API request count
- per-route request breakdown
- write vs read behavior

Wall-clock time is still important, but it should be treated as a noisier,
secondary signal and likely compared using medians or repeated runs rather than
single hard thresholds.

### Keep results structured

The benchmark runner should emit structured outputs such as:

- `results.json`
- `summary.md`
- per-case API route breakdown JSON
- raw command artifacts and logs

This likely implies either:

- extending `scripts/http-log-summary.sh` with a machine-readable JSON mode, or
- moving the log parsing into Go while keeping the shell script as a thin
  convenience wrapper

## Recommended Initial Scope

For v1, keep the benchmark suite narrow and stable.

Suggested resource focus:

- portal
- api
- api version
- api publication
- application auth strategy
- optionally control plane

Reasoning:

- these are representative declarative resources
- they exercise planner and executor behavior
- they avoid turning the benchmark into a content-blob upload benchmark

Areas to defer until later unless specifically required:

- huge binary assets
- Gmail-backed portal application scenarios
- event gateway coverage
- every supported resource type in one initial suite

## CI Strategy

Recommended workflow design:

- new dedicated workflow, separate from the standard E2E workflow
- triggers:
  - `workflow_dispatch`
  - scheduled weekly run
- reuse the same environment and matrix patterns already established in E2E
- use dedicated benchmark orgs if available
- upload artifacts for every run

Recommended initial posture:

- informational and non-blocking at first
- compare results to an approved baseline
- open or update an issue when thresholds are exceeded

Possible later evolution:

- smaller targeted benchmark smoke checks for PRs
- benchmark summaries posted to GitHub Discussions or workflow summaries

## Local Developer Workflow

The local benchmark command should mirror CI as closely as possible.

Recommended developer entrypoints:

- `make benchmark-declarative`
- `make benchmark-declarative-case CASE=medium-single`

Expected env vars should match the E2E conventions where possible, for example:

- `KONGCTL_E2E_KONNECT_PAT`
- `KONGCTL_E2E_KONNECT_BASE_URL`

This keeps the operational model familiar and avoids introducing another set of
credentials and environment conventions.

## Suggested Benchmark Dimensions

Initial matrix:

- size:
  - `small`
  - `medium`
  - `large`
  - `xl`
- layout:
  - `single-file`
  - `multi-file`
- phase:
  - `apply_create`
  - `apply_noop`

Potential additional dimensions later:

- repetitions per case
- region/base URL
- `plan` vs `apply` vs `sync`
- resource family subsets

## Baseline and Regression Tracking

Recommended model:

- keep raw artifacts for each run
- emit a normalized structured result for the suite
- compare the latest run against a checked-in approved baseline JSON

Recommended regression policy:

- request-count regressions are primary
- wall-clock regressions are secondary
- use tighter thresholds for request count changes
- use looser thresholds for duration changes because SaaS timing is noisier

Potential long-term history options:

- checked-in baseline JSON snapshots
- workflow summary markdown
- pinned GitHub Discussion with historical result summaries
- issue creation or issue comment updates on regression

## Open Questions

These are still unresolved and should be answered before implementation is
locked in:

1. Should v1 cover only core declarative resources, or also event gateways and
   org or team resources?
2. Are dedicated benchmark Konnect orgs available, separate from the normal E2E
   org pool?
3. Should the first version remain informational only, or should some portion
   run as a PR-time quality gate?
4. How should approved baselines be stored and reviewed over time?
5. Is GitHub Discussion posting required in v1, or can it follow after the
   runner and workflow exist?

## Recommended Next Step

Before implementation starts, convert this note into a more concrete execution
plan with:

- target file and package layout
- benchmark fixture generation strategy
- result schema
- CI workflow shape
- baseline comparison rules
- phased delivery plan for v1 and later enhancements
