# Policy consumer rename regressions

These independent scenarios extend #2308 to agents, consumers, consumer
groups, and models. Each creates a resource with an attached policy, verifies
the attachment remotely, changes both names and refs, and requires one sync
to complete the rename. Remote checks require exactly one resource and policy
with the new names and the intended attachment. A subsequent plan must be a
no-op. Retries are disabled; cleanup follows the assertions.

## Pre-fix live reproduction

Production code was unchanged from
`a3e431cba39a603bb7235f3dbccd3bbb7629f645`; HEAD was the MCP reproduction
commit `7da01c11`. Run date: September 24, 2026. Environment: Konnect `.com`,
Go 1.26.8, Linux amd64, locally built `kongctl/dev`, CGO disabled.
The additional test-only scenarios were committed as `77aaf922`.

```sh
CGO_ENABLED=0 KONGCTL_E2E_BIN="$PWD/kongctl" \
  make test-e2e-scenarios SCENARIO=ai-gateway/policy-rename
```

All four scenarios passed setup and remote attachment assertions. Each
failed at `rename/single-sync`, with one applied change, one failed change,
and two skipped changes. The persisted order and dependency chain were:

1. Create `pass-list-policy`.
2. Delete `pass-listener-policy`, depending on step 1.
3. Delete the old policy consumer, depending on step 2.
4. Create the renamed policy consumer, depending on steps 1 and 3.

Model policy deletion returned HTTP 400 with:

```text
policy.id: constraint failed (type: foreign) as another entity references this value
```

Agent, consumer, and consumer-group policy deletion returned HTTP 400 with
an empty field/reason (`Bad Request: : `). To establish causality, a separate
diagnostic cleanup repeated each old policy DELETE while attached (400),
deleted only its referencing resource (204), then repeated the identical
policy DELETE (204). This diagnostic was performed after the failed tests;
it is not part of the rename scenario or a passing rename result.

Artifacts: `/home/rspurgeon/go/e2e-artifacts/20260924-102838`.
Each scenario's `steps/rename/commands/single-sync/stdout.txt` contains the
persisted plan, dependencies, and execution result. Successful setup checks
are under `steps/create/commands`. `policy-delete-diagnostic.json` records
the three controlled deletion checks.

No production fix was introduced before these live red tests. Existing
lifecycle scenarios and replay eligibility remain unchanged.

## Post-fix live validation

Agents, consumers, and consumer groups passed completely in
`/home/rspurgeon/go/e2e-artifacts/20260924-103522`. The model rename, remote
attachment checks, no-op plan, and child deletion also passed in that run;
its final cleanup assertion incorrectly counted gateway cascade deletion
as two changes. The model scenario now explicitly deletes and verifies the
remaining provider before deleting the gateway. Rename assertions and the
single-sync transition are unchanged.

The complete model scenario passed in
`/home/rspurgeon/go/e2e-artifacts/20260924-103611`.
Every rename applied all four changes with zero failures or skips; remote
checks verified the old resources were absent, the new resources existed
with the intended attachment, and a subsequent sync plan had zero changes.

## Full-detach review regressions

All four scenarios now additionally detach the surviving resource and delete
its policy in one sync, check remote detachment and policy absence, and require
a no-op plan. A later reattachment preserves combined child-deletion coverage.

Before the review fix, all four passed the rename then failed planning at
`detach-and-delete-policy/single-sync` with `still references it`. Production
revision: `2784a312`. Red artifacts:
`/home/rspurgeon/go/e2e-artifacts/20260924-120157`.

All four complete scenarios passed after the fix, including the detach sync,
remote checks, convergence, and cleanup. Green artifacts:
`/home/rspurgeon/go/e2e-artifacts/20260924-120608`.
