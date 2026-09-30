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

## Data-plane validation

A live data-plane validation ran against a full-snapshot Konnect control
plane, so granular entity deltas were not exercised. Control-plane creation
success is not data-plane readiness; see the
[cutover guidance][mcp-cutover].

[mcp-cutover]:
  ../../../../../docs/declarative.md#ai-gateway-mcp-server-replacements
