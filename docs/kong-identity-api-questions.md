# Kong Identity API team questions

Questions about observed HTTP behavior and API resource contracts. Record each
team response with its date, applicable environment, and API version.

Unless stated otherwise, observations were collected on October 30, 2026
(US Central), using direct HTTP requests to `https://us.api.konghq.com`.
Specification links are pinned to the inspected platform-api PR 3877 revision.
Temporary test resources were deleted after the probes.

## Environment review before engineering follow-up

The original `.com` findings are dated production observations, not automatic
limitations of the prerelease API in `.tech`. The October 31 direct HTTP
revalidation used US `.tech` and separate disposable records under the user's
default directory, plus an isolated auth server. The inspected PR remains
open at `98137d7be3772b67693cc1ceb0469e77f8ecf0f5`.

| Area | US `.tech` result | Engineering status |
| --- | --- | --- |
| Principal active (Q10) | False create/read/list, true/false updates and omitted-field preservation work. | Backend blocker cleared for directly provisioned records; deployment/provisioning differences remain. |
| Vault enablement (Q1) | True persists and unlocks provider/key lists. Restored to false after testing. | Backend blocker cleared in the tested org; production rollout remains separate. |
| Principal key applicability (Q27) | User, agent and unclassified parents support generated/imported keys. | Production user-404 not reproduced. |
| Principal key names (Q29) | Generated/imported names and rename work; duplicates are accepted. | Availability improved; names are not unique identifiers. |
| Group keys (Q33) | List/create/read/PATCH/delete work. | Production 501 not reproduced; ownership updates still fail. |
| Group team subtype (Q32) | Create rejects team with 400, only group accepted. | Different from production; deployment/entitlement contract needs clarification. |
| Principal type (Q9) | Non-null type change gets explicit immutable error; null is silently ignored. | Create-only policy remains; nullable/error semantics remain open. |
| Ownership PATCH (Q4–Q5, Q11, Q28) | Principal/key map changes ignored; imported key create rejects management map. | Still reproducible. |
| Integer precision (Q31) | Large integer digits still round, including arrays. | Still reproducible. |
| Identity parent scoping (Q21) | GET/DELETE accept another existing principal's path. | Still reproducible; scoped collection proof required. |
| Auth-server behavior (Q12–Q13, Q15–Q18) | Duplicate claims, inflated totals, PATCH method reset, two total secret slots and rejected interactive grants persist. | Still reproducible in tested org. |
| Runtime prerequisites (Q30) | Administrative bearer plus a valid key still gets mTLS-context error. | Supported runtime context still required. |
| Unverified contracts | Provisioning lifecycle, importer/user enrollment, workspace fixtures, cross-resource deletion variants and recovery guarantees. | Do not present these as resolved or as reproduced `.tech` defects. |
| Cursor query (Q22) | Shared source annotation and SDK wire mismatch remain established separately. | Keep upstream spec correction and SDK tests. |

Sanitized request/response evidence is saved locally under
`/home/rspurgeon/.cache/kong-identity-public-sdk/tech-revalidation/`:
`directory-contracts.json`, plus `auth/`, `followup/` and `capabilities/`
results. Issued material was not written into those logs; temporary credentials
and their parent fixtures were removed. The manual demo registrations remain.
Default directory public settings were restored; its update timestamp changed
during the Vault toggle.

No broad compatibility guard was lifted during this validation pass. Group
and principal credential contracts must not be inferred from one another.
Operations involving imported teams, user-enrolled credentials or a supported
runtime context still need their own verification.

## Directory API questions

### Q1 Directory Token Vault enablement

**US .tech retest (October 31, 2026):** PUT with `vault_enabled: true` returns
and retains true in US `.tech`; provider/key lists change from 403 (Vault
disabled) to 200. The original false setting was restored. This availability
blocker is cleared for the tested `.tech` organization; production rollout
remains a separate question.

**Question:** Is `vault_enabled` supported in the US directory API today?
Is enablement awaiting deployment, gated by an entitlement, or failing to
persist?

**Observed:** PUT with `vault_enabled: true` succeeded but returned false.
A subsequent GET also returned false. Other supplied directory settings and
`managed_by` entries were retained.

