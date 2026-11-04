# Portal teams replay

## Apply child-scope refresh

The [fresh live recording][refresh-record] passed all scenario assertions
and before/after resets under the existing acceptance-3 organization lock.
It was captured from commit `155dfd85` for issue #2706.
[Three network-isolated replays][refresh-replay] each consumed all 107
recorded exchanges and passed every assertion.

Scenario inputs, overlays, assertions, and replay eligibility are unchanged.
Apply planning now skips children absent from the manifest, so the recording
omits their former comparison reads. Declared-child reads and mutations
retain their assertions, and command/reset boundaries stay intact.

The annotations were reviewed against the new recording and rebound to its
exact raw-candidate SHA-256. These ranges retain the previous command scopes;
all other exchanges remain strictly ordered. Mandatory creation, repeated
request, and read-before-write dependencies remain enforced.

| Previous range | Refreshed range |
| --- | --- |
| 3-5 | 3-5 |
| 73-75 | 39-41 |
| 78-80 | 43-45 |
| 93-95 | 52-54 |
| 103-105 | 61-63 |
| 133-135 | 85-87 |
| 142-144 | 93-95 |
| 161-162 | 106-107 |

The committed cassette preserves the fresh recording's requests, responses,
media types, provenance, and input fingerprint. Large recordings are packed
losslessly into hash-addressed chunks. Matcher and sanitizer rules are
unchanged. The current validation above covers the committed representation.

## Previous recording (superseded)

The [live recording][record] captured 163 HTTP exchanges under the existing
acceptance-3 organization lock, with successful before/after resets. Three
[network-isolated replays][replay] passed using the reviewed annotations.
The only scenario edits remove ten command-local HTTP debug-dump environment
blocks. Inputs, overlays, commands, and assertions are otherwise unchanged.

### Coverage and ordering

Coverage includes nested and root-level teams, lookup by name, description
updates, sync deletion, API role assignment and removal, no-op plans, and
the declarative dump round trip.

The first strict replay failed at exchange 3 because independent team
creations execute concurrently. The annotation in `parallel-phases.json`
is bound to the exact candidate SHA-256. It permits reordering only within
these small groups of independent operations:

| Exchanges | Operation |
| --- | --- |
| 3-5 | Create mobile, backend, and frontend teams |
| 73-75 | Read each team's roles during the role-assignment plan |
| 78-80 | Assign one API role to each of the three teams |
| 93-95 | Read team roles during the idempotent plan |
| 103-105 | Read team roles for the dump |
| 133-135 | Read team roles during the plan from the dump |
| 142-144 | Read team roles during the removal plan |
| 161-162 | Read the remaining desired roles in the final no-op plan |

Role planning iterates a Go map keyed by team, so independent reads can
arrive in different orders even without concurrent HTTP calls. No phase
crosses a command or mixes reads with writes. All other exchanges remain
strictly ordered. Portal creation (2) precedes team creation; API creation
(77) precedes role assignments. The backend update (31), mobile deletion
(48), QA team creation (61), and QA role removal (148) remain strict
barriers before their verification commands.

Request methods, paths, queries, bodies, response content types, and all
163 required exchanges remain checked by the existing matcher. No matcher
or sanitizer changes are needed. The dump includes public default logo
and favicon payloads, identical to the existing visibility cassette.
The validated cassette is losslessly packed into hash-addressed chunks to
keep every file below the repository's size limit. Packing does not change
interactions or their order.

### Measurements

The three loopback-only runs passed all scenario assertions and consumed
all 163 exchanges. Scenario durations were 3.405, 3.530, and 3.321 seconds;
wrapper durations were 4.104, 4.124, and 4.018 seconds. Ten consecutive
local replays also passed before isolated validation.

The frozen weighted-v1 baseline contains 20 successful live observations
with a 20.340-second median. Relative to that historical median, replay
saves about 16.2 seconds of execution work per eligible PR. This is not a
same-source benchmark or a measured workflow speedup. Live scenario time
already includes its reset, so do not count that avoided reset twice.
Shared shard resets, job setup, replay startup, and other shards remain.

Recording's 25.722-second scenario duration includes proxy overhead and
is not the normal live baseline.

Main and force-live runs remain live. Eligible PR replay skips the scenario
reset entirely; it does not replay reset transactions. Recording's final
reset removes remaining resources outside the cassette.

Input or assertion changes require a new reviewed recording in the same
PR; follow the repository replay refresh procedure.

[record]: https://github.com/Kong/kongctl/actions/runs/35419035814
[replay]: https://github.com/Kong/kongctl/actions/runs/35419655877

[refresh-record]: https://github.com/Kong/kongctl/actions/runs/41902501492
[refresh-replay]: https://github.com/Kong/kongctl/actions/runs/41906270080
