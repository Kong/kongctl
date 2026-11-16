# Organization teams dump replay

The [live recording][record] captured 504 exchanges under the primary
acceptance organization lock with successful before/after resets. Its
scenario passed in 135.877 seconds. Selected users are existing lookup
subjects, not created users. Live runs still require their email inputs;
replay uses reserved synthetic identities without Konnect credentials.

## Coverage

The original membership and reconstruction coverage remains: temporary
system-account creation, both teams and their roles, user and account
memberships, dump/no-op planning, a reset boundary, reconstruction from the
dump, and final read-back.

A separate namespace contains 51 APIs and one team with 101 assigned Admin
and Viewer roles. Five dumps use `--page-size` values 1, 10, 99, 100 and
101 and must export all 101 assignments. Both apply and sync plans use the
complete original manifest and must report zero changes, independently of
dump output.

The recording verifies the actual assigned-role HTTP requests:

| Configured page size | Role request size | Pages | Returned role counts |
| --- | --- | --- | --- |
| 1 | 1 | 101 | One role on every page |
| 10 | 10 | 11 | Ten roles on ten pages, then one |
| 99 | 99 | 2 | 99, 2 |
| 100 | 100 | 2 | 100, 1 |
| 101 | 100 | 2 | 100, 1 |

Konnect rejects assigned-role page sizes above 100. The reader caps larger
CLI settings for this endpoint and continues paging; the team collection
still receives the configured size. No pagination reads are skipped.

Two subsequent syncs each remove exactly one assignment. Dump and no-op
apply/sync planning are repeated at 100 and 99 roles. Each of those role
collections takes exactly one page, with no extra empty-page request.
Deletion lookup stops on the page containing the target, while planning
reads every page. Unit tests additionally cover empty collections, 1-item
pages, boundaries around 10 and 200, capped responses, larger configured
page sizes, and later-page failures.

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
| 250-354 | Dump 101 roles with page size 1 |
| 355-369 | Dump 101 roles with page size 10 |
| 370-375 | Dump 101 roles with page size 99 |
| 376-381 | Dump 101 roles with page size 100 |
| 382-387 | Dump 101 roles with page size 101 |
| 388-391, 392-395 | Original 101-role manifest apply and sync plans |
| 396-403 | Remove one assignment to reach 100 roles |
| 404-408 | Dump exactly 100 roles |
| 409-411, 412-414 | Original 100-role manifest apply and sync plans |
| 415-421 | Remove one assignment to reach 99 roles |
| 422-426 | Dump 99 roles |
| 427-429, 430-432 | Original 99-role manifest apply and sync plans |
| 433-504 | Final reset, including all 51 APIs |

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
| 437-449 | Final reset resource inventory |

The bulk pagination bootstrap uses `maxConcurrency: 1` to keep its creation
order deterministic. Its 153 writes have a three-minute command budget.
Those requests, all pagination reads and both single-assignment deletions
remain strictly ordered. Every phase is bounded to one command; parent
creation, repeated-target order and read-before-delete dependencies remain
enforced by the existing matcher. No matching or sanitization rules changed.

The [isolated candidate validation][replay] matched all 504 exchanges three
times with only loopback networking before promotion. Packing preserves
all requests, responses, ordering and provenance. The committed packed
representation is verified separately in isolated CI.

Only eligible PRs replay this scenario. Main and force-live runs remain
live and require the registered users. Input, command or assertion changes
require a new recording in the same PR; replay eligibility is unchanged.

[record]: https://github.com/Kong/kongctl/actions/runs/43527056354
[replay]: https://github.com/Kong/kongctl/actions/runs/43528255273
