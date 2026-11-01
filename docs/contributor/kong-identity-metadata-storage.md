# Kong Identity metadata storage

This ledger tracks where KongCTL can persist ownership, namespace, protection,
and local resource identity for the resource families in #1132 and #1133.
API storage capabilities and implemented KongCTL behavior are recorded
separately. A name, UUID, issuer, or key fingerprint identifies a resource;
it does not establish which tool or namespace owns it.

The current source is the public preview SDK at `2f51ede0670e`, generated
from the [directory][directory-spec] and [auth-server][auth-spec]
specifications, plus two read-only Registry inventory operations.
Initial live observations are from US `.com`; the October 31 `.tech`
revalidation below distinguishes deployment-specific behavior. Keep this
ledger current as each resource is implemented.

## Implemented and active resource work

| Resource | API metadata fields | KongCTL storage | Verification |
| --- | --- | --- | --- |
| Principal CSV declarations | No additional API field | Existing principal/identity/basic-auth/generated-key conventions | CSV is an input frontend to ordinary declarations. Public metadata merge/sync, UUID assertions and source bindings are client intent; no CSV-specific ownership map is stamped. Empty/omitted collections retain normal scope rules. #1419. |
| Shared OAuth credential/enrollment commands | None | No ownership stamp; private local capture context only | Explicit pending create, enrollment-URL issuance and shared retirement verify provider membership without adopting authorizations. Capture files identify directory/provider/credential; URLs stay outside public output and declarations. #2662. |
| Static shared provider credentials | None | Managed directory through declared provider | Declarative default/priority-slot create, explicitly selected PUT, scoped sync/delete and public dump. Ref stays local; no child metadata is stamped. Fresh directory protection, provider mode, routing and encryption-key guards apply. #2662. |
| Vault providers | None | Managed directory | Declarative create/PATCH, scoped sync/delete and public dump are implemented; credential guards and inherited parent protection apply. |
| Provider credential inspection | None | None; read-only | Scoped get/list projects shared routing/status and user principal metadata. Reads select through complete lists and never invoke enrollment-producing detail GET. No metadata is stamped. #2662/#2663. |
| Principal Vault credential inspection | None | None; read-only | Provider/template/status/expiry metadata is projected without tokens. Empty collection is live verified; populated user enrollment/revocation still needs Q30's runtime setup. #2663. |
| Token Vault keys | None | Managed directory | Verified live lifecycle. |
| Directory | `labels`, `managed_by` | `managed_by` | Create, GET, list, PUT and live lifecycle pass. Deletion requires enabled Vault, exact explicitly authorized child retirements and then fresh empty key/provider/Trusted IdP inventories. A directory-only declaration grants no child retirement authority (Q37). |
| Principal | `labels`, `managed_by`, runtime `metadata` | `managed_by`, including local ref | Create, GET, list, PUT, adoption and live lifecycle pass. PATCH metadata gap is Q11. Deletion requires explicitly authorized identity, username-record, membership and generated-key retirements, followed by fresh empty checks. Unmanaged keys block deletion (Q24). |
| Principal identities, all four variants | `labels`, `managed_by` | `managed_by`, including local ref | Declarative lifecycle, scoped get/list, adoption and dump pass live verification. Parent ownership is independent. Q21 requires collection membership proof. Workspace writes/adoption are deferred under Q25; Q26 governs associated-resource deletion effects. |
| Generated principal/group API-key declarations | `managed_by`; no labels | Four immutable creation markers; current managed parent supplies namespace/protection | Declarative generated creation/private capture, updates, scoped sync/delete, saved plans, dump/explain/scaffold and live lifecycle pass. No independent key adoption/protection. Imported create rejects metadata and PATCH ignores map changes (Q4/Q28–Q30). #1417/#2659. |
| Principal/group API-key commands | `managed_by`; no labels | Explicit operations preserve existing maps; no ownership stamp | Generated/imported private capture, complete public reads, expiration updates, principal scalar rename and explicit scoped retirement. Imported/legacy keys stay outside declarative ownership. #1417/#2659. |
| Basic-auth username record | `managed_by`; no labels | Four-entry `managed_by`; unique username matching | Declarative lifecycle, adoption, protection, scoped sync/delete, dump and external UUID/username lookup are implemented. Ref stays local. Full PUT never sends password values; deletion requires empty passwords. #1418. |
| Basic-auth passwords | `managed_by`; no labels | Explicit imperative operations preserve maps; no ownership stamping | Public-only reads, private generated/imported capture, expiration/material changes and explicit deletion. No declarative password ownership/adoption is exposed. Q35 governs metadata-only writes; SDK expiration is corrected (Q36). #1418. |
| Ordinary Principal Groups | `labels`, `managed_by`, runtime `metadata` | Four-entry `managed_by`; unique directory/name matching | Declarative lifecycle, PUT hydration, adoption, protection and dump are implemented. Group metadata stays independent of directory ownership. Members/keys must be removed before deletion. #2657. |
| Teams/imported group inspection | Same fields as groups | Read-only in KongCTL | Returned provisioning/team-manager presence is retained. Team/importer writes/adoption are deferred (Q32). |
| Direct principal group membership | None | Inherits managed ordinary-group authority | No independent stamp or adoption. Match parent/type/member ID, route by entry ID. Explicit collection sync, group protection/adoption, saved references and dump are implemented; team/importer/nested writes remain deferred. #2658, Q32. |
| Auth server | `labels`; no `managed_by` | `labels` | Create, GET, list, PATCH, adoption and full live lifecycle pass. Name uniqueness, omission, label null removal and array clearing verified. |
| Auth-server scope | None | None on child; parent namespace/protection | Declarative lifecycle inherits managed parent authority and explicit collection scope. Complete client/claim observations guard deletion. |
| Auth-server claim | None | None on child; parent namespace/protection | Declarative lifecycle, typed scope links and dump supported; ambiguous names fail before mutation. Parent adoption supplies authority. |
| Auth-server OAuth client | `labels`; no `managed_by` | Independent `labels` | Declarative lifecycle, adoption and live credential continuity pass. Namespace/protection are separate from the server. Every PATCH preserves the fresh auth method when omitted (Q15). Initial secret goes only to private capture. #1401 remains In Progress until merge. |
| Auth-server client secret | None | None | Explicit create/capture, get/list metadata, enable/disable and confirmed delete. Two-record limit includes disabled entries (Q16). No independent declarative identity. #1404 is In Progress; initial creation capture is integrated with #1401. |

