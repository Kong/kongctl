# Konnect `.com` E2E baseline

Repository: `kong/kongctl`
Cohort: `cache-enabled`

Full successful runs: 2 of 20
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
| workflow_admission_delay_seconds | 3.0s | 4.0s | 4.0s |
| queue_to_required_status_seconds | 651.0s | 912.0s | 912.0s |
| build_job_seconds | 49.0s | 322.0s | 322.0s |
| build_kongctl_seconds | 4.0s | 243.0s | 243.0s |
| build_scenario_binary_seconds | 1.0s | 38.0s | 38.0s |
| build_setup_seconds | 11.0s | 25.0s | 25.0s |
| harness_job_seconds | 60.0s | 78.0s | 78.0s |
| harness_setup_seconds | 10.0s | 11.0s | 11.0s |
| harness_test_seconds | 38.0s | 53.0s | 53.0s |
| longest_shard_seconds | 485.0s | 496.0s | 496.0s |
| shard_spread_seconds | 267.0s | 332.0s | 332.0s |

## Reset cost per workflow run

| Metric | p50 | p75 | p90 |
| --- | ---: | ---: | ---: |
| count | 164.0 | 164.0 | 164.0 |
| duration_ms | 475189.0ms | 507850.0ms | 507850.0ms |
| list_calls | 2459.0 | 2459.0 | 2459.0 |
| list_duration_ms | 463423.0ms | 496309.0ms | 496309.0ms |
| resources_found | 1614.0 | 1614.0 | 1614.0 |
| delete_calls | 103.0 | 103.0 | 103.0 |
| delete_duration_ms | 10080.0ms | 10268.0ms | 10268.0ms |
| resources_deleted | 84.0 | 84.0 | 84.0 |

## Included runs

