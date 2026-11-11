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

Generated and imported API keys have an immutable creation convention: `tool`,
`version`, `kongctl-ref` and `kongctl-create-type`. Managed principal/group keys
inherit current parent authority. Keys under external principals add a fifth
entry, `kongctl-namespace`, and own only that namespace/ref within the parent
UUID.
They cannot add protection as a sixth entry, adopt existing keys, or transfer
ownership. This is the client ownership convention, not API immutability.
The SDK can represent imported-key management maps and PATCH map entries;
null removes a key. These API capabilities do not expose imperative Identity
writes. Declarative key updates retain creation identity markers.
Names are mutable for principal/group keys; omitted names are preserved and
explicit null clears them. Imported principal keys accept deferred `!secret`
sources for creation only; existing managed keys can omit the source. Imported
creation needs no private capture, but generated issuance still does. Bulk
secret writes skip existing material, and explicit selection errors with
rotation guidance. Dumps record creation type and public metadata; recreation
requires a new source. Imported group-key creation ignores initial names and
remains unsupported (API-14).

List observations must follow every cursor before local matching or pruning.
Duplicate matches stop planning and execution. Saved writes recheck parent
scope, UUID, ownership and protection. Explicit child collections scope sync;
omission preserves collections. Parent deletion requires separately authorized
child retirements and fresh empty-dependency checks, never an implicit cascade.

## Compatibility boundaries

- Hydrate omitted fields from fresh GET responses before full PUTs. Principal
  type is immutable after creation. Preserve missing `active` rather than
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
  Vault authorization writes are outside the supported declarative contract.
  Identity has no imperative resource create/patch/delete commands. Adoption
  and org-chart import are separate supported workflows. Read commands exclude
  secret material and avoid detail GETs that issue new enrollment URLs.
- Vault is a directory capability, not an independent resource. Enable an
  existing Vault before observing its children. Key/provider/credential/IdP
  writes need an explicitly managed directory. Static secret updates require
  explicit write selection; dumps never include secret material.

Change compatibility guards only with a focused regression test and direct
API evidence for the complete intended lifecycle. SDK request fields alone do
not establish deployed behavior. Keep serialization corrections reproducible
in `tools/sdk-patches` and use `go.mod` as the SDK version authority.

## API follow-up tracker

The API behavior and requested verification columns can be sent to the API
engineering team without the KongCTL column. They describe HTTP contracts,
not declarative-management requirements. API IDs are stable tracking labels;
they do not reuse the old investigation question numbers.

**Evidence:** T = US `.tech`; C = US `.com`. API-01–API-12 were tested
using direct HTTP on November 9, 2026, with fresh GETs after mutations.
API-01 includes ordinary principals in both environments and an existing
imported principal in C; no imported-principal fixture was available in T.
API-02/04/11 cover generated and imported keys under both principals and
groups. API-06 covers zero, one and multiple scope links at page sizes
1, 2 and 10. API-09 includes signed int64 boundaries. These probes verify
administrative responses and stored values, not runtime authentication.
A subsequent `.tech` confirmation used `kongctl api` for the seven behaviors
reported to engineering: API-01/02/04 (keys)/05/06/07/09 and group creation
(API-13). Exact integer request bodies were passed through `--body-file`;
raw HTTP GETs corroborated numeric storage independently of the CLI formatting
bug discovered during the probes. That formatter is now corrected. Imported-key
POST with `managed_by` now persists the map in T,
and the SDK/client now exposes that field through a targeted model patch.
A further `.tech` `kongctl api` pass on November 9 rechecked API-03,
API-04 (passwords), API-08, API-10 and API-12: all reproduced. API-08 was later
confirmed as expected immutability, as recorded below. The pass also refreshed
CON-01/02/03/04 (principal cascade)/06/07 administrative observations.
The collection response omits enrollment URLs, but successive detail GETs
return different URLs. Their reuse, validity and invalidation were not tested.
Service build versions were not exposed. CON-05/08/09/10 and the group portion
of CON-04 retain historical evidence: T October 31–November 1;
C November 6–7, 2026. No production calls were made in these follow-up passes.

