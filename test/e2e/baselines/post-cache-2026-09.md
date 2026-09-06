# Konnect `.com` E2E baseline

Repository: `kong/kongctl`
Cohort: `cache-enabled`

Full successful runs: 14 of 20
Status: **collecting**

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
| workflow_admission_delay_seconds | 4.0s | 30.0s | 546.0s |
| queue_to_required_status_seconds | 657.0s | 912.0s | 1102.0s |
| build_job_seconds | 49.0s | 55.0s | 64.0s |
| build_kongctl_seconds | 4.0s | 8.0s | 11.0s |
| build_scenario_binary_seconds | 1.0s | 5.0s | 6.0s |
| build_setup_seconds | 22.0s | 25.0s | 29.0s |
| harness_job_seconds | 72.0s | 75.0s | 78.0s |
| harness_setup_seconds | 10.0s | 10.0s | 11.0s |
| harness_test_seconds | 51.0s | 53.0s | 53.0s |
| longest_shard_seconds | 480.0s | 505.0s | 513.0s |
| shard_spread_seconds | 267.0s | 332.0s | 346.0s |

## Reset cost per workflow run

| Metric | p50 | p75 | p90 |
| --- | ---: | ---: | ---: |
| count | 164.0 | 164.0 | 164.0 |
| duration_ms | 415210.0ms | 462432.0ms | 486068.0ms |
| list_calls | 2459.0 | 2459.0 | 2459.0 |
| list_duration_ms | 404252.0ms | 450914.0ms | 474589.0ms |
| resources_found | 1614.0 | 1614.0 | 1614.0 |
| delete_calls | 103.0 | 103.0 | 103.0 |
| delete_duration_ms | 8984.0ms | 9985.0ms | 10082.0ms |
| resources_deleted | 84.0 | 84.0 | 84.0 |

## Included runs

