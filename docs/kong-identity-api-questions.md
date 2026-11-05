# Kong Identity API questions — US .tech

Based on HTTP observations from October 31–November 1, 2026.

Administration: `https://us.api.konghq.tech`
Runtime: `https://us.identity.konghq.tech`

Single-letter route placeholders represent resource IDs. Questions about
untested guarantees are identified as contract clarifications.

## Runtime integration

<a id="q30-runtime-authentication-prerequisites"></a>

### Q1 Runtime authentication prerequisites

`POST /{directoryName}/v2/authenticate`
`POST /{directoryName}/v2/vault/enrollment/resolve`

Authentication with an imported API key and an administrative bearer token
returns 400, "mTLS headers missing". Enrollment resolution with an issued
signed token and an administrative bearer returns the same error.

**Question:** What client certificate, control-plane association and
organization context do these endpoints require? Please provide a supported
working request example for authentication and OAuth enrollment.

## Principals and credentials

<a id="q9-principal-type-changes"></a>

### Q2 Principal type changes

`PUT /v2/directories/D/principals/P`

Changing a user to an agent returns 400, "type is immutable after creation".
Setting `type: null` returns 200 but retains the original type.

**Question:** Should all attempted type changes, including null, return an
immutable-field error?

<a id="q11-principal-patch-ignores-ownership-updates"></a>

### Q3 Principal PATCH ignores ownership updates

`PATCH /v2/directories/D/principals/P`

Partial and complete `managed_by` changes, including key removal with null,
return 200 without changing the stored map. Full PUT retains supplied changes.

**Question:** Should PATCH update `managed_by`, or reject unsupported changes?
What are the intended merge, replacement and key-removal rules?

### Q4 Imported API key ownership

`POST /v2/directories/D/principals/P/api-keys`
`POST /v2/directories/D/groups/G/api-keys`

Imported-key creation rejects `managed_by` with 400. Creation without it works;
PATCH does not attach the map afterward. Generated-key creation accepts it.

**Question:** Can imported keys accept management metadata atomically during
creation, or is another operation intended for this?

<a id="q28-api-key-patch-metadata-and-expiration-omission"></a>

### Q5 API-key PATCH metadata and expiration omission

`PATCH /v2/directories/D/principals/P/api-keys/K`
`PATCH /v2/directories/D/groups/G/api-keys/K`

For generated and imported keys, PATCH ignores `managed_by` changes and null
removal. Omitting `expires_at` clears expiration; explicit dates and null work.

**Question:** Should PATCH apply management-map changes and preserve omitted
expiration? Which operation can update metadata without changing key material?

<a id="q29-api-key-name-availability"></a>

### Q6 API-key name availability

Principal keys support names and renames, but `name: null` returns 200 without
clearing the name. Generated group keys accept a name at creation; imported
group keys ignore it. Group-key PATCH ignores both renames and null.

**Question:** What name-setting and clearing rules apply to each key type?
Should unsupported changes return an error instead of success?

<a id="q35-basic-auth-and-password-metadata-updates"></a>

### Q7 Basic-auth and password metadata updates

`PATCH /v2/directories/D/principals/P/basic-auths/B/passwords/W`

Password creation retains `managed_by`. PATCH ignores map changes and clears
an omitted `expires_at`. Password PUT requires secret material. In contrast,
a basic-auth username record can update its map through PUT without passwords.

**Question:** Which request updates password management metadata while
preserving its existing secret and expiration? Should PATCH preserve omitted
expiration and apply the supplied map changes?

<a id="q31-integer-precision-in-principal-and-group-metadata"></a>

### Q8 Integer precision in principal and group metadata

Principal PUT and group creation accept `9007199254740993` but return and
store `9007199254740992`. Negative values and integer arrays also lose digits.

**Question:** What integer precision is guaranteed? Can unsupported values be
rejected rather than silently rounded, including during data-plane propagation?

## Resource relationships and deletion

<a id="q21-principal-identity-detail-and-delete-parent-scoping"></a>

### Q9 Principal-identity detail and delete parent scoping

`GET /v2/directories/D/principals/B/identities/I`
`DELETE /v2/directories/D/principals/B/identities/I`

Identity I belongs to principal A. GET through another existing principal B
in the same directory returns I with 200. DELETE through B returns 204 and
removes A's identity.

**Question:** Should these operations verify that the identity belongs to the
principal in the path?

<a id="q24-principal-deletion-with-attached-resources"></a>

### Q10 Principal deletion with attached resources

`DELETE /v2/directories/D/principals/P`
`DELETE /v2/directories/D/groups/G`