Administration routes use `https://us.api.konghq.tech` or
`https://us.api.konghq.com`; runtime routes use
`https://us.identity.konghq.tech`. Single-letter path segments are resource
IDs: D directory, P principal, G group, A Auth Server; the other letters name
IDs of their enclosing resources. They are placeholders, not literal values.

**Tracking:** Engineering reports that the seven originally reported fixes
landed in development, with production rollout pending. Direct checks confirm
the reported behavior changes except direct `type: team` creation, which is
deferred. Additional key-name fixes were independently observed in T.
Production entries below describe the last C test; they do not establish the
current rollout state. No engineering ticket IDs or service build versions
have been recorded. Replace `—` with the issue link when available.

Keep API resolution separate from client cleanup. A verified backend fix does
not mean the SDK/client exposes it, the workaround has been removed, or the
complete credential/runtime lifecycle has been verified.

### November 10 production revalidation

Fresh `kongctl api` calls in US `.com`, using the configured
`gh-2752-api-key-imported_com` test profile and disposable fixtures, confirmed
the following behavior. These observations supersede the historical C results
below for the cases tested; they do not establish all-case rollout completion.
Every mutation was checked with a fresh GET, and all fixtures were deleted.
Credential material was kept in memory and excluded from reported results.

- API-01: Ordinary principal management-map PATCH merges entries and removes
  null-valued entries. Imported principals were not retested.
- API-02: Generated/imported principal/group keys retain creation management
  maps and support PATCH merges and entry removal. Imported principal keys
  retain canonical kongctl creation markers, with and without an independent
  namespace marker.
- API-04 (keys) and API-11: All four key variants preserve expiration when
  omitted, accept explicit null expiration, and support scalar renames and
  null name clearing.
- API-05: Labels-only updates preserve `client_secret_basic` for both tested
  clients. Runtime credential continuity was not tested.
- API-06: Two claims with three scope links each and two clients with two
  scope links each report total 2 at page sizes 1, 2 and 10. Zero/one-link
  cases and full pagination traversal were not repeated.
- API-07: Wrong-parent identity GET and DELETE return 404; a subsequent GET
  under the actual parent confirms the identity survives.
- API-09: Principal PUT and group POST/PUT preserve positive and negative
  integers beyond the exact float64 range, including int64 bounds in arrays.
- API-03 and API-04 (passwords): Management-map PATCH still ignores additions
  and removals; omitted expiration still clears it. Metadata-only password
  PUT returns 400.
- API-08: Agent-to-user PUT returns 400; null type returns 200 while retaining
  agent type. Principal type is confirmed immutable; these observations do
  not establish a defect. The reverse transition was not repeated.
- API-10: PATCH null still retains both JWKS URI and display name; hydrated
  PUT clears JWKS URI.
- API-12: GitHub-provider creation with empty scopes still selects defaults;
  PATCH empty scopes and null endpoint overrides return 400. Empty-string
  endpoint overrides remain accepted. Custom OAuth was not retested.
- API-14: Imported group-key creation still loses the supplied initial name;
  subsequent PATCH rename persists. Generated group and both principal-key
  variants retain their initial names.

API-13 remains deferred. No CON-series contracts or runtime authentication
were revalidated. The older explicit `.tech` test profile returned 401, so
this pass makes no new development-environment claims.

### Current disposition

- **Outstanding behavior findings:** API-03, API-04 (passwords),
  API-10 and API-12 remain reproduced. An additional live check found API-14:
  imported group-key creation ignores the name, although later PATCH works.
  API-12 concerns unsupported clearing semantics; its documented intended
  behavior still needs confirmation.
- **Verified behavior:** API-01, API-02, API-05, API-06, API-07, API-09 and
  API-11, plus the key portion of API-04, passed the development checks and
  the November 10 production cases described above. Coverage limits still
  apply; historical C entries below do not describe current rollout state.
