# Organization user plan/sync replay

The [live recording][record] captured 59 exchanges under the existing
primary acceptance organization lock with successful before/after resets.
The selected users are pre-registered lookup targets, not created users.
Their email inputs remain required for live runs. Replay supplies reserved
synthetic identities and needs neither credentials nor registered users.

## Coverage and ordering

Coverage retains bootstrap sync, saved sync plans, text diffs, membership
addition and role removal, two no-op checks, and cleanup.

Reviewed phases are bound to the exact recording SHA-256:

| Exchanges | Independent work within one command |
| --- | --- |
| 6-10 | Initial API/team creation and assignments |
| 21-24 | Per-user reads during sync planning |
| 25-27 | Membership addition and independent role lookup/removal |
| 32-35 | Per-user reads in the no-op diff |
| 40-43 | Per-user reads in the post-sync plan |
| 47-50 | Per-user reads during cleanup planning |
| 51-52 | Independent membership removals |
| 56-59 | Independent API/team lookup and deletion |

Command boundaries were checked against local replay harness timestamps.
All other exchanges stay strictly ordered. Parent creation, repeated-target
ordering, and read-before-delete dependencies remain enforced. The sole
empty-response create exception is distinct users added to the same team;
matching still checks the exact user ID, method, endpoint, path, and body.

The refreshed recording passed [isolated validation][replay] three times
with only loopback networking. Candidate data is sanitized before
publication; no real user emails, profile names, or
real-to-synthetic mappings are committed. Full user collections, active
states, memberships, and roles remain represented.

## Measurements

The three isolated replays matched all 59 exchanges. Scenario durations
were 2.097, 2.107, 2.109 seconds; wrapper durations were
2.660, 2.755, 2.671 seconds. Recording timings include proxy overhead
and are not a normal live baseline. These numbers measure execution work,
not workflow wall-clock savings; setup and shared live-shard resets remain.

Only eligible PRs replay this scenario. Main and force-live runs remain
live and still require the registered users. Changes to inputs, commands,
or assertions require a new recording in the same PR. See the repository
replay policy for the identity and environment-input restrictions.

[record]: https://github.com/Kong/kongctl/actions/runs/35553696438
[replay]: https://github.com/Kong/kongctl/actions/runs/35554143695
