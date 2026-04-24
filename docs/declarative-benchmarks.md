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

Repeat each selected case to collect local samples:

```sh
make benchmark-declarative-case CASE=medium \
  BENCHMARK_FLAGS="--repeat 3"
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
- `summary.txt`: terminal-oriented summary printed by the `make` target
- `dashboard.md`: generated discussion dashboard body
- `regressions.md`: generated regression issue body
- `regressions.json`: machine-readable regression status
- `history-report.json`: detailed current-vs-history report
- per-command artifacts under `benchmarks/<case>/commands/`
- generated fixture files under `benchmarks/<case>/inputs/`
- `http-metrics.json` next to each measured command

The primary regression signal is HTTP request count. Wall-clock duration is also
tracked, but it is noisier because the benchmark runs against SaaS APIs.
The suite duration includes fixture generation and destructive org reset. The
per-phase durations measure only `kongctl apply` commands.

When `--history-dir` or `KONGCTL_BENCHMARK_HISTORY_DIR` is set, the runner scans
prior `results.json` files under that directory. If the directory has a `runs/`
subdirectory, only `runs/` is scanned so `latest/` copies are not counted twice.

Request-count regressions compare the current median request count to recent
history. Duration regressions compare the current median wall-clock duration to
recent history using the larger of:

- the configured duration threshold percentage
- three median absolute deviations from historical samples
- a 500 ms absolute floor

The default minimum history is three historical samples per case phase. Override
it with `--min-history-samples` or `KONGCTL_BENCHMARK_MIN_HISTORY_SAMPLES`.

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

The `Declarative Benchmark` workflow can be run manually and also runs on a
schedule:

- Sunday through Friday at 06:00 UTC: `small,medium`
- Saturday at 06:00 UTC: `large,xl`

Size selectors include both layouts, so `small,medium` runs single-file and
multi-file cases for each size.

Manual runs accept:

- `case`: case selector passed to the local runner
- `command_timeout`: timeout for each measured `kongctl` command
- `benchmark_flags`: extra runner flags
- `repeat`: number of times to execute each selected case

Configure these in the `benchmark` environment:

- `KONGCTL_BENCHMARK_KONNECT_PAT`: PAT for the dedicated benchmark org
- optional `KONGCTL_BENCHMARK_KONNECT_BASE_URL`
- optional `KONGCTL_BENCHMARK_DISCUSSION_NUMBER`: discussion number to update
  with the generated dashboard

Scheduled runs use `--repeat 3` by default. Manual runs default to one
repetition unless the `repeat` input or `benchmark_flags` overrides it.

The workflow uploads benchmark artifacts and writes `summary.md` to the GitHub
Actions job summary. It also stores generated summaries on the
`benchmark-results` branch:

- `runs/YYYY/MM/DD/<run-id>-<case>/`
- `latest/`

Raw command output, logs, and generated fixtures remain workflow artifacts.
The branch intentionally stores summaries and structured result JSON only.

## Dashboard and Alerts

The workflow updates a GitHub Discussion when
`KONGCTL_BENCHMARK_DISCUSSION_NUMBER` is configured. The discussion body is
replaced with `dashboard.md` from the latest run, which includes current
medians, recent-history medians, and regression status.

When `regressions.json` reports `has_regressions: true`, the workflow opens or
updates one rolling issue titled:

```text
[benchmark-regression] Declarative benchmark regressions
```

The issue body is replaced with `regressions.md`, and repeated regressions add a
comment that links to the latest workflow run. Passing runs do not automatically
close the issue; that remains a human decision while the benchmark signal is
being tuned.
