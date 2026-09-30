# AI Gateway custom policies

This live scenario exercises a streaming custom-policy definition and a
policy instance that uses it. It requires the custom-policy API and AI Gateway
2.2 support. No attached data plane or external plugin installation is assumed.

Coverage includes:

- Gateway, definition, and instance creation dependencies.
- List and get-by-name read-back of Lua schema, handler, and metadata.
- Definition updates followed by a zero-change plan.
- Dump and reload without changes.
- Omitted child collections preserving existing definitions.
- Rejection of definition deletion while a retained instance uses it.
- Explicit empty collections deleting the instance before its definition.
- Child and gateway deletion read-back.

Read-back assertions use bounded polling. Mutation commands are not retried,
and plan assertions retain exact counts and dependency checks.

The installed variant remains covered by local tests, not this live scenario:
it requires an independently supplied plugin implementation. This scenario
does not validate data-plane execution of the Lua handler.

Run with:

```sh
make test-e2e-scenarios SCENARIO=ai-gateway/custom-policy
```

The scenario uses the default stable classification required by the AI Gateway
scenario suite. Backend availability is still being established: an unavailable
endpoint fails CI rather than being skipped or reported as advisory. This
scenario is not replay-enabled.