Directory and ordinary group ownership use `tool`, `version`, `kongctl-namespace`, and
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

Generated-key declarations use ordinary-parent authority. Their map stores
`tool`, creator `version`, `kongctl-ref` and `kongctl-create-type: v1`;
namespace/protection come from the current managed principal/group. Native and
CLI US `.tech` checks verify marker retention, current parent authority and
scope/deletion guards. These immutable markers differ from the five-entry
independent principal/identity maps. Legacy/unrecognized maps are not adopted.

| Resource | API storage | Planned direction and unresolved limits |
| --- | --- | --- |
| Importer-managed teams | `labels`, `managed_by`, runtime `metadata` | Schema availability does not grant importer mutation authority. Ordinary group management is implemented; team/importer contracts remain deferred (Q32). #2657. |
| Imported principal API keys | Read/update schemas expose `managed_by`; imported create lacks it | Direct create rejects management metadata, and later PATCH ignores it. Q4/Q28 rule out a verified ownership stamp or adoption workaround. #1417. |
| Imported group API keys | `managed_by`; no labels | Explicit commands are implemented. Imported create rejects the map and PATCH ignores changes/removal; declarative ownership/adoption is unavailable. Generated declarations use parent authority as described above. #2659, Q28/Q33. |
| Password declarative ownership | `managed_by`; no labels | Generated/imported create retain supplied metadata. PATCH ignores map changes and clears omitted expiration. Full PUT retains the map but requires explicit material. Explicit commands preserve maps without stamping ownership. Metadata-only adoption and declarative password management remain unavailable (Q35). #1418. |

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
| Group memberships | Generated entry ID; member type/ID under group | Direct principal declarations inherit managed ordinary-group authority, including protection/adoption and explicit collection sync. No independent ownership field. Group-to-group creation is rejected (Q32); team/imported mutations are deferred. #2658. |
| Token Vault keys | Directory + `kid` | Parent ownership; see below. |
| Vault providers | Directory + unique name | Declarative lifecycle inherits managed-directory namespace/protection. Explicit collection authorizes sync; fresh empty credential observations guard deletion. No independent metadata. #2660. |
| Shared provider credentials | Generated ID; default/routing-priority slot under provider | Static declarations inherit managed-directory authority; refs stay local and material replacement is explicit. Shared OAuth uses explicit private capture/reissuance and retirement without metadata stamping. OAuth routing PUT and actual consent remain outside this verification. Q30/Q45–Q47, #2662. |
| User-enrolled provider credentials | Principal/provider association | Public provider/principal inspection is implemented without adopting authorizations. Actual enrollment/revocation needs supported runtime context; it is not established by empty lists. Q30, #2663. |
| Directory Trusted IdPs | Unique issuer URL per directory; generated ID | Declarative lifecycle inherits explicit managed-directory authority/protection, with no child metadata. Public full PUT preserves omitted settings and supports nullable/empty clears. Explicit collections scope sync; associated principals remain independently owned and require proven detachment before retirement. Q37/Q38/Q44, #2664. |
| Org-chart import operation | Directory plus CSV import contract | Explicit preview/execution stamps no KongCTL metadata. New users/teams have empty maps; adopted users can retain old managed_by while becoming imported. Principal management rejects that state and dump uses external UUIDs. Private report context carries directory ID, dry-run mode and an input hash; it is not remote ownership. Q48–Q51, #2665. |
| Runtime authentication and lookup responses | Authenticated principal/credential | Runtime data, not a new declarative resource or ownership store. |
| Principal group inspection | Group ID plus direct/inherited status and nullable membership-entry ID | Scoped get/list projects group type, manager and provisioning fields with complete pagination. It stamps no metadata and grants no authority to mutate inherited/imported relationships. #2658. |
| Registry Agent principal associations | Association UUID; stored directory/principal UUID tuple under an Agent | Read-only global incoming-reference checks before Identity retirement. No metadata is stamped and no Registry mutation/adoption authority is inferred. Planning and fresh execution block on links or unknown inventories. #1414/#1415/#2419, Q53. |

