# Incremental managed_by support

Resources opt into writing `managed_by` independently of how kongctl decides
ownership and protection. This allows each resource to migrate in two stages:

The [migration tracker][migration] records resource adoption across releases.

| Stage | Writes | Lifecycle authority |
| --- | --- | --- |
| Existing | Existing labels | Existing label rules |
| Write metadata | Existing labels plus tool/version | Existing label rules |
| Migrate ownership | Tool/version and lifecycle fields | managed_by rules |

Resources without verified `managed_by` support remain unchanged. Migrating
ownership stops using kongctl's management labels for lifecycle decisions;
user-defined public labels remain supported.

## Stage one: write metadata alongside existing labels

Register the capability in the resource's own registration, using its verified
API limit. For example, AI Gateway model providers register:

```go
WithManagedBy(ManagedByPolicy{MaxEntries: 5}),
```

There is no central resource allowlist to edit. Registration does not add
namespace participation, change ownership or protection, adopt existing
objects, or alter label handling. Existing management labels must continue to
be written and consulted until that resource's ownership migration is reviewed.

The common planning pass stamps create/update fields through
`resources.PrepareManagedBy`. Each enrolled resource's state create/update
method calls the shared request helper before sending the SDK request:

```go
if err := stampManagedByRequest(ctx, resources.ResourceTypeAIGatewayProvider,
    &req); err != nil {
    return "", err
}
```

This helper is in `internal/declarative/state/managed_by.go`; it has no
AI Gateway parent or ownership assumptions. It reads the registered policy,
clones the SDK request, writes `tool: kongctl` and the executing binary's
`version`, validates capacity, and verifies that the SDK retains the metadata.
An unenrolled type is unchanged. An incompatible SDK request fails before an
HTTP write. SDK serialization errors are sanitized to avoid exposing secrets.

For mutable policies, use this path for every create/update, including
replacements, saved plans, direct state calls and secret-only updates. Plan
previews show the planning
binary version; execution records the executing binary version. Local builds
without release information record `dev`.

`PrepareManagedBy` generates the complete tool-owned map. It discards outside
entries, stamps tool/version, and retains lifecycle keys only for explicitly
owned resources. Resource adapters still verify the observed owner/protection
before writing; normalization is not adoption or transfer. Public labels and
credentials retain their separate preservation contracts.

`managed_by` is reserved at registered resource boundaries in the common
explain/load schema. Declarative input is rejected, including empty/null maps,
root/nested declarations, and SDK unions. Explain/scaffold omit the field. Dump
must omit the entire map and translate supported lifecycle metadata into
`kongctl` configuration so dump/reload remains valid. Internal SDK requests and
saved plans still carry generated metadata.

Only version differences are ignored by `managedByValuesEqual`. Missing or
incorrect tool values and outside keys must produce visible metadata drift, even
with an otherwise unchanged payload. Required writes stamp the executing
binary's version. Version upgrades alone do not trigger writes.

Consumer credentials have no in-place update path: metadata-only drift must warn
with the observed/canonical maps, not rotate a credential. Preserve the
runtime-certificate key requirement and AppRole metadata-write guard.

Before enrolling a resource, cover:

- Verified create/update payload support, GET/list persistence, SDK variants,
  key constraints, entry limits, and complete-map replacement.
- Every write path, including execution with a different binary version from
  the one that generated a saved plan.
- Unchanged namespace ownership, protection, deletion/pruning scopes, and
  management label writes during the write-only stage.
- No-op reconciliation, dump/reload, canonical-map limits, and SDK field loss.
- Credential and other write-only field preservation using behavioral or
  equivalent backend evidence, beyond a redacted GET response alone.

The initial adoption covered 14 AI Gateway child families, including consumer
credentials. Current registrations also include Kong Identity families; the
registry is authoritative for enrollment. Adding this capability never opts
other resource families into a blanket migration.

## Create-only provenance with inherited lifecycle

Some credential APIs retain management metadata at creation but ignore later
map changes. `ManagedByPolicy.CreateOnly` allows explicit enrollment without
requesting a metadata update during a later plan. Existing mutable policies
keep normal create/update stamping. This option does not authorize adoption,
metadata replacement or independent child protection.

