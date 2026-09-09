# Konnect `.com` E2E baseline

Repository: `kong/kongctl`
Cohort: `cache-enabled`
Allocation: `weighted-v1:a30975c68af38565631cb7379995b28ad40edfeee6a58344ae1dc52aa0c323af`

Full successful runs: 20 of 20
Status: **frozen preliminary**

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
| workflow_admission_delay_seconds | 5.0s | 90.0s | 480.0s |
| queue_to_required_status_seconds | 644.0s | 835.0s | 1091.0s |
| build_job_seconds | 71.0s | 76.0s | 147.0s |
| build_kongctl_seconds | 16.0s | 17.0s | 20.0s |
| build_scenario_binary_seconds | 10.0s | 10.0s | 11.0s |
| build_setup_seconds | 25.0s | 29.0s | 35.0s |
| harness_job_seconds | 74.0s | 75.0s | 77.0s |
| harness_setup_seconds | 9.0s | 9.0s | 11.0s |
| harness_test_seconds | 53.0s | 54.0s | 54.0s |
| longest_shard_seconds | 417.0s | 456.0s | 461.0s |
| shard_spread_seconds | 210.0s | 240.0s | 261.0s |

## Reset cost per workflow run

| Metric | p50 | p75 | p90 |
| --- | ---: | ---: | ---: |
| count | 165.0 | 165.0 | 165.0 |
| duration_ms | 392523.0ms | 477492.0ms | 501223.0ms |
| list_calls | 2430.0 | 2430.0 | 2430.0 |
| list_duration_ms | 381991.0ms | 466125.0ms | 488978.0ms |
| resources_found | 1617.0 | 1619.0 | 1619.0 |
| delete_calls | 103.0 | 105.0 | 105.0 |
| delete_duration_ms | 8674.0ms | 9974.0ms | 10651.0ms |
| resources_deleted | 84.0 | 86.0 | 86.0 |

## Included runs