Token Vault public keys now inherit ownership from an explicitly managed
directory. Matching uses the verified RFC 7638 fingerprint, never nonunique
names or a locally stored ref. Adoption is through the parent; external
directories authorize inspection only. A declared collection supplies sync
authority, omission preserves it, and parent protection applies to updates and
deletions. Dump exports keys beneath enabled managed parents only. Directory
deletion can plan exact authorized key removals, then requires fresh empty
key/provider/IdP inventories at execution. Q6 asks about future API ownership
fields; Q37's native cascade/visibility and atomicity limitations still apply.
See [the key tutorial](../examples/declarative/identity/vault-keys.md), #2661.

Credential Providers now inherit explicit managed-directory authority for
declarative create/PATCH, saved plans, scoped sync/delete and dump. Matching
uses the directory's unique provider name; ref is local only. Adoption is
through the directory, and external directory references are read-only for
provider declarations. Neither children nor templates gain metadata fields.
Provider updates/deletions inherit parent protection; creation remains allowed
under a protected parent, matching other inherited children.

Planning requires empty credentials or exact explicitly authorized static
credential retirements. Provider execution still requires an actual empty
collection after those deletions; a directory/provider declaration alone grants
no credential retirement authority.
Exact planned provider retirements may precede directory deletion, whose final
guard still requires actual empty inventories. A late unplanned attachment
cannot authorize a cascade. Q41 tracks the lack of an atomic conditional API
deletion. Deferred client_secret sources use explicit write selection after
creation and never become ownership metadata. Q30 bounds runtime verification;
Q39/Q40/Q42 track replacement, initial enablement and clearing; Q43 separates
static-secret setting persistence from unverified runtime effects. See #2660
and the [provider guide](../examples/declarative/identity/vault-providers.md).

Inherited ownership is not metadata persisted on a child. Scopes and claims
require a declared managed server; its namespace and update/delete protection
apply to child writes. An explicit collection is authoritative for sync;
omission preserves it. Adoption is through the parent, and external parent
references do not authorize these writes. Other rows remain proposed where
noted. A parent with ownership fields alone does not establish collection
authority for another resource family.

## Change log

- November 1, principal CSV: loader expansion and offline structural inspection
  add no remote metadata store. Principals/identities/username records/generated
  keys retain the existing conventions. Public metadata_mode and UUID
  assertions are client-only; saved metadata_merge intent is versioned in plan
  1.2 and execution preserves fresh unmentioned values for a merge. CSV sources,
  private key material and generated output files are not exported by dump.

- November 1, generated principal/group key declarations: immutable four-entry
  provenance/ref/creation-intent maps, parent-derived namespace/protection,
  whole-plan private capture, supported public updates, exact duplicate/ref
  guards, scoped sync and explicit retirement ordering. Dump exports no
  material, capture paths or map. The new live scenario verifies default-style
  external directory lookup, both parent kinds, saved ownership guards,
  expiration omission/null, principal rename, group create-only names,
  foreign/imported/legacy preservation, replacement ordering and parent delete.

