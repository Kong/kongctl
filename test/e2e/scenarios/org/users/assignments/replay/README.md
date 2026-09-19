# Organization user assignments replay

The [live recording][record] captured 96 exchanges under the existing
primary acceptance organization lock with successful before/after resets.
The selected users are pre-registered lookup targets, not created users.
Their email inputs remain required for live runs. Replay supplies reserved
synthetic identities and needs neither credentials nor registered users.

## Coverage and ordering

Coverage retains team memberships, direct and team roles, jq filtering,
email/ID selectors, dump/no-op planning, cleanup, and portal entity references.

Reviewed phases are bound to the exact recording SHA-256:

| Exchanges | Independent work within one command |
| --- | --- |
| 4-8 | Independent user membership/role reads during initial apply |
| 9-17 | Initial API/team creation and independent assignments |
| 22-27 | Per-user reads in the no-op diff |
| 38-40 | Per-user role reads within dump |
| 45-50 | Team/user assignment reads during planning from dump |
| 54-59 | Per-user reads in cleanup planning |
| 60-66 | Independent membership removals and role lookup/deletion |

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

The three isolated replays matched all 96 exchanges. Scenario durations
were 2.762, 2.756, 2.773 seconds; wrapper durations were
3.548, 3.502, 3.524 seconds. Recording timings include proxy overhead
and are not a normal live baseline. These numbers measure execution work,
not workflow wall-clock savings; setup and shared live-shard resets remain.

Only eligible PRs replay this scenario. Main and force-live runs remain
live and still require the registered users. Changes to inputs, commands,
or assertions require a new recording in the same PR. See the repository
replay policy for the identity and environment-input restrictions.

[record]: https://github.com/Kong/kongctl/actions/runs/35463074418
[replay]: https://github.com/Kong/kongctl/actions/runs/35463667949
