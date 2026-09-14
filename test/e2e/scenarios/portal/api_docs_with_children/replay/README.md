# Portal API documents replay

The unchanged scenario passed live Konnect recording in
[run 34860627806][record], including both recording resets and all document
hierarchy, content, publication status, deletion and dump round-trip assertions.
The candidate contains 268 exchanges. Its 84.382-second scenario duration
includes recording proxy overhead and is not the normal live baseline.

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
