# Konnect `.com` E2E baseline

Repository: `kong/kongctl`
Cohort: `cache-enabled`

Full successful runs: 20 of 20
Status: **complete**

The report scans successful `e2e.yaml` runs newest-first and retains only
runs with a complete latest-attempt metrics manifest for every `.com` shard.
Build, harness, scenario, coverage-verification, and required-status jobs
must succeed. Short gate-only runs are excluded.
Percentiles use the nearest-rank method.
Latency starts at the selected attempt's creation time. Jobs reused from an
earlier attempt are excluded. Cache-enabled identifies the cache-reporting
step introduced by #2069 and includes both hits and misses. Keep that step
when changing the cache policy. Each run ID contributes one saved successful
attempt; reruns are not independent samples.

## Latency

| Metric | p50 | p75 | p90 |
| --- | ---: | ---: | ---: |
| workflow_admission_delay_seconds | 8.0s | 546.0s | 1292.0s |
| queue_to_required_status_seconds | 701.0s | 1102.0s | 2031.0s |
| build_job_seconds | 51.0s | 60.0s | 63.0s |
| build_kongctl_seconds | 5.0s | 10.0s | 12.0s |
| build_scenario_binary_seconds | 1.0s | 6.0s | 7.0s |
| build_setup_seconds | 22.0s | 25.0s | 26.0s |
| harness_job_seconds | 72.0s | 75.0s | 76.0s |
| harness_setup_seconds | 10.0s | 10.0s | 11.0s |
| harness_test_seconds | 51.0s | 53.0s | 53.0s |
| longest_shard_seconds | 480.0s | 496.0s | 513.0s |
| shard_spread_seconds | 256.0s | 331.0s | 346.0s |

## Reset cost per workflow run

| Metric | p50 | p75 | p90 |
| --- | ---: | ---: | ---: |
| count | 164.0 | 164.0 | 164.0 |
| duration_ms | 418556.0ms | 450436.0ms | 475189.0ms |
| list_calls | 2459.0 | 2459.0 | 2459.0 |
| list_duration_ms | 407132.0ms | 439797.0ms | 463423.0ms |
| resources_found | 1614.0 | 1614.0 | 1614.0 |
| delete_calls | 103.0 | 103.0 | 103.0 |
| delete_duration_ms | 9186.0ms | 9552.0ms | 10082.0ms |
| resources_deleted | 84.0 | 84.0 | 84.0 |

## Included runs