| Run / Attempt | Attempt created | Queue-to-status | Build | Longest shard | Spread | Resets |
| --- | --- | ---: | ---: | ---: | ---: | ---: |
| [34368479403 / 1](https://github.com/Kong/kongctl/actions/runs/34368479403/attempts/1) | 2026-09-09T15:10:23Z | 644s | 72s | 465s | 240s | 165 |
| [34260665443 / 1](https://github.com/Kong/kongctl/actions/runs/34260665443/attempts/1) | 2026-09-08T18:02:28Z | 670s | 147s | 426s | 179s | 165 |
| [34257236164 / 1](https://github.com/Kong/kongctl/actions/runs/34257236164/attempts/1) | 2026-09-08T17:27:45Z | 507s | 70s | 329s | 102s | 165 |
| [34253289486 / 1](https://github.com/Kong/kongctl/actions/runs/34253289486/attempts/1) | 2026-09-08T16:48:18Z | 686s | 76s | 461s | 228s | 165 |
| [34245299396 / 1](https://github.com/Kong/kongctl/actions/runs/34245299396/attempts/1) | 2026-09-08T15:31:31Z | 624s | 79s | 411s | 211s | 165 |
| [34241003911 / 1](https://github.com/Kong/kongctl/actions/runs/34241003911/attempts/1) | 2026-09-08T14:51:41Z | 1091s | 79s | 427s | 197s | 165 |
| [34240227901 / 1](https://github.com/Kong/kongctl/actions/runs/34240227901/attempts/1) | 2026-09-08T14:44:39Z | 593s | 68s | 409s | 220s | 165 |
| [34237954006 / 1](https://github.com/Kong/kongctl/actions/runs/34237954006/attempts/1) | 2026-09-08T14:23:34Z | 862s | 63s | 449s | 253s | 165 |
| [34235927544 / 1](https://github.com/Kong/kongctl/actions/runs/34235927544/attempts/1) | 2026-09-08T14:04:27Z | 737s | 72s | 450s | 231s | 165 |
| [34134847373 / 1](https://github.com/Kong/kongctl/actions/runs/34134847373/attempts/1) | 2026-09-07T14:47:08Z | 2376s | 66s | 397s | 187s | 165 |
| [34134840177 / 1](https://github.com/Kong/kongctl/actions/runs/34134840177/attempts/1) | 2026-09-07T14:47:03Z | 1805s | 64s | 417s | 203s | 165 |
| [34134809067 / 1](https://github.com/Kong/kongctl/actions/runs/34134809067/attempts/1) | 2026-09-07T14:46:41Z | 640s | 71s | 463s | 279s | 165 |
| [34133685166 / 1](https://github.com/Kong/kongctl/actions/runs/34133685166/attempts/1) | 2026-09-07T14:34:26Z | 576s | 66s | 399s | 204s | 165 |
| [34128517952 / 1](https://github.com/Kong/kongctl/actions/runs/34128517952/attempts/1) | 2026-09-07T13:38:23Z | 773s | 309s | 368s | 172s | 165 |
| [34106480009 / 1](https://github.com/Kong/kongctl/actions/runs/34106480009/attempts/1) | 2026-09-07T09:31:15Z | 881s | 245s | 456s | 256s | 165 |
| [34075125370 / 1](https://github.com/Kong/kongctl/actions/runs/34075125370/attempts/1) | 2026-09-07T02:05:44Z | 835s | 69s | 350s | 152s | 165 |
| [34074836321 / 1](https://github.com/Kong/kongctl/actions/runs/34074836321/attempts/1) | 2026-09-07T02:00:31Z | 624s | 64s | 459s | 264s | 165 |
| [34073199721 / 1](https://github.com/Kong/kongctl/actions/runs/34073199721/attempts/1) | 2026-09-07T01:29:53Z | 547s | 74s | 374s | 176s | 165 |
| [34068350161 / 1](https://github.com/Kong/kongctl/actions/runs/34068350161/attempts/1) | 2026-09-06T23:58:24Z | 640s | 73s | 457s | 261s | 165 |
| [34067536921 / 1](https://github.com/Kong/kongctl/actions/runs/34067536921/attempts/1) | 2026-09-06T23:40:13Z | 615s | 54s | 393s | 210s | 164 |

## Organization shards

| Run | Organization | Admission delay | Selected | Execution |
| --- | --- | ---: | ---: | ---: |
| 34368479403 | `kongctl-acceptance` | 4s | 38 | 234s |
| 34368479403 | `kongctl-acceptance-2` | 5s | 32 | 225s |
| 34368479403 | `kongctl-acceptance-3` | 5s | 31 | 426s |
| 34368479403 | `kongctl-acceptance-4` | 4s | 33 | 465s |
| 34368479403 | `kongctl-acceptance-5` | 4s | 32 | 404s |
| 34260665443 | `kongctl-acceptance` | 4s | 38 | 391s |
| 34260665443 | `kongctl-acceptance-2` | 4s | 32 | 392s |
| 34260665443 | `kongctl-acceptance-3` | 4s | 31 | 426s |
| 34260665443 | `kongctl-acceptance-4` | 3s | 33 | 247s |
| 34260665443 | `kongctl-acceptance-5` | 3s | 32 | 280s |
| 34257236164 | `kongctl-acceptance` | 3s | 38 | 325s |
| 34257236164 | `kongctl-acceptance-2` | 4s | 32 | 329s |
| 34257236164 | `kongctl-acceptance-3` | 3s | 31 | 276s |
| 34257236164 | `kongctl-acceptance-4` | 3s | 33 | 315s |
| 34257236164 | `kongctl-acceptance-5` | 3s | 32 | 227s |
| 34253289486 | `kongctl-acceptance` | 4s | 38 | 461s |
| 34253289486 | `kongctl-acceptance-2` | 3s | 32 | 335s |
| 34253289486 | `kongctl-acceptance-3` | 3s | 31 | 233s |
| 34253289486 | `kongctl-acceptance-4` | 3s | 33 | 239s |
| 34253289486 | `kongctl-acceptance-5` | 3s | 32 | 422s |
| 34245299396 | `kongctl-acceptance` | 3s | 38 | 219s |
| 34245299396 | `kongctl-acceptance-2` | 3s | 32 | 200s |
| 34245299396 | `kongctl-acceptance-3` | 3s | 31 | 364s |
| 34245299396 | `kongctl-acceptance-4` | 3s | 33 | 411s |
| 34245299396 | `kongctl-acceptance-5` | 3s | 32 | 201s |
| 34241003911 | `kongctl-acceptance` | 5s | 38 | 374s |
| 34241003911 | `kongctl-acceptance-2` | 5s | 32 | 230s |
| 34241003911 | `kongctl-acceptance-3` | 5s | 31 | 427s |
| 34241003911 | `kongctl-acceptance-4` | 5s | 33 | 378s |
| 34241003911 | `kongctl-acceptance-5` | 41s | 32 | 367s |
| 34240227901 | `kongctl-acceptance` | 3s | 38 | 222s |
| 34240227901 | `kongctl-acceptance-2` | 4s | 32 | 409s |
| 34240227901 | `kongctl-acceptance-3` | 4s | 31 | 189s |
| 34240227901 | `kongctl-acceptance-4` | 4s | 33 | 212s |
| 34240227901 | `kongctl-acceptance-5` | 4s | 32 | 368s |
| 34237954006 | `kongctl-acceptance` | 9s | 38 | 449s |
| 34237954006 | `kongctl-acceptance-2` | 6s | 32 | 411s |
| 34237954006 | `kongctl-acceptance-3` | 4s | 31 | 196s |
| 34237954006 | `kongctl-acceptance-4` | 3s | 33 | 216s |
| 34237954006 | `kongctl-acceptance-5` | 5s | 32 | 216s |
| 34235927544 | `kongctl-acceptance` | 3s | 38 | 450s |
| 34235927544 | `kongctl-acceptance-2` | 3s | 32 | 314s |
| 34235927544 | `kongctl-acceptance-3` | 3s | 31 | 285s |
| 34235927544 | `kongctl-acceptance-4` | 2s | 33 | 219s |
| 34235927544 | `kongctl-acceptance-5` | 3s | 32 | 276s |
| 34134847373 | `kongctl-acceptance` | 3s | 38 | 240s |
| 34134847373 | `kongctl-acceptance-2` | 3s | 32 | 397s |
| 34134847373 | `kongctl-acceptance-3` | 3s | 31 | 284s |
| 34134847373 | `kongctl-acceptance-4` | 3s | 33 | 220s |
| 34134847373 | `kongctl-acceptance-5` | 2s | 32 | 210s |
| 34134840177 | `kongctl-acceptance` | 3s | 38 | 388s |
| 34134840177 | `kongctl-acceptance-2` | 2s | 32 | 214s |
| 34134840177 | `kongctl-acceptance-3` | 3s | 31 | 417s |
| 34134840177 | `kongctl-acceptance-4` | 3s | 33 | 217s |
| 34134840177 | `kongctl-acceptance-5` | 2s | 32 | 216s |
| 34134809067 | `kongctl-acceptance` | 3s | 38 | 440s |
| 34134809067 | `kongctl-acceptance-2` | 3s | 32 | 330s |
| 34134809067 | `kongctl-acceptance-3` | 3s | 31 | 184s |
| 34134809067 | `kongctl-acceptance-4` | 3s | 33 | 463s |
| 34134809067 | `kongctl-acceptance-5` | 3s | 32 | 426s |
| 34133685166 | `kongctl-acceptance` | 4s | 38 | 399s |
| 34133685166 | `kongctl-acceptance-2` | 4s | 32 | 207s |
| 34133685166 | `kongctl-acceptance-3` | 4s | 31 | 195s |
| 34133685166 | `kongctl-acceptance-4` | 4s | 33 | 219s |
| 34133685166 | `kongctl-acceptance-5` | 4s | 32 | 216s |
| 34128517952 | `kongctl-acceptance` | 3s | 38 | 368s |
| 34128517952 | `kongctl-acceptance-2` | 3s | 32 | 265s |
| 34128517952 | `kongctl-acceptance-3` | 3s | 31 | 215s |
| 34128517952 | `kongctl-acceptance-4` | 3s | 33 | 298s |
| 34128517952 | `kongctl-acceptance-5` | 3s | 32 | 196s |
| 34106480009 | `kongctl-acceptance` | 4s | 38 | 449s |
| 34106480009 | `kongctl-acceptance-2` | 4s | 32 | 200s |
| 34106480009 | `kongctl-acceptance-3` | 4s | 31 | 423s |
| 34106480009 | `kongctl-acceptance-4` | 4s | 33 | 456s |
| 34106480009 | `kongctl-acceptance-5` | 4s | 32 | 212s |
| 34075125370 | `kongctl-acceptance` | 3s | 38 | 295s |
| 34075125370 | `kongctl-acceptance-2` | 4s | 32 | 350s |
| 34075125370 | `kongctl-acceptance-3` | 3s | 31 | 198s |
| 34075125370 | `kongctl-acceptance-4` | 3s | 33 | 233s |
| 34075125370 | `kongctl-acceptance-5` | 4s | 32 | 331s |
| 34074836321 | `kongctl-acceptance` | 3s | 38 | 431s |
| 34074836321 | `kongctl-acceptance-2` | 2s | 32 | 195s |
| 34074836321 | `kongctl-acceptance-3` | 3s | 31 | 336s |
| 34074836321 | `kongctl-acceptance-4` | 3s | 33 | 459s |
| 34074836321 | `kongctl-acceptance-5` | 3s | 32 | 407s |
| 34073199721 | `kongctl-acceptance` | 3s | 38 | 238s |
| 34073199721 | `kongctl-acceptance-2` | 3s | 32 | 342s |
| 34073199721 | `kongctl-acceptance-3` | 3s | 31 | 350s |
| 34073199721 | `kongctl-acceptance-4` | 3s | 33 | 374s |
| 34073199721 | `kongctl-acceptance-5` | 3s | 32 | 198s |
| 34068350161 | `kongctl-acceptance` | 3s | 38 | 226s |
| 34068350161 | `kongctl-acceptance-2` | 4s | 32 | 212s |
| 34068350161 | `kongctl-acceptance-3` | 3s | 31 | 196s |
| 34068350161 | `kongctl-acceptance-4` | 3s | 33 | 457s |
| 34068350161 | `kongctl-acceptance-5` | 39s | 32 | 218s |
| 34067536921 | `kongctl-acceptance` | 3s | 36 | 393s |
| 34067536921 | `kongctl-acceptance-2` | 2s | 32 | 265s |
| 34067536921 | `kongctl-acceptance-3` | 2s | 33 | 187s |
| 34067536921 | `kongctl-acceptance-4` | 3s | 32 | 183s |
| 34067536921 | `kongctl-acceptance-5` | 2s | 32 | 194s |

## Individual scenario durations

| Scenario | Samples | Median | p90 |
| --- | ---: | ---: | ---: |
| `portal/visibility/scenario.yaml` | 20 | 35.58s | 52.67s |
| `dump/portal-owned/scenario.yaml` | 20 | 32.81s | 50.78s |
| `ai-gateway/consumer-pagination/scenario.yaml` | 19 | 28.36s | 46.38s |
| `event-gateway/plan/sync-workflow/scenario.yaml` | 20 | 23.19s | 43.03s |
| `portal/api_docs_with_children/scenario.yaml` | 20 | 22.37s | 35.44s |
| `portal/sync/scenario.yaml` | 20 | 20.41s | 37.51s |
| `apis/nested-child-lifecycle/scenario.yaml` | 20 | 19.89s | 29.62s |
| `event-gateway/produce-policy/scenario.yaml` | 20 | 18.76s | 29.48s |
| `all/scenario.yaml` | 20 | 18.25s | 32.26s |
| `portal/teams/scenario.yaml` | 20 | 18.14s | 28.04s |
| `event-gateway/plan/apply-workflow/scenario.yaml` | 20 | 17.51s | 32.54s |
| `ai-gateway/consumer/scenario.yaml` | 20 | 16.84s | 24.31s |
| `dump/organization-teams/scenario.yaml` | 20 | 16.50s | 20.11s |
| `org/users/assignments/scenario.yaml` | 20 | 16.48s | 19.93s |
| `portal/email-templates/scenario.yaml` | 20 | 16.13s | 19.43s |
| `portal/custom-domain/scenario.yaml` | 20 | 16.11s | 27.02s |
| `event-gateway/external-sync/scenario.yaml` | 20 | 15.96s | 24.40s |
| `portal/identity_providers/scenario.yaml` | 20 | 15.79s | 28.51s |
| `yaml-tags/env/scenario.yaml` | 20 | 14.94s | 18.04s |
| `org/system-accounts/assignments/scenario.yaml` | 20 | 14.71s | 17.56s |
| `event-gateway/consume-policy/scenario.yaml` | 20 | 14.68s | 28.48s |
| `portal/customization/scenario.yaml` | 20 | 14.51s | 17.51s |
| `org/teams/roles/scenario.yaml` | 20 | 14.42s | 17.70s |
| `portal/audit-log-webhook/scenario.yaml` | 20 | 14.33s | 22.42s |
| `ai-gateway/data-plane-certificate/scenario.yaml` | 20 | 14.07s | 16.67s |
| `event-gateway/static-key/scenario.yaml` | 20 | 13.84s | 16.51s |
| `dcr-providers/workflow/scenario.yaml` | 20 | 13.63s | 21.42s |
| `portal/page-frontmatter-conflict/scenario.yaml` | 20 | 13.00s | 15.45s |
| `ai-gateway/mcp-server/scenario.yaml` | 20 | 12.86s | 24.04s |
| `ai-gateway/policy-matrix/scenario.yaml` | 20 | 12.80s | 15.14s |
| `apis/root-level-publication-visibility/scenario.yaml` | 20 | 12.29s | 17.66s |
| `diff/command-coverage/scenario.yaml` | 20 | 12.10s | 18.54s |
| `ai-gateway/model-provider/scenario.yaml` | 20 | 12.08s | 18.90s |
| `org/users/sync/scenario.yaml` | 20 | 11.83s | 14.16s |
| `event-gateway/backend-cluster/scenario.yaml` | 20 | 11.52s | 17.95s |
| `org/users/plan/sync-workflow/scenario.yaml` | 20 | 11.51s | 13.86s |
| `ai-gateway/consumer-group/scenario.yaml` | 20 | 11.39s | 21.09s |
| `dump/filtered/scenario.yaml` | 20 | 11.33s | 16.47s |
| `apis/control-plane-implementation/scenario.yaml` | 20 | 11.30s | 17.92s |
| `org/teams/external-role/scenario.yaml` | 20 | 11.27s | 14.01s |
| `portal/auth-strategy-link/scenario.yaml` | 20 | 11.24s | 20.23s |
| `apis/comprehensive-fields/scenario.yaml` | 20 | 11.14s | 13.53s |
| `dump/analytics-dashboards/scenario.yaml` | 20 | 11.08s | 13.06s |
| `org/system-accounts/plan/sync-workflow/scenario.yaml` | 20 | 11.05s | 13.49s |
| `ai-gateway/model/scenario.yaml` | 20 | 10.55s | 12.96s |
| `ai-gateway/agent/scenario.yaml` | 20 | 10.44s | 15.63s |
| `org/system-accounts/sync/scenario.yaml` | 20 | 10.38s | 12.51s |
| `event-gateway/tls-trust-bundle/scenario.yaml` | 20 | 10.23s | 12.32s |
| `portal/ip-allow-list/scenario.yaml` | 20 | 10.07s | 20.01s |
| `event-gateway/dataplane-certificate/scenario.yaml` | 20 | 10.02s | 12.38s |
| `ai-gateway/root/scenario.yaml` | 20 | 9.74s | 15.53s |
| `apis/region/scenario.yaml` | 20 | 9.68s | 12.95s |
| `ai-gateway/runtime-tls/scenario.yaml` | 20 | 9.52s | 12.58s |
| `external/api-parent/scenario.yaml` | 20 | 9.46s | 14.73s |
| `portal/email/scenario.yaml` | 20 | 9.35s | 14.35s |
| `deck/basic/scenario.yaml` | 20 | 9.32s | 17.64s |
| `control-plane/serverless/scenario.yaml` | 20 | 9.19s | 13.54s |
| `protected-resources/apis/scenario.yaml` | 20 | 9.19s | 14.16s |
| `external/portal-publication/scenario.yaml` | 20 | 9.14s | 11.04s |
| `portal/external-sync/scenario.yaml` | 20 | 9.14s | 16.78s |
| `plan/apply-workflow/scenario.yaml` | 20 | 9.04s | 13.71s |
| `ai-gateway/vault/scenario.yaml` | 20 | 8.93s | 13.25s |
| `event-gateway/cluster-policy/scenario.yaml` | 20 | 8.84s | 13.49s |
| `event-gateway/schema-registry/scenario.yaml` | 20 | 8.83s | 14.07s |
| `event-gateway/virtual-cluster/scenario.yaml` | 20 | 8.83s | 16.36s |
| `event-gateway/listener-policy/scenario.yaml` | 20 | 8.51s | 13.28s |
| `org/users/plan/apply-workflow/scenario.yaml` | 20 | 8.45s | 10.09s |
| `ai-gateway/config-store/scenario.yaml` | 20 | 8.40s | 15.96s |
| `org/token/scenario.yaml` | 20 | 8.22s | 9.92s |
| `catalog/service/scenario.yaml` | 20 | 8.21s | 12.14s |
| `dump/ai-gateways/scenario.yaml` | 20 | 8.17s | 10.05s |
| `event-gateway/dump/scenario.yaml` | 20 | 8.16s | 10.18s |
| `org/system-accounts/plan/apply-workflow/scenario.yaml` | 20 | 8.16s | 10.07s |
| `protected-resources/portals/scenario.yaml` | 20 | 8.07s | 11.53s |
| `portal/pages/scenario.yaml` | 20 | 7.98s | 14.72s |
| `ai-gateway/model-matrix/scenario.yaml` | 20 | 7.82s | 11.82s |
| `deck/sync/scenario.yaml` | 20 | 7.78s | 11.96s |
| `dump/control-planes/scenario.yaml` | 20 | 7.74s | 11.66s |
| `analytics/dashboard/scenario.yaml` | 20 | 7.73s | 10.95s |
| `event-gateway/diff/scenario.yaml` | 20 | 7.70s | 14.74s |
| `event-gateway/listener/scenario.yaml` | 20 | 7.60s | 12.01s |
| `adopt/full/scenario.yaml` | 20 | 7.58s | 13.62s |
| `ai-gateway/auth-strategy/scenario.yaml` | 20 | 7.50s | 14.26s |
| `portal/default_application_auth_strategy/scenario.yaml` | 20 | 7.43s | 13.44s |
| `control-plane/data-plane-certificate/scenario.yaml` | 20 | 7.41s | 13.20s |
| `portal/oidc-auth-strategy/scenario.yaml` | 20 | 7.41s | 9.25s |
| `portal/publication-auth-omitted-noop/scenario.yaml` | 20 | 7.02s | 10.70s |
| `event-gateway/topic-aliases/scenario.yaml` | 20 | 6.98s | 11.01s |
| `external/ai-gateway-parent/scenario.yaml` | 20 | 6.93s | 11.40s |
| `portal/idp_team_group_mappings_readback/scenario.yaml` | 20 | 6.91s | 13.04s |
| `apis/eventual-consistency-polp/scenario.yaml` | 20 | 6.89s | 8.20s |
| `portal/auth_settings/scenario.yaml` | 20 | 6.87s | 13.79s |
| `portal/integrations/scenario.yaml` | 20 | 6.82s | 10.65s |
| `portal/assets/scenario.yaml` | 20 | 6.74s | 12.44s |
| `apis/versions-pagination/scenario.yaml` | 20 | 6.67s | 8.16s |
| `adopt/auth-strategy-adopt/scenario.yaml` | 20 | 6.61s | 8.04s |
| `external/api-impl/scenario.yaml` | 20 | 6.61s | 10.12s |
| `delete/declarative/scenario.yaml` | 20 | 6.45s | 12.16s |
| `portal/default_application_auth_strategy_ref_selection/scenario.yaml` | 20 | 6.39s | 8.15s |
| `declarative/rename-sync-delete/scenario.yaml` | 20 | 6.25s | 12.79s |
| `org/teams/get/scenario.yaml` | 20 | 6.20s | 7.84s |
| `portal/edit/scenario.yaml` | 20 | 6.20s | 9.50s |
| `portal/app-auth-strategy/scenario.yaml` | 20 | 6.14s | 10.68s |
| `require-namespace/portal/scenario.yaml` | 20 | 5.97s | 9.97s |
| `plan/sync-partial-scope/scenario.yaml` | 20 | 5.90s | 8.71s |
| `control-plane/sync-groups/scenario.yaml` | 20 | 5.88s | 8.89s |
| `protected-resources/event-gateways/scenario.yaml` | 20 | 5.88s | 11.67s |
| `apis/child-delete-namespace/scenario.yaml` | 20 | 5.67s | 11.27s |
| `control-plane/plan/apply-workflow/scenario.yaml` | 20 | 5.62s | 7.41s |
| `delete/partial-delete/scenario.yaml` | 20 | 5.59s | 10.08s |
| `control-plane/groups/scenario.yaml` | 20 | 5.57s | 8.24s |
| `org/users/get/scenario.yaml` | 20 | 5.55s | 6.74s |
| `deck/multi-file/scenario.yaml` | 20 | 5.50s | 8.41s |
| `portal/api_with_attributes/scenario.yaml` | 20 | 5.35s | 9.60s |
| `adopt/create-portal-adopt-dump-plan/scenario.yaml` | 20 | 5.22s | 7.52s |
| `portal/snippets/scenario.yaml` | 20 | 5.00s | 9.50s |
| `adopt/event-gateway-adopt/scenario.yaml` | 20 | 4.81s | 7.33s |
| `protected-resources/control-planes/scenario.yaml` | 20 | 4.79s | 7.46s |
| `delete/idempotent/scenario.yaml` | 20 | 4.74s | 5.91s |
| `plan/sync-workflow/scenario.yaml` | 20 | 4.74s | 8.86s |
| `delete/plan-based/scenario.yaml` | 20 | 4.66s | 7.25s |
| `control-plane/get/scenario.yaml` | 20 | 4.49s | 6.77s |
| `event-gateway/control-planes/scenario.yaml` | 20 | 4.46s | 8.71s |
| `control-plane/delete-groups/scenario.yaml` | 20 | 4.38s | 6.48s |
| `org/teams/plan/sync-workflow/scenario.yaml` | 20 | 4.37s | 7.24s |
| `deck/env-vars/scenario.yaml` | 20 | 4.33s | 8.67s |
| `namespace/defaults-sync/scenario.yaml` | 20 | 4.23s | 6.63s |
| `org/get-org/scenario.yaml` | 20 | 4.20s | 5.35s |
| `adopt/team-create-adopt-dump/scenario.yaml` | 20 | 4.14s | 5.87s |
| `org/teams/apply/scenario.yaml` | 20 | 4.07s | 6.51s |
| `plan/remote-url-workflow/scenario.yaml` | 20 | 4.02s | 7.40s |
| `org/teams/sync/scenario.yaml` | 20 | 4.01s | 6.44s |
| `deck/idempotent/scenario.yaml` | 20 | 3.93s | 7.64s |
| `org/system-accounts/scenario.yaml` | 20 | 3.92s | 6.36s |
| `delete/dry-run/scenario.yaml` | 20 | 3.90s | 6.25s |
| `event-gateway/dependency-order/scenario.yaml` | 20 | 3.87s | 7.78s |
| `ai-gateway/policy/scenario.yaml` | 20 | 3.64s | 5.83s |
| `require-namespace/external/scenario.yaml` | 20 | 3.60s | 5.82s |
| `control-plane/sync/scenario.yaml` | 20 | 3.56s | 5.70s |
| `errors/declarative/scenario.yaml` | 20 | 3.54s | 5.43s |
| `protected-resources/org/teams/scenario.yaml` | 20 | 3.54s | 7.12s |
| `portal/team_group_mappings_no_idp/scenario.yaml` | 20 | 3.50s | 8.08s |
| `yaml-tags/file/scenario.yaml` | 20 | 3.46s | 6.69s |
| `control-plane/apply/scenario.yaml` | 20 | 3.45s | 6.51s |
| `org/teams/plan/apply-workflow/scenario.yaml` | 20 | 3.44s | 6.54s |
| `event-gateway/backend-cluster-pagination/scenario.yaml` | 20 | 3.25s | 5.31s |
| `apis/get-api-by-name-and-id/scenario.yaml` | 20 | 3.11s | 5.91s |
| `event-gateway/adopt/scenario.yaml` | 20 | 3.08s | 6.77s |
| `dump/dcr-provider/scenario.yaml` | 20 | 3.00s | 5.83s |
| `external/portal-sync/scenario.yaml` | 20 | 2.88s | 5.81s |
| `portal/snippets-pagination/scenario.yaml` | 20 | 2.65s | 5.33s |
| `portal/email-domains/scenario.yaml` | 20 | 2.45s | 4.09s |
| `explain/command-coverage/scenario.yaml` | 20 | 1.41s | 1.47s |
| `analytics/dashboard-repro/scenario.yaml` | 20 | 1.01s | 1.34s |
| `scaffold/command-coverage/scenario.yaml` | 20 | 0.97s | 1.05s |
| `namespace/validation/scenario.yaml` | 20 | 0.86s | 1.02s |
| `declarative/plan-mode-validation/scenario.yaml` | 20 | 0.58s | 0.59s |
| `ai-gateway/maturity/scenario.yaml` | 20 | 0.45s | 0.46s |
| `patch/command-coverage/scenario.yaml` | 20 | 0.45s | 0.49s |
| `lint/command-coverage/scenario.yaml` | 20 | 0.34s | 0.37s |
| `portal/identity_providers_duplicate_type/scenario.yaml` | 20 | 0.19s | 0.20s |
| `portal/auth_settings_deprecated_fields/scenario.yaml` | 20 | 0.18s | 0.21s |
| `smoke/version/scenario.yaml` | 20 | 0.06s | 0.06s |
| `auth/get-me/scenario.yaml` | 20 | 0.00s | 0.00s |
| `event-gateway/principal-metadata/scenario.yaml` | 20 | 0.00s | 0.00s |
| `portal/applications/scenario.yaml` | 20 | 0.00s | 0.00s |
