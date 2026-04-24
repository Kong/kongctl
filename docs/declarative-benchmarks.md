# Declarative Benchmarks

The declarative benchmark runner measures live Konnect API behavior for larger
declarative configuration sets. It is local-first: GitHub Actions runs the same
`make` targets that developers can run on their machines.

The initial benchmark suite focuses on API resources and API documents. The
runner generates deterministic fixtures for these case sizes:

- `small`
- `medium`
- `large`
- `xl`

Each size is rendered in both layouts:

- `single-file`
- `multi-file`

Each case resets the target Konnect org, applies the generated configuration,
then immediately reapplies the same configuration to measure no-op behavior.

## Credentials

Use a dedicated Konnect org for benchmark runs. Configure the benchmark PAT with
one of these environment variables:

```sh
export KONGCTL_BENCHMARK_KONNECT_PAT="$(cat ~/.konnect/benchmark-pat)"
```

or, for compatibility with existing E2E setup:

```sh
export KONGCTL_E2E_KONNECT_PAT="$(cat ~/.konnect/benchmark-pat)"
```

The benchmark base URL defaults to `https://us.api.konghq.com`. Override it
with:

```sh
export KONGCTL_BENCHMARK_KONNECT_BASE_URL="https://us.api.konghq.com"
```

## Local Execution

Run the full declarative benchmark suite:

```sh
make benchmark-declarative
```

Run one case:

```sh
make benchmark-declarative-case CASE=medium-single
```

Accepted case selectors include:

- `all`
- a size, such as `small` or `xl`
- a layout, such as `multi-file`
- a case name, such as `large-multi-file`
- the shortened layout alias, such as `large-multi`

Pass additional runner flags with `BENCHMARK_FLAGS`:

```sh
make benchmark-declarative-case CASE=small-single \
  BENCHMARK_FLAGS="--command-timeout 10m"
```

Artifacts are written under `.benchmark-artifacts/<timestamp>` by default. The
latest run is linked from `.latest-benchmark`.

The benchmark runner forces measured `kongctl apply` commands to use debug
logging by default so HTTP request and response log lines are available for
metrics. Override this only for troubleshooting:

```sh
export KONGCTL_BENCHMARK_LOG_LEVEL=trace
```

## Results

Every run writes:

- `results.json`: structured suite, case, phase, duration, and HTTP metrics
- `summary.md`: Markdown summary suitable for workflow summaries or issues
- per-command artifacts under `benchmarks/<case>/commands/`
- generated fixture files under `benchmarks/<case>/inputs/`
- `http-metrics.json` next to each measured command

The primary regression signal is HTTP request count. Wall-clock duration is also
tracked, but it is noisier because the benchmark runs against SaaS APIs.
The suite duration includes fixture generation and destructive org reset. The
per-phase durations measure only `kongctl apply` commands.

## Baseline Comparison

Compare a run against a previous `results.json`:

```sh
make benchmark-declarative-case CASE=medium-single \
  BENCHMARK_FLAGS="--baseline path/to/results.json"
```

Request-count regressions can fail the run:

```sh
make benchmark-declarative-case CASE=medium-single \
  BENCHMARK_FLAGS="--baseline path/to/results.json --fail-on-regression"
```

The default allowed request-count increase is five percent. Duration increases
greater than fifty percent are reported in the comparison summary but do not
fail the run.

## GitHub Actions

The `Declarative Benchmark` workflow is manual-only. It accepts:

- `case`: case selector passed to the local runner
- `command_timeout`: timeout for each measured `kongctl` command
- `benchmark_flags`: extra runner flags

Configure these in the `benchmark` environment:

- `KONGCTL_BENCHMARK_KONNECT_PAT`
- optional `KONGCTL_BENCHMARK_KONNECT_BASE_URL`

The workflow uploads benchmark artifacts and writes `summary.md` to the GitHub
Actions job summary.

## Result Storage Options

For now, artifacts are the source of truth. They preserve raw command output,
logs, generated fixtures, structured metrics, and the human summary.

Clean follow-up storage options are:

- **Issue ledger**: keep one tracking issue for benchmark history. Each run
  appends `summary.md`, links the artifact, and labels request-count
  regressions.
- **Discussion ledger**: keep one discussion for benchmark history. This is
  better for long-running performance records and less noisy than issue
  comments.
- **Checked-in baseline**: commit an approved `results.json` snapshot for the
  current suite. This gives the strictest review path for baseline updates.

The issue or discussion ledger should store summaries and links. The raw
artifact bundle should remain attached to the workflow run so investigations can
inspect command logs and generated inputs.
