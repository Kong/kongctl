# Kong Identity metadata storage

This ledger tracks where KongCTL can persist ownership, namespace, protection,
and local resource identity for the resource families in #1132 and #1133.
API storage capabilities and implemented KongCTL behavior are recorded
separately. A name, UUID, issuer, or key fingerprint identifies a resource;
it does not establish which tool or namespace owns it.

The current source is the public preview SDK at `bf1d470c108c`, generated
from the [directory][directory-spec] and [auth-server][auth-spec]
specifications.
Initial live observations are from US `.com`; the October 31 `.tech`
revalidation below distinguishes deployment-specific behavior. Keep this
ledger current as each resource is implemented.

## Implemented and active resource work

| Resource | API metadata fields | KongCTL storage | Verification |
| --- | --- | --- | --- |
| Directory | `labels`, `managed_by` | `managed_by` | Create, GET, list, PUT and live lifecycle pass. |
| Principal | `labels`, `managed_by`, runtime `metadata` | `managed_by`, including local ref | Create, GET, list, PUT, adoption and live lifecycle pass. PATCH metadata gap is Q11. Deletion requires explicit authorized identity deletions, then a fresh empty collection (Q24). |
| Principal identities, all four variants | `labels`, `managed_by` | `managed_by`, including local ref | Declarative lifecycle, scoped get/list, adoption and dump pass live verification. Parent ownership is independent. Q21 requires collection membership proof. Workspace writes/adoption are deferred under Q25; Q26 governs associated-resource deletion effects. |
| Principal API-key inspection | `managed_by`; no labels | Read-only in KongCTL | Generated create retention is verified by direct probes, but PATCH ignores management changes. Scoped get/list exposes public metadata only. No ownership writes/adoption are exposed (Q4/Q27–Q30). |
| Group/team inspection | `labels`, `managed_by`, runtime `metadata` | Read-only in KongCTL | Scoped get/list preserve returned fields and verify complete directory membership. Direct probes retain four ownership entries, but product stamping/adoption is not exposed yet. #2657. |
| Direct group membership inspection | None | None | Scoped entry-ID get/list distinguish the relationship from its member. No product mutation or inherited/imported authority is implied. #2658, Q32. |
| Auth server | `labels`; no `managed_by` | `labels` | Create, GET, list, PATCH, adoption and full live lifecycle pass. Name uniqueness, omission, label null removal and array clearing verified. |
| Auth-server scope | None | None on child; parent namespace/protection | Declarative lifecycle inherits managed parent authority and explicit collection scope. Complete client/claim observations guard deletion. |
| Auth-server claim | None | None on child; parent namespace/protection | Declarative lifecycle, typed scope links and dump supported; ambiguous names fail before mutation. Parent adoption supplies authority. |
| Auth-server OAuth client | `labels`; no `managed_by` | Independent `labels` | Declarative lifecycle, adoption and live credential continuity pass. Namespace/protection are separate from the server. Every PATCH preserves the fresh auth method when omitted (Q15). Initial secret goes only to private capture. #1401 remains In Progress until merge. |
| Auth-server client secret | None | None | Explicit create/capture, get/list metadata, enable/disable and confirmed delete. Two-record limit includes disabled entries (Q16). No independent declarative identity. #1404 is In Progress; initial creation capture is integrated with #1401. |

Directory ownership uses `tool`, `version`, `kongctl-namespace`, and
`kongctl-protected`. Principal and identity ownership add `kongctl-ref` as the
fifth entry.
Client-side filtering and duplicate checks use the complete remote collection.
Public labels and principal runtime metadata retain their API meanings.
Normal usage examples resolve the platform-provided `default` directory as
external, so they write no ownership metadata on that directory. Its managed
principals and identities still persist their own five-entry ownership maps.
If default is missing, the separate `directories.yaml` bootstrap example
creates it with four ownership entries under `identity-directory` and enables
KongCTL protection. Subsequent tutorials still use external lookup; their
cleanup preserves the directory. The .tech manual test setup instead matches
new .com provisioning with empty `managed_by`/`labels`, verified by fresh GET.
Runtime `metadata` is not an ownership store. Its integer values have a
verified API precision limit (Q31); exact numeric identifiers should be strings.

