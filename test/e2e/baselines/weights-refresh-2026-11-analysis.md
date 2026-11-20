# Production shard weight refresh — November 2026

Related: #2840, #2053, and #2100.

## Method and provenance

The frozen training and held-out observation files were collected with
`scripts/e2e_baseline.py` from successful `e2e.yaml` runs. Each retained run
has complete five-organization `.com` metrics from one successful attempt,
successful build, harness, verification, and required-status jobs, and an
exact allocation identity. Partial reruns, failed/cancelled runs, `.tech`,
and gate-only runs are excluded by the existing collector. Cache hits and
misses both qualify. The files retain run URLs, attempt numbers, source
SHAs, shard order, and individual scenario results.

Training uses live-shard observations from ten full-live and fifteen
PR-route runs, collected November 20 by scanning the latest 200 successful
runs. The training window is November 10 at 17:24 UTC through November 18 at
02:07 UTC. The next two qualifying full-live and five PR-route runs are held
out entirely, spanning November 18 at 02:59 through November 20 at 03:00 UTC.
Every training observation precedes every held-out run. PR-route means the
allocation's `:pr-replay:` identity, including
dispatches that exercise that route; it does not mean workflow event type.
Only live-shard metrics enter this comparison. Replay durations never enter
weight generation or live load estimates.

The full-live and PR-route training files retain their distinct allocation
identities; only the generator pools their live scenario samples. Runtime
reports and post-rollout comparisons must keep those routes separate.

Source run IDs, newest first within each cohort:

```text
Training, full live:
44220905764, 44190735685, 44142303148, 44131899855, 44003770322,
43533006222, 43523624535, 43273843303, 43156484234, 42864695008

Training, PR route:
43998341915, 43984957140, 43982936412, 43529533122, 43513013626,
43510735246, 43498876013, 43494190981, 43488186352, 43434283589,
43347048531, 43252748028, 43168431133, 43168230906, 43164128495

Held out, full live:
44432249552, 44235514070

Held out, PR route:
44664711279, 44636838022, 44596625739, 44424566576, 44227201477
```

`scripts/e2e_weights.py` is unchanged: normalize paths, deduplicate run IDs
using the latest attempt, retain positive finite passing durations for
currently discovered paths, require at least ten samples, and round each
scenario's median to milliseconds. Failed, skipped, zero-duration, and
retired-path observations do not contribute weights. The global fallback is
the median of qualified scenario medians. Unknown paths and paths below the
sample threshold use that fallback; they remain assigned.

## Refreshed snapshot and fallback coverage

The new allocation identity is:

```text
weighted-v1:8f9b1063ad0cbbf38ab0b5cf741c2e6df7eb52e2395132e77ad9c141c6c2f7d6
```

The 5,510 passing samples qualify 233 of 237 current paths, versus 162 in
September. Of those paths, 211 have 25 samples, twenty have ten samples,
one has twelve, and one has twenty-three. The regenerated fallback is
21,185ms, replacing 8,402ms.

All 75 training skips belong to three paths, each skipped in all 25 runs:

- `auth/get-me/scenario.yaml`
- `event-gateway/principal-metadata/scenario.yaml`
- `portal/applications/scenario.yaml`

`ai-cost-management/declarative/scenario.yaml` was absent from training.
These four paths use the fallback, as will future unknown paths. No skipped
duration is treated as a passing sample, and no scenario is removed.

## Held-out comparison

All seven held-out runs improve in the counterfactual simulation. Median
paired longest-load reduction is 193.17s for PRs (range 126.75–316.91s) and
229.17s for full live (range 116.55–341.79s). Each original placement exactly
reproduces the organizations recorded in that run's metrics, providing a
control for inventory, organization order, and pin interpretation.

The following percentiles use the collector's nearest-rank method. Values
are seconds of summed scenario work, excluding shared overhead:

| Route | Runs | Metric | Old | Refreshed |
| --- | ---: | --- | ---: | ---: |
| PR | 5 | Longest p50 | 1594.03 | 1386.32 |
| PR | 5 | Longest p90 | 1703.23 | 1415.37 |
| PR | 5 | Spread p50 | 606.19 | 90.17 |
| PR | 5 | Spread p90 | 649.27 | 100.56 |
| Full live | 2 | Longest p50 | 1833.55 | 1513.17 |
| Full live | 2 | Longest p90 | 1854.96 | 1717.00 |
| Full live | 2 | Spread p50 | 420.86 | 155.98 |
| Full live | 2 | Spread p90 | 612.63 | 188.43 |

