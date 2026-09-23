# AI Gateway 2.1 fields

This scenario creates and updates a runtime 2.1 gateway with the new API
fields, then checks their exact remote values through declarative dumps:

- Model `input_cost_list`, `output_cost_list`, and `cache_read_cost_list`.
- Multiple model selector aliases, supported by the SDK 0.68.1 spec.
- Policy `condition`.
- MCP listener and conversion-listener `allowed_versions` and both cache
  hints, including zero TTLs and public/private cache scopes.
- MCP upstream-server `upstream_protocol_version`.

Both source manifests and dumps must produce no-change sync plans after
each stage. Creation/update counts are checked before mutation; retried
syncs check successful completion without assuming a final-attempt count.
Cleanup is namespace-scoped and verifies the gateway is absent afterward.
The initial isolation check prevents taking over a pre-existing test gateway.

Run with the standard E2E credentials for an environment supporting 2.1:

```sh
make test-e2e-scenarios SCENARIO=ai-gateway/runtime-2-1
```

This adds 14 CLI commands to the live scenario inventory. It does not change
existing replay membership or recordings. Model costs are positive because
the initial `.tech` validation rejected zero cache-read cost as missing;
local integration tests separately verify that zero is serialized. This
scenario validates control-plane state, not data-plane inference.
