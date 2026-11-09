# Portal sync replay

## Publication lifecycle expansion

The [fresh live recording][record] passed the expanded scenario and
before/after resets under the existing acceptance-3 organization lock. It
was captured from commit `2a6fceba` for issue #2727. The reviewed candidate
passed [three network-isolated replays][candidate-replay], each consuming
all 372 exchanges and passing every scenario assertion.

Coverage now positively verifies both initial publications, verifies that
Voice remains published after SMS is unpublished, re-publishes the existing
SMS API, and checks idempotency after re-publication. The existing apply
retention, removal idempotency, dump after removal, dump-to-plan round trip,
API deletion, and final cleanup checks remain intact. Replay eligibility is
unchanged.

## Ordering review

The annotations are bound to the exact raw-candidate SHA-256. Each range
stays within one command; publication verification commands remain strictly
ordered. Mandatory creation, repeated-request, and read-before-write
dependencies remain enforced within annotated ranges.

| Exchanges | Command scope |
| --- | --- |
| 1-7 | Initial sync inventory |
| 8-40 | Initial creation and dependent children |
| 47-80 | Apply with omitted SMS publication |
| 81-120 | Sync inventory before publication removal |
| 123-160 | Idempotency after removal |
| 163-212 | Dump after removal |
| 213-249 | Plan from dump |
| 250-289 | Re-publication inventory and API lookup |
| 296-334 | Idempotency after re-publication |
| 335-362 | Sync inventory before API deletion |
| 366-372 | Final cleanup |

Publication removal at 121, re-publication's not-found lookup and PUT at
290-291, and API deletion at 363-364 retain strict ordering. The initial
publication GETs at 43-46, Voice isolation check at 161-162, and restored
publication GETs at 292-295 cannot cross those mutations or command
boundaries. The not-found response preserves its problem media type.

The committed cassette preserves the fresh recording's requests, responses,
media types, provenance, and current input fingerprint. Packing into seven
hash-addressed chunks is lossless: the unpacked representation equals each
of the three CI-validated candidates. Matching and sanitization rules are
unchanged.

[record]: https://github.com/Kong/kongctl/actions/runs/42722303294
[candidate-replay]: https://github.com/Kong/kongctl/actions/runs/42723428150
