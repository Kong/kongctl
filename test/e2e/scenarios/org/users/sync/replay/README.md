# Organization user sync replay

The [live recording][record] captured 65 exchanges under the existing
primary acceptance organization lock with successful before/after resets.
The selected users are pre-registered lookup targets, not created users.
Their email inputs remain required for live runs. Replay supplies reserved
synthetic identities and needs neither credentials nor registered users.

## Coverage and ordering

Coverage retains initial apply, membership/role changes through sync,
read-back of both users and team roles, no-op diffs, and cleanup.

Reviewed phases are bound to the exact recording SHA-256:

| Exchanges | Independent work within one command |
| --- | --- |
| 4-7 | Per-user reads during initial apply planning |
| 8-14 | Initial API/team creation and assignments |
| 19-22 | Per-user reads in the first no-op diff |
| 27-30 | Per-user reads during update planning |
| 31-35 | Independent assignment changes during sync |
| 46-49 | Per-user reads in the second no-op diff |
| 53-56 | Per-user reads during cleanup planning |
| 57-61 | Membership removal and role lookups/deletions |

Command boundaries were checked against local replay harness timestamps.
All other exchanges stay strictly ordered. Parent creation, repeated-target
ordering, and read-before-delete dependencies remain enforced. The sole
empty-response create exception is distinct users added to the same team;
matching still checks the exact user ID, method, endpoint, path, and body.

Ten consecutive local replays passed. [Isolated validation][replay] runs
all assertions three times with only loopback networking. Candidate data
is sanitized before publication; no real user emails, profile names, or
real-to-synthetic mappings are committed. Full user collections, active
states, memberships, and roles remain represented.

## Measurements

The three isolated replays matched all 65 exchanges. Scenario durations
were 1.712, 1.706, 1.747 seconds; wrapper durations were
2.592, 2.285, 2.541 seconds. Recording timings include proxy overhead
and are not a normal live baseline. These numbers measure execution work,
not workflow wall-clock savings; setup and shared live-shard resets remain.

Only eligible PRs replay this scenario. Main and force-live runs remain
live and still require the registered users. Changes to inputs, commands,
or assertions require a new recording in the same PR. See the repository
replay policy for the identity and environment-input restrictions.

[record]: https://github.com/Kong/kongctl/actions/runs/35463077601
[replay]: https://github.com/Kong/kongctl/actions/runs/35463671463