| Run / Attempt | Attempt created | Queue-to-status | Build | Longest shard | Spread | Resets |
| --- | --- | ---: | ---: | ---: | ---: | ---: |
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
| `portal/visibility/scenario.yaml` | 14 | 42.11s | 51.68s |
| `dump/portal-owned/scenario.yaml` | 14 | 32.45s | 50.87s |
| `portal/api_docs_with_children/scenario.yaml` | 14 | 30.21s | 35.43s |
| `all/scenario.yaml` | 14 | 28.77s | 33.86s |
| `event-gateway/produce-policy/scenario.yaml` | 14 | 26.12s | 28.33s |
| `portal/teams/scenario.yaml` | 14 | 25.48s | 26.86s |
| `event-gateway/plan/apply-workflow/scenario.yaml` | 14 | 21.15s | 32.47s |
| `event-gateway/plan/sync-workflow/scenario.yaml` | 14 | 20.65s | 37.16s |
| `portal/auth-strategy-link/scenario.yaml` | 14 | 20.07s | 21.23s |
| `event-gateway/external-sync/scenario.yaml` | 14 | 20.01s | 24.07s |
| `portal/identity_providers/scenario.yaml` | 14 | 20.00s | 29.18s |
| `portal/audit-log-webhook/scenario.yaml` | 14 | 18.61s | 21.82s |
| `portal/sync/scenario.yaml` | 14 | 17.82s | 32.14s |
| `ai-gateway/consumer/scenario.yaml` | 14 | 16.84s | 20.31s |
| `apis/root-level-publication-visibility/scenario.yaml` | 14 | 16.61s | 17.63s |
| `dump/organization-teams/scenario.yaml` | 14 | 16.37s | 17.34s |
| `portal/ip-allow-list/scenario.yaml` | 14 | 16.18s | 20.61s |
| `org/users/assignments/scenario.yaml` | 14 | 15.73s | 17.12s |
| `portal/custom-domain/scenario.yaml` | 14 | 15.55s | 23.25s |
| `event-gateway/virtual-cluster/scenario.yaml` | 14 | 15.39s | 16.56s |
| `ai-gateway/model-provider/scenario.yaml` | 14 | 15.38s | 18.46s |
| `ai-gateway/mcp-server/scenario.yaml` | 14 | 15.22s | 23.76s |
| `deck/basic/scenario.yaml` | 14 | 15.08s | 17.80s |
| `apis/control-plane-implementation/scenario.yaml` | 14 | 14.96s | 18.38s |
| `diff/command-coverage/scenario.yaml` | 14 | 14.70s | 17.95s |
| `apis/nested-child-lifecycle/scenario.yaml` | 14 | 14.38s | 25.23s |
| `event-gateway/backend-cluster/scenario.yaml` | 14 | 14.38s | 17.70s |
| `ai-gateway/agent/scenario.yaml` | 14 | 14.19s | 15.50s |
| `portal/email-templates/scenario.yaml` | 14 | 14.10s | 16.65s |
| `org/system-accounts/assignments/scenario.yaml` | 14 | 13.93s | 14.96s |
| `portal/email/scenario.yaml` | 14 | 13.64s | 14.52s |
| `portal/external-sync/scenario.yaml` | 14 | 13.41s | 17.22s |
| `dump/filtered/scenario.yaml` | 14 | 13.30s | 16.29s |
| `portal/default_application_auth_strategy/scenario.yaml` | 14 | 12.97s | 13.79s |
| `plan/apply-workflow/scenario.yaml` | 14 | 12.96s | 13.86s |
| `yaml-tags/env/scenario.yaml` | 14 | 12.84s | 15.82s |
| `control-plane/data-plane-certificate/scenario.yaml` | 14 | 12.74s | 13.83s |
| `ai-gateway/data-plane-certificate/scenario.yaml` | 14 | 12.72s | 13.57s |
| `event-gateway/consume-policy/scenario.yaml` | 14 | 12.72s | 23.82s |
| `portal/pages/scenario.yaml` | 14 | 12.62s | 14.97s |
| `control-plane/serverless/scenario.yaml` | 14 | 12.59s | 13.47s |
| `declarative/rename-sync-delete/scenario.yaml` | 14 | 12.54s | 14.74s |
| `ai-gateway/vault/scenario.yaml` | 14 | 12.49s | 13.35s |
| `portal/customization/scenario.yaml` | 14 | 12.36s | 14.86s |
| `event-gateway/listener-policy/scenario.yaml` | 14 | 12.22s | 13.14s |
| `ai-gateway/auth-strategy/scenario.yaml` | 14 | 11.95s | 15.19s |
| `external/api-parent/scenario.yaml` | 14 | 11.94s | 14.37s |
| `dcr-providers/workflow/scenario.yaml` | 14 | 11.79s | 18.90s |
| `apis/region/scenario.yaml` | 14 | 11.50s | 12.67s |
| `org/users/sync/scenario.yaml` | 14 | 11.32s | 11.94s |
| `org/users/plan/sync-workflow/scenario.yaml` | 14 | 11.22s | 12.05s |
| `portal/idp_team_group_mappings_readback/scenario.yaml` | 14 | 11.10s | 12.85s |
| `event-gateway/schema-registry/scenario.yaml` | 14 | 11.08s | 13.96s |
| `portal/auth_settings/scenario.yaml` | 14 | 10.98s | 13.92s |
| `apis/comprehensive-fields/scenario.yaml` | 14 | 10.75s | 11.62s |
| `ai-gateway/config-store/scenario.yaml` | 14 | 10.71s | 15.92s |
| `org/system-accounts/plan/sync-workflow/scenario.yaml` | 14 | 10.50s | 11.67s |
| `ai-gateway/runtime-tls/scenario.yaml` | 14 | 10.49s | 12.10s |
| `ai-gateway/model/scenario.yaml` | 14 | 10.47s | 11.37s |
| `ai-gateway/policy-matrix/scenario.yaml` | 14 | 10.34s | 13.21s |
| `portal/publication-auth-omitted-noop/scenario.yaml` | 14 | 10.29s | 11.85s |
| `portal/integrations/scenario.yaml` | 14 | 10.17s | 11.01s |
| `delete/declarative/scenario.yaml` | 14 | 10.04s | 12.61s |
| `org/teams/roles/scenario.yaml` | 14 | 9.98s | 15.66s |
| `org/system-accounts/sync/scenario.yaml` | 14 | 9.82s | 10.55s |
| `apis/child-delete-namespace/scenario.yaml` | 14 | 9.78s | 11.43s |
| `protected-resources/event-gateways/scenario.yaml` | 14 | 9.78s | 11.52s |
| `dump/control-planes/scenario.yaml` | 14 | 9.68s | 11.30s |
| `protected-resources/apis/scenario.yaml` | 14 | 9.56s | 14.45s |
| `event-gateway/dataplane-certificate/scenario.yaml` | 14 | 9.51s | 10.67s |
| `event-gateway/static-key/scenario.yaml` | 14 | 9.41s | 14.60s |
| `portal/api_with_attributes/scenario.yaml` | 14 | 9.35s | 10.05s |
| `event-gateway/diff/scenario.yaml` | 14 | 9.33s | 14.69s |
| `event-gateway/listener/scenario.yaml` | 14 | 9.22s | 11.84s |
| `event-gateway/topic-aliases/scenario.yaml` | 14 | 9.09s | 10.84s |
| `protected-resources/portals/scenario.yaml` | 14 | 9.03s | 12.11s |
| `ai-gateway/consumer-group/scenario.yaml` | 14 | 8.82s | 16.29s |
| `external/portal-publication/scenario.yaml` | 14 | 8.73s | 9.73s |
| `delete/partial-delete/scenario.yaml` | 14 | 8.68s | 10.22s |
| `ai-gateway/root/scenario.yaml` | 14 | 8.63s | 13.69s |
| `portal/app-auth-strategy/scenario.yaml` | 14 | 8.60s | 10.73s |
| `event-gateway/cluster-policy/scenario.yaml` | 14 | 8.41s | 13.40s |
| `org/token/scenario.yaml` | 14 | 8.33s | 8.68s |
| `deck/env-vars/scenario.yaml` | 14 | 8.24s | 8.98s |
| `event-gateway/control-planes/scenario.yaml` | 14 | 8.20s | 8.52s |
| `org/users/plan/apply-workflow/scenario.yaml` | 14 | 7.97s | 9.08s |
| `adopt/full/scenario.yaml` | 14 | 7.93s | 12.66s |
| `org/system-accounts/plan/apply-workflow/scenario.yaml` | 14 | 7.92s | 8.54s |
| `adopt/auth-strategy-adopt/scenario.yaml` | 14 | 7.55s | 8.50s |
| `control-plane/sync-groups/scenario.yaml` | 14 | 7.52s | 9.15s |
| `portal/team_group_mappings_no_idp/scenario.yaml` | 14 | 7.02s | 8.30s |
| `external/ai-gateway-parent/scenario.yaml` | 14 | 6.66s | 11.06s |
| `protected-resources/org/teams/scenario.yaml` | 14 | 6.66s | 7.37s |
| `delete/plan-based/scenario.yaml` | 14 | 6.45s | 6.89s |
| `plan/remote-url-workflow/scenario.yaml` | 14 | 6.43s | 7.73s |
| `apis/eventual-consistency-polp/scenario.yaml` | 14 | 6.40s | 6.84s |
| `event-gateway/dependency-order/scenario.yaml` | 14 | 6.35s | 7.98s |
| `deck/idempotent/scenario.yaml` | 14 | 6.30s | 8.42s |
| `portal/page-frontmatter-conflict/scenario.yaml` | 14 | 6.29s | 11.14s |
| `adopt/create-portal-adopt-dump-plan/scenario.yaml` | 14 | 6.28s | 8.51s |
| `portal/snippets/scenario.yaml` | 14 | 6.17s | 9.57s |
| `control-plane/plan/apply-workflow/scenario.yaml` | 14 | 6.15s | 7.08s |
| `catalog/service/scenario.yaml` | 14 | 5.99s | 10.64s |
| `portal/edit/scenario.yaml` | 14 | 5.96s | 9.34s |
| `org/teams/plan/apply-workflow/scenario.yaml` | 14 | 5.95s | 6.65s |
| `org/teams/plan/sync-workflow/scenario.yaml` | 14 | 5.93s | 7.46s |
| `adopt/team-create-adopt-dump/scenario.yaml` | 14 | 5.89s | 7.34s |
| `portal/assets/scenario.yaml` | 14 | 5.89s | 10.74s |
| `control-plane/apply/scenario.yaml` | 14 | 5.88s | 6.82s |
| `adopt/event-gateway-adopt/scenario.yaml` | 14 | 5.82s | 8.24s |
| `dump/dcr-provider/scenario.yaml` | 14 | 5.74s | 6.42s |
| `org/teams/get/scenario.yaml` | 14 | 5.61s | 6.85s |
| `event-gateway/adopt/scenario.yaml` | 14 | 5.57s | 6.58s |
| `dump/ai-gateways/scenario.yaml` | 14 | 5.48s | 8.91s |
| `plan/sync-partial-scope/scenario.yaml` | 14 | 5.37s | 8.57s |
| `deck/sync/scenario.yaml` | 14 | 5.29s | 10.13s |
| `org/teams/external-role/scenario.yaml` | 14 | 5.28s | 9.57s |
| `org/users/get/scenario.yaml` | 14 | 5.27s | 6.04s |
| `deck/multi-file/scenario.yaml` | 14 | 5.25s | 8.30s |
| `analytics/dashboard/scenario.yaml` | 14 | 5.24s | 8.65s |
| `require-namespace/portal/scenario.yaml` | 14 | 5.21s | 8.51s |
| `control-plane/delete-groups/scenario.yaml` | 14 | 5.19s | 6.65s |
| `portal/oidc-auth-strategy/scenario.yaml` | 14 | 5.18s | 7.96s |
| `event-gateway/backend-cluster-pagination/scenario.yaml` | 14 | 5.15s | 5.79s |
| `portal/default_application_auth_strategy_ref_selection/scenario.yaml` | 14 | 5.10s | 6.42s |
| `apis/versions-pagination/scenario.yaml` | 14 | 4.97s | 5.91s |
| `org/system-accounts/scenario.yaml` | 14 | 4.92s | 6.42s |
| `ai-gateway/model-matrix/scenario.yaml` | 14 | 4.77s | 8.88s |
| `external/portal-sync/scenario.yaml` | 14 | 4.71s | 6.08s |
| `event-gateway/tls-trust-bundle/scenario.yaml` | 14 | 4.70s | 9.27s |
| `external/api-impl/scenario.yaml` | 14 | 4.68s | 8.77s |
| `control-plane/get/scenario.yaml` | 14 | 4.34s | 6.86s |
| `ai-gateway/policy/scenario.yaml` | 14 | 4.33s | 5.76s |
| `plan/sync-workflow/scenario.yaml` | 14 | 4.20s | 7.48s |
| `org/get-org/scenario.yaml` | 14 | 4.15s | 4.81s |
| `yaml-tags/file/scenario.yaml` | 14 | 4.12s | 6.48s |
| `dump/analytics-dashboards/scenario.yaml` | 14 | 4.10s | 8.68s |
| `apis/get-api-by-name-and-id/scenario.yaml` | 14 | 4.05s | 6.09s |
| `portal/snippets-pagination/scenario.yaml` | 14 | 4.01s | 5.30s |
| `delete/dry-run/scenario.yaml` | 14 | 3.81s | 6.15s |
| `control-plane/groups/scenario.yaml` | 14 | 3.77s | 6.90s |
| `org/teams/apply/scenario.yaml` | 14 | 3.72s | 6.19s |
| `require-namespace/external/scenario.yaml` | 14 | 3.71s | 6.04s |
| `event-gateway/dump/scenario.yaml` | 14 | 3.66s | 7.09s |
| `control-plane/sync/scenario.yaml` | 14 | 3.61s | 5.80s |
| `protected-resources/control-planes/scenario.yaml` | 14 | 3.34s | 6.41s |
| `namespace/defaults-sync/scenario.yaml` | 14 | 3.15s | 5.89s |
| `errors/declarative/scenario.yaml` | 14 | 3.08s | 4.78s |
| `org/teams/sync/scenario.yaml` | 14 | 2.88s | 5.37s |
| `delete/idempotent/scenario.yaml` | 14 | 1.85s | 4.12s |
| `portal/email-domains/scenario.yaml` | 14 | 1.50s | 3.34s |
| `explain/command-coverage/scenario.yaml` | 14 | 1.34s | 1.43s |
| `analytics/dashboard-repro/scenario.yaml` | 14 | 1.03s | 1.31s |
| `scaffold/command-coverage/scenario.yaml` | 14 | 0.98s | 1.00s |
| `namespace/validation/scenario.yaml` | 14 | 0.91s | 1.06s |
| `declarative/plan-mode-validation/scenario.yaml` | 14 | 0.57s | 0.59s |
| `patch/command-coverage/scenario.yaml` | 14 | 0.46s | 0.47s |
| `ai-gateway/maturity/scenario.yaml` | 14 | 0.42s | 0.45s |
| `lint/command-coverage/scenario.yaml` | 14 | 0.34s | 0.35s |
| `portal/auth_settings_deprecated_fields/scenario.yaml` | 14 | 0.19s | 0.19s |
| `portal/identity_providers_duplicate_type/scenario.yaml` | 14 | 0.19s | 0.20s |
| `smoke/version/scenario.yaml` | 14 | 0.06s | 0.07s |
| `auth/get-me/scenario.yaml` | 14 | 0.00s | 0.00s |
| `event-gateway/principal-metadata/scenario.yaml` | 14 | 0.00s | 0.00s |
| `portal/applications/scenario.yaml` | 14 | 0.00s | 0.00s |
