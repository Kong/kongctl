# Kong Identity contributor guide

Use the [implementation guide][implementation] for engine integration and
[workflow examples][examples] for manifests. Discover fields through
`kongctl explain identity`; the code and tests define supported operations.

## API surfaces and implementation

Directory administration manages principals, identities, groups, credentials,
Trusted IdPs and Token Vault configuration. Auth Server administration manages
issuers, scopes, claims, OAuth clients and client secrets. Runtime
authentication, lookup, token retrieval and consent are separate operations;
administrative CRUD does not verify a runtime integration.

Resources register in `internal/declarative/resources`. Directory and Auth
Server planners dispatch to typed child planners. State clients observe the
API; executor adapters repeat ownership and dependency checks on fresh state.
Identity dump collectors export public declarations and external references.
Keep loading, planning, saved execution and dump consistent when extending a
resource. Metadata visibility alone never grants management authority.

## Matching and ownership

| Resource | Remote identity | Authority storage |
| --- | --- | --- |
| Directory | Unique name | Independent `managed_by` |
| Principal | Directory, namespace, ref | Independent `managed_by` |
| Principal identity | Principal, namespace, ref | Independent `managed_by` |
| Ordinary group | Directory, unique name | Independent `managed_by` |
| Basic-auth username | Directory-wide username | Independent `managed_by` |
| Group membership | Group and principal UUID | Managed group |
| Auth Server | Unique name | Labels |
| OAuth client | Server and unique name | Independent labels |
| Scope/claim | Server and name | Managed Auth Server |
| Vault key | Directory and public-key `kid` | Managed directory |
| Credential Provider | Directory and unique name | Managed directory |
| Static shared credential | Provider and default/priority slot | Directory |
| Trusted IdP | Directory and exact issuer URL | Managed directory |

Independent maps contain `tool`, `version`, `kongctl-namespace` and
`kongctl-protected`. Principals and identities also persist `kongctl-ref`
because display names are not unique. Basic-auth refs stay local. Labels and
public `metadata` are separate from tool ownership. Auth Server/client labels
use the existing reserved `KONGCTL-*` convention.

Generated API keys have an immutable creation convention: `tool`, `version`,
`kongctl-ref` and `kongctl-create-type`. Managed principal/group keys inherit
current parent authority. Keys under external principals add a fifth entry,
`kongctl-namespace`, and own only that namespace/ref within the parent UUID.
They cannot add protection as a sixth entry, adopt existing keys, or transfer
ownership. The deployed key PATCH accepts but ignores management-map changes.

List observations must follow every cursor before local matching or pruning.
Duplicate matches stop planning and execution. Saved writes recheck parent
scope, UUID, ownership and protection. Explicit child collections scope sync;
omission preserves collections. Parent deletion requires separately authorized
child retirements and fresh empty-dependency checks, never an implicit cascade.

## Compatibility boundaries

- Hydrate omitted fields from fresh GET responses before full PUTs. Principal
  type is currently create-only. Preserve missing `active` rather than
  inventing a value; developer-portal principals cannot be deactivated.
- Principal `metadata_mode: merge` preserves unmentioned values at execution;
  `sync` replaces the map. Merge intent requires plan format 1.2. A declared
  principal UUID asserts an existing namespace/ref match; it is not adoption.
- Imported principals remain external in declarative configuration. PUT can
  retain importer-controlled name/description while updating public maps, but
  that does not establish the full update/delete contract. Native imports can
  change provisioning and reporting relationships; do not treat retained
  ownership metadata as permission to reconcile the principal.
- Native org-chart import processes a complete roster and can deactivate
  absent people. It does not stamp KongCTL ownership. `!csv` is a separate
  input frontend for ordinary managed declarations. Repeated import preserves
  principal/key UUIDs but can recreate the org-root identity marker.
- Identity association variants are custom, OIDC, Auth Server client and
  control-plane Consumer. Workspace settings and importer-owned teams/nested
  group writes are outside the supported declarative contract.
- Basic-auth passwords, client-secret rotation, OAuth enrollment and user
  Vault authorizations use explicit commands where supported. Secret capture
  and administrative status do not prove consent or runtime authentication.
  Inspection must avoid detail GETs that issue new enrollment URLs.
- Vault is a directory capability, not an independent resource. Enable an
  existing Vault before observing its children. Key/provider/credential/IdP
  writes need an explicitly managed directory. Static secret updates require
  explicit write selection; dumps never include secret material.

Change compatibility guards only with a focused regression test and direct
API evidence for the complete intended lifecycle. SDK request fields alone do
not establish deployed behavior. Keep serialization corrections reproducible
in `tools/sdk-patches` and use `go.mod` as the SDK version authority.

## Testing

Use loader, planner, executor and dump tests for scope, duplicates, stale plans
and nullable/omitted fields. Credential tests must verify private capture,
no-op reapplication, public updates, replacement ordering and safe retirement.
Use E2E scenarios for complete CLI workflows, including import and external
principal keys. Cover supported apply, sync, delete and dump behavior rather
than checking only creation.

Identity E2E scenarios reuse `default` and preserve its UUID. Their
`useDefaultDirectory: true` fixture resets directory contents before and after
each scenario; use dedicated test organizations. Directory CRUD beyond that
single slot requires `KONGCTL_E2E_IDENTITY_MULTI_DIRECTORY=1` and extra quota.
Reset retires children in dependency order, releases imported users for
cleanup, and restores the default TTL/access/Vault settings. Preserve enabled
replay coverage when changing scenarios; see [replay policy][replay].

[implementation]: declarative-resource-implementation-guide.md
[examples]: ../examples/declarative/identity/README.md
[replay]: ../../test/e2e/replay-policy.md