Principal deletion succeeds with identities, API keys, basic-auth records,
passwords and group memberships attached. Child routes then return 404 and
memberships disappear. Group deletion also succeeds with API keys attached.
These observations do not establish runtime credential revocation.

**Question:** Which descendants are deleted, detached or retained? Is there a
conditional delete that atomically rejects a parent with attached resources?

<a
  id="q37-directory-deletion-with-trusted-idps-and-disabled-vault-visibility"
></a>

### Q11 Directory deletion with Trusted IdPs and disabled Vault visibility

`DELETE /v2/directories/D?force=false`

Attached principals or groups cause 409. Attached Trusted IdPs, Vault keys or
providers without credentials do not: deletion returns 204 in either Vault
state. Disabling Vault hides those resources behind 403 responses.

**Question:** Which dependencies should block non-forced deletion? How can
they be inspected without enabling Vault? Is an atomic nonempty-dependency
precondition available?

<a id="q41-provider-deletion-with-an-attached-shared-credential"></a>

### Q12 Provider deletion with an attached shared credential

`DELETE /v2/directories/D/vault/providers/V`

Deleting a provider with an active static shared credential returns 204; the
credential's detail route then returns 404. User-enrolled credentials were
not tested.

**Question:** Is this cascade intentional, and does it also affect user
authorizations? Can deletion atomically reject providers with credentials?

<a id="q44-trusted-idp-deletion-and-existing-principal-associations"></a>

### Q13 Trusted IdP deletion and existing principal associations

`DELETE /v2/directories/D/trusted-idps/T`

Deletion returns 204 while associated principals exist. The principals and
their OIDC identities are readable afterward with the original issuer.
Runtime authentication after trust removal was not tested.

**Question:** What authentication or Vault access is invalidated by trust
deletion? Is a conditional delete available while associations exist?

<a id="q52-principal-identity-read-after-create-visibility"></a>

### Q14 Principal-identity read-after-create visibility

`POST /v2/directories/D/principals/P/identities`
`GET /v2/directories/D/principals/P/identities`

In one captured sequence, OIDC identity creation returned 201, followed
immediately by an empty parent-scoped list. Twenty subsequent direct trials
returned the new identity in the first list; the incident was not reproduced.

**Question:** What read-after-write visibility guarantee follows a successful
create? If visibility can lag, what verification interval is recommended?

<a id="q14-creation-retry-contracts"></a>

### Q15 Creation retry contracts

Principal display names and auth-server claim names are non-unique. A
successful create can therefore be difficult to identify after a lost response.
Timeout and server-error outcomes were not injected into the service.

**Contract clarification:** Do these POST operations support idempotency keys
or request-outcome lookup? What recovery procedure is supported after an
uncertain creation result?

<a id="q19-recovery-after-uncertain-generated-secret-creation"></a>

### Q16 Recovery after uncertain generated-secret creation

OAuth client, client-secret and password creation return secret material;
later reads omit it. Successful creation was verified; lost-response recovery
was not tested against the service.

**Contract clarification:** Can creation be deduplicated or its outcome
queried? Can issued material be recovered, or must the affected record be
identified and revoked before issuing a replacement?

## Authorization servers

<a id="q12-duplicate-claim-names"></a>

### Q17 Duplicate claim names

`POST /v1/auth-servers/A/claims`

Two claims with the same name and different values are accepted.

**Question:** Are duplicate claim names intentional? If multiple enabled
claims with that name apply, which value appears in tokens or userinfo?

<a id="q13-claim-pagination-total-exceeds-result-count"></a>

### Q18 Claim pagination total exceeds result count

`GET /v1/auth-servers/A/claims?page[size]=1&page[number]=1`

Two claims linked to three scopes each report `meta.page.total: 6`.
Only two claims are returned across the pages; subsequent pages are empty.

**Question:** Should the total count claims rather than scope associations?

<a id="q18-client-pagination-total-exceeds-result-count"></a>

### Q19 Client pagination total exceeds result count

`GET /v1/auth-servers/A/clients?page[size]=1&page[number]=1`

Two clients with two allowed scopes each report `meta.page.total: 4`.
Only two clients are returned across the pages.

**Question:** Should the total count clients rather than allowed-scope
associations?

<a id="q15-client-patch-resets-omitted-authentication-method"></a>

### Q20 Client PATCH resets omitted authentication method

`PATCH /v1/auth-servers/A/clients/C`

A labels-only PATCH changes `token_endpoint_auth_method` from
`client_secret_basic` to `client_secret_post`. Including the existing method
explicitly preserves it.