- November 1, generated-key declarative preparation: create-only provenance
  and inherited persisted refs are now supported by the metadata engine.
  Fixed creation markers cannot override namespace/protection or tool/ref
  fields; registration checks capacity and clones the configured marker map.
  Key helpers accept pre-reserved captures for whole-plan preflight. Existing
  commands keep their current ownership boundary. Full key resource enrollment
  and its lifecycle checks remain in progress under #1417/#2659.

- November 1, principal/group API-key commands: shared scoped write helpers
  reserve exclusive 0600 captures before a single POST, then verify creation
  and fresh public read-back. Explicit PATCH preserves observed management
  maps and always sends expiration; name-only principal changes hydrate it.
  Group rename and imported group naming are rejected. Deletion requires
  scoped membership and a final absence check. No runtime authentication or
  ownership-transfer guarantee is inferred. SDK remains `90596b498bcb`.

- November 1, org-chart upload/reporting: explicit native full-file preview or
  execution requires a new private report file. Directory ID, dry-run and the
  exact uploaded-byte SHA-256 are local correlation/uncertainty context, not
  remote ownership or a state cache. Normal output has counts and value-free
  notices; full diagnostics remain private. The command does not stamp maps,
  enforce declarative namespace/protection, release imported principals or
  reactivate returning users automatically.

- October 31 US Central, org-chart ownership: native import retains an adopted
  user's UUID and existing managed_by while changing provisioned_by to imported.
  It creates new users/teams with empty maps. Retained markers do not authorize
  ordinary principal management: declarative writes/pruning/adoption reject
  imported state, saved writes recheck it, and dump uses external UUIDs.
  Native import stamps no KongCTL ownership. Explicit deactivation/release is
  separate from declarative configuration. Q48–Q50 capture API contracts.

- October 31 US Central, shared OAuth enrollment: explicit commands create
  pending records, capture/reissue URLs in exclusive private files, and retire
  shared static/OAuth records. They neither stamp ownership nor enforce
  declarative namespace/protection, following other explicit secret commands.
  Private capture IDs are local recovery context, not remote management data.
  Ordinary reads still use only collections. User authorization adoption and
  revocation remain outside this administrative slice.

- October 31 US Central, static shared declarations: namespace/protection and
  adoption derive from the explicitly managed directory, via its declared
  static/shared provider. Default or priority identifies each record; ref stays
  local and no metadata is stamped. Secret writes are deferred and explicitly
  selected after creation. Public dump omits material and source bindings.
  Scoped sync/retirement and saved routing/key/parent guards pass live `.tech`
  CLI verification. OAuth/user authorization ownership remains separate.

- October 31 US Central, static credential write foundations: fresh static/shared
  provider and complete routing inventories guard single-attempt create,
  explicit material replacement and delete. No metadata is stamped. Three
  creates/replacements/deletes and three pre-write guards pass through actual
  helpers in `.tech`, with no credential detail GET. Explicit inherited parent
  authority and deferred secret selection remain declarative integration work.

- October 31 US Central, credential observation: provider/principal get/list
  expose public metadata only, without enrollment-producing detail GET or
  ownership stamping. Q45/Q46 record priority/default-slot identity and PUT
  material/omission behavior. SDK `90596b498bcb` adds three cursor-tag fixes
  from patch source `c775a9c4`; all sixteen wire methods pass. Static shared
  declarations and explicit enrollment/rotation remain subsequent work.

  All twenty-four Identity live scenarios pass with the new SDK, including
  real provider credential paging and public-only artifacts. Repository gates,
  early-stop cleanup and final empty inventory checks pass. Populated user
  credential shapes have unit coverage; live user enrollment/revocation is
  still outside this verification.

- October 31 US Central, Trusted IdP declarations: inherited directory ownership,
  exact issuer matching, public hydrated PUT, scoped sync/delete, saved guards
  and public dump/reload are integrated. Cross-namespace independent OIDC
  retargeting/deletion can precede retirement. No principal metadata is
  stamped by trust configuration. All twenty-three Identity live scenarios and
  final isolated lifecycle/early-stop cleanup pass in `.tech`. Repository gates
  pass; #2664 stays open/In Progress until main merge.

- October 31 US Central, Trusted IdP write foundations: create, hydrated public PUT
  and guarded delete are verified in `.tech`. No metadata is stamped and no
  principal ownership is implied. Full declarative integration is in progress
  under #2664, using explicit managed-directory authority. Q44 records native
  deletion leaving associated principals and identities readable.

