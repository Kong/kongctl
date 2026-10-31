# Manage a group's direct principal memberships

Use [memberships.yaml](memberships.yaml) with the organization's existing
directory named `default`. If it is missing, first bootstrap it with
[directories.yaml](directories.yaml). The external directory lookup never
creates or adopts the directory.

The example creates an agent, an ordinary group and a direct membership:

```sh
kongctl plan --mode apply -f docs/examples/declarative/identity/memberships.yaml
kongctl apply -f docs/examples/declarative/identity/memberships.yaml \
  --auto-approve
kongctl get identity groups --directory-name default
kongctl get identity group-memberships \
  --directory-name default --group-name OrderOperators
```

The membership response has two identifiers: `id` selects the relationship
entry for API requests, while `member.id` identifies the principal.
Configuration accepts a principal UUID or `!ref dispatcher#id`. The principal
must belong to the group's directory. New groups and principals can be
created in the same saved plan; execution orders the relationship last.

The group stores four ownership entries in `managed_by`. Membership entries
have no labels, `managed_by`, namespace or protection of their own. They
inherit authority from the managed group, even if their principal belongs to
another namespace or is an external reference. Directory protection does not
restrict that independently owned group. An external group cannot authorize
membership writes.

Applying the same configuration produces no changes. A local membership ref
is not persisted: matching uses the group and principal ID. Renaming the local
ref does not replace the relationship. Changing the principal adds a different
relationship; ordinary `apply` retains undeclared members. Use `sync` to remove
members omitted from an explicitly selected membership collection.

Omitting `memberships` preserves the collection. An explicit `memberships: []`
selects its direct principal memberships for removal during sync. Sync also
reconciles the selected parent group collection: include other groups you
intend to retain in that namespace, or use a dedicated namespace. It does not
delete the member principals merely because their memberships were removed.

A protected group permits adding new memberships, following the existing
protected-parent child pattern. It blocks membership removal. Unprotect the
group in a separate update before pruning members. Saved writes recheck fresh
group identity, ownership, protection and the selected relationship.

To manage an existing ordinary group and its direct memberships, explicitly
adopt the group first:

```sh
kongctl adopt identity group OrderOperators \
  --directory-name default --namespace fulfillment
```

Adoption preserves the membership entries. There is no independent membership
adoption command or ownership stamp: declaring a managed group and selecting
its membership collection establishes authority over that collection. Review
existing members before an authoritative sync.

Dump includes direct principal memberships under managed ordinary groups,
using entry IDs as local refs and typed references to principals included in
the dump. Other targets retain UUIDs:

```sh
kongctl dump declarative --resources identity.directories \
  --filter-name default --include-child-resources > identity.yaml
kongctl plan --mode apply -f identity.yaml
```

Unowned and importer/team groups remain external references. A managed group
with unsupported nested relationships fails export rather than producing an
incomplete authoritative membership collection. Nested groups, team/importer
mutations and batch creation remain deferred under API question Q32.

Group or principal deletion requires explicit authorized deletions for every
observed direct membership first. A complete delete plan orders detachments
before those resources, and execution checks their collections again. An
undeclared membership blocks parent deletion. Group deletion also requires
an empty credential observation; no key is implicitly removed. These client
checks do not provide an atomic server-side nonempty-resource precondition.
