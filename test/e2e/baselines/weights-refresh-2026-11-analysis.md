# Production shard weight refresh — November 2026

Related: #2840, #2053, and #2100.

## Sources and regeneration

Weights use live-shard timings from 25 successful `.com` runs: ten full-live
and fifteen PR-route runs. `scripts/e2e_baseline.py` collected complete
five-shard metrics from matching successful attempts, excluding partial
reruns, failed/cancelled workflows, `.tech`, and gate-only runs. PR-route
includes dispatches with a `:pr-replay:` allocation identity. Replay timings
never enter the weights. Training spans November 10 at 17:24 UTC through
November 18 at 02:07 UTC; all held-out runs occurred later.

The two `weights-refresh-2026-11-training-*.json.gz` files retain generator
inputs: run IDs, attempts, route identity, and scenario results/durations.
They omit unrelated workflow/reset metadata and retain the original full
observation hashes as `original_source_sha256`. Gzip uses a zero timestamp
for deterministic storage. Inspect either file with `gzip -dc <path>`.

```text
Training, full live:
44220905764, 44190735685, 44142303148, 44131899855, 44003770322,
43533006222, 43523624535, 43273843303, 43156484234, 42864695008

Training, PR route:
43998341915, 43984957140, 43982936412, 43529533122, 43513013626,
43510735246, 43498876013, 43494190981, 43488186352, 43434283589,
43347048531, 43252748028, 43168431133, 43168230906, 43164128495
```

Regenerate and print the allocation identity with:

```sh
make refresh-e2e-weights
python3 scripts/e2e_weights.py --print-allocation-id
```

The generator accepts plain or gzip-compressed observation JSON. Its
statistics and sample threshold are unchanged: deduplicate runs using the
latest attempt, use positive finite passing durations for active paths,
require ten samples, and round per-scenario medians to milliseconds. The
fallback is the median of qualified scenario medians. The snapshot records
the compressed inputs' hashes; changing their representation changes the
allocation identity even when the weights themselves stay identical.

Current identity:

```text
weighted-v1:6bbdbceee76361f16d5510c616a212971214d46dfe1a154f380794892ba3d20f
```

## Coverage and held-out results

The 5,510 passing samples qualify 233 of 237 current paths, up from 162.
There are 211 paths with 25 samples, twenty with ten, one with twelve, and
one with twenty-three. The fallback changes from 8,402ms to 21,185ms.
All 75 training skips belong to three paths skipped in all 25 runs:

- `auth/get-me/scenario.yaml`
- `event-gateway/principal-metadata/scenario.yaml`
- `portal/applications/scenario.yaml`

`ai-cost-management/declarative/scenario.yaml` was absent from training.
These four paths and future unknown paths use the fallback and remain
assigned. Skips never count as passing timing samples.

The actual Go allocator was run against both snapshots using each held-out
inventory, recorded organization order, and six manifest pins. The September
snapshot reproduced every
recorded assignment. Both placements assigned every scenario exactly once
and respected all pins. Existing `TestWeightedRepositoryPlan` also verifies
the current 237-scenario corpus; run `^TestWeighted` with `-tags=e2e`.

| Held-out run | Route | Old longest | New longest | Old spread | New spread |
| --- | --- | ---: | ---: | ---: | ---: |
| 44664711279 | PR | 1703.23 | 1386.32 | 649.27 | 71.60 |
| 44636838022 | PR | 1552.08 | 1415.37 | 396.56 | 100.56 |
| 44596625739 | PR | 1594.03 | 1400.86 | 609.80 | 90.17 |
| 44424566576 | PR | 1643.99 | 1372.30 | 606.19 | 99.20 |
| 44227201477 | PR | 1417.79 | 1291.04 | 378.74 | 47.54 |
| 44432249552 | Full | 1854.96 | 1513.17 | 612.63 | 155.98 |
| 44235514070 | Full | 1833.55 | 1717.00 | 420.86 | 188.43 |

Values are seconds of scenario-work sums, holding measured durations fixed
when moving scenarios. They exclude shared overhead and are not measured
post-rollout shard times. Median paired reduction is 193.17s for PRs and
229.17s for full live. Two full-live holdouts cannot establish tail behavior.
Full holdout fixtures and the one-off comparison test are not retained in
this PR; the table records the completed evaluation. September weights remain
available in Git at `c0ea03eb:test/e2e/baselines/scenario-weights.json`.

## Rollout measurement still required

After deployment, collect twenty complete successful runs per route:

```sh
make collect-e2e-progress E2E_PROGRESS_MODE=live E2E_BASELINE_COUNT=20 \
  E2E_BASELINE_SCAN=150
make collect-e2e-progress E2E_PROGRESS_MODE=pr E2E_BASELINE_COUNT=20 \
  E2E_BASELINE_SCAN=150
```

Outputs live under `.e2e-artifacts/progress/` in identity-specific cohorts.
Retain observations before artifact expiry. Compare measured longest-shard
p50/p90 and spread separately for PR and full-live routes, using nearest-rank
percentiles. The old reference measured PR longest p50/p90 of 1553/1704s and
spread of 608/740s across twenty runs; full-live values were 1783/1856s and
532/702s across twelve runs. These references combine training and holdout
runs above; their full workflow observations are not retained in this PR.
Report cohort sizes, inventory changes, overhead, admission delays, and
failed/cancelled workflows alongside successful-run latency. Do not pool
routes, identities, or jobs from different attempts.

Allocator, replay membership, organization pins, and reset policy are
unchanged. Roll back with `KONGCTL_E2E_SHARD_STRATEGY=modulo`; existing tests
verify that rollback works without valid weights. The twenty-run rollout
evaluation remains outstanding under #2840.
