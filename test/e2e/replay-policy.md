# PR scenario replay

`replay-scenarios.json` is the explicit list of replay-enabled scenarios. A
cassette on disk alone does not enable a scenario. Every enabled scenario
must have a reviewed, sanitized live recording beside its inputs, pass all
normal scenario assertions in isolation, and have a current input fingerprint.

The enabled subset is `control-plane/get`, `control-plane/apply`,
`control-plane/plan/apply-workflow`, `control-plane/sync`,
`event-gateway/consume-policy`,
`portal/api_docs_with_children`, `portal/ip-allow-list`, `portal/pages`,
`portal/sync`, `portal/teams`, and `portal/visibility`.

| Scenario | Successful recording and three isolated replays |
| --- | --- |
| get | [34377519108][get] |
| apply | [34427742802][apply] |
| plan/apply-workflow | [34428189915][plan] |
| sync | [Recording][sync], [isolated phases][sync-replay] |
| event-gateway/consume-policy | [Recording][consume-record], [isolated replays][consume-replay] |
| portal/sync | [Recording][portal-record], [isolated replays][portal-replay] |
| portal/visibility | [Recording][visibility-record], [isolated replays][visibility-replay] |
| portal/api_docs_with_children | [Recording][docs-record], [isolated replays][docs-replay] |
| portal/ip-allow-list | [Recording][ip-record], [isolated replays][ip-replay] |
| portal/pages | [Recording and isolated replays][pages-record] |
| portal/teams | [Recording][teams-record], [isolated replays][teams-replay] |

[get]: https://github.com/Kong/kongctl/actions/runs/34377519108
[apply]: https://github.com/Kong/kongctl/actions/runs/34427742802
[plan]: https://github.com/Kong/kongctl/actions/runs/34428189915
[sync]: https://github.com/Kong/kongctl/actions/runs/35249677971
[sync-replay]: https://github.com/Kong/kongctl/actions/runs/35250398004
[consume-record]: https://github.com/Kong/kongctl/actions/runs/35115143565
[consume-replay]: https://github.com/Kong/kongctl/actions/runs/35116004092
[portal-record]: https://github.com/Kong/kongctl/actions/runs/34517553668
[portal-replay]: https://github.com/Kong/kongctl/actions/runs/34521886789
[visibility-record]: https://github.com/Kong/kongctl/actions/runs/34609380360
[visibility-replay]: https://github.com/Kong/kongctl/actions/runs/34610832683
[docs-record]: https://github.com/Kong/kongctl/actions/runs/34860627806
[docs-replay]: https://github.com/Kong/kongctl/actions/runs/34862188973
[ip-record]: https://github.com/Kong/kongctl/actions/runs/35165520273
[ip-replay]: https://github.com/Kong/kongctl/actions/runs/35166388118
[pages-record]: https://github.com/Kong/kongctl/actions/runs/35361530847
[teams-record]: https://github.com/Kong/kongctl/actions/runs/35419035814
[teams-replay]: https://github.com/Kong/kongctl/actions/runs/35419655877

## Routing

Same-repository PRs targeting `.com` run the enabled subset through a local
HTTPS replay proxy. All other scenarios still run against real Konnect.
There is no agent, code-path mapping, model call, or per-PR approval step.

Main's debounced runs, merge queues, manual runs, trusted fork runs and `.tech`
runs stay fully live. Add the `e2e:force-live` PR label to request an all-live
run. Adding/removing the label triggers the existing PR workflow. Maintain
the label with the same permissions as other workflow-control labels.
The required status returns to pending on that label change; an already
running replay cannot overwrite it after force-live has been requested.

`e2e.yaml` builds once and publishes a deterministic `e2e-routing.json` plan.
The live harness validates that the plan partitions the complete inventory
exactly once, removes replay scenarios before weighted allocation, and never
enters their reset or scenario code. The separate replay job uses the same
compiled binaries without Konnect credentials or external networking.

The final verifier independently checks the routing plan, replay job outcome,
recorded/isolated replay results, and exact live manifest/result coverage.
Missing, duplicated, skipped or failed replay scenarios fail `E2E Required`.
A stale cassette or unexpected request fails; there is no automatic live
fallback. No scenarios run in a mixture of live and replay mode.

The environment-wide workflow queue, per-org locks, main debounce timer and
superseded-PR checks are unchanged. Replay does not acquire an org lock, but
the enclosing workflow still holds its environment queue slot.

## Recording and expansion