- **Unconfirmed contracts/integration:** CON-01–CON-10 still need engineering
  decisions or runtime verification. CON-01/02/03/04 (principals)/06/07 have
  fresh T administrative observations; CON-05/08/09/10 still need retesting.
  They are questions, not ten established service defects.
- **Deferred:** API-13, direct team creation. Keep the ordinary-group guard;
  follow up separately without blocking the current investigation.
- **Expected behavior:** API-08 is closed as a defect finding. Principal type
  is immutable after creation; retain planner and saved-plan validation.
- **Client updates:** The SDK represents imported-key `managed_by` and
  nullable key names. Principal/group metadata accepts signed 64-bit integers;
  API JSON output and jq filtering preserve exact integer values. The existing
  key ownership convention and remaining backend workarounds are retained.
  Development behavior is the implementation baseline; production rollout
  compatibility is not implemented through environment-specific branches.

### Outstanding behavior findings

These are the remaining observed failures or ambiguous update semantics,
confirmed again through `kongctl api` in T on November 9. Fresh GETs verified
the stored values. API-04 includes a verified fix for keys and a reproduced
password behavior. API-12 needs an intended clearing contract; validation
errors alone do not establish a defect.

| ID / API | Observed API behavior | Engineering action / verification | KongCTL handling | Status; engineering issue |
| --- | --- | --- | --- | --- |
| API-03<br>`PATCH /v2/directories/D/principals/P/basic-auths/B/passwords/W` | **T/C:** Password POST retains `managed_by`; PATCH returns 200 but ignores map updates/removal. Metadata-only PUT returns 400 requiring `secret`. | Provide/document a metadata-only update preserving the existing password and expiration, or explicitly reject unsupported metadata PATCH. Verify with fresh GET and authentication. | Read-only password inspection; no declarative password material writes or adoption. | T reproduced with kongctl api Nov 9; last C test also reproduced; — |
| API-04<br>`PATCH /v2/directories/D/principals/P/api-keys/K`<br>`PATCH /v2/directories/D/groups/G/api-keys/K`<br>`PATCH /v2/directories/D/principals/P/basic-auths/B/passwords/W` | **C:** Omitting `expires_at` clears expiration for principal keys, group keys and passwords. **T:** Key expiration survives omission; password expiration still clears. Explicit timestamps and null work in both. | Preserve expiration when omitted, or document intentional replacement semantics. Verify a future expiration survives unrelated PATCH fields and explicit null still clears it. | Preserve key expiration explicitly during updates. Password writes are outside declarative support; nullable SDK serialization alone does not expose a command. | T keys verified; password omission reproduced with kongctl api Nov 9; C rollout/retest pending; — |
| API-10<br>`PATCH /v2/directories/D/trusted-idps/T`<br>`PUT /v2/directories/D/trusted-idps/T` | **T/C:** PATCH `jwks_uri: null` and `display_name: null` return 200 but retain their values. Hydrated PUT clears `jwks_uri`. | Apply documented null clearing for both fields, or reject unsupported clearing. Verify fresh GETs and preservation of unrelated settings; JWKS clearing should restore issuer discovery. | Hydrated full PUT for trust updates and nullable overrides. | T reproduced with kongctl api Nov 9; last C test also reproduced; — |
| API-12<br>`POST /v2/directories/D/vault/providers`<br>`PATCH /v2/directories/D/vault/providers/V` | **T/C:** GitHub-provider creation with `scopes: []` selects template defaults; custom-oauth creation keeps an empty list. PATCH from nonempty scopes to `[]` returns 400. Null token/authorization endpoint overrides return 400; empty strings return 200 and remain empty effective endpoints. | Document separate operations/values for an empty scope list, template defaults and removal of endpoint overrides. Verify stored and effective values for each supported request. | Reject unsupported clearing rather than guess inheritance or silently substitute defaults. | T clearing behavior reproduced with kongctl api Nov 9; intended contract needed; — |
| API-14<br>`POST /v2/directories/D/groups/G/api-keys`<br>`PATCH /v2/directories/D/groups/G/api-keys/K` | **T, fresh live check:** Creating `type: imported` with a non-null `name` succeeds but returns a null name. The same request retains `managed_by`. A subsequent PATCH rename persists. | Apply the supported initial name or reject it explicitly. Confirm whether the restriction is intentional for imported group keys; align the shared create schema with that contract. | Generated keys remain declaratively supported; imported-key provisioning is outside the declarative contract. No imperative Identity write command is exposed. | T reproduced Nov 9; imported variant outside declarative support; — |