- October 31, Credential Provider declarations: directory-inherited namespace,
  protection and adoption, explicit collection sync, public dump, saved writes
  and credential-aware retirement are implemented without child metadata.
  Secret-only updates carry verified parent ID/namespace/protection context.

- October 31, Credential Provider write foundations: internal create/PATCH/
  guarded-delete helpers add no child metadata, adoption or declaration
  authority. Public DCR settings persist through fresh GET verification.
  Parent ownership and collection enrollment remain implementation work.

- October 31, Credential Provider inspection: complete provider/template
  pagination, exact scoped names and public formats are implemented without
  metadata stamping or adoption. No credential/enrollment endpoint is called
  by ordinary inspection. Declarations remain in progress under #2660.

- October 31, Token Vault public keys: declarative lifecycle and public
  inspection use fingerprint identity and explicitly managed directory
  ownership. No child metadata is stamped. Parent adoption, collection sync,
  protection, saved writes, public dump/reload and ordered retirement are
  covered by a live `.tech` scenario. No SDK revision beyond `2597fe249e26`
  is needed. Runtime encryption/private-key retention is not claimed.

- October 31 UTC, directory deletion observations: enabled Vault and fresh
  empty key/provider/Trusted IdP collections are now preconditions for planned
  and saved deletion. Disabled/unknown/incomplete inventories block instead
  of granting cascade authority. No metadata is stamped on these children and
  no parent adoption or child retirement is implicit. This does not establish
  inherited writable ownership; Q37's backend atomicity question stays open.

- October 31 UTC, Vault contract evaluation: five RSA/EC public keys match
  independently computed RFC 7638 kids; duplicate material conflicts while
  duplicate names work. Provider names conflict on create and update. Neither
  resource exposes labels/managed_by. Four direct parent-delete probes remove
  the directory with an attached key/provider under force=false, in both Vault
  states; disabled Vault hides the same collections. No inherited ownership
  policy is implied. No provider/key declarative writer is claimed by this
  contract/SDK preparation work.

- October 31 UTC, explicit password operations: generated/imported creation
  captures values privately without ownership stamping. Expiration-only PATCH
  sends an explicit date/null and verifies the existing management map. Material
  replacement uses fresh GET/full PUT/read-back, preserving omitted expiration
  and arbitrary management metadata. Selected/all deletion requires explicit
  scope and confirmation. Passwords remain excluded from declarative ownership,
  adoption, sync and dump; attached records still block username deletion.

- October 31 UTC, Trusted IdP inspection: configuration and associated-principal
  commands preserve public metadata and do not stamp IdP ownership. IdPs have
  no labels or managed_by field. Associated principals retain their existing
  independent ownership; matching an issuer does not imply JIT provisioning
  or authorize principal mutation. SDK `30c3834c9846` corrects both list query
  tags. Declarative trust configuration remains pending its ownership/deletion
  policy; native force=false and disabled-Vault visibility are Q37.

- October 31 UTC, credential expiration SDK correction: preview
  `79bc3dbbb928` restores omitted/date/null request support without changing
  ownership storage. No principal/group key or password product writes are
  enabled. Imported-key ownership and mutable key/password management maps
  still need verified backend contracts (Q4/Q28/Q35).

- October 31 UTC, declarative basic-auth username records: exact username
  identity uses four canonical managed_by entries, independently of the
  principal/directory. Refs stay local. Full PUT/adoption never sends password
  values, and dump excludes passwords. Principal deletion now observes
  username records and API keys; username deletion requires empty passwords.
  Final plans order authorized child deletes first. Password ownership writes
  remain outside this declaration under Q35 and explicit secret-write policy.

- October 31 UTC, basic-auth inspection: username and password get/list now
  expose public management metadata without writing ownership or displaying
  credential material. Complete cursor observations and explicit username
  selectors pass live `.tech` checks. Shared username-record write helpers
  provide verified PUT/readback and password-aware deletion foundations;
  declarative enrollment and principal credential guards are still pending.

- October 31 UTC, ordinary groups: directory/name identity uses four canonical
  managed_by entries, independent of directory ownership. Public labels and
  runtime metadata retain their API meaning. Adoption/PUT/dump and scoped
  sync/delete are implemented; team/importer writes stay deferred. Group
  deletion checks memberships and keys; principal deletion checks incoming
  direct memberships. Relationships still have no independent metadata store.


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

- October 31, direct memberships: membership entries have no metadata store.
  Declarative authority inherits the ordinary group's namespace/protection;
  only an explicitly selected collection permits sync pruning. Group adoption
  preserves existing links and grants collection authority, without stamping
  entries. External groups and importer/team/nested mutations are excluded.
