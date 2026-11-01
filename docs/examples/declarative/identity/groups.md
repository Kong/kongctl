# Manage ordinary groups and inspect direct memberships

Groups organize principals inside a directory. Ordinary groups accept user,
agent and unclassified principals. Teams are the organization-chart subtype;
US `.com` tests accept user principals, but the current `.tech` test org rejects
team creation and accepts only ordinary groups. A group is separate from a
Konnect organization IAM team. Names are unique within a directory and
case-sensitive for local lookup.

`groups.yaml` declares ordinary groups under an externally resolved directory
named `default`. Groups start empty; [memberships.yaml](memberships.yaml) and the
[membership tutorial](memberships.md) add direct principal relationships.
The directory is neither adopted nor modified by this example.

```sh
kongctl plan --mode apply -f groups.yaml --output-file group-plan.json
kongctl diff --plan group-plan.json
kongctl apply --plan group-plan.json --auto-approve
kongctl get identity groups --directory-name default -o json
kongctl plan --mode apply -f groups.yaml
```

Change a description, label or runtime metadata value, then use `diff` and
`apply` again. Omitted public settings are preserved from a fresh read before
PUT; an explicit empty description or map clears that setting. Metadata has
the same integer precision restrictions as principals (Q31).

Names are required and allow only 1–255 ASCII letters/digits. Matching uses
directory ID plus name. Changing a name declares a new resource: apply retains
the old group, while scoped sync can remove it only after its descendants
have been removed. Local refs are not stored remotely because names identify
groups uniquely. Ownership uses four `managed_by` entries: `tool`, `version`,
`kongctl-namespace`, and `kongctl-protected`. Labels and runtime metadata are
public API fields, separate from ownership.

Ordinary groups have independent namespace/protection, including below a
protected or external directory. Only directly provisioned groups with
observed `provisioned_by: konnect_user` are managed. Team/importer writes are
deferred. Omitted/null type has the API's documented ordinary-group meaning;
read commands preserve actual field presence.

For an existing unowned ordinary group, adopt it explicitly:

```sh
kongctl adopt identity group FulfillmentOperators --directory-name default \
  --namespace identity-groups
```

Adoption preserves public settings and protection. Existing namespace
ownership requires `--overwrite-namespace`. A protection-only unprotect must
be applied separately from public changes. External group declarations can
look up a name or ID within a declared directory without taking ownership.

Omitted group collections preserve existing groups. An explicit `groups: []`
under a directory prunes only the source default namespace. Foreign and
unowned groups are retained. Deletion requires fresh empty membership and
API-key observations; unreadable/unsupported key endpoints block deletion.
Remove descendants explicitly before running:

```sh
kongctl delete -f groups.yaml --auto-approve
```

The default directory remains. Principal deletion likewise refuses direct
group memberships until they are explicitly removed. These checks require
directory-wide group/membership read access; they are client checks, not an
atomic service precondition (Q24).

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

For the groups a principal belongs to, including inherited ancestor teams,
use [principal group inspection](principal-groups.md). Its `direct` and nullable
`membership_id` fields distinguish the two relationships without granting
authority over importer-managed links.

Ordinary group lifecycle and adoption are implemented under #2657. Direct
membership declarations inherit managed ordinary-group authority under #2658;
memberships have no independent ownership field. Inspection does not assign
ownership or authorize mutation of an imported team. Direct tests reject group-to-group creation despite the
schema (Q32). Group API-key operations work in US `.tech`; earlier US `.com`
requests returned 501 (Q33). Ownership PATCH still fails for keys in `.tech`.
Group deletion blocks while those relationships or credentials remain.
See the [API questions](../../../kong-identity-api-questions.md) and
[metadata ledger](../../../contributor/kong-identity-metadata-storage.md).

The inspection scenario explicitly probes team creation. A successful create
exercises team reads/membership; the exact recorded Q32 enum rejection
records teams as unavailable and uses an ordinary alternate group for scope
and pagination checks. Other failures fail the scenario. This does not
establish importer behavior or grant team mutation authority.