Use this shared procedure for new replay coverage and cassette refreshes.
The manual `e2e-replay.yaml` workflow uses CI credentials and the existing
acceptance-3 lock and before/after resets; no local PAT or new organization
is needed. After pushing the scenario changes, record the complete scenario
and replay it three times in the separate network-isolated job (replace
both placeholders):

```sh
gh workflow run e2e-replay.yaml --repo Kong/kongctl \
  --ref YOUR_BRANCH -f mode=record -f scenario=SUITE/SCENARIO
```

The candidate is uploaded only after a passing live scenario and final reset.
Monitor the run and download its sanitized `replay-candidate` artifact.
Review the exchanges against the scenario and its assertions. The candidate
remains usable if subsequent isolated replay fails. If concurrent operations
need annotations, follow [ordering validation](#recording-format-and-ordering)
below to iterate with `source_run` without another live recording.

After all three isolated replays pass, promote the candidate (or the
annotated `validated-cassette.json`) under the scenario's
`replay/cassette.json`, together with corresponding annotations and any
packed chunks. For newly enabled scenarios, add the directory to the sorted
policy list. The recorder never commits or overwrites reviewed cassettes
automatically. Verify the committed representation with isolated replay,
run applicable repository checks, and include recording and replay-validation
run links in the PR.

### Refresh existing replay coverage

Use this procedure when modifying an enabled scenario's steps, inputs,
overlays, or assertions/expected files, or when validation reports a stale
cassette. Replay coverage is intentional: completing the scenario change
includes refreshing its recording in the same PR.

1. Check `test/e2e/replay-scenarios.json`, including the PR's base version if
   eligibility has already been edited. Preserve existing membership and
   keep the old cassette while preparing its replacement. Removing coverage
   requires explicit maintainer authorization; a stale recording alone is
   not authorization. Keep assertions and replay validation strict rather
   than removing eligibility or changing routing tests to accept its loss.
2. Follow [Recording and expansion](#recording-and-expansion) above for the
   updated scenario on the working branch.
   The old cassette may remain stale during this step. The manual workflow
   defers only the selected old cassette's input-fingerprint comparison;
   schema, provenance, sanitization, eligibility, and other cassettes remain
   checked. The same exception applies to `source_run` validation. Normal
   PR tests and fresh candidates still require current fingerprints.
3. If scenario inputs change again, record again; manually editing the
   fingerprint is not a refresh.

If recording or validation is blocked, report the blocker while preserving
replay eligibility. Keep the PR incomplete rather than switching it to
live-only routing or weakening matching, sanitization, or fingerprint checks.

### Recording format and ordering

New recordings use cassette schema v2, which also preserves an allowlisted
response media type (`application/json` or `application/problem+json`; absent
is allowed only for an empty body). This is significant: the SDK uses the
problem media type to classify expected 404s as typed not-found errors.
Remaining v1 Control Plane cassettes retain their original JSON response
behavior. Arbitrary response headers, cookies, and authorization are never
recorded. A failed scenario publishes no candidate; diagnostics contain only
local command names and bounded, already-sanitized HTTP error exchanges.

V2 cassettes may explicitly annotate `parallel_phases`: disjoint, one-based
interaction ranges of 2–64 exchanges. Outside those ranges, order stays strict.
Inside a range every method, path, query, and body must still match exactly,
once. Generated UUID references depend on their recorded creation. Repeated
targets keep their stream order, except distinct successful creates with
distinct generated IDs. In a wholly read-only phase, bodyless GETs with
different queries may also reorder (for example, publications filtered by
different API IDs). Queries still match exactly; repeated identical requests
retain their stream order. Ancestor reads cannot cross updates or deletes.
Additional `after` dependencies can constrain ordering but cannot remove these
mandatory dependencies. Never annotate a whole scenario as unordered.

An exact-match request within the current parallel phase may arrive before
its prerequisites finish. The proxy reserves that exchange and waits up to
five seconds, releasing its lock so other requests can satisfy the dependency.
It never consumes the response early or searches a later phase. Missing
dependencies time out with recorded interaction numbers; unexpected requests
and duplicate consumption still fail. Proxy failure or shutdown wakes waiters.
This accommodates concurrent CLI scheduling without relaxing matching or
changing the cassette's dependency rules.

The Portal candidate annotations separate initial inventory, initial creation,
individual read-only planning/dump commands, and final cleanup. Publication
removal and API deletion remain barriers; reads from later scenario states
cannot satisfy earlier requests. An annotation is a reviewed assertion about
independent operations, not a general simulation of Konnect state.

Control Plane sync annotates only exchanges 10–12: creating the new control
plane is independent of looking up/deleting the old one, but the old lookup
must precede its deletion. All inventory and final-cleanup requests retain
strict order. Its refreshed v2 cassette covers UPDATE and idempotency after
CREATE and UPDATE, and passed three isolated replays of all 16 exchanges.
The [scenario replay review](scenarios/control-plane/sync/replay/README.md)
documents the command boundaries and recording provenance.

To iterate on an existing candidate without resetting a live organization:

```sh
gh workflow run e2e-replay.yaml --repo Kong/kongctl \
  --ref YOUR_REVIEWED_BRANCH -f mode=replay -f scenario=portal/sync \
  -f source_run=34517553668
```

Only candidate JSON is downloaded from the source run; executables are built
from the selected branch. Optional `replay/parallel-phases.json` annotations
are bound to the exact source cassette SHA-256. A different recording requires
new review, not reuse of old positional assumptions. All three isolated
replays must pass before promoting `validated-cassette.json` from the result
artifact. Recording never applies annotations from another run.

Large validated v2 cassettes can be stored as ordered JSON chunks, each at
most 400,000 bytes, below the repository's per-file limit:

```sh
python3 scripts/e2e_replay.py pack --scenario portal/sync \
  --cassette PATH_TO_VALIDATED_CASSETTE --output-dir NEW_DIRECTORY
```

The new directory contains `cassette.json` and `interactions/*.json`. Promote
both together. Chunk names are SHA-256 digests checked on every load; chunks
are local, bounded, and cannot be symlinks. Packing verifies an exact round
trip: all recorded requests, responses, ordering and provenance are unchanged.
It is only a storage format, not a sanitizer or approval mechanism. Run the
packed cassette through isolated replay as part of PR validation too.

Scenario-local overlays and workdir-local generated plan files are supported.
Inline `inputOverlayOps` may set literal boolean/plain-string fields in an
existing YAML file at the scenario's `testdata` root. Selectors must be literal
`ref` filters (optionally nested), ending in `| [0]`. The unchanged Go harness
performs the edits; replay validation does not implement another overlay
engine. Operations, targets and assertions remain input-fingerprinted.
Templates, nested replacement values, external ops files, YAML aliases and
duplicate keys require separate support and remain rejected.
Only the fixed `KONGCTL_LOG_LEVEL: info` scenario environment block is allowed.
Scenario control checks use parsed YAML: fields inside command assertion
`expect.fields` are data, so a header named `env` is not an environment
override. Actual scenario/step/command overrides remain rejected, including
flow-style declarations. Duplicate keys, YAML aliases and custom tags are
rejected before eligibility is evaluated.

Only a standalone `resetOrg: true` as the first command of the first step is
supported. Recording uses the existing locked before/after reset; replay
begins with an empty recorded state. A later reset would be skipped by the
current replay environment and could invalidate a stateful round-trip test,
so it fails eligibility rather than silently losing that boundary.

Plain scalar `!file` references may resolve to existing files inside scenario
`testdata`, or the referencing file's own overlay copied onto that tree.
Overlay-only document/spec files are fingerprinted and receive the same
exact-content public-fixture checks; another overlay is not a fallback search
path. Remote files, parent traversal, symlinks, arbitrary environment overrides
and custom creation commands remain unsupported. Every input, overlay and
assertion file is fingerprinted, including document and OpenAPI content.

Public document/spec strings containing example credentials or email addresses
are preserved only when they match a fingerprinted input file: exact text for
documents, or the complete parsed OpenAPI object when kongctl serializes a YAML
spec as JSON. This does not exempt substrings, changed fields, sensitive JSON
field names outside an opaque public spec, or the recording credential itself.
Literal URLs in these payloads are data, not permission to fetch them; replay
still has loopback-only networking.

The secret-scan baseline lists only reviewed integrity hashes and the exact
existing public example value repeated in the Portal fixture. It does not
exclude cassette directories or weaken the recorder's sensitive-data checks.

`make setup-e2e-replay` installs the pinned test-only YAML parser into ignored
`.e2e-artifacts/replay-python`. Build/check/metrics Make targets include this
setup; CI performs it before network isolation. No kongctl dependency changes.
Dependency installation belongs to job setup, not scenario execution savings.

`event-gateway/consume-policy` passed its expanded live scenario and three
isolated replays, each matching all 211 exchanges. Two bounded phases cover
independent schema-validation-parent/static-key creation and cleanup.
The decrypt_fields child lifecycle and all assertions remain intact. Replay
scenario execution took 6.855–6.971 seconds; wrappers took 7.565–7.744 seconds.
Its scenario-local replay README records provenance, phase boundaries and
measurement limitations. The cassette retains the live source provenance;
the isolated job's validated artifact adds only the reviewed annotations.

`portal/sync` is enabled after its complete live scenario and three isolated
replays passed (293 HTTP interactions each), followed by three isolated
replays of the packed cassette. No scenario assertions were
removed to obtain a successful recording. The isolated wrappers took
3.68–4.26 seconds across both validations, compared with 18.82 seconds for the
same-source normal live scenario and a 22.72-second historical median.
Recording's own 47.37-second
scenario duration includes proxy overhead and is not the live baseline.

Group scenarios have exhibited nondeterministic independent request ordering
and stay live pending reviewed phase annotations and isolated validation.
Serverless additionally uses a harness creation command. The
certificate scenario needs a reviewed public-PEM and environment-input policy.
Product maturity alone is not sufficient to enable these scenarios.

`portal/visibility` is enabled after its complete live recording and three
isolated replays passed (390 HTTP interactions each). All visibility updates,
no-op plans, dumps and cleanup assertions remain intact. The isolated wrappers
took 4.81–4.96 seconds, compared with a 38.70-second historical live median
(20 observations). This is not a same-source comparison or a measured workflow
speedup. Recording's 50.07-second scenario includes proxy overhead and is not
the live baseline. Its scenario-local replay README documents the reviewed
phase boundaries, including the strict private/public visibility barriers.

`portal/api_docs_with_children` is enabled after its complete live recording
and three isolated replays passed (268 HTTP interactions each). The scenario
still asserts document hierarchy, content/status updates, child and parent
deletion, and dump round-trip behavior. Its replay README records the reviewed
command boundaries. Replay wrappers took 3.90–4.56 seconds; promotion avoids
one additional scenario reset on PRs. Main continues to run it live.

`portal/pages` is enabled after live recording and three strict, isolated
replays of all 71 exchanges. Page hierarchy, metadata and content updates,
no-op and malformed-frontmatter plans, and leaf deletion assertions remain
intact. No ordering annotations or matcher changes were needed. Its
[replay review](scenarios/portal/pages/replay/README.md) records provenance
and the timing comparison, including the avoided scenario reset.

`portal/teams` is enabled after live recording and three isolated replays
of all 163 exchanges. Team creation, updates, sync deletion, role assignment
and removal, no-op plans, and dump round-trip assertions remain intact.
Only small groups of independent team operations may reorder; mutation
and command boundaries stay strict. Its
[replay review](scenarios/portal/teams/replay/README.md) documents those
groups, provenance, and timing. PR replay avoids one more scenario reset;
main continues to run the complete scenario live.

## Measurement

The routing artifact records exact live/replay membership. Replay artifacts
contain per-scenario execution and wrapper durations, interaction counts and
an execution-bound result report. They use separate artifact names, not live
`e2e-metrics-*` artifacts. Reduced live runs receive an allocation ID suffix
`:pr-replay:<membership hash>` so they cannot match the old full-live cohort.

Count avoided scenario resets as well as API execution time. Record-mode
before/after resets are useful feasibility evidence, not a direct estimate of
normal shard savings. Shared shard setup/reset remains while any live work
uses that shard. Measure the longest shard and total workflow duration,
including replay startup/download overhead, before claiming a net speedup.

The PR replay job summary compares each scenario's wrapper duration with its
historical live median and sample count from
`baselines/weighted-v1-2026-09-observations.json`. Missing history is shown as
`n/a`, not zero. This frozen September 6–9 baseline includes normal scenario
resets. The percentages describe execution work, not a causal estimate of PR
completion time. Keep collecting reduced-live allocation cohorts separately
to evaluate the longest remaining shard and queue/admission delays.

Use `make collect-e2e-progress E2E_PROGRESS_MODE=pr` for an ongoing snapshot
of the current reduced-live allocation. Its cumulative observations continue
growing after the initial 20-run target; its report shows the latest window.
The default `live` mode selects the full-live allocation separately. See the
[harness README](README.md#ongoing-progress-after-a-baseline-is-complete) for
retention and cache-category reporting.
