# Control plane sync replay

The expanded scenario was recorded in [run 35249677971][recording] from
commit `e3ef7d42d`. The live scenario and
final reset passed. Its initial strict replay failed at the replacement
step; the sanitized candidate remains the source for annotation validation.

All three [isolated replays][validation] passed with the reviewed annotation,
each consuming all 16 exchanges and passing the expanded scenario assertions.
The committed cassette is the identical `validated-cassette.json` produced
by all three attempts. It differs from the live candidate only by the
reviewed `parallel_phases`; its input fingerprint and provenance are intact.

## Recording review

The recording contains 16 exchanges. Generated IDs and control plane and
telemetry endpoints are sanitized. Requests, response bodies and media types
retain the recorded semantics, including the updated description and labels.
The scenario assertions and input fingerprint are unchanged by annotation.

| Exchanges | Command and state |
| --- | --- |
| 1–2 | Initial sync: inventory and CREATE of `kongctl-e2e-cp` |
| 3 | Repeat sync: initial state, no writes |
| 4–6 | Update sync: inventory, name lookup, PATCH of the same ID |
| 7 | Get: updated description, labels and standard cluster type |
| 8 | Repeat sync: updated state, no writes |
| 9 | Replacement sync: inventory of the updated original |
| 10–12 | Replacement execution: create new CP, look up and delete old CP |
| 13 | Get: only the replacement remains |
| 14–16 | Cleanup: inventory, lookup and deletion of the replacement |

Only exchanges 10–12 may reorder. Exchange 10 creates `kongctl-e2e-cp-2`
with a new ID, independent of the old control plane lookup at 11 and its
deletion at 12. The matcher's mandatory same-target dependency keeps 11
before 12. Inventory at 9 and verification at 13 remain strict barriers.
All other exchanges, including UPDATE, both no-op checks and cleanup, stay
strictly ordered. No phase spans commands.

`parallel-phases.json` binds this review to the exact unannotated candidate
SHA-256. The positions were reviewed against this recording, rather than
copied from the previous cassette.

[recording]: https://github.com/Kong/kongctl/actions/runs/35249677971
[validation]: https://github.com/Kong/kongctl/actions/runs/35250398004
