# Overnight main validation

`E2E / Main Scheduler` (`e2e-main-debounce.yaml`, retained for existing links)
opens one full-live validation opportunity at **07:07 UTC every day**. This
is 01:07 US Central standard time or 02:07 daylight time. GitHub schedules
can be delayed. Pushes no longer start timers or dispatch scenario runs.

Every 15 minutes (`*/15 * * * *`, UTC), the scheduler reconciles results and
rechecks a deferred opportunity, including during the day. An opportunity
remains pending until it can dispatch. It chooses the latest default-branch
SHA immediately before reservation and coalesces intervening merges. No run
is needed if that revision has already passed full-live validation, or if
the accumulated diff since the last success contains no E2E-relevant files.
The relevant-path rules are shared with PR readiness. Large/truncated or
divergent comparisons require validation. The first opportunity establishes
a baseline even if no previous result is available.

The scheduler uses the existing configured environment and organization
pool. It yields to active or queued scenario workflows, including trusted
fork, manual and merge-queue runs. It also consults every open same-repo PR
using the readiness controller: review-ready PRs waiting on prerequisite CI,
and ready PRs without terminal validation evidence take priority. Drafts,
unresolved reviews, completed failures and already validated PRs do not
reserve future capacity. Forks still require explicit trusted execution.
The check conservatively yields across both environment queues.

The scenario workflow rechecks PR priority and the latest main SHA after
acquiring its environment queue slot. If a PR arrived or a merge superseded
the selected SHA, it skips expensive work and returns the opportunity for
the next scheduler evaluation. Once admitted, the exact selected SHA is
used for builds and scenarios. A new merge during active validation does
not cancel the run; it remains eligible for the next overnight opportunity.
The existing environment concurrency queues and organization locks keep
active live use exclusive. PRs arriving after live work starts must wait
for that run to finish; active environments are never preempted.

## Run now and recover

A human with repository Maintain or Admin access can request pending main
validation immediately:

```sh
gh workflow run e2e-main-debounce.yaml --repo Kong/kongctl --ref main
```

Run-now respects PR priority, existing reservations and already validated
revisions. A deferred manual request remains eligible for 15-minute retries.
Validation failures, cancellations and timeouts remain visible and do not
retry continuously: use run-now or wait for the next nightly opportunity.
For main, use run-now for a fresh suite after failure; failed-job reruns are
only admitted while the original reservation is still current.

Reservations are written before dispatch. If dispatch fails ambiguously,
the scheduler does not duplicate it. Inspect Actions for an existing
`main · scenarios · overnight` run first. If no matching run exists after
ten minutes, a maintainer can recover:

```sh
gh workflow run e2e-main-debounce.yaml --repo Kong/kongctl --ref main \
  -f recover_reservation=true
```

Recovery cannot clear a visible active reservation. API errors fail closed;
the next scheduled reconciliation remains available.

## State and visibility

The dedicated data-only branch `automation/e2e-main-state` holds
`state.json`. Only the serialized scheduler writes it, using its
`contents: write` token; scenario runs only read it. Do not delete or edit
this branch during normal operation. Branch rules must permit its workflow
token to create/update it. No PAT, new organization or repository variable
is required, and state does not expire with Actions artifacts.

State retains the dispatch ticket and exact SHA, pending opportunity,
pending-since timestamp, latest outcome/run link, last failure, and last
successful full-live SHA/time. A successful workflow with skipped or missing coverage
verification never advances the baseline. Failures remain linked after
subsequent deferrals. Successful validation of an older SHA cannot mark a
newer SHA as validated.

Each scheduler summary includes the schedule, retry cadence, decision,
deferral reasons, reservation, last result, last success and validation age.
Warnings surface a missing baseline, success older than 36 hours, pending
work older than 24 hours, and the latest failed/cancelled validation. These
warnings appear in Actions; they do not create issues or send messages.

## Verification and rollout measurements

`make test-e2e-readiness` exercises merge batches, idle/busy nights,
prerequisite CI, late PRs, newer merges, manual requests, ambiguous dispatch,
recovery, failed validation and successful SHA accounting against API fakes.
`make test-e2e-metrics` also tests queue overlap calculation. Neither changes
scenarios nor removes replay coverage.

After deployment, observe at least one merge batch and a busy overnight
window. Record the scheduler run links and compare an equal observation
window with the previous debounced runs:

```sh
python3 scripts/e2e_main_metrics.py --repo Kong/kongctl --since 2026-11-20
```

The read-only report lists seconds each PR spent queued while a main run
held its workflow slot, affected run links by ID, and current full-live
validation age. It includes both historical debounced and new overnight
titles. Queue wait is creation to the first admission job start; main use
is admission to workflow completion. This measures contention overlap,
not proof that main caused every queued second. Reruns and missing admission
jobs are excluded; use a window below GitHub's 1,000-run search limit.
Initial before/after production measurements require the scheduler to be
merged and to have observed traffic; local tests cannot establish them.
