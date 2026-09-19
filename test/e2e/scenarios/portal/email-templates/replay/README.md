# Portal email templates replay

The [live recording][record] captured 97 HTTP exchanges under the existing
acceptance-3 organization lock, with successful before/after resets. The
scenario, inputs, overlays, commands, and assertions are unchanged.
Three [network-isolated replays][replay] passed with the reviewed
annotations. The validated cassette is losslessly packed into two
hash-addressed chunks to keep each file below the repository size limit.

## Coverage and ordering

Coverage includes initial template creation, content updates and null field
handling, disabling a template, dump round-trip planning, content-only
updates without re-enabling the template, idempotency, sync removal, and
final portal deletion.

The first strict replay failed at exchange 21 because independent template
updates can read and write their separate targets in different orders.
The annotation in `parallel-phases.json` is bound to the exact recording
SHA-256. It permits reordering only within five bounded groups:

| Exchanges | Operation |
| --- | --- |
| 4-5 | Create the two custom templates using independent PATCH requests |
| 14-15 | Read the two templates during update planning |
| 19-24 | Read inventory and update the two templates in one apply command |
| 51-52 | Read the templates during planning from the dump |
| 88-93 | Read inventory, remove one template, and update the other in sync |

Command boundaries were checked against local replay harness timestamps.
Initial planning consumes 1-2, initial apply 3-5, update planning 6-18, and
update apply 19-24. Dump and round-trip planning consume 25-41 and 42-57.
Content-only planning, apply, and idempotency consume 58-69, 70-72, and
73-83. Removal planning and sync consume 84-87 and 88-93; final deletion
consumes 94-97. No phase spans commands.

The existing matcher's mandatory dependencies preserve repeated requests
and read-before-write ordering on the same template or its ancestors.
One template's update does not depend on the other template's update.
Portal creation (3) precedes both template creations; the content-only
update (72) and final portal deletion (97) remain strictly ordered.
No later-state response can satisfy an earlier command.

No matcher or sanitizer changes are needed. All request methods, paths,
queries, bodies, response media types, and required exchanges remain
checked. Default logo/favicon responses are identical to the previously
approved Portal teams cassette; no credentials are recorded.

## Measurements

The three loopback-only runs passed all assertions and matched all 97
exchanges. Scenario durations were 2.118, 2.109, and 2.101 seconds; wrapper
durations were 2.870, 2.742, and 2.692 seconds. Ten consecutive local
replays with the reviewed phases also passed before isolated validation.

The frozen weighted-v1 baseline has 20 successful live observations with
a 16.530-second median. Relative to that historical median, replay saves
about 13.8 seconds of execution work per eligible PR. This is not a
same-source benchmark or a measured workflow wall-clock speedup. Live
scenario time already includes its reset; do not add that saving twice.
Shared shard resets, setup, replay startup, and work on other shards remain.

Recording's 14.664-second scenario duration includes proxy overhead and
is not the normal live baseline.

Only eligible PRs replay this scenario. Main and force-live runs remain
live. Replay skips the initial scenario reset entirely, rather than
replaying reset transactions. Shared live-shard resets remain unchanged.

Input or assertion changes require a new reviewed recording in the same
PR; follow the repository replay refresh procedure.

[record]: https://github.com/Kong/kongctl/actions/runs/35446057376
[replay]: https://github.com/Kong/kongctl/actions/runs/35446555297