### Verified development behavior and rollout follow-up

These items are not current development defect reports for the tested cases.
Keep production compatibility handling until rollout and regression checks
justify changing it. API-01 does not establish imported-principal behavior in T;
credential continuity and runtime authentication are separate checks.

| ID / API | Observed API behavior | Engineering action / verification | KongCTL handling | Status; engineering issue |
| --- | --- | --- | --- | --- |
| API-01<br>`PATCH /v2/directories/D/principals/P`<br>`PUT /v2/directories/D/principals/P` | **C:** Partial/full `managed_by` PATCH and null-key removal return 200 without changing the map. **T ordinary principal:** PATCH merges keys and null removes a key. Hydrated PUT persists the map in both. C imported principal has the same PATCH/PUT behavior. | Apply supported map updates/removals, or reject unsupported requests. Verify partial/full maps, missing keys and null removal with fresh GETs; document merge versus replacement. | Fresh GET plus hydrated PUT remains the full-update contract for metadata merge/sync, adoption and protection. Fixed management PATCH does not establish all public-field replacement semantics. | T verified for ordinary principals; imported T not tested; C rollout/retest pending; — |
| API-02<br>`PATCH /v2/directories/D/principals/P/api-keys/K`<br>`PATCH /v2/directories/D/groups/G/api-keys/K` | **C:** Generated/imported principal/group key PATCH returns 200 but ignores `managed_by` updates/removal. **T:** These PATCH operations persist changes and remove null-valued keys. Generated-key POST retains the map in both. Raw imported-key POST also retains `managed_by` in T; C creation with that field was not rechecked. | Provide/document an operation that updates management metadata without replacing key material. Verify map changes/removal, unchanged expiration and credential continuity. | Fixed creation identity markers remain the declarative convention; no adoption or ownership transfer. The SDK represents imported metadata; imported-key provisioning and arbitrary metadata writes are not exposed as Identity commands. | T verified, including imported-key POST; SDK/client updated; C rollout/retest pending; — |
| API-05<br>`PATCH /v1/auth-servers/A/clients/C` | **C:** Labels-only PATCH silently changes `client_secret_basic` to `client_secret_post`. **T:** The method stays `client_secret_basic`. Explicitly sending the method preserves it in both. | Preserve each supported authentication method when omitted. Verify a labels-only update using fresh GET and the original client credentials. | Hydrate and explicitly resend the current authentication method during updates and adoption; verify readback. | T verified; C rollout/retest pending; — |
| API-06<br>`GET /v1/auth-servers/A/claims?page[size]=1&page[number]=1`<br>`GET /v1/auth-servers/A/clients?page[size]=1&page[number]=1` | **C:** Two claims linked to three scopes each report total 6; two clients linked to two scopes each report total 4. Actual counts are 2. Totals are correct with zero/one scope link. **T:** Totals match actual counts for all tested associations/page sizes. | Count resources rather than scope associations. Verify totals and complete traversal with zero, one and multiple scope links at multiple page sizes. | Exhaust actual pages to a terminal empty page; do not use totals to decide when traversal is complete. | T verified; C rollout/retest pending; — |
| API-07<br>`GET /v2/directories/D/principals/P/identities/I`<br>`DELETE /v2/directories/D/principals/P/identities/I` | **T/C:** Wrong-parent GET and DELETE now return 404. The identity is still readable under its actual parent after both attempts. The historical cross-parent exposure/deletion did not reproduce. | Confirm deployment of principal/identity relationship checks. Keep wrong-parent GET/DELETE regressions that verify the actual owner's identity survives; no current reproduction to submit as an open defect. | Prove membership through the selected principal's complete identity collection before detail reads and mutations. | Verified in T and last C test; retain regression coverage; — |
| API-09<br>`PUT /v2/directories/D/principals/P`<br>`POST /v2/directories/D/groups`<br>`PUT /v2/directories/D/groups/G` | **C:** Metadata integer `9007199254740993` becomes `9007199254740992` in the write response and fresh GET; negatives, arrays and int64 maximum also lose precision. **T:** All tested integers remain exact across principal PUT and group POST/PUT. | Define the supported numeric range. Preserve supported integers exactly or reject unsupported values before persistence; verify negative values, arrays and boundaries. | Accept signed int64 metadata and verify readback. Preserve exact numbers through loading, saved plans, SDK mapping, dump and CLI JSON/jq output. | T raw precision verified; client rounding corrected; C rollout/retest pending; — |
| API-11<br>`PATCH /v2/directories/D/principals/P/api-keys/K`<br>`PATCH /v2/directories/D/groups/G/api-keys/K` | **C:** Principal scalar renames work; null clearing is ignored. Group scalar renames and null clearing are ignored. **T:** Scalar rename and null clearing work for generated/imported principal/group keys. | Document which names are mutable/nullable. Apply supported changes or return a validation error instead of ignoring them; verify fresh GET. | Support declarative scalar rename and explicit null name clearing for generated principal/group keys, preserving expiration and creation identity markers. | T verified Nov 9; C rollout/retest pending; — |

