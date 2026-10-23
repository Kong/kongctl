# Organization user assignments replay

The [live recording][record] captured 97 exchanges under the existing primary
acceptance organization lock with successful before/after resets. Existing
users remain lookup targets; replay substitutes reserved synthetic identities.

Coverage retains memberships, direct/team roles, jq filtering, email/ID
selectors, dump/no-op planning, cleanup, and Portal entity references. The
refresh adds a user-role text command that verifies role/entity/region headers,
expected content, and omission of internal columns. JSON checks remain intact.

## Coverage and ordering

Reviewed phases are bound to the exact source recording SHA-256:

| Exchanges | Independent work within one command |
| --- | --- |
| 4–8 | Initial per-user membership and role reads |
| 9–17 | API/team creation and independent assignments |
| 22–27 | Per-user reads during the no-op diff |
| 39–41 | Per-user role reads during dump |
| 46–51 | Team/user assignment reads during planning from dump |
| 55–60 | Team/user assignment reads during cleanup planning |
| 61–67 | Membership removal and role lookup/deletion |

The added text read is exchange 35 and remains strictly ordered. Mutation
phases retain mandatory parent-creation, generated-ID, repeated-target, and
read-before-delete dependencies. Distinct users added to the same team retain
exact user-ID/body matching under the existing bounded membership exception.
No phase crosses a command boundary or changes request/response matching.

All three [isolated replays][replay] passed, consuming all 97 exchanges each.
The promoted cassette contains the validated annotations. Its complete user
collections, active states, roles, and memberships remain represented without
real emails, profile names, or real-to-synthetic mappings.

The isolated wrappers took 3.864, 3.744, 3.706 seconds. These measure execution work,
not workflow wall-clock savings; shared setup and resets remain. Recording
includes proxy/reset overhead and is not a normal live baseline.

Only eligible PRs replay this scenario. Main and force-live runs remain live
and require the registered user email inputs. Scenario changes require a new
recording in the same PR; replay eligibility is preserved.

[record]: https://github.com/Kong/kongctl/actions/runs/39983071496
[replay]: https://github.com/Kong/kongctl/actions/runs/39984329275
