# Portal visibility replay

The unchanged scenario was recorded against live Konnect in
[run 34609380360][record]. It passed all assertions and both recording resets,
capturing 390 exchanges. Recording's 50.07-second scenario duration includes
proxy overhead and is not the normal live baseline.

[Three loopback-only CI replays][replay] passed all 390 interactions each.
Wrapper durations were 4.805, 4.958 and 4.951 seconds; scenario durations were
4.122, 4.177 and 4.096 seconds. The historical normal live median is 38.695
seconds across 20 observations, not a same-source or workflow comparison.
Thirty repeated local replays also passed. The packed cassette expands to
exactly the validated CI cassette; no exchanges or assertions were removed.

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
at 126 and publication PUTs at 213 (private) and 260 (public) are barriers,
as are their immediately preceding lookups and following assertion reads.
Cleanup lookups must precede their corresponding deletions. No response from
a later visibility state can cross the intervening mutation.

The live recording passed, but strict replay failed because independent root
creation arrived in a different order (requests at recorded indices 9/10
arrived while strict replay expected 8). Exact request comparison confirmed
matching contents, not a request-body or transport discrepancy. The phase
annotations address this scheduling difference without changing assertions.

Each planning phase also has two publication-list GETs filtered by different
API IDs. Their order varies independently of the returned contents. The
read-only phase permits this query-specific reordering, while repeated
identical requests and all mutating phases retain their stream dependencies.

[record]: https://github.com/Kong/kongctl/actions/runs/34609380360
[replay]: https://github.com/Kong/kongctl/actions/runs/34610832683
