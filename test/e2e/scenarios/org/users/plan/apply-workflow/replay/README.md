# Organization user plan/apply replay

The [live recording][record] captured 38 exchanges under the existing
primary acceptance organization lock with successful before/after resets.
The selected users are pre-registered lookup targets, not created users.
Their email inputs remain required for live runs. Replay supplies reserved
synthetic identities and needs neither credentials nor registered users.

## Coverage and ordering

Coverage retains saved apply plans, text diffs, exact change counts,
application, no-op diffing, assignment cleanup, and resource deletion.

Reviewed phases are bound to the exact recording SHA-256:

| Exchanges | Independent work within one command |
| --- | --- |
| 4-6 | Per-user reads during initial planning |
| 7-12 | API/team creation and independent assignments in apply |
| 17-20 | Per-user reads in the no-op diff |
| 24-27 | Per-user reads in cleanup planning |
| 28-31 | Independent membership removals and role lookup/deletion |
| 35-38 | Independent API/team lookup and deletion |

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

The three isolated replays matched all 38 exchanges. Scenario durations
were 1.549, 1.537, 1.461 seconds; wrapper durations were
2.143, 2.236, 2.493 seconds. Recording timings include proxy overhead
and are not a normal live baseline. These numbers measure execution work,
not workflow wall-clock savings; setup and shared live-shard resets remain.

Only eligible PRs replay this scenario. Main and force-live runs remain
live and still require the registered users. Changes to inputs, commands,
or assertions require a new recording in the same PR. See the repository
replay policy for the identity and environment-input restrictions.

[record]: https://github.com/Kong/kongctl/actions/runs/35553695006
[replay]: https://github.com/Kong/kongctl/actions/runs/35554062188