Auth-server label ownership follows the existing `KONGCTL-namespace` and
`KONGCTL-protected` convention. It does not gain a `managed_by` field through
the preview SDK. An absent protection label means false. The implementation
is tracked in #1400 and remains In Progress until merged to main.
Q34 asks whether labels are the intended long-term management metadata
contract for auth servers and OAuth clients, or `managed_by` is planned.

## Remaining resources with API metadata storage

These rows describe available API fields, not completed implementations.

| Resource | API storage | Planned direction and unresolved limits |
| --- | --- | --- |
| Principal Groups, including team subtype | `labels`, `managed_by`, runtime `metadata` | Direct POST/GET/list/full PUT retain all four directory-style ownership entries; names are unique per directory. Partial PUT clears omitted maps. Group implementation is in progress, with no product writes yet. Import-owned teams require separate treatment (Q32). #2657. |
| Generated principal API keys | `managed_by`; no labels | Direct create retains all five ref/ownership entries. PATCH ignores changes (Q28); adoption and mutable ownership remain unavailable. Inspection is implemented under #1417, which remains open until merge. |
| Imported principal API keys | Read/update schemas expose `managed_by`; imported create lacks it | Direct create rejects management metadata, and later PATCH ignores it. Q4/Q28 rule out a verified ownership stamp or adoption workaround. #1417. |
| Group API keys, generated/imported | `managed_by`; no labels | US `.tech` list/create/read/PATCH/delete work; generated create retains the supplied map. Imported create rejects it and PATCH ignores changes/removal on both kinds. US `.com` returned 501. No product writes or ownership stamping are exposed yet. #2659, Q33. |
| Basic-auth username records | `managed_by`; no labels | Possible persisted ref/ownership after live contract checks. Passwords have their own metadata. #1418. |
| Basic-auth passwords | `managed_by`; no labels | Possible persisted ref/ownership. Write-only secret handling and password-slot limits also govern support. #1418. |

Presence in a response does not establish that every create/update variant can
write the field. Record that distinction when testing each endpoint.

Further identity probes verify AI-to-API Gateway consumer retargeting through
PUT. Q25 records unverified workspace semantics: two supplied workspace values
returned 500 without changing either consumer association. No valid workspace
fixture was available, so this is not evidence that all workspace values fail.
Q26 records that OAuth-client deletion removes its identity from GET/list,
while API Gateway consumer/control-plane deletion leaves the identity visible.
An ownership stamp on an identity does not prevent these native API effects.

## Resources without an ownership metadata field

None of these resources has `labels` or `managed_by` in its inspected request
and response shapes. KongCTL currently stores no independent namespace, ref,
or protection metadata on them.

| Resource | Identity or parent | Implementation boundary |
| --- | --- | --- |
| Auth-server scopes | Unique name within auth server | Inherit managed parent namespace and update/delete protection. Explicit collection defines sync authority; fresh references guard deletion. #1402. |
| Auth-server claims | Name filter and generated ID; duplicate names accepted | Same parent/collection authority; duplicate matching fails, including dump. Scope links stay within the parent. Q12–Q14, #1403. |
| Auth-server client secrets | Generated secret ID under client | Explicit lifecycle with private file capture; metadata-only reads, enable/disable and confirmed delete. No declarative secret-slot name. #1404. |
| Group memberships | Generated entry ID; member type/ID under group | No independent ownership fields. Manual membership creation, duplicate rejection and scoped reads work; manual team-user removal also works. Group-to-group creation is rejected (Q32). Import-managed authority needs clarification. #2658. |
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

- October 31 UTC, principal active: optional public state now supports
  declarative create/update/dump with presence-aware verification. Ownership
  remains in the same five-entry managed_by map. Active omission preserves
  state during adoption/protection updates. Developer-portal deactivation is
  rejected; setting false can change provenance as documented by the API.