`PersistsRef` also supports resources registered with `WithNamespaceFrom`.
For those children, the ref is retained alongside tool/version without copying
parent namespace/protection into the child's map. `CreationValues` can add fixed
markers for creation intent that public fields cannot otherwise reveal.
Capacity checks include every generated entry. Fixed markers cannot override
tool, version, ref, namespace or protection, and registration clones their map.

Create requests still stamp the executing version. Update adapters for such an
API must not call `stampManagedByRequest` or send a management map: the generic
request helper does not infer the operation from an SDK struct. The stored
version records issuance rather than later public-field changes. A fresh
observation must verify unchanged metadata after each supported update.

Parent-derived lifecycle authority needs full resource integration: declared
managed parents, current provisioning/namespace/protection checks, exact key
identity, duplicate and incompatible-marker handling, explicit sync scopes,
saved-plan validation and authorized child-before-parent retirement. A tool/ref
marker alone does not authorize a mutation. Imported or legacy ownership shapes
must not be silently adopted. Generated Kong Identity principal/group keys use
this contract with four immutable entries: tool, creator version, persisted
ref and `kongctl-create-type: v1`. Their full implementation verifies current
parent authority on saved writes, hydrates omitted expiration, captures each
new key privately and orders replacement capture before old-key retirement.
The [grouped access example][identity-workflows]
and [compatibility tracker](kong-identity-compatibility.md) describe supported
updates, explicit collection authority and imported/legacy boundaries.

## Stage two: migrate lifecycle authority

Make ownership migration a separate, resource-specific change. Define the
namespace and protection fields, their reserved-key and capacity rules, and
the transition from existing labels before changing lifecycle decisions.
Do not infer independent ownership from `managed_by.tool`, version, source
defaults or a parent alone. An inherited-lifecycle family must enforce its
explicit managed-parent and resource-identity contract.

Specify explicit adoption and transfer procedures, handling for conflicting
label/metadata ownership, and compatibility with old binaries and saved plans.
Avoid accepting whichever representation happens to authorize a mutation.
During the write-only stage labels remain authoritative; after the reviewed
handoff, the resource's managed_by ownership contract becomes authoritative.
Stop writing and consulting kongctl management labels only at that handoff.
Document whether historical labels are retained temporarily or cleaned up.

Verify execution-time ownership and protection, scoped pruning, replacements,
cross-owner dependencies, and cascades before shipping. A metadata backfill must
preserve credentials and public labels and obey the existing owner. Provide user
migration examples, contributor guidance, live evidence, and regression coverage
in each resource PR.

AI Gateway models already have independent ownership through the separate
`WithIndependentChildOwnership` capability. Their namespace/protection keys are
additional to tool/version; all four are generated by kongctl. The contract is
documented in
[AI Gateway child ownership](../ai-gateway-child-ownership.md).

Kong Identity directories are managed roots whose lifecycle authority starts
in `managed_by`. They register namespace participation followed by
`WithManagedBy(ManagedByPolicy{MaxEntries: 5, OwnsLifecycle: true})`.
`UsesManagedByOwnership` distinguishes these roots from resources that only
write provenance. This retains the same four generated keys used by owned AI
Gateway children without enrolling directories in child scope capture or
protection inheritance. Directory adapters check `tool: kongctl` and their
own namespace/protection after a fresh GET before writes. Matching uses the
API's unique name, and unowned directories require explicit adoption.

Kong Identity principals enroll independent child ownership under directories,
with `ManagedByPolicy{MaxEntries: 5, PersistsRef: true}`. Their fifth generated
key, `kongctl-ref`, is required because display names are not unique. The
shared stamper retains this key only for explicitly enrolled policies; it does
not add it to AI Gateway children or directories. Principal adapters recheck
the directory/namespace/ref tuple, complete-list duplicates, and protection
before mutations. Full PUT uses a fresh read to preserve omitted public fields.
See the [Identity compatibility tracker](kong-identity-compatibility.md) for
Q9–Q11 restrictions and the criteria for removing them.

[migration]: https://github.com/Kong/kongctl/issues/2521

[identity-workflows]: ../examples/declarative/identity/README.md
