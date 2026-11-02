# Inspect a principal's groups

Use this view to understand which groups apply to a principal. An ordinary
group membership can be assigned directly. An imported organization chart can
also give a user membership in an ancestor team through a nested team. The
principal-group collection distinguishes those relationships.

The [membership declarations](memberships.yaml) create ordinary groups and
direct principal links beneath the external directory named `default`. The
[organization-chart workflow](org-chart.md) manages imported users and teams
through its separate complete-file operation. If default is absent, use the
[directory bootstrap](README.md) first.

Find the principal's UUID; display names are not unique:

```sh
./kongctl get identity principals --directory-name default -o text
```

Supply that UUID explicitly to inspect all its groups:

```sh
./kongctl get identity principal groups \
  --directory-name default --principal-id PRINCIPAL_UUID -o text

./kongctl list konnect identity principal groups \
  --directory-name default --principal-id PRINCIPAL_UUID \
  --page-size 1 -o json
```

Small page size is optional. The reader traverses the complete collection;
it does not return only the first page. A next-page URL supplies a cursor,
while subsequent requests retain the configured API host and scoped route.
Repeated cursors/IDs, malformed pages and failed later pages return an error
without partial results.

| Field | Meaning |
| --- | --- |
| `id`, `name`, `type` | The group, which can be an ordinary group or an imported team. `id` is the group UUID. |
| `direct` | `true` for a direct relationship, `false` for inherited membership through a nested team. |
| `membership_id` | Relationship-entry UUID for a direct membership; null for an inherited membership. It is different from the group and principal UUIDs. |
| `provisioned_by` | The observed provisioning source, such as `konnect_user` or `imported`; omitted values are not defaulted by the client. |
| `team_manager` | Observed manager ID, display name and nullable email, when returned by the API. |

Select one result by **group UUID or exact group name**:

```sh
./kongctl get identity principal group GROUP_UUID \
  --directory-name default --principal-id PRINCIPAL_UUID -o yaml

./kongctl get identity principal group 'Engineering' \
  --directory-name default --principal-id PRINCIPAL_UUID -o text
```

Selection uses the principal's complete group collection. A membership-entry
UUID does not select a group. A group elsewhere in the directory is not a
match unless this principal belongs to it. Ambiguous names fail explicitly.
There is no separate association detail endpoint or global group lookup used
to satisfy a missing result.

For example, an imported worker can have a direct relationship to their
manager's team and an inherited relationship to the next ancestor team.
Both can report `type: team` and `provisioned_by: imported`. The direct
relationship has an entry UUID; the inherited one has `membership_id: null`.
Text detail displays the manager fields and direct/inherited status without
falling back to JSON. JSON/YAML preserve the public field structure.

These commands are read-only and stamp no labels or `managed_by`. A direct
flag and an entry UUID do not authorize deleting an importer-managed link.
Use ordinary [membership declarations](memberships.md) for an explicitly
managed ordinary group, and the complete-file import workflow for the
organization chart. Inherited membership is not a direct entry to remove.
Principal-group inspection is not a new declarative resource or adoption path.

The SDK preview for this branch includes the principal-group cursor correction
under API question Q22. No new API field or runtime authentication fixture is
needed. This administrative projection does not prove runtime authorization
or Token Vault credential enrollment/revocation.