### Confirmed expected behavior

API-08 (`PUT /v2/directories/D/principals/P`) is closed as a defect finding:
principal type is immutable after creation, as confirmed by the maintainer.
Non-null type changes return 400; explicit null retains the existing type.
The November 10 C check reconfirmed agent-to-user rejection and null retaining
agent type. KongCTL rejects declared type changes during planning and saved
execution, including attempts to clear a classification with null. Omission
preserves the existing type. No API change is requested for API-08.

### Deferred follow-up

| ID / API | Observed API behavior | Engineering action / verification | KongCTL handling | Status; engineering issue |
| --- | --- | --- | --- | --- |
| API-13<br>`POST /v2/directories/D/groups` | **T, fresh `kongctl api` confirmation:** `{"name":"apiprobe-example-team","type":"team"}` returns 400 with a `type` / `allOf` validation error; the same API accepts ordinary `type: group`. The SDK create schema exposes both enum values. C was not rechecked. | Is direct creation of `type: team` supported, or restricted to the org-chart importer? Align the create schema with supported behavior, or identify the prerequisite for direct team creation. | Reject direct team management; ordinary groups remain supported. | Deferred: T creation rejected; follow up separately; — |

### Contract clarifications and integration prerequisites

These observations may be intentional behavior. The requested work is to
establish a documented contract or supported alternative, not presume a bug.
CON-01/02/03/04 (principals)/06/07 now have fresh T administrative checks.
CON-05 was not repeated because deleting the shared default directory would
remove its existing contents. CON-08/09 need an isolated importer fixture;
this pass did not import a roster or change imported users. CON-10 needs a
supported runtime fixture. Administrative status codes do not prove runtime
revocation or authentication behavior.