**Specification:** [DirectoryVaultEnabled][vault-definition] is a boolean
that enables Token Vault when true and defaults to disabled. It is referenced
by the [create body][vault-create], [PUT body][vault-replace],
[PATCH body][vault-update], and [directory response][vault-response].

### Q2 Default directory lifecycle

**Question:** Does every organization automatically receive a directory
named `default`? When is it created, can it be renamed or deleted, and does
it have any special behavior beyond that name?

**Observed:** GET `/v2/directories` returned an empty list for one test
organization. The inspected schema exposes name and ID without a default
marker.

**Additional user observation (October 31, 2026):** A recently created
organization has a directory named `default`, created October 28. It has empty
`labels`/`managed_by`, TTLs of 600 seconds, `allow_all_control_planes: true`, an
empty control-plane allowlist and `vault_enabled: false`. This establishes an
observed provisioning default, without establishing the universal lifecycle
guarantee. Usage examples now look up `default` externally. The separate
`.tech` test organization returned an empty collection before a manual
bootstrap. At the user's request, a directory named `default` was created
there with those reported settings and empty ownership/label maps; a fresh
GET retained all supplied values. This test setup does not establish an
automatic-provisioning guarantee. External lookup still never creates one.

### Q3 Directory quota and control plane access

**US .tech retest (October 31, 2026):** Creating a second disposable directory
returns 403 with max directories 1 in the tested `.tech` organization. No
existing directory was deleted to bypass that quota.

**Question:** Is `allow_all_control_planes: true` permanently required when
an organization has a one-directory quota? Which organizations may restrict
access to an explicit control-plane allowlist, and how is that capability
exposed through the API?

**Observed:** Directory creation with false was rejected with an error
stating that it must be true when the directory quota is one. Creation with
explicit true succeeded. The [schema default is false][directory-spec].

## Resource identity and update contracts

