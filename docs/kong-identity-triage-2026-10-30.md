# Kong Identity support in KongCTL: triage report

Date: October 30, 2026

KongCTL branch: `ki-triage`
KongCTL commit inspected: `2beef7be`

## Current branch status

The initial survey below is historical. Development on `ki-triage` has since
delivered directories and principals, including declarative lifecycle, reads,
adoption, examples, and live E2E scenarios. Principal work is pushed in
`c1e3cba3`. Authorization-server root support is the next implemented slice;
its full live lifecycle scenario and protected-resource teardown pass.
Scopes and claims now have declarative lifecycle support as managed-server
children, plus get/list, examples and live E2E coverage. Their issues #1402 and
#1403 remain In Progress until merge. OAuth clients and secrets remain
separate implementation work. Sections 19–20 record contracts and questions.

The public preview SDK is now pinned to `dfd890eb1a03`, including targeted
serialization fixes for clearing auth-server origins and claim scope links.
Existing KongCTL SDK patches are retained. This continues the selective
public SDK approach rather than switching to the internal SDK's unrelated
model differences.

Keep these two running logs alongside this report:

- [API questions for engineering](kong-identity-api-questions.md), phrased in
  terms of observed API behavior and resource contracts.
- [Metadata storage ledger](contributor/kong-identity-metadata-storage.md),
  distinguishing labels, `managed_by`, and resources without independent
  metadata storage. It separates schema capability from verified behavior.

[Compatibility tracking](contributor/kong-identity-compatibility.md) records
temporary principal restrictions and criteria for removing them. Work remains
on branches; issues stay In Progress until merge to main. No PR is open for
these implementation slices.

## Executive assessment

We can resume Kong Identity work without waiting for server-side
`managed_by` filtering. Client-side pagination and matching are acceptable
under the stated project trade-off. Several other concerns in the earlier
lookup report have also improved.

The recommendation is to start from current main, selectively reuse the
paused directory implementation, and deliver support in stages. Remaining
limitations vary by resource; they do not justify blocking all Kong Identity
support.

