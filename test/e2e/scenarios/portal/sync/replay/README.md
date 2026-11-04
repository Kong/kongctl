# Portal sync replay

## Apply child-scope refresh

The [fresh live recording][refresh-record] passed all scenario assertions
and before/after resets under the existing acceptance-3 organization lock.
It was captured from commit `155dfd85` for issue #2706.
[Three network-isolated replays][refresh-replay] each consumed all 281
recorded exchanges and passed every assertion.

Scenario inputs, overlays, assertions, and replay eligibility are unchanged.
Apply planning now skips children absent from the manifest, so the recording
omits their former comparison reads. Declared-child reads and mutations
retain their assertions, and command/reset boundaries stay intact.

Coverage includes apply retention of an omitted publication, scoped sync
pruning, idempotency, dump-to-plan round trips, API deletion, and final portal
cleanup. Publication and API deletions retain their intervening verification
commands.

The annotations were reviewed against the new recording and rebound to its
exact raw-candidate SHA-256. These ranges retain the previous command scopes;
all other exchanges remain strictly ordered. Mandatory creation, repeated
request, and read-before-write dependencies remain enforced.

| Previous range | Refreshed range |
| --- | --- |
| 1-7 | 1-7 |
| 8-40 | 8-40 |
| 43-82 | 43-76 |
| 83-122 | 77-116 |
| 125-162 | 119-156 |
| 163-212 | 157-206 |
| 213-255 | 207-243 |
| 256-283 | 244-271 |
| 287-293 | 275-281 |

The committed cassette preserves the fresh recording's requests, responses,
media types, provenance, and input fingerprint. Large recordings are packed
losslessly into hash-addressed chunks. Matcher and sanitizer rules are
unchanged. The current validation above covers the committed representation.

[refresh-record]: https://github.com/Kong/kongctl/actions/runs/41902496945
[refresh-replay]: https://github.com/Kong/kongctl/actions/runs/41906264892