| ID / API | Observed API behavior | Engineering action / verification | KongCTL handling | Status; engineering issue |
| --- | --- | --- | --- | --- |
| CON-01<br>`POST /v2/directories/D/vault/providers`<br>`PATCH /v2/directories/D/vault/providers/V` | **T:** Create has no `enabled` field in its schema; sending raw `enabled: false` still creates an enabled provider. PATCH can disable it. | Confirm whether atomic disabled creation is supported. Expose it in the schema/API if intended; otherwise document that creation is enabled and a second request is required. | Apply requested disabled state through a separate PATCH after creation; do not promise atomic disabled creation. | T administrative behavior rechecked Nov 9; contract/runtime follow-up open; — |
| CON-02<br>`PUT /v2/directories/D/vault/providers/V/credentials/C` | **T, fresh:** Routing-only PUT without `secret` returns 400 and retains the old selector/priority. PUT with the same secret but omitted routing returns 200 and clears selector/priority to the default slot. Stored-secret runtime continuity was not tested. | Document replacement defaults and whether a material-preserving routing update exists. Verify both default and priority slots and unchanged material for any supported metadata-only operation. | Require explicit secret-write selection and supplied material; hydrate and recheck current routing before PUT. | T administrative behavior rechecked Nov 9; contract/runtime follow-up open; — |
| CON-03<br>`GET /v2/directories/D/vault/providers/V/credentials/C` | **T, fresh:** Creating a pending shared GitHub OAuth credential and reading its detail return enrollment URLs. Two successive GETs return different URLs; the list response omits them. Reuse, validity, expiry and invalidation were not tested. | Document issuance on detail GET and the lifetime/reuse of each URL. Does another GET invalidate earlier URLs or consent sessions? Verify metadata-only list behavior and supported revocation. | Use metadata-only collection reads for inspection; no enrollment URL issuance/capture command is exposed. | T administrative behavior rechecked Nov 9; contract/runtime follow-up open; — |
| CON-04<br>`DELETE /v2/directories/D/principals/P`<br>`DELETE /v2/directories/D/groups/G` | **T, fresh principal test:** DELETE returns 204 with an attached custom identity, basic-auth username and password; all three child GETs then return 404. Group cascades retain historical observations. Runtime revocation was not established. | Document the per-resource cascade and revocation contract. Provide/document an atomic reject-if-attached option if available; verify descendants and runtime effects. | Require explicitly authorized retirements and fresh empty-dependency checks. Client preflight is not atomic with deletion. | T principal cascade rechecked Nov 9; group/runtime checks and contract open; — |
| CON-05<br>`DELETE /v2/directories/D?force=false`<br>`GET /v2/directories/D/vault/keys`<br>`GET /v2/directories/D/vault/providers` | **T:** Non-forced directory deletion returns 409 with principals/groups but 204 with Vault keys, providers without credentials or Trusted IdPs. Disabled Vault collection reads return 403. | Specify which dependencies non-forced deletion must reject and how disabled contents can be inspected. Verify an atomic nonempty-dependency precondition if supported. | Inspect dependencies before deletion; enable Vault for dependency discovery. Explicit child retirement is required. | Needs contract; — |
| CON-06<br>`DELETE /v2/directories/D/vault/providers/V` | **T, fresh:** Deleting a disabled static-secret provider with an attached shared credential returns 204; credential GET then returns 404. User authorizations, enabled-provider behavior and runtime revocation were not tested in this pass. | Document cascade behavior for shared and user credentials, including runtime effects. Provide/document a conditional delete that rejects attached credentials if supported. | Require complete empty credential inventory, including independently managed authorizations, before provider retirement. | T administrative behavior rechecked Nov 9; contract/runtime follow-up open; — |
| CON-07<br>`DELETE /v2/directories/D/trusted-idps/T` | **T, fresh:** The associated-principal list first confirms an OIDC-linked principal. Deleting the trust returns 204; both principal and identity GETs still return 200 afterward. Runtime authentication/Vault effects were not tested. | Specify authentication/Vault invalidation after deletion and whether reject-while-associated is available. Verify active and inactive associated principals. | Require empty associated-principal inventory after independently authorized detachments. | T administrative behavior rechecked Nov 9; contract/runtime follow-up open; — |
| CON-08<br>`POST /v2/directories/D/org-chart/import` | **C:** Repeating the identical roster preserves principal, email-identity and external-ID-identity UUIDs but recreates the `kong:org_chart_root` identity UUID. | Confirm whether the marker identity ID is intentionally ephemeral. Preserve it across unchanged imports if stable references are supported; otherwise document the lifetime. | Keep imported principals external. A full dump containing the old marker UUID may need regeneration after another import. | Needs contract; — |
| CON-09<br>`PUT /v2/directories/D/principals/P` | **C:** With `provisioned_by: imported`, changed name/description returns 409. Preserving those fields while changing `metadata`, `labels` or `managed_by` succeeds without changing provisioning/activity. | Document which fields administrative clients may change on imported principals, which operation to use, and how later imports reconcile those fields. | Do not adopt/reconcile imported principals as ordinary managed principals; manage supported children through external UUID references. | Needs contract; — |
| CON-10<br>`POST /{directoryName}/v2/authenticate`<br>`POST /{directoryName}/v2/vault/enrollment/resolve` | **T, runtime host:** Administrative bearer requests without a supported runtime context return 400, `mTLS headers missing`. This does not establish a service defect. | Provide a working certificate, control-plane/organization-context and request example for authentication and enrollment resolution, including supported runtime versions. | Keep administrative lifecycle verification separate; populated user-authorization/revocation testing needs a supported runtime fixture. | Needs runtime fixture; — |

