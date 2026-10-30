# Shared AI Gateway ownership infrastructure

Models, providers, auth strategies, policies, custom policy definitions,
agents, MCP servers, vaults, consumers, consumer groups, CA certificates, and
SNIs support independent kongctl namespace ownership. Runtime certificates
remain deferred pending a credential-preserving metadata PATCH API. Config
stores, their secrets, and data plane certificates remain excluded.

See the [release scope and ownership setup guide](ai-gateway-federation.md),
[model ownership and setup][models],
[provider ownership](ai-gateway-provider-ownership.md),
[auth-strategy ownership](ai-gateway-auth-strategy-ownership.md),
[policy ownership](ai-gateway-policy-ownership.md),
[custom definition ownership](ai-gateway-custom-policy-ownership.md),
[agent ownership](ai-gateway-agent-ownership.md), and
[the remaining six families](ai-gateway-remaining-child-ownership.md).

## Ownership and collection scope

Every AI Gateway child API that supports `managed_by` is stamped on create or
update with `tool: kongctl` and `version: <running binary version>` (`dev` for
local development builds). Saved plans use the executing binary's version at
write time. These provenance fields do not establish ownership or enable
federation. A version change alone does not schedule a write.

Writing is explicitly registered through `WithManagedBy` and each resource's
`ManagedByPolicy`. The shared planner and state request helper also support
later non-AI Gateway enrollment. See the
[incremental migration guide](contributor/managed-by.md) for write-only
enrollment alongside existing management labels and the later ownership step.

Participating children also persist `kongctl-namespace` and `kongctl-protected`
in `managed_by`. Their explicit namespace overrides the source file default,
then the literal `default` namespace. Gateway metadata supplies neither
ownership nor protection.

`managed_by` is reserved for Kong tools and cannot be set in declarative
configuration. Use `labels` for user metadata and `kongctl` for ownership and
protection. Kongctl replaces the complete map with its generated keys; existing
outside entries are removed. Metadata drift appears in plans and diffs even when
no public payload changes. A version-only difference does not schedule a write;
required writes record the executing binary's version. Public labels remain
preserved unless explicitly changed.

Protected children must be unprotected separately before other writes.
Unprotection may normalize the tool-owned map but cannot change namespace,
public payload, or secrets. Dumps omit the entire `managed_by` map and express
child ownership/protection through `kongctl`, so they remain reloadable.

Consumer credentials cannot be updated in place. Metadata-only drift produces a
warning showing the old and canonical map, without deleting or recreating the
credential. Replacement remains explicit. Runtime certificate metadata writes
still require a selected private key; unsafe AppRole writes remain blocked.
These API limitations also apply to metadata reconciliation.

Dump warns and skips individual children with invalid ownership metadata,
preserving valid siblings. It does not interpret invalid protection values as
unprotected.

New children receive generated ownership metadata when kongctl creates them.
An existing unowned name match requires explicit adoption through the API. A
match owned by another namespace requires explicit transfer; a child already
owned by the intended namespace needs neither operation. Defaults and
references never adopt an observed child. Names are unique per actual gateway
and family. See [ownership setup for existing resources][existing] for the
setup checklist.

Apply does not prune. Sync scopes each participating collection by source,
namespace, and gateway. Omission leaves the collection untouched. An explicitly
empty collection scopes only that source's namespace. Multiple source files
and gateway aliases merge their scopes without expanding gateway pruning.

Direct `delete -f` against an external gateway deletes only declared,
unprotected children owned by the source namespace, across all participating
families. It does not prune undeclared siblings. Missing declarations are
ignored; foreign and unowned name matches fail. Excluded child declarations
are retained with a warning. Deleting a managed gateway itself uses the cascade
contract below.

Dependency checks use complete gateway observations and the entire final plan.
A retained foreign or unowned user blocks direct deletion of a referenced
child. Planned detachments and user deletions precede target deletion, and new
referenced resources precede the writes which attach them. Same-name
replacements
delete existing users before
deleting/recreating their target and creating replacement users. Updating a
retained user to the same target name does not establish detachment. Saved plans
retain
these ordering constraints across namespaces and aliases.

