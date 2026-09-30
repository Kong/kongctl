# MCP server replacements

This live scenario uses two independent gateways with identical route paths.
Each has a passthrough listener, conversion listener, and source-backed
listener. It exercises server-only renames, then server/auth/policy renames
with changed key-header names and response headers. Both stages save a sync
plan, check its direct replacement edges and execution order, execute that
saved file, and verify no-op convergence. Cleanup only targets its gateways.

```sh
KONGCTL_E2E_RESET=0 \
  KONGCTL_E2E_ARTIFACTS_DIR=.e2e-artifacts/mcp-replacement \
  make test-e2e-scenarios SCENARIO=ai-gateway/mcp-replacement
```

This new scenario is live-only; existing replay membership is unchanged.
The scenario validates API acceptance of overlapping routes for all three
serving variants. Planner tests cover matcher differences, defaults, gateway
and namespace isolation, many-to-many replacement edges, and cycle freedom.
An executor test rejects the replacement create and verifies that saved-plan
execution never calls deletion of the old server.

## Data-plane validation on September 30, 2026

An isolated Konnect `.com` gateway and local Docker data plane ran
`kong/kong-ai-gateway:2.1.0`, image digest
`sha256:bf784fd530115a7ca3b05e016490678c37b43366cf89434e25626c173fee6310`.
The data plane used `KONG_INCREMENTAL_SYNC=on`; its debug logs confirmed
`[kong.sync.v2] get_delta` requests. The control plane returned full snapshots
for these changes, so this run does not establish behavior under granular
entity deltas or every possible intermediate state.

A conversion listener answered MCP `initialize` calls approximately every
100 ms while executing a server-only saved sync plan, an auth/policy/server
saved sync plan, an additive overlap plan, and retirement of the old server.
The overlap was held for 10 seconds to inspect duplicate-route precedence.
The test used one consumer credential and sent both old/new header names
on sustained requests, then separately probed each authentication scheme.

The first run returned 500/500 valid MCP responses; a second run, with
distinct authentication during the extended overlap, returned 498/498.
Neither run recorded a 404, failed request, or invalid MCP response. Both
automatic rename stages converged to no-op plans. All temporary containers
and gateways were removed afterward.

In the second run, replacement creation/deletion completed at 23.88 seconds
after test start, but the first response with the new policy header arrived
at 26.34 seconds. During extended overlap, the new server began serving at
41.74 seconds, although its creation completed at 35.31 seconds. Before
retiring the old server, its key-header-only probe returned 401 and the
new server's key-header-only probe returned a valid 200 with the new policy
header. Thus the old route did not necessarily win while both existed.
This observation is specific to this runtime and configuration.

Local artifacts for this implementation run are under:

- `.e2e-artifacts/gh2356-scenarios/20260930-120117`
- `.e2e-artifacts/gh2356-traffic`
- `.e2e-artifacts/gh2356-traffic-auth-overlap`

The traffic directories contain saved plans, command results, data-plane
logs, and timestamped samples in `traffic.json`. They are local evidence,
not committed recordings. Review raw logs before sharing them because they
may contain runtime credentials. The finite traffic samples demonstrate
this tested cutover, not a zero-downtime guarantee or session continuity.
