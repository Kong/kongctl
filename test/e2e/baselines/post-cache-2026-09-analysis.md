# E2E cache and optimization assessment — September 6, 2026

Build caching is delivering the expected benefit on normal PRs and main runs.
The next largest measured opportunity is pin-aware weighted sharding.

## Evidence and collection

The cumulative post-cache observation file now contains 14 successful full
`.com` runs, including 11 observations added to the three already present in
the local worktree. The frozen uncached baseline contains seven runs. The
20-run post-cache target remains incomplete. No workflow or scenario behavior
was changed during this assessment.

Collection used `scripts/e2e-baseline.py` with `--count 20 --scan 150`, the
`cache-enabled` cohort, cumulative observations, and `--allow-partial`.
Percentiles below use the collector's nearest-rank method. Latency is measured
from the selected attempt's creation time through required-status completion.
Gate-only runs, incomplete runs, and failed workflows are excluded from the
baseline. One latest/saved attempt is represented per workflow run ID.

An additional build-log audit examined 16 completed workflows with successful
cache-enabled build jobs since the #2069 experiment. It includes the two
failed workflows omitted from the successful-run baseline. Both failures
occurred in scenario execution; their build and harness jobs succeeded.
The audit does not count #2069's earlier attempts again.

## Build-cache value

| Metric | Uncached p50 (7 runs) | Cache-enabled p50 (14 runs) |
| --- | ---: | ---: |
| Build job | 309 s | 49 s |
| Build kongctl | 240 s | 4 s |
| Build scenario binary | 38 s | 1 s |
| Setup Go in build job | 10 s | 22 s |
| Harness job | 75 s | 72 s |
| Queue to required status | 1,001 s | 657 s |
| Longest scenario shard | 551 s | 480 s |
| Shard spread | 345 s | 267 s |

The build-job median improved by 260 seconds, or 84%. Observed overall latency
improved by 344 seconds, or 34%. The latter is not wholly attributable to
caching: scenario and reset times also improved, and queueing varies.

All 15 cache hits in the supplemental audit completed the build job in
42–64 seconds; their median was 49 seconds. The sole miss was the first
available main run after the cache/dependency change, at 322 seconds. Its
cache-save step took 11 seconds. Subsequent main and PR builds restored that
dependency key. The extra setup cost is already included in build-job time.

Cache evidence retained from the `Report Go cache status` build step:

| Workflow run | Outcome | Cache hit | Build job |
| --- | --- | --- | ---: |
| 34026314755 | success | true | 55 s |
| 34009619734 | failure in scenarios | true | 56 s |
| 34007840146 | success | true | 64 s |
| 34006266279 | success | true | 60 s |
| 34006212666 | success | true | 42 s |
| 34004626083 | success | true | 44 s |
| 34004275715 | success | true | 46 s |
| 34003980091 | success | true | 50 s |
| 34003060095 | success | true | 50 s |
| 34002577998 | success | true | 43 s |
| 34002088234 | failure in scenarios | true | 48 s |
| 34000248405 | success | true | 43 s |
| 33995527990 | success | true | 51 s |
| 33975197426 | success | true | 42 s |
| 33972692322 | success | false | 322 s |
| 33906754133 (attempt 3) | success | true | 49 s |

This is broader evidence than identical-commit reruns: the two astra-refactor
runs built kongctl in 8–11 seconds and the declarative-capabilities PR built it
in 10 seconds, with cache hits in all three. Those full workflows passed.

The sample is still clustered: six of the 14 successful observations came
from the Apple notarization PR, and dependencies changed little. Do not treat
the observed hit rate as a long-term guarantee. Dependency and toolchain
changes can still cause misses. There is no present evidence justifying more
complex build cache keys, periodic refresh, or a dedicated warming workflow.

## Next optimization: weighted sharding

The pinned `kongctl-acceptance` shard was slowest in 10 of 14 successful runs.
It receives 43 scenarios; other shards receive 30–31. Median execution was
480 seconds on that shard versus 200 seconds on `kongctl-acceptance-4`.

