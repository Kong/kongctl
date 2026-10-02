# MCP listener source rename regression

This live scenario covers #2365 using an upstream MCP server and a listener
whose `sources` contain the server's plain name. It asserts that a single
sync creates the renamed source, updates the listener, and deletes the old
source in that order. Command retries are disabled so a second sync cannot
hide the original failure.

The scenario also rejects deleting a source still referenced by a retained
listener, renames the source back by executing a saved plan, verifies the
remote source names and listener attachment, and checks convergence after
both renames. Cleanup deletes both servers together before removing the
scenario's gateway. An initial delete removes only this scenario's gateway
to make local reruns deterministic.

Run with the standard E2E credentials:

```sh
make test-e2e-scenarios SCENARIO=ai-gateway/mcp-source-rename
```

The scenario is automatically discovered by the full E2E suite and runs live
in CI. Existing replay eligibility is unchanged.
