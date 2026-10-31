# Kong Identity metadata storage

This ledger tracks where KongCTL can persist ownership, namespace, protection,
and local resource identity for the resource families in #1132 and #1133.
API storage capabilities and implemented KongCTL behavior are recorded
separately. A name, UUID, issuer, or key fingerprint identifies a resource;
it does not establish which tool or namespace owns it.

The current source is the public preview SDK at `6ca9d1f8b309`, generated
from the [directory][directory-spec] and [auth-server][auth-spec]
specifications.
Live observations are from US `.com`. Keep this ledger current as each
resource is implemented.

## Implemented and active resource work

| Resource | API metadata fields | KongCTL storage | Verification |
| --- | --- | --- | --- |
| Directory | `labels`, `managed_by` | `managed_by` | Create, GET, list, PUT and live lifecycle pass. |
| Principal | `labels`, `managed_by`, runtime `metadata` | `managed_by`, including local ref | Create, GET, list, PUT, adoption and live lifecycle pass. PATCH metadata gap is Q11. |
| Auth server | `labels`; no `managed_by` | `labels` | Create, GET, list, PATCH, adoption and full live lifecycle pass. Name uniqueness, omission, label null removal and array clearing verified. |
| Auth-server scope | None | None on child; parent namespace/protection | Declarative lifecycle inherits managed parent authority and explicit collection scope. Complete client/claim observations guard deletion. |
| Auth-server claim | None | None on child; parent namespace/protection | Declarative lifecycle, typed scope links and dump supported; ambiguous names fail before mutation. Parent adoption supplies authority. |
| Auth-server OAuth client | `labels`; no `managed_by` | Independent `labels` | Declarative lifecycle, adoption and live credential continuity pass. Namespace/protection are separate from the server. Every PATCH preserves the fresh auth method when omitted (Q15). Initial secret goes only to private capture. #1401 remains In Progress until merge. |
| Auth-server client secret | None | None | Explicit create/capture, get/list metadata, enable/disable and confirmed delete. Two-record limit includes disabled entries (Q16). No independent declarative identity. #1404 is In Progress; initial creation capture is integrated with #1401. |

Directory ownership uses `tool`, `version`, `kongctl-namespace`, and
`kongctl-protected`. Principal ownership adds `kongctl-ref` as the fifth entry.
Client-side filtering and duplicate checks use the complete remote collection.
Public labels and principal runtime metadata retain their API meanings.

Auth-server label ownership follows the existing `KONGCTL-namespace` and
`KONGCTL-protected` convention. It does not gain a `managed_by` field through
the preview SDK. An absent protection label means false. The implementation
is tracked in #1400 and remains In Progress until merged to main.

## Remaining resources with API metadata storage

These rows describe available API fields, not completed implementations.

| Resource | API storage | Planned direction and unresolved limits |
| --- | --- | --- |
| Principal identities, all four variants | `labels`, `managed_by` | Create, GET, list and PUT retain all five ref/ownership values in live probes, including API Gateway consumer response type. Get/list is implemented; declarative stamping/adoption remain #1416. PATCH metadata is rejected. Q21 requires fresh collection-membership proof; Q22 corrects cursor requests. |
| Principal Groups, including team subtype | `labels`, `managed_by` | Prefer the principal ownership pattern after live checks. Import-owned teams require separate treatment. #2657. |
| Generated principal and group API keys | `managed_by`; no labels | Persist ref/ownership only after create and update retention are verified. Generated-secret capture is separate. #1417, #2659. |
| Imported principal and group API keys | Read/update expose `managed_by`; imported create lacks it | No atomic ownership stamp at creation in the inspected body. Q4 tracks the gap; do not equate a later PATCH with atomic creation. #1417, #2659. |
| Basic-auth username records | `managed_by`; no labels | Possible persisted ref/ownership after live contract checks. Passwords have their own metadata. #1418. |
| Basic-auth passwords | `managed_by`; no labels | Possible persisted ref/ownership. Write-only secret handling and password-slot limits also govern support. #1418. |

Presence in a response does not establish that every create/update variant can
write the field. Record that distinction when testing each endpoint.

## Resources without an ownership metadata field

None of these resources has `labels` or `managed_by` in its inspected request
and response shapes. KongCTL currently stores no independent namespace, ref,
or protection metadata on them.