**Question:** Should PATCH preserve the authentication method when omitted?

<a id="q16-client-secret-limit-counts-disabled-records"></a>

### Q21 Client secret limit counts disabled records

Creating a third client secret returns 403, maximum two, even when both
existing records are disabled.

**Question:** Is the limit two total records rather than two active secrets?
What rotation sequence is supported when both slots are occupied?

<a id="q17-authorization-code-grants-unavailable"></a>

### Q22 Authorization-code grants unavailable

A client PATCH requesting `["authorization_code","refresh_token"]` returns
400, "authorization code grant type is not allowed". Client-credentials
creation and authentication work.

**Question:** What enables authorization-code and refresh-token grants?
Are they controlled by configuration or entitlement?

<a id="q34-auth-server-and-oauth-client-management-metadata"></a>

### Q23 Auth-server and OAuth-client management metadata

Auth-server and OAuth-client POST accept an extra `managed_by` property, but
responses and subsequent reads do not include it. Their schemas expose labels.

**Question:** Are labels the intended management metadata field, or is
`managed_by` planned? If planned, how should existing label metadata migrate?

## Vault providers and credentials

<a id="q6-vault-and-trusted-idp-ownership"></a>

### Q24 Vault and trusted IdP ownership

Vault keys, providers, provider credentials and Trusted IdPs have no `labels`
or `managed_by` field in the inspected schemas or US `.tech` responses.

**Question:** Is this omission intentional? Is support for a writable
`managed_by` field planned for these resources?

<a id="q38-trusted-idp-jwks-override-clearing-through-patch"></a>

### Q25 Trusted IdP JWKS override clearing through PATCH

`PATCH /v2/directories/D/trusted-idps/T`

`{"jwks_uri":null}` returns 200 but retains the override. Full PUT omitting
the field clears it. PATCH with `display_name: null` does clear that field.

**Question:** Should null clear the JWKS override through PATCH and restore
issuer discovery?

<a id="q39-replacing-a-custom-oauth-dcr-provider"></a>

### Q26 Replacing a custom OAuth DCR provider

`PUT /v2/directories/D/vault/providers/V`

A custom OAuth provider created with DCR enabled cannot be replaced using the
same template: PUT returns 400 about a missing registration endpoint. The PUT
schema exposes neither `dcr_enabled` nor `registration_endpoint`.

**Question:** How can an existing DCR provider be replaced while preserving
registration configuration? Are these fields intentionally create-only?

<a id="q40-provider-initial-enabled-state-and-put-omission"></a>

### Q27 Provider initial enabled state and PUT omission

Provider POST with `enabled: false` creates an enabled provider; the create
schema has no enabled field. PATCH can disable it. PUT preserves an omitted
enabled value but resets an omitted `refresh_buffer_seconds` to zero.

**Question:** Is atomic disabled creation supported? Which PUT fields
preserve omitted values, and which reset to defaults?

<a id="q42-clearing-provider-scopes-and-endpoint-overrides"></a>

### Q28 Clearing provider scopes and endpoint overrides

`PATCH /v2/directories/D/vault/providers/V`

Creating a provider with `scopes: []` selects template defaults; PATCH with
`scopes: []` returns 400. Null endpoint overrides return 400, while empty
strings return 200 and persist as empty effective endpoints.

**Question:** How can an override be removed to inherit template endpoints?
How can an explicitly empty scope list be distinguished from template defaults?

<a id="q43-settings-accepted-by-static-secret-providers"></a>

### Q29 Settings accepted by static-secret providers

Static-secret provider POST/PATCH accept and persist OAuth-related settings,
including scopes, PKCE and endpoint URLs. The template guidance instead says
to supply only name and shared credential type. Runtime effects were not tested.

**Question:** Which settings apply to static-secret providers? Should
inapplicable OAuth settings be rejected or documented as having no effect?

<a id="q45-shared-credential-cardinality-and-routing-uniqueness"></a>

### Q30 Shared credential cardinality and routing uniqueness

`POST /v2/directories/D/vault/providers/V/credentials`

Static and shared OAuth providers accept multiple shared credentials. A
routing priority can be occupied once, with one separate default slot.
Selectors can repeat at different priorities. The POST description instead
says static-secret providers hold at most one credential.

**Question:** Can the per-provider cardinality, priority uniqueness and default
slot rules be documented for both credential types?

<a id="q46-credential-put-routing-and-material-preservation-behavior"></a>

### Q31 Credential PUT routing and material-preservation behavior