| Run / Attempt | Attempt created | Queue-to-status | Build | Longest shard | Spread | Resets |
| --- | --- | ---: | ---: | ---: | ---: | ---: |
| [34047830584 / 1](https://github.com/Kong/kongctl/actions/runs/34047830584/attempts/1) | 2026-09-06T17:11:44Z | 2102s | 53s | 492s | 256s | 164 |
| [34047278188 / 1](https://github.com/Kong/kongctl/actions/runs/34047278188/attempts/1) | 2026-09-06T17:01:21Z | 2058s | 52s | 414s | 254s | 164 |
| [34046810193 / 1](https://github.com/Kong/kongctl/actions/runs/34046810193/attempts/1) | 2026-09-06T16:52:03Z | 2031s | 62s | 578s | 374s | 165 |
| [34046799058 / 1](https://github.com/Kong/kongctl/actions/runs/34046799058/attempts/1) | 2026-09-06T16:51:52Z | 1300s | 62s | 493s | 331s | 164 |
| [34046778067 / 1](https://github.com/Kong/kongctl/actions/runs/34046778067/attempts/1) | 2026-09-06T16:51:26Z | 663s | 63s | 414s | 217s | 164 |
| [34046202376 / 1](https://github.com/Kong/kongctl/actions/runs/34046202376/attempts/1) | 2026-09-06T16:40:27Z | 701s | 58s | 343s | 173s | 164 |
| [34026314755 / 1](https://github.com/Kong/kongctl/actions/runs/34026314755/attempts/1) | 2026-09-06T10:02:36Z | 535s | 55s | 359s | 180s | 164 |
| [34007840146 / 1](https://github.com/Kong/kongctl/actions/runs/34007840146/attempts/1) | 2026-09-06T02:59:07Z | 539s | 64s | 357s | 65s | 164 |
| [34006266279 / 1](https://github.com/Kong/kongctl/actions/runs/34006266279/attempts/1) | 2026-09-06T02:21:19Z | 1139s | 60s | 396s | 236s | 164 |
| [34006212666 / 1](https://github.com/Kong/kongctl/actions/runs/34006212666/attempts/1) | 2026-09-06T02:20:06Z | 641s | 42s | 480s | 286s | 164 |
| [34004626083 / 1](https://github.com/Kong/kongctl/actions/runs/34004626083/attempts/1) | 2026-09-06T01:43:24Z | 1102s | 44s | 392s | 205s | 164 |
| [34004275715 / 1](https://github.com/Kong/kongctl/actions/runs/34004275715/attempts/1) | 2026-09-06T01:35:05Z | 1009s | 46s | 583s | 392s | 164 |
| [34003980091 / 1](https://github.com/Kong/kongctl/actions/runs/34003980091/attempts/1) | 2026-09-06T01:28:27Z | 532s | 50s | 344s | 147s | 164 |
| [34003060095 / 1](https://github.com/Kong/kongctl/actions/runs/34003060095/attempts/1) | 2026-09-06T01:07:11Z | 707s | 50s | 508s | 319s | 164 |
| [34002577998 / 1](https://github.com/Kong/kongctl/actions/runs/34002577998/attempts/1) | 2026-09-06T00:56:12Z | 657s | 43s | 489s | 329s | 164 |
| [34000248405 / 1](https://github.com/Kong/kongctl/actions/runs/34000248405/attempts/1) | 2026-09-06T00:02:43Z | 709s | 43s | 505s | 346s | 164 |
| [33995527990 / 1](https://github.com/Kong/kongctl/actions/runs/33995527990/attempts/1) | 2026-09-05T22:18:22Z | 686s | 51s | 513s | 337s | 164 |
| [33975197426 / 1](https://github.com/Kong/kongctl/actions/runs/33975197426/attempts/1) | 2026-09-05T15:34:06Z | 571s | 42s | 405s | 245s | 164 |
| [33972692322 / 1](https://github.com/Kong/kongctl/actions/runs/33972692322/attempts/1) | 2026-09-05T14:44:15Z | 912s | 322s | 496s | 332s | 164 |
| [33906754133 / 3](https://github.com/Kong/kongctl/actions/runs/33906754133/attempts/3) | 2026-09-04T19:07:59Z | 651s | 49s | 485s | 267s | 164 |

## Organization shards

| Run | Organization | Admission delay | Selected | Execution |
| --- | --- | ---: | ---: | ---: |
| 34047830584 | `kongctl-acceptance` | 3s | 43 | 492s |
| 34047830584 | `kongctl-acceptance-2` | 3s | 31 | 274s |
| 34047830584 | `kongctl-acceptance-3` | 4s | 31 | 291s |
| 34047830584 | `kongctl-acceptance-4` | 3s | 30 | 278s |
| 34047830584 | `kongctl-acceptance-5` | 3s | 30 | 236s |
| 34047278188 | `kongctl-acceptance` | 3s | 43 | 272s |
| 34047278188 | `kongctl-acceptance-2` | 3s | 31 | 395s |
| 34047278188 | `kongctl-acceptance-3` | 3s | 31 | 160s |
| 34047278188 | `kongctl-acceptance-4` | 4s | 30 | 414s |
| 34047278188 | `kongctl-acceptance-5` | 4s | 30 | 344s |
| 34046810193 | `kongctl-acceptance` | 4s | 43 | 578s |
| 34046810193 | `kongctl-acceptance-2` | 4s | 31 | 253s |
| 34046810193 | `kongctl-acceptance-3` | 3s | 31 | 204s |
| 34046810193 | `kongctl-acceptance-4` | 3s | 31 | 271s |
| 34046810193 | `kongctl-acceptance-5` | 3s | 30 | 206s |
| 34046799058 | `kongctl-acceptance` | 3s | 43 | 493s |
| 34046799058 | `kongctl-acceptance-2` | 4s | 31 | 397s |
| 34046799058 | `kongctl-acceptance-3` | 3s | 31 | 162s |
| 34046799058 | `kongctl-acceptance-4` | 4s | 30 | 412s |
| 34046799058 | `kongctl-acceptance-5` | 3s | 30 | 178s |
| 34046778067 | `kongctl-acceptance` | 2s | 43 | 288s |
| 34046778067 | `kongctl-acceptance-2` | 2s | 31 | 197s |
| 34046778067 | `kongctl-acceptance-3` | 4s | 31 | 346s |
| 34046778067 | `kongctl-acceptance-4` | 3s | 30 | 414s |
| 34046778067 | `kongctl-acceptance-5` | 3s | 30 | 343s |
| 34046202376 | `kongctl-acceptance` | 3s | 43 | 291s |
| 34046202376 | `kongctl-acceptance-2` | 2s | 31 | 208s |
| 34046202376 | `kongctl-acceptance-3` | 3s | 31 | 343s |
| 34046202376 | `kongctl-acceptance-4` | 4s | 30 | 220s |
| 34046202376 | `kongctl-acceptance-5` | 2s | 30 | 170s |
| 34026314755 | `kongctl-acceptance` | 3s | 43 | 320s |
| 34026314755 | `kongctl-acceptance-2` | 4s | 31 | 359s |
| 34026314755 | `kongctl-acceptance-3` | 3s | 31 | 179s |
| 34026314755 | `kongctl-acceptance-4` | 4s | 30 | 343s |
| 34026314755 | `kongctl-acceptance-5` | 3s | 30 | 284s |
| 34007840146 | `kongctl-acceptance` | 4s | 43 | 299s |
| 34007840146 | `kongctl-acceptance-2` | 4s | 31 | 350s |
| 34007840146 | `kongctl-acceptance-3` | 4s | 31 | 344s |
| 34007840146 | `kongctl-acceptance-4` | 4s | 30 | 357s |
| 34007840146 | `kongctl-acceptance-5` | 3s | 30 | 292s |
| 34006266279 | `kongctl-acceptance` | 2s | 43 | 306s |
| 34006266279 | `kongctl-acceptance-2` | 3s | 31 | 396s |
| 34006266279 | `kongctl-acceptance-3` | 2s | 31 | 160s |
| 34006266279 | `kongctl-acceptance-4` | 2s | 30 | 200s |
| 34006266279 | `kongctl-acceptance-5` | 3s | 30 | 344s |
| 34006212666 | `kongctl-acceptance` | 3s | 43 | 480s |
| 34006212666 | `kongctl-acceptance-2` | 39s | 31 | 194s |
| 34006212666 | `kongctl-acceptance-3` | 3s | 31 | 299s |
| 34006212666 | `kongctl-acceptance-4` | 3s | 30 | 194s |
| 34006212666 | `kongctl-acceptance-5` | 3s | 30 | 346s |
| 34004626083 | `kongctl-acceptance` | 3s | 43 | 392s |
| 34004626083 | `kongctl-acceptance-2` | 3s | 31 | 326s |
| 34004626083 | `kongctl-acceptance-3` | 3s | 31 | 226s |
| 34004626083 | `kongctl-acceptance-4` | 3s | 30 | 187s |
| 34004626083 | `kongctl-acceptance-5` | 4s | 30 | 341s |
| 34004275715 | `kongctl-acceptance` | 4s | 43 | 583s |
| 34004275715 | `kongctl-acceptance-2` | 2s | 31 | 213s |
| 34004275715 | `kongctl-acceptance-3` | 4s | 31 | 344s |
| 34004275715 | `kongctl-acceptance-4` | 2s | 30 | 191s |
| 34004275715 | `kongctl-acceptance-5` | 3s | 30 | 294s |
| 34003980091 | `kongctl-acceptance` | 3s | 43 | 295s |
| 34003980091 | `kongctl-acceptance-2` | 3s | 31 | 219s |
| 34003980091 | `kongctl-acceptance-3` | 5s | 31 | 344s |
| 34003980091 | `kongctl-acceptance-4` | 3s | 30 | 197s |
| 34003980091 | `kongctl-acceptance-5` | 3s | 30 | 344s |
| 34003060095 | `kongctl-acceptance` | 4s | 43 | 508s |
| 34003060095 | `kongctl-acceptance-2` | 3s | 31 | 325s |
| 34003060095 | `kongctl-acceptance-3` | 3s | 31 | 348s |
| 34003060095 | `kongctl-acceptance-4` | 3s | 30 | 359s |
| 34003060095 | `kongctl-acceptance-5` | 3s | 30 | 189s |
| 34002577998 | `kongctl-acceptance` | 3s | 43 | 489s |
| 34002577998 | `kongctl-acceptance-2` | 3s | 31 | 290s |
| 34002577998 | `kongctl-acceptance-3` | 3s | 31 | 160s |
| 34002577998 | `kongctl-acceptance-4` | 2s | 30 | 197s |
| 34002577998 | `kongctl-acceptance-5` | 3s | 30 | 300s |
| 34000248405 | `kongctl-acceptance` | 3s | 43 | 505s |
| 34000248405 | `kongctl-acceptance-2` | 4s | 31 | 405s |
| 34000248405 | `kongctl-acceptance-3` | 3s | 31 | 159s |
| 34000248405 | `kongctl-acceptance-4` | 39s | 30 | 277s |
| 34000248405 | `kongctl-acceptance-5` | 3s | 30 | 164s |
| 33995527990 | `kongctl-acceptance` | 4s | 43 | 513s |
| 33995527990 | `kongctl-acceptance-2` | 3s | 31 | 261s |
| 33995527990 | `kongctl-acceptance-3` | 3s | 31 | 176s |
| 33995527990 | `kongctl-acceptance-4` | 3s | 30 | 267s |
| 33995527990 | `kongctl-acceptance-5` | 3s | 30 | 342s |
| 33975197426 | `kongctl-acceptance` | 3s | 43 | 405s |
| 33975197426 | `kongctl-acceptance-2` | 2s | 31 | 191s |
| 33975197426 | `kongctl-acceptance-3` | 2s | 31 | 160s |
| 33975197426 | `kongctl-acceptance-4` | 2s | 30 | 194s |
| 33975197426 | `kongctl-acceptance-5` | 3s | 30 | 346s |
| 33972692322 | `kongctl-acceptance` | 4s | 43 | 496s |
| 33972692322 | `kongctl-acceptance-2` | 4s | 31 | 402s |
| 33972692322 | `kongctl-acceptance-3` | 3s | 31 | 287s |
| 33972692322 | `kongctl-acceptance-4` | 4s | 30 | 413s |
| 33972692322 | `kongctl-acceptance-5` | 3s | 30 | 164s |
| 33906754133 | `kongctl-acceptance` | 4s | 43 | 485s |
| 33906754133 | `kongctl-acceptance-2` | 4s | 31 | 336s |
| 33906754133 | `kongctl-acceptance-3` | 4s | 31 | 349s |
| 33906754133 | `kongctl-acceptance-4` | 4s | 30 | 218s |
| 33906754133 | `kongctl-acceptance-5` | 4s | 30 | 285s |

## Individual scenario durations

| Scenario | Samples | Median | p90 |
| --- | ---: | ---: | ---: |
| `dump/portal-owned/scenario.yaml` | 20 | 38.19s | 49.91s |
| `portal/visibility/scenario.yaml` | 20 | 36.85s | 51.68s |
| `portal/api_docs_with_children/scenario.yaml` | 20 | 30.21s | 35.43s |
| `all/scenario.yaml` | 20 | 25.56s | 33.81s |
| `event-gateway/plan/apply-workflow/scenario.yaml` | 20 | 24.17s | 32.47s |
| `event-gateway/plan/sync-workflow/scenario.yaml` | 20 | 22.76s | 43.05s |
| `event-gateway/produce-policy/scenario.yaml` | 20 | 22.56s | 28.05s |
| `portal/identity_providers/scenario.yaml` | 20 | 22.29s | 29.18s |
| `portal/teams/scenario.yaml` | 20 | 21.51s | 26.70s |
| `ai-gateway/consumer-pagination/scenario.yaml` | 1 | 20.62s | 20.62s |
| `event-gateway/external-sync/scenario.yaml` | 20 | 20.01s | 24.07s |
| `portal/sync/scenario.yaml` | 20 | 19.67s | 36.91s |
| `portal/audit-log-webhook/scenario.yaml` | 20 | 18.61s | 22.00s |
| `ai-gateway/mcp-server/scenario.yaml` | 20 | 17.77s | 23.64s |
| `portal/auth-strategy-link/scenario.yaml` | 20 | 17.09s | 21.19s |
| `ai-gateway/consumer/scenario.yaml` | 20 | 16.84s | 20.37s |
| `dump/organization-teams/scenario.yaml` | 20 | 16.18s | 17.34s |
| `portal/custom-domain/scenario.yaml` | 20 | 16.18s | 26.61s |
| `org/users/assignments/scenario.yaml` | 20 | 15.73s | 17.12s |
| `ai-gateway/model-provider/scenario.yaml` | 20 | 15.38s | 18.64s |
| `apis/nested-child-lifecycle/scenario.yaml` | 20 | 15.34s | 29.60s |
| `deck/basic/scenario.yaml` | 20 | 15.34s | 17.83s |
| `event-gateway/consume-policy/scenario.yaml` | 20 | 15.23s | 27.67s |
| `apis/control-plane-implementation/scenario.yaml` | 20 | 14.78s | 18.38s |
| `apis/root-level-publication-visibility/scenario.yaml` | 20 | 14.38s | 17.63s |
| `ai-gateway/agent/scenario.yaml` | 20 | 14.19s | 15.50s |
| `portal/email-templates/scenario.yaml` | 20 | 14.10s | 16.65s |
| `portal/ip-allow-list/scenario.yaml` | 20 | 14.08s | 20.50s |
| `org/system-accounts/assignments/scenario.yaml` | 20 | 13.93s | 14.96s |
| `dcr-providers/workflow/scenario.yaml` | 20 | 12.80s | 21.12s |
| `event-gateway/virtual-cluster/scenario.yaml` | 20 | 12.65s | 16.09s |
| `portal/pages/scenario.yaml` | 20 | 12.62s | 14.97s |
| `portal/customization/scenario.yaml` | 20 | 12.36s | 14.86s |
| `diff/command-coverage/scenario.yaml` | 20 | 12.27s | 17.88s |
| `org/teams/roles/scenario.yaml` | 20 | 11.97s | 15.63s |
| `ai-gateway/consumer-group/scenario.yaml` | 20 | 11.95s | 18.65s |
| `external/api-parent/scenario.yaml` | 20 | 11.94s | 14.37s |
| `portal/external-sync/scenario.yaml` | 20 | 11.92s | 17.13s |
| `event-gateway/backend-cluster/scenario.yaml` | 20 | 11.59s | 17.70s |
| `portal/email/scenario.yaml` | 20 | 11.57s | 14.25s |
| `apis/region/scenario.yaml` | 20 | 11.50s | 12.67s |
| `org/users/sync/scenario.yaml` | 20 | 11.32s | 11.94s |
| `dump/filtered/scenario.yaml` | 20 | 11.26s | 16.29s |
| `event-gateway/diff/scenario.yaml` | 20 | 11.26s | 14.67s |
| `org/users/plan/sync-workflow/scenario.yaml` | 20 | 11.22s | 12.05s |
| `portal/default_application_auth_strategy/scenario.yaml` | 20 | 11.21s | 13.73s |
| `portal/idp_team_group_mappings_readback/scenario.yaml` | 20 | 11.10s | 13.05s |
| `plan/apply-workflow/scenario.yaml` | 20 | 11.02s | 13.86s |
| `event-gateway/static-key/scenario.yaml` | 20 | 10.94s | 14.55s |
| `control-plane/data-plane-certificate/scenario.yaml` | 20 | 10.80s | 13.52s |
| `ai-gateway/vault/scenario.yaml` | 20 | 10.77s | 13.14s |
| `yaml-tags/env/scenario.yaml` | 20 | 10.75s | 15.69s |
| `protected-resources/apis/scenario.yaml` | 20 | 10.72s | 14.24s |
| `ai-gateway/config-store/scenario.yaml` | 20 | 10.71s | 15.86s |
| `control-plane/serverless/scenario.yaml` | 20 | 10.67s | 13.31s |
| `event-gateway/listener-policy/scenario.yaml` | 20 | 10.64s | 12.88s |
| `declarative/rename-sync-delete/scenario.yaml` | 20 | 10.55s | 13.56s |
| `ai-gateway/data-plane-certificate/scenario.yaml` | 20 | 10.54s | 13.57s |
| `org/system-accounts/plan/sync-workflow/scenario.yaml` | 20 | 10.50s | 11.67s |
| `ai-gateway/runtime-tls/scenario.yaml` | 20 | 10.49s | 12.32s |
| `ai-gateway/policy-matrix/scenario.yaml` | 20 | 10.34s | 13.09s |
| `ai-gateway/auth-strategy/scenario.yaml` | 20 | 10.27s | 14.96s |
| `adopt/full/scenario.yaml` | 20 | 10.15s | 13.85s |
| `event-gateway/cluster-policy/scenario.yaml` | 20 | 9.89s | 13.26s |
| `org/system-accounts/sync/scenario.yaml` | 20 | 9.82s | 10.55s |
| `apis/child-delete-namespace/scenario.yaml` | 20 | 9.78s | 11.48s |
| `protected-resources/event-gateways/scenario.yaml` | 20 | 9.78s | 11.52s |
| `portal/auth_settings/scenario.yaml` | 20 | 9.74s | 13.64s |
| `dump/control-planes/scenario.yaml` | 20 | 9.68s | 11.40s |
| `apis/comprehensive-fields/scenario.yaml` | 20 | 9.18s | 11.41s |
| `delete/declarative/scenario.yaml` | 20 | 9.09s | 12.61s |
| `event-gateway/schema-registry/scenario.yaml` | 20 | 9.09s | 13.96s |
| `event-gateway/topic-aliases/scenario.yaml` | 20 | 9.09s | 10.84s |
| `portal/publication-auth-omitted-noop/scenario.yaml` | 20 | 9.01s | 11.19s |
| `ai-gateway/root/scenario.yaml` | 20 | 8.97s | 15.38s |
| `ai-gateway/model/scenario.yaml` | 20 | 8.74s | 11.14s |
| `delete/partial-delete/scenario.yaml` | 20 | 8.68s | 10.26s |
| `event-gateway/dataplane-certificate/scenario.yaml` | 20 | 8.63s | 10.28s |
| `portal/integrations/scenario.yaml` | 20 | 8.56s | 10.65s |
| `event-gateway/control-planes/scenario.yaml` | 20 | 8.20s | 8.52s |
| `external/ai-gateway-parent/scenario.yaml` | 20 | 8.18s | 11.06s |
| `org/token/scenario.yaml` | 20 | 8.10s | 8.68s |
| `org/users/plan/apply-workflow/scenario.yaml` | 20 | 7.97s | 9.08s |
| `org/system-accounts/plan/apply-workflow/scenario.yaml` | 20 | 7.92s | 8.54s |
| `portal/api_with_attributes/scenario.yaml` | 20 | 7.92s | 9.99s |
| `protected-resources/portals/scenario.yaml` | 20 | 7.88s | 11.68s |
| `external/portal-publication/scenario.yaml` | 20 | 7.67s | 9.46s |
| `event-gateway/listener/scenario.yaml` | 20 | 7.58s | 11.82s |
| `adopt/auth-strategy-adopt/scenario.yaml` | 20 | 7.55s | 8.71s |
| `control-plane/sync-groups/scenario.yaml` | 20 | 7.52s | 9.15s |
| `portal/app-auth-strategy/scenario.yaml` | 20 | 7.28s | 10.71s |
| `portal/edit/scenario.yaml` | 20 | 7.17s | 9.36s |
| `portal/snippets/scenario.yaml` | 20 | 7.15s | 9.57s |
| `portal/team_group_mappings_no_idp/scenario.yaml` | 20 | 7.02s | 8.30s |
| `portal/page-frontmatter-conflict/scenario.yaml` | 20 | 6.82s | 12.69s |
| `deck/env-vars/scenario.yaml` | 20 | 6.74s | 8.85s |
| `catalog/service/scenario.yaml` | 20 | 6.63s | 11.74s |
| `dump/ai-gateways/scenario.yaml` | 20 | 6.62s | 8.85s |
| `deck/multi-file/scenario.yaml` | 20 | 6.54s | 8.30s |
| `portal/assets/scenario.yaml` | 20 | 6.49s | 12.40s |
| `plan/sync-partial-scope/scenario.yaml` | 20 | 6.46s | 8.57s |
| `apis/eventual-consistency-polp/scenario.yaml` | 20 | 6.38s | 6.84s |
| `control-plane/plan/apply-workflow/scenario.yaml` | 20 | 6.25s | 7.08s |
| `adopt/create-portal-adopt-dump-plan/scenario.yaml` | 20 | 6.07s | 8.35s |
| `portal/oidc-auth-strategy/scenario.yaml` | 20 | 6.07s | 7.89s |
| `deck/sync/scenario.yaml` | 20 | 5.92s | 11.76s |
| `control-plane/apply/scenario.yaml` | 20 | 5.88s | 6.82s |
| `require-namespace/portal/scenario.yaml` | 20 | 5.86s | 9.65s |
| `adopt/event-gateway-adopt/scenario.yaml` | 20 | 5.82s | 8.20s |
| `org/teams/get/scenario.yaml` | 20 | 5.61s | 6.85s |
| `adopt/team-create-adopt-dump/scenario.yaml` | 20 | 5.57s | 6.94s |
| `event-gateway/adopt/scenario.yaml` | 20 | 5.57s | 6.60s |
| `protected-resources/org/teams/scenario.yaml` | 20 | 5.44s | 7.31s |
| `org/teams/external-role/scenario.yaml` | 20 | 5.42s | 11.09s |
| `analytics/dashboard/scenario.yaml` | 20 | 5.37s | 9.72s |
| `event-gateway/tls-trust-bundle/scenario.yaml` | 20 | 5.37s | 10.43s |
| `event-gateway/dependency-order/scenario.yaml` | 20 | 5.36s | 7.98s |
| `delete/plan-based/scenario.yaml` | 20 | 5.34s | 6.85s |
| `plan/remote-url-workflow/scenario.yaml` | 20 | 5.33s | 7.73s |
| `ai-gateway/model-matrix/scenario.yaml` | 20 | 5.27s | 10.45s |
| `org/users/get/scenario.yaml` | 20 | 5.27s | 6.04s |
| `external/api-impl/scenario.yaml` | 20 | 5.14s | 9.83s |
| `deck/idempotent/scenario.yaml` | 20 | 5.09s | 7.82s |
| `control-plane/get/scenario.yaml` | 20 | 5.08s | 6.86s |
| `dump/dcr-provider/scenario.yaml` | 20 | 4.96s | 6.26s |
| `org/teams/plan/apply-workflow/scenario.yaml` | 20 | 4.94s | 6.42s |
| `portal/default_application_auth_strategy_ref_selection/scenario.yaml` | 20 | 4.93s | 6.41s |
| `yaml-tags/file/scenario.yaml` | 20 | 4.77s | 6.41s |
| `org/teams/apply/scenario.yaml` | 20 | 4.75s | 6.18s |
| `org/teams/plan/sync-workflow/scenario.yaml` | 20 | 4.75s | 7.32s |
| `dump/analytics-dashboards/scenario.yaml` | 20 | 4.71s | 9.93s |
| `delete/dry-run/scenario.yaml` | 20 | 4.67s | 6.15s |
| `apis/get-api-by-name-and-id/scenario.yaml` | 20 | 4.64s | 6.09s |
| `plan/sync-workflow/scenario.yaml` | 20 | 4.60s | 8.63s |
| `ai-gateway/policy/scenario.yaml` | 20 | 4.46s | 5.72s |
| `control-plane/delete-groups/scenario.yaml` | 20 | 4.33s | 6.46s |
| `require-namespace/external/scenario.yaml` | 20 | 4.33s | 5.91s |
| `control-plane/sync/scenario.yaml` | 20 | 4.32s | 5.80s |
| `event-gateway/backend-cluster-pagination/scenario.yaml` | 20 | 4.32s | 5.63s |
| `control-plane/groups/scenario.yaml` | 20 | 4.17s | 7.96s |
| `org/get-org/scenario.yaml` | 20 | 4.10s | 4.59s |
| `event-gateway/dump/scenario.yaml` | 20 | 4.05s | 8.39s |
| `org/system-accounts/scenario.yaml` | 20 | 3.99s | 6.42s |
| `apis/versions-pagination/scenario.yaml` | 20 | 3.95s | 5.91s |
| `external/portal-sync/scenario.yaml` | 20 | 3.86s | 5.95s |
| `protected-resources/control-planes/scenario.yaml` | 20 | 3.85s | 7.17s |
| `portal/snippets-pagination/scenario.yaml` | 20 | 3.69s | 5.24s |
| `errors/declarative/scenario.yaml` | 20 | 3.40s | 5.33s |
| `namespace/defaults-sync/scenario.yaml` | 20 | 3.38s | 6.41s |
| `org/teams/sync/scenario.yaml` | 20 | 3.18s | 6.12s |
| `delete/idempotent/scenario.yaml` | 20 | 2.03s | 4.53s |
| `portal/email-domains/scenario.yaml` | 20 | 1.59s | 3.85s |
| `explain/command-coverage/scenario.yaml` | 20 | 1.37s | 1.43s |
| `analytics/dashboard-repro/scenario.yaml` | 20 | 1.03s | 1.32s |
| `scaffold/command-coverage/scenario.yaml` | 20 | 0.98s | 1.00s |
| `namespace/validation/scenario.yaml` | 20 | 0.91s | 1.04s |
| `declarative/plan-mode-validation/scenario.yaml` | 20 | 0.57s | 0.58s |
| `patch/command-coverage/scenario.yaml` | 20 | 0.45s | 0.47s |
| `ai-gateway/maturity/scenario.yaml` | 20 | 0.43s | 0.44s |
| `lint/command-coverage/scenario.yaml` | 20 | 0.34s | 0.35s |
| `portal/auth_settings_deprecated_fields/scenario.yaml` | 20 | 0.19s | 0.19s |
| `portal/identity_providers_duplicate_type/scenario.yaml` | 20 | 0.19s | 0.20s |
| `smoke/version/scenario.yaml` | 20 | 0.06s | 0.07s |
| `auth/get-me/scenario.yaml` | 20 | 0.00s | 0.00s |
| `event-gateway/principal-metadata/scenario.yaml` | 20 | 0.00s | 0.00s |
| `portal/applications/scenario.yaml` | 20 | 0.00s | 0.00s |
