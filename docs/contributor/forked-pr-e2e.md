# Forked PR E2E Validation

Fork pull requests cannot receive secret-backed E2E validation automatically.
When a fork PR changes files that require E2E, use this maintainer workflow
after the PR is out of draft and review comments are resolved.

## Standard Workflow

1. Review the PR for malicious or risky changes.

   Pay special attention to build scripts, tests, dependency updates,
   workflow-adjacent files, shell execution, and network calls.

2. Approve the pending fork workflows in the PR.

   This lets the normal unprivileged `Checks` and `CI Test` workflows run. The
   readiness controller never automatically dispatches secret-backed E2E
   for forks. Human readiness approval does not grant secrets to fork code.

3. Confirm the exact PR head SHA that was reviewed.

   ```shell
   gh pr view <pr-number> \
     --repo Kong/kongctl \
     --json headRefOid \
     --jq .headRefOid
   ```

4. Run the trusted `E2E / Scenario Suite` workflow from the base repository.

   ```shell
   gh workflow run e2e.yaml \
     --repo Kong/kongctl \
     --ref main \
     -f trusted_pr_number=<pr-number> \
     -f trusted_head_sha=<full-sha> \
     -f konnect_environment=<com-or-tech>
   ```

   Choose `tech` for `konnect-env:tech` PRs; otherwise use the configured
   `KONGCTL_E2E_KONNECT_ENV` target, defaulting to `com`.

5. Wait for the trusted `E2E / Scenario Suite` workflow to finish.

   The trusted run publishes keyed validation evidence on the reviewed SHA
   and updates the trusted E2E PR comment. The readiness controller then
   updates `E2E Required` for the current revision and environment.

6. Re-run trusted E2E if the contributor pushes another commit.

   Trusted E2E applies only to the exact reviewed SHA used in
   `trusted_head_sha`.

## Reading the PR Status

`E2E Required` is the merge gate for E2E. It is a commit status, not a
workflow job name.

For fork PRs, the PR may still show ordinary CI workflows waiting for
approval. Those are not the trusted E2E result. The canonical fork E2E result is
the trusted `workflow_dispatch` run from `main`, reflected by:

- the `E2E Required` commit status on the reviewed SHA
- the trusted E2E PR comment
- the linked trusted E2E workflow run

GitHub displays the workflow name as a checks-screen section, each job name as
a row in that section, and each step name inside its opened job.
`E2E Required` is separate: it is the protected commit-status context used as
the merge gate. Multiple PR events can still create multiple workflow
sections; the distinct names clarify those runs without changing trigger or
deduplication behavior.

If `E2E Required` becomes pending again after a trusted run, check whether the
PR head SHA or target environment changed. Incidental events preserve valid
evidence. Re-run trusted E2E against the current reviewed SHA and environment
when those requirements change. For `.tech`, include
`-f konnect_environment=tech` in the trusted dispatch command.

## Access Control

The trusted E2E command is safe to show in the PR comment. It does not grant
permission by itself. GitHub requires sufficient write access to
`Kong/kongctl` to manually dispatch the upstream workflow.

The workflow also verifies that `trusted_pr_number` is a fork PR and that
`trusted_head_sha` still matches the current PR head before checking out or
running the fork code. This protects against stale or mismatched inputs, but it
does not replace maintainer review.