| Resource | Identity or parent | Implementation boundary |
| --- | --- | --- |
| Auth-server scopes | Unique name within auth server | Inherit managed parent namespace and update/delete protection. Explicit collection defines sync authority; fresh references guard deletion. #1402. |
| Auth-server claims | Name filter and generated ID; duplicate names accepted | Same parent/collection authority; duplicate matching fails, including dump. Scope links stay within the parent. Q12–Q14, #1403. |
| Auth-server client secrets | Generated secret ID under client | Explicit lifecycle with private file capture; metadata-only reads, enable/disable and confirmed delete. No declarative secret-slot name. #1404. |
| Group memberships | Member type/ID under group | Relationship authority comes from an explicit collection contract. Import-managed memberships remain distinct. #2658. |
| Token Vault public keys | Directory; generated `kid` fingerprint | Name is present, but team guidance establishes uniqueness for `kid`. Ownership and protection require an explicit contract. Q6, #2661. |
| Token Vault Credential Providers | Unique name per directory | Matching is possible, but no independent tool/namespace/protection stamp exists. Q6, #2660. |
| Shared provider credentials | Generated ID under provider | Secret/enrollment lifecycle and multiplicity need verification. Q6–Q7, #2662. |
| User-enrolled provider credentials | Principal/provider association | Inspect and explicitly revoke where supported; do not treat visibility as administrative ownership. #2663. |
| Directory Trusted IdPs | Generated ID; display name not unique | No independent ownership stamp or verified unique human moniker. Q6, #2664. |
| Org-chart import operation | Directory plus CSV import contract | The import request has no metadata field. Its output principals/groups can expose metadata, but remain subject to importer ownership. Q8, #2665. |
| Runtime authentication and lookup responses | Authenticated principal/credential | Runtime data, not a new declarative resource or ownership store. |

Inherited ownership is not metadata persisted on a child. Scopes and claims
require a declared managed server; its namespace and update/delete protection
apply to child writes. An explicit collection is authoritative for sync;
omission preserves it. Adoption is through the parent, and external parent
references do not authorize these writes. Other rows remain proposed where
noted. A parent with ownership fields alone does not establish collection
authority for another resource family.

## Change log

- Initial directory/principal implementation: metadata persisted in
  `managed_by`; committed through `c1e3cba3` on `ki-triage`.
- Auth-server contract probe: conventional ownership labels retained on create
  and PATCH, with null-based individual label removal. Probe resources were
  deleted with `force=false`. Declarative lifecycle, read commands, adoption,
  example and full live scenario are implemented under #1400 on `ki-triage`.
- Rows without verification reflect inspected request/response schemas and
  existing API questions; live retention is pending for those families.
- Auth-server child probes, October 31 UTC: scopes and claims expose no
  ownership fields. Scope names reject duplicates; claim names allow them.
  Deleting a scope silently detaches client and claim associations. Get/list
  commands now inspect both families, including duplicate-safe UUID lookup.
  Declarative child lifecycle now uses the managed parent plus explicit
  collection authority. Independent child adoption is unavailable because no
  ownership field exists; adopt the parent. Saved writes recheck its identity,
  namespace and protection. Claim/principal creation does not replay uncertain
  POST outcomes; Q14 tracks the API idempotency contract.
- OAuth clients/secrets, October 31 UTC: conventional client labels persist
  through direct HTTP create, reads and partial updates; the client name is
  unique per server (duplicate POST returns 409). Namespace/protection writes
  now have independent declarative ownership and live lifecycle coverage. The
  new commands inspect client configuration and project
  secrets onto a type containing only ID, enabled state and timestamps.
  Client pagination traverses terminal pages because totals exceed client
  counts with multiple scope links (Q18). Complete observations also guard
  scope deletion.
  Q15 records the unrelated authentication-method reset on PATCH, Q16 the
  two-record secret limit, and Q17 unavailable authorization-code grants.
- Explicit secret lifecycle: creation reserves a new `0600` file before POST,
  persists issued material and record IDs only there, and returns metadata to
  normal output. Enable/disable and delete select explicit record IDs and do
  not add remote ownership fields. Private capture metadata identifies the
  credential; it is not a tool/namespace/protection stamp on the API resource.
  Live rotation verifies actual authentication, disabled/deleted rejection,
  imported material and two-slot limits. Q19 tracks uncertain-create recovery.
- Declarative OAuth clients: labels store independent namespace/protection,
  including beneath external servers. Complete local matching uses unique
  client names and rejects unowned or foreign records before mutation. Saved
  writes recheck parent identity and client identity/ownership/protection.
  Initial creation reserves private capture before any mutations; only those
  local files contain generated credentials. Local capture metadata is not a
  substitute for remote client labels or ownership on secret records. Client
  namespace scopes do not widen auth-server pruning. Scope detachment/deletion
  dependencies use the actual explicit PATCH list and fresh reference checks.

- Principal-identity verification: all four variants generate IDs and retain
  labels/managed_by on create and PUT. This verifies a future ownership store;
  current product support is inspection, with no automatic metadata writes.
  Scoped reads prove collection membership because detail/delete paths accept
  another existing principal (Q21). The SDK fixes bracketed cursor requests
  for directories, principals and identities (Q22); future resource list
  implementations must verify their own generated wire parameters.

See the [API-only question log](../kong-identity-api-questions.md) for team
follow-up and the [compatibility tracker](kong-identity-compatibility.md) for
temporary implementation restrictions and their removal criteria.

[directory-spec]: https://github.com/Kong/platform-api/blob/98137d7be3772b67693cc1ceb0469e77f8ecf0f5/src/konnect/definitions/kong-identity/v2/openapi.yaml
[auth-spec]: https://github.com/Kong/platform-api/blob/98137d7be3772b67693cc1ceb0469e77f8ecf0f5/src/konnect/definitions/sliver/v1/openapi.yaml
