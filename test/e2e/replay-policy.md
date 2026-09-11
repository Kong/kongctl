# PR scenario replay

`replay-scenarios.json` is the explicit list of replay-enabled scenarios. A
cassette on disk alone does not enable a scenario. Every enabled scenario
must have a reviewed, sanitized live recording beside its inputs, pass all
normal scenario assertions in isolation, and have a current input fingerprint.

The enabled subset is `control-plane/get`, `control-plane/apply`,
`control-plane/plan/apply-workflow`, `control-plane/sync`, and `portal/sync`.

| Scenario | Successful recording and three isolated replays |
| --- | --- |
| get | [34377519108][get] |
| apply | [34427742802][apply] |
| plan/apply-workflow | [34428189915][plan] |
| sync | [Recording][sync], [isolated phases][sync-replay] |
| portal/sync | [Recording][portal-record], [isolated replays][portal-replay] |

[get]: https://github.com/Kong/kongctl/actions/runs/34377519108
[apply]: https://github.com/Kong/kongctl/actions/runs/34427742802
[plan]: https://github.com/Kong/kongctl/actions/runs/34428189915
[sync]: https://github.com/Kong/kongctl/actions/runs/34520312018
[sync-replay]: https://github.com/Kong/kongctl/actions/runs/34521883600
[portal-record]: https://github.com/Kong/kongctl/actions/runs/34517553668
[portal-replay]: https://github.com/Kong/kongctl/actions/runs/34521886789

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

The manual `e2e-replay.yaml` workflow supports a `scenario` choice. Record one
complete scenario under the existing acceptance-3 lock, then replay it three
times in the workflow's separate network-isolated job:

```sh
gh workflow run e2e-replay.yaml --repo Kong/kongctl \
  --ref YOUR_REVIEWED_BRANCH -f mode=record -f scenario=control-plane/apply
```

The candidate is uploaded only after a passing live scenario and final reset.
Download `replay-candidate` from the run, review it, and promote it only after
the isolated replay job passes. Commit it under the scenario's
`replay/cassette.json` and add its directory to the sorted policy list. The
recorder never commits or overwrites reviewed cassettes automatically.

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

The Portal candidate annotations separate initial inventory, initial creation,
individual read-only planning/dump commands, and final cleanup. Publication
removal and API deletion remain barriers; reads from later scenario states
cannot satisfy earlier requests. An annotation is a reviewed assertion about
independent operations, not a general simulation of Konnect state.

Control Plane sync annotates only exchanges 4–6: creating the new control
plane is independent of looking up/deleting the old one, but the old lookup
must precede its deletion. All inventory and final-cleanup requests retain
strict order. Its v2 cassette passed three isolated replays; the original
strict-order flake was also reproduced using the unchanged main replay engine.

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
Plain scalar `!file` references may resolve to existing files inside scenario
`testdata`, including references from overlays copied onto that tree. Remote
files, parent traversal, symlinks, arbitrary environment overrides and custom
creation commands remain unsupported. Every input/overlay/assertion file is
fingerprinted, including document and OpenAPI content.

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

`portal/visibility` is supported for manual recording but remains live on PRs
until its complete recording and three isolated replays pass. Visibility
updates, no-op plans, dumps and cleanup assertions must all remain intact.

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