Latest SDK result: a selective public SDK preview now includes the planned
Identity APIs, and unchanged KongCTL builds and passes all unit tests against
it. [Section 14](#14-selective-public-sdk-preview) records the branches,
dependency version, validation, and remaining limits.

This assessment comes from GitHub issues and PRs, local API and SDK sources,
current KongCTL code, and the paused branches. API behavior was not verified
against a live environment, and the paused branches were not built or tested.
The initial investigation changed no code. Later authorized issue updates
and SDK experiments are documented below. The `ki-triage` worktree now uses
the preview SDK for Identity development, as recorded in Section 14.

## 1. API landscape

There are three distinct API surfaces:

| API surface | Purpose |
| --- | --- |
| Authorization-server administration | Configure OAuth/OIDC token issuers, clients, scopes, claims, and client authentication. |
| Directory administration | Manage principals, external identity links, credentials, groups, and token-vault configuration. |
| Runtime authentication and lookup | Authenticate credentials, resolve principals, and perform runtime identity operations. |

The canonical sources inspected were:

- [Authorization-server specification][auth-spec]:
  `src/konnect/definitions/sliver/v1/openapi.yaml`.
- [Directory specification][directory-spec]:
  `src/konnect/definitions/kong-identity/v2/openapi.yaml`.
- [Runtime specification][runtime-spec]:
  `src/konnect/definitions/kong-identity-auth/v2/openapi.yaml`.

"Sliver" is the internal specification name for the authorization-server API.
Its management routes are under `/v1/auth-servers`. Directory management uses
`/v2/directories`. KongCTL's initial work primarily concerns those two
administration surfaces.

The local directory source was specification version `1.24.1`. The live head
of [platform-api PR #3877][platform-pr] was version `1.28.0`, commit
`98137d7be3772b67693cc1ceb0469e77f8ecf0f5`. The PR head was inspected for
forthcoming public coverage; local source alone would understate it. The PR
was open at the time of inspection. Schema presence does not by itself prove
deployment or SDK availability.

## 2. Issue map and intended scope

[#1132][issue-1132] is the directory epic. [#1400][issue-1400] tracks the
auth-server resource itself; its broader parent is [#1133][issue-1133].

| Family | Tracking issues |
| --- | --- |
| Directory SDK prerequisite | #1413 |
| Directories | #1414 |
| Principals | #1415 |
| Principal identities | #1416 |
| Principal API keys | #1417 |
| Basic-auth records and passwords | #1418 |
| Principal CSV import | #1419 |
| Auth-server SDK prerequisite | #1399 |
| Auth servers | #1400 |
| Auth-server clients | #1401 |
| Auth-server scopes | #1402 |
| Auth-server claims | #1403 |
| Client-secret lifecycle | #1404 |

The issues request declarative lifecycle support, including plan, diff,
apply, sync, delete, dump, explain, and scaffold. They also request get/list
and adoption support. Directory work includes an ergonomic `default`
directory workflow, while still supporting explicitly selected directories.

Secret-related issues qualify declarative support where a safe
representation is possible. Client-secret management specifically leaves
room for an imperative lifecycle.

## 3. Concepts and resource relationships

### Directory, principal, identity, and credential

A directory is a regional boundary holding principals. It also controls
which control planes may authenticate against it and can enable Token Vault.

A principal represents an entity accessing services: a person, workload,
application, or agent. Its metadata can drive gateway policy. It is distinct
from a Konnect administrative user or Dev Portal developer account.

An **identity** connects that principal to an identifier recognized elsewhere:

- An OIDC issuer and claim name/value.
- A Kong Identity auth-server/client pair.
- A Gateway control-plane/Consumer pair, with workspace information where
  applicable.
- A custom key/value pair.

A **credential** is authentication material the entity presents, such as an
API key or username/password. One principal can have multiple identities and
credentials, including multiple of the same type. They are separate
resources with separate lifecycles.

For example, a service can have one principal containing policy metadata,
an identity linking it to an OAuth client, and an API-key credential for a
different authentication path.

Source: [principal concepts][principal-docs] and [directory specification][directory-spec].

### Authorization server, client, scope, claim, and secret

An authorization server issues OAuth/OIDC tokens. It configures audience,
signing algorithm, issuer/discovery information, trusted origins, and Dynamic
Client Registration settings.

A client represents the application requesting tokens. It configures grant
types, response types, redirect URIs, token durations, authentication method,
and allowed scopes.

A scope is a named permission. A claim supplies a token or userinfo
attribute, using a static or server-evaluated template value. Scopes and
claims belong to the auth server, rather than to an individual client.

Clients reference allowed scope IDs through `allow_scopes`, or allow all
scopes. Claims reference scope IDs through `include_in_scopes`, or apply to
all scopes. KongCTL should resolve local references to those IDs.

A default scope is included without being explicitly requested only when
the client is allowed that scope. Disabled scopes cannot be requested.
Discovery visibility is a separate setting from client authorization.

A client secret authenticates the OAuth client. It is distinct from a
principal's API-key or basic-auth credentials.

An `auth_server_client` identity connects the OAuth client to a directory
principal. Creating an OAuth client alone does not establish the entire
principal lifecycle.

Sources: [authorization-server specification][auth-spec] and
[authorization-server concepts][auth-docs].

### Groups and token-vault resources

| Resource | Meaning |
| --- | --- |
| Group | Organizes principals or other groups for authorization. |
| Team | A group type representing an organization-chart node. Group type is immutable. |
| Membership | A relationship connecting a group to a principal or another group. |
| Vault provider | Configures an upstream credential source, such as OAuth or a static secret. |
| Directory trusted IdP | Configures an external identity issuer trusted for directory/vault flows. |
| Provider credential | An admin-managed shared credential or a user authorization obtained through enrollment. |
| Vault key | A public encryption key protecting vault data; it is not an API authentication key. |
| Principal vault credential | Enrollment-derived authorization associated with a principal. |
| Provider template | Read-only catalog of provider behavior, endpoints, and default scopes. |

Trusted IdPs and vault providers serve different purposes: one establishes
trust in identity assertions; the other configures the upstream credential
source.

Source: [platform-api PR #3877][platform-pr] and [directory specification][directory-spec].

## 4. Current KongCTL and SDK status

Current main has no integrated Kong Identity directory/principal or
auth-server management. Existing Portal identity-provider support and AI
Gateway identity resources are separate surfaces.

The paused work contains useful implementation:

| Area | Work present on the paused directory branch |
| --- | --- |
| Directories | Declarative load/validation/lifecycle, dump, ownership, references, get/list/adopt. |
| Principals | Directory-scoped get/list; principal selected by ID. |
| Principal identities | Get/list and display of the four identity variants. |
| Principal/identity declarative mutation | Not implemented. |
| Credentials | No management implementation. |
| Authorization servers | No implementation. |
| Supporting material | Directory examples, tests, E2E scenario, and cleanup support. |

See [PR #1433][directory-pr] and its [identity command package][old-cli].

The SDK prerequisite remains relevant. Current main's effective SDK
dependency is `v0.79.1-0.20261030170919-d76cbad8eb3b`. Inspection of that
cached dependency and the local SDK checkout found no directory or
auth-server management operations.

The local SDK checkout was at `a3bc4bed`, an October 8 automated OAS update.
Existing identity-provider APIs, DCR configuration, and Event Gateway
principal references do not constitute Kong Identity management coverage.

The issues' references to SDK `v0.36.0`/`v0.40.0` are stale, although their
underlying coverage concern persists. Integrating current typed operations,
or deliberately selecting a narrow temporary client, remains necessary.

## 5. Corrections to the earlier lookup report

| Earlier concern | Current assessment |
| --- | --- |
| No server-side ownership filtering | Accepted performance trade-off: scan complete collections and match locally. |
| Principal identities lack `managed_by` | Current schemas include it on create, replace, and responses. |
| Identity creation requires a supplied ID | The inspected create schema makes ID optional. |
| API keys have no name | PR #3877 exposes `name`, but it is nullable and not documented as unique. |
| Ownership updates have no clear-key mechanism | `ManagedByUpdate` documents `null` to remove a key; omitted-key behavior still needs confirmation. |

Principal identities therefore look substantially more feasible than the
old report suggests. Their PATCH schema still lacks `managed_by`, so
adoption and ownership updates need deliberate handling through supported
replacement semantics.

Sources: [directory schemas][directory-spec] and [common metadata schemas][metadata-spec].

### Reusing the AI Gateway ownership work

Current AI Gateway helpers retain namespace/protection plus tool/version,
but **do not persist local `ref`**. They would discard a newly supplied ref
key. Existing provider reconciliation also matches by API name before
checking ownership.

We can reuse pagination, ownership checks, sync scoping, and secret-handling
patterns, but must explicitly extend the identity contract for resources
without a natural moniker.

The desired logical lookup is:

```text
parent scope + resource type + tool + namespace + ref
```

Parent scope and resource type are provided by the route and reconciliation
context. The remote metadata must retain the remaining identity information.
Every matching collection must be fully paginated. Duplicate matches must
produce an error before mutation rather than selecting an arbitrary match.

The common API `managed_by` schema permits at most five entries. Tool,
version, namespace, protection, and ref would use all five. Its values also
have a 63-character limit and a restricted character pattern. KongCTL refs
must fit those constraints or use a documented encoding.

Sources: [current metadata helper][kongctl-managed-by],
[AI Gateway provider ownership][provider-ownership], and
[common metadata schema][metadata-spec].

## 6. Feasibility of the original resource families

| Resource | Recommended support and limitations |
| --- | --- |
| Directories | Full declarative support. Unique name provides matching; establish explicit `default` behavior. |
| Principals | Full declarative support using persisted ownership/ref metadata and client-side matching. Display name is not a safe unique key. |
| Principal identities | Declarative support using current `managed_by`; verify replacement and ownership-update behavior. |
| Basic-auth username records | Declarative support; public docs specify username uniqueness within a directory. |
| Passwords | Separate resource lifecycle from secret material and rotation; reads cannot recover the secret. |
| Generated API keys | Ownership-based reconciliation is feasible; securely capture the creation-time secret. |
| Imported API keys | Create schema lacks `managed_by`; create-then-PATCH leaves a failure/retry window. |
| Auth servers and clients | Declarative support using names and labels; use client PATCH for ordinary updates. |
| Auth-server scopes and claims | Declarative support is plausible, but define parent ownership because neither has labels nor `managed_by`. |
| Client secrets | Begin with explicit create/enable/disable/delete/rotation operations. |

Sources: [directory specification][directory-spec],
[auth-server specification][auth-spec], and
[credential behavior][principal-docs].

### Matching and ownership on the auth-server side

The auth-server specification has no `managed_by`. Auth servers and clients
have labels; scopes and claims have neither labels nor `managed_by`.

The public auth-server tutorial describes auth-server name uniqueness within
organization and region. Client and scope names are explicitly unique within
an auth server. Claim names have filtering but no explicit uniqueness
guarantee in the inspected specification; exact name matching with duplicate
rejection is workable.

The lack of per-child metadata does not automatically block scopes/claims.
It means KongCTL must define whether a managed auth server owns its entire
scope/claim collection, or apply another explicit parent-ownership policy.
It cannot treat those children exactly like independently tagged resources.

Sources: [auth-server source][auth-spec] and
[auth-server setup tutorial][auth-tutorial].

### Secret and update boundaries

- Creating an OAuth client already generates an initial secret. Secure
  output handling is needed during client support, before the separate
  secret-management issue is complete.
- Client PUT requires secret material. PATCH avoids that requirement for
  ordinary reconciliation.
- API-key secrets cannot be changed in place and cannot be retrieved after
  creation. Imported secret drift cannot be checked through read APIs.
- Password secrets are returned at creation. PATCH changes metadata/expiry;
  replacing secret material uses PUT, whose inspected schema requires both
  `secret` and `expires_at`.
- Public docs describe up to two passwords per username and two active
  client secrets, supporting rotation.
- Client-secret PATCH changes enabled status. New secret creation and
  rotation need deliberate secure output.
- Auth-server deletion normally refuses when children remain; force deletion
  cascades. KongCTL should order owned-child deletion and avoid indiscriminate
  force deletion.

Secret values cannot be faithfully reconstructed by `dump`, and write-only
secret drift cannot be detected through read APIs. Dump round-trip criteria
must distinguish configuration and credential metadata from secret recovery.

Sources: [auth-server source][auth-spec], [directory source][directory-spec],
[principal documentation][principal-docs], and [secret rotation guide][rotation-docs].

## 7. Additional scope in platform-api PR #3877

[The PR][platform-pr] principally makes 45 existing gated operations public,
rather than introducing 45 new operations. It also exposes directory
vault/TTL fields, principal type/active/groups, API-key name, organization
chart fields, and control-plane identity `control_plane_type`.

| Additional family | KongCTL assessment |
| --- | --- |
| Groups and teams | Good declarative candidates; unique group names and `managed_by`. Group type is immutable. |
| Memberships | Reconcile by member type/ID; confirm duplicate and cycle rules. No `managed_by`. |
| Group API keys | Same secret and imported-key concerns as principal keys. |
| Directory trusted IdP | Feasible using unique issuer URL; currently one per directory. No ownership metadata. |
| Vault providers | Need safe matching and ownership policy. Name exists and is filterable, but uniqueness is unspecified. |
| Provider credentials | Admin writes manage shared credentials; user enrollment remains operational. Credential multiplicity/matching needs clarification. |
| Vault keys | Potentially match by deterministic `kid` thumbprint; define ownership and replacement. |
| Principal vault credentials | Inspect and revoke; no ordinary admin create operation. |
| Provider templates | Read-only discovery. |
| Org chart import | Explicit import operation initially. |

### Vault-specific limitations

Vault providers, provider credentials, and vault keys lack `managed_by` and
labels. Client-side filtering cannot compensate for absent ownership fields.
Names or deterministic identifiers can assist matching, but safe ownership
and deletion still require a policy.

Provider credential POST/PUT manage shared credentials. User credentials
originate through principal OAuth enrollment. Lists and detail reads can
include both types; configuration cannot replace user consent/enrollment.

Provider client secrets and static shared secrets are not returned by read
APIs. KongCTL must distinguish omitted/preserve behavior from rotation and
must not claim to detect secret drift.

Provider responses expose effective endpoints, while writes accept
overrides. Diffing needs a policy for effective values versus explicitly
configured values. Provider creation also has registration fields absent
from update schemas, requiring immutable/replacement decisions.

Vault keys have deterministic thumbprint IDs and a five-key directory limit.
PATCH renames a key; changing public key material requires replacement.
Deleting a key affects selection for newly encrypted credentials and does
not rewrite already encrypted credentials.

Trusted IdP PUT resets omitted optional fields, while PATCH preserves omitted
fields. JIT provisioning defaults true; vault access defaults false.

Admin-owned reconciliation must avoid pruning JIT-provisioned,
import-managed, or user-enrolled resources merely because they are visible
in a parent collection.

### Import and still-gated operations

The org-chart importer does not establish completion of #1419. That issue
asks for general principal CSV upsert, identities, credentials, and metadata
merge/sync modes. The organization importer has a narrower purpose and its
own synchronization behavior.

Realm configuration, computed listings, org-chart root lookup,
directory-wide API-key listing, and quotas remain gated. PR #3877 does not
change the separate authorization-server specification.

The auth-server source also includes JWK operations, claim testing,
refresh-secret actions, token inspection/revocation, trusted IdPs, and
Connected Apps. These extend beyond the original five tracked resource
families. JWK and auth-server trusted-IdP routes remain gated in the local
source and should not silently expand the initial deliverable.

Sources: [PR scope and exclusions][platform-pr] and [auth-server source][auth-spec].

## 8. Recommendation for PR #1433

Replace its implementation base while preserving useful work.

At inspection, the directory branch was 685 commits behind main and six
ahead. The SDK branch was 685 behind and two ahead. GitHub reported PR #1433
as conflicting. The directory branch delta touched 87 files, 58 of which
also changed on main. This overlap is maintenance evidence, not an exact
textual conflict count.

The old SDK branch redirected the dependency to an internal prerelease,
`v0.2.1-0.20260701194831-123d2277eee1`, with compatibility edits for older API
shapes. Replaying that dependency change would risk undoing months of SDK
updates.

Its integration plumbing predates current resource registration, root
inventories, child ownership, references, payload contracts, secret handling,
and reconciliation hardening. The old directory defaulting derives required
`name` from `ref`, which current repository guidance prohibits. Its matching,
list-error handling, replacement hydration, and pagination also need review.

Start a fresh branch from main and selectively port the resource behavior,
read commands, examples, and test intent. Retain #1433 as a reference until
replacement work lands. A wholesale rebase would spend effort preserving
obsolete integration decisions.

Useful salvage sources:

- [Directory resource][old-resource].
- [Directory planner][old-planner].
- [Directory executor adapter][old-adapter].
- [Directory/principal/identity CLI package][old-cli].
- [Directory E2E scenario][old-e2e].
- [Examples and README][old-examples].

New work should follow the
[current resource implementation guide](contributor/declarative-resource-implementation-guide.md).

## 9. Recommended work sequence

1. **Confirm the API/SDK contract.** Identify deployed versions, include both
   management specifications in current SDK generation, and verify
   forthcoming public coverage.
2. **Implement directories on current main.** Reuse paused work selectively,
   including get/list/adopt and the directory lifecycle.
3. **Establish principal reconciliation.** Persist ref identity, reject
   duplicates, scan all pages, and scope deletions to resources KongCTL owns.
4. **Add identities and basic-auth records.** Validate ownership updates,
   replacement behavior, and cross-resource references.
5. **Add secure credential output and lifecycle support.** Cover initial
   OAuth client secrets as well as generated keys/passwords.
6. **Develop auth servers, scopes, clients, and claims as a separate track.**
   Resolve scope references and parent ownership explicitly.
7. **Add groups/memberships, then vault configuration.** Keep enrollment,
   import, rotation, and revocation as explicit operations where appropriate.
8. **Design general CSV import after these contracts stabilize.**

For each supported resource, lifecycle support includes the relevant plan,
diff, apply, delete, sync, dump, scaffold, explain, inspection, adoption,
documentation, and test work. Sync must preserve the distinction between
omitted and explicitly empty child collections.

## 10. Prioritized engineering requests

1. Confirm deployed support for identity `managed_by` and the GA schemas,
   including current SDK generation/availability.
2. Add atomic `managed_by` support to imported API-key creation, for both
   principal and group keys. Create-then-PATCH leaves an orphan/retry window.
3. Document ownership-map merge/replace/null behavior and support identity
   ownership updates consistently.
4. Provide ownership metadata, or documented stable keys and lifecycle rules,
   for vault resources. Clarify safe coexistence with other managers.
5. Clarify shared-provider credential multiplicity, matching with selectors
   and priorities, secret preservation, immutable fields, and replacement.
6. Confirm group membership uniqueness/cycle rules and the boundaries of
   import-managed state.

Server-side ownership filtering remains a useful optimization request. It is
no longer the blocker for initial KongCTL support under the accepted
client-side filtering trade-off.

## 11. Short discussion starting point

- We can restart useful work now, beginning with directories and principals.
- Principal identities now have ownership metadata, improving their outlook.
- Current SDK management coverage still needs integration.
- Reuse the paused PR's useful pieces on current main instead of rebasing it
  wholesale.
- Keep secret handling and ownership policies explicit; the new vault
  resources have more unresolved questions than the original directory work.

## 12. Verified SDK compatibility diagnosis

After building the internal SDK branch, the initial summary of KongCTL
compile failures mixed pre-existing public/internal differences with new
Identity-related generator name collisions. The following controlled builds
separate them. No KongCTL source files were changed for these checks.

### Builds compared

Current KongCTL is `2beef7be`. The historical internal-SDK development
snapshot is local ref
`backup/aigw-2-3-triage-before-rebase-20261029`, commit `c0473ece`.

The old patched internal SDK is `8108bd7e` from October 29. The new Identity
SDK is `d596cf53`, branch `kongctl/kong-identity-patched`.

| KongCTL source | SDK | Build result |
| --- | --- | --- |
| Current main, `2beef7be` | Current patched public, `d76cbad8` | Pass |
| Current main, `2beef7be` | Old internal, `8108bd7e` | Fail in the same broad integration areas |
| Historical internal branch, `c0473ece` | Old internal, `8108bd7e` | Pass |
| Historical internal branch, `c0473ece` | New internal, `d596cf53` | Fail on generated identifier changes |
| Historical internal branch with diagnostic Go overlays | New internal, `d596cf53` | Pass |

The overlays change six Go expressions across five files. They are confined
to the local diagnostic cache and do not modify either KongCTL checkout.

### Existing compatibility code explains most initial failures

The historical branch already includes the API implementation,
API-version request, control-plane filter, Portal OIDC input, and service
output adaptations in commit `4d1ca02c`. It also uses the old internal force
enum names. Current main uses a patched public SDK and does not contain that
internal compatibility delta.

The merged AI Gateway 2.3 commit `3b822db0` on October 30 pins the public SDK
at `d76cbad8`. It is therefore incorrect to interpret the current-main build
failures as proof that all those APIs changed during the new Identity build.

Comparing old and new internal OpenAPI inputs confirms that the relevant
API implementation, service reference, control-plane filter, Portal OIDC,
Service, Google Tag Manager, and Event Gateway SCRAM schemas are unchanged.
The API and API-version operation contracts are unchanged too.

For example, the API-version request-object signature was already present
in the old internal SDK: its additional internal `spec_format` parameter
affects method generation. That is a public/internal surface difference,
not a new Identity API behavior.

### New identifier collisions

The newly included APIs share the SDK's Go namespaces with existing APIs.
Their generic names cause these existing identifiers to be qualified:

- API update `Force` becomes `UpdateAPIQueryParamForce`; the new auth-server
  delete request uses the generic `Force` name.
- API-version update `QueryParamForce` becomes
  `UpdateAPIVersionQueryParamForce`; directory deletion now uses
  `QueryParamForce`.
- Google Tag Manager integration `Type` becomes
  `GoogleTagManagerIntegrationType`; principal filtering introduces a
  different `Type` object. Three expressions on the historical branch need
  the qualified integration enum/constant.
- Event Gateway SCRAM `Algorithm` becomes
  `VirtualClusterAuthenticationSaslScramAlgorithm`; auth-server signing
  introduces an `Algorithm` enum.
- Upstream health-check `Active` becomes `UpstreamActive` because principal
  filtering introduces `Active`. The SDK branch already handles this in its
  deep-copy generation recipe.

The diagnostic build uses the first four mappings. No wire API behavior is
changed by those identifier substitutions.

### Generator and input verification

Both internal SDK revisions use Speakeasy `1.805.0`; their
`.speakeasy/gen.yaml` settings are identical. The current public patched SDK
also uses `1.805.0`. Its pre-existing settings differ from the internal SDK
for shared error components and persistent edits, as well as the SDK version.
The new build preserved the internal settings rather than replacing them
with public generation settings.

Public generation uses the public bundle; internal generation uses the
complete bundle. This intentionally exposes different optional/internal
fields and can affect generated model shapes and method signatures.

The source metadata also identifies genuine input updates beyond adding the
two management specs:

- AI Gateway source version `2.2.31` to `2.3.0`.
- Portal management source version `3.14.1` to `3.15.1`.
- New directory source `1.27.2` and auth-server source `1.9.1`.
- Updated workspace processing selects the supported non-workspace
  Consumer-to-Consumer-Group membership routes instead of their unsupported
  workspace counterparts.

Those updates should be assessed on their own merits. They do not explain
the unchanged contracts behind the initial six-area compatibility list.

The practical next step is to reuse the known historical internal-SDK
compatibility work on current main, account for the new generated names,
and validate the resulting branch. Stabilizing names for the new API types
is another option to evaluate before adding broad compatibility work.

Diagnostic artifacts are under
`/home/rspurgeon/.cache/kong-identity-sdk/`: the old/new build logs,
`spec-comparison.json`, and `diagnosis-overlay-*.json` preserve the evidence.

## 13. Exact gated additions on established resources

The following source values explain concrete public/internal differences.
These are field/parameter or operation gates; they do not mark the entire
established resource as internal. Sources were inspected at platform-api
commit `9adee27ed`.

### Addition and gating timestamps

These are the introducing commits' UTC committer timestamps, verified
against their diffs, rather than the most recent formatting/blame dates.
The age column counts calendar days as of October 30, 2026. These dates
describe specification history, not deployment or SDK publication dates.
The listed additions were gated when introduced.

| Resource / addition | Added to specification (UTC) | Age | Source |
| --- | --- | --- | --- |
| API versions: `spec_format` | 2026-09-18 16:09:29 | 42 days | [#3500][timeline-spec-format] |
| Portal OIDC: original back-channel logout toggle | 2026-10-19 16:41:11 | 11 days | [#3756][timeline-logout-toggle] |
| Portal OIDC: read-only support/session-support fields | 2026-10-22 13:00:38 | 8 days | [#3798][timeline-logout-support] |
| Portal OIDC: current nested `backchannel_logout` object | 2026-10-22 14:11:07 | 8 days | [#3806][timeline-logout-object] |
| API implementations: current `environment_id` field | 2026-08-24 21:59:56 | 67 days | [#3308][timeline-environment-id] |
| Gateway Service: Konnect-only `description` | 2025-08-06 00:35:22 | 450 days | [#1577][timeline-service-description] |
| Gateway Service: `managed_by` | 2025-08-12 11:05:28 | 444 days | [#1621][timeline-service-managed-by] |
| Gateway config: validation endpoint reusing Service as input | 2026-10-23 06:29:14 | 7 days | [#3805][timeline-config-validation] |
| Portal creation: `create_default_content` | 2026-01-29 21:34:48 | 274 days | [#2201][timeline-portal-content] |
| Portal creation: `theme_name` | 2026-10-29 15:43:58 | 1 day | [#3850][timeline-portal-theme] |

The API implementation environment feature predates its current ID form:
the earlier gated environment field was introduced on July 27, 2026,
19:30:29 UTC (95 days), in [#3048][timeline-environment-original]. The
August 24 change replaced the previous environment-name representation
with the current ID representation while retaining the gates.

Service description's introducing commit has an August 5 author date in
US Pacific time, but its UTC timestamp is August 6 as shown above.

The age range is therefore substantial: some internal/public differences
come from features over a year old, while others are days old. The separate
Go identifier collisions discussed in section 12 arose when the Identity
specifications were added to this SDK build on October 30; that is not the
age of the existing API field whose Go type was renamed.

### API versions

Source: `src/konnect/definitions/api-builder/v3/openapi.yaml:2189`.

`components.parameters.SpecFormat` declares query parameter `spec_format`,
referencing `SpecFormatSchema`, with both `x-internal: true` and
`x-unstable: true`. Public SDK generation removes the parameter. Internal
generation retains it, adding a parameter to create/fetch operations. With
`go.maxMethodParams: 2`, the internal methods use request objects instead
of the public positional signatures. This was already true in the old
internal SDK.

### Portal OIDC identity-provider configuration

Source: `src/common/definitions/identity_provider.yaml:364`.

`PortalOIDCIdentityProviderConfig` declares:

```yaml
x-property-annotations:
  backchannel_logout: [x-unstable, x-internal]
```

The internal-only nested object has `enabled`, `supported`, and
`session_supported`. The latter two are `readOnly: true`; `enabled`
defaults to false. The existing `client_secret` is `writeOnly: true`.
The generated internal configuration separates input and output shapes,
including `PortalOIDCIdentityProviderConfigInput`; the public configuration
does not contain back-channel logout at all.

### API implementation environment scope

Source: `src/konnect/definitions/api-builder/v3/openapi.yaml:4513`.

Both `ApiImplementationGatewayServiceEntity` and
`ApiImplementationControlPlaneEntity` declare:

```yaml
x-property-annotations:
  environment_id: [x-internal, x-unstable]
```

Implementation list-item schemas also gate `environment` with the same
annotations. These add environment scope to the existing implementation
resources in the internal SDK.

The Go `ServiceReference` rename has another factor: the internal service
catalog vulnerability APIs retain a separate schema literally named
`ServiceReference`, declared at
`src/konnect/definitions/service-catalog/v1/openapi.yaml:9829`. The API
implementation schema has `x-speakeasy-name-override: Service Reference`.
That shared Go name requires disambiguation. The environment-field gate
alone should not be described as the sole cause of the constructor rename.

### Gateway Services

Source:
`src/konnect/definitions/control-planes-config/v2/generated.yaml:12806`.

The Service schema gates both `description` and `managed_by` with
`[x-unstable, x-internal]`. Description is explicitly Konnect-only metadata
that is not synced to Gateway. Public Service models lack both properties.

The return-type split also has an operation-level cause:
`POST /control-planes/{controlPlaneId}/config/validate` has both flags at
`src/konnect/definitions/control-planes-config/v2/konnect.yaml:1788`.
Its `ControlPlaneConfig.services` request property reuses the Service
schema. Public usage of that schema is response-only; internal usage adds
an input context. Because `Service.url` is `writeOnly: true`, the internal
SDK generates distinct Service and ServiceOutput shapes. Merely describing
this as two additional metadata fields would miss the input-context change.

### Portal creation

Source: `src/konnect/definitions/portal-management/v3/openapi.yaml:5983`.

The create request gates both `create_default_content` and `theme_name`
with `[x-unstable, x-internal]`. The former is an optional boolean
defaulting to false; the latter selects the theme for generated content.
The current KongCTL patch bundle already preserves omission of this
internal field rather than injecting false on every create request.

### Differences with no corresponding gated field

The control-plane `name` filter schema is structurally identical in the
public and internal inputs. Its Go type-name difference is name resolution,
not a newly gated control-plane filter property.

Likewise, the Identity additions change the generated force-enum, Google
Tag Manager type, Event Gateway algorithm, and upstream Active names through
shared namespace collisions. The existing API parameters and values remain
unchanged. Lifting gates on established fields would not by itself resolve
all of these generated-name differences.

## 14. Selective public SDK preview

The public SDK approach works for the proposed Identity scope. It provides
the new SDK surface while retaining the existing public SDK API consumed by
current KongCTL. No KongCTL compatibility source changes are required.

### Branches and dependency version

Both development branches are pushed, and both worktrees are clean:

- Platform API: `build/ki-public-sdk`, commit `57350ae722c6`.
  This includes both SDK product selections and the Identity v2 source from
  PR #3877 at `98137d7be3772b67693cc1ceb0469e77f8ecf0f5`.
- Public SDK: `kongctl/kong-identity-public-preview`, commit
  `3bc3e39b8ae42ff5924861bc1f628bb4794fdad7`.
- Public SDK base: `d76cbad8eb3b07577ac30fda2454be0b33ee74ba`, which is the
  version current KongCTL already uses.
- KongCTL patch bundle: `2beef7beeab6b3abd1e0d39978a1c48c203d83dd`.
- Speakeasy: `1.805.0`, with the public generation configuration unchanged.

The confirmed Go module is `github.com/Kong/sdk-konnect-go`, at:

```text
v0.79.1-0.20261031012019-3bc3e39b8ae4
```

This is a commit-derived development version. No SDK/spec release, tag, or
new PR was created for the experiment. The earlier product-selection PR
#3895 remains separate.

### Scope and preparation

The final SDK contains 1,102 operations: the 990 existing public operations
and 112 added Identity/auth-server operations. The additions cover the
original auth-server and directory work, plus groups and memberships, group
keys, Token Vault providers/templates/credentials/public keys, principal
vault credential inspection/revocation, directory trusted IdPs, and org-chart
import. Principal subtype, active-state and related planned fields are also
available.

The normal public pipeline uses the selectively ungated Identity source
from PR #3877. Other specifications keep their visibility annotations.
Computed endpoints, realm configuration, org-root reads, directory-wide key
listing, quotas, and internal Sliver features remain excluded.

The SDK branch's `identity-preview/prepare.cjs` performs the small additional
preview preparation. It excludes five unrelated Connected Apps operations
selected with Sliver, and gives new schemas and list filters explicit Go
names. This prevents collisions with existing names such as `Scope`,
`Client`, `GrantType`, `TokenEndpointAuthMethod`, `CredentialType`, and
`QueryParamFilter`. Both vault credential variants need distinct enum names.
Existing KongCTL code keeps its current public SDK names.

The SDK's `go.mod`, `go.sum`, and `.speakeasy/gen.yaml` are unchanged from the
public base. The initial KongCTL compatibility test used a copied module
file with a local SDK replacement and left the worktree unchanged.

Following the maintainer's instruction, the `ki-triage` worktree has now
adopted the preview commit in its actual `go.mod` replacement. `go.sum` and
the local vendor directory have been refreshed. The replacement uses the
pushed Go module version above rather than a local filesystem path. No
KongCTL source changes were needed.

After adoption, the normal repository checks also passed: `go fix ./...`,
`make format`, `make build`, `make lint` (zero issues), `make test`, and
`make test-integration`. Unit and integration tests ran with race detection.
The built local executable reports the preview SDK version in its Go build
metadata. Adoption logs are under the existing log directory's `adoption/`
subdirectory.

### Validation

The following checks passed:

- Full platform-api build and public SDK generation.
- SDK generation's compilation, staticcheck, and OpenAPI linting. Linting
  reports zero errors, with warnings remaining.
- Reference-resolved comparison of all 990 existing public operations:
  request/response shapes are unchanged, ignoring description whitespace.
  Global servers, security requirements and security schemes also match.
- AST comparison of 37,119 existing exported declarations: no incompatible
  type member/tag, method signature, or constant changes were found. The
  audit allows additional declarations/members and ignores comments/order.
- SDK serialization/unit tests and generated field guards.
- Compilation of all SDK packages, including interfaces and mocks.
- Unchanged KongCTL build and all unit tests with `CGO_ENABLED=0`.
- The resulting KongCTL executable runs its `version` command successfully.
- Go resolves the pushed SDK commit to the development version above.

The SDK branch saves operation coverage, API audit results and input
provenance under `identity-preview/`. Its root `KONG_IDENTITY_BUILD.md`
records reproduction steps. Local logs, the audit helper, the temporary
module files and the test binary are under:

```text
/home/rspurgeon/.cache/kong-identity-public-sdk/
```

### Remaining limits and next step

This validates SDK and KongCTL source compatibility. It does not validate
deployment availability or live behavior of the new endpoints, and it does
not implement Identity commands or declarative resources in KongCTL.
Ownership, secret handling, update semantics and importer-managed membership
constraints still need the resource-by-resource treatment described earlier.
Client-side `managed_by` matching remains the accepted trade-off.

Two local generation details are documented in the SDK guide: use a fresh
public SDK base when regenerating, because a second linked-worktree run
failed to load Speakeasy's persistent-edit snapshot; and install pinned
ifacemaker directly when the isolated mise tool directory cannot locate its
configured Go dependency. Neither workaround changes the public SDK model
or generator settings.

The standard workflow uploaded code samples during earlier attempts. The
final preview preparation disables that publishing step; no SDK or spec
release was published.

The `ki-triage` worktree is now the Identity implementation branch and
consumes the public preview version above. The next work can begin directory
support from the current main baseline. This removes the need to adapt
unrelated existing resources to the full internal SDK before starting
Identity implementation.

### Issue tracking

SDK tasks #1399 and #1413 and their parent epics #1133 and #1132 are now
marked **In Progress** in the KongCTL GitHub project. Both SDK task bodies
record the preview commit, patch bundle, operation inventory, branch adoption,
and passing checks. SDK generation and branch integration are covered;
operation-specific serialization and live API behavior validation remain
open, and the KongCTL changes have not merged.

For subsequent Identity work, mark an issue In Progress when work starts and
mark it Done only after its changes merge to main.

## 15. First declarative implementation: directories

The first implementation on `ki-triage` is `identity.directories`, tracked by
[issue #1414](https://github.com/Kong/kongctl/issues/1414), which is In Progress.
It uses the public SDK preview recorded above. This section supersedes the
earlier statement that this branch contains SDK integration only.

### Selection across the three API surfaces

Directory administration provides a practical first configuration resource:
directories have documented unique names, support mutable configuration and
`managed_by`, and do not contain write-only credentials. Auth-server
administration remains a later configuration candidate with its own ownership
and secret contracts. Runtime authentication and lookup are operational calls;
they do not have the same declarative resource lifecycle.

The common user path may be referencing the existing directory named
`default`. The examples prominently show an external name selector and the
explicit ID alternative. No implicit directory creation or special default
marker is assumed. The live test organization initially had no directories.

### Implemented locally

- Grouped `identity.directories` configuration with explicit `ref` and `name`.
- All directory settings: description, control-plane allowlist and unrestricted
  access, positive/negative cache TTLs, vault enablement, and public labels.
- Ownership/protection in the same four generated `managed_by` keys used by
  AI Gateway children: `tool`, `version`, `kongctl-namespace`, and
  `kongctl-protected`. Public labels remain separate. A shared policy explicitly
  enrolls this managed root without giving it child scope semantics.
- Full cursor pagination before local matching, duplicate detection, explicit
  adoption, namespace isolation, protected updates/deletion, and scoped sync.
- Fresh GET plus full PUT for updates. Omitted settings are preserved; explicit
  empty maps/lists/descriptions clear settings. PATCH lacks description and
  `managed_by` support.
- External directory selectors/IDs, control-plane UUIDs, same-plan and external
  control-plane references, and standard control-plane lookup tags.
- Plan, apply, sync, delete, diff, dump, explain, scaffold, get/list, and adopt.
- No forced cascading deletion through unmanaged directory descendants.

Examples are in `docs/examples/declarative/identity/`. The live scenario is
`test/e2e/scenarios/identity/directories/`, including all mutable fields and
create/update/no-op plans, a saved plan with a new control-plane reference,
dump/reload, omitted-field preservation, empty collections, external selection,
scoped sync, protection/unprotection, adoption/takeover, and explicit deletion.
It deletes its temporary resources and does not perform a whole-org reset.

### Verified platform constraints

The running [API team question log](kong-identity-api-questions.md) records
the open questions and their specification links for business-hours follow-up.

The isolated `.com` US API tests confirmed create/list/GET/PUT persistence of
all four ownership keys. Description, TTLs, labels, and control-plane settings
were retained across full replacements. Explicit collection clears and partial
updates also passed the live scenario.

The test organization permits one directory and requires
`allow_all_control_planes: true`. KongCTL does not invent that required value;
the live manifest and main example specify it. Restricted-access false is
covered by local payload/planning tests and shown in a separate example for
organizations that permit it.

Token Vault enablement remains a deployment/API limitation. On 2026-10-30
local time, both KongCTL and a direct HTTP PUT requested `vault_enabled: true`.
The API accepted the request but returned false in the response and a fresh
GET. KongCTL now reports this mismatch as an error. The live scenario verifies
false, while local tests verify a true request and the ignored-value error.
The API team must clarify availability before claiming live enablement.

Directory children and the newer principal/group, credential-provider,
Trusted IdP, org-chart, and Token Vault child resources remain separate work.
There is no additional assumption here about their uniqueness or ownership.

### Validation artifacts

The complete live lifecycle scenario passed and cleaned up both its directory
and control plane. Artifacts, command outputs, and local validation logs are
under `/home/rspurgeon/.cache/kong-identity-public-sdk/`. Existing enabled
replay scenarios and their recordings were preserved.

Final checks passed: Go modernization, formatting with no further changes,
CGO-disabled build, lint with zero issues, race-enabled unit tests, integration
tests, E2E inventory/replay-allocation checks, and the live directory scenario.
The final live run artifacts are in
`directory-e2e/20261030-213415/` under the cache directory above.

## 16. Failure cleanup and principal inspection follow-up

Directory E2E fixtures now use generated unique names and register teardown
before scenario commands run. Teardown executes after success, failure, and
early stops, independently of the canceled test context. It removes only the
named fixtures and principal children explicitly marked with the directory
UUID; an unmarked principal stops cleanup before deletion. Directory deletion
never requests a forced cascade. Existing shared reset and enabled replay
scenarios/recordings remain unchanged.

Live validation covered the normal lifecycle and early stops after resource
creation, protection, unmanaged creation, and principal creation. A real failed
dump round trip also exercised successful teardown. That failure exposed a
long-name bug: dump used the directory name as a local ref, which exceeds the
63-character ref limit for valid long API names. Dump now uses the directory
UUID, consistent with other resource exports, with round-trip regression
coverage.

Principal get/list commands now support explicit directory name/ID selection
and principal UUID lookup. Duplicate display names are not treated as unique
identifiers. A normalized read model preserves the presence of `active`; a
missing response field remains omitted in JSON/YAML and unknown in text.
Tests cover pagination, malformed/incomplete reads, both directory selectors,
user/agent/unclassified principals, typed metadata, and duplicate display names.

Direct HTTP probes retained the five-entry ownership map on create/PUT,
including `kongctl-ref`. They also reproduced API/spec mismatches for type
updates, missing active state, and PATCH ownership updates. The engineering
question log records exact observations and request examples in Q9–Q11.
The tested deployment supports initial type classification and ownership PUT;
these results do not establish support for type changes or active-state writes.

The approved principal declarative lifecycle is implemented in section 17.
Principal-identity commands remain separate follow-up work.

Validation passed: modernization, formatting, CGO-disabled build, standard lint
with zero issues, race-enabled unit tests, integration tests, the complete E2E
harness suite with race detection, replay/allocation inventory checks, the full
live scenario, and stopped-run teardown checks. An additional E2E-tag lint run
reported existing findings in the unchanged merge/loader/runner code; none were
reported in the new cleanup helper. Logs remain under
`/home/rspurgeon/.cache/kong-identity-public-sdk/`.


## 17. Principal declarative lifecycle

Implemented on `ki-triage`; #1415 remains In Progress until merge.
Principals support nested declarations under managed/external directories and
grouped declarations with an explicit directory selector. Their independent
ownership identity is directory ID + namespace + ref. KongCTL stores the ref
as the fifth generated `managed_by` entry, `kongctl-ref`; public display names
may repeat. All matching scans every page and rejects duplicate tuples. Saved
updates/deletes repeat complete duplicate and fresh ownership checks.

Implemented behavior includes plan/apply/sync/delete, saved plans, adoption,
dump with directory children, explain/scaffold, and get/list. PUT hydrates
omitted fields from a fresh response before overlaying saved changes. Metadata
and labels can be cleared explicitly; integer metadata retains its precision.
Protection requires a separate unprotect operation. Adoption requires a UUID,
explicit directory, namespace, and ref, and rejects an existing conflicting
tuple.

An explicit empty nested principal collection selects its source namespace
for pruning. Omission selects no pruning. External-only directory declarations
never authorize directory pruning. Directory ownership grants no principal
mutation authority. Unowned and foreign-namespace principals are retained.
Creation/deletion dependencies include parent changes across namespaces.

Temporary compatibility restrictions are intentionally tracked in
`docs/contributor/kong-identity-compatibility.md`, separately from API-only
engineering questions in `docs/kong-identity-api-questions.md`:

- Q9: type is create-only; explicit changes and classified-to-null fail.
  Omitting type preserves existing classification.
- Q10: active configuration is rejected, absent responses remain unknown,
  and PUT/dump never synthesize false.
- Q11: management metadata changes, protection, and adoption use fresh GET
  plus PUT until PATCH semantics are verified.

The tracker records removal criteria and the production/test locations to
revisit when the team answers. These are temporary deployment compatibility
choices, not claims that the API will permanently require them.

`docs/examples/declarative/identity/principals.yaml` groups user, agent, and
unclassified examples under an existing directory. The live directory scenario
also covers principal creation, no-op planning, field/metadata updates,
omission, type/null/active rejection, dump/reload, protection, adoption,
explicit deletion, and scoped sync. The expanded live run and a deliberate
stop after protecting a principal both passed. Teardown removed the generated
directory and its marked principals;
a fresh directory list was empty.

Artifacts under `/home/rspurgeon/.cache/kong-identity-public-sdk/`:

- `principal-decl-live-final/20261030-231029`
- `principal-decl-stopped-protected/20261030-231233`

Modernization, formatting, CGO-disabled build, lint with zero issues,
race-enabled unit tests, integration tests, harness race tests, and
replay/allocation inventory checks pass. Logs use the `principal-decl-` prefix.
The execution regression covers integer metadata through saved-plan cloning
and environment resolution as well as SDK mapping. Enabled replay scenarios
and recordings remain unchanged. #1415 remains In Progress until merge;
implementation is on the working branch and no PR has been opened.

## 18. Authorization-server root lifecycle

The branch now manages `identity.auth_servers`, with explicit name/audience,
description, signing algorithm, trusted origins, public labels, and the public
DCR token-duration setting. Get/list, adoption, explain/scaffold, plan/diff,
saved apply, dump/reload, protection, delete, and scoped sync are implemented.
Examples are in `docs/examples/declarative/identity/auth-servers.yaml`.

Direct US `.com` HTTP probes verified name uniqueness, create/GET/list/PATCH,
omission preservation, individual label removal, array clearing, signing
changes, DCR duration updates, and non-cascading deletion. Newly created
servers had no automatic scopes, claims, or clients. Auth servers expose
`labels` rather than `managed_by`; the new metadata ledger records that
distinction and the storage capabilities of each remaining family.

The SDK originally dropped explicit empty trusted origins from PATCH. A
targeted `omitzero` patch now preserves empty arrays while omitting nil ones;
its HTTP wire regression fails before the patch and passes afterward. The
preview SDK branch is pushed at `56d878d0739c`, and this branch uses its Go
pseudo-version `v0.79.1-0.20261031043735-56d878d0739c`. The patch tool also now
preserves multiline generated overlay entries without duplicating them.

The full auth-server live scenario passes, including create/update of every
exposed field, omission, explicit clearing, saved plans, no-op plans,
dump/reload, external references, omitted sync scope, protection, adoption,
explicit takeover, deletion, and empty-collection sync. A deliberate stop
after protection also passes teardown. A fresh server list is empty. The
directory/principal live scenario passes again against the updated SDK.

Artifacts under `/home/rspurgeon/.cache/kong-identity-public-sdk/`:

- `auth-root-e2e/20261030-235503`
- `auth-root-e2e-stopped/20261030-235539`
- `principal-sdk-update-regression/20261030-235647`

Modernization, clean formatting, CGO-disabled build, zero-issue lint,
race-enabled unit tests, integration tests, harness race tests, and
replay/allocation inventory checks pass. Logs use the `auth-root-` prefix.
Existing replay selections and recordings are unchanged. #1400 remains
In Progress until merge; OAuth clients, scopes, claims, and secrets remain
separate work. No additional API question arose from the root contract;
internal SDK serialization issues stay outside the API team question log.

Deletion always requests `force=false`, so children may block root deletion.
Only the public DCR duration is exposed; internal DCR settings remain outside
the manifest schema and are preserved by partial writes.

## 19. Scope and claim contracts and inspection

Direct US `.com` probes verified all public scope settings and claim settings,
PATCH omission, false booleans, empty descriptions and scope associations,
static JSON strings, and verbatim server templates such as `${ uuidv4 }`.
Scope creation rejects duplicate names with 409. Claim creation accepts
duplicate names and returns different IDs; Q12 asks how collisions affect
issued claims.

Claim-list totals do not reliably count claims. Two claims each linked to
three scopes returned two items but total six; page-size-one traversal
returned the claims on pages one and two, then empty pages with the same
inflated total. This is reproducible directly and recorded as Q13. The
collection reader follows actual pages through the terminal empty page,
validates pagination metadata and unique nonempty IDs, and discards the full
result after any page failure. Duplicate-name reads fail and require an ID.

New commands accept exactly one `--auth-server-id` or `--auth-server-name`:

```sh
kongctl get identity auth-server-scopes --auth-server-name existing-issuer
kongctl get identity auth-server-claims --auth-server-name existing-issuer
kongctl get identity auth-server-claim <claim-id> \
  --auth-server-id <server-id> -o yaml
```

JSON, YAML and text inspection are covered. The live child scenario exercises
page size one, ambiguous claim names, UUID lookup, false booleans, scope
associations, JSON-valued strings, and untouched server templates. Both full
execution and a stop after fixture creation pass cleanup. Teardown checks all
scope/claim fixture names against the generated parent's UUID, requires an
empty client collection, deletes claims before scopes, and keeps root delete
at `force=false`. It refuses all deletion when an unmarked child or client is
observed. The metadata ledger records that neither scopes nor claims stores
independent ownership metadata.

Live artifacts under `/home/rspurgeon/.cache/kong-identity-public-sdk/`:

- `auth-child-reads-e2e/20261031-000810`
- `auth-child-reads-stopped/20261031-000920`

Modernization, clean formatting, CGO-disabled build, zero-issue lint,
race-enabled unit/integration/harness tests and replay/allocation inventory
checks pass. Auth-server root lifecycle also passes after the cleanup change
in `auth-root-child-read-regression/20261031-001009`. A fresh server list is
empty. Logs use the `auth-child-` prefix. Enabled replay coverage and
recordings remain unchanged.

Declarative lifecycle remains unfinished. Next steps are an explicit
parent/collection ownership contract, same-plan scope references, dump,
omitted/empty collection scope, and deletion guards. Direct DELETE of a scope
silently removed it from both a client allowlist and a claim's scope list;
fresh complete observations must guard this side effect. Root deletion with
children and `force=false` returned 400 without deleting the server.

The generated claim PATCH currently omits an explicit empty
`include_in_scopes`. Fix and wire-test that serialization before enabling
declarative association clearing. This is an SDK issue, separate from Q13.
OAuth client IDs observed in the probe are opaque strings rather than UUIDs;
future client support must retain that distinction. Probe client secrets
were never logged, and every disposable resource was deleted.

## 20. Declarative scopes and claims

`identity.auth_servers[].scopes` and `.claims` now support declarative
lifecycle.
Grouped `identity.scopes` and `identity.claims` also accept an `auth_server`
reference to a managed server. All public mutable settings are exposed, with
explicit names and claim values, optional-field preservation, false booleans,
empty descriptions/association lists, typed scope references, and same-plan
creation dependencies. Static JSON strings and server templates remain intact.

Scopes and claims store no independent ownership metadata. Parent namespace
and update/delete protection apply. Explicit collections are authoritative
for sync; omission preserves them. Adoption is through the managed server.
External parent references cannot authorize these writes. Read-only inspection
still supports explicit parent selection. Duplicate claim names fail before
mutation or dump. The metadata ledger records these boundaries.

Scope deletion validates complete observed client and claim references.
Planned claim detachment/deletion runs first, and execution checks fresh
references again. A retained or newly introduced client/claim reference blocks
deletion. Root deletion remains `force=false`; declared children run first,
while undeclared children can block the server delete. Sync an explicit child
collection to remove children while retaining the server.

The preview SDK at `dfd890eb1a03` preserves explicit empty claim scope arrays.
Its wire regression failed before the targeted tag fix and passes afterward.
The shared patch bundle source is `b7e43b8d`, and existing KongCTL patches
remain installed. No SDK release or PR was created.

Q14 asks about creation idempotency/recovery. Claim and principal creation now
disable retries in both SDK and HTTP transport; synthetic 503 tests with both
retry layers enabled verify exactly one POST. An uncertain result requires a
fresh observation/plan rather than an automatic replay. No service fault was
injected. Typed SDK 404 handling was also fixed so repeated saved deletes can
recognize records already removed by an earlier attempt.

The complete live declarative scenario passes: saved create/apply, same-plan
scope IDs, every field update, omission/clearing, no-op plans, diff,
dump/reload,
parent adoption with children, protection, omitted sync collections, reference
guards, detach-before-prune, recreation and ordered deletion. A deliberate
stop while the parent was protected also passes cleanup. Unit coverage adds
parent/namespace isolation, cross-server scope rejection, stale ownership,
late client references, and retry safety. The directory/principal live scenario
passes again with the updated SDK and create policy.

Artifacts under `/home/rspurgeon/.cache/kong-identity-public-sdk/`:

- `auth-child-decl-live-final/20261031-004139`
- `auth-child-decl-cached-final/20261031-004953`
- `auth-child-decl-stopped/20261031-004341`
- `principal-claim-sdk-regression/20261031-004514`

All final gates pass in order: modernization, clean formatting, CGO-disabled
build, zero-issue lint, unit/integration/harness race tests, and scenario
inventory checks. `auth-child-final-status.json` records zero exit codes.
Planning caches complete unfiltered scope/claim/client observations per
parent; execution still reads fresh state. The complete live scenario passes
again after that cache integration. Enabled replay coverage and recordings
are unchanged.

The grouped example is `docs/examples/declarative/identity/auth-servers.yaml`.
The new scenario is `test/e2e/scenarios/identity/auth-server-declarative`.
Issues #1402/#1403 remain In Progress until merge, with no implementation PR.
OAuth clients and secret lifecycle remain the next auth-server work. Directory
identities, credentials, groups, and release-gated Vault/IdP/import families
remain tracked in their existing issues and API questions.

<!-- Sources: long link destinations are kept intact for valid Markdown. -->

[issue-1132]: https://github.com/Kong/kongctl/issues/1132
[issue-1133]: https://github.com/Kong/kongctl/issues/1133
[issue-1400]: https://github.com/Kong/kongctl/issues/1400
[platform-pr]: https://github.com/Kong/platform-api/pull/3877
[directory-pr]: https://github.com/Kong/kongctl/pull/1433
[principal-docs]: https://developer.konghq.com/identity/principals/
[auth-docs]: https://developer.konghq.com/identity/auth-servers/
[auth-tutorial]: https://developer.konghq.com/ai-gateway/set-up-kong-identity-for-a2a/
[rotation-docs]: https://developer.konghq.com/how-to/rotate-auth-server-client-secrets/
[auth-spec]: /home/rspurgeon/dev/kong/platform-api/src/konnect/definitions/sliver/v1/openapi.yaml
[directory-spec]: /home/rspurgeon/dev/kong/platform-api/src/konnect/definitions/kong-identity/v2/openapi.yaml
[runtime-spec]: /home/rspurgeon/dev/kong/platform-api/src/konnect/definitions/kong-identity-auth/v2/openapi.yaml
[metadata-spec]: /home/rspurgeon/dev/kong/platform-api/src/common/definitions/konnect_properties.yaml
[kongctl-managed-by]: ../internal/declarative/resources/managed_by.go
[provider-ownership]: ../internal/declarative/planner/ai_gateway_provider_ownership.go
[old-resource]: https://github.com/Kong/kongctl/blob/gh-1414-ki-directory/internal/declarative/resources/identity.go
[old-planner]: https://github.com/Kong/kongctl/blob/gh-1414-ki-directory/internal/declarative/planner/identity_directory_planner.go
[old-adapter]: https://github.com/Kong/kongctl/blob/gh-1414-ki-directory/internal/declarative/executor/identity_directory_adapter.go
[old-cli]: https://github.com/Kong/kongctl/tree/gh-1414-ki-directory/internal/cmd/root/products/konnect/identity
[old-e2e]: https://github.com/Kong/kongctl/blob/gh-1414-ki-directory/test/e2e/scenarios/identity/directories/scenario.yaml
[old-examples]: https://github.com/Kong/kongctl/blob/gh-1414-ki-directory/docs/examples/declarative/identity/README.md
[timeline-spec-format]: https://github.com/Kong/platform-api/commit/d8873ace2a76ce851c23dcc689e89b404529a2a0
[timeline-logout-toggle]: https://github.com/Kong/platform-api/commit/8c1997aa3d903dd7f2fb2080c2094f21fdf66094
[timeline-logout-support]: https://github.com/Kong/platform-api/commit/b98cd78cf0757bed00dcc5f42fa90a0d87d93ccf
[timeline-logout-object]: https://github.com/Kong/platform-api/commit/dc964b6ca10dda24e61bafa64ddd5ab425cde938
[timeline-environment-id]: https://github.com/Kong/platform-api/commit/1fa3c1ebd6cc36732428b8bdcb87340649cde361
[timeline-service-description]: https://github.com/Kong/platform-api/commit/a64532d282db631c263a7e7a5b8e537e055f453f
[timeline-service-managed-by]: https://github.com/Kong/platform-api/commit/6daa7304ecd17373e5ce3676bfbc90060a1ce686
[timeline-config-validation]: https://github.com/Kong/platform-api/commit/30a23d25c756681a14470b5f3091d2e663130aa0
[timeline-portal-content]: https://github.com/Kong/platform-api/commit/0556e7410419870fb9a2f87fc8c81e0bfae498e7
[timeline-portal-theme]: https://github.com/Kong/platform-api/commit/810709c402f65e403a9d1e593c5feeffcf4a672e
[timeline-environment-original]: https://github.com/Kong/platform-api/commit/332d90437a4d08ece256c0b895c5f7ab36805657
