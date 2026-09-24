# MCP server and policy rename regression

This scenario covers #2308 using a real `http-log` policy attached to a
passthrough MCP listener. It changes both names and refs, performs one sync,
checks remote resource names and the policy attachment, and requires a no-op
sync plan. Command retries are disabled so a corrective second sync cannot
hide the failure. Child deletion and gateway cleanup follow the transition.
The existing MCP lifecycle scenario remains unchanged.

## Pre-fix live reproduction

Production revision: `a3e431cba39a603bb7235f3dbccd3bbb7629f645`.
Only this new scenario was added before running the reproduction.
Test-only revision: `7da01c11`.
Environment: Konnect `.com`, Go 1.26.8, Linux amd64, locally built
`kongctl/dev`, CGO disabled. Run date: September 24, 2026.

```sh
make build
CGO_ENABLED=0 KONGCTL_E2E_BIN="$PWD/kongctl" \
  make test-e2e-scenarios SCENARIO=ai-gateway/mcp-policy-rename
```

Use an absolute binary path: Go runs the tests from the E2E package directory.

Setup created all three resources and both remote assertions passed,
including `policies: [pass-listener-policy]` on `pass-listener`.
The read-only rename plan contained two creates and two deletes. The single
rename sync failed with one applied change, one failure, and two skipped
changes. Its execution order and persisted dependencies were:

1. Create `pass-list-policy`.
2. Delete `pass-listener-policy`, depending on step 1.
3. Delete `pass-listener`, depending on step 2.
4. Create `pass-list`, depending on steps 1 and 3.

The policy delete returned HTTP 400:

```text
policy.id: constraint failed (type: foreign) as another entity references this value
```

Local artifacts:
`/home/rspurgeon/go/e2e-artifacts/20260924-094744`.
Under `tests/Test_Scenarios_scenarios_ai-gateway_mcp-policy-rename_scenario.yaml`,
the `steps/create/commands` artifacts contain the successful setup checks;
`steps/rename/commands/capture-plan` contains the plan;
`steps/rename/commands/single-sync` contains the failed execution and plan.

The expected success assertion was not weakened to accept this error.
No production code was modified for this reproduction.

## Post-fix live validation

The same scenario passed on September 24, 2026 with the dependency fix.
Artifacts: `/home/rspurgeon/go/e2e-artifacts/20260924-103516`.
One rename sync applied all four changes with zero failures or skips.
Remote checks confirmed only `pass-list` and `pass-list-policy` remained,
with the expected attachment; the following sync plan had zero changes.
Child deletion and gateway cleanup also passed.

The unchanged `ai-gateway/mcp-server` lifecycle scenario passed separately:
`/home/rspurgeon/go/e2e-artifacts/20260924-103759`.
