# Portal API documents replay

## Apply child-scope refresh

The [fresh live recording][refresh-record] passed all scenario assertions
and before/after resets under the existing acceptance-3 organization lock.
It was captured from commit `155dfd85` for issue #2706.
[Three network-isolated replays][refresh-replay] each consumed all 256
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
| 125-132 | 113-120 |
| 138-175 | 126-163 |
| 182-218 | 170-206 |
| 225-242 | 213-230 |
| 243-259 | 231-247 |
| 260-262 | 248-250 |
| 263-268 | 251-256 |

The committed cassette preserves the fresh recording's requests, responses,
media types, provenance, and input fingerprint. Large recordings are packed
losslessly into hash-addressed chunks. Matcher and sanitizer rules are
unchanged. The current validation above covers the committed representation.

## Previous recording (superseded)

The unchanged scenario passed live Konnect recording in
[run 34860627806][record], including both recording resets and all document
hierarchy, content, publication status, deletion and dump round-trip assertions.
The candidate contains 268 exchanges. Its 84.382-second scenario duration
includes recording proxy overhead and is not the normal live baseline.

[Three loopback-only CI replays][replay] passed all 268 exchanges each.
Wrapper durations were 4.152, 4.560 and 3.897 seconds; scenario durations were
3.336, 3.904 and 3.317 seconds. The latest six eligible reduced-live PR runs
before promotion had a 30.23-second live median for this scenario. This is
an observational execution comparison, not a same-source or workflow speedup.
The packed cassette preserves the validated exchanges and provenance exactly.

The annotation in `parallel-phases.json` is bound to that exact candidate's
SHA-256. It permits independent operations only within individual command
phases, using the existing matcher's mandatory creation-ID, repeated-target
and read/mutation dependencies. Every request still matches exactly once.

| Exchanges | Scenario operation |
| --- | --- |
| 1–7 | Initial empty inventory |
| 8–40 | Initial independent roots and child creation |
| 43–83 | No-op plan after creation |
| 84–124 | Plan document content and publication status updates |
| 125–132 | Apply the two independent document updates |
| 138–175 | Plan child document deletion |
| 182–218 | Plan parent document deletion |
| 225–242 | Dump APIs and children |
| 243–259 | No-op plan from the dump |
| 260–262 | Cleanup inventory |
| 263–268 | Independent cleanup lookups and deletions |

All other exchanges remain strictly ordered. Update verification at 133–137
cannot cross the PATCHes at 131–132. The lookup and DELETE sequences at
176–179 and 219–222 remain strict, followed by their assertion reads.
No document response from a later state may satisfy an earlier command.
Within the update phase, mandatory dependencies retain ancestor inventory
reads and each document's GET before its PATCH.

The first strict replay stopped at exchange 11 during concurrent creation.
The annotations address independent request scheduling without relaxing exact
request matching or changing the scenario. They must not be reused for a
different recording without reviewing its command boundaries.

[record]: https://github.com/Kong/kongctl/actions/runs/34860627806
[replay]: https://github.com/Kong/kongctl/actions/runs/34862188973

[refresh-record]: https://github.com/Kong/kongctl/actions/runs/41902468557
[refresh-replay]: https://github.com/Kong/kongctl/actions/runs/41906250011