## Gateway deletion

The confirmed maintainer contract allows the gateway owner to delete the whole
gateway, including foreign, unowned, and protected descendants. Planner warnings
show affected families, namespaces, names, counts, and protected descendants.
Warnings remain in saved plans and are displayed during execution. Protected
descendants warn rather than veto a parent cascade. The gateway's own ownership
and protection remain enforced; a nonowner cannot obtain cascade authority by
referencing it. There is no additional confirmation flow.

Live saved-plan validation rejected a nonowner namespace and gateway protection
added after planning, then allowed the owner to cascade through foreign,
unowned, and protected models with the original warnings. All disposable
validation resources were removed.

Direct child deletion still requires the child's ownership and unprotected
state. To unprotect a child, apply only that metadata change first; do not
combine it with payload mutation or secret rotation.

Existing group-side membership behavior is unchanged: the group owner manages
the complete optional member set, including foreign consumers. Credentials
inherit their consumer's independently owned namespace and lifecycle.

## Credentials and remaining validation

Unselected secrets in object arrays no longer remove the entire array. Public
header names, parameter names and locations survive while write-only values
are omitted. Scalar secret arrays still omit as a complete group; mixed
selection is rejected. Scalar secrets and nested object secrets continue to
omit only their selected paths. No credentials are implicitly rewritten or
embedded in plans or logs.

A disposable live runtime probe verified provider basic-header credential
continuity across a CLI metadata-only PUT. A local upstream received the
expected synthetic Authorization credential before the update and the same
credential afterward, without selecting or resolving the manifest's `!secret`
source. The post-update check used a fresh runtime with an empty local config
cache, synchronized to Konnect's expected config version. The public header
definition and updated metadata were also retained. Credentials were compared
in memory and were not printed or embedded in plans.

Provider basic-header continuity is verified behaviorally. The provider
integration also verifies identical parameter values, Azure client secrets,
AWS keys/session tokens and GCP service-account JSON in fresh runtimes after
metadata-only protection and unprotection PUTs. See [provider evidence and
limits](ai-gateway-provider-ownership.md). Retaining a redacted GET shape
alone is insufficient. Auth-strategy OIDC client-secret arrays and proxy
authorization values were also compared in fresh synchronized runtimes after
protection and unprotection PUTs; see [auth-strategy evidence and
limits](ai-gateway-auth-strategy-ownership.md). MCP and vault evidence,
including the AppRole restriction, is recorded in the
[remaining-family guide](ai-gateway-remaining-child-ownership.md).

Provider auth parameter values and AWS session tokens now use the same
reviewed `!secret` declaration machinery as header values. Public parameter
names and locations remain in unselected PUT payloads; parameter values are
write-only, including values such as an API version. Declare them through a
deferred source and select them explicitly when changing them.

Provider basic header/parameter arrays, Azure client secrets, AWS secret access
keys/session tokens, and GCP service-account JSON use distinct write-only
shapes. Auth-strategy scalar client-secret arrays must be selected as a
complete group when rotating them. Vault credentials retain the verified
restrictions described in the
[remaining-family guide](ai-gateway-remaining-child-ownership.md). Runtime
certificate keys still require a credential-preserving metadata operation
before federation. Certificate key writes remain explicit
and required for current certificate updates; this infrastructure does not
invent a preserve-on-omission contract for them.

## API limits

Ownership is cooperative client behavior, not server-enforced authorization.
List endpoints lack ownership filters: planning reads each complete gateway
collection once per family and run, then filters reconciliation views only.
Pagination errors, missing responses, malformed next links and cursor cycles
fail planning rather than yielding destructive partial observations.

Lists are not guaranteed snapshots during concurrent changes. Execution-time
ownership and protection checks reduce stale-plan risk but cannot close the
check/write race. The model foundation observed no usable ETag enforcement.
Server-side filtering and conditional ownership/version writes remain API
follow-ups. Gateway cascade warnings describe planning-time observations;
concurrent changes can alter the descendants before the delete request.

[models]: ai-gateway-model-ownership.md
[existing]: ai-gateway-federation.md#ownership-setup-for-existing-resources