- October 31 UTC, native `.tech` revalidation: active state is observable and
  writable, Vault enablement persists, user principals support keys and group
  key operations are available. None changes ownership storage: generated keys
  retain their initial map, imported keys cannot acquire one through tested
  PATCH, and principal/key ownership PATCH still ignores changes. Principal-key
  names are supported but non-unique; group-key names have narrower update
  support. Team creation is rejected in this `.tech` org despite `.com`
  accepting it. Auth-server/client POST accepts an extra `managed_by` field
  without exposing it in the response or subsequent reads; labels remain their
  verified ownership store. Native precision, scoping, expiration omission and
  auth-server PATCH/pagination issues persist. All probe records were removed;
  the manual directory settings were restored.
- October 31 UTC, group/membership inspection: product get/list now expose
  observed metadata without writing ownership. SDK `bf1d470c108c` fixes both
  cursor requests. Complete collection proofs scope detail reads; group
  memberships still have no independent namespace/protection store. The
  credential-free scenario uses four-entry group fixture markers, separate
  from production ownership policy. Principal deletion removes its links in
  direct API tests; future lifecycle work must account for those relationships.
- October 31 UTC, group contract probes: unique names and full replacement
  retain `managed_by` ownership values, including protection changes. No group
  resource implementation is exposed yet. Memberships have no ownership
  field; deleting a group removes its relationships but retains member
  principals. Q32 records member-type and importer boundaries. Group key
  GET/POST return 501 (Q33), so ownership writes cannot be verified there.
- October 31 UTC, numeric metadata probes: runtime principal/group metadata
  can round large integer digits. This does not change ownership storage in
  `managed_by`, whose values are strings. Q31 tracks API precision; principal
  input and write-result guards prevent silently accepting changed digits.
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

- Principal-identity inspection milestone: all four variants generate IDs and
  retain labels/managed_by on create and PUT. That milestone established an
  ownership store, with inspection but no automatic metadata writes.
  Scoped reads prove collection membership because detail/delete paths accept
  another existing principal (Q21). The SDK fixes bracketed cursor requests
  for directories, principals and identities (Q22); future resource list
  implementations must verify their own generated wire parameters.
- Principal-identity write-foundation milestone: PUT projections preserve public
  labels and every management-map entry for all four variants, but do not
  synthesize ownership or adopt a resource. Direct probes additionally verify
  OAuth-client/API Gateway consumer retargeting and AI Gateway consumer
  metadata retention. Declarative stamping and adoption were still pending.
  Principal delete/sync now refuse attached identities at planning and
  execution, independent of their metadata; Q24 tracks the wider API deletion
  contract and an atomic nonempty-child precondition.
- Declarative identities: generated five-entry maps persist on create, PUT
  and adoption. Namespace/ref matching is independent of principal ownership;
  empty collection scope does not widen principal or directory pruning.
  Unowned principals/identities export as external references. Workspace-bearing
  identities also export externally while Q25 remains unverified. Observed
  client/Gateway references supply deletion dependencies and saved scopes for
  fresh execution checks. Collections outside the declarations are not scanned;
  native API effects described in Q26 remain a limit of metadata protection.
- Principal API-key probes: generated creation retains namespace/ref/protection
  metadata atomically; imported creation rejects it. Both generated/imported
  PATCH ignore management-map changes, including complete maps. Metadata-only
  PATCH also clears an omitted expiration. Q27 records `user` principal 404s,
  Q28 the update behavior, Q29 unavailable names and Q30 runtime mTLS setup.
  Key GET/list omit secret material. No declarative ownership updates or
  adoption are claimed from schema presence alone.

See the [API-only question log](../kong-identity-api-questions.md) for team
follow-up and the [compatibility tracker](kong-identity-compatibility.md) for
temporary implementation restrictions and their removal criteria.

[directory-spec]: https://github.com/Kong/platform-api/blob/98137d7be3772b67693cc1ceb0469e77f8ecf0f5/src/konnect/definitions/kong-identity/v2/openapi.yaml
[auth-spec]: https://github.com/Kong/platform-api/blob/98137d7be3772b67693cc1ceb0469e77f8ecf0f5/src/konnect/definitions/sliver/v1/openapi.yaml
