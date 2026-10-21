# Control plane sync replay

The expanded scenario was recorded in [run 39666956462][recording] from
commit `2e59e4c5`. The live scenario and before/after resets passed.
The recording preserves all 20 exchanges, including default sync-mode diffs
for UPDATE and DELETE and dump-to-sync-plan convergence after replacement.

## Recording review

Generated IDs and telemetry endpoints are sanitized. Request bodies,
response bodies and media types retain the recorded semantics.

| Exchanges | Command and state |
| --- | --- |
| 1–2 | Initial sync: inventory and CREATE of `kongctl-e2e-cp` |
| 3 | Repeat sync: initial state, no writes |
| 4 | Default-mode diff: UPDATE of the original control plane |
| 5–7 | Update sync: inventory, name lookup and PATCH |
| 8 | Get: updated description, labels and standard cluster type |
| 9 | Repeat sync: updated state, no writes |
| 10 | Default-mode diff: DELETE original and CREATE replacement |
| 11 | Replacement sync: inventory of the updated original |
| 12–14 | Replacement execution: create new CP, look up and delete old CP |
| 15 | Get: only the replacement remains |
| 16 | Filtered dump: only the replacement remains |
| 17 | Sync-mode plan from the dump: no writes |
| 18–20 | Cleanup: inventory, lookup and deletion of the replacement |

Only exchanges 12–14 may reorder. Creating the replacement is independent
of looking up and deleting the original. The matcher's mandatory same-target
dependency keeps the original lookup before its deletion. Inventory, diff,
dump and verification commands remain strict barriers. No phase spans
commands.

`parallel-phases.json` binds the review to the exact unannotated candidate
SHA-256. Strict replay initially failed on the independent replacement
operations; annotations change only scheduling, with exact request matching
and every scenario assertion retained.

[recording]: https://github.com/Kong/kongctl/actions/runs/39666956462

Three [network-isolated CI replays][validation] passed all 20 exchanges.
The committed cassette is identical to each validated CI output.

[validation]: https://github.com/Kong/kongctl/actions/runs/39670010464
