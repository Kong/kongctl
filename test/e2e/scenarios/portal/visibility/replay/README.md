# Portal visibility replay

## Apply child-scope refresh

The [fresh live recording][refresh-record] passed all scenario assertions
and before/after resets under the existing acceptance-3 organization lock.
It was captured from commit `155dfd85` for issue #2706.
[Three network-isolated replays][refresh-replay] each consumed all 348
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
| 1-7 | 1-7 |
| 8-40 | 8-40 |
| 43-83 | 43-77 |
| 84-124 | 78-112 |
| 128-168 | 116-150 |
| 169-209 | 151-185 |
| 216-256 | 192-226 |
| 263-303 | 233-267 |
| 304-314 | 268-278 |
| 315-327 | 279-291 |
| 328-355 | 292-319 |
| 356-381 | 320-339 |
| 382-384 | 340-342 |
| 385-390 | 343-348 |

The committed cassette preserves the fresh recording's requests, responses,
media types, provenance, and input fingerprint. Large recordings are packed
losslessly into hash-addressed chunks. Matcher and sanitizer rules are
unchanged. The current validation above covers the committed representation.

## Previous recording (superseded)

The expanded scenario was recorded against live Konnect in
[run 39667015919][record] from commit `71fd9e5a`. It passed all assertions and
both recording resets, capturing 390 exchanges. Coverage includes explicit
SIPR false at creation, authentication/RBAC/SIPR false-to-true updates,
readback, no-op planning and preservation through dump/plan round-trip.

Three [network-isolated CI replays][replay] passed all 390 exchanges each.
The packed cassette expands to the identical validated CI output and retains
the current input fingerprint. Ordering review changes only scheduling.

The annotation in `parallel-phases.json` is bound to the exact SHA-256 of that
candidate. It permits independent operations inside these one-based ranges;
every request still matches exactly once, with the shared matcher's mandatory
creation-ID and read/mutation dependencies. Never reuse these positions for
a different recording without reviewing the new command boundaries.

| Exchanges | Scenario operation |
| --- | --- |
| 1–7 | Initial empty inventory |
| 8–40 | Initial creation of independent roots and their children |
| 43–83 | No-op apply after creation |
| 84–124 | Plan the Portal settings update |
| 128–168 | No-op plan after the Portal update |
| 169–209 | Plan private publication visibility |
| 216–256 | Plan restoring public publication visibility |
| 263–303 | No-op plan after restoring public visibility |
| 304–314 | Dump the API and its children |
| 315–327 | Plan from the API dump |
| 328–355 | Dump the Portal and its children |
| 356–381 | Plan from the Portal dump |
| 382–384 | Cleanup inventory |
| 385–390 | Independent cleanup lookups/deletions |

All other exchanges stay strictly ordered. In particular, the Portal PATCH
at 126 includes all three security booleans. It and the publication PUTs
at 213 (private) and 260 (public) are barriers,
as are their immediately preceding lookups and following assertion reads.
Cleanup lookups must precede their corresponding deletions. No response from
a later visibility state can cross the intervening mutation.

The live recording passed, but strict replay failed at interaction 9 when
independent root creation arrived in a different order. Exact request
comparison confirmed matching contents. The reviewed phase annotations
address scheduling without changing requests, responses or assertions.

Each planning phase also has two publication-list GETs filtered by different
API IDs. Their order varies independently of the returned contents. The
read-only phase permits this query-specific reordering, while repeated
identical requests and all mutating phases retain their stream dependencies.

[record]: https://github.com/Kong/kongctl/actions/runs/39667015919
[replay]: https://github.com/Kong/kongctl/actions/runs/39669883194

[refresh-record]: https://github.com/Kong/kongctl/actions/runs/41902506032
[refresh-replay]: https://github.com/Kong/kongctl/actions/runs/41906275369
