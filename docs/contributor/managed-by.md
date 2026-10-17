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

Use this path for every create/update, including replacements, saved plans,
direct state calls, and secret-only updates. Plan previews show the planning
binary version; execution records the executing binary version. Local builds
without release information record `dev`.

`PrepareManagedBy` preserves unrelated entries in the supplied map. It does
not fetch remote state or establish the API's update semantics. The resource
adapter must preserve the complete intended metadata map, including unrelated
remote entries where required, and verify credential/omission contracts. A
capacity error must fail without discarding entries or mutating the request.
The API-specific limit includes the two generated entries.

Ignore tool/version differences when comparing desired and observed metadata:
they describe the last actual write. A binary upgrade alone must not cause a
write, trigger credential rotation, or force an update to a protected object.
The shared planner helper `managedByValuesEqual` implements that comparison;
resource adapters must use it where they compare metadata. Unrelated metadata
and payload differences must still be detected.

Before enrolling a resource, cover:

- Verified create/update payload support, GET/list persistence, SDK variants,
  key constraints, entry limits, and unrelated metadata preservation.
- Every write path, including execution with a different binary version from
  the one that generated a saved plan.
- Unchanged namespace ownership, protection, deletion/pruning scopes, and
  management label writes during the write-only stage.
- No-op reconciliation, dump/reload, metadata overflow, and SDK field loss.
- Credential and other write-only field preservation using behavioral or
  equivalent backend evidence, beyond a redacted GET response alone.

Current enrollment covers the 14 previously supported AI Gateway child
families, including consumer credentials. Config stores, their secrets, data
plane certificates, and non-AI Gateway resources are not enrolled by this PR.
This is the initial adoption of a reusable facility, not a blanket migration.

## Stage two: migrate lifecycle authority

Make ownership migration a separate, resource-specific change. Define the
namespace and protection fields, their reserved-key and capacity rules, and
the transition from existing labels before changing lifecycle decisions.
Do not infer ownership from `managed_by.tool`, version, source defaults, or a
parent resource.

Specify explicit adoption and transfer procedures, handling for conflicting
label/metadata ownership, and compatibility with old binaries and saved plans.
Avoid accepting whichever representation happens to authorize a mutation.
During the write-only stage labels remain authoritative; after the reviewed
handoff, the resource's managed_by ownership contract becomes authoritative.
Stop writing and consulting kongctl management labels only at that handoff.
Document whether historical labels are retained temporarily or cleaned up.

Verify execution-time ownership and protection, scoped pruning, replacements,
cross-owner dependencies, and cascades before shipping. A metadata backfill
must preserve credentials and unrelated metadata and obey the existing owner.
Provide user migration examples, contributor guidance, live evidence, and
regression coverage in each resource PR.

AI Gateway models already have independent ownership through the separate
`WithIndependentChildOwnership` capability. Their namespace/protection keys
are additional to tool/version, so their five-entry limit leaves one unrelated
entry. That existing contract is documented in
[AI Gateway child ownership](../ai-gateway-child-ownership.md).

[migration]: https://github.com/Kong/kongctl/issues/2521
