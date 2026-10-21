# Organization user sync replay

The [live recording][record] captured 80 exchanges from commit `556dea98`
under the primary acceptance organization lock, with successful before/after
resets. The selected users are pre-registered lookup targets, not created
users. Their email inputs remain required for live runs. Replay supplies
reserved synthetic identities without credentials or registered users.

## Coverage and ordering

Coverage retains initial apply, membership/role changes through sync,
readback of both users and team roles, and no-op diffs. Cleanup explicitly
scopes an empty team collection, verifies both users' roles are empty and
the managed team is absent, checks diff convergence, and verifies API deletion.

Reviewed phases are bound to the exact recording SHA-256:

| Exchanges | Independent work within one command |
| --- | --- |
| 4–7 | Per-user reads during initial apply planning |
| 8–14 | Initial API/team creation and assignments |
| 19–22 | Per-user reads in the first no-op diff |
| 27–30 | Per-user reads during update planning |
| 31–35 | Independent assignment changes during sync |
| 46–49 | Per-user reads in the second no-op diff |
| 53–56 | Per-user reads during cleanup planning |
| 57–61 | Membership removal and role lookups/deletions |
| 72–75 | Per-user reads in the post-cleanup no-op diff |

All other exchanges stay strictly ordered. Team lookup and deletion at
62–63 follow assignment removal. User-role readbacks at 64–67 and the team
absence read at 68 are barriers before the final no-op diff (69–75).
API cleanup (76–79) precedes the final API absence read (80).
No phase spans commands. Parent creation, repeated-target ordering and
read-before-delete dependencies remain enforced. Distinct users added to
the same team retain exact request-body matching and creation dependencies.

Three [network-isolated CI replays][replay] passed all 80 exchanges.
The committed cassette is identical to each validated CI output.

Candidate data is sanitized before publication. User emails, names and
registration timestamps are pseudonymized, UUID references remain consistent,
and full collections, active states, memberships and roles are preserved.
The optional account type is restricted to documented SDK enum values.
No real-to-synthetic mappings are committed.

Only eligible PRs replay this scenario. Main and force-live runs remain
live and still require the registered users. Changes to inputs, commands
or assertions require a new recording in the same PR.

[record]: https://github.com/Kong/kongctl/actions/runs/39669819402
[replay]: https://github.com/Kong/kongctl/actions/runs/39670928048
