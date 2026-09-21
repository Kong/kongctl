# Organization teams dump replay

The [live recording][record] captured 114 exchanges under the existing
primary acceptance organization lock with successful before/after resets.
The selected users are pre-registered lookup targets, not created users.
Their email inputs remain required for live runs. Replay supplies reserved
synthetic identities and needs neither credentials nor registered users.

## Coverage and ordering

Coverage retains temporary system-account creation, both teams and their
roles, user and system-account memberships, dump/no-op planning, a real
reset boundary, reconstruction from the dump, and final read-back.

Unlike the other replay scenarios, all three in-scenario resets execute
against the proxy. Initial reset consumes 1-16; initial account creation
is 17; bootstrap sync is 18-30; dump and no-op plan are 31-39 and 40-46.
The mid-run reset consumes 47-66, including membership removal (49),
system-account deletion (64), and both team deletions (65-66). Account
recreation (67) precedes reconstruction (68-82). Read-back occupies 83-94;
final reset consumes 95-114. No reset or state transition is skipped.

Reviewed phases are bound to the exact recording SHA-256:

| Exchanges | Independent work within one command |
| --- | --- |
| 4-16 | Initial reset resource inventory |
| 25-30 | Independent team creation and assignments |
| 32-36 | Team/user reads during initial dump |
| 43-46 | Team/user/account reads in the no-op plan |
| 51-63 | Mid-round-trip reset resource inventory |
| 77-82 | Independent reconstruction operations |
| 87-91 | Team/user reads during the verification dump |
| 99-111 | Final reset resource inventory |

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

The three isolated replays matched all 114 exchanges. Scenario durations
were 1.276, 1.280, 1.474 seconds; wrapper durations were
1.862, 1.870, 2.159 seconds. Recording timings include proxy overhead
and are not a normal live baseline. These numbers measure execution work,
not workflow wall-clock savings; setup and shared live-shard resets remain.

Only eligible PRs replay this scenario. Main and force-live runs remain
live and still require the registered users. Changes to inputs, commands,
or assertions require a new recording in the same PR. See the repository
replay policy for the identity and environment-input restrictions.

[record]: https://github.com/Kong/kongctl/actions/runs/35553690971
[replay]: https://github.com/Kong/kongctl/actions/runs/35554100472
