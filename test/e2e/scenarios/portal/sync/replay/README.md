# Portal sync replay

## Publication removal readback refresh

The [fresh live recording][record] passed the expanded scenario and
before/after resets under the existing acceptance-3 organization lock. It
was captured from commit `3d50336b` for the review follow-up to issue #2727.
The reviewed candidate passed
[three network-isolated replays][candidate-replay], each consuming all 374
exchanges and passing every scenario assertion.

Coverage positively verifies both initial publications, verifies that Voice
remains published after SMS is unpublished, directly verifies an empty SMS
publication list, re-publishes the existing SMS API, and checks idempotency
after re-publication. The existing apply retention, removal idempotency, dump
after removal, dump-to-plan round trip, API deletion, and final cleanup checks
remain intact. Replay eligibility is unchanged.

The [committed chunked representation][committed-replay] also passed three
network-isolated replays from commit `e60fb615`, consuming all 374 exchanges
in every run.

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
| 165-214 | Dump after removal |
| 215-251 | Plan from dump |
| 252-291 | Re-publication inventory and API lookup |
| 298-336 | Idempotency after re-publication |
| 337-364 | Sync inventory before API deletion |
| 368-374 | Final cleanup |

Publication removal at 121, re-publication's not-found lookup and PUT at
292-293, and API deletion at 365-366 retain strict ordering. The initial
publication GETs at 43-46, Voice isolation check at 161-162, empty SMS
publication readback at 163-164, and restored publication GETs at 294-297
cannot cross those mutations or command boundaries. The not-found response
preserves its problem media type.

The committed cassette preserves the fresh recording's requests, responses,
media types, provenance, and current input fingerprint. Packing into seven
hash-addressed chunks is lossless: the unpacked representation equals each
of the three CI-validated candidates. Matching and sanitization rules are
unchanged.

[record]: https://github.com/Kong/kongctl/actions/runs/42733116346
[candidate-replay]: https://github.com/Kong/kongctl/actions/runs/42734178986
[committed-replay]: https://github.com/Kong/kongctl/actions/runs/42735053013
