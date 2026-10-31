# Kong Identity API team questions

Questions about observed HTTP behavior and API resource contracts. Record each
team response with its date, applicable environment, and API version.

Unless stated otherwise, observations were collected on October 30, 2026
(US Central), using direct HTTP requests to `https://us.api.konghq.com`.
Specification links are pinned to the inspected platform-api PR 3877 revision.
Temporary test resources were deleted after the probes.

## Directory API questions

### Q1 Directory Token Vault enablement

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

### Q3 Directory quota and control plane access

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

**Question:** Can imported principal and group API keys accept `managed_by`
in the initial create request, so the key and its management metadata are
created atomically?

The inspected imported-key create body does not expose `managed_by`. Is
there another supported way to attach it during creation?

### Q5 Ownership update semantics

**Question:** For principals, identities, and credentials, which operations
replace the whole `managed_by` map, which merge keys, and how are keys removed?
Are management metadata updates supported for every resource that exposes it?

**Observed:** Directory and principal PUT requests retained the supplied
management metadata values. Principal PATCH behaved differently; see Q11.
Omission and removal semantics still need confirmation across resource types.

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

### Q10 Principal active state is absent

**Question:** When will principal create/GET/list/update responses expose
`active`, and which deployed environments support setting it? If writes are
supported before the field is returned, how can their effective value be
verified?

**Observed:** Requests supplied true and false, but no response included
`active`. This does not establish whether the writes took effect.

**Specification:** The [active schema][principal-active] describes the
boolean state and its update/provisioning restrictions.

**Reproduction:** Create a disposable principal with `"active": true`;
then PUT its observed mutable settings with `"active": false`. Inspect both
responses, GET that principal, and list its directory. None included active
in the tested deployment.

### Q11 Principal PATCH ignores ownership updates

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

**Question:** Are duplicate case-sensitive claim names within an authorization
server intentional? If multiple enabled claims with the same name apply to a
token or userinfo response, how is the resulting value selected?

**Observed:** Two POST requests to
`/v1/auth-servers/{authServerId}/claims` with `name: department` and different
values both returned 201 and different IDs. Scope creation, in contrast,
rejects a duplicate name with 409. Observed on October 31, 2026 UTC in US
`.com`; temporary records were removed.

### Q13 Claim pagination total exceeds result count

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

**Question:** When are `authorization_code` and `refresh_token` grants
available for ordinary authorization-server clients? Are they gated by
environment, entitlement, or another setting?

**Observed:** PATCH of a test client with
`{"grant_types":["authorization_code","refresh_token"]}` returns 400 with
`grant_types: authorization code grant type is not allowed`. The inspected
schema enumerates both values. `client_credentials` creation succeeds.
Observed October 31, 2026 UTC in US `.com`; temporary records were removed.

### Q18 Client pagination total exceeds result count

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