These questions concern the resource contracts in
[platform-api PR 3877](https://github.com/Kong/platform-api/pull/3877).

### Q4 Imported API key ownership

**US .tech retest (October 31, 2026):** Imported key create still rejects
`managed_by` with 400 for user, agent, unclassified and group parents. Creation
without it succeeds; subsequent PATCH still leaves the empty map unchanged. This
remains an open ownership limitation in `.tech`.

**Question:** Can imported principal and group API keys accept `managed_by`
in the initial create request, so the key and its management metadata are
created atomically?

The inspected imported-key create body does not expose `managed_by`. Is
there another supported way to attach it during creation?

**Observed (October 31, 2026 UTC, US `.com`):** Imported create with
`managed_by` returns 400, identifying that property as unsupported. Creation
without it succeeds and returns an empty map. A subsequent complete-map PATCH
returns 200 but GET still shows the empty map (Q28), so a second write does not
currently provide an ownership workaround.

### Q5 Ownership update semantics

**US .tech retest (October 31, 2026):** Principal PATCH ignores partial/full
management maps and null removal in `.tech`; full group PUT retains all four
ownership entries, while a name-only PUT clears omitted public maps.
Principal/group key PATCH ignores map changes/removal. Map behavior must remain
resource/operation-specific.

**Question:** For principals, identities, and credentials, which operations
replace the whole `managed_by` map, which merge keys, and how are keys removed?
Are management metadata updates supported for every resource that exposes it?

**Observed:** Directory and principal PUT requests retained the supplied
management metadata values. Principal PATCH behaved differently; see Q11.
Omission and removal semantics still need confirmation across resource types.
Principal API-key PATCH also ignores partial and complete management maps;
see Q28 for its expiration side effect.

### Q6 Vault and trusted IdP ownership

**Question:** Do vault keys, credential providers, provider credentials, and
trusted IdPs expose a creator, managing-system identifier, or external
identifier when `managed_by` is unavailable? What is the supported way to
distinguish records created by different administrators or systems?

**Known:** The team states that provider names are unique per directory and
vault keys have unique `kid` values. Those identify records but do not
identify their creator or managing system.

### Q7 Shared provider credential matching and updates

**Question:** Can one provider have multiple credentials created by an
administrator? What uniqueness constraints apply, and which fields besides
the generated ID identify a credential within its provider?

Do updates preserve omitted secret material? Are selectors and priority
mutable, and how are omitted or empty values handled? Do these rules differ
for credentials created through user enrollment?

### Q8 Groups and imported organization charts

**Question:** Are group memberships unique by member type and ID, and are
group cycles rejected? May manually created memberships coexist with
organization-chart imports? Which records may an import overwrite or remove?

## Principal deployment questions

### Q9 Principal type changes

**US .tech retest (October 31, 2026):** A non-null user-to-agent PUT now returns
400 with `type is immutable after creation`, instead of the earlier production
silent success. PUT with type null still returns 200 and retains user.
Create-only type remains appropriate; nullable-update/error semantics still need
clarification.

**Question:** Are principal type changes intentionally unsupported, or is the
deployed update implementation behind the specification? Can a principal
change between user/agent or return to null? If type is immutable, should an
attempted change produce a validation error?

**Observed:** Creation retained user, agent, and unclassified/null types.
PUT requesting agent for an existing user succeeded but returned user.
Subsequent GET and list responses also returned user. PATCH with `type: null`
likewise left the type unchanged.

**Specification:** The [type schema][principal-type] is referenced by both
update bodies without an immutable-field restriction.

**Reproduction:** For a disposable principal created as user, PUT the
observed mutable settings back with `"type": "agent"`, or send:

```http
PATCH /v2/directories/{directoryId}/principals/{principalId}
Content-Type: application/json

{"type": null}
```

Both requests succeeded but retained user in the response and subsequent GET.

<a id="q10-principal-active-state-is-absent"></a>

### Q10 Principal active state availability

**US .tech retest (October 31, 2026):** Agent create with active=false,
detail/list reads, PUT false-to-true and PATCH true-to-false-to-true are
verified, including fresh GETs. User/agent/unclassified creation with false is
retained, and unrelated PUT/PATCH with active omitted preserves false. The
`.tech` backend limitation is cleared for directly provisioned records; other
provisioning paths and production rollout remain separate.

**Question:** Which releases and provisioning paths support reading and
setting `active`? US `.tech` supports both for directly provisioned principals,
while earlier US `.com` responses omitted it. Is that difference expected,
and how should callers detect support in a deployment?

**Observed:** Requests supplied true and false, but no response included
`active`. This does not establish whether the writes took effect.

**Additional observation (user-provided, October 31, 2026, `.tech`):** A
new agent created without an explicit active setting returns `active: true`
and `provisioned_by: konnect_user` on GET. This establishes field visibility
for that resource/deployment, but does not establish support for true/false
updates. What accounts for the difference from the earlier US `.com`
responses, and what deployment/provisioning contract governs availability?

An independent direct HTTP GET in US `.tech` subsequently confirmed that
same `active: true` value. No active-state mutation was attempted on the
manual demonstration principal.

**Specification:** The [active schema][principal-active] describes the
boolean state and its update/provisioning restrictions.

**Original US .com reproduction:** Create a disposable principal with
`"active": true`;
then PUT its observed mutable settings with `"active": false`. Inspect both
responses, GET that principal, and list its directory. None included active
in the tested deployment.

### Q11 Principal PATCH ignores ownership updates

**US .tech retest (October 31, 2026):** Partial/full management-map PATCH and
null removal still return 200 without changing stored values in `.tech`. The PUT
workaround remains necessary.

**Question:** Is principal PATCH expected to update `managed_by` today?
The specification exposes it, but a requested version change was ignored.
Are PATCH updates gated separately from PUT updates?

**Observed:** Principal PUT retained changed values in a five-entry
`managed_by` map. PATCH changing only the version entry succeeded but
returned the previous map.

**Reproduction:** After creating a disposable principal with
`managed_by.version` set to `probe-put`, send:

```http
PATCH /v2/directories/{directoryId}/principals/{principalId}
Content-Type: application/json

{"managed_by": {"version": "probe-patched"}}
```

The response and subsequent reads retained `probe-put`. PUT changing the
complete management metadata map did retain the requested values.

## Authorization-server child questions

### Q12 Duplicate claim names

**US .tech retest (October 31, 2026):** Two claims with the same name are
accepted in `.tech`. Matching by a duplicate name remains ambiguous.

**Question:** Are duplicate case-sensitive claim names within an authorization
server intentional? If multiple enabled claims with the same name apply to a
token or userinfo response, how is the resulting value selected?

**Observed:** Two POST requests to
`/v1/auth-servers/{authServerId}/claims` with `name: department` and different
values both returned 201 and different IDs. Scope creation, in contrast,
rejects a duplicate name with 409. Observed on October 31, 2026 UTC in US
`.com`; temporary records were removed.

### Q13 Claim pagination total exceeds result count

**US .tech retest (October 31, 2026):** Two `.tech` claims linked to three
scopes each return two records but meta.page.total=6. Page-size-one requests
exhaust the real records and then return empty pages. The pagination workaround
remains necessary.

**Question:** Should claim-list `meta.page.total` count claims, rather than
their scope associations? Can the total be corrected so it agrees with the
items returned across all pages?

**Observed:** An authorization server with two claims, each linked to three
scopes, returned two items with `page[size]=100` but reported total 6. With
`page[size]=1`, pages 1 and 2 returned one claim each; pages 3 through 7 were
empty and still reported total 6. Detail responses retained all three scope
IDs. A separate probe with three claims reported total 4. These results are
consistent with counting joined claim/scope rows; that explanation is an
inference, not a confirmed implementation detail.

**Reproduction:** Create three scopes and two claims, set each claim's
`include_in_scopes` to those three IDs, then GET
`/v1/auth-servers/{authServerId}/claims?page[size]=1&page[number]=1` and
increment the page number. Observed on October 31, 2026 UTC in US `.com`;
temporary records were removed.

### Q14 Creation retry contracts

**Question:** Do authorization-server claim creation and directory principal
creation support idempotency keys or another deduplication contract? After a
timeout or server error with an uncertain creation outcome, what is the
supported recovery procedure? Do the guarantees differ between these APIs?

**Known:** Claim names are not unique (Q12), so resending the same create body
can produce another record. No idempotency parameter is documented in the
inspected create operations. Principal display names are likewise non-unique,
and `managed_by` values have no documented uniqueness constraint. No failure
was injected into the live service.

### Q15 Client PATCH resets omitted authentication method

**US .tech retest (October 31, 2026):** A labels-only PATCH changes
client_secret_basic to client_secret_post in `.tech`; fresh GET confirms the
reset. Explicit preservation remains necessary.

**Question:** Should PATCH preserve `token_endpoint_auth_method` when the
field is omitted? Currently an unrelated update changes the authentication
method to `client_secret_post`. Is this defaulting intentional?

**Observed:** Create a client with
`token_endpoint_auth_method: client_secret_basic`, then PATCH its URL with
`{"labels":{"team":"platform"}}`. Both the PATCH response and subsequent
GET return `client_secret_post`. The same reset occurs from `none`.
Including the existing method explicitly in the PATCH preserves it.
These were direct HTTP requests, observed October 31, 2026 UTC in US `.com`.
Temporary records were removed.

### Q16 Client secret limit counts disabled records

**US .tech retest (October 31, 2026):** The third secret returns 403/max 2 even
when both existing records are disabled in `.tech`. Disabled records still
consume the two slots.

**Question:** Is the client-secret limit two records in total, including
disabled secrets, rather than two active secrets? What is the supported
rotation sequence when both slots already exist?

**Observed:** A client starts with one enabled secret. Creating a second,
disabled secret succeeds with 201. Creating a third disabled secret returns
403, even after disabling the first secret so that neither record is active.
Deleting the second record permits another creation. Creating a third enabled
secret also returns 403. Observed October 31, 2026 UTC in US `.com`;
temporary records were removed.

### Q17 Authorization-code grants unavailable

**US .tech retest (October 31, 2026):** A direct `.tech` PATCH requesting
authorization_code/refresh_token returns 400: authorization code grant type is
not allowed. This grant limitation remains in the tested organization.

**Question:** When are `authorization_code` and `refresh_token` grants
available for ordinary authorization-server clients? Are they gated by
environment, entitlement, or another setting?

**Observed:** PATCH of a test client with
`{"grant_types":["authorization_code","refresh_token"]}` returns 400 with
`grant_types: authorization code grant type is not allowed`. The inspected
schema enumerates both values. `client_credentials` creation succeeds.
Observed October 31, 2026 UTC in US `.com`; temporary records were removed.

### Q18 Client pagination total exceeds result count

**US .tech retest (October 31, 2026):** Two `.tech` clients with two allowed
scopes each return two records but meta.page.total=4. Continue observing
terminal pages rather than trusting that total.

**Question:** Should client-list `meta.page.total` count clients rather than
allowed-scope associations? Can it agree with the items across all pages?

**Observed:** Two clients, each with two `allow_scopes` IDs, report total 4.
At page size one, pages 1 and 2 contain one client each; later pages are
empty with total 4. This resembles the claim-list behavior in Q13; counting
joined associations is an inference. Observed October 31, 2026 UTC in US
`.com` using direct HTTP requests against disposable test fixtures.

### Q19 Recovery after uncertain generated-secret creation

**Question:** Do OAuth client and client-secret creation support idempotency
keys, request-outcome lookup, or another recovery contract after a timeout or
server error? Can an issued value be recovered, or must the affected secret
record be identified and explicitly revoked before creating a replacement?

**Known:** Client creation and separate secret creation return generated
material once; subsequent GET/list responses omit it. No idempotency parameter
is documented in the inspected operations. Separate secret creation consumes
one of two total slots (Q16). Synthetic client-side tests exercise uncertain
responses without retrying; no failure was injected into the live service.

### Q20 Client Basic authentication availability and enum annotation

**Question:** Is `client_secret_basic` generally available for authorization
server clients, and should its `TokenEndpointAuthMethod.x-enum-dev` annotation
be removed?

**Observed (October 31, 2026 UTC):** The canonical Sliver schema lists
`client_secret_basic` both in `enum` and `x-enum-dev`. US `.com` accepts it on
client create and PATCH. A client-credentials token request using HTTP Basic
succeeds with the initial issued secret, including after public configuration
and label updates that
explicitly preserve this method. Public specification filtering removes the
enum value despite that deployed behavior.

**Status:** Open. Confirm availability/gating and align the schema annotation
with the supported API contract. This does not resolve the separate omitted
method reset in Q15 or unavailable authorization-code grant in Q17.

## Principal identities and directory pagination

### Q21 Principal-identity detail and delete parent scoping

**US .tech retest (October 31, 2026):** A custom identity belonging to principal
A is returned through principal B's path with 200 in `.tech`. DELETE through B
returns 204 and removes A's identity. The collection-membership guard remains
necessary.

**Question:** Should identity GET and DELETE verify that the identity belongs
to the principal named in the request path, consistently with list and PUT?

**Observed (October 31, 2026 UTC, US `.com`):** Create two principals A and B
in one directory, then create custom identity I under A. Listing A's
identities contains I; listing B's identities is empty.

- GET `/v2/directories/D/principals/B/identities/I` returns 200 and I.
- PUT through B returns 404 and leaves I unchanged under A.
- DELETE through B returns 204 and actually deletes I from A.
- A missing principal or directory returns 404 on GET. Both parents used in
  the discrepancy above exist. OIDC GET also returns an identity through the
  other existing principal's path.

All resources were isolated probe fixtures and were removed afterward.
Confirm the expected parent-association validation for each method and whether
the detail/delete behavior will be aligned with the collection route.

### Q22 Cursor-page query serialization

**Question:** Should the shared `CursorPageQuery` parameter explicitly declare
`style: deepObject` and `explode: true` to match the deployed query contract?

**Observed (October 31, 2026 UTC, US `.com`):** The shared parameter is an
object named `page` without a declared style. Standard form/explode serialization
produces `?size=1&after=...`. The deployed directory, principal and identity
list endpoints ignore these flat keys: responses report the default size 20.
`?page[size]=1` returns size 1. Following identity list links with the
bracketed `page[after]` cursor retrieves distinct successive pages.
Principal API-key lists also accept the bracketed parameters and return
next-page URLs that traverse three distinct pages successfully.
Group and group-membership requests have the same flat SDK encoding defect;
direct bracketed queries retrieve successive pages of those collections too.

The missing annotation is in
`src/common/definitions/metadatas.yaml#/components/parameters/CursorPageQuery`.
Confirm the intended wire names and whether the shared definition should be
corrected for all operations referencing it.

### Q23 Principal identity quotas

**Question:** Are identity-count limits scoped per principal, directory or
organization, and how can an API client discover its applicable limit?

**Observed (October 31, 2026 UTC, US `.com`):** After ten custom identities had
been created under one isolated principal, another POST returned 403 with
`Maximum number of [identities] exceeded. Max allowed: [10]`. The response
does not identify the quota's scope. The inspected identity operation does
not expose quota discovery. All probe resources were removed afterward.

### Q24 Principal deletion with attached resources

**US .tech retest (October 31, 2026):** Native `.tech` principal DELETE succeeds
with API keys, a custom identity and a group membership attached; child
key/identity routes then return 404 and the group membership disappears. Group
DELETE with keys also succeeds, after which the key collection returns 404.
Runtime revocation was not established by these administrative observations.

**Question:** What happens to identities, credentials and group memberships
when a principal is deleted? Is there a conditional-delete option that
atomically rejects a principal with attached resources, or an equivalent
precondition that prevents resources added after a client checks for them
from being removed?

**Observed (October 31, 2026 UTC, US `.com`):** DELETE on an isolated principal
with OAuth-client and Gateway-consumer identities returns 204. A fresh
principal GET and identity collection GET both return 404. A second probe
also included an AI Gateway consumer identity and had the same result.
No force flag or nonempty-collection precondition is documented on the
inspected principal-delete operation. These observations establish that the
principal and its child collection become inaccessible; they do not establish
the physical deletion or retention policy for every descendant family.

Confirm which associated resources are deleted, detached or retained, and
whether independently managed associations can prevent parent deletion.
All probe resources were disposable and were removed afterward.

Additional API-key probes show that principal DELETE also succeeds with
generated and imported keys attached. The principal's key detail and collection
routes then return 404. Runtime revocation was not established because the
runtime endpoint requires a supported mTLS setup (Q30).

### Q25 Consumer identity workspace configuration

**Question:** When is `workspace_id` required or supported on a
`control_plane_consumer` identity? What is its format and scope, where can a
valid value be obtained, and does support differ between API Gateway and AI
Gateway consumers? Can you provide a working create/PUT example and describe
omission/clearing behavior?

**Observed (October 31, 2026 UTC, US `.com`):** Both consumer families work
when this field is omitted. Adding `workspace_id` to an otherwise working PUT
returns 500 for both a text value and a syntactically valid UUID; fresh GETs
retain the original association without the field. No actual workspace was
provisioned for these probes, so they do not establish that a valid workspace
is unsupported. The inspected schema defines only `type: string`, with no
description, format or availability constraint. Confirm the supported values
and whether invalid values should produce a validation error instead of 500.

### Q26 Identity behavior after deleting an associated client or consumer

**Question:** Which identity deletion or retention rules are guaranteed when
an associated OAuth client, Gateway consumer or control plane is deleted?
Should the API reject referenced-target deletion, remove the identity or
retain it, and are these rules consistent across API and AI Gateway consumers?

**Observed (October 31, 2026 UTC, US `.com`):** Deleting an associated OAuth
client returns 204; its identity subsequently returns 404 and disappears
from the principal's identity collection. Deleting an associated API Gateway
consumer returns 204 while its identity remains readable. Deleting that
control plane also returns 204 and leaves the same identity in both GET and
list, with its original IDs. No AI consumer/control-plane deletion policy was
established by these observations. All fixtures were removed afterward.

Confirm the intended lifecycle, including whether retained associations can
be distinguished from associations whose targets still exist.

### Q27 Principal types supported by API-key credentials

**US .tech retest (October 31, 2026):** API-key list and generated/imported
create succeed for fresh user, agent and unclassified principals in `.tech`. The
production user-principal 404 is not reproduced. This applicability blocker is
cleared for the tested `.tech` organization.

**Question:** Which principal types support API-key credentials? Should create
and list return "principal not found" for an existing `user` principal, or
should that type support keys or return an explicit unsupported-type error?

**Observed (October 31, 2026 UTC, US `.com`):** Fresh principals created as
`user`, `agent`, explicit null and omitted type all return 200 from principal
GET. API-key create/list return 404 for the `user`, with detail "The requested
principal was not found". The same operations succeed for `agent` and both
unclassified variants. Generated creation retains all five supplied management
entries. This comparison used the same directory and administration token.

### Q28 API-key PATCH metadata and expiration omission

**US .tech retest (October 31, 2026):** Principal and group key PATCH still
ignore partial/full ownership maps and null removal. Omitting expiration clears
it; explicit future expiration and null work. This remains a live `.tech`
limitation.

**Question:** Does API-key PATCH support `managed_by` changes, and should an
omitted `expires_at` preserve expiration? What operation can change management
metadata without changing credential material or its expiration?

**Observed (October 31, 2026 UTC, US `.com`):** Generated creation retains a
future expiration and the supplied management map. PATCH with a partial map,
then with a complete five-entry map changing version/ref/protection, returns
200 but GET retains the original map. Imported-key PATCH likewise leaves its
empty map unchanged. An expiration-only PATCH works, and explicit null clears
expiration. Omitting expiration from a metadata-only PATCH also clears it.
Sending the existing expiration explicitly preserves it. PUT returns 405.

Confirm supported merge/removal semantics and whether successful responses
should reject unsupported metadata changes instead of ignoring them.

### Q29 API-key name availability

**US .tech retest (October 31, 2026):** Principal generated/imported names are
accepted and returned; PATCH renames both. Duplicate names under one principal
are accepted, so names do not supply uniqueness. Group generated create accepts
a name, but rename PATCH leaves it unchanged; group imported create ignores name
and reads null, and rename is ignored. Availability is
parent/operation-specific.

**Question:** When will API-key `name` be supported consistently on create,
read and update? Until then, should PATCH reject the field explicitly?

**Observed (October 31, 2026 UTC, US `.com`):** The inspected upcoming schema
includes nullable names. Generated create with a nonempty name returns 400
because `name` is unsupported; the same request without name succeeds. GET and
list omit name. PATCH with a nonempty name or null returns 200, but fresh GET
still omits it. No uniqueness rule was inferred.

### Q30 Runtime authentication prerequisites

**US .tech retest (October 31, 2026):** A direct request to
us.identity.konghq.tech/default/v2/authenticate with a valid imported probe key
and an administrative bearer token still returns 400, mTLS headers missing. No
context headers were fabricated. A supported runtime integration setup remains
necessary.

**Question:** What is the supported way to call a directory's runtime
`/{directoryName}/v2/authenticate` endpoint from an administrative integration
test? Which client certificate, control-plane association and organization
context are required? Can you provide a working request example?

**Observed (October 31, 2026 UTC, US `.com`):** Generated and imported keys can
be created and inspected through administration endpoints. Requests to
`us.identity.konghq.com` using an administration bearer token return 400 with
"mTLS headers missing". This does not establish whether either key can
authenticate. No internal mTLS context headers were fabricated. A supported
runtime setup is needed to verify credential continuity and revocation.

### Q31 Integer precision in principal and group metadata

**US .tech retest (October 31, 2026):** Exact JSON comparisons reproduce
9007199254740993 -> 9007199254740992, its negative/array variants, and
1152921504606846976 -> 1152921504606847000 in principal PUT and group create.
The numeric guard remains necessary.

**Question:** What integer range and precision are guaranteed for metadata on
principals and groups, including integer arrays and propagation to the data
plane? Can requests retain the exact integer digits instead of silently
rounding values during JSON conversion?

**Observed (October 31, 2026 UTC, US `.com`):** Group create/PUT and principal
PUT accept integer metadata but can return different digits. Fresh GETs for
user, agent and unclassified principals return `9007199254740992` after
receiving `9007199254740993`; the same change occurs in integer arrays and for
the negative value. `9007199254740991`, `9007199254740992` and
`9007199254740994` return unchanged. `1152921504606846976` returns as
`1152921504606847000`. Both int64 extrema are also rounded, producing digits
outside their original int64 bounds. These comparisons parse raw JSON using
integer-preserving decoding rather than floating-point assertions.

The observed results match a binary64 conversion followed by decimal JSON
serialization; the internal representation has not been established. Confirm
the supported contract and validation behavior for values that cannot retain
their requested digits. All disposable fixtures were removed afterward.

### Q32 Group membership types and team removal

**US .tech retest (October 31, 2026):** Ordinary groups accept
user/agent/unclassified members; duplicates return 409, nested groups and
self-membership return 400. Creating a team returns 400, type must be one of:
group. Therefore the production manual-team results cannot be generalized to
`.tech`; importer behavior remains unverified.

**Question:** Which group-to-group memberships are currently supported, and
which team memberships may be created or removed through the membership API?
How can clients distinguish manual relationships from importer-managed ones?

**Observed (October 31, 2026 UTC, US `.com`):** A plain group accepts user,
agent and unclassified principal members and rejects a duplicate member with
409. Adding another group returns 400, "groups cannot be members of a group",
although the inspected schema permits groups. A team accepts a user principal,
but rejects agent/unclassified members. A self-membership is rejected.
Membership detail reads use the generated entry ID and reject another group's
path. A user membership created manually under a directly created team can
also be deleted by its entry ID. No import was involved in that probe. The
schema describes team membership as importer-managed, including a removal
restriction; confirm how that differs for manual and imported teams before
clients infer authority over their relationships.

Further direct tests confirm ordinary-group membership DELETE returns 204,
then detail returns 404. Principal GET adds/removes the group ID as membership
changes. Deleting a member principal removes its membership entry from the
group collection. Missing-directory group/membership routes return 404. The
test organization has a one-directory quota, so two existing directories could
not be used to establish cross-directory behavior; that scope remains unproved.

### Q33 Group API-key deployment availability

**US .tech retest (October 31, 2026):** Group-key list/create/read/PATCH/delete
are available in `.tech`, replacing the production 501 result. Generated
creation retains ownership; imported creation rejects it and later ownership
PATCH is ignored. Name updates differ from principal keys (Q29). Native parent
deletion removes administrative access to keys (Q24).

**Question:** In which deployment/release are group API-key operations
available, and what response should identify an unsupported capability?

**Observed (October 31, 2026 UTC, US `.com`):** GET and POST on an existing
plain group's `/api-keys` collection return 501 with no response body. The
group itself supports create, get, list, PUT and delete. Schema and generated
SDK presence therefore do not establish group-key availability.

### Q34 Auth-server and OAuth-client management metadata

**US .tech retest (October 31, 2026):** Auth-server and OAuth-client POST accept
an extra managed_by property but do not expose it in the response or subsequent
reads; inspected schemas still do not expose it. Labels remain the verified
management metadata field. Successful acceptance of an unknown field does not
establish a supported ownership write.

**Question:** Is `labels` the intended long-term metadata field for identifying
the system managing auth servers and OAuth clients, or is `managed_by` planned
for these resources? If it is planned, what coexistence and migration contract
should clients use for management metadata already stored in labels?

**Known:** The inspected auth-server and OAuth-client schemas expose `labels`
but do not expose `managed_by`, unlike the directory administration schemas.
Label updates work in the verified lifecycle. This is a question about the
intended metadata contract; working label storage does not depend on adding a
new field.

## Recorded answers

No team answers have been recorded yet.

[vault-definition]: https://github.com/Kong/platform-api/blob/98137d7be3772b67693cc1ceb0469e77f8ecf0f5/src/konnect/definitions/kong-identity/v2/openapi.yaml#L3579-L3583
[vault-create]: https://github.com/Kong/platform-api/blob/98137d7be3772b67693cc1ceb0469e77f8ecf0f5/src/konnect/definitions/kong-identity/v2/openapi.yaml#L3551-L3552
[vault-replace]: https://github.com/Kong/platform-api/blob/98137d7be3772b67693cc1ceb0469e77f8ecf0f5/src/konnect/definitions/kong-identity/v2/openapi.yaml#L3575-L3576
[vault-update]: https://github.com/Kong/platform-api/blob/98137d7be3772b67693cc1ceb0469e77f8ecf0f5/src/konnect/definitions/kong-identity/v2/openapi.yaml#L3604-L3605
[vault-response]: https://github.com/Kong/platform-api/blob/98137d7be3772b67693cc1ceb0469e77f8ecf0f5/src/konnect/definitions/kong-identity/v2/openapi.yaml#L4217-L4218
[directory-spec]: https://github.com/Kong/platform-api/blob/98137d7be3772b67693cc1ceb0469e77f8ecf0f5/src/konnect/definitions/kong-identity/v2/openapi.yaml#L3511-L3519
[principal-type]: https://github.com/Kong/platform-api/blob/98137d7be3772b67693cc1ceb0469e77f8ecf0f5/src/konnect/definitions/kong-identity/v2/openapi.yaml#L4353-L4363
[principal-active]: https://github.com/Kong/platform-api/blob/98137d7be3772b67693cc1ceb0469e77f8ecf0f5/src/konnect/definitions/kong-identity/v2/openapi.yaml#L4431-L4437