| Run / Attempt | Attempt created | Queue-to-status | Build | Longest shard | Spread | Resets |
| --- | --- | ---: | ---: | ---: | ---: | ---: |
| [33972692322 / 1](https://github.com/Kong/kongctl/actions/runs/33972692322/attempts/1) | 2026-09-05T14:44:15Z | 912s | 322s | 496s | 332s | 164 |
| [33906754133 / 3](https://github.com/Kong/kongctl/actions/runs/33906754133/attempts/3) | 2026-09-04T19:07:59Z | 651s | 49s | 485s | 267s | 164 |

## Organization shards

| Run | Organization | Admission delay | Selected | Execution |
| --- | --- | ---: | ---: | ---: |
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
| `portal/visibility/scenario.yaml` | 2 | 44.05s | 51.68s |
| `dump/portal-owned/scenario.yaml` | 2 | 41.92s | 50.95s |
| `all/scenario.yaml` | 2 | 28.80s | 33.86s |
| `event-gateway/produce-policy/scenario.yaml` | 2 | 26.66s | 26.84s |
| `event-gateway/plan/apply-workflow/scenario.yaml` | 2 | 25.80s | 32.53s |
| `portal/teams/scenario.yaml` | 2 | 25.51s | 26.05s |
| `portal/identity_providers/scenario.yaml` | 2 | 23.81s | 28.94s |
| `event-gateway/plan/sync-workflow/scenario.yaml` | 2 | 22.76s | 43.19s |
| `portal/auth-strategy-link/scenario.yaml` | 2 | 20.39s | 20.90s |
| `ai-gateway/mcp-server/scenario.yaml` | 2 | 19.47s | 24.26s |
| `portal/sync/scenario.yaml` | 2 | 19.31s | 36.91s |
| `portal/api_docs_with_children/scenario.yaml` | 2 | 17.84s | 29.25s |
| `portal/ip-allow-list/scenario.yaml` | 2 | 17.33s | 20.82s |
| `apis/root-level-publication-visibility/scenario.yaml` | 2 | 17.24s | 17.45s |
| `dump/organization-teams/scenario.yaml` | 2 | 16.67s | 17.03s |
| `org/users/assignments/scenario.yaml` | 2 | 16.02s | 16.50s |
| `portal/custom-domain/scenario.yaml` | 2 | 15.55s | 26.14s |
| `apis/control-plane-implementation/scenario.yaml` | 2 | 15.54s | 18.28s |
| `event-gateway/virtual-cluster/scenario.yaml` | 2 | 15.39s | 15.56s |
| `apis/nested-child-lifecycle/scenario.yaml` | 2 | 15.34s | 28.97s |
| `diff/command-coverage/scenario.yaml` | 2 | 15.33s | 18.07s |
| `ai-gateway/agent/scenario.yaml` | 2 | 15.03s | 15.06s |
| `event-gateway/backend-cluster/scenario.yaml` | 2 | 14.93s | 17.70s |
| `event-gateway/consume-policy/scenario.yaml` | 2 | 14.32s | 27.67s |
| `portal/external-sync/scenario.yaml` | 2 | 14.27s | 17.22s |
| `org/system-accounts/assignments/scenario.yaml` | 2 | 14.17s | 14.35s |
| `dump/filtered/scenario.yaml` | 2 | 13.92s | 16.29s |
| `portal/email/scenario.yaml` | 2 | 13.79s | 14.05s |
| `protected-resources/apis/scenario.yaml` | 2 | 13.65s | 14.52s |
| `plan/apply-workflow/scenario.yaml` | 2 | 13.26s | 13.42s |
| `yaml-tags/env/scenario.yaml` | 2 | 13.23s | 15.82s |
| `ai-gateway/data-plane-certificate/scenario.yaml` | 2 | 13.04s | 13.25s |
| `portal/default_application_auth_strategy/scenario.yaml` | 2 | 12.97s | 13.45s |
| `declarative/rename-sync-delete/scenario.yaml` | 2 | 12.83s | 13.08s |
| `control-plane/data-plane-certificate/scenario.yaml` | 2 | 12.81s | 13.19s |
| `dcr-providers/workflow/scenario.yaml` | 2 | 12.80s | 21.12s |
| `org/teams/roles/scenario.yaml` | 2 | 12.75s | 15.89s |
| `ai-gateway/config-store/scenario.yaml` | 2 | 12.66s | 16.62s |
| `ai-gateway/auth-strategy/scenario.yaml` | 2 | 12.64s | 15.45s |
| `control-plane/serverless/scenario.yaml` | 2 | 12.59s | 12.70s |
| `event-gateway/listener-policy/scenario.yaml` | 2 | 12.59s | 12.64s |
| `ai-gateway/vault/scenario.yaml` | 2 | 12.56s | 12.77s |
| `event-gateway/diff/scenario.yaml` | 2 | 12.02s | 14.84s |
| `event-gateway/static-key/scenario.yaml` | 2 | 11.81s | 14.60s |
| `org/users/plan/sync-workflow/scenario.yaml` | 2 | 11.77s | 11.89s |
| `portal/auth_settings/scenario.yaml` | 2 | 11.54s | 13.92s |
| `event-gateway/external-sync/scenario.yaml` | 2 | 11.43s | 19.46s |
| `event-gateway/schema-registry/scenario.yaml` | 2 | 11.43s | 13.96s |
| `org/users/sync/scenario.yaml` | 2 | 11.39s | 11.64s |
| `ai-gateway/policy-matrix/scenario.yaml` | 2 | 11.32s | 13.34s |
| `apis/comprehensive-fields/scenario.yaml` | 2 | 10.80s | 11.08s |
| `event-gateway/cluster-policy/scenario.yaml` | 2 | 10.76s | 13.41s |
| `portal/publication-auth-omitted-noop/scenario.yaml` | 2 | 10.69s | 10.90s |
| `delete/declarative/scenario.yaml` | 2 | 10.51s | 12.61s |
| `org/system-accounts/plan/sync-workflow/scenario.yaml` | 2 | 10.50s | 11.11s |
| `ai-gateway/model/scenario.yaml` | 2 | 10.47s | 10.70s |
| `portal/integrations/scenario.yaml` | 2 | 10.27s | 10.38s |
| `external/ai-gateway-parent/scenario.yaml` | 2 | 10.13s | 11.00s |
| `ai-gateway/consumer/scenario.yaml` | 2 | 10.10s | 16.78s |
| `ai-gateway/consumer-group/scenario.yaml` | 2 | 10.05s | 18.99s |
| `org/system-accounts/sync/scenario.yaml` | 2 | 9.98s | 10.17s |
| `portal/audit-log-webhook/scenario.yaml` | 2 | 9.88s | 17.77s |
| `event-gateway/listener/scenario.yaml` | 2 | 9.83s | 11.91s |
| `event-gateway/dataplane-certificate/scenario.yaml` | 2 | 9.78s | 10.14s |
| `protected-resources/portals/scenario.yaml` | 2 | 9.78s | 11.68s |
| `portal/api_with_attributes/scenario.yaml` | 2 | 9.40s | 9.55s |
| `ai-gateway/model-provider/scenario.yaml` | 2 | 9.15s | 15.02s |
| `external/portal-publication/scenario.yaml` | 2 | 8.92s | 9.18s |
| `ai-gateway/root/scenario.yaml` | 2 | 8.63s | 15.35s |
| `portal/app-auth-strategy/scenario.yaml` | 2 | 8.63s | 10.73s |
| `org/token/scenario.yaml` | 2 | 8.38s | 8.42s |
| `deck/env-vars/scenario.yaml` | 2 | 8.30s | 8.71s |
| `deck/basic/scenario.yaml` | 2 | 8.23s | 14.19s |
| `org/users/plan/apply-workflow/scenario.yaml` | 2 | 8.04s | 8.52s |
| `adopt/full/scenario.yaml` | 2 | 7.93s | 14.84s |
| `org/system-accounts/plan/apply-workflow/scenario.yaml` | 2 | 7.92s | 8.35s |
| `portal/snippets/scenario.yaml` | 2 | 7.82s | 9.48s |
| `portal/edit/scenario.yaml` | 2 | 7.71s | 9.34s |
| `control-plane/sync-groups/scenario.yaml` | 2 | 7.52s | 9.15s |
| `portal/email-templates/scenario.yaml` | 2 | 7.50s | 13.16s |
| `adopt/auth-strategy-adopt/scenario.yaml` | 2 | 7.39s | 8.31s |
| `apis/region/scenario.yaml` | 2 | 7.32s | 10.58s |
| `dump/ai-gateways/scenario.yaml` | 2 | 7.12s | 8.95s |
| `portal/pages/scenario.yaml` | 2 | 7.06s | 12.06s |
| `plan/sync-partial-scope/scenario.yaml` | 2 | 7.05s | 8.57s |
| `deck/multi-file/scenario.yaml` | 2 | 6.89s | 8.30s |
| `portal/page-frontmatter-conflict/scenario.yaml` | 2 | 6.82s | 12.68s |
| `portal/customization/scenario.yaml` | 2 | 6.81s | 11.92s |
| `adopt/event-gateway-adopt/scenario.yaml` | 2 | 6.80s | 7.55s |
| `deck/idempotent/scenario.yaml` | 2 | 6.78s | 7.82s |
| `event-gateway/dependency-order/scenario.yaml` | 2 | 6.75s | 7.98s |
| `protected-resources/org/teams/scenario.yaml` | 2 | 6.70s | 6.73s |
| `plan/remote-url-workflow/scenario.yaml` | 2 | 6.62s | 7.80s |
| `apis/eventual-consistency-polp/scenario.yaml` | 2 | 6.60s | 6.66s |
| `delete/plan-based/scenario.yaml` | 2 | 6.60s | 6.63s |
| `portal/assets/scenario.yaml` | 2 | 6.49s | 12.49s |
| `catalog/service/scenario.yaml` | 2 | 6.45s | 11.70s |
| `portal/oidc-auth-strategy/scenario.yaml` | 2 | 6.34s | 7.83s |
| `adopt/create-portal-adopt-dump-plan/scenario.yaml` | 2 | 6.28s | 8.51s |
| `ai-gateway/runtime-tls/scenario.yaml` | 2 | 6.04s | 9.87s |
| `portal/idp_team_group_mappings_readback/scenario.yaml` | 2 | 6.02s | 10.87s |
| `external/api-parent/scenario.yaml` | 2 | 5.98s | 11.23s |
| `org/teams/plan/apply-workflow/scenario.yaml` | 2 | 5.97s | 6.08s |
| `org/teams/plan/sync-workflow/scenario.yaml` | 2 | 5.95s | 7.68s |
| `deck/sync/scenario.yaml` | 2 | 5.92s | 11.89s |
| `dump/dcr-provider/scenario.yaml` | 2 | 5.91s | 6.02s |
| `control-plane/get/scenario.yaml` | 2 | 5.67s | 6.87s |
| `portal/default_application_auth_strategy_ref_selection/scenario.yaml` | 2 | 5.64s | 6.42s |
| `require-namespace/portal/scenario.yaml` | 2 | 5.58s | 9.74s |
| `control-plane/delete-groups/scenario.yaml` | 2 | 5.50s | 6.65s |
| `dump/control-planes/scenario.yaml` | 2 | 5.40s | 9.13s |
| `org/users/get/scenario.yaml` | 2 | 5.39s | 5.56s |
| `protected-resources/event-gateways/scenario.yaml` | 2 | 5.29s | 9.24s |
| `yaml-tags/file/scenario.yaml` | 2 | 5.29s | 6.39s |
| `event-gateway/tls-trust-bundle/scenario.yaml` | 2 | 5.25s | 11.01s |
| `analytics/dashboard/scenario.yaml` | 2 | 5.24s | 9.65s |
| `event-gateway/backend-cluster-pagination/scenario.yaml` | 2 | 5.20s | 5.79s |
| `ai-gateway/model-matrix/scenario.yaml` | 2 | 5.19s | 10.47s |
| `org/system-accounts/scenario.yaml` | 2 | 5.15s | 7.30s |
| `external/api-impl/scenario.yaml` | 2 | 5.14s | 9.91s |
| `org/teams/external-role/scenario.yaml` | 2 | 5.09s | 11.09s |
| `apis/child-delete-namespace/scenario.yaml` | 2 | 5.06s | 9.22s |
| `delete/dry-run/scenario.yaml` | 2 | 5.06s | 6.22s |
| `apis/get-api-by-name-and-id/scenario.yaml` | 2 | 5.00s | 6.13s |
| `org/teams/apply/scenario.yaml` | 2 | 4.99s | 6.18s |
| `apis/versions-pagination/scenario.yaml` | 2 | 4.97s | 5.91s |
| `external/portal-sync/scenario.yaml` | 2 | 4.97s | 6.09s |
| `delete/partial-delete/scenario.yaml` | 2 | 4.87s | 8.48s |
| `require-namespace/external/scenario.yaml` | 2 | 4.86s | 6.18s |
| `ai-gateway/policy/scenario.yaml` | 2 | 4.74s | 5.80s |
| `dump/analytics-dashboards/scenario.yaml` | 2 | 4.71s | 9.93s |
| `event-gateway/topic-aliases/scenario.yaml` | 2 | 4.71s | 8.72s |
| `control-plane/sync/scenario.yaml` | 2 | 4.55s | 5.61s |
| `plan/sync-workflow/scenario.yaml` | 2 | 4.53s | 8.72s |
| `org/get-org/scenario.yaml` | 2 | 4.37s | 4.40s |
| `portal/snippets-pagination/scenario.yaml` | 2 | 4.32s | 5.24s |
| `control-plane/groups/scenario.yaml` | 2 | 4.07s | 7.94s |
| `event-gateway/dump/scenario.yaml` | 2 | 4.05s | 8.40s |
| `protected-resources/control-planes/scenario.yaml` | 2 | 3.85s | 7.04s |
| `event-gateway/control-planes/scenario.yaml` | 2 | 3.80s | 6.96s |
| `adopt/team-create-adopt-dump/scenario.yaml` | 2 | 3.50s | 9.90s |
| `control-plane/plan/apply-workflow/scenario.yaml` | 2 | 3.33s | 5.84s |
| `namespace/defaults-sync/scenario.yaml` | 2 | 3.32s | 6.51s |
| `control-plane/apply/scenario.yaml` | 2 | 3.26s | 5.40s |
| `errors/declarative/scenario.yaml` | 2 | 3.19s | 5.39s |
| `portal/team_group_mappings_no_idp/scenario.yaml` | 2 | 3.02s | 6.46s |
| `org/teams/sync/scenario.yaml` | 2 | 3.01s | 6.33s |
| `event-gateway/adopt/scenario.yaml` | 2 | 2.58s | 5.25s |
| `org/teams/get/scenario.yaml` | 2 | 2.55s | 5.44s |
| `delete/idempotent/scenario.yaml` | 2 | 2.03s | 4.46s |
| `portal/email-domains/scenario.yaml` | 2 | 1.58s | 3.84s |
| `explain/command-coverage/scenario.yaml` | 2 | 1.05s | 1.43s |
| `scaffold/command-coverage/scenario.yaml` | 2 | 0.96s | 0.99s |
| `analytics/dashboard-repro/scenario.yaml` | 2 | 0.90s | 1.31s |
| `namespace/validation/scenario.yaml` | 2 | 0.86s | 0.89s |
| `declarative/plan-mode-validation/scenario.yaml` | 2 | 0.56s | 0.56s |
| `patch/command-coverage/scenario.yaml` | 2 | 0.44s | 0.45s |
| `ai-gateway/maturity/scenario.yaml` | 2 | 0.34s | 0.45s |
| `lint/command-coverage/scenario.yaml` | 2 | 0.23s | 0.34s |
| `portal/identity_providers_duplicate_type/scenario.yaml` | 2 | 0.19s | 0.21s |
| `portal/auth_settings_deprecated_fields/scenario.yaml` | 2 | 0.11s | 0.19s |
| `smoke/version/scenario.yaml` | 2 | 0.05s | 0.06s |
| `auth/get-me/scenario.yaml` | 2 | 0.00s | 0.00s |
| `event-gateway/principal-metadata/scenario.yaml` | 2 | 0.00s | 0.00s |
| `portal/applications/scenario.yaml` | 2 | 0.00s | 0.00s |
