# Weighted allocation evaluation — September 9, 2026

Related: #2053, activation #2100, and the separate replay experiment #2058.

The completed weighted observation file preserves twenty full successful
`.com` runs from September 6 at 23:40 UTC through September 9 at 15:10 UTC.
Every shard identifies the original `weighted-v1:a30975...23af` snapshot.
The dataset includes twenty distinct source SHAs, eight main runs and twelve
PR runs. Samples are observational and multiple revisions of a PR are not
independent experiments. Scenario counts range from 165 to 166.

## Observed comparison

Compare against the frozen twenty-run `post-cache-2026-09` modulo baseline:

| Metric | Modulo | Weighted |
| --- | ---: | ---: |
| Longest shard p50 | 480s | 417s |
| Longest shard p90 | 513s | 461s |
| Shard spread p50 | 256s | 210s |
| Shard spread p90 | 346s | 261s |
| Queue-to-required p50 | 701s | 644s |
| Queue-to-required minus initial admission p50 | 634s | 620s |
| Build job p50 | 51s | 71s |
| Harness job p50 | 72s | 74s |

The longest shard improved about 13% at p50 and 10% at p90. The original
acceptance shard was longest in thirteen modulo runs versus six weighted
runs. This supports keeping weighted allocation enabled. It is not evidence
that sharding alone caused every overall latency change: queueing, API
latency, source changes, and scenario changes remain confounders.

The recent cache-log audit found eighteen hits (70s median build) and two
misses (245s and 309s). The historical uncached build median was 309s.
Keep build caching enabled. A cache hit restores an archive, not necessarily
all compilation results required by the current source revision.

## Reruns and next steps

Historically expected Konnect timeouts and workflow-job reruns are not treated
as evidence of a regression introduced by weighted sharding. Full successful
reruns qualify if all jobs and five shard metrics belong to the same attempt.
Partial reruns do not: combining retained and rerun jobs would manufacture a
cross-attempt shard comparison and inconsistent workflow timing. Their
individual scenario timings can support a separately scoped future analysis.

The initial measurement target is complete. Preserve these observations,
keep allocation and cache policy unchanged, and vet the smallest replay slice
in #2058. Replay results must never enter this live performance dataset.
Reducing exposure to historically variable live APIs in selected PR scenarios
does not require eliminating those upstream timeouts first. Main remains live.
