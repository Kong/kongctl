# Weighted sharding activation

Related: #2053 and the report-only foundation in #2094.

## Frozen reference

The completed `post-cache-2026-09-observations.json` contains 20 successful
full `.com` runs from September 4–6, 2026. The companion report is preserved
as collected, without pooling subsequent weighted execution into it.

Measured pre-activation p50 values:

| Metric | Seconds |
| --- | ---: |
| Build job | 51 |
| Longest shard | 480 |
| Shard spread | 256 |
| Queue to required status | 701 |
| Workflow admission delay | 8 |

Workflow admission delay p90 is 1,292 seconds. Queueing remains a substantial
confounder for overall latency; concurrency is unchanged in this experiment.
Successful-run percentiles do not measure failure rate, and multiple revisions
from the same PR are correlated samples.

## Activation snapshot

Weights use only this completed cache-enabled baseline, not the older uncached
cohort. There are 162 qualified scenarios with 20 passing observations each.
The other three scenarios use the 8,402ms fallback weight. All 165 scenarios
remain assigned, and all 12 organization pins remain enforced.

The snapshot SHA-256 is:

```text
a30975c68af38565631cb7379995b28ad40edfeee6a58344ae1dc52aa0c323af
```

The measurement identity is `weighted-v1:` followed by that hash. Regeneration
from the frozen input is deterministic. Do not refresh weights during the
initial 20-run experiment; any changed snapshot requires a separate observation
file and measurement identity.

Offline validation against the repository corpus predicts longest-shard load
of 452.26s under modulo versus 318.21s under weighted allocation. These are
sums of scenario median weights, not measured job durations or promised savings.
They must not be compared directly with the measured 480s baseline median.

## Rollout and evaluation

The activation PR exercises weighted allocation in its own full `.com` CI run;
after merge, normal full `.com` runs use it too. Filtered, unsharded, and `.tech`
runs continue using the original selector. Reset policy and concurrency remain
unchanged. Weighted execution order is deterministic normalized-path order.

After passing CI and human review, collect normal runs with:

```sh
make collect-e2e-baseline E2E_BASELINE_COUNT=20 E2E_BASELINE_SCAN=150
```

This now updates the separate `weighted-v1-2026-09-*` files. The collector
requires the exact algorithm and snapshot identity, rejects mixed allocations,
and refuses to overwrite an observation file from another experiment.
Keep committing partial observations before artifact expiry.

Evaluate the first 20 successful weighted runs against the frozen reference:
longest-shard p50/p90, spread, and overall completion time, with build/harness
cost and admission delays shown separately. Independently inspect all failed
and cancelled workflows and beta failures. Success requires measured runtime
improvement without worse reliability; better predictions alone do not count.

For coverage/pinning defects or reproducible ordering-related failures, set the
repository variable `KONGCTL_E2E_SHARD_STRATEGY` to `modulo`. New target-selection
jobs capture that value once for their entire matrix. In-flight matrices keep
their selected strategy. Rollback does not depend on valid weights. Collect any
rollback runs separately as documented in the E2E README. Never weaken scenario
assertions or reset safety to keep a speed improvement.
