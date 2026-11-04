# Portal pages replay

## Apply child-scope refresh

The [fresh live recording][refresh-record] passed all scenario assertions
and before/after resets under the existing acceptance-3 organization lock.
It was captured from commit `155dfd85` for issue #2706.
[Three network-isolated replays][refresh-replay] each consumed all 43
recorded exchanges and passed every assertion.

Scenario inputs, overlays, assertions, and replay eligibility are unchanged.
Apply planning now skips children absent from the manifest, so the recording
omits their former comparison reads. Declared-child reads and mutations
retain their assertions, and command/reset boundaries stay intact.

Every exchange remains strictly ordered; no parallel annotations are needed.

The committed cassette preserves the fresh recording's requests, responses,
media types, provenance, and input fingerprint. Large recordings are packed
losslessly into hash-addressed chunks. Matcher and sanitizer rules are
unchanged. The current validation above covers the committed representation.

## Previous recording (superseded)

The scenario passed [live recording and three isolated replays][record]
with 71 HTTP exchanges. Recording used the existing acceptance-3 lock and
successful before/after organization resets. The only scenario edit changes
`baseInputsPath: ./testdata` to the equivalent `baseInputsPath: testdata`,
the spelling required by replay preflight. Inputs, steps, and assertions
are otherwise unchanged.

### Coverage and ordering

The cassette exercises portal creation and parent/child pages, metadata
updates to the child, a content-only update and subsequent no-op plan,
planning content with a missing opening frontmatter delimiter, and leaf
deletion while preserving its parent.

Every exchange remains strictly ordered. The parent page is created at
exchange 4 before the child references its ID at exchange 5. Metadata and
content updates occur at exchanges 22 and 39; later reads observe those
states. Exchange 69 deletes only the child, followed by the final read-back
assertions. Recording's final organization reset removes the remaining
portal; that reset is outside the cassette, as for other replay scenarios.

No parallel-phase annotations, matcher relaxations, or sanitization changes
were needed. UUIDs use the existing deterministic placeholders; request
methods, paths, queries, bodies, and response media types remain preserved.
The cassette is byte-for-byte the successful candidate, below the per-file
size limit, so no chunk packing is needed.

### Measurements

The three loopback-only runs matched all 71 exchanges and passed every
scenario assertion. Scenario durations were 2.065, 1.990, and 2.021 seconds;
wrapper durations were 2.730, 2.600, and 2.659 seconds.

The frozen weighted-v1 baseline has 20 successful live observations with
an 8.025-second median. Relative to that historical baseline, replay saves
about 5.4 seconds of execution work per PR. This is not a same-source
benchmark or a measured workflow speedup. Live scenario timing already
includes its reset, so the avoided reset must not be added again. Shared
shard resets, job setup, and time on other shards remain.

Recording's 11.311-second scenario duration includes proxy overhead and
is not the normal live baseline.

Only eligible PRs replay this scenario. Main and force-live runs remain
live. Input or assertion changes require a new reviewed recording in the
same PR; follow the repository replay refresh procedure.

[record]: https://github.com/Kong/kongctl/actions/runs/35361530847

[refresh-record]: https://github.com/Kong/kongctl/actions/runs/41902492036
[refresh-replay]: https://github.com/Kong/kongctl/actions/runs/41902492036
