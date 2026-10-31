# Inspect groups and direct memberships

Groups organize principals inside a directory. Ordinary groups accept user,
agent and unclassified principals. Teams are the organization-chart subtype;
direct tests accept user principals. A group is separate from a Konnect
organization IAM team. Names are unique within a directory and case-sensitive
for local lookup.

Use the preview SDK branch and select the directory explicitly:

```sh
kongctl get identity groups --directory-name default
kongctl get identity group Platform --directory-name default -o yaml
kongctl get identity group <group-id> --directory-id <directory-id> -o json
kongctl list konnect identity groups --directory-name default --page-size 1
```

The last command still returns the complete collection. `--page-size` controls
each API request, and the reader follows every cursor before displaying data.
Lookup by UUID or exact name verifies directory collection membership, rejects
ambiguous matches and fetches fresh detail. Public labels, runtime metadata,
`managed_by`, group type and returned provisioning fields are visible. Absent
provisioning/team-manager values do not gain defaults from the SDK.

Read direct relationships with either a group name or UUID:

```sh
kongctl get identity group-memberships \
  --directory-name default --group-name Platform
kongctl get identity group-memberships \
  --directory-id <directory-id> --group-id <group-id> -o yaml
kongctl get identity group-membership <membership-entry-id> \
  --directory-name default --group-name Platform -o json
```

The membership entry's `id` identifies the relationship. `member.id` identifies
its principal or nested group; they are different IDs. Detail lookup accepts
the entry ID and proves it belongs to the selected group before returning it.
The text list shows `ID`, `MEMBER_ID`, `MEMBER_TYPE` and `DISPLAY_NAME`.
These commands inspect direct entries without expanding inherited membership.

Group and membership declarative writes/adoption are still being implemented
under #2657/#2658. Groups expose `managed_by`; memberships have no independent
ownership field. Inspection does not assign ownership or authorize mutation of
an imported team. Direct tests reject group-to-group creation despite the
schema (Q32), and group API-key collection operations return 501 (Q33).
Future declarative deletion must account for relationships and credentials.
See the [API questions](../../../kong-identity-api-questions.md) and
[metadata ledger](../../../contributor/kong-identity-metadata-storage.md).