Across both cohorts, 162 passing scenarios each have 21 duration samples.
The three other scenarios are recorded as skipped, so their times should not
be used as meaningful weights. Caching occurs before scenario execution,
making both cohorts useful for an exploratory duration model. Code changes,
organization differences, and temporal correlation still limit inference.

An offline simulation compared the current modulo assignment, pin-aware
uniform balancing, and pin-aware descending-median-weight balancing:

- Preassign all 12 pins and include their weight in the organization's load.
- Assign remaining scenarios to the lowest predicted load with stable ties.
- Train weights on other runs, excluding the evaluated run's entire PR branch
  among the cache-enabled observations. Retained uncached observations are
  also training data. Missing successful history uses the training median.
- Sum the evaluated run's actual individual scenario durations under each
  proposed assignment. The current modulo sums reproduce actual longest
  shards within approximately two seconds, which checks the reconstruction.

| Assignment | Runs improved | Median estimated saving |
| --- | ---: | ---: |
| Pin-aware uniform weights | 12 / 14 | 91 s |
| Pin-aware historical weights | 13 / 14 | 136 s |

Weighted assignment was approximately 21 seconds slower on the remaining
run. These are simulated savings, not measured CI improvements. The model
assumes a scenario retains its observed duration when moved to another
organization or position. Training can include later runs, so this is a
retrospective branch-held-out comparison, not a prospective forecast.

There is enough history to develop and test the scheduler and weight
generation now. Require completeness, no overlap, deterministic output, pin
preservation, and conservative defaults. Publish predicted shard loads and
validate actual savings in a standalone rollout. Keep scenario coverage and
organization isolation intact. Do not claim this simulation completes the
live validation required by #2053.

## Other opportunities

1. Workflow pipelining still matters for busy periods. Median admission delay
   is only four seconds, but p90 is 546 seconds and the maximum is 571 seconds.
   Removing the full-workflow lock could address that tail. First finish the
   cached baseline and perform the two controlled overlapping runs specified
   in #2053, retaining per-organization serialization. Do not combine this
   rollout with a shard-assignment change.
2. Harness caching is a smaller opportunity. On warm successful runs, the
   harness finished a median 23 seconds after the build. That is the median
   maximum possible saving from accelerating the harness alone with these
   job timings. A restore-only cache experiment could avoid competing saves,
   but restore cost reduces the attainable saving. Prioritize sharding.
3. Reset listing accounts for 97.5% of total measured reset wall time across
   the post-cache runs. Median cumulative reset time is 415 seconds across
   all five shards, including 404 seconds listing and nine seconds deleting.
   These totals are not directly subtractable from workflow wall time. More
   deletion cleanup alone cannot remove the dominant list cost. Reduced
   resets still require a verified cleanup contract and failure-safe policy;
   do not skip resets solely on the strength of these timing numbers.

Collect six more successful full cache-enabled runs for the planned 20-run
baseline. Additional observations should include normal feature changes and
main runs, rather than deliberately repeating the same commit to fill the
target. Continue preserving observations before artifact expiry.

## Sources and retained outputs

- [Cumulative observations](post-cache-2026-09-observations.json)
- [Generated percentile and scenario report](post-cache-2026-09.md)
- [Frozen uncached report](stage0-2026-09.md)
- [Optimization specification](https://github.com/Kong/kongctl/issues/2053)
- [Cache implementation and experiment](https://github.com/Kong/kongctl/pull/2069)
- [Refactoring PR build](https://github.com/Kong/kongctl/actions/runs/34026314755)
- [Main cache miss](https://github.com/Kong/kongctl/actions/runs/33972692322)
- [Subsequent main cache hit](https://github.com/Kong/kongctl/actions/runs/33995527990)
- [setup-go cache restore source](https://github.com/actions/setup-go/blob/v7.0.0/src/cache-restore.ts)
- [GitHub cache scope reference](https://docs.github.com/en/actions/reference/workflows-and-actions/dependency-caching)

Local audit scripts, cache-log excerpts, and per-run simulation results are
also retained under `/home/rspurgeon/.cache/e2e-analysis.dLljuBEN/`.