`PUT /v2/directories/D/vault/providers/V/credentials/C`

Static credential PUT requires secret material even for routing-only changes.
Omitting routing fields selects the default slot; explicit null is rejected.
Shared OAuth PUT and detail GET issue enrollment URLs; the tested OAuth
records were pending.

**Question:** Which operation changes routing while preserving static secret
material or an active OAuth authorization? What triggers re-enrollment?

<a id="q47-shared-oauth-enrollment-url-lifetime-and-invalidation"></a>

### Q32 Shared OAuth enrollment URL lifetime and invalidation

`GET /v2/directories/D/vault/providers/V/credentials/C`

Each GET of a pending shared OAuth credential returns an enrollment URL.
Issued URLs were not followed, so their lifetime and invalidation were not
verified.

**Question:** How long is a URL valid, can it be reused, and what invalidates
it: another GET, completed consent, credential deletion or provider deletion?
What encryption-key setup is required to complete enrollment?

## Directories and groups

<a id="q2-default-directory-lifecycle"></a>

### Q33 Default directory lifecycle

One test organization initially returned no directories.

**Question:** Is a directory named `default` automatically provisioned for
every organization? Does it have special behavior beyond its name, and may
it be renamed or deleted?

<a id="q3-directory-quota-and-control-plane-access"></a>

### Q34 Directory quota and control plane access

The test organization rejects a second directory with 403, maximum one.

**Question:** How can an organization discover its directory quota? Which
organizations can restrict directory access to a control-plane allowlist?

<a id="q32-group-membership-types-and-team-removal"></a>

### Q35 Group membership types and team removal

Direct team creation returns 400, allowing only `group`; organization-chart
imports do create teams. Ordinary groups accept principal members, while
group-to-group and self-memberships return 400.

**Question:** Which team and nested-group operations are supported? Which team
relationships may be changed through the membership API, and how are imported
relationships identified?

## Organization-chart import

<a id="q48-canonical-organization-chart-csv-contract"></a>

### Q36 Canonical organization-chart CSV contract

`POST /v2/directories/D/org-chart/import`

Required columns are `external_id`, `email`, `given_name`, `family_name` and
`manager_external_id`. Missing imported users become inactive. Omitting the
`cost_center` column preserves values; blank cells remove them. Invalid files
can return HTTP 200 with row errors.

**Question:** Can the CSV schema, normalization, manager/root validation and
whole-file synchronization rules be documented? Can separate import sources
have independent scopes?

<a id="q49-import-adoption-and-provisioning-transitions"></a>

### Q37 Import adoption and provisioning transitions

Import matches an API-created user by email, retains its UUID and
`managed_by`, and changes provisioning to `imported`. Ordinary PUT/DELETE
then return 409. Explicit deactivation changes provisioning to `konnect_user`
and allows deletion.

**Question:** What is the matching precedence between external ID and email?
Can existing managed users be excluded from adoption? What is the supported
release/re-import lifecycle?

<a id="q51-returning-imported-users-remain-inactive"></a>

### Q38 Returning imported users remain inactive

Adding a previously omitted user back to an import restores membership and
keeps the UUID, but leaves `active:false` and `provisioned_by: konnect_user`.
The report counts the user as unchanged with `inactive_not_reactivated`.

**Question:** Is this intentional when import caused the deactivation? Which
explicit action restores active/imported state, and how do later imports
treat that action?

## Public API specification

<a id="q20-client-basic-authentication-availability-and-enum-annotation"></a>

### Q39 Client Basic authentication availability and enum annotation

HTTP Basic client-credentials authentication works with
`token_endpoint_auth_method: client_secret_basic`, but the method is marked
`x-enum-dev` in the specification.

**Question:** Should this working method be included in the public enum?
What availability restriction, if any, does the annotation represent?

<a id="q22-cursor-page-query-serialization"></a>

### Q40 Cursor-page query serialization

List endpoints honor `page[size]` and `page[after]`. A flat `size=1` is ignored
and returns the default page size 20. The shared `CursorPageQuery` parameter
does not declare its object style.

**Question:** Should it specify `style: deepObject` and `explode: true` so the
published query contract describes the bracketed parameter names?

<a id="q50-public-import-response-schema"></a>

### Q41 Public import response schema

Import-created principals/teams return `provisioned_by: imported`, but that
value is marked `x-enum-dev`; the import description instead says
`hris_import`. Reports also return `counts.users.adopted`, absent from the
inspected public report schema.

**Question:** Can the public enum, import description and report schema match
the returned provisioning value and adoption count?
