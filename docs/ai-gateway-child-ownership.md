# Shared AI Gateway ownership infrastructure

Models currently support independent kongctl namespace ownership. The shared
infrastructure prepares other mutable children for separate resource PRs; it
does not enable provider, consumer, or group federation. Config stores and data
plane certificates remain excluded. See [model ownership and migration][models]
for the supported manifest and explicit
adoption
workflow.

## Ownership and collection scope

Participating children persist `kongctl-namespace` and `kongctl-protected` in
`managed_by`. Their explicit namespace overrides the source file default, then
the literal `default` namespace. Gateway metadata supplies neither ownership
nor protection. Raw reserved entries are rejected. Other tools' metadata is
preserved, including entries added after planning. The merged map may contain
at most five entries, including the two reserved entries; overflow fails before
a write rather than discarding metadata.

An unowned name match requires explicit adoption through the API. A match owned
by another namespace requires explicit transfer. Defaults and references never
adopt an observed child. Names are unique per actual gateway and family.

Apply does not prune. Sync scopes each participating collection by source,
namespace, and gateway. Omission leaves the collection untouched. An explicitly
empty collection scopes only that source's namespace. Multiple source files
and gateway aliases merge their scopes without expanding gateway pruning.

Dependency checks use complete gateway observations and the entire final plan.
A retained foreign or unowned user blocks direct deletion of a referenced
child. Planned detachments and user deletions precede target deletion, and new
referenced resources precede the writes which attach them. Same-name replacements
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

Existing consumer group-side membership behavior is unchanged. Broader
consumer/group ownership integration belongs to the remaining-child workstream.

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

Live evidence covers provider basic-header credential continuity only. Other
write-only shapes still need their own evidence. Retaining a redacted GET
shape alone is insufficient.

Inspection also identified catalog gaps: provider auth parameter values and AWS
session tokens currently lack reviewed `!secret` path capabilities. Their safe
write/preservation contract must be added and validated in the provider
integration; this PR does not expand that resource's secret declarations.

Provider basic header/parameter arrays, Azure client secrets, AWS secret access
keys/session tokens, and GCP service-account JSON use distinct write-only
shapes. Auth-strategy scalar client-secret arrays, vault credentials, and
runtime certificate keys need their resource-specific omission/replacement
contracts verified before federation. Certificate key writes remain explicit
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