SDK serialization corrections (cursor query names, explicit empty collections,
omitted authentication methods and nullable expiration) are tracked separately
in `tools/sdk-patches`. They are not evidence of service-side API defects.
Client-side filtering and duplicate detection are accepted design choices;
server-side `managed_by` filtering is not a prerequisite for these workflows.

The CLI rounding defect found during API-09 verification is corrected:
`kongctl api` JSON output, typed request assignments and jq filtering preserve
exact integers. Signed 64-bit Identity metadata is covered through saved plans,
execution and dump/replan. Isolated live checks in T verified these paths,
omitted metadata retention, declarative key names, scoped sync and
delete. They preserved the shared directory and unrelated resources. This
client correction is separate from the API precision fix.

## Testing

Run `scripts/identity_api_probe.py` to retest API-01–API-12 independently
of SDK serialization and KongCTL compatibility handling. It reads the selected
profile's `KONGCTL_<PROFILE>_KONNECT_PAT` environment variable; the explicit
`--environment` selects the US administration hostname. Each output directory
must be new. Use dedicated test organizations and avoid concurrent changes.

```sh
python3 scripts/identity_api_probe.py \
  --environment com \
  --profile rspurgeon-e2e-kongctl-testing_com \
  --output .cache/identity-api-probe-com
```

```sh
python3 scripts/identity_api_probe.py \
  --environment tech \
  --profile rspurgeon-e2e-kongctl-testing_tech \
  --output .cache/identity-api-probe-tech
```

The probe reuses `default`, temporarily enables Vault when necessary, removes
its disposable fixtures and restores directory settings. An optional
`--imported-principal-id UUID` tests an existing imported principal's management
map and restores its public fields; it never imports a roster. Raw evidence
contains credentials and stays in private files; share only the observations
in `summary.json`. Each observation references its last HTTP evidence file.
Exit 0 means the probes completed, even when API defects reproduced. Exit 1
means incomplete coverage or failed cleanup; inspect those rows before filing
issues. No runtime authentication or credential-continuity claim is made.

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
CI enables this coverage by default in test organizations with a three-directory
quota. Local runs still require explicit opt-in.
Reset retires children in dependency order, releases imported users for
cleanup, and restores the default TTL/access/Vault settings. Preserve enabled
replay coverage when changing scenarios; see [replay policy][replay].

[implementation]: declarative-resource-implementation-guide.md
[examples]: ../examples/declarative/identity/README.md
[replay]: ../../test/e2e/replay-policy.md
