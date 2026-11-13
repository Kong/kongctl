# Organization teams dump replay

The [live recording][record] captured 397 exchanges under the primary
acceptance organization lock with successful before/after resets. Its
scenario passed in 122.049 seconds. Selected users are existing lookup
subjects, not created users. Live runs still require their email inputs;
replay uses reserved synthetic identities without Konnect credentials.

## Coverage

The original membership and reconstruction coverage remains: temporary
system-account creation, both teams and their roles, user and account
memberships, dump/no-op planning, a reset boundary, reconstruction from the
dump, and final read-back.

A separate namespace contains 51 APIs and one team with 101 assigned Admin
and Viewer roles. This exceeds the reader's requested 100-role page size.
Five dumps use `--page-size` values 1, 10, 99, 100 and 101 and must export all
101 assignments. Both apply and sync plans use the complete original
manifest and must report zero changes, independently of dump output.

Two subsequent syncs each remove exactly one assignment. Dump and no-op
apply/sync planning are repeated at 100 and 99 roles. The recording proves
two assigned-role requests at 101 roles (100 plus 1), and exactly one at
100 and 99. No page request, assertion, state transition or reset is skipped.
Unit tests additionally cover empty collections, 1-item pages, boundaries
around 10 and 200, capped response sizes and later-page failures.

## Command boundaries and ordering

| Exchanges | Scenario operation |
| --- | --- |
| 1-16 | Initial reset |
| 17 | Create temporary system account |
| 18-30 | Bootstrap teams, roles and memberships |
| 31-39 | Original organization-team dump |
| 40-46 | No-op plan from that dump |
| 47-66 | Reset before reconstruction |
| 67 | Recreate temporary system account |
| 68-82 | Reconstruct teams and memberships from the dump |
| 83-94 | Read back reconstructed teams and memberships |
| 95-249 | Bootstrap pagination APIs, team and 101 roles |
| 250-279 | Five dumps with different page-size settings |
| 280-283, 284-287 | Original 101-role manifest apply and sync plans |
| 288-296 | Remove one assignment to reach 100 roles |
| 297-301 | Dump exactly 100 roles |
| 302-304, 305-307 | Original 100-role manifest apply and sync plans |
| 308-314 | Remove one assignment to reach 99 roles |
| 315-319 | Dump 99 roles |
| 320-322, 323-325 | Original 99-role manifest apply and sync plans |
| 326-397 | Final reset, including all 51 APIs |

Reviewed phases are bound to the exact candidate SHA-256:

| Exchanges | Independent work within one command |
| --- | --- |
| 4-16 | Initial reset resource inventory |
| 25-30 | Independent team creation and assignments |
| 32-36 | Team/user reads during the initial dump |
| 43-46 | Team/user/account reads in the no-op plan |
| 51-63 | Mid-round-trip reset resource inventory |
| 77-82 | Independent reconstruction operations |
| 87-91 | Team/user reads during the verification dump |
| 330-342 | Final reset resource inventory |

The bulk pagination bootstrap uses `maxConcurrency: 1` to keep its creation
order deterministic. Those requests, all pagination reads and both
single-assignment deletions remain strictly ordered. Every phase is
bounded to one command; parent creation, repeated-target order and
read-before-delete dependencies remain enforced by the existing matcher.
No matcher or sanitization rules were changed.

The [isolated validation][replay] matched all 397 exchanges three times
with only loopback networking before promotion. Scenario durations were
7.356, 7.398 and 7.340 seconds. Packing preserves all requests, responses,
ordering and provenance. A separate validation of the committed packed
representation is linked in the PR.

Only eligible PRs replay this scenario. Main and force-live runs remain
live and require the registered users. Input, command or assertion changes
require a new recording in the same PR; replay eligibility is unchanged.

[record]: https://github.com/Kong/kongctl/actions/runs/43497700222
[replay]: https://github.com/Kong/kongctl/actions/runs/43499384606