Two full-live holdouts are descriptive only and cannot establish a tail
improvement. Retaining ten full-live training runs lets the twenty paths
routed to replay in PRs still meet the unchanged sample threshold using
their live timings. The larger post-rollout cohorts remain necessary.

## Reproduction

Regenerate the active snapshot from both committed training files:

```sh
make refresh-e2e-weights
python3 scripts/e2e_weights.py --print-allocation-id
```

The source paths and their SHA-256s are embedded in the generated JSON. The old
snapshot is preserved as `scenario-weights-2026-09.json`; the September
observations and analysis remain frozen. `scenarios_weighted.go` still
embeds `baselines/scenario-weights.json` and needs no path change.

Run the actual Go allocator against both snapshots without Konnect access:

```sh
KONGCTL_E2E_BIN="$PWD/kongctl" CGO_ENABLED=0 go test -tags=e2e \
  ./test/e2e -run '^TestWeighted' -count=1 -v
```

`TestWeightedRefreshHoldouts` uses each held-out run's complete discovered
live inventory, the recorded organization order, and manifest pins frozen in
the held-out JSON's `scenario_assignments` field at refresh time. This keeps
the historical comparison reproducible when manifests change later.
It rejects training/holdout overlap, duplicates, missing assignments, and
pin violations for both snapshots, and checks that the old placement matches
the recorded shard inventories. It sums the same recorded scenario
durations under old and new placement, including reported skip durations.
These are counterfactual scenario-work sums, not measured new shard times:
they exclude job setup, shared resets, and scheduling overhead, and assume
durations would remain constant after movement. Current corpus validation
also checks all 237 scenarios and six pins across five organizations. The
allocator, reset policy, replay membership, pins, and concurrency are unchanged.

## Rollout measurement still required

The committed training and holdout files also freeze this measured reference
under the old allocation identity (seconds, nearest-rank percentiles):

| Route | Runs | Longest p50 | Longest p90 | Spread p50 | Spread p90 |
| --- | ---: | ---: | ---: | ---: | ---: |
| PR | 20 | 1553 | 1704 | 608 | 740 |
| Full live | 12 | 1783 | 1856 | 532 | 702 |

These are actual shard execution times, distinct from the held-out scenario
work sums above. The cohorts have different sizes and inventories.

This PR activates the new embedded snapshot when its binaries are built.
Offline estimates do not complete the post-rollout acceptance criterion.
After merge, retain twenty complete successful runs per route under the new
identity, in separate cohorts:

```sh
make collect-e2e-progress E2E_PROGRESS_MODE=live E2E_BASELINE_COUNT=20 \
  E2E_BASELINE_SCAN=150
make collect-e2e-progress E2E_PROGRESS_MODE=pr E2E_BASELINE_COUNT=20 \
  E2E_BASELINE_SCAN=150
```

The collector retains cumulative observations under identity-specific paths
in `.e2e-artifacts/progress/`. Preserve partial collections before artifact
expiry. Commit the observations and a follow-up analysis when each cohort
reaches twenty complete runs. `make collect-e2e-baseline` now defaults to a
separate November full-live observation/report path, so it cannot overwrite
the completed September experiment.

Compare measured longest-shard p50/p90 and spread (max minus min shard
execution time) with the old-identity observations, separately for PR and
full-live routes. Use nearest-rank percentiles as in the existing collector;
show cohort size, source SHA/inventory changes, reset/build/harness cost,
admission delay, and all failed/cancelled runs alongside successful samples.
Do not pool identities, routes, or retained jobs from different attempts.
The committed training plus held-out files provide twenty old-identity PR
runs and twelve full-live runs. Recombine within each route using
`scripts/e2e_baseline.py`'s `observation_document`/`merge_runs`, then use its
`--frozen` report mode. Extend the old full-live reference if additional
qualifying runs become available before artifact expiry. Successful-run
percentiles alone do not establish reliability or causal speedup.

Rollback remains `KONGCTL_E2E_SHARD_STRATEGY=modulo`; the existing offline
rollback tests verify that it uses the original selector without depending
on valid weights. Restore the frozen September snapshot for a weighted-only
rollback. New matrices capture strategy once; in-flight matrices retain
their original identity. Do not weaken assertions or reset safety to retain
a runtime improvement.
