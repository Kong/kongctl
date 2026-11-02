# Kong Identity support in KongCTL: triage report

Date: October 30, 2026

KongCTL branch: `ki-triage`
KongCTL commit inspected: `2beef7be`

## Current branch status

Latest update: November 1, 2026 (US Central).

The preceding checkpoint is `94556ccb`: generated principal/group API-key
declarations, private capture, saved authority checks and scoped retirement.
Thirty Identity live scenarios passed together at that checkpoint. The
auth-server and directory administrative families have examples and broad
declarative lifecycle support; explicit operations handle secret lifecycle,
shared OAuth enrollment and complete-file organization-chart import.

General principal CSV declarations, offline inspection, optional UUID
assertions and public metadata merge/sync policies are delivered under #1419
(section 65). The preceding increment delivered principal-group inspection under
#2658: ordinary direct and imported direct/inherited groups, nullable entry IDs
and manager/provisioning fields (section 67). All thirty-two Identity live
scenarios passed at that checkpoint. All thirty-three Identity live scenarios
and current repository gates now pass with the Registry guards (section 69).
Privacy/cleanup checks pass and default settings are restored. Replay
membership/cassettes are unchanged;
all twenty enabled replays passed at the preceding CSV checkpoint.

The public SDK preview is now 2f51ede0670e, retaining patch source 5680da0f.
It adds two Registry inventory GETs while preserving all pre-existing public
declarations and wire tags. All seventeen cursor-wire methods pass without
skips. The
[per-operation coverage matrix](contributor/kong-identity-api-sdk-coverage.md)
records 112 Identity operations plus two Registry dependency reads, 99 consumer
methods with positive deployed evidence, unused alternatives/follow-ons and
the remaining runtime boundary.

The main unfinished implementation is populated principal Vault authorization
revocation (#2663), which needs Q30's supported runtime enrollment fixture.
Generated-only password declarations also warrant reassessment against the
newer immutable API-key ownership convention (#1418, section 70). They are not
implemented; Q35's adoption limitation alone does not rule out that subset.
The acceptance audit also found the Registry Agent incoming-reference
requirement in #1414/#1415/#2419. A native principal deletion leaves its Registry
association present (Q53). Read-only SDK support and retirement guards are
implemented and verified by all thirty-three live scenarios (section 69).
Repository checks pass. This is a verified stopping point for manual review;
it does not complete the pending runtime revocation work.
Compatibility restrictions remain tied to their API questions; successful
administrative writes do not establish runtime authentication or revocation.
Q52 records one identity POST/list visibility discrepancy, with twenty native
follow-up trials unable to reproduce it. No POST replay workaround was added.

The [API questions](kong-identity-api-questions.md),
[compatibility tracker](contributor/kong-identity-compatibility.md) and
[metadata ledger](contributor/kong-identity-metadata-storage.md) are current
handoff documents. Issues stay open/In Progress until main merge. Implementation
is paused for review; the maintainer has requested a draft PR. The preview SDK
remains a branch dependency without a release. The implied default-directory
imperative flag remains a later-iteration TODO.

## Historical branch checkpoints

Latest verification: October 31, 2026 (US Central).

Provider and principal Vault credential metadata now have scoped get/list
commands, public projections and a live `.tech` scenario. Provider reads avoid
the detail GET that issues OAuth enrollment capabilities. Native probes find
multiple shared credentials, unique routing priorities/default slots and PUT
material/omission constraints (Q45/Q46). Section 56 records those contracts,
the read boundary and the next declarative work. SDK preview `90596b498bcb`
adds only three generated cursor-query tag fixes, without component-model
changes. All twenty-four Identity live scenarios, repository gates, artifact
verification and early-stop cleanup pass. Default settings are restored and
the inspected resource inventories are empty.

Directory Trusted IdPs now have nested/grouped declarative lifecycle, public
hydrated PUT, nullable override/empty mapping clears, scoped sync/delete and
dump/reload. They inherit explicit managed-directory authority without owning
associated principals. Proven independent OIDC detachments can precede trust
retirement. All twenty-three Identity live scenarios pass in US `.tech`;
the final trust lifecycle, early-stop cleanup and repository gates also pass.
Section 55 records the contract and evidence. #2664 stays In Progress until
main merge. Its original SDK checkpoint was `2597fe249e26`; the current preview
revision is described above.

Credential Providers now have nested/grouped declarative lifecycle under
explicitly managed directories: create/PATCH, selected deferred secret writes,
scoped sync/delete, adoption through the parent and public dump. The first live
US `.tech` scenario passes, including saved authority/credential guards and
missing-directory bootstrap. Section 53 records its contract and evidence.
#2660 stays In Progress until main merge. Sections 51–52 record the read and write
prerequisites.

Credential Provider configuration and the built-in provider-template catalog
now have public get/list support. Section 51 records the read surface and
new direct API findings. Inspection does not adopt resources or call
credential/enrollment endpoints.

Token Vault public keys now have declarative lifecycle and public get/list
support under explicitly managed directories. Fingerprints identify keys,
names can repeat, and key material changes create another record. Section 50
records collection authority, quota/rotation behavior and verification. This
requires no further SDK revision beyond `2597fe249e26`.

Directory deletion now requires explicitly enabled Vault and fresh empty
key/provider/Trusted IdP collections during planning and saved execution.
Disabled/unknown/incomplete observations block deletion without automatic
enablement or implicit child retirement. Section 49 records this boundary,
examples and verification. Exact authorized key/provider/trust retirements
can precede the fresh empty checks as described in sections 50, 53 and 55;
other attachments cannot cascade.

Basic-auth username records now support declarative lifecycle, independent
four-entry managed_by ownership, adoption, external UUID/username lookup,
scoped sync/delete and dump. Saved writes recheck authority; passwords block
username deletion, and API keys/unplanned usernames block principal deletion.
Reads expose public metadata only. The live lifecycle passes in US `.tech`;
section 42 records the boundary and evidence. The SDK expiration correction
is now verified (section 43). Explicit password create/capture, expiration,
material replacement and selected/all deletion are implemented separately from
declarations (section 46). Q35 still blocks metadata-only password adoption;
no declarative password ownership or rotation is claimed.

The initial survey below is historical. Development on `ki-triage` has since
delivered directories and principals, including declarative lifecycle, reads,
adoption, examples, and live E2E scenarios. Principal work is pushed in
`c1e3cba3`. Authorization-server root support is the next implemented slice;
its full live lifecycle scenario and protected-resource teardown pass.
Scopes and claims now have declarative lifecycle support as managed-server
children, plus get/list, examples and live E2E coverage. Their issues #1402 and
#1403 remain In Progress until merge. OAuth client and secret metadata get/list
are implemented. Explicit secret creation/capture, enable/disable and deletion
now have live rotation coverage. Client declarative lifecycle, adoption and
initial-secret capture pass live verification, including runtime credential
continuity. Sections 19–26 record contracts, verification and questions.
#1401 and #1404 remain In Progress.

The public preview SDK is now pinned to `2597fe249e26`, including targeted
serialization fixes for clearing origins, claim scope links and client arrays,
and for omitting an unset client authentication method.
Existing KongCTL SDK patches are retained. This continues the selective
public SDK approach rather than switching to the internal SDK's unrelated
model differences.

Principal identities now support all four declarative variants, independently
owned through five-entry `managed_by` maps. Nested/grouped configuration,
typed references, external principals, adoption, dump, saved plans and scoped
sync/delete are implemented. Fresh collection membership checks address the
GET/DELETE parent-scoping discrepancy (Q21); the SDK fixes cursor requests
(Q22). Principal deletion requires explicit identity deletions first, followed
by a fresh empty collection (Q24). Workspace writes/adoption remain deferred
until a valid API contract is demonstrated (Q25). Associated-resource deletion
has differing native API effects (Q26); checks cover declared collections,
with a warning about OAuth associations outside those scopes. Section 26
records this implementation and its verification.

Principal API-key inspection is implemented under #1417. US `.tech` supports
keys on user/agent/unclassified principals and non-unique names; original US
`.com` user/name limitations are deployment-specific. Management-map PATCH is ignored,
and omitted expiration is cleared by PATCH. Imported creation rejects
management metadata. These contracts defer declarative key writes/adoption;
scoped get/list exposes public metadata with complete pagination and no secret
material. Q27–Q30 record the API-only follow-up, and Section 27 records evidence.

Ordinary groups now support declarative lifecycle/adoption, independent
four-entry managed_by ownership and directory/name matching. Full PUT
preserves omitted public settings. Saved writes repeat scope/ownership and
protection checks; group deletion requires empty membership and key
observations. Principal deletion also guards incoming direct memberships.
Direct principal memberships now inherit managed ordinary-group authority:
add-only apply, explicit collection sync, saved references/deletion ordering,
dump, group adoption/protection and examples are implemented under #2658.
Team/importer and nested-group writes remain deferred (Q32).
Group/team and direct membership get/list preserve full pagination and read
presence. Group keys work in US .tech; earlier US .com returned 501, which
blocks safe declarative group deletion. The tested .tech org rejects team
creation. Q31 guards metadata precision; Q32/Q33 bound importer/key behavior.
Section 37 records the group lifecycle implementation and verification;
section 38 records direct principal membership lifecycle.

Principal active configuration now supports explicit false/true creation and
updates, saved plans, diff, dump, omission, protection and adoption. The SDK
already has the required optional request booleans. Missing response fields
remain unknown; explicit reconciliation requires observable state. Developer
portal deactivation is rejected according to the spec. Section 35 records
verification; Q10 still tracks deployment/provisioning differences.

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

Source: [principal concepts][principal-docs] and [directory
specification][directory-spec].

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

Source: [platform-api PR #3877][platform-pr] and [directory
specification][directory-spec].

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

Sources: [directory schemas][directory-spec] and [common metadata
schemas][metadata-spec].

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

Sources: [PR scope and exclusions][platform-pr] and [auth-server
source][auth-spec].

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
and labels can be cleared explicitly; client serialization retains integer
precision. Later direct API tests found server-side rounding of some large
integers; see Q31 and Section 28 for the additional validation.
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

## 21. OAuth clients and secret metadata inspection

Get/list commands now inspect OAuth clients by an explicit parent server and
opaque client ID or `--client-name`. They expose public configuration in
JSON, YAML and text. Client IDs are not inferred from UUID syntax. Exact-name
lookup scans all pages, rejects duplicates and rechecks the detail response.
Secret inspection requires an explicit client ID/name within that server;
its output type contains only ID, enabled state and timestamps. Secret values
cannot be serialized by these commands, even if returned unexpectedly.

Direct HTTP probes confirmed unique client names (409 on duplicate), label
merge/null removal, array clearing, durations, scope policies and one-time
generated secrets. They also found new engineering questions:

- Q15: labels-only PATCH resets an omitted client authentication method to
  `client_secret_post`, including from `client_secret_basic` or `none`.
  Future writes must preserve the fresh method explicitly.
- Q16: the two-secret limit counts disabled records. A third creation returns
  403 even with no enabled secrets; explicitly deleting a record frees a slot.
- Q17: authorization-code grants are enumerated in the schema but rejected
  in the current US `.com` environment.
- Q18: two clients linked to two scopes each report client-list total four.
  Terminal pages contain no additional clients. Client observations now follow
  actual pages rather than that total; incomplete results still fail closed.
  This correction also applies to existing scope-deletion reference guards.

Client labels provide a future independent ownership store. Secret records
have no labels, `managed_by`, or name. No declarative secret-slot identity is
invented. Client creation and explicit rotation still require secure capture,
retry/recovery behavior and SDK serialization fixes for empty arrays before
they can ship. No client writes are exposed by this inspection slice.

A direct runtime probe minted a client-credentials token with the original
secret before and after a label PATCH that explicitly retained
`client_secret_basic`; both returned 200. Deleting the secret produced 401.
Creating a replacement with supplied material returned 201, echoed the
supplied value once, and restored token issuance (200). Values stayed in
memory and were never logged; the temporary client/server were removed.

The new scenario creates two clients with multiple scope links, a disabled
second secret and a second server. It checks pagination, every public read
field, parent/client isolation, selector errors and all output formats.
Fixture cleanup scans all child collections before deleting anything, and
deletes only generated clients bearing the server-ID fixture label. This also
supports a deliberate stop immediately after creating a secret.
Enabled replay coverage and recordings are unchanged.

Live artifacts under `/home/rspurgeon/.cache/kong-identity-public-sdk/`:

- `oauth-read-live-fixed/20261031-010726`
- `oauth-read-stopped/20261031-010825`
- `oauth-client-observation-regression/20261031-010908`

The first client run exposed Q18; guarded cleanup refused the incomplete
inventory. All of those exact generated fixtures were subsequently inspected
and removed. The corrected run and stopped run both clean up successfully.
The scope/claim lifecycle scenario also passes with the client traversal fix.
An artifact scan checked 193 JSON files in the successful read/stopped runs
and found no unredacted `secret` or `client_secret` fields.

All final quality gates pass: modernization, formatting, CGO-disabled build,
zero-issue lint, unit/integration/harness race tests and scenario inventory.
`oauth-read-final-status.json` records all zero exit codes. A final remote
inventory is empty after fixture cleanup.

Commands and boundaries are documented in
`docs/examples/declarative/identity/auth-server-clients.md`. #1401 and #1404
remain open/In Progress; their declarative and rotation acceptance criteria
are not complete. API questions and metadata findings are in the running logs.

## 22. Explicit client-secret lifecycle and capture

The new `create`, `patch` and `delete identity auth-server-client-secret`
commands manage explicit secret records under a selected server/client.
Both direct and `konnect` forms work. Creation generates material or imports
an env/file source, reserves a new `0600` capture file before POST, and returns
only metadata to stdout. Existing paths and symlinks fail before creation.
Enabled state is explicit for PATCH; deletion uses standard confirmation.
There is no automatic secret-slot retirement or ordinary declarative rotation.

Capture includes server/client/secret IDs and issued material. A failure keeps
recovery evidence; uncertain creation is never replayed through SDK or HTTP
transport. Missing response values and missing/replaced output paths fail with
known record IDs when available. No automatic rollback deletes the record.
Source parser errors and SDK error bodies cannot publish supplied material.
Q19 asks engineering for the API's uncertain-create recovery contract.

The `identity/client-secrets` live scenario verifies generated and imported
material, `0600` capture, overwrite refusal, two-slot rejection, metadata reads,
all mutation routes and validation errors. It also authenticates with the
captured values: disabled and deleted secrets return 401; enabled generated
and imported secrets mint tokens (200). The helper reports only status and
presence booleans. Secret files live in a private test-managed directory
outside artifacts and are removed after remote fixture cleanup, including
deliberate stops. The scenario does not expose material in recorded vars or
public output.

The public preview SDK is `221af2edb788`, with KongCTL patch source
`2489716c`. SDK wire regressions failed before the new fixes: empty client
arrays were dropped and an omitted authentication method was injected as
`client_secret_post`. The patched SDK preserves empty grant, response,
redirect and allowed-scope arrays and omits an unset method. The server's Q15
defaulting still requires future client writers to carry the fresh method.
All previous targeted patches are retained. Multiple replacements in one file
now accumulate before writing; patch-tool tests cover that and atomic drift
failure. The SDK branch is pushed without release or PR.

Initial client creation and its generated-secret capture remain part of
#1401; no client declarative declaration or write executor is enabled yet.
#1404 remains open/In Progress until merge and coordination with #1401.
The metadata ledger records that secret capture IDs are local output metadata,
not remote ownership/namespace/protection storage.

Final artifacts under `/home/rspurgeon/.cache/kong-identity-public-sdk/`:

- `secret-lifecycle-isolation-final/20261031-014115`: complete rotation at trace
  level, including wrong-client PATCH/delete rejection and runtime checks.
- `secret-lifecycle-stopped-final/20261031-014205`: deliberate stop immediately
  after issuance; remote fixture cleanup and private directory removal pass.
- `secret-sdk-auth-children-regression/20261031-014252`: existing full
  auth-server/scope/claim declarative lifecycle with the updated SDK.

The live helper scans public artifacts for both issued values and Basic-auth
encodings before removing private captures; the check is clean. Five distinct
capture paths across full/stopped runs were absent after teardown. The final
remote server inventory is empty. All quality gates pass, with zero status
codes in `secret-lifecycle-final-status.json`. Patch-tool tests (seven cases)
and SDK root/models/internal/pkg tests also pass. Enabled replay scenarios
and cassettes are unchanged. No implementation PR or SDK release was created.

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

## 23. Declarative OAuth clients and initial credentials

Issue #1401 now implements nested `identity.auth_servers[].clients` and grouped
`identity.clients`, including independent namespace/protection labels, exact
unique-name matching, parent isolation, typed same-plan scope references and
all public client fields. Required API name/grant/response fields stay explicit.
Clients can belong to an external server and a different namespace from their
parent. Their namespace does not broaden auth-server pruning.

Plan, diff, saved apply, sync, delete, dump, explain/scaffold and adoption are
integrated. Omitted optional fields preserve current configuration; explicit
empty arrays clear it. Every client PATCH and adoption preserves the fresh
authentication method when omitted (Q15), without sending credential material.
Scope deletion can follow a proven client detachment or deletion, but retained
or late references still block it. Saved writes recheck remote parent identity
and client identity, ownership and protection.

Initial client creation requires `--secret-output-dir` at execution. Before any
API mutations, the executor reserves new `0600` capture files for all client
creates. Resolved parent IDs are persisted before POST. Generated secret values
exist only in those files; normal execution reports paths and resource IDs.
Plans, dumps and ordinary output contain no material or output paths. Unused
reservations are removed; uncertain attempts retain recovery metadata. SDK and
transport issue a single creation attempt. No automatic rotation or rollback
tries to recover an inaccessible value.

The new `identity/declarative-clients` scenario verifies all public fields,
secret-file privacy, saved-plan application, repeated no-op plans, dump/replan,
updates and empty arrays, scope detachment/removal, protection, adoption,
external servers, namespace-isolated sync, explicit deletion and cleanup.
It mints real client-credentials tokens using the initial secret before and
after updates/adoption and scans artifacts for both material and Basic-auth
encoding. The full live run passed at
`/home/rspurgeon/.cache/kong-identity-public-sdk/declarative-client-live-4/20261031-023002`.
Final validation and early-stop cleanup are recorded with the implementation
commit. Existing replay-enabled coverage is unchanged.

Q20 adds one API-only question: whether the working `client_secret_basic`
method should retain its canonical `x-enum-dev` annotation. The public preview
enum filters it out, while SDK passthrough and live authentication work.
Validation accepts this verified canonical value explicitly. Authorization-code
and refresh-token grant enums remain excluded from this preview; Q17 records
the deployed authorization-code denial. Unsupported internal client fields and
independent secret-slot declarations remain outside the implemented contract.

Examples: `docs/examples/declarative/identity/auth-server-clients.yaml` and its
companion tutorial. Metadata: client labels; secret records still have no
remote ownership store. Issues #1401/#1404 remain In Progress until merge; no
implementation PR has been opened.

Final client validation: modernize, clean formatting, build, lint, unit race
tests, integration tests, harness race tests and weighted scenario inventory
all pass. All seven live Identity scenarios pass together at
`/home/rspurgeon/.cache/kong-identity-public-sdk/declarative-client-identity-regression/20261031-023727`.
A final client run also verifies clearing the login URI at
`/home/rspurgeon/.cache/kong-identity-public-sdk/declarative-client-final-live/20261031-024003`.
Early-stop cleanup passes at
`/home/rspurgeon/.cache/kong-identity-public-sdk/declarative-client-stopped/20261031-024044`.
All five private capture paths from these runs were removed after cleanup.

## 24. Principal-identity contracts and inspection

Issue #1416 is In Progress. Direct probes verified custom, OIDC, OAuth-client
and API Gateway consumer identities. All generate IDs without user IDs and
retain all five managed_by ref/ownership values plus public labels on create,
GET/list and PUT. Custom and OIDC association fields update through PUT;
custom type-specific PATCH works. Labels/managed_by PATCH is rejected. A
probed discriminator change was rejected. The API Gateway response returns
read-only `control_plane_type: api_gateway`.

The detail/delete paths have a reproducible discrepancy (Q21). With two
existing principals in one directory, GET through the wrong principal returns
the other principal's identity and DELETE actually removes it. PUT through
the wrong principal returns 404. The collection routes filter correctly. New
get/list commands inspect all four types and verify complete parent membership
before a detail GET. Future writers must use fresh membership proof before
mutations; GET alone cannot establish parent identity. Fixture cleanup applies
that guard and checks marked descendants before removing anything.

A generated SDK request mismatch was also found. Directory, principal and
identity list requests emitted flat size/after keys, while the deployed API
uses bracketed page[size]/page[after]. Flat keys are ignored, leaving page size
20, which hid the problem in earlier small collections. SDK wire tests fail
before the correction and pass afterward. Preview SDK 6ca9d1f8b309 changes
only those three request tags, with patch source ef232ca0. No API additions,
response-model differences or internal SDK shapes are introduced. Earlier
KongCTL patches remain installed; the operation inventory is still 112 new
Identity operations and 990 baseline operations. Q22 records the missing
shared specification style.

The read scenario verifies actual ten-page traversal from trace records, all
four types, metadata, opaque OAuth client IDs, Gateway response type, workspace
omission, JSON/YAML/text, explicit product selection and wrong-parent refusal.
The original attempt at 24 fixtures hit a maximum-ten identity quota; Q23 asks
about quota scope and discoverability. The scenario now stays within that
observed limit while verifying cursor requests directly. No artificial local
quota is imposed. API probes and failed-run cleanup removed their fixtures.

The next #1416 increment remains the complete declarative lifecycle: nested
and grouped declarations, persisted-ref identity, typed and external refs,
same-plan dependencies, hydrated PUT updates, adoption, independent ownership
and protection, dump, safe sync/delete, and lifecycle examples/E2E. Existing
importer/JIT/Portal-owned identities cannot be managed merely because they
appear under a managed principal. Parent deletion/cascade contracts also need
explicit coverage when children gain independent ownership.

Final verification for this increment: modernize, clean formatting, build,
lint, unit race tests, integration tests, harness race tests, patch-tool tests
and weighted scenario inventory all pass. All eight live Identity scenarios
pass together at
`/home/rspurgeon/.cache/kong-identity-public-sdk/identity-inspection-all-regression/20261031-031724`.
The new scenario verifies ten identity pages through explicit request-trace
assertions rather than inferring traversal from a small result count.
Early stop after the seventh custom association also passes at
`/home/rspurgeon/.cache/kong-identity-public-sdk/identity-inspection-stopped/20261031-031917`.
The three generated client capture files in the complete run and the stopped
run's capture were removed. Fresh inventories show no directories, auth
servers or remaining probe/test control planes. Replay-enabled coverage is
unchanged. Work remains on branches; no PR opened.

## 25. Identity write foundations and principal deletion boundary

Further direct US `.com` probes verify that OAuth-client identities can move
to a different server/client and API Gateway consumer identities can move
to a different control plane/consumer through PUT, retaining all five
management entries. AI Gateway OAuth consumers also link using the
`control_plane_consumer` shape; responses report `ai_gateway` and metadata
PUT retains the association. Cross-Gateway-type retargeting was not tested.
Every probe fixture was removed afterward, including both Gateway families.

Principal DELETE succeeds with identities still attached. The principal and
its identity collection then return 404. The inspected delete operation
does not specify descendant retention or a nonempty-child precondition.
API-only question Q24 asks engineering to document identities, credentials
and membership behavior and provide an atomic conditional-delete contract.

Declarative principal delete and sync pruning now require a complete empty
identity collection, both when generating a plan and immediately before the
parent delete. This applies regardless of identity ownership, namespace or
protection. Missing/failed observations stop deletion. It protects saved plans
against later identities, but a concurrent creation between the last check
and DELETE requires an API precondition to eliminate. This check is specific
to identities; other descendant families need their own verified treatment.

Shared Identity write helpers prepare complete PUT bodies for all four
variants, excluding read-only fields while preserving labels, managed_by and
workspace presence. Create disables both POST retry layers and verifies
parent-scoped read-back. PUT/DELETE require fresh complete membership proof;
PUT rejects type changes and verifies response identity/metadata. These are
implementation foundations: no new imperative mutation commands or
declarative identity declarations are exposed. Persisted-ref matching,
metadata stamping, independent ownership/protection, typed/external refs,
same-plan dependencies, dump, adoption and lifecycle examples remain active
#1416 work. Existing importer/JIT/Portal identities remain outside management.

Verification: modernize, clean formatting, build, zero-issue lint, unit race
tests, integration tests, harness race tests and weighted scenario inventory
pass. All eight live Identity scenarios pass together at
`/home/rspurgeon/.cache/kong-identity-public-sdk/identity-write-guard-regression/20261031-034108`.
The focused scenario also passes at
`/home/rspurgeon/.cache/kong-identity-public-sdk/identity-write-guard-live/20261031-034019`.
It saves a principal-delete plan while the collection is empty, adds ten
identities, then verifies both new delete/sync planning and saved-plan
execution refuse removal while the principal remains readable. All four
generated client capture paths across these runs were removed; fresh
directory/auth-server inventories are empty. Replay-enabled coverage is
unchanged. Work remains on `ki-triage`, with no PR opened.

## 26. Declarative principal identities

Issue #1416 now implements custom, OIDC, OAuth-client and Gateway consumer
identity associations. Nested `principals[].identities` and grouped
`identity.identities` declarations support plan, diff, saved apply, sync,
delete, dump, explain/scaffold and adoption. Required association fields stay
explicit; generated IDs are discovered through complete parent collections
and independent tool/namespace/ref identity in five-entry `managed_by` maps.
Public labels remain ordinary API data. Parent ownership/protection is
independent, including external principals selected by UUID.

Updates hydrate full PUT bodies from fresh reads, retain omitted public
fields, and verify current identity, parent membership, ownership, protection
and duplicate tuples. Creates issue a single POST attempt. Type changes
require separate associations. Adoption preserves public configuration and
protection while explicitly changing the ownership tuple. Unowned principals
and identities dump as external references. Child namespaces and explicit
empty collections cannot broaden directory/principal pruning authority.
Existing referenced IDs resolve before protected payload comparison, including
targets in other namespaces that need no operation.

Final plans require explicit authorized identity deletions before deleting a
principal; execution still requires a fresh empty collection. For observed
OAuth-client/server or Gateway references, deletion/retargeting precedes
target deletion. Saved plans retain the observed directory/principal scopes
and recheck them at execution, so late unowned associations also block removal.
These checks cover declared collections, not an organization-wide reverse
inventory. OAuth-client deletion warns about native effects outside those
scopes. API atomic preconditions remain necessary to close concurrent races.

Two additional engineering questions record direct US `.com` observations:

- Q25: consumer identities accept omitted workspace, but text and UUID-shaped
  workspace values return 500 on otherwise working PUTs. Neither probe used a
  known existing workspace. A valid example, format, availability and clearing
  contract are needed. Declarative workspace writes and workspace-bearing
  adoption are deferred; reads retain the field and dumps use external refs.
- Q26: deleting an associated OAuth client removes its identity from GET/list;
  deleting an API Gateway consumer and then its control plane leaves the
  identity visible with the old IDs. AI consumer deletion was not verified.
  Engineering should establish the intended retention/deletion contract.

Further direct probes verify AI-to-API Gateway consumer retargeting and
management-map retention. All disposable probe fixtures were removed.
The SDK pin remains `6ca9d1f8b309`; this increment changes no generated models.

The example is `docs/examples/declarative/identity/principal-identities.yaml`,
with a short companion tutorial. The new live scenario
`identity/declarative-identities` covers all four types under a protected
principal, independent namespaces, same-plan client dependencies, no-op
planning, association updates, label removal, diff, dump/replan, adoption,
protection, omitted/empty sync, foreign/unowned preservation and ordered
parent teardown. Its focused live run passes at
`/home/rspurgeon/.cache/kong-identity-public-sdk/declarative-identity-live-2/20261031-044615`.
Unit regressions also cover saved-plan ownership/protection/duplicate changes,
failed observations, late references and external dump boundaries.

Final validation passes: modernization, clean formatting, CGO-disabled build,
zero-issue lint, unit race tests, integration tests, harness race tests,
weighted scenario inventory and replay/metrics checks. All nine live Identity
scenarios pass together at
`/home/rspurgeon/.cache/kong-identity-public-sdk/declarative-identity-final-regression/20261031-054209`.
Early stop after creating and inspecting all four types also passes at
`/home/rspurgeon/.cache/kong-identity-public-sdk/declarative-identity-stopped/20261031-054437`.
The complete run's five generated client capture paths and the stopped run's
two paths were removed. Additional direct custom/OIDC probes verify that PUT
with omitted labels or explicit empty labels clears them while retaining the
management map; fresh hydration therefore preserves omitted manifest labels.
Those disposable probe fixtures were removed too. Replay-enabled coverage is
unchanged. #1416 and related issues remain open/In Progress until main merge;
work stays on `ki-triage`, with no implementation PR opened.

## 27. Principal API-key contracts and inspection

Issue #1417 is In Progress. Direct US `.com` probes compare principal types:
user, agent, explicit null and omitted type are all readable after creation,
but API-key create/list return "principal not found" only for user. Agent and
both unclassified variants support generated keys. Creation retains all five
management entries and a supplied future expiration. Imported creation with
management metadata returns 400; without it, creation succeeds with an empty
map. Duplicate imported material under another principal returns 409.

Both generated/imported PATCH ignore complete and partial management-map
changes while returning success. Explicit expiration updates and null clearing
work; omitted expiration is also cleared. PUT returns 405. Name-bearing create
is rejected, while name-bearing PATCH is ignored and reads omit the field.
These are Q4/Q27–Q29 contracts. Verified adoption, mutable ownership/protection
and the normal declarative lifecycle remain pending working metadata updates
and atomic imported creation.

Key GET/PATCH/DELETE through a different existing agent return 404, and its
collection is empty. Thus the principal-identity scoping workaround is not
assumed for keys. Principal deletion with keys returns 204; their detail and
collection then return 404 (additional Q24 evidence). Runtime calls with an
administration token return "mTLS headers missing" (Q30). They do not prove
credential continuity or revocation. All disposable probe fixtures were removed.

The product adds `get|list [konnect] identity principal-api-keys` and individual
UUID lookup, with explicit directory name/ID and principal UUID. A public-only
projection includes expiration, management metadata and timestamps, plus a
name if returned. Material and inferred creation methods are absent. Complete
cursor observations reject page errors, duplicate IDs/cursors and malformed
responses without returning partial inventories. Credential errors omit raw
response bodies.

SDK `70a2818dde13` adds one request-tag correction for principal API-key lists,
from patch bundle `0103edc5`. Wire tests fail before and pass after it; prior
patches remain, with no model/API-inventory changes. The mutable expiration
request field is separately missing because the merged shared `ExpiresAt`
schema is read-only, unlike the canonical directory input. That must be
corrected before write enrollment; direct API expiration support is verified.

The example tutorial is
`docs/examples/declarative/identity/principal-api-keys.md`. The live inspection
scenario covers generated key metadata under agent/unclassified principals,
all output formats, explicit product selection, empty collections, wrong-parent
refusal, actual three-page traversal and artifact scans. Test material remains
in private captures. Opted-in cleanup verifies marked key collections before
any deletion and rechecks metadata/emptiness before removing parents. The
first focused run passes at
`/home/rspurgeon/.cache/kong-identity-public-sdk/principal-key-inspection-live/20261031-060908`.

Final validation passes: modernization, clean formatting, CGO-disabled build,
zero-issue lint, unit race tests, integration tests, harness race tests,
weighted scenario inventory and replay/metrics checks. All ten live Identity
scenarios pass together at
`/home/rspurgeon/.cache/kong-identity-public-sdk/principal-key-inspection-all-regression/20261031-061542`.
Stopping immediately after creating all four generated keys also passes at
`/home/rspurgeon/.cache/kong-identity-public-sdk/principal-key-inspection-stopped/20261031-061846`.
The five client capture paths and the key scenario's private capture directory
were removed after the complete run; the stopped run's private directory was
also removed. Fresh directory/auth-server/control-plane inventories are empty.
Replay-enabled coverage is unchanged. Work remains on branches, with no PR.

Principal production delete/sync currently observes identity associations;
API-key descendant checks are still pending key lifecycle enrollment and a
verified user-type applicability contract. Fixture cleanup checks are separate
test authority. Further feasible work includes groups/memberships after live
verification and other credentials where their contracts support safe writes.

## 28. Group contracts and integer metadata retention

Group work has started under #2657, which remains In Progress. Direct US `.com`
tests verified ordinary groups and the team subtype, unique alphanumeric names
within a directory, four-page cursor traversal, full replacement, immutable
type and retention of the four directory-style `managed_by` ownership values.
Omitting public maps from PUT clears them, so future declarative updates need
fresh hydration. No group resource implementation is exposed yet.

Ordinary groups accept user, agent and unclassified principal memberships;
duplicate members return 409. Group-to-group membership returns 400 despite
the inspected schema. Manually created teams accept user memberships, and
those manual relationships can be removed. No importer fixture was involved.
Q32 asks for the intended member-type and importer authority contract. Group
deletion removes membership relationships while member principals remain.
Group API-key GET and POST return 501 (Q33); credential lifecycle and ownership
retention cannot be inferred from the shared principal-key schema. Disposable
probe groups, memberships, principals and directories were removed.

These probes also exposed a precision bug in the deployed metadata API.
Principal PUT and group POST/PUT accept `9007199254740993` but return
`9007199254740992`; fresh GET confirms the changed digits. Arrays and negative
values are affected. `1152921504606846976` returns as `1152921504606847000`,
while `9007199254740994` and the returned large decimal remain stable. Exact
Python integer decoding established these differences. Q31 asks for the
supported numeric contract and data-plane behavior; the binary64 conversion
model is an inference from observations.

Principal manifest validation and saved-plan execution now reject integer
digits that change through that observed conversion. Stable integers and
strings retain their API types. Create readback and PUT responses are checked
against requested metadata; mismatches fail explicitly after the remote write,
without retry or rollback. Existing metadata omitted from a manifest is still
preserved through fresh hydration. This concerns runtime `metadata`, not the
string ownership map in `managed_by`. Exact numeric identifiers should use
strings. The compatibility tracker records when these guards can be relaxed.

The new `identity/metadata-integers` live scenario passes at
`/home/rspurgeon/.cache/kong-identity-public-sdk/identity-integer-live/20261031-065350`.
It rejects affected scalar/array values before writes, applies a saved plan
with stable large integers and an exact string identifier, compares raw JSON
digits after create and omission-preserving update, verifies no-op planning
and dump/replan, and deletes the two managed resources. Numeric equality does
not use floating-point assertion decoding. Existing client-only large-integer
fixtures now use a server-stable value; local precision tests alone did not
prove the deployed API preserved arbitrary integer digits.

Modernization, clean formatting, CGO-disabled build, zero-issue lint, unit race
tests, integration tests, harness race tests, weighted inventory, SDK patch,
installer, replay and metrics checks pass. All eleven live Identity scenarios
pass together at
`/home/rspurgeon/.cache/kong-identity-public-sdk/identity-integer-all-regression/20261031-065506`.
Stopping the new scenario immediately after saved-plan creation/apply also
passes fixture cleanup at
`/home/rspurgeon/.cache/kong-identity-public-sdk/identity-integer-stopped/20261031-065803`.
The complete run's five generated capture paths and five private directories
no longer exist. Fresh directory/auth-server/control-plane inventories are
empty. Replay eligibility and cassettes are unchanged. SDK pin remains
`70a2818dde13`; this increment changes KongCTL validation and verification,
not the API schema or SDK model.

## 29. Group and direct membership inspection

The branch now exposes `get|list [konnect] identity group[s]` and
`group-membership[s]`. Groups accept an exact name or UUID within an explicit
directory. Membership detail accepts the relationship entry UUID, distinct
from its member UUID, with an explicit group UUID/name and directory UUID/name.
Both resolvers prove collection membership before fresh detail reads. Readers
reject malformed/missing collection metadata, repeated IDs/cursors and
duplicate member targets without exposing partial output. Public fields are
preserved, while absent provisioning/team-manager fields gain no SDK defaults.
JSON, YAML and text formats are covered.

SDK wire tests reproduced flat query keys on `ListGroups` and
`ListGroupMembership`. Patch source `f79e767d` extends the current Q22 request
tag fix to six Identity operations. SDK `bf1d470c108c` includes those two
replacements and the complete existing patch snapshot; SDK unit tests pass.
The Go module replacement consumes that branch commit. No new spec release or
SDK PR was created.

Further direct API tests verified ordinary-group and manual-team membership
removal. Principal GET reports group IDs as links change; principal deletion
removes those links. Missing-directory routes reject reads. The organization
permits only one directory, so scope isolation between two existing directories
could not be directly established. Complete collection proofs avoid relying
on detail routes alone. Q32 records the observations and remaining importer
authority question. Q33 still prevents verifying group credential behavior.

The new `identity/groups` live scenario passes at
`/home/rspurgeon/.cache/kong-identity-public-sdk/group-read-live/20261031-071956`.
It covers four groups (ordinary, team, explicit-null default and omitted-type
default), three principal types, ordinary/team direct entries, all output
formats, explicit product aliases, public metadata, entry/member-ID distinction,
wrong-group rejection and real four-page/three-page cursor traversal. Cleanup
opts into manually created, credential-free groups; all fixture ownership and
member scopes are checked before deletion, then freshly rechecked before
removing entries/groups/parents. Existing scenarios gain no group permission
requirement. This fixture authority does not authorize production group-key or
importer-managed relationship mutation. The tutorial is
`docs/examples/declarative/identity/groups.md`.

Final validation passes: modernization, clean formatting, CGO-disabled build,
zero-issue lint, unit/integration/harness race tests, weighted inventory,
SDK-patch/installer checks, and replay/metrics checks. All twelve Identity live
scenarios pass together at
`/home/rspurgeon/.cache/kong-identity-public-sdk/group-read-all-regression/20261031-072142`.
Stopping immediately after all four groups and their memberships are created
also passes at
`/home/rspurgeon/.cache/kong-identity-public-sdk/group-read-stopped/20261031-072454`.
The complete run's five generated capture paths and five private directories
were removed automatically. Replay eligibility/cassettes are unchanged.
Fresh directory/auth-server/control-plane inventories are empty after both
the complete regression and stopped run.

## 30. Requested stopping point and remaining work

The user requested a pause after completing the current verified increment.
The objective remains the feasible work in epics #1132/#1133; no issue should
close before merge to main. Work remains on `ki-triage`, with no implementation
PRs. The full API-only question list and metadata ledger remain the handoff
sources, alongside compatibility removal criteria.

| API surface | Declarative resources implemented | Important boundaries |
| --- | --- | --- |
| Directory administration | Directories; user/agent/unclassified principals; OIDC, custom, auth-server-client and Gateway-consumer principal identities | Plan/diff/apply/sync/delete/dump, references and independent ownership supported. Principal type is create-only, active is deferred, workspace-bearing identities are deferred and affected integers are rejected. Adoption/get/list are available for supported resources. |
| Auth-server administration | Auth servers; scopes; claims; OAuth clients, including private initial-secret capture | Broad lifecycle, refs, dump and reads supported. Scope/claim authority is through the managed parent and explicit collections; parent adoption supplies authority. OAuth clients have independent ownership labels. Secret rotation/enable/disable/delete are explicit commands, not named declarative secret slots. |
| Runtime authentication/lookup | No new declarative resource family | OAuth client credential continuity is tested through token requests. Identity credential runtime checks still need the supported mTLS context (Q30). Visibility/authentication responses do not grant management ownership. |

Inspection-only additions are principal API keys, groups/teams and direct group
memberships. Group and membership declarative lifecycle remains the current
next implementation target. It needs independent group ownership/adoption,
explicit membership collection authority, saved parent/member dependencies,
reference and deletion guards, dump/explain/scaffold, configuration examples
and full lifecycle scenarios. Group key availability and imported-team rules
must remain explicit rather than being inferred from a shared SDK schema.

Other pending families are principal API-key writes/adoption, basic-auth
credentials/passwords, CSV and organization-chart imports, Token Vault keys,
credential providers/shared credentials, user-enrolled credential inspection
and revocation, and Trusted IdPs. Supported subsets should proceed only after
their own deployed lifecycle, ownership, secret and matching contracts pass.

The preview SDK uses the public SDK structure with the selected directory and
auth-server specifications and existing KongCTL patches. Its current revision
is `bf1d470c108c`, on `kongctl/kong-identity-public-preview`, locally at
`/home/rspurgeon/dev/kong/ki-public-sdk-build/sdk-konnect-go`. SDK inclusion
issues #1413/#1399 stay In Progress until the normal upstream generation and
main-branch integration are complete. No additional SDK work is required for
the implemented read/lifecycle slices. Before principal key writes, reconcile
the missing request `expires_at` field with its canonical spec contract and
verify omitted/value/null serialization; that is separate from API questions
about ignored ownership updates.

Verified on October 31: [platform-api product inclusion PR
#3895](https://github.com/Kong/platform-api/pull/3895) merged at 07:21:50 UTC,
adding both management specifications to both Go SDK product groups.
[GA gating PR #3877](https://github.com/Kong/platform-api/pull/3877) remains
open at revision `98137d7be3772b67693cc1ceb0469e77f8ecf0f5`. Product inclusion is
therefore complete upstream; normal public availability of the proposed
additions still depends on gating changes and downstream SDK generation.

Pending engineering questions are in
[`docs/kong-identity-api-questions.md`](kong-identity-api-questions.md).
They cover directory Vault/default/quota behavior (Q1–Q3), ownership and stable
matching on forthcoming resources (Q4–Q8), principal type/active/PATCH behavior
(Q9–Q11), auth-server pagination/retry/authentication/secret limits (Q12–Q20),
identity scoping/quotas/workspaces/native deletion effects (Q21–Q26), API-key
applicability/update/name/runtime contracts (Q27–Q30), integer precision (Q31),
membership/importer boundaries (Q32), and group key availability (Q33). Several
have bounded workarounds; they are not all blockers for existing support. No
engineering answers have been recorded. The
[metadata ledger](contributor/kong-identity-metadata-storage.md) distinguishes
labels, `managed_by`, and resources without an independent ownership store.

## 31. Default-directory manual demonstration setup

At the user's request, the `.tech` organization selected by profile
`rspurgeon-e2e-kongctl-testing_tech` now has a directory named `default`, ID
`01a1922b-ed6a-7c8d-9938-7164fb435035`. It was created through the API with the
reported new `.com` organization settings: empty description/labels/managed_by,
all control planes allowed, an empty allowlist, positive/negative TTLs of 600
seconds and Vault disabled. Fresh detail GET verifies every supplied value.
This is the user's manual-demo baseline, not a disposable scenario fixture.

Examples cover both common entry paths. `external-default.yaml` and the normal
principal/identity tutorials resolve an existing default without adoption or
directory mutation. `directories.yaml` separately bootstraps a missing default
with the same public settings, KongCTL ownership under `identity-directory`
and protection enabled. An existing unowned default requires explicit adoption
before that managed manifest can be used. Both entry paths then use external
lookup in the service tutorial, keeping directory lifecycle separate from its
independently managed principals and identities.

Read-only plans against the prepared `.tech` organization verify two tutorial
principal creations, three principal-example creations, and five combined
service-demo creations, all with zero directory mutations. The managed
bootstrap manifest correctly rejects the existing unowned default rather than
taking ownership implicitly. No principal, OAuth client or auth server has
been created by this setup. The manual guide and manifests are under
`.cache/identity-orders-demo`; earlier plans that created a separate demo
directory were archived. Example/documentation edits remain local pending
review and commit.

## 32. Manual demonstration follow-ups

These are implementation follow-ups, separate from API-only questions.
Completed items are marked below as manual review identifies improvements.

- [x] Default directory selection: directory-scoped imperative commands imply
  `--directory-name default` when neither directory flag is supplied.
  Explicit name/ID overrides and mutual exclusion are preserved. Missing
  default fails lookup without automatic creation/adoption. Auth-server
  commands retain their own parent selectors without a directory dependency.
- [ ] Configurable directory selection: bind `--directory-id` and
  `--directory-name` to profile configuration paths using the existing flag
  binding conventions, with environment-variable support. Document the config
  paths in command help. Explicit directory flags must override configured
  selectors, including a selector of the other kind; reject conflicting
  effective selectors. Use `default` only when neither flags nor configuration
  select a directory. Choose the exact config paths during implementation.
- [ ] Text detail rendering: an individual `auth-server-client` requested with
  `-o text` currently goes through a branch that JSON-formats the detail
  object. Replace that shortcut with the normal text/detail presentation and
  check other newly added Identity commands for the same inconsistency.
- [ ] Token-duration diff rendering: `access_token_duration` is incorrectly
  redacted by the generic sensitive-field display policy; keep actual secret
  redaction while showing non-secret numeric durations.
- [x] Principal active implementation: October 31 native `.tech` probes now
  verify false creation, true/false updates and omitted-field preservation
  across directly provisioned user/agent/unclassified principals (Q10).
  Declarative support and lifecycle coverage are implemented in section 35,
  including presence-aware reads and missing-field compatibility.

The user's first manual create/read pass verifies an agent principal with the
five-entry `managed_by` tuple, application labels and runtime metadata, plus
an OAuth client with independently owned labels, `client_credentials`, an
explicit allowed scope, a 600-second access-token lifetime and
`client_secret_basic`. The OAuth client ID is opaque (`ka5swfe0x78oz86x`),
whereas the principal ID is a generated UUID. No implementation changes were
made in response to these observations. Q34 now asks about the intentional
lack of `managed_by` on auth servers and OAuth clients; the metadata ledger
already records label-based ownership for both.

## 33. Revalidate production findings against the prerelease environment

The user identified `.tech` as the intended current-spec testing environment.
The original `.com` observations remain production compatibility evidence;
they must not be generalized into prerelease API limitations. The engineering
question log now starts with an environment-specific revalidation matrix for
all question families.

Read-only direct `.tech` checks confirm active=true on the manual agent,
an empty principal API-key collection with HTTP 200, and an empty group list.
Only active visibility changes the earlier `.com` assessment. These reads do
not establish type mutability, active transitions, credential ownership PATCH,
large-integer preservation or group-key deployment. The manual identity GET
also confirms the expected server/client association and persisted ownership.

Prioritize disposable principal field/map/numeric probes, credential writes,
groups/keys/memberships and auth-server native PATCH/pagination behavior.
Preserve the user's default directory and manual registrations. Do not use
KongCTL's existing guards or hydration as evidence of the native API contract;
direct requests are needed to decide whether those workarounds can be lifted.
No implementation restrictions were changed during this read-only pass.


## 34. Native US .tech revalidation results

October 31 direct API revalidation completed 272 requests across directory,
principal, group, credential, auth-server and runtime surfaces. These are
environment-specific observations, not a guarantee that every organization
has the same release or entitlements. The earlier US `.com` evidence is
retained and labeled separately in the
[API question log](kong-identity-api-questions.md).

### Availability improvements

- Principal `active`: false creation, detail/list reads, PUT activation,
  PATCH deactivation/reactivation and omitted-field preservation all work.
  User, agent and unclassified principals retain false during unrelated
  PUT/PATCH. These checks use directly provisioned records; other
  provisioning paths remain unverified.
- Directory Vault: `vault_enabled: true` persists and enables provider/key
  lists. The setting was restored to false. Provider templates list eight
  entries. Provider, credential, trusted-IdP and key write lifecycles were
  not validated by these availability probes.
- Principal keys: user, agent and unclassified parents accept both generated
  and imported keys. Names and renames work, but duplicate names are accepted.
- Group keys: list/create/read/PATCH/delete are available. Generated creation
  retains the supplied ownership map. Imported creation rejects it, and
  PATCH does not update ownership. Generated names work at creation; imported
  names and subsequent renames are ignored in the tested group-key routes.

### Restrictions still necessary

Principal ownership PATCH still ignores full/partial maps and null removal;
fresh read plus PUT remains necessary. Group full PUT retains ownership but
clears omitted public maps, so hydration is required. Key PATCH still ignores
ownership and clears an omitted expiration. Imported keys cannot acquire an
ownership map through the tested create/PATCH sequence. Large integer
rounding in principal/group metadata still reproduces.

Principal type changes now receive an explicit immutable-field error when
the replacement type is non-null; null still succeeds without clearing the
existing value. Keep the create-only policy. Identity detail/delete still
accept another existing principal's path, so complete scoped collection
proofs remain necessary before mutation.

Auth-server probes reproduce duplicate claim names, inflated claim/client
totals, labels-only client PATCH resetting Basic to post authentication,
interactive grants rejected, and two total secret slots even when disabled.
Scope deletion still detaches claim/client associations. Auth-server/client
responses do not expose an extra `managed_by` property supplied at creation;
labels remain the verified ownership field (Q34).

Ordinary groups accept user/agent/unclassified members and reject nested
groups. Unlike the earlier `.com` deployment, this `.tech` organization
rejects team creation and accepts only type group. Do not treat prerelease
deployment as a uniform superset of production.

### Boundaries and cleanup

Native principal deletion with a key, identity and membership succeeds;
the related administrative endpoints then return 404 and the membership
disappears. Group deletion likewise removes administrative access to its
keys. These observations do not prove physical deletion or runtime
revocation. Runtime authentication with a valid imported key and an
administrative bearer still requires the supported mTLS context (Q30).

All disposable records were removed; cleanup responses were 204 or expected
404 after cascade tests. The manual issuer/client/principal/identity remain.
The default directory's public settings are restored, including Vault false;
its update timestamp changed during the temporary toggle. Sanitized evidence
is local under
`/home/rspurgeon/.cache/kong-identity-public-sdk/tech-revalidation/`:
`directory-contracts.json`, plus `auth/`, `followup/` and `capabilities/`
results. A recursive audit verifies secret-bearing fields are redacted.
Full unit tests (`make test`, including race detection) pass after the
documentation/example changes. No implementation or SDK changes were made
during this validation pass.

### Recommended implementation order

1. Add declarative principal active state, including false/true/no-op/dump
   and saved-plan coverage. The current SDK already has optional request
   booleans; this needs client implementation, not a new field in the SDK.
   Keep deployment-aware handling of absent response fields.
2. Finish ordinary groups and direct memberships with independent group
   ownership and explicit collection authority. Defer imported teams and
   nested groups until their supported contracts are clear.
3. Evaluate generated principal/group keys as a bounded lifecycle slice.
   Before writes, repair the preview SDK's missing request `expires_at`
   and verify omitted/value/null encoding. Mutable ownership, adoption,
   imported-key ownership and runtime verification remain constraints.
4. Probe Vault/provider/credential/trusted-IdP CRUD and ownership separately;
   successful Vault enablement clears the entry barrier but does not prove
   safe declarative matching or reconciliation for those child resources.

The full engineering handoff remains the API-only question log. Q1, Q10,
Q27, Q29 and Q33 now distinguish improvements from still-open rollout and
contract questions. Q4/Q5/Q11/Q28/Q31 and auth-server/scoping findings still
have reproducible `.tech` evidence. Automatic default provisioning,
importer coexistence, workspace-bearing identities, recovery guarantees and
the remaining deletion/runtime variants were not settled by this pass.

## 35. Principal active declarative lifecycle

Optional boolean active now supports explicit false/true in creation, diffs,
saved plans, PUT updates, no-op planning, sync and dump. Omission preserves the
current value through the API's documented PUT contract, including adoption
and protection changes. Null is rejected. No default is synthesized locally.

Planning and execution require observable state before an explicit update.
Developer-portal deactivation is rejected according to the specification;
other provisioning actors have not been live-tested. Explicit create/update
readback verifies the requested boolean using presence-preserving GETs.
An unverifiable or ignored value is reported after the remote write, without
automatic retry or rollback. Earlier deployments omitting active still support
omitted declarations and inspection; dump never invents false.

Ownership remains the same five-entry managed_by map. The preview SDK already
has optional request booleans, so this slice changes no SDK files or pin.
Examples demonstrate active true and false. The directory lifecycle scenario
adds active.py, covering false creation, saved activation, omission,
protection/unprotection, adoption, diff, dump, sync and cleanup. When responses
omit the field, it asserts Q10's explicit reconciliation error instead.

Verification on October 31:

- Modernization, formatting, CGO-disabled build and zero-issue lint pass.
- Unit race tests, integration race tests and E2E harness race tests pass.
- SDK patch, installer, replay-policy and 140 metrics tests pass.
- The standalone active lifecycle against the externally selected default
  passes; it deletes only its own generated principal.
- The complete directory/principal scenario passes in US .tech:
  /home/rspurgeon/.cache/kong-identity-public-sdk/active-directory-e2e/20261031-104858.
- All twelve Identity scenarios then ran together: eleven pass, including
  directory/principal lifecycle; group inspection fails in fixture setup
  because this environment rejects team creation. This is the Q32 deployment
  difference, not a principal-active regression. Follow-up test work must
  retain ordinary-group coverage and verify team availability explicitly.
  Evidence:
  /home/rspurgeon/.cache/kong-identity-public-sdk/active-all-e2e/20261031-105016.

The user authorized removing all .tech fixtures for validation. The prior
orders demo issuer/client/principal/identity and default directory were
removed to free the one-directory quota; old demo IDs and captured OAuth
secrets are no longer valid. The normal unmanaged default settings were
restored after final scenario validation; see section 36. Existing local secret files were not read
or deleted. #1415 remains In Progress until merge to main.

## 36. Final .tech regression run and default restoration

All twelve Identity scenarios pass together after the group inspection fixture
was updated to account for Q32's exact team-unavailable response. The fixture
records team_available=false and uses an explicitly identified ordinary
alternate group for the same pagination, membership and parent-scope checks.
When team creation succeeds, the team branch is still exercised. Other
errors fail the scenario. This changes test fixtures, not the API type or
product write behavior.

Evidence:
`/home/rspurgeon/.cache/kong-identity-public-sdk/active-all-e2e-final/20261031-105721`.
The combined run covers all existing auth-server, client/secret, scope/claim,
directory/principal, identity, key inspection, numeric and group/membership
inspection scenarios. The group-specific corrected run also passes:
`/home/rspurgeon/.cache/kong-identity-public-sdk/active-group-e2e-final/20261031-105603`.
Replay eligibility and recordings are unchanged. Unit tests pass again after
the scenario changes.

Fresh inventories after cleanup show zero directories and auth servers. A new
unmanaged directory named default was then created with the standard observed
new-organization settings and verified by GET: unrestricted control planes,
empty allowlist/description/labels/managed_by, both TTLs 600 and Vault false.
Its ID is `01a192b1-d9df-7c25-a491-1366f78e5353`. Principal/group collections
are empty and no test auth servers remain. All seven referenced test credential
capture paths are absent after teardown. The original orders-demo resources
and IDs are retired; generate fresh plans and a fresh secret-output directory
when recreating that demonstration. Existing local user capture files were
not read or removed.

Principal active support is committed as 6dc67da5. The SDK pin remains
bf1d470c108c, with no SDK modifications needed for this field. The next
implementation slice is ordinary groups/direct memberships with verified
ownership, saved references, omission/protection/adoption, descendant checks,
examples and full declarative lifecycle scenarios. Team/importer behavior,
key mutable ownership, and runtime mTLS prerequisites remain bounded by the
API-only questions. The overall epic work is still active; no issue is closed
before main merge and no PR has been opened.

## 37. Ordinary group declarative lifecycle

Ordinary groups now support plan/diff, saved apply, scoped sync/delete,
external name/UUID lookup, adoption, dump/replan, explain and scaffold.
Names are required ASCII alphanumeric values and are unique within the
selected directory. Four canonical managed_by entries store tool/version,
namespace and protection; refs remain local. Ownership is independent of
directory ownership/protection, including under external directories.

Full PUT hydrates omitted description, labels and runtime metadata, overlays
only planned fields, and verifies retained ownership/public values.
Explicit empty maps/description clear fields. Group metadata uses the existing
Q31 integer guard in loading and saved execution. Name changes create a new
identity, following name-matched resource behavior; ordinary apply preserves
the old group and explicit scoped sync may remove it only when safe.

The implemented mutation boundary is directly provisioned ordinary groups
with observable konnect_user provenance and no team-manager state. The
schema's omitted/null group type means ordinary group; inspection preserves
field presence. Team/importer writes remain deferred (Q32). Duplicate
directory/name matches, duplicate declarations across directory aliases,
unowned/foreign targets and changed saved-plan identity fail before mutation.
External-only group declarations never widen pruning scope.

Group deletion observes complete direct membership collections and requires
an empty, terminal unfiltered key page. Any key, failed/malformed observation
or unsupported key endpoint blocks deletion. No credential is rotated or
removed to make group deletion succeed. Principal plan/delete/sync also
observes incoming direct group entries and refuses implicit membership
removal, including foreign/protected/unowned groups. This adds group and
membership read requirements for principal deletion. Explicit membership
detachment dependencies belong to the next #2658 slice; no membership
declarative mutation is claimed here.

Native US .tech probes verify that full group PUT retains its four ownership
entries, an existing membership entry and the observed key record. Directory
DELETE with force=false rejects a directory containing a group with 409.
Evidence is local at group-write-contracts.json and
group-directory-delete-contract.json under
/home/rspurgeon/.cache/kong-identity-public-sdk. Runtime key continuity is
unverified without Q30's mTLS setup, and client checks do not provide Q24's
atomic conditional deletion.

SDK pin remains bf1d470c108c. Existing group write models suffice. The new
group-key emptiness check omits pagination arguments; it adds no SDK patch.
Before full group-key pagination, verify/fix the still-flat listGroupKeys
page tags; the six prior cursor corrections do not cover that seventh route.
Credential writes also need the missing expires_at request model and
omitted/value/null serialization already tracked in #1417/#2659.

Verification:

- Modernization, formatting, CGO-disabled build and zero-issue lint pass.
- Unit/integration/harness race tests pass, alongside SDK patch, installer,
  replay-policy and 140 metrics checks.
- The new 43-command declarative-groups scenario passes in US .tech,
  including a new protected parent, all public fields, saved creation,
  diff/update, clearing/omission, dump/apply/no-op, scoped external lookup,
  adoption, protection, membership guards and namespace-scoped deletion.
- All thirteen Identity scenarios pass together:
  /home/rspurgeon/.cache/kong-identity-public-sdk/group-all-identity-live/20261031-121823.
- Early stops after clearing labels, attaching a membership, and protecting
  a group all pass automatic teardown. Evidence is under group-stop-cleared,
  group-stop-membership and group-stop-protected in the same cache root.
- A read-only plan of docs/examples/declarative/identity/groups.yaml against
  restored default produces exactly two group creations and no directory
  mutations. The example was not applied.

Fixture cleanup now has an explicit namespace allowlist for lifecycle groups
under a freshly generated directory/name prefix. It accepts canonical owned
metadata, protected fixtures and deliberately cleared public labels, or
explicitly marked unowned adoption fixtures. Unknown/imported/foreign shapes
and changing ownership/membership stop cleanup. This credential-free fixture
authority does not expand product cascading deletion permissions. Replay
eligibility and recordings are unchanged.

Final inventories show no test auth servers/principals/groups. The unmanaged
default directory was restored with its saved standard settings; its ID is
`01a192fe-6115-7aac-96a9-731d5f02cf51`. All seven referenced test capture paths
are absent after teardown. Existing user capture files were untouched.

The overall epic goal remains active. #2657/#1415 stay In Progress until main
merge; #2658's direct membership declarations are next. API-only questions,
metadata storage and compatibility boundaries are updated. No PR is opened.

## 38. Direct principal membership declarative lifecycle

Direct memberships now support nested groups[].memberships and grouped
identity.memberships declarations. The API's required member.type is explicit
and limited to the verified principal variant. member.id accepts a principal
UUID or typed principal reference. A complete directory observation proves
literal/resolved membership targets; unresolved same-plan targets must declare
the same directory. Group/principal creation dependencies survive saved plans.

Membership entries have no metadata store. Their namespace and protection come
from a managed ordinary group, independently of directory protection or
principal ownership. External groups grant no mutation authority. A local ref is
not persisted; the group's identity plus principal ID matches a relationship,
while the generated entry ID routes GET/DELETE. Refs can change without
replacing the relationship. A changed principal adds a different relationship;
apply preserves undeclared members and explicit collection sync removes
omissions. Sync retains the existing parent-group collection scope, documented
in the tutorial; membership-only intent must not be mistaken for ignoring
declared group scope.

Omitted collections preserve links. Explicit empty collections select direct
principal entries for pruning. Protected groups permit new members under the
existing protected-parent creation pattern, but removal requires a separate
unprotection. Simultaneous unprotection does not authorize removal. Saved
execution checks fresh group name, namespace, provisioning/protection and
membership identity. POST is single-attempt through both retry layers and
verifies scoped readback without rollback or blind retry.

Final cross-namespace plans require explicitly authorized membership deletions
before deleting a group or principal. Unplanned links, including links from
foreign/imported groups, block parent deletion. Executors still require fresh
empty observations; any group key blocks group deletion. A disappearing-group
404 is accepted only after fresh scoped proof that the group is absent. These
checks do not supply Q24's atomic server-side precondition.

Dump exports direct principal entries under managed ordinary groups with entry
IDs as local refs and typed references to included principals. Other targets
retain UUIDs. A restore-planning regression verifies fresh principal IDs are
bound when recreating managed resources. Unowned/team/importer groups stay
external; unsupported nested membership blocks managed collection export instead
of silently exporting a partial authoritative list. Adoption applies to the
owning group and preserves links; there is no independent membership adoption
command. Teams, nested-group writes, inherited relationships and batch
operations remain outside the verified slice (Q32).

The selective public preview SDK remains bf1d470c108c. Existing group/member
write models and cursor fixes suffice; no SDK patch or module pin changed.
Remaining credential write-model/pagination issues and runtime mTLS setup are
unchanged. Examples now include memberships.yaml and memberships.md, centered on
external lookup of default with separate directories.yaml bootstrap guidance.
The metadata ledger and compatibility/removal notes record inherited
relationship authority and unsupported boundaries.

The new 37-command US .tech scenario passes, including saved-delete replay
after its directory is removed. The final combined run passes all fourteen
Identity scenarios:
/home/rspurgeon/.cache/kong-identity-public-sdk/membership-all-identity-final/20261031-130426.
The final standalone lifecycle also passes at membership-live-final/20261031-125957
under the same cache root.
It verifies new directory/group/principal dependencies, all writable member
fields, saved creation, idempotency, diff, add-only application, sync pruning,
omitted/empty collections, dump/replan/apply, protection, group adoption with
existing links, stale-owner rejection, and explicit child-before-parent
deletion. The initial run reached final teardown but used an unresolved test
binary placeholder; correcting it to the harness's .bin and e2e profile fixed
the test invocation. No API defect was inferred from that harness failure.

Modernization, formatting, CGO-disabled build and zero-issue lint pass. Full
unit/integration/harness race tests pass, alongside SDK patch, installer,
replay-policy and 140 metrics checks. Enabled replay coverage and cassettes
are unchanged. Deliberate stops after created members, group protection and
foreign group ownership all pass automatic teardown. Their evidence is under
membership-stop-created/20261031-130906,
membership-stop-protected/20261031-130915 and
membership-stop-foreign/20261031-130933 in the same cache root.

Fresh direct API inventories confirm an unmanaged default directory with its
saved public settings, zero principals/groups and zero auth servers. Its ID
is 01a19327-5a26-753e-bb9e-05957807805b. Evidence is saved in
membership-final-inventory.json. All seven referenced private capture paths
from the full run are absent after teardown. Existing user capture files
were not read or removed. A read-only plan of memberships.yaml creates exactly
one principal, group and membership, with zero directory mutations; it was
not applied.

No additional API behavior question was found in this membership slice.
Q24/Q32 still bound atomic deletion, nested groups and importer/team writes.
Future credential work must verify its own ownership/secret contracts and
SDK models; membership success does not establish credential behavior.
#2658/#2657/#1415 remain open/In Progress until merge to main. No PR is opened
and the overall epic goal remains active.

## 39. Basic-auth contract verification and SDK preparation

Direct US .tech probes establish a feasible basic-auth username-record slice.
Create/list/GET/full PUT work for user, agent and unclassified principals.
Creation retains supplied management metadata. PATCH ignores management-map
changes/removal; full PUT retains the supplied map and clears omitted maps.
Username conflicts return 409 under the same or a different principal in the
directory. Full PUT can rename a username while keeping existing password
IDs administratively readable. Runtime authentication continuity is still
unverified without Q30's supported context.

Generated/imported password creation retains management metadata and explicit
expiration. GET/list do not return secret values. Password PATCH ignores
management-map changes/removal and clears omitted expiration. Explicit null
and future expiration work. Password PUT requires both secret and expires_at;
omitting either returns 400. Supplying both permits replacement and metadata
updates. This is not a metadata-only preservation operation. Two total
password slots include expired records; a third create returns 403 even after
expiring one record. Q35 asks for ownership updates that preserve secret and
expiration. Declarative password enrollment needs an explicit safe secret
policy; metadata-only adoption is unavailable.

Wrong-principal basic-auth GET/DELETE and wrong-principal/wrong-basic-auth
password GET return 404. Principal DELETE succeeds with attached records and
passwords, and basic-auth DELETE succeeds with passwords; descendant reads
then return 404. These are administrative cascade observations, not proof of
runtime revocation. Principal/credential deletion needs explicit empty or
authorized child-deletion checks, without inherited credential authority.

Native bracketed page[size]=1 requests traverse two pages for both basic-auth
and password collections. Flat size=1 is ignored and returns page size 20.
Existing SDK request tags for both lists are still flat. The expanded SDK wire
regression fails exactly those two methods while the prior six lists pass.
The patch bundle now enrolls those two request-tag corrections and preserves
all prior patches. Root unit tests and patch-tool tests pass before the SDK
snapshot is prepared. No resource API inventory or model fields change in
this pagination correction. Password write models separately omit expires_at;
that issue must be fixed before any SDK password writer relies on it.

Sanitized native evidence is local under
/home/rspurgeon/.cache/kong-identity-public-sdk:
basic-auth-contracts.json, basic-auth-boundaries.json and
basic-auth-pagination.json. All temporary password/basic-auth/principal
fixtures were deleted. The unmanaged default directory was not changed.
Generated and supplied secret material stayed in process memory; persisted
secret fields are redacted. No original user capture was read or altered.

#1418 is open/In Progress on Project 79. Next: prepare/push the SDK correction,
verify its wire/full tests and consume the new pin, then enroll verified
username-record lifecycle, safe password inspection and credential deletion
guards. No password declarative lifecycle or new CLI is claimed by this
preparation step. Existing Q24/Q30 and new API-only Q35 record the boundaries.
No PR is opened; the overall epic goal remains active.

## 40. Basic-auth pagination SDK preview

KongCTL patch source d15373927f432be4ab7da870b398c54c1d972717 records the
basic-auth/password cursor corrections and Q35's direct API evidence. SDK
995778a1c448a0f4290de804f3facb9edb5b4fb4 is pushed to
kongctl/kong-identity-public-preview. The generated code delta is exactly two
request tags: ListPrincipalBasicAuths and ListPasswords now use deepObject
for page. No operations, response models, write fields or unrelated internal
attributes change. All prior KongCTL patches remain in the snapshot, with
source provenance tied to d1537392.

The wire regression fails both new list cases before correction and passes
all eight Identity list cases afterward, including encoded cursor values.
SDK root/model/internal/pkg tests and internal race tests pass. The first
SDK make test.unit invocation used CGO disabled, which cannot run race tests;
it was rerun successfully with CGO enabled for testing. Application builds
remain CGO-disabled. Patch-tool tests and KongCTL unit tests pass for the
source change. The consumer replacement is updated to
v0.79.1-0.20261031183454-995778a1c448 and builds successfully.

Consumer modernization/format/build and zero-issue lint pass. Unit,
integration and harness race tests pass, alongside SDK patch, installer,
replay-policy and 140 metrics checks. All fourteen Identity live scenarios
pass together on the new SDK:
/home/rspurgeon/.cache/kong-identity-public-sdk/basic-auth-sdk-all-identity/20261031-134030.
Fresh direct inventories confirm empty principal/group/auth-server
collections and the restored unmanaged default directory
01a19347-410c-71fe-aac5-339b0c5c66c1. All seven referenced private capture
paths are absent after teardown. Saved public directory settings are retained;
existing user capture files were not read or changed. Evidence is local in
basic-auth-sdk-final-inventory.json. Replay eligibility/recordings are unchanged.

Basic-auth username declarative lifecycle, password metadata inspection and
credential parent-deletion guards are the next implementation slice.
Password write models still lack expires_at; no password writer is enabled
by this query-only correction. Runtime continuity remains unverified under
Q30, and metadata-only password updates remain Q35. No PR or SDK release is
created; the overall epic goal stays active.

## 41. Basic-auth inspection and username-record write foundations

The branch adds scoped `get|list [konnect] identity principal-basic-auths`
and `principal-basic-auth-passwords`, with singular forms for detail. The
parent directory remains explicit, normally `--directory-name default`;
principals require UUIDs because their display names are not unique.
Basic-auth ID arguments are UUIDs, while `--username` always selects the
exact username, including UUID-shaped usernames. Password commands select a
username-record parent with either `--basic-auth-id` or `--username`.

Dedicated response projections contain only public username/password
metadata. They never serialize SDK credential items or propagate raw
credential error bodies. Both detail and list support JSON, YAML and text
tables. Complete cursor traversal validates raw data/page presence and
rejects repeated IDs/cursors, malformed observations and failed pages without
returning partial results. Existing SDK 995778a1c448 supplies the bracketed
page query contract; no new SDK revision or model change was needed here.

Shared username-record helpers now provide single-attempt creation, full
PUT and fresh readback verification of ID/username/management metadata.
The request projection contains no password fields. Metadata is not stamped
by these helpers: ownership decisions and fresh omitted-field hydration
belong to the future declarative callers. Deletion requires a complete empty
password collection and never removes password slots automatically. Failure
after a write reports the uncertainty without retry or rollback; a client
emptiness check cannot prevent concurrent attachments atomically.

The new live `identity/basic-auth` scenario passes in US `.tech`. It exercises
user, agent and unclassified principals, five username records, five password
records including generated/imported material and expiring/nonexpiring
entries, exact username/ID lookup, all output formats, explicit product/list
forms and wrong-principal/wrong-credential 404s. Request traces prove actual
page-size-one traversal for both collections. The scenario scans outputs/logs
for issued material and its base64 encoding. Captures are private and outside
artifacts. Fixture teardown verifies all descendants before removing
passwords, username records and principals; unexpected leftovers are not
marked for the harness's generic principal cascade cleanup.

Final live evidence:
/home/rspurgeon/.cache/kong-identity-public-sdk/basic-auth-inspection-final/20261031-140226.
An earlier full run also passed. A deliberate stop immediately after
directory creation verified directory teardown. Fresh API inventory confirms
empty principal/group/auth-server collections and restored unmanaged default
01a19357-bd21-7cd8-9e5c-f18b5261f847. Both full runs' private capture paths
are absent after teardown. Public directory settings match the saved baseline;
existing user capture files were not accessed. Final inventory is saved in
basic-auth-foundation-final-inventory.json under the same cache directory.

Modernization, formatting, CGO-disabled build, zero-issue lint, full unit race
tests, integration race tests, SDK patch tests, installer tests, E2E harness
race tests, all 140 metrics tests and replay-input validation pass. Integration
and auxiliary check logs are basic-auth-foundation-integration.log and
basic-auth-foundation-other-gates.log in that cache. The pre-commit executable
is not installed in this environment. Replay membership/cassettes are
unchanged. Examples are documented in
docs/examples/declarative/identity/basic-auth.md; compatibility restrictions
and the metadata storage ledger are updated. No new API discrepancy was
found; Q35 still records the password metadata-update question.

This is a foundation, not completed #1418 acceptance. Declarative resource
registration, namespace/ref identity policy, planning/execution/adoption/dump,
full lifecycle examples/E2E and principal credential-deletion guards remain
the next implementation work. Password request models still need expires_at
and an explicit secret-source/capture/rotation policy; metadata-only adoption
has no verified preserving request. No imperative credential mutations,
declarative credential ownership or runtime authentication guarantee is
introduced here. Keep #1418 open/In Progress until main merge; no PR is opened
and the overall epic goal stays active.

## 42. Declarative basic-auth usernames and credential-aware deletion

Basic-auth username records now support the standard declarative lifecycle:
nested principals[].basic_auths or grouped identity.basic_auths, saved
plan/apply/delete/sync, diff, protection, adoption, external UUID/username
lookup, dump, explain and scaffold. Every record requires an explicit username
and local ref. Exact username identity is unique within a directory; refs
remain local. Four canonical managed_by entries store namespace/protection,
independently of the directory/principal. Client-side complete collection
scans and duplicate checks are used; no server filtering is assumed.

Metadata updates use the verified fresh full-PUT/readback path without sending
password material. Changing a username declares another record, rather than
implicitly renaming/moving a credential. Apply preserves the old record;
explicit scoped sync can prune it only when unprotected and password-free.
Omitted collections and external-only selections do not authorize pruning;
foreign/unowned records are retained. Saved execution rejects changed scope,
username, ownership or protection, and rechecks descendants immediately before
deletion. No write metadata is stored in public labels or runtime metadata.

Any password, including an expired one, blocks username deletion. Principal
delete/sync requires explicitly authorized identity, membership and username
deletions; any API key blocks it. Final merged plans order cross-namespace
child deletes before their principal. Execution requires fresh empty
credential observations, so later attachments cannot silently cascade through
a saved plan. Errors/malformed responses block deletion; a legacy user-key
404 is not an empty collection. These client checks cannot eliminate the
attachment race without an atomic backend precondition (Q24).

Dump translates owned username metadata into kongctl and uses UUIDs as local
refs; unowned usernames stay external. Passwords are excluded. A related
common-path defect was verified directly: dumping an unmanaged default
directory produced a declaration requiring adoption on reload. Unmanaged
directory export now uses an external exact-name selector, retaining managed
children without taking over the directory. Claimed but invalid directory
ownership fails validation. This is a client export fix, not an API defect.

The live identity/declarative-basic-auth scenario passes with same-plan
protected directory/principal creation, independent child namespaces, local
ref-only changes, protection/unprotection, adoption, existing-password and
expiration preservation, external username lookup, managed/unmanaged-directory
dump/replan, omitted/empty sync, foreign/unowned retention, stale ownership
and password guards, API-key/principal deletion guards, mixed grouped/nested
declarations, cross-namespace child-before-parent deletion and repeated saved
deletion after the parent is absent. Native password/key fixtures use private
capture and explicit verified retirement. Secret/base64 artifact scans pass;
these administrative observations do not establish runtime authentication
continuity (Q30).

Final complete live regression: all sixteen Identity scenarios pass together
in 276.946 seconds. Artifacts are under
/home/rspurgeon/.cache/kong-identity-public-sdk/,
basic-auth-declarative-final-identity/20261031-143225.
The standalone expanded lifecycle also passes at
basic-auth-declarative-complete/20261031-143036 in that cache.

The documented basic-auth.yaml example was separately applied against the
unmanaged default directory: two resources created, four username and five
principal metadata entries verified, normal replan and dump/replan both
unchanged, then both resources deleted through the CLI. The default record
was exactly unchanged. Evidence is basic-auth-example-demo/summary.json and
its public command artifacts in the same cache. Existing user capture files
were not accessed.

Modernization, formatting, CGO-disabled build, zero-issue lint, full unit race
tests, integration race tests, SDK patch tests, installer checks, E2E harness
race tests and all 140 metrics tests pass. Final check log:
basic-auth-declarative-final-gates-v2.log. The replay check target passes;
replay membership and cassettes are unchanged. The pre-commit executable is
not installed. Fresh direct API inventory proves empty principal/group/auth
server collections and restored unmanaged default
01a19377-2f73-711b-a3a7-bad75ffe231a, matching saved public settings. All seven
referenced private capture directories are absent. Final evidence is
basic-auth-declarative-final-inventory.json in that cache.

No SDK revision was needed: preview 995778a1c448 already supplies username
write models and the corrected cursor requests. Password request models still
lack expires_at, and safe secret-source/capture/rotation policy is separate
work. Metadata-only password adoption has no verified preserving request
(Q35). No declarative passwords or imperative password mutations are claimed.
Examples, resource reference, implementation guide, compatibility tracker and
metadata ledger are updated. Keep #1418/#1415/#1414/#1132 open/In Progress
until main merge; no PR is opened. The broader epic goal stays active.

## 43. Credential expiration SDK correction

SDK preview `79bc3dbbb928b1129be0687d6fa8acd37c560ba1` is pushed to
`kongctl/kong-identity-public-preview`. KongCTL patch source is
`190e5b5716c951fb13a1d1126ef38a0b9d8d5b70`, also pushed to `ki-triage`.
The consumer replacement is
`v0.79.1-0.20261031201742-79bc3dbbb928`. No release or PR is created.

Canonical credential requests reference writable nullable ExpiresAt from
common/properties.yaml. The combined specification instead selected the
same-named read-only IAM timestamp from common/identity.yaml. Generation
therefore omitted expiration, including password PUT's required property.
Q36 records that composition issue for engineering, separately from backend
PATCH ownership and omission behavior (Q28/Q35).

Preview preparation now adds one dedicated writable nullable schema and
retargets exactly six request schemas: principal/group generated/imported
API-key create, key PATCH, password create/PATCH and password PUT. Five
optional fields use the SDK's existing OptionalNullable date-time wrapper;
unset/date/null remain distinct. Required password PUT uses a nullable time
pointer and always sends the field. No global nullable-wrapper switch or
existing public field type changes are introduced. Models, examples and wire
regressions are maintained in the source patch bundle.

The 30-case HTTP wire regression has 21 failures before schema correction;
schema correction alone leaves nine null failures. Full correction passes
all cases. Two preparation tests verify the correction's scope, idempotence
and drift rejection. An AST audit finds no removed or changed existing
exported members/signatures across 39,350 declarations, with 14 added methods.
The operation inventory stays 1,102 (990 baseline plus 112 Identity additions).
The SDK audit confirms
zero unrelated schema/operation changes and unchanged global configuration.
Verified formatting-only regeneration changes were removed from 385 unrelated
files. Existing KongCTL patches and upstream persistent edits are retained.

Full pinned generation succeeds, with zero specification lint errors and
255 warnings. A missing linked-worktree pristine snapshot was
recovered into the shared Git object store rather than disabling persistent
edits. The current snapshot ref is also pushed, so future regeneration can
fetch it. SDK root/model/internal/pkg tests and the internal race suite pass.

KongCTL compiles against the new pin without application-code changes.
Modernization, formatting, CGO-disabled build and make test-all pass, including
lint, unit/integration/harness race tests, installer checks, nine patch-tool
tests and 140 metrics tests. The default replay cassette check passes; replay
membership and cassettes are unchanged. All sixteen Identity live scenarios
pass together in 273.613 seconds. Their artifacts are under
credential-expiration-all-identity/20261031-151929 in the evidence cache.
Fresh direct API inventory confirms empty principal/group/auth-server
collections and restored unmanaged default
01a193a2-4bfe-733a-adb3-e71309787a37, with its saved public settings. All seven
referenced private capture directories are absent. Final evidence is
credential-expiration-final-inventory.json and the consumer gate log is
credential-expiration-consumer-gates.log. The pre-commit executable remains
unavailable; the explicit repository checks above were run.

Native SDK requests pass 84 US `.tech` checks: generated/imported key/password
creation with omitted/date/null expiration; principal/group key and password
PATCH with those states; password PUT with a timestamp or null. Fresh raw GETs
verify each resulting expiration. Omitted PATCH still clears expiration; the
SDK faithfully represents that request rather than fixing backend semantics.
All disposable credentials, username records, principals and groups are
removed. Generated/imported secret values stay in memory and are not logged.
The check uses explicit `.tech` transport routing because these SDK operations
have per-operation server URLs. It establishes administrative behavior only;
runtime authentication/continuity still needs Q30's integration context.

Evidence under /home/rspurgeon/.cache/kong-identity-public-sdk/:

- credential-expiration-wire-before.log
- credential-expiration-wire-schema-only.log
- credential-expiration-wire-after.log
- credential-expiration-preparation-tests.log
- credential-expiration-go-api-audit.json
- credential-expiration-native-sdk.json
- credential-expiration-generate-final.log
- credential-expiration-sdk-final-tests.log
- credential-expiration-sdk-final-race.log

The SDK contains the scoped audit and generation provenance under
identity-preview/.

This removes the SDK prerequisite for further credential work. It does not
enable a product writer or change metadata storage. Imported keys still cannot
acquire verified ownership; key/password metadata PATCH is ignored; password
PUT requires secret material. Explicit secret-source/capture/rotation policy,
declarative feasibility and lifecycle examples/E2E remain the next work under
#1417/#1418/#2659. Keep those issues and SDK issues #1413/#1399 In Progress
until main merge. The broader epic goal stays active.

## 44. Trusted IdP contract evaluation

Issue #2664 is In Progress. Direct US `.tech` probes establish deterministic
issuer URL matching within a directory: duplicate issuer create and update
return 409. Multiple issuers can use the same display name. Create/get/list,
PATCH/full PUT and delete work; no labels or managed_by store is available.
Inherited ownership under an explicitly managed directory follows the existing
auth-server scope/claim pattern and is being evaluated for this family.

Creation validates reachable signing keys through issuer discovery or an
explicit JWKS override. A temporary auth server provides both for isolated
fixtures; example.test placeholder discovery correctly fails validation.
False/true JIT provisioning and Vault access settings persist. Full PUT
resets omitted display name, JWKS override and JIT mapping, and restores
documented boolean defaults. A replacement mapping drops omitted mapping
fields; empty mapping/metadata work. PATCH with jit alone empty preserves the
old mapping; null jit is rejected. A fresh full representation is therefore
available for omitted-field preservation and explicit mapping removal.

PATCH display_name null clears it. PATCH jwks_uri null instead retains the
previous override despite success; PUT omission clears it. API-only Q38 asks
for that nullable PATCH contract. The observed jwks_uri distinguishes a stored
override from discovery: issuer-only creation and clearing PUT return null.

An ordinary principal with an OIDC identity whose issuer matches the IdP
appears in its associated-principal collection. Deleting the IdP leaves that
principal and its identity readable. Trust configuration does not imply
ownership of these principals. Runtime provisioning/revocation is not tested.

Vault must be enabled for IdP reads/writes. Disabling it while IdPs exist
succeeds and hides their list/detail behind 403. Directory DELETE with
force=false still succeeds with an attached IdP, with Vault either enabled
or disabled. The same request rejects a directory containing a principal or
group with 409 and preserves the child. API-only Q37 asks which dependencies
must block deletion and how hidden records can be inspected without mutation.
The choice of directory-deletion guard is under review before enrolling IdP
declarative writes. No IdP application code is claimed at this point.

Native bracketed page-size-one traversal visits both IdPs. Flat size=1 is
ignored and returns size 20. The preview SDK has the same CursorPageQuery
serialization gap as Q22 on ListTrustedIdps and ListTrustedIdpPrincipals.
The expanded wire regression fails exactly these two methods before targeted
request-tag corrections and passes all ten list methods afterward. Patch
source is being prepared; no request/response fields or operation inventory
change is needed.

All disposable auth servers, IdPs, principals and identities were removed.
The default directory was recreated during the explicit parent-deletion
probes and restored with its original public settings and empty ownership
maps. Evidence under /home/rspurgeon/.cache/kong-identity-public-sdk/:

- trusted-idp-contracts.json
- trusted-idp-followup.json
- directory-child-delete-contracts.json
- trusted-idp-cursor-wire-before.log
- trusted-idp-cursor-wire-after.log

The broader epic goal stays active; no PR is opened.

## 45. Trusted IdP inspection and declarative principal associations

Preview SDK `30c3834c9846636c6866e818f8085b46c237dc23` is pushed to
kongctl/kong-identity-public-preview from patch source
280bdeccb5e107286904184d84484e05efcc3e38. The consumer replacement is
v0.79.1-0.20261031204931-30c3834c9846. The only generated-code changes are
Page query tags in ListTrustedIdps and ListTrustedIdpPrincipals. All previous
patches, including credential expiration, are retained. Schemas, response
models, global generator settings, module dependencies and the 1,102-operation
inventory are unchanged. Generation, SDK tests/race checks and the two
expiration preparation tests pass. The expanded actual-wire regression fails
exactly these two new methods before correction and passes all ten afterward.

KongCTL get/list now support identity trusted-idp[s] by UUID or exact
--issuer-url, and identity trusted-idp-principals by parent UUID or issuer.
Both require an explicit directory name/ID. Display names never select an
IdP. New public projections preserve false values and null JWKS/display names,
expose all JIT mapping fields, and reject missing required configuration
instead of synthesizing SDK defaults. Complete cursors, unique/valid IDs,
pagination metadata and fresh issuer lookup are checked. Failed or incomplete
observations produce no partial inventory. Errors suppress raw response
bodies. Text displays the full issuer URL; JSON/YAML expose public fields.

The read commands do not stamp ownership, provision principals or exercise
runtime authentication. The associated-principal collection includes directly
managed OIDC associations, not only JIT-provisioned records. The new
trusted-idps.md tutorial and trusted-idp-principals.yaml example use an
externally referenced default directory and an already configured issuer.
Principal and identity lifecycle retain their existing independent managed_by
metadata. Trust configuration declarations and adoption remain pending the
ownership/deletion policy, including the Q37 visibility problem. Production
directory deletion behavior is unchanged by this inspection slice.

The new identity/trusted-idps live scenario creates three native IdPs using
a disposable auth server for discovery/public signing keys, then declaratively
creates two principals and their OIDC associations under an external directory.
It verifies all public fields, shared display names, exact issuer lookup,
default/explicit product paths, all output formats, empty/wrong scope cases,
three IdP pages and two associated-principal pages. Reads leave full native
configuration unchanged. A final disabled-Vault read fails explicitly.
Principals/identities are deleted through the declarative CLI; each verified
IdP is removed explicitly before parent teardown. No credential values are
returned or captured by this scenario; the issuer exposes public signing keys.

Fixture cleanup opts into requireEmptyTrustedIdPs: true. It observes an empty
IdP collection before any cleanup writes and again before directory deletion;
it never authorizes deleting remaining IdPs. Other scenarios gain no new Vault
permissions. Missing/nonempty/error cases are covered by harness tests.
A first fixture-format failure left three IdPs and the guard refused parent
cleanup; only the exact fixture issuers were removed before restoring default.
The corrected complete scenario passes in 12.761 seconds. A second attempt
identified that automatic text columns hid the unique issuer; fixed text
output and a regression now keep it visible. Those fixture/UI findings are
separate from API-only Q37/Q38.

Evidence under /home/rspurgeon/.cache/kong-identity-public-sdk/ includes:

- trusted-idp-cursor-wire-before.log and trusted-idp-cursor-wire-after.log
- trusted-idp-cursor-generate.log and trusted-idp-cursor-sdk-tests.log
- trusted-idp-cursor-sdk-race.log
- trusted-idp-inspection-v3/20261031-160719
- trusted-idp-before-all-inventory.json

All seventeen Identity live scenarios pass together on this SDK pin in
275.796 seconds. Artifacts are trusted-idp-all-identity/20261031-160943.
An additional complete repeat passes, and a verified stop immediately after
create-issuer runs only the two creation commands and passes cleanup:
trusted-idp-inspection-stopped-v2/20261031-161817. All seven referenced private
capture directories from the full suite are absent. Replay membership and
cassettes are unchanged; the default cassette check passes.

Modernization, formatting, CGO-disabled build and final make test-all pass,
including zero-issue lint, unit/integration/harness race tests, SDK patch and
installer checks, and 140 metrics tests. The final gate log is
trusted-idp-final-gates-v2.log. The documented example separately plans exactly
two creates (principal and identity) with its issuer environment variable;
evidence is trusted-idp-example-plan.json. The pre-commit executable is not
installed.

Fresh direct inventory, including temporarily enabling Vault to inspect its
hidden collections and restoring the saved settings, confirms zero principals,
groups, IdPs, providers, vault keys and auth servers. The final default is
unmanaged with its original public settings:
01a193d3-e99f-7cbe-9475-4502618fa341. Evidence is
trusted-idp-final-inventory.json in the cache above.

SDK #1413/#1399, resource #2664 and directory epic #1132 stay In Progress
until main merge. Declarative IdP configuration/adoption and the production
parent-deletion policy are not delivered by this inspection slice. No PR or
release is created; the broader goal stays active.

## 46. Explicit basic-auth password lifecycle

Create/patch/delete identity principal-basic-auth-password now operate under
an explicitly selected directory, principal and basic-auth record. Parents
resolve by basic-auth UUID or exact username, including UUID-shaped usernames.
Create supports generated material or existing deferred file/environment
sources, with required exclusive 0600 capture. Public output contains only
metadata. The capture records directory/principal/basic-auth/password IDs;
its pending state identifies the parent after an uncertain create.

Expiration-only PATCH sends an explicit timestamp or null. Supplying a source
to patch explicitly requests in-place PUT replacement. Fresh GET hydrates
omitted expiration and the full management map; missing response fields block
the write instead of becoming null/empty defaults. Both operations verify
public read-back. They never stamp ownership or silently rotate material.
PUT takes supplied material once and cannot prove runtime authentication from
public metadata. POST and PUT disable SDK and transport retries; failures do
not trigger another write or implicit rollback. Captured values survive a
later public-field/read-back failure. Whole password-route trace bodies are
suppressed, including arbitrary error details that could echo material.

Selected deletion uses the normal confirmation. Explicit --all deletes every
password under the selected username, including unowned records, and verifies
an empty collection. No omitted declaration or slot limit authorizes this
operation. Expired records still occupy one of two slots. Passwords remain
excluded from declarative ownership, adoption, dump and sync. Their presence
continues to block username deletion; ordinary username plans stay unchanged
after password operations. Q35's metadata-only update problem remains open.

Native US `.tech` mutation probes pass 44 requests: wrong-principal and wrong
username PATCH/PUT/DELETE return 404 and preserve the original record;
wrong-principal delete-all returns 404. Delete-all works on empty/populated
collections, with fresh empty list and 404 details. All fixtures are removed.
The sanitized evidence is password-mutation-contracts.json and
password-mutation-contract-summary.json under the local evidence cache.
Q19 now also asks about password issuance recovery; timeout/503 behavior is
synthetic-client coverage, not an injected live-service failure.

The new identity/passwords scenario passes in 17.76 seconds in US `.tech`.
It exercises generated/imported capture, file/environment sources, exclusive
file/symlink refusal, timestamp/null expiration, preserving PUT, arbitrary
management maps, wrong-parent rejection, expired-slot limits, all output
formats, selected/all deletion, unchanged plans and declarative deletion
guards. It scans normal output/logs/artifacts for material and encoded values,
keeps captures outside artifacts, and explicitly verifies all descendants
before fixture teardown. The unmanaged default directory is restored.
Evidence: password-lifecycle-v1/20261031-164604. This is administrative
coverage; runtime password authentication requires Q30's integration setup.

The new passwords.md tutorial builds on basic-auth.yaml under the external
default directory. It documents creation, private material transfer,
expiration/replacement, overlapping-record rotation and explicit retirement.
The metadata ledger and compatibility tracker record the exact boundaries.
SDK `30c3834c9846` already contains the necessary pagination/expiration
corrections; this slice requires no SDK revision, release or new patch.

Final make test-all passes: zero-issue lint, unit/integration/harness race
tests, nine SDK patch tests, installer checks and 140 metrics tests.
Modernization, stable formatting, CGO-disabled build and the default cassette
check also pass. Existing replay membership/cassettes are unchanged; no full
isolated replay claim is made. Early lint failures were limited to test UUID
constant names flagged as credentials, line wrapping and Go idioms; these
are corrected without disabling the linters. Final evidence is
password-final-gates-v2.log and password-verified-build.log.

All eighteen Identity live scenarios pass together in 304.390 seconds:
password-all-identity/20261031-164719. All eight referenced private capture
directories are absent. A separate stop immediately after create-directory
runs exactly one command and passes cleanup in 0.999 seconds:
password-lifecycle-stopped/20261031-165244. Fresh native inventory includes
temporarily enabling Vault to inspect hidden collections, then restoring the
saved settings. The unmanaged default directory is restored as
01a193f3-6e4a-7992-8223-42ccdb9502ca; principal/group/IdP/provider/vault-key and
auth-server inventories are empty. See password-final-inventory.json and
password-artifact-verification.json in the evidence cache.

Issue #1418 and epic #1132 remain In Progress until main merge. No PR or SDK
release is created; the broader resource implementation goal stays active.

## 47. Token Vault key/provider contracts and SDK preparation

Issues #2660/#2661 are In Progress. Direct US `.tech` probes verify the
credential-provider template catalog, provider configuration and public
encryption keys before enrolling their declarations. The first probe passes
113 requests and removes all fixtures. Bracketed page-size-one requests
traverse eight templates, five providers and five keys. All five RSA/EC keys
match independently computed RFC 7638 SHA-256 thumbprints (RSA-2048 and
P-256/P-384/P-521). Private keys exist only in generator memory; probe fixtures
contain public keys, without OAuth/client/credential secret material.

Providers have directory-unique names: duplicate create and rename conflict
with 409. Static and OAuth configuration create/read/PATCH work; templates
provide effective endpoints and defaults. Static-secret templates reject a
client_id. Template changes can succeed through PUT. GitHub PUT needs the
static client_id even though the request schema does not mark it required.
PATCH accepts nonempty valid scopes and rejects an empty collection. Creating
Datadog without client_id selects its documented DCR/PKCE defaults. No OAuth
consent, DCR registration, secret preservation or runtime flow is exercised.

The 58-request follow-up verifies duplicate public material returns 409 below
the quota, including CRLF PEM. Names are non-unique. PKCS#1 RSA PUBLIC KEY,
RSA-1024 and invalid material return 400; the supported representation in
these tests is PKIX PUBLIC KEY. PATCH changes the key name; a supplied new
public_key is ignored rather than replacing immutable material. A sixth key
returns quota-exceeded 403. Quota validation precedes duplicate checks when
already full, so a full-quota response is not uniqueness evidence.

Custom OAuth DCR provider PUT fails with 400 even when the original creation
fields are supplied: it reports a move to a template without a registration
endpoint. ReplaceVaultProviderBody omits both registration_endpoint and
dcr_enabled. Q39 records the direct request, preserving GET and clearing-scope
question. A separate 12-request probe confirms POST enabled:false still
creates an enabled provider, explicit PATCH false works, PUT omission retains
false, and explicit PUT true sets true. The same omitted PUT resets refresh
buffer to zero. Q40 asks for initial-disable and omission semantics.

Four isolated parent-delete tests show directory force=false removes a
directory with one Vault key or one empty-credential provider, with Vault
either enabled or disabled. The old child then returns 404. Disabling Vault
hides key/provider list/detail behind 403, just as it hides Trusted IdPs.
Q37 now includes these families. This defeats treating force=false as a
universal child-preservation guard. Public matching alone does not confer
namespace/protection or deletion ownership; no independent metadata store
exists on either family. Declarative enrollment needs that explicit policy.
The probe restores the unmanaged default directory and saved public settings;
the final restored ID is 01a193f8-bb9f-7a2d-9bef-792f5c170696.

Native evidence is vault-contracts.json, vault-contract-summary.json,
vault-followup-contracts.json, vault-followup-summary.json and
vault-provider-enabled-contracts.json under the local evidence cache. Q22 now
also records the three list operations' flat SDK Page tags. The expanded
actual-wire regression passes the previous ten list operations and fails
exactly Vault keys/providers/templates before correction. Targeted tags and
regression coverage are added to the SDK patch source, with no model/schema
or unrelated request changes. SDK generation/integration remains the next
step; no new product command or declaration is claimed by this preparation.

## 48. Token Vault cursor SDK preview integration

SDK 2597fe249e2602e37377758a4fce8c9da0a79a9b is pushed to
kongctl/kong-identity-public-preview, from patch source
bd2ee64085b0b02eeaca800780d9d3d3b3837cdd. KongCTL consumes
v0.79.1-0.20261031220541-2597fe249e26. No application compatibility change
is required. The exact generated-code delta from 30c3834c9846 is three Page
tags, in ListVaultKeys, ListVaultProviders and ListCredentialProviderTemplates:
form becomes deepObject. The cursor wire test has thirteen operations and
fails exactly these three before correction; all thirteen pass afterward.

The SDK's identity-preview/vault-cursor-audit.json verifies exact tag-only
changes. Prepared OpenAPI, global generation settings, response models,
go.mod/go.sum and the 1,102-operation inventory are unchanged. Prior targeted
credential expiration, omission/default and array patches are retained.
Generation succeeds with zero errors and 255 existing validation warnings;
SDK root/model/internal/pkg tests, internal race checks and the two expiration
preparation tests pass. This SDK change does not resolve Q37/Q39/Q40.

A standalone SDK read probe passes against native US `.tech` fixtures:
three keys on three pages, three providers on three pages and eight templates
on eight pages, all with page size one and matching get-by-ID/name responses.
All 28 SDK requests route to US `.tech` through the same host-rewrite strategy
required by the existing helpers; no global-server override alone is assumed.
Fixture teardown verifies public key identity, provider names and empty
credential collections before explicit deletion, then restores directory
settings. Evidence: vault-sdk-live-v2.log and vault-sdk-live-summary.json.
The first probe attempt had wrong Go method argument shapes and failed at
compile time; fixture cleanup passed, and the corrected standalone probe
passes. It is not an API failure or another SDK patch.

No new product command or declarative resource is enabled by this pin alone.
The next implementation work is public key/provider/template integration
using these verified observations, with ownership and parent-deletion policy
explicitly established before writable declarations.

Consumer modernization, stable formatting, CGO-disabled build and final
make test-all pass: zero-issue lint, full unit/integration/harness race tests,
nine SDK patch tests, installer checks and 140 metrics tests. The default
cassette check passes; replay membership/cassettes are unchanged. All eighteen
Identity live scenarios pass together on the new pin in 297.747 seconds,
with eight referenced private capture directories removed. Artifacts are
vault-sdk-all-identity/20261031-170816; gate evidence is
vault-consumer-gates.log and vault-artifact-verification.json.

Fresh native inventory temporarily enables Vault to inspect hidden child
collections and then restores the saved settings. The final unmanaged default
is 01a19406-3817-74ac-b4f6-300f806fd4fb; principal/group/IdP/provider/vault-key
and auth-server inventories are empty. Evidence is
vault-sdk-final-inventory.json. SDK #1413/#1399, #2660/#2661, ownership #2656
and directory epic #1132 remain In Progress until main merge. No PR, SDK
release, provider/key product writer or implicit parent adoption is created.

## 49. Directory Vault dependency deletion guard

Native Q37 proves force=false does not preserve attached Vault children.
Directory delete and scoped-sync planning now read fresh directory detail,
validate its ID/name, and require observable empty Vault key/provider/Trusted
IdP collections. Saved execution repeats current settings and all dependency
observations before the API DELETE. Existing namespace/protection checks are
retained. Older saved plans do not bypass the new execution precondition;
an already absent directory remains an idempotent deletion success.

The shared helper sends no pagination, sort or filter arguments. An empty
data array with valid terminal pagination proves empty; missing data/meta,
nonterminal emptiness, fractional/invalid size, read failure or missing APIs
block deletion. Disabled or unknown vault_enabled is an explicit error.
Planning and deletion never enable Vault. The operator must explicitly enable
it and retire dependencies first. No key/provider/IdP ownership is stamped,
no adoption is inferred, and no remaining child is removed automatically.
Ordinary directory create/update and external-default principal workflows do
not acquire these inspection requirements. The SDK stays 2597fe249e26.

This follows the established fail-closed credential-deletion behavior rather
than treating a successful native non-forced DELETE as a preservation proof.
It deliberately restricts directory deletion where the API cannot expose its
dependencies, including deployments unable to retain vault_enabled:true.
It does not make observation/delete atomic: Q37 requests a backend conditional
contract for attachments between the final read and DELETE. Future writable
child integration must prove explicit authorized removals in planning and
fresh emptiness after those child operations, not bypass this precondition.

The new identity/directory-delete-guards live scenario passes independently
in 39.746 seconds (directory-guard-v1/20261031-172951). It creates one child
from each family, then verifies plan/delete, scoped-sync planning/execution,
and previously saved deletion all refuse the attached record. Explicitly
disabling Vault with that child preserves the directory and blocks saved
execution; re-enabling restores visibility. Late disabling of an otherwise
empty directory also blocks deletion. After explicit child retirement and
enablement, saved deletion succeeds and repeated execution is idempotent.
The issuer fixture supplies public OIDC discovery; the key fixture contains
only public material. No credential secret or runtime-auth claim is involved.

Fixture cleanup opts into requireEmptyVaultDependencies. It observes all three
empty collections before any cleanup writes and immediately before parent
deletion; missing/nonempty/error/nonterminal results preserve the fixture.
It grants no child-deletion or implicit-enable authority. Tests cover invalid
configuration and each refusal, while other scenarios gain no new permissions.
Existing managed-directory teardown examples now enable inspection explicitly,
after separate unprotection. New directory-deletion.md documents those steps,
raw collection inspection, permissions, saved plans and the atomicity limit.

Early broad live runs exposed two teardown fixtures needing that explicit
step: memberships had combined enablement with unprotection (correctly
rejected), and numeric metadata still deleted a Vault-disabled directory.
Those fixture sequences are corrected; memberships and directory lifecycle
pass in the second run. Numeric metadata then passes separately in 6.762
seconds (directory-guard-integers-fixed/20261031-174451). These are client
fixture changes, not additional API findings. Failed broad runs are retained
as directory-guard-all-identity and directory-guard-all-identity-v2; they are
not passing-suite evidence.

Final verification passes all nineteen Identity live scenarios together in
334.715 seconds: directory-guard-all-identity-v3/20261031-174619. All eight
referenced private capture directories are absent. A separate early stop
after create-issuer runs exactly two commands and passes the new fixture
cleanup in 3.346 seconds: directory-guard-stopped/20261031-175625. Fresh native
inventory, including an explicit temporary Vault inspection and restoration,
confirms empty principal/group/key/provider/IdP/auth-server inventories. The
unmanaged default and public settings are restored with ID
01a1942d-c2ba-7ccc-ab74-25d906544915.

Modernization, stable formatting, CGO-disabled build and make test-all pass,
including zero-issue regular lint, full unit/integration/harness race tests,
nine SDK patch tests, installer checks and 140 metrics tests. Final unit and
focused helper/cleanup checks also pass after strengthening populated-response
coverage. The default cassette check passes and all replay membership/cassettes
are unchanged. An optional whole-repository E2E-tag lint run reports existing
findings outside this change; after correcting new line wrapping, E2E-tag lint
restricted to changed lines reports zero issues. It is not a clean expanded
repository-lint claim.

Evidence is directory-guard-gates.log, directory-guard-final-unit.log,
directory-guard-final-lint.log, directory-guard-e2e-diff-lint.log,
directory-guard-artifact-verification.json and
directory-guard-final-inventory.json under the local evidence cache. Issue
#1414, ownership #2656, Vault child issues and epic #1132 stay In Progress
until main merge. No PR or SDK revision is needed for this guard. Writable
Vault children remain separate implementation work under an explicit
ownership/collection policy.

## 50. Token Vault public key declarations and inspection

Implemented `identity.directories[].vault_keys` and grouped
`identity.vault_keys` with a `directory` ref. Required `name` and `public_key`
remain explicit. The shared parser accepts PKIX `PUBLIC KEY` material for
RSA >= 2048 bits and EC P-256/P-384/P-521, rejects private/other encodings, and
computes RFC 7638 SHA-256 fingerprints. Independent vectors from native probes
cover both RSA keys, all three EC curves and equivalent CRLF PEM. Complete
reads verify route IDs, directory, material/fingerprint, algorithm, size and
required timestamps; malformed, repeated or partial inventories fail closed.

Keys have no metadata store. Explicitly managed directory ownership and a
declared collection supply authority, as for inherited auth-server children.
Parent adoption preserves existing keys and authorizes subsequent explicit
collection management. No individual key metadata/adoption or hidden state is
invented. External parents remain read-only. Omitted collections preserve keys;
explicit empty collections authorize pruning. Saved writes recheck directory
identity, namespace, ownership, protection, enablement and current key identity.
Protected parents permit creation but block child updates/deletion, following
the existing inherited-child policy. Existing disabled Vaults must be explicitly
enabled before key planning; new enabled parents and keys can share one plan.

Matching uses `kid`, not nonunique name or local ref. Name changes PATCH the
existing route; material changes create another record. Apply preserves old
material. Scoped sync creates new material before retiring old keys. Planning
and saved creation enforce the five-key ceiling; capacity requires an explicit
selected retirement before adding material, never an inferred deletion.
POST retries are disabled. Public response validation distinguishes an
uncertain outcome from success; a fresh plan can observe any created key.

Directory deletion planning accepts only exact authorized key IDs/fingerprints
scheduled for deletion and still requires empty provider/Trusted IdP
collections. Parent dependencies order key retirement first. Execution retains
the prior guard's fresh actual-empty checks and never cascades an extra child.
Late attachments block parent deletion even after authorized key retirements;
repeated saved deletion is idempotent. The Q37 observation/delete race remains
an API contract limitation, not something these client checks can eliminate.

Public get/list supports UUID or exact kid selection, all output formats and
complete cursor traversal. Text summarizes public identity/algorithm/size;
JSON/YAML include public PEM. Dump exports public keys only beneath managed,
enabled parents and reloads without changes. External/disabled parent key
collections are omitted without claiming emptiness. Explain/scaffold,
default-directory bootstrap/adoption examples and the metadata ledger describe
these boundaries. No private material or runtime encryption behavior is
claimed; the deletion effect is attributed to the API specification.

Live testing exposed an unrelated-family dependency leak: the AI Gateway
name-replacement pass added delete-before-create edges to Identity keys with
duplicate names. Its pass is now limited to AI Gateway kinds. A same-name key
rotation regression preserves the intended create-before-retirement order;
existing AI Gateway dependency tests pass. This was a KongCTL bug, not a new
engineering API question. The first live run also used an incorrect test flag;
the scenario now uses the established `--page-size`.

The complete new scenario passes in US `.tech` in 32.250 seconds:
`vault-keys-live-v3/20261031-181711`. It covers explicit parent adoption,
RSA/EC public inspection, actual three-page lists and fingerprint lookup,
duplicate names, same-route rename, CRLF no-op, grouped declarations,
ownership/protection refusal of saved writes, disabled Vault behavior,
dump/reload, omitted/empty sync, material rotation, quota retirement, late-child
parent refusal, saved/repeated/direct deletion and new-parent-plus-key creation.
The unmanaged default and saved settings were restored after fixture cleanup.
Failed v1/v2 runs are retained as failed evidence, not passing validation.

Final validation passes modernization, stable formatting, CGO-disabled build
and make test-all: zero-issue lint, full unit/integration/harness race tests,
nine SDK patch tests, installer checks and 140 metrics tests. Default cassette
validation passes; all twenty replay memberships and their cassettes are
unchanged. This is not a claim of twenty isolated replay executions.

All twenty Identity live scenarios pass together in US `.tech` in 365.701
seconds: `vault-key-all-identity/20261031-182243`. All diagnostics report
passed, and all eight referenced private capture directories are absent.
An early stop after the single directory-creation command passes guarded
cleanup in 2.178 seconds: `vault-key-stopped/20261031-182931`.
Both checked-in examples produce the expected one- and two-create plans during
read-only `.tech` verification. The key example loads actual public PEM through
`!file`.

Fresh inventory confirms empty principal/group/key/provider/IdP/auth-server
collections. The unmanaged default and saved settings are restored with ID
`01a1944c-0cb9-7abe-aa41-8dc8b1f0bbfb`; Vault is false after inspection.
Evidence is `vault-key-build.log`, `vault-key-gates.log`,
`vault-key-replay.log`, `vault-key-artifact-verification.json`,
`vault-key-final-inventory.json` and the example plan files in the local cache.

#2661 stays open/In Progress until main merge. Providers, shared/enrolled
credentials, Trusted IdP declarations and importer work remain separate slices.

## 51. Credential Provider template catalog and public inspection

Added `get/list identity vault-provider` and `credential-provider-template`,
including plural aliases and public JSON/YAML/text. Providers accept UUID or
exact directory-scoped names; `--provider-name` disambiguates UUID-shaped names.
Global templates require no directory selector and remain visible when a
directory's Vault is disabled. Providers expose public configuration and
effective endpoint values, not stored override metadata or secret material.
The tutorial explains how upstream Credential Providers differ from Kong
auth-server OAuth clients and how shared versus user credentials relate to
them. Templates are a built-in catalog, not writable desired-state resources.

Both collections traverse actual cursors through SDK `2597fe249e26`, with raw
response validation to avoid synthesized defaults. Required identity, boolean,
array, timestamp and pagination fields must be observable. Repeated IDs/names,
wrong directory responses, incomplete pages, malformed values and read failures
return no partial collection. False/empty values and optional unknown PKCE
requirements retain their meanings. Public models cannot carry unexpected
client secrets, credential values, enrollment URLs or state tokens. Ordinary
inspection issues GET requests to directory/provider/template configuration
routes, without ownership, enablement or enrollment side effects.

A 51-request native `.tech` follow-up establishes three further boundaries.
An active static shared credential survives a public provider PATCH with the
same visible ID/status, but deleting its provider returns 204 and its own GET
then returns 404. Q41 asks for dependency/conditional deletion contracts;
user-enrolled cascade behavior is not inferred. The custom DCR public PATCH
response keeps the effective registration endpoint and DCR mode. Stored-secret
retention requires runtime verification. The documented enrollment
resolve-state route could expose the stored client secret to its gateway, but
signed enrollment
resolve with an administrative bearer returns 400 for missing mTLS context.
Q30 now includes that integration requirement; no internal headers were forged.

Q42 records that OAuth create scopes:[] selects template defaults and PATCH
scopes:[] fails. Null endpoint PATCH fails; empty endpoint strings succeed and
become actual empty effective endpoints rather than returning to template
defaults. These are direct API observations. Supplied secrets and signed
capabilities existed only in memory; evidence stores selected public fields,
statuses and booleans. All fixtures were removed and default settings restored.
Evidence is provider-preservation-contracts.json and
provider-preservation-summary.json in the local cache. Platform PR 3877 is
still open at inspected head 98137d7be3772b67693cc1ceb0469e77f8ecf0f5.

The new inspection scenario passes in US `.tech` in 14.626 seconds:
provider-inspection-live-v1/20261031-185322. It checks all eight templates and
five providers spanning static/shared, static OAuth, custom DCR and template
DCR defaults. Real page-size-one traversals, exact name/UUID lookup including
UUID-shaped names, every public field, all formats, unchanged configuration,
unchanged management metadata and disabled-Vault/catalog behavior pass. It
asserts that every product request is GET against `.tech` and no credential
route is called. Cleanup observes empty credential collections before removing
the known native providers; guarded parent cleanup then restores the default.

Final validation passes modernization, stable formatting, CGO-disabled build
and make test-all: zero-issue lint, full unit/integration/harness race tests,
nine SDK patch tests, installer checks and 140 metrics tests. Default cassette
validation passes; the twenty existing replay memberships and cassettes are
unchanged. No claim of twenty isolated replay executions is made. A first gate
run stopped on a synthetic response-only G101 warning; the fixture now uses the
repository's narrow rule-specific annotation for its intentional exclusion
sentinel. No global linter configuration or production protection was weakened.

All twenty-one Identity live scenarios pass together in US `.tech` in 388.684
seconds: provider-reader-all-identity/20261031-190115. Every diagnostic reports
passed, and all eight referenced private capture directories are absent. A
separate early stop after the single directory-create command passes guarded
cleanup in 2.172 seconds: provider-reader-stopped/20261031-190923. Fresh native
inventory verifies empty principal/group/key/provider/IdP/auth-server
collections; the unmanaged default is restored with saved settings and Vault
false, ID 01a19470-8c80-7988-826b-0fa8e2f45206. Evidence is
provider-reader-gates-v2.log, provider-reader-replay.log,
provider-reader-artifact-verification.json and
provider-reader-final-inventory.json
under the local cache. The SDK stays 2597fe249e26; no release or PR is created.

The next writable provider slice must reuse explicit managed-directory/collection
authority, add fresh credential-dependency guards in planning and execution,
compare configured endpoint literals to effective values without inventing
stored overrides, and use verified PATCH semantics instead of unsafe PUT
hydration. Deferred client-secret sources and explicit update selection must
follow #1748, without claiming read-based secret drift or runtime preservation.
Create-only configuration changes and clearing need explicit supported
boundaries until Q39/Q40/Q42 are resolved. #2660 remains open/In Progress until
main merge; this checkpoint does not complete its declarative acceptance.

## 52. Credential Provider write foundations and persistence verification

The next #2660 prerequisite supplies internal create, PATCH and guarded DELETE
helpers with SDK/mock accessors. This checkpoint does not register a resource
kind, add CLI mutation commands or complete declarative acceptance. Parent
ownership/protection and explicit collection authority remain responsibilities
of the upcoming planner/executor integration.

Create requires the API name and template explicitly, validates static-secret
credential scope and incompatible client/DCR settings, and checks supplied
public configuration against its response. Names use the provider schema's
ASCII pattern, without borrowing the unrelated Vault key length constraint.
Explicit empty scopes and endpoint strings fail before mutation because their
observed API meanings cannot represent declarative clearing (Q42). Secret
values may be supplied to the typed helper but are absent from its public
return model. Both SDK and HTTP-client write retries are disabled; an uncertain
create must be inspected rather than automatically repeated.

PATCH reads fresh directory/provider identity and expected name before writing.
It verifies requested public fields and preservation of omitted public fields
in both the PATCH response and a separate GET. This covers DCR registration,
static client ID, credential type, PKCE, scopes, enabled/refresh values and
effective endpoints. Only explicit client-secret writes send client_secret;
ordinary public updates omit it. This proves request omission and public
persistence, not the actual stored-secret value or runtime enrollment.
Replacement PUT remains outside the verified path under Q39/Q40.

DELETE repeats a fresh name/UUID observation and requires an unfiltered,
terminal empty credential collection. Missing, malformed, nonterminal and
failed observations block mutation. The request omits page/sort arguments,
so it does not depend on the unfixed credential cursor annotation. It never
reads credential detail or mints enrollment capabilities. A shared terminal
empty-collection predicate also retains existing directory deletion behavior
with the correct parent context in errors. The guard cannot atomically
exclude a concurrent attachment; Q41 asks for that API contract.

A focused 49-request direct US `.tech` probe verifies fresh GET persistence of
custom OAuth DCR settings after public PATCH. It also finds that static-secret
POST persists scopes, PKCE, refresh and endpoint settings despite narrower
template guidance. PATCH persists scopes/PKCE/token/authorization settings;
enabling DCR fails. Q43 asks which static-secret settings affect runtime.
No runtime effect or successful dynamic registration is inferred from these
configuration observations.

A separate 31-request probe invokes the actual new SDK-backed helpers in US
`.tech`: DCR creation and public PATCH persist, explicit refresh zero persists,
and an active shared credential prevents any provider DELETE request. Guarded
cleanup succeeds after explicit credential removal. No credential detail GET
is made. All providers, credentials and the public RSA fixture are removed;
the unmanaged default directory's settings are restored. The standalone probe
rewrites both URL host and HTTP request host to `.tech` and rejects other
destinations; it does not rely on a global SDK server setting overriding
operation-specific server URLs.

Public evidence is provider-write-persistence-contracts.json and
provider-sdk-writes-summary.json under the local evidence cache. Test secret
material is generated in memory and is not written to evidence. The SDK stays
2597fe249e26; no SDK revision, release or PR is required for this foundation.

Next: register the complete provider declaration contract, inherit explicit
managed-directory namespace/protection, integrate create/PATCH and fresh
credential guards with saved execution, and add scoped sync/delete, dump,
deferred secret-source handling, examples and lifecycle E2E. Provider PUT and
clearing limits must remain explicit rather than silently discarding fields.
#2660 remains open/In Progress until merge to main.

The secret-only integration needs explicit context work: current
secretResourceNamespace handles selected roots but not IdentityDirectory, and
applyInheritedProtection runs on namespace plans before secret-write synthesis.
A synthesized provider update therefore needs verified directory ID,
namespace and protecting-parent context even when there is no public drift.
Tests must cover that case and ensure create-only fields supplied as secret
context are validated rather than silently discarded by PATCH translation.

Verification passes: Go modernization, stable formatting, CGO-disabled build
and make test-all, including zero-issue lint, full race unit/integration tests,
E2E harness race checks, nine SDK patch tests, 140 metrics tests and installer
checks. The final log is provider-write-foundation-final-gates.log. No E2E
scenario, enabled replay membership or cassette is changed; this slice uses
targeted SDK wire tests and the native/live helper probes rather than claiming
provider declaration E2E coverage. Final inventory evidence is
provider-write-foundation-final-inventory.json: one unmanaged default
directory, original settings restored and empty principal/group/IdP/provider/
key/auth-server inventories.

## 53. Credential Provider declarative lifecycle

Provider declarations are now integrated through resource/schema registration,
nested and grouped loading, namespace selection, explicit collection scopes,
planning/state access, saved execution, dependency ordering and public dump.
`identity.directories[].vault_providers` and grouped
`identity.vault_providers` use directory/name matching. Ref is local; no name
is derived from it and no independent provider metadata is invented.

Writes require a declared managed directory. Parent adoption grants that
root's namespace/protection, and an explicit provider collection authorizes
sync pruning. Omission preserves the collection. External directory references
are read-only for declarations. Provider creation is permitted under parent
protection, while update/delete require an unprotected parent, consistent with
other inherited children. Keys/providers share the fresh directory authority
policy at execution.

Create supports the API configuration fields and optional deferred client
secret sources. Initial enabled:false uses POST then PATCH with another fresh
parent check; it is not atomic and a failed second step leaves a created
provider to inspect/replan. Public updates use PATCH, including false and zero
values. Template, static client ID, credential type, DCR mode and registration
endpoint changes fail clearly because the verified PATCH schema lacks them;
neither unsupported PUT hydration nor silent field dropping is used. A name
change creates another record; apply preserves the old provider and explicit
sync can retire it after guards.

During sync, replacement creates precede provider retirements under the same
directory, matching the Vault-key ordering policy. A failed registration does
not authorize deleting the previous configuration. Live coverage verifies
this ordering for a name change.

`!secret {source: !env NAME}` follows the existing write-only framework.
Creation includes the configured source once. Later updates need
`--write-secret ref#client_secret` or `--write-secrets`; ordinary reconciliation
does not read or rewrite it. Secret-only plans now retain the directory's
resolved ID, namespace and protecting-parent context even with no public drift.
Saved execution repeats ownership, protection and public create-context
checks. Source values never enter plan, diff, public read or dump output.
Public state cannot prove stored-secret continuity or successful enrollment.

Provider deletion requires a full fresh directory-scoped identity observation
and terminal empty credential observation, both during planning and saved
execution. It never auto-deletes credentials or reads enrollment detail.
Directory deletion can order exact authorized provider retirements alongside
keys, then still requires actual empty inventories after those writes. A late
unplanned provider blocks the parent; a late credential blocks its provider.
Q41 retains the API race between final observation and DELETE.

Dump exports public settings under enabled managed directories, omits secrets
and pins effective endpoints. Empty scopes are reproducible only when the
template also has no defaults; otherwise export fails. Invalid empty endpoints
also fail instead of producing an invalid declaration. External/disabled
parents do not gain an exported provider collection. Q39/Q40/Q42/Q43 retain
replacement, initial enablement, clearing and static-setting runtime limits.

The default-directory example groups static-secret, static OAuth application
and custom DCR providers, including deferred credentials. The guide covers
existing-directory adoption/enablement, missing-directory bootstrap, public
inspection, explicit secret writes, scoped sync, dump and deletion boundaries.
Loader coverage verifies the example without resolving its secret. Unit tests
cover saved authority/context changes, all mutable fields, create-only errors,
secret omission/selection, empty/omitted scopes, late dependencies and dump
reproducibility. The reviewed default inventory adds only the new resource's
empty default/override/exclusion entry.

The new live scenario identity/declarative-vault-providers first passes in US
`.tech` in 25.48s. It verifies all three modes and public fields, initial false,
saved create, ordinary/secret-only updates, parent adoption/protection, diff,
scoped sync, public dump/reload, late credential/provider guards, ordered
deletion, repeat deletion and missing-directory bootstrap. It generates test
secrets only in memory and checks artifacts for their raw/base64 values and
the administrative PAT. Public settings do not prove runtime OAuth consent,
dynamic registration or secret continuity. The final twenty-two-scenario
Identity regression passes in 407.214s; its expanded provider scenario passes
in 28.26s using a directory actually named default. Artifact verification
confirms all twenty-two outcomes and removal of eight private capture
directories. A guarded early stop after directory creation also passes and
removes its fixture. The unmanaged default and original settings are restored;
principal/group/IdP/provider/key/auth-server inventories are empty.

Final modernization, stable formatting, CGO-disabled build and make test-all
pass: zero-issue lint, full unit/integration and harness race checks, nine SDK
patch tests, 140 metrics tests and installer checks. The default cassette check
passes; no existing replay membership or cassette changes. The new scenario
is live-only under the current replay policy. SDK wire coverage verifies the
deletion guard blocks either shared or user credential response variants;
actual user enrollment is outside the live scenario under Q30.

Evidence under the local cache includes provider-declarative-final-gates-v2.log,
provider-declarative-all-final/20261031-201328,
provider-declarative-final-artifact-verification.json,
provider-declarative-stopped.log, provider-declarative-replay-check.log and
provider-declarative-final-inventory.json. The earlier twenty-two-scenario run
and isolated key/default-provider runs are intermediate verification; the
final run also covers replacement ordering.

SDK version remains 2597fe249e26. No generated SDK change, release or PR is
needed. The provider-credential cursor correction remains future work before
full credential pagination; the deletion guard uses no page/sort arguments.
#2660, ownership contract #2656 and directory epic #1132 remain open/In Progress
until main merge. Shared/user provider credentials, Trusted IdP declarations
and importer/team work remain separate feasible investigations/implementations.

## 54. Trusted IdP write foundations

The next active resource is Trusted IdP configuration (#2664). Full PUT has
been verified to preserve hydrated public configuration, clear nullable
display/JWKS overrides and normalize empty JIT mappings. The helper checks
fresh directory membership and exact issuer, makes one mutation attempt and
verifies the receipt plus fresh readback. It does not establish ownership.

An associated-principal observation includes inactive principals. Explicit
OIDC identity retargeting/deletion removes those associations. Native trust
deletion accepts existing associations and leaves their principals/identities
readable, so the guarded helper requires terminal empty associations first.
Q44 records API-only questions about runtime trust removal and a conditional
deletion contract; the client cannot close the observation/DELETE race.

A 47-request native `.tech` probe and a separate 36-request SDK-helper probe
pass. Their public evidence is trusted-idp-lifecycle-contracts.json,
trusted-idp-lifecycle-summary.json and trusted-idp-sdk-write-summary.json in
the local evidence cache. All fixtures were removed and original default
settings restored. No SDK revision beyond 2597fe249e26 is needed.

Full declarative loading, planning, saved execution, scoped sync, dump,
examples and lifecycle E2E are still to be integrated. The helpers are a
foundation for those features, not completion of #2664. Ownership will follow
the explicit managed-directory policy used by Vault keys and providers;
principals and identities retain independent authority.

Verification passes: Go modernization, formatting, CGO-disabled build and
make test-all, including zero-issue lint, race unit/integration and harness
checks, nine SDK patch tests, 140 metrics tests and installer checks. The log
is trusted-idp-write-foundation-gates.log. Final independent inventory is
trusted-idp-write-foundation-final-inventory.json: one unmanaged default with
original settings restored and empty principal/group/IdP/provider/key/
auth-server inventories. No E2E scenario or replay membership changed in this
foundation slice; the native and actual-helper probes verify its write paths.

## 55. Trusted IdP declarative lifecycle

Trusted IdP declarations are integrated through registration/schema,
nested/grouped loading, namespace/collection scope, planning/state access,
saved execution and public dump. Required fields are ref and issuer_url.
Matching uses exact issuer per directory; display names may be duplicated,
and local refs are not persisted. Authority/protection comes from an explicitly
managed directory, without inventing labels or managed_by. External directory
references remain read-only for trust configuration. Existing Vault must be
enabled before its complete collection can be observed.

Create accepts all public settings. Hydrated full PUT preserves omitted
settings from a fresh detail read, including both booleans, nullable overrides
and JIT mapping. Explicit display_name/jwks_uri null and jit:{} clear those
settings; a supplied mapping replaces its entries. Nonnullable booleans,
mapping objects and claim names reject null rather than silently dropping it.
Helpers verify full public receipts/readback and do not retry uncertain writes.
Q38 is handled through verified PUT instead of relying on PATCH null clearing.

Explicit collections scope sync and omission preserves them. Issuer changes
create a separate trust record; scoped sync creates the replacement before
retiring the old configuration. It never retargets identities automatically.
Full final plans may order independently authorized OIDC retargeting/deletion
or principal deletion before retirement, including across namespaces. Every
matching identity must detach; ownership-only changes, unknown associations
and planned new attachments do not count. Execution repeats exact directory
membership/issuer and parent ownership/protection/Vault checks, then requires
actual terminal empty associations. Inactive principals still count.

Directory retirement can authorize exact key/provider/trust deletions before
its final actual-empty checks. A late unplanned trust blocks the parent; a
late association blocks its trust. Native trust deletion leaves principals
and identities readable, and client observations cannot close the final
attachment/DELETE race (Q37/Q44). Administrative configuration does not prove
runtime token exchange/JIT behavior or grant ownership over provisioned output.

Dump exports complete public configuration under enabled managed parents,
including explicit nullable overrides/empty mappings. Examples and the guide
cover existing-default adoption/enablement, missing-default bootstrap, saved
plans, diff, public reads, omission, scoped sync and retirement. They explain
the existing source-default namespace rule for explicitly empty independently
owned identity collections. Ref and issuer roles remain distinct.

The first complete live lifecycle passes in US .tech. It covers three trust
records, duplicate display names, all public settings, fresh-state hydration,
clears, parent adoption/protection, saved association/parent guards,
replacement ordering, cross-namespace OIDC retargeting and deletion, dump,
scoped deletion/repeat and bootstrap. Initial scenario attempts exposed test
namespace setup errors; guarded recovery removed only the verified fixture
records and restored original default settings. No API defect was inferred
from those test setup errors. All twenty-three Identity live scenarios pass
together (445.191s), including this scenario in 30.17s and an actual default
bootstrap. The final isolated lifecycle passes in 33.48s after strict null
schema/validation checks; early stop after issuer creation passes in 3.41s.

Modernization, stable formatting, CGO-disabled build and make test-all pass:
zero-issue lint, full race unit/integration and harness checks, nine SDK patch
tests, 140 metrics tests and installer checks. The default cassette check
passes; no existing replay membership/cassette changes. The new scenario is
live-only. Artifact verification confirms twenty-three passing outcomes,
removal of eight private capture directories, actual default bootstrap and
absence of the administrative token in artifacts. Independent final inventory
confirms an unmanaged default with restored original settings and empty
principal/group/IdP/provider/key/auth-server inventories.

Evidence under the local cache includes trusted-idp-declarative-all/
20261031-205835, trusted-idp-declarative-final/20261031-210659,
trusted-idp-declarative-final-gates.log,
trusted-idp-declarative-artifact-verification.json,
trusted-idp-declarative-stopped.log, trusted-idp-declarative-replay-check.log
and trusted-idp-declarative-final-inventory.json. #2664 stays open/In Progress
until main merge.

SDK remains 2597fe249e26. No SDK generation/revision, release or PR is required
for this resource. Shared/user provider credentials and importer/team work
remain separate implementations. Ownership tracking #2656 and directory epic
#1132 stay open/In Progress, as does #2664.

## 56. Credential identity, SDK cursors and public observation

The shared provider-credential API accepts multiple credentials for static
secrets and shared OAuth, contradicting the inspected POST description's
single-static-credential wording. A 71-request direct US .tech probe and a
separate 93-request routing matrix establish a unique routing-priority slot
plus one default slot per tested provider. Selectors can repeat at different
priorities. POST/PUT collisions return 409; moving a default to a priority
frees the default slot. Wrong-provider GET/PUT/DELETE return 404 and preserve
the original credential. Q45 asks for the formal cardinality/uniqueness
contract.

Static PUT requires secret material. Selector/priority-only or empty PUT
returns 400. With a secret, routing can change; omission resets both fields
to the default slot, while explicit null is rejected. Shared OAuth PUT accepts
public routing changes but issues another enrollment URL, as does detail GET.
The tested OAuth records stay pending. Q46 asks for material-preserving update
operations and explicit omission/enrollment documentation. No active enrollment
or stored-material continuity is inferred from public state. Supplied secrets
and issued capabilities stayed in memory and were excluded from evidence.

Native provider credential pagination traverses five records at size one.
Group-key pagination traverses three records. Flat size=1 queries report the
default 20. An empty principal Vault-credential inventory honors bracketed
size one; populated user pagination still needs Q30's runtime fixture. A
separate 32-request probe verifies the latter two collection contracts. Its
first fixture omitted the required generated-key type and was corrected; this
was a test setup error, not an API defect. Every probe removes its fixture
records and restores the original default directory settings.

SDK patch source c775a9c4 extends the existing Q22 query fix to
ListVaultProviderCredentials, ListVaultCredentials and ListGroupKeys. Before
patching, the expanded wire regression fails exactly those three methods;
afterward all sixteen methods pass without skips. The public preview SDK
branch is pushed at 90596b498bcb, consumed as
v0.79.1-0.20261101022339-90596b498bcb. Its generated-code delta from
2597fe249e26 is exactly three Page query tags; no component models or unrelated
APIs change.
Patch snapshot/provenance hashes, Go SDK package tests and internal race tests
pass, and post-generation application is idempotent. No release or PR is made.

KongCTL now exposes provider-scoped vault-provider-credential(s) and
principal-scoped principal-vault-credential(s) under get/list. UUID selection
uses a complete scoped collection. The provider reader's interface excludes
detail GET, preventing an ordinary metadata lookup from minting a capability.
Provider selection supports explicit UUID or exact name; directory/principal
selectors follow existing conventions. JSON/YAML preserve the shared/user
public variants, including routing priority zero and nullable defaults/expiry;
text uses summary tables. Unknown, malformed, repeated or failed observations
yield no partial inventory. Public types contain no secrets or enrollment URL.
Whole credential-route trace bodies are redacted, including error plaintext.

Examples explain default-directory preparation, provider/credential/principal
concepts, selectors, paging and enrollment boundaries. The initial live
scenario passes in 11.52s: three active static slots, two pending shared OAuth
slots, real pagination, ID/name scope, all output formats, empty principal
authorization inventory, unchanged public records and absence of secret or
capability artifacts. Request traces confirm no credential detail GET. Unit
tests also cover populated user/principal projections, malformed pagination,
unknown variants, duplicate IDs/cursors, public formatting and trace redaction.
Populated user enrollment/revocation is not live verified; Q30 still requires
a supported runtime integration.

This is a read/SDK foundation for #2662/#2663, not complete credential CRUD or
declarative ownership. No metadata is stamped or user authorization pruned.
The next feasible declaration candidate is static shared credentials, using
the observed default/priority slot and explicit managed-parent authority,
with selected deferred secret writes for replacements. The routing key and
material requirements must be explicit; no whole-provider singleton or secret
comparison assumption is valid. OAuth enrollment needs a deliberate private
capability flow, and user revocation needs an actual enrolled runtime fixture.

Evidence is provider-credential-contracts.json,
provider-credential-routing-contracts.json,
credential-other-cursor-contracts.json,
credential-cursor-sdk-delta.json, credential-cursor-wire-verification.log,
credential-cursor-sdk-tests.log, credential-cursor-sdk-race-tests.log and
credential-inspection-final-gates.log under the local evidence cache. The
twenty-four-scenario Identity regression passes in 449.164s, with the expanded
credential scenario passing in 13.49s. Early stop after directory creation
passes cleanup in 2.06s. Artifact verification confirms twenty-four outcomes,
removal of eight private capture directories, actual default bootstrap and no
administrative token in artifacts. Final independent inventory confirms one
unmanaged default, original settings restored and empty principal/group/IdP/
provider/key/auth-server inventories.

Final modernization, stable formatting, CGO-disabled build and make test-all
pass: zero-issue lint, full race unit/integration and harness checks, nine SDK
patch tests, 140 metrics tests and installer checks. The default cassette
check passes; no existing replay membership or cassette changes. The new
scenario is live-only. Evidence also includes credential-inspection-all/
20261031-213556, credential-inspection-artifact-verification.json,
credential-inspection-stopped.log, credential-inspection-replay-check.log and
credential-inspection-final-inventory.json. #2662/#2663 stay open/In Progress
until main merge; declarations/enrollment/revocation are subsequent work.

## 57. Static shared credential write foundations

The next #2662 slice implements guarded static credential create, explicit
material replacement and deletion. The narrow write interface excludes
credential detail GET. Fresh provider reads verify exact directory/provider
UUIDs, name and static_secret/shared mode. Complete list observations reject
duplicate priority/default slots, user/pending variants, missing UUIDs and
stale routing before writing. Unrelated slots remain intact. These helpers
do not establish management ownership/protection; those checks belong to the
declarative execution integration that follows this foundation.

Replacement always receives explicit material. A nil requested routing object
preserves freshly observed selector/priority, avoiding the native PUT reset.
An explicit empty routing object requests the default slot; a nonempty pair
can change the selector/priority. The expected pre-write routing is checked
against fresh state, and occupied destination slots block before PUT. Each
mutation makes one attempt through both HTTP and SDK retry layers. Public
receipt and full collection readback must agree; delete requires the target
UUID to disappear. No stored-material or runtime-continuity claim is inferred.

Actual helper calls in US .tech verify three creates, three replacements and
three deletions, with three conflicts rejected before mutation. Coverage
includes a default, priority zero, repeated selectors at distinct priorities,
material-only replacement preserving routing, selector replacement and a
priority move. The helper performs 34 reads and nine writes, with no credential
detail GET. Initial probe setup omitted an encryption key and received HTTP
400 on creation; adding the public-key fixture makes the lifecycle pass. This
is a fixture correction, not evidence of a new API defect. The public
credential guide now calls out that prerequisite.

Wire regressions exercise both enabled retry layers, secret/capability
exclusion, wrong provider name/directory/template/mode, later-page failures,
duplicate default/priority slots, stale routing, collisions, unexpected
variants, receipt/readback disagreement, retained-after-delete responses and
invalid local IDs/routing. Failed observations return no success result and
no credential material. Actual live fixtures are removed, and an independent
inventory confirms the original unmanaged default settings and empty
principal/group/IdP/provider/key/auth-server inventories.

Evidence is static-vault-credential-write-live.json and
static-vault-write-foundation-final-inventory.json under the local evidence
cache. The metadata ledger and compatibility tracker distinguish internal
write foundations from exposed declarative support. SDK 90596b498bcb already
provides the mutation methods, so no SDK change, release or PR is needed.

The next integration must include nested/grouped declarations, the observed
default/priority identity, deferred !secret sources and explicit replacement
selection, fresh inherited directory ownership/protection, encryption-key
dependencies, scoped sync/parent retirement ordering, dump/explain/scaffold,
examples and a live CLI lifecycle. No credential declaration or new imperative
write command is exposed yet. OAuth enrollment/user revocation remain separate
under Q30/Q45/Q46. #2662 stays open/In Progress until main merge.

Modernization, stable formatting, CGO-disabled build and make test-all pass:
zero-issue lint, full race unit/integration/harness checks, nine SDK patch
tests, 140 metrics tests and installer checks. The ordered log is
static-vault-write-foundation-gates.log. No existing E2E scenario or replay
membership/cassette is changed. The actual-helper lifecycle above verifies the
new writes; the previous twenty-four CLI scenarios are not rerun for this
internal-only foundation.

## 58. Static shared Vault credential declarations

Static shared credentials now integrate through registration/schema, grouped
and nested loading, inherited namespace/collection scope, planning, saved
execution and public dump. Grouped identity.vault_provider_credentials entries
select a declared provider; nested shared_credentials sit under that provider.
Only static_secret/shared providers beneath explicitly managed directories are
accepted. External directories do not grant mutation authority. There is no
child labels/managed_by field, and none is invented. Ref remains local.

Matching uses the observed default or nonnegative priority slot per provider.
Selectors can repeat and are not monikers. Every observed collection must be
complete and unambiguous; unexpected user/pending variants stop static writes.
Changing ref preserves a matched record. Changing priority creates another
record; scoped sync creates replacement slots before retiring omitted slots.
Changing a selector at its existing priority is an update requiring explicit
secret selection, because the native PUT requires material. Secret-only writes
preserve routing and carry the same ownership/protection context as ordinary
changes. Saved routing integers retain exact client-side precision.

Create requires a deferred !secret source. Ordinary reapplication does not
compare or rewrite material. Selected replacement uses the established secret
intent/preflight pipeline, with no value in plans, diffs, dump or artifacts.
Fresh execution verifies directory UUID/name/namespace/owner/protection and
Vault enablement, provider UUID/name/static mode, target UUID/routing and actual
encryption-key availability. Missing sources fail before mutation. All writes
make one attempt and verify public receipts/readback; runtime material recovery
is not inferred from metadata.

Directory, provider and encryption-key creates precede credential writes.
Planning requires a registered or planned retained key. Key retirements follow
credential writes so a plan cannot discard its own encryption prerequisite.
Explicit credential deletions may precede provider retirement, whose saved
execution still requires actual emptiness. A directory/provider declaration
alone grants no credential retirement authority. Late undeclared attachments
block the parent. Client checks cannot close the final native attachment/delete
race documented in Q41.

Dump traverses static credentials only under enabled managed directories and
exports public routing without secret values/source bindings. Reload matches
existing slots with no source; recreation requires adding one. OAuth/user
authorizations are not adopted or pruned. The default-directory example and
tutorial cover preparation/adoption, missing-directory bootstrap, deferred
environment/file sources, explicit replacement, scoped sync, public inspection,
dump and retirement. The metadata ledger and implementation guide are updated.

The first complete US .tech CLI lifecycle passes in 31.09s. The expanded final
isolated run adds local-ref identity and late-credential parent guards. It
also verifies all public routing fields, saved plans, missing-source preflight,
namespace/protection/routing/key changes, selector writes, priority replacement
ordering, omission, grouped declarations, dump/reload, scoped pruning, ordered
deletion and creation from no directory. Test source directories are removed;
commands/traces/artifacts exclude material and enrollment URLs.

Initial scenario attempts corrected explicit explain discovery, the test's
working directory for deferred file boundaries, and executing sync/delete plans
through their matching verbs. The repository lint pass also exposed a model
dependency causing a test-only import cycle; model validation now stays local.
These were implementation/test issues, not new API defects. Q30/Q45/Q46 remain
the API-only runtime and formal contract questions. Compatibility notes include
the conditions for revisiting explicit selector-write material and ownership.

SDK remains 90596b498bcb. Existing methods cover this slice; no SDK revision,
release or PR is required. #2662 remains open/In Progress pending main merge;
shared OAuth enrollment and user revocation remain separate work.

Final Identity regression passes all twenty-five live scenarios together in
493.526s, including the new credential lifecycle in 27.19s. After moving its
private file source/workspace outside artifact inputs, the final isolated
scenario passes in 27.78s. An earlier isolated attempt exceeded the harness's
one-minute subprocess default and was killed before cleanup; the harness
correctly refused to delete its nonempty directory. Process inspection confirmed
no remaining fixture process, then scoped recovery removed three credentials,
one provider and one key, restored the original default and removed its private
workspace. The scenario now has a four-minute command budget. No new API defect
was inferred from this timeout.

Early stop after directory creation passes cleanup in 2.18s. Independent artifact
verification confirms twenty-five successful outcomes, removal of eight existing
private capture directories, final private-source workspace removal, actual
default bootstrap and absence of the administrative token in artifacts. The
scenario checks its material values and their base64 forms before copying only
public files into artifacts. Final inventory confirms an unmanaged default with
original settings restored and empty principal/group/IdP/provider/key/auth-server
inventories.

Modernization, stable formatting, CGO-disabled build and full make test-all pass:
zero-issue lint, race unit/integration and harness checks, nine SDK patch tests,
140 metrics tests and installer checks. Precision regression covers saved
routing integers above the exact float64 range. The default cassette check
passes for control-plane/get; no existing replay membership/cassette changes.
The new scenario remains live-only.

Evidence under the local cache is static-vault-credentials-all/
20261031-222311, static-vault-credentials-private-v2/20261031-223633,
static-vault-credentials-final-gates.log,
static-vault-credentials-artifact-verification.json,
static-vault-credentials-stopped.log,
static-vault-credentials-timeout-recovery.json and
static-vault-credentials-final-inventory.json.

## 59. Explicit shared OAuth enrollment and credential retirement

Shared OAuth providers now expose deliberate pending-credential creation,
enrollment-URL issuance and shared-record retirement. Provider configuration
and encryption keys retain their declarative lifecycle. Consent is an explicit
external workflow; a captured URL never means completed authorization.

The new create identity vault-provider-credential command accepts a shared
authorization_code provider, optional paired selector/priority and a required
new private capture file. Create identity vault-provider-enrollment takes an
existing credential UUID and deliberately invokes the capability-producing
detail GET. Both operations reserve an exclusive 0600 file before that request,
make one attempt through both retry layers and save enrollment_url only in the
private file. Normal JSON/YAML/text contains public metadata. Ordinary get/list
continue to use collections, never that detail GET. Pending creation does not
require a registered key in the tested API; successful consent/runtime use is
not established by that administrative behavior.

Fresh complete public observations establish provider UUID/name/shared mode,
template kind, target membership and unambiguous default/priority slots before
issuance. Create rejects occupied slots and static/user providers. Reissuance
requires an existing shared OAuth record and a different filename. Response
identity, routing and timestamps, then independent public collection readback,
are checked. URLs are captured before metadata validation so an inconsistent
receipt does not silently discard an issued capability. Uncertain requests or
capture failures retain the reserved file and never delete a remote record
automatically. Errors withhold capability-bearing API bodies.

Delete identity vault-provider-credential selects one shared static or OAuth
UUID, uses normal confirmation, rechecks provider membership and verifies its
absence through the public collection. It never issues an enrollment URL or
deletes user authorizations. Like other explicit secret commands, these
operations use API authorization without stamping ownership or enforcing
declarative namespace/protection. No OAuth credential is adopted, dumped as a
declaration or pruned through static credential sync. Provider/directory
retirement still requires actual empty credential inventories.

Native US .tech probes verify pending creation without keys, default and
priority-zero slots, repeated URL-producing detail GETs, identical complete
public metadata across those GETs and 404 after record deletion. Seven returned
capabilities stay in memory only; probe output excludes them, supplied material
and the administration token. All fixtures are removed and original unmanaged
default settings restored. Q47 asks API-only questions about URL lifetime,
reuse, invalidation and consent prerequisites. Active OAuth authorization
continuity, upstream invalidation and routing PUT remain unverified under
Q30/Q46; no operation invents runtime mTLS context or follows a URL.

The first live CLI scenario passes in 12.85s, covering private URL capture,
pending metadata, routed creation, public pagination, type/slot/file guards,
explicit reissuance and static/OAuth retirement. Its private files are outside
artifact inputs and removed before checked public files are copied. The
published default-directory example/tutorial, metadata ledger and compatibility
tracker explain these boundaries. SDK remains 90596b498bcb; existing methods
cover this slice, with no SDK release or PR.

Final regression passes all twenty-six live Identity scenarios together in
US .tech (500.380s), including the expanded enrollment lifecycle in 11.45s.
It additionally checks wrong-parent issuance/deletion against another existing
OAuth provider. Early stop immediately after directory creation passes cleanup
in 2.15s. Independent verification confirms all twenty-six outcomes, removal
of eight existing private capture directories and two new private workspaces,
the enrollment scenario's ten public assertions, actual default bootstrap and
absence of the administration token from artifacts. The lifecycle scans all
generated public files for its material, enrollment URLs/query capabilities and
their base64 forms before copying them into artifacts.

Ordered modernization, stable formatting, CGO-disabled build and full
make test-all pass: zero-issue lint, race unit/integration/harness coverage,
nine SDK patch tests, 140 metrics tests and installer checks. The default replay
check passes for control-plane/get; existing replay membership/cassettes are
unchanged and the new scenario is live-only. Evidence in the local cache is
shared-oauth-enrollment-contracts.json, vault-enrollment-all/20261031-230814,
vault-enrollment-final-gates.log, vault-enrollment-artifact-verification.json
and vault-enrollment-stopped.log. Final independent inventory in
vault-enrollment-final-inventory.json confirms the unmanaged default's original
public settings are restored and principal/group/IdP/provider/key/auth-server
inventories are empty. #2662 stays open/In Progress until main merge; actual
user enrollment/revocation and active authorization continuity remain separate
unfinished work.

## 60. Organization-chart contracts and principal ownership boundary

Started #2665 on ki-triage and marked it In Progress in project 79 before the
evaluation. Direct US .tech multipart dry-run probes establish five required
CSV headers: external_id, email, given_name, family_name and
manager_external_id. A blank manager identifies the tested root. Dry-run
reports user/team/membership counts without changing the directory. The
specification currently calls the CSV canonical without listing these columns.

A completed 65-request native lifecycle creates three users, two teams and
three memberships, preserves IDs on identical re-import and reports unchanged
users. A manager change updates memberships and removes the emptied team.
Omitted cost_center preserves its metadata value; a blank cell removes it.
Absent imported users become inactive while an unrelated manual user remains
unchanged. A duplicate email/external-ID file returns HTTP 200 with row errors
and zero counts; the full inventory remains unchanged. Header-only input gets
400 with a 1–50000-principal requirement. Q48 asks for the canonical format,
normalization and whole-file/source-scope contracts.

Import also adopts an existing user by reserved email identity. It retains
UUID, managed_by, protection markers and unrelated metadata, changes display
name and sets provisioned_by to imported. Ordinary principal PUT and DELETE
return 409; importer-reserved identity deletion returns 400. Explicit native
active:false PATCH changes provisioning to konnect_user, after which individual
deletion works. The completed probe removes every fixture through import
reconciliation/deactivation and individual retirement, leaving directory
settings and timestamp unchanged. Q49 asks for adoption precedence and the
supported release/re-import lifecycle. Opaque map entries are not assumed to
be a backend-enforced protection policy.

An earlier probe expected row validation to use HTTP 400 and stopped at the
valid HTTP 200/error-report response. Its cleanup did not yet know the
deactivation release path. After confirming that process was terminal and all
remaining users had the synthetic probe identities, guarded native recovery
removed its three users through an explicitly authorized directory cascade and
restored the original unmanaged default settings. The corrected complete probe
uses no directory cascade. No permanent product force-delete path was added.

Principal declaration planning, scoped pruning, adoption and saved mutation
execution now reject imported provisioning even when old ownership markers
survive. Omission without collection authority leaves those records alone;
ordinary unowned imported state is still excluded by ownership filtering.
Directory dump emits imported users as external UUIDs instead of recreatable
managed declarations. Independently owned children keep their own authority.
This guard does not deactivate, release or transfer a user automatically.

Q50 records two public-schema differences: imported remains under x-enum-dev
despite being returned by the public import operation, and counts.users.adopted
is returned without an inspected schema field. The generated Go string enum
preserves unknown values; native CLI inspection verifies that behavior. No SDK
revision is needed for this guard; typed import reporting must account for the
missing adoption count. The native description's hris_import wording differs
from returned imported. No public org-root read is promoted by this work.

Focused planner/executor/adoption/dump tests pass. The first live boundary
scenario failed because its JSON-formatted declarations used a .json extension;
the loader requires .yaml/.yml. After correcting filenames, it passes in
11.657s. It verifies native adoption, fresh apply/delete/scoped-sync/no-op
rejection, saved update/delete rejection, adoption refusal, user/team reads,
external dump/replan and complete unchanged public state across rejected
operations. HR rows, command results and traces remain in a private temporary
workspace that is removed; only boolean assertions and a path/removal receipt
are published.

Final full regression passes all twenty-seven Identity live scenarios together
in US .tech (512.554s), including the new boundary in 12.13s. The final isolated
scenario passes in 12.734s after matching every desired public principal field
to the imported record; even that otherwise no-op declaration is rejected.
The published example CSV is actually uploaded with dry_run=true and produces
three-user/two-team counts without changing inventories. Independent artifact
checks confirm all twenty-seven outcomes, eight old private capture
directories and two Vault workspaces removed, a separately verified absent HR
workspace, no copied HR files, and no administrative token or synthetic HR
values in public boundary artifacts. The isolated final run independently
checks the same privacy/removal properties.

Ordered modernization, stable formatting, CGO-disabled build and full
make test-all pass: zero-issue lint, full race unit/integration/harness checks,
nine SDK patch tests, 140 metrics tests and installer checks. The default replay
check passes for control-plane/get; no replay membership/cassette is changed.
The new scenario is live-only. Final evidence includes
org-chart-boundary-all/20261031-233546,
org-chart-boundary-final/20261031-234436, org-chart-boundary-gates.log and
org-chart-boundary-artifact-verification.json in the local cache.
Early stop immediately after directory creation passes guarded cleanup in
2.268s; org-chart-boundary-stopped.log records that check. The final isolated
privacy evidence is org-chart-boundary-final-artifact-verification.json.
Independent final inventory in org-chart-boundary-final-inventory.json confirms
the unmanaged default's original public settings are restored and
principal/group/IdP/provider/key/auth-server inventories are empty.

The synthetic org-chart.csv example and org-chart.md explain native import
versus ordinary declarative configuration. Metadata and compatibility ledgers
record the provisioning override. This foundation does not expose an import
command, claim chart CRUD or complete arbitrary principal CSV import #1419.
Explicit import upload/reporting remains the next #2665 integration, including
single-attempt uploads, private diagnostic handling and full-file intent.

Evidence in the local cache: org-chart-dry-run-contracts.jsonl,
org-chart-lifecycle-contracts.json, org-chart-probe-recovery.json and
org-chart-boundary-second/20261031-233508. Keep issues open/In Progress until
main merge. No SDK release or PR.

## 61. Explicit full-file organization-chart import

Create identity org-chart-import now calls the native multipart import
operation through the existing public SDK preview. Exactly one of --dry-run
and --execute is required, with explicit directory name/UUID, a regular CSV
file and a new private report path. Local preflight checks UTF-8, CSV syntax
and the observed 1–50000 data-row bound. Canonical column, identity and hierarchy
validation stays with the native full-file endpoint. The client does not loop
over rows through principal CRUD or invent an automatically reconciled import
resource. Existing default directories need no adoption; missing-default
bootstrap remains a separate step.

Both SDK and transport retries are disabled for preview and execution. A new
exclusive 0600 report file is reserved before the POST, with directory UUID,
mode and an exact-byte SHA-256 for local correlation. Complete native reports
are captured before interpretation, preserving unknown fields and the observed
adoption count missing from the typed SDK. Available SDK API-error bodies or
transport diagnostics stay private; unexpected non-JSON bodies are retained
as base64. Incomplete/unrecoverable reads may leave only request context.
Uncertainty never triggers another upload, rollback or automatic resource
retirement. The hash is not remote ownership, local resource state or a
transaction guarantee.

Public JSON/YAML/text contains counts and value-free diagnostic rows/types/
fields. Native warning codes principal_adopted, email_changed, quota_shortfall
and inactive_not_reactivated are exposed without detailed text. Error fields
are restricted to canonical CSV columns; unknown types/fields become other and
invalid/out-of-range row numbers are omitted. Native report messages and extra
fields remain private. Import request/response bodies and read/transport
diagnostics are wholly redacted in traces, including error responses.
HTTP 200 with row errors still prints a safe summary and exits unsuccessfully
after capture. No general runtime rollback guarantee is inferred.

The first CLI lifecycle passes in 13.730s. It verifies a nonmutating preview,
adoption with retained foreign managed_by/metadata, complete private warnings,
text/JSON/YAML and explicit product selection, stable re-import IDs, manager
moves/team pruning, cost-center omission/clearing, departures, inactive returns,
full-file row errors, UTF-8 and intent/file guards. Input files and reports are
outside artifact inputs and deleted before checked public logs/output are
copied. Fixtures are explicitly reconciled/released/retired; no product force
delete or automatic active change is introduced.

Full Identity regression passes all twenty-eight live scenarios together in
540.760s, with the new workflow in 12.98s. Final isolated verification passes in
16.388s after preserving available raw SDK error bodies rather than only their
formatted message; a real HTTP 400 report is checked privately. Unknown/non-JSON
bodies and uncertain transport failures are covered by actual SDK wire tests,
including both retry layers enabled outside the operation and exactly one
attempt. Error captures with no recoverable body retain their request context.
Early stop immediately after directory creation passes cleanup in 2.291s.

Independent artifact checks confirm all twenty-eight successful outcomes,
removal of eight existing private capture directories and four private
workspaces, no copied private HR sources/reports and no administrative token
in artifacts. The workflow checks its synthetic cell values and base64 forms
before copying public traces/output; final isolated verification checks its
absent workspace, safe notices and 114 public files for the administration
token. Independent final inventory confirms original unmanaged default public
settings restored and empty principal/group/IdP/provider/key/auth-server
collections.

Ordered modernization, stable formatting, CGO-disabled build and full
make test-all pass: zero-issue lint, race unit/integration/harness checks,
nine SDK patch tests, 140 metrics tests and installer checks. Default cassette
validation passes for control-plane/get; existing replay membership/cassettes
are unchanged and the new scenario is live-only. The initial gate attempt
corrected a test EOF comparison and two long test lines; these were local
implementation issues, not API defects. Evidence is org-import-all/
20261101-001050, org-import-final/20261101-002422, org-import-gates.log,
org-import-artifact-verification.json,
org-import-final-artifact-verification.json, org-import-stopped.log and
org-import-final-inventory.json under the local cache.

A 73-request native extension establishes that reintroducing a departed user
preserves the UUID and restores membership, but retains active:false and
konnect_user provisioning. The API emits inactive_not_reactivated and counts
the person as unchanged. Q51 asks for the intended return/reactivation contract;
the command surfaces the warning without patching active automatically. The
native probe removes all fixtures individually and leaves directory settings
and timestamp unchanged.

The example/tutorial, metadata ledger and compatibility conditions describe
full-file authority, private reports, explicit intent and uncertainty. The
operation stamps no KongCTL metadata and does not enforce declarative parent
namespace/protection. Earlier principal guards remain: current imported state
overrides retained markers, while dump uses external UUIDs. Preview is not a
saved plan. This completes an operational importer slice, not general CSV
import #1419, chart CRUD or a public org-root read. SDK stays 90596b498bcb;
no SDK revision/release or PR is needed. #2665 remains open/In Progress until
main merge. Q48–Q51 retain API contract/schema boundaries.

## 62. Principal and group API-key lifecycle

Work under #1417/#2659 verifies direct US .tech contracts using three principal
types and two ordinary groups. Generated creation retains the five ownership
entries and expiration; duplicate management tuples are accepted and detected
across three pages. Imported creation rejects managed_by, and full replacement
or per-key null PATCH leaves existing maps unchanged. Omitted expiration clears
it; explicit dates/null work. Wrong existing parent GET/PATCH/DELETE return 404
without changing the original key. All keys and parents are retired, leaving
default directory settings and timestamp unchanged. Evidence is
api-key-lifecycle-contracts.json in the local cache.

Principal generated/imported scalar names work, while null clearing returns
200 and retains the previous value. Generated group creation retains a name;
imported group naming and all group name PATCH changes are ignored. Q28/Q29
now include November 1 observations and short HTTP routes. Questions remain
API-centered and do not presume the API enforces opaque management markers.

Shared SDK-backed helpers and explicit create/patch/delete commands now cover
both parents, with public get/list and full cursor traversal. Creation requires
a new exclusive 0600 capture, reserves it before POST, disables both retry
layers and saves generated/imported material before public verification.
Public response/read-back mismatch, missing material and uncertain errors
never replay creation or delete issued credentials. Captures record the
directory, principal/group and known key UUIDs. Imported material uses the
existing secret-env/file source pattern. No ownership stamp is added.

PATCH only supports expiration and scalar principal names. Fresh observations
hydrate omitted expiration; requests always send a timestamp/null to preserve
the intended state. Public receipts and fresh read-back must retain management
metadata and creation identity. Unsupported group naming is rejected before
mutation. Explicit deletion requires confirmation, scoped collection membership
and observed absence. Key route bodies/read/transport diagnostics are wholly
redacted, including arbitrary error bodies. Text detail now honors text output.

The first new lifecycle scenario failed local group-name validation because
its fixture names contained spaces. Correct alphanumeric names pass the full
scenario in 19.61s. It covers generated/imported capture, environment/file
sources, three actual pages under both parent types, supported updates,
wrong-parent rejection, rotation/retirement, arbitrary map preservation,
declarative parent no-op/deletion guards, output formats and private workspace
removal. Fixtures and captures are retired before publishing checked outputs.
The default directory is restored unmanaged after isolated verification.

api-key-parents.yaml supplies ordinary group/principal declarations under the
external default directory. The expanded tutorial describes explicit key
lifecycle and cleanup. Generated-key declarations, inherited/fixed ownership
policy, saved plans, dump and authorized parent ordering remain integration
work; imported ownership/adoption remains unavailable. No runtime continuity
or revocation claim is made without Q30's supported setup. SDK remains
90596b498bcb; no SDK change/release or PR is required for these commands.

Review also found that the older key reader could treat missing pagination
metadata as a terminal page through generated zero values. Shared key reads now
parse the public raw collection and require data, meta/page, positive size and
an explicit next cursor/null before accepting it. Missing components fail the
whole observation. This is a client observation guard, not a new API defect.
Focused actual-SDK tests cover missing data/meta/page/size/next, invalid IDs,
failed pages, duplicate IDs/cursors, capture/uncertainty and scoped mutation.

Full Identity regression passes all twenty-nine live scenarios together in
563.529s, including the new lifecycle in 19.82s. Final isolated verification
after the strict pagination change passes in 25.83s. Independent checks confirm
all outcomes, eight existing private capture directories and five private
workspaces removed, no copied key sources/captures and no administrative token
or its base64 form in artifacts. Both parent collections actually traverse
three pages with bracketed size/after parameters. Final isolated privacy
verification independently checks the same key workflow and removed workspace.

Early stop after directory creation passes guarded cleanup in 0.94s. The
published api-key-parents.yaml plans exactly one principal and one ordinary
group through the external default directory. Independent final inventory
confirms the unmanaged default settings restored, with empty principal, group,
Trusted IdP, provider, Vault-key and auth-server collections. Evidence under the
local cache includes api-key-writes-all/20261101-004959,
api-key-writes-final/20261101-005933, api-key-writes-stopped.log,
api-key-writes-all-artifact-verification.json,
api-key-writes-final-artifact-verification.json,
api-key-writes-example-plan.json and api-key-writes-final-inventory.json.

Final ordered modernization, stable formatting, CGO-disabled build and full
make test-all pass, including zero-issue lint, unit/integration/harness race
checks, nine SDK patch tests, 140 metrics tests and installer checks. Final
results/log are api-key-writes-final-gates.json/.log. Default replay validation
passes for control-plane/get; all twenty enabled memberships/cassettes are
unchanged and the new scenario is live-only. #1417/#2659 remain open and
project 79 In Progress until merge to main. No implementation PR or SDK
release is created.

## 63. Generated-key declarative authority and capture preparation

The explicit key workflow is committed and pushed as 83a98b1d. The next feasible
declarative design uses the existing ordinary-child rule: current namespace and
update/delete protection come from a declared managed principal or ordinary
group. The default directory may remain external. Key-level adoption and
independent protection changes are not invented over an immutable map.

The key map will contain tool, creator version, kongctl-ref and
kongctl-create-type:v1. The first three provide provenance/local lookup;
creation intent must be recorded explicitly because key metadata does not
reveal generated versus imported creation. Namespace/protection are resolved
through the parent rather than storing stale copies. Version records issuance
for these immutable maps, while existing mutable resources retain last-write
provenance. Required manifest type:v1 remains explicit. Imported bodies are
outside this generated declaration; normal inspection still infers no type.

A native US .tech probe verifies four-entry retention under both parent types,
mutable parent namespace/protection without key-map changes, expiration changes
preserving markers and duplicate-ref detection across two pages. It removes
every key/parent individually and leaves the entire original default directory
object unchanged. Evidence: generated-key-parent-authority.json in the cache.
This is a convention stored in the API, not API-enforced ownership or uniqueness.

Metadata registration now supports persisted refs for inherited-namespace
resources, fixed creation markers and create-only stamping. Reserved ownership
keys cannot be overridden by fixed markers, capacity includes all generated
entries, and marker maps are cloned. Plan updates skip immutable map stamping;
existing mutable policies keep their behavior. No resource is enrolled into the
new policy until its full integration is ready.

The existing key helper now accepts a private file reserved by whole-plan
preflight. Before POST it binds resolved directory/parent IDs while preserving
local ref/namespace/parent-ref capture context and discarding unrelated IDs.
Material is captured before read-back. Nil, closed, replaced or completed files
fail before another create; unsupported input leaves a reservation available
for the caller to discard. Explicit CLI creation reuses the same path and still
reserves its own new 0600 file. No SDK change is required.

Remaining enrollment must deliver both key kinds through resource registration,
loading, planning, state, execution, dump, explain/scaffold, examples and E2E.
Exact parent/tool/ref/creation-marker matching must reject duplicate candidates
and collisions with incompatible management shapes. Scope authority must
distinguish omitted versus empty key collections and preserve all foreign or
unrecognized keys. Every saved mutation must repeat parent management,
provisioning, namespace/protection and key identity checks. Explicitly
authorized child retirements must run before fresh parent-empty checks.
Uncertain issuance must never replay creation, match material or automatically
delete the issued key. This preparation does not expose key declarations yet.

When a sync adds replacement refs and retires old refs under one parent, old
key retirement must depend on successful replacement issuance/capture. A
failed create must preserve the existing key rather than relying on incidental
change order. Explicitly empty collections remain deliberate retirement intent.

Focused actual-SDK tests verify pre-reserved capture under both parents,
resolved ID binding, public-only results and capture before read-back. Closed,
replaced, nil and already completed captures issue no new key; unused
unsupported-input reservations can still be discarded. Metadata tests verify
inherited ref retention, fixed intent, creator-version behavior, capacity,
reserved-key rejection, caller-map isolation and unchanged mutable policies.
The initial lint pass required a tagged switch in a new test; no API change
was needed.

Modernization, stable formatting, CGO-disabled build and full make test-all
pass, including zero-issue lint, unit/integration/harness race checks, nine SDK
patch tests, 140 metrics tests and installer checks. All twenty-nine Identity
live scenarios pass together on the frozen build in 542.312s; the existing key
workflow passes in 21.45s. Independent checks confirm all outcomes, eight
private capture directories and five workspaces removed, no copied private key
sources/captures and no administrative token or base64 form in artifacts.
Default settings are restored unmanaged and principal/group/IdP/provider/key/
auth-server inventories are empty. Replay membership/cassettes are unchanged;
the default control-plane/get validation passes. No new scenario or SDK pin.

Evidence: generated-key-preparation-all/20261101-011923,
generated-key-preparation-all-artifact-verification.json,
generated-key-preparation-final-inventory.json and
generated-key-preparation-gates.json/.log under the local cache. The native
parent-authority proof is separate from the existing command E2E; full
declarative-key lifecycle proof is still required before exposing those kinds.

## 64. Generated principal and group API-key declarations

Both generated key kinds are integrated through registration, nested/top-level
loading, namespace selection, reference/sync scope, planning, state, saved
execution, dump and explain/scaffold. The example and tutorial are
docs/examples/declarative/identity/generated-api-keys.yaml and
generated-api-keys.md. They use an external lookup for the existing default
directory and manage the principal/group parents.

Required type:v1 records generated creation intent. The key's canonical map
contains tool, creator version, kongctl-ref and kongctl-create-type:v1. Current
managed-parent namespace and update/delete protection supply authority; no
independent key namespace/protection is stored or exposed. Imported and legacy
keys are not silently adopted. Complete parent-scoped scans reject duplicate
refs, incompatible marker shapes and legacy collisions before mutation.

Saved writes repeat parent provisioning, tool/ref/name, namespace/protection
and scoped key UUID/ref checks. The generic executor has an optional scoped-ID
lookup contract so these resources retain duplicate/authority errors instead
of falling back to names that cannot identify them. Existing resource lookup
policies are unchanged. Key creation depends on new parent/directory creates.

Whole-plan preflight reserves exclusive private captures before any mutation.
Resolved parent IDs are bound before one generated POST; material is captured
before public read-back. Uncertain creation is never automatically replayed or
retired. Supported principal scalar names and expiration updates preserve all
immutable markers. Omitted expiration is hydrated because native PATCH clears
it; explicit null clears it. Group names stay creation-only and null name
clearing is rejected under Q29.

Omitted collections preserve keys. Explicit empty sync scopes retire only the
recognized generated convention. Foreign/imported/legacy records are retained
and prevent parent retirement. Final plans require explicitly authorized
attached-key deletes before parent deletion, followed by fresh empty checks.
During replacement sync, retirement depends on successful replacement creation
and capture. Failed replacement preserves the old keys. Q24's lack of an
atomic API conditional-delete precondition is still an engineering question.

Dump exports recognized generated keys only beneath verified managed parents,
including nullable expiration and explicit creation intent. It omits material,
capture paths and management maps. Reloading a dump produces no changes.
The reflection schema treats time.Time as a scalar, matching its JSON form;
the loader tests caught and corrected the initial object-shape description.

Focused actual-SDK tests cover both kinds, new-parent resolution, capture
preflight/failure, stale creates, expiration hydration/null, stable creator
version, inherited authority, duplicate/legacy refs, wrong UUIDs and importer
ownership, scoped pruning, replacement/retirement ordering and dump/reload.
Modernization, stable formatting, the CGO-disabled build and full make test-all
pass with zero lint issues, unit/integration/harness race checks, nine SDK patch
tests, 140 metrics tests and installer checks. All thirty Identity live
scenarios pass together. Independent inspection confirms eight private capture
directories and six workspaces removed, no copied key sources/captures and no
administrative token/base64 in artifacts. Default settings are restored.

The final focused live scenario additionally verifies duplicate and legacy-ref
conflicts in both planning and saved updates after the scoped-ID refinement.
Its work and captures stay outside artifacts; only sanitized outputs, traces
and a removed-workspace receipt are published. Evidence lives under the local
cache in generated-key-declarative-all, generated-key-declarative-final and
generated-key-declarative-final-gates.log, with independent artifact receipts.
Replay membership/cassettes are unchanged and control-plane/get input/schema
validation passes. SDK remains 90596b498bcb; no SDK changes, release or PR.

API follow-up remains Q28 management-map writes/imported adoption, Q29 group
name updates/null clearing and Q30 supported runtime mTLS integration. These
are API behavior questions in docs/kong-identity-api-questions.md; this slice
does not claim runtime authentication/revocation proof. #1417/#2659 stay open
and In Progress until main merge. General CSV import #1419 and populated
principal Vault authorization/revocation #2663 remain separate work.

## 65. General principal CSV declarations and public metadata modes

Issue #1419 has a YAML collection frontend rather than a separate mutating
bulk importer. A principal collection can use !csv with a relative path and
explicit metadata_mode:merge or sync. Each record represents one principal;
public maps and nested identity/generated-key/basic-auth arrays use JSON
cells. The example uses an external directory named default. The separate
native org-chart importer retains its own CSV format and provisioning scope.

The offline inspect identity principal-csv command validates a bounded local
file and prints structural counts in text/JSON/YAML, with
remote_state_checked:false. Normal plan/diff/apply/sync/delete then use the
existing resource planners, ownership rules, capture preflight and retirement
guards. There is no second API write engine or new remote metadata convention.

CSV requires unique local refs and supports optional UUID assertions. An
assertion must match the principal already selected by directory, namespace
and persisted ref; it cannot create, adopt or retarget a resource. Duplicate
UUIDs are checked case-insensitively. Duplicate headers/refs/JSON keys,
unsupported fields, parent conflicts, malformed values and excessive JSON
nesting are rejected before API mutation, with row/column errors that withhold
cell values. Regular local UTF-8 inputs use the existing !file path/symlink
boundary and size limit. CSV does not resolve embedded templates, environment
or secret encodings; reference-looking strings stay literal.

Blank cells omit a property. Explicit empty child arrays scope ordinary sync
retirement; an empty/header-only CSV supplies an explicit empty principal
collection. Unknown, imported and foreign resources retain the ordinary
preservation rules. Generated API-key creation requires private capture before
any planned writes. Imported key material/passwords remain in explicit
workflows under Q28/Q35, rather than being silently granted ownership through
CSV. Cross-resource references and deferred secret configuration can accompany
the CSV in ordinary YAML declarations.

Public metadata merge adds/replaces supplied keys and preserves unmentioned
keys; arrays replace their complete value. Planning previews the effective
map but saves only the patch with explicit merge intent. Execution merges it
against fresh current state, validates the complete map and preserves integer
precision and unrelated public fields before PUT. Sync replaces the map;
explicit empty sync clears it, and empty merge is a no-op. Existing YAML
without metadata_mode retains replacement behavior. These public-field modes
are separate from collection deletion through the sync command.

New saved plans use format 1.2 so an older executor rejects merge intent rather
than treating its patch as a replacement map. This binary still reads ordinary
1.0/1.1 plans. Merge intent is rejected on those versions, unsupported resource
types/actions or absent/null patches. Dump emits ordinary configuration without
CSV bindings, metadata_mode, private captures or raw management maps. Reload
replans to zero changes.

The first full live run passes the new CSV lifecycle and thirty of thirty-one
Identity scenarios. Its one failure is an existing Trusted IdP sequence:
identity POST returns 201 and the immediate parent-scoped list returns empty.
Creation is not replayed. All fixtures are cleaned and default settings are
restored. Twenty subsequent native active/inactive-user trials have no
immediate list miss and preserve the entire original default directory. Q52
records the observation and remaining uncertainty without claiming the full
surrounding trust-replacement sequence was reproduced.

Broader replay verification finds an independent planner integration bug:
apply/delete with no declared directory reads the directory collection. A
focused zero-read regression fails before the guard and passes after it.
The guard preserves declared external parents, child planning and existing
explicit sync scope. All twenty enabled replay scenarios then pass without
changing any cassette or registry membership. No SDK/API correction is needed.

Modernization, stable formatting, CGO-disabled build and corrected full
make test-all pass, including zero-issue lint, unit/integration/harness race
checks, nine SDK patch tests, 140 metrics tests and installer checks. All
thirty-one Identity live scenarios pass together in 598.552s; the new CSV
lifecycle passes in 19.79s with twelve assertions. Independent artifact checks
verify saved merge/legacy replacement, UUID errors, dump no-op, eight removed
private capture directories, seven removed workspaces and no copied CSV/key
sources/captures or administrative token/base64. Default public settings are
restored unmanaged, and principal/group/IdP/provider/Vault-key/auth-server
inventories are empty. The successful trust sequence adds no identity POST
retry or scoped-proof bypass; Q52 still records the earlier occurrence.

Evidence is in the local cache under principal-csv-all,
identity-read-visibility-native.json,
principal-csv-replay-corrected-results.json,
principal-csv-corrected-final-gates.json/.log and
principal-csv-corrected-all/20261101-021808, with independent
artifact/declarative-key/CSV verification receipts and the final inventory.
The example/tutorial are principal-csv.yaml, principals.csv and principal-csv.md
under docs/examples/declarative/identity. The metadata ledger and compatibility
tracker record client-only intent and removal criteria. SDK remains
90596b498bcb; no SDK change, release or PR.

## 66. Principal group inspection contracts and SDK cursor preparation

The remaining-requirements audit finds a specific #2658 gap: principal detail
shows group IDs, but the dedicated principal-group collection has no CLI reader.
That collection also exposes direct/inherited status, nullable membership-entry
ID, group type, provisioning and team manager. It is a read projection, not a
new desired-state membership type or an ownership store.

Native US .tech HTTP probes verify two ordinary direct groups, disjoint
page-size-one reads, empty results for an unrelated principal and 404 for a
missing principal. An actual imported root/manager/worker hierarchy returns one
direct and one inherited team for the worker. Both retain imported provisioning
and manager data; only the direct membership has a membership-entry ID. No team
membership mutation authority is inferred. Fixture cleanup removes imported
teams through the known complete-file workflow, then retires only the known
test principals and preserves the entire original unmanaged default directory.
CSV rows and import reports stay in memory and are not saved in evidence.

Pagination metadata contains a next-page URL. The existing CursorTracker
extracts its page[after] value and keeps requests on the originally selected
SDK route. The probe initially submitted the complete URL as an opaque cursor;
the resulting 400 is a probe error, not an API defect. Correct extraction
produces two disjoint pages and a terminal null cursor. Flat size=1 is ignored,
confirming Q22 applies to this operation too.

The canonical SDK patch manifest now corrects ListPrincipalGroups to the same
bracketed query encoding as the other sixteen Identity lists. The expanded
actual-SDK wire regression fails only this new operation before the correction
and passes all seventeen afterward. The local generated delta is exactly one
request-tag change plus the new regression case; no public component fields or
unrelated resource APIs change. Nine patch-tool tests, all actual-SDK
package/model/internal/pkg tests and the seventeen-case wire check pass without
skips. Modernization, stable formatting, CGO-disabled consumer build and full
make test-all pass, including zero lint issues and unit/integration/harness
race checks. Existing live/replay evidence remains the 5f65cd87 checkpoint;
this preparation does not yet expose a new command.

Evidence: principal-group-native-contracts.json and principal-groups-sdk-wire.log
in the local cache. The source bundle must be committed before SDK snapshot
preparation so provenance identifies an exact source revision. Pushing that
preview update, consuming its pin and completing the reader/examples/live E2E
are subsequent work. The current consumer remains on SDK 90596b498bcb; #2658
is open/In Progress and no implementation PR or SDK release is opened.

## 67. Principal group inspection and per-operation coverage

The #2658 acceptance gap is closed on the working branch. Get/list identity
principal-group(s) accepts an explicit directory ID/name and principal UUID.
It reads the principal's complete group projection; optional selection uses
group UUID or exact name. Entry IDs do not select groups, ambiguous names fail
and an unrelated directory-level group cannot satisfy a missing association.
There is no invented association detail endpoint or global lookup fallback.

Public output retains group type, direct status, nullable membership-entry ID,
provisioning and manager ID/display-name/email. Missing optional provisioning
fields do not acquire SDK defaults. A missing direct field, inconsistent entry
ID/status, duplicate group/cursor, malformed page or failed later page returns
an error without partial output. Next-link cursors keep requests on the selected
API host and directory/principal route. JSON/YAML and text summary/detail are
implemented; text detail includes manager fields without falling back to JSON.

This projection has no labels or managed_by. Inspection stamps no ownership
metadata and grants no authority over direct importer-managed links or inherited
memberships. Ordinary direct declarations and complete-file organization-chart
import retain their existing separate write boundaries. The example/tutorial is
principal-groups.md alongside groups.md and memberships.md; no new declarative
resource type or membership adoption path is introduced.

Public SDK preview 37580cf8cdf591506b2e9fd55d28e917b1dcac35 is pushed and consumed
as v0.79.1-0.20261101084607-37580cf8cdf5. Its snapshot identifies patch source
5680da0f5da0a6a26a0a6e8f758da5d888a64553 and verifies all 23 bundle hashes.
The generated-code delta from 90596b498bcb is exactly one query-tag line in
models/operations/listprincipalgroups.go, with no component-model changes.
The remaining five changed files are tests or patch snapshot/provenance.
Post-generation application is idempotent. All seventeen cursor-wire methods
pass without skips; the previous preview fails only the new method. SDK
package/model/internal/pkg tests and root/internal race tests pass. Q22 records
the additional route using the same shared query-annotation issue; there is no
new API defect or unrelated internal/public SDK compatibility change.

The contributor API/SDK coverage matrix now inventories all 112 operations in
the selected preview's auth-server/directory/template surfaces. Every generated
operation ID maps to an actual SDK service method. Ninety-seven production
methods have positive deployed evidence: captured HTTP success or six named
live scenario assertion sets. Fifteen SDK-only operations are explicitly
dispositioned as alternate methods, optional batch/follow-on work or the pending
principal Vault revocation. Symbol availability is not presented as lifecycle
support. The matrix keeps still-gated/runtime surfaces distinct and supplies
the minimum verified current dependency for #1399/#1413.

The focused new live scenario passes in 11.70s. All thirty-two Identity live
scenarios then pass together in 616.372s; principal-group inspection takes
11.57s with eleven assertions covering all three output formats. Two ordinary
direct memberships and a real imported hierarchy each traverse two SDK pages.
Checks cover group versus entry IDs, parent isolation, empty/missing cases,
manager/provisioning fields and unchanged native state/ownership after reads.

HR rows/reports stay in memory. Employee-bearing CLI outputs/traces remain in
a private temporary workspace and are destroyed before artifacts are written.
Only counts, safe page-request summaries and a removed-workspace receipt are
published. Independent inspection verifies exactly those public inputs, eleven
outcomes, two-page query shape/host/method checks, eight removed capture
directories and eight removed workspaces, with no copied CSV/key/HR outputs or
administrative token/base64. Default public settings are restored unmanaged;
principal/group/IdP/provider/Vault-key/auth-server inventories are empty.

Modernization, stable formatting, CGO-disabled build and full make test-all
pass, including zero-issue lint, unit/integration/harness race checks, nine
patch-tool tests, 140 metrics tests and installer checks. No enabled replay
scenario, cassette or policy membership is changed. Evidence is under
principal-groups-first, principal-groups-all/20261101-025558,
principal-groups-final-gates.json/.log and the final inventory in the local
cache, with artifact/declarative-key/CSV/projection verification receipts.

Questions stay API-focused. No new question beyond Q22's added affected route
is required by this read slice. #2663 still needs a supported runtime enrollment
fixture (Q30) before populated user authorization/revocation can be verified.
No principal revocation command, secret enrollment or runtime authentication
claim is added. Issues stay open/In Progress until main merge; no PR or SDK
release is opened.

## 68. Registry Agent incoming-reference audit finding

The issue-by-issue audit finds an explicit #1414/#1415 requirement for Registry
Agent principal links tracked in #2419. This is a separate Agent Registry
surface, not AI Gateway agents. The chosen SDK has no Registry operations and
the existing directory/principal deletion checks do not inventory those links.
An empty incoming-reference observation cannot be inferred from absent SDK
coverage.

Native US .tech GET /v1/agents succeeds. A disposable logical agent and
Identity principal can be linked through POST /v1/agents/{agentId}/principals.
Native principal DELETE then returns 204; fresh GET is 404 while the agent's
principal collection still retains the original directory/principal tuple.
The association, agent and remaining test principal state are explicitly
cleaned, preserving the entire original default directory object. Evidence:
registry-agent-read-availability.json and
registry-principal-delete-contract.json in the local cache. No resource values
or credentials are saved. Q53 records the direct API behavior; directory
deletion has not been probed and is not claimed as a reproduced defect.

This is meaningful incomplete work, not a documentation-only caveat. The next
slice will add only typed SDK list-agents/list-agent-principals operations and
fresh fail-closed incoming-reference checks before principal/directory
retirement, including saved plans and references outside selected sync scope.
No Registry CRUD, automatic unlinking or association adoption is implied.
Broader Registry feature prerequisites remain #2412/#2413/#2420; the limited
read surface is needed by this explicit Identity safety requirement.

SDK experimentation is isolated in
/home/rspurgeon/dev/kong/ki-public-sdk-build/sdk-konnect-go-registry-guard on
local branch kongctl/identity-registry-guard-sdk, based on the clean verified
37580cf8cdf5 preview. Existing public SDK shapes must stay unchanged. The
consumer still uses the verified 37580cf8cdf5 pin while this guard work is
prepared. No PR or SDK release is opened.

## 69. Registry reference retirement guards

The SDK preview branch now includes two typed GETs: /v1/agents and
/v1/agents/{agentId}/principals. They come from Agent Registry v1 at
platform-api 79a35a9a281b. Frozen selected inputs, an additive transform,
namespaced schemas, reference anchors and source hashes are included. Real
preparation is idempotent. Generation initially exposed shared inline Name/Meta
collisions; direct references to explicitly named new components resolve those
without changing any existing public model. An AST comparison preserves all
61,406 pre-existing declarations, signatures and wire tags. SDK package/model/
internal/pkg tests, root/internal race tests, six preparation tests and all
seventeen cursor cases pass. The SDK is pushed at 2f51ede0670e and the consumer
pin is v0.79.1-0.20261101101317-2f51ede0670e. No release is published.

Direct US .tech probes verify three-Agent numbered pagination, linked and
empty association pages, terminal empty pages, exact target tuples and retained
page parameters. Fixtures are removed and the original default object is
unchanged. Only counts and boolean checks are saved.

KongCTL inventories every Agent and association before producing a final
principal/directory deletion plan, including scoped sync. It does not restrict
the observations by namespace or ownership. Fresh execution repeats the guard
immediately before target retirement. Matching uses directory/principal UUIDs;
a directory match blocks deletion even if its linked principal no longer
exists. Missing readers, denied/unavailable APIs and incomplete/malformed/
repeated/changing page observations block retirement. The configured regional
origin is supplied explicitly for both SDK reads. No Registry write, automatic
unlinking, adoption or new ownership convention is added. This is still a
client observation, not an atomic backend deletion guarantee (Q53).

The focused live Registry guard scenario passes in 17.18s, with ten outcomes,
13 CLI invocations and 45 Registry GETs. It covers principal delete/scoped sync,
late-link saved principal/directory execution, a dangling principal association,
preserved updates/no-op planning and successful retirement after explicit
unlinking. All HTTP requests use the configured .tech host; consumer Registry
requests are GET-only. The private workspace is removed and only count/host/
method receipts are saved. Cleanup removes fixture Agents/links/principals/
directory and the runner restores the unmanaged default settings.

Evidence is in registry-pagination-contract.json,
registry-sdk-public-api-comparison.json, registry-sdk-tests.log,
registry-sdk-race-tests.log and registry-guards-first/20261101-041722 under the
local cache. Focused planner/executor/helper tests pass, including missing
readers and failed observations. All thirty-three Identity live scenarios pass
together in 633.61s on this pin; the new Registry scenario takes 12.16s. Final
modernization, stable formatting, CGO-disabled build and full make test-all
pass, including zero-issue lint, unit/integration/harness race checks, patch
tests, metrics and installer checks.

Independent artifact checks verify ten Registry outcomes, 13 CLI invocations,
45 Registry GETs, configured hosts, preserved group cursor queries, all eight
removed capture directories and nine removed private workspaces. No private
sources/credential captures are copied and no administrative token or base64
form is present. Default public settings are restored unmanaged; principal,
group, Trusted IdP, provider, Vault-key, auth-server and Registry Agent
inventories are empty. Full evidence is registry-guards-all/20261101-042019,
its artifact/key/CSV/Registry verification receipts, the final inventory and
registry-guard-final-gates.log under the local cache.

All twenty replay inputs/cassettes remain unchanged; they are not claimed as
rerun on this new pin. The examples explain the Registry read-permission/
deployment requirement and explicit unlinking; compatibility and metadata
ledgers record its boundary. #2663 remains unverified pending Q30's runtime
fixture. This checkpoint is a stopping point for manual review, not completion
of unverified runtime behavior. Issues stay open/In Progress until main merge;
no PR or SDK release is opened and no next feature is started.

## 70. Acceptance-boundary audit after the tested checkpoint

The implementation checkpoint remains 250e65e1, with SDK 2f51ede0670e and all
33 Identity scenarios passing. This audit changes no code or live resources.
It distinguishes an external prerequisite from a client design that has not
yet been implemented; it is not a claim that every epic requirement is done.

Principal Vault revocation (#2663) still lacks a populated runtime enrollment
fixture. The SDK DELETE exists, but empty administrative collections cannot
prove positive authorization lookup, revocation or token continuity. Q30 asks
for the supported certificate/control-plane/organization setup. No internal
context headers or fabricated authorization records should substitute for it.

The earlier password boundary (#1418) predates generated API-key declarations.
Q35 proves that password metadata-only adoption cannot preserve existing
material through the tested PATCH/PUT behavior. It does not prove all new
declarative password enrollment impossible: generated/imported create retain
supplied managed_by, and explicit expiration updates work. The generated-key
implementation now uses immutable creation markers with current managed-parent
authority rather than mutable per-key namespace/protection.

A generated-only password subset could follow that convention under a managed
username. This is an inference from verified administrative behavior and
existing code, not a supported resource or a verified new password workflow.
It needs explicit generated intent, duplicate/ref matching, current username
authority/protection, complete third-level scope checks, two-slot capacity,
uncertain-create/private-capture handling, saved plans, dump/no-op/sync/delete
and CSV disposition. Existing/imported adoption, material drift and automatic
rotation are separate concerns and must not be inferred. Runtime password
authentication remains unverified under Q30 independently of administrative
lifecycle support, just as with the current administrative API-key workflows.

The password guide, metadata ledger, compatibility tracker and #1418 record
this distinction. No new engineering question is needed for an unimplemented
client design; Q35's API-only question remains accurate. No next feature is
started while the manual-testing pause preference is pending.

## 71. Default directory selection during the manual walkthrough

Directory-scoped imperative reads, adoption and credential/import commands
now select the directory named `default` when both directory flags are
omitted. Explicit names and IDs override the fallback; supplying both flags
or an explicitly empty selector is rejected. Help displays the default.
Auth-server selectors and declarative directory declarations are unchanged.
Configuration-path and environment bindings remain a separate TODO beside
the completed default-selection item in section 32.

Shared selector tests cover the fallback, explicit overrides, conflicts and
empty values. Directory lookup tests prove missing-default failure without
creation/adoption and prove that ID selection does not perform name lookup.
The rebuilt binary returns identical US `.tech` principal collections with
the implicit default, explicit default name and explicit directory UUID.
No live resources were changed for this verification.

Modernization, stable formatting, CGO-disabled build and `make test-all` pass,
including zero-issue lint, unit/integration checks and E2E harness tests.
Build/check invocations use `GOFLAGS=-buildvcs=false` because automatic Go VCS
stamping tries Git in `/home/rspurgeon` rather than this linked worktree; the
Makefile still supplies version/commit/date linker values. Local check output:
`.cache/identity-feature-walkthrough/default-directory-checks.log`.
The 33 Identity live scenarios were not rerun for this selector-only patch.

## 72. Identity command hierarchy during the manual walkthrough

`identity` now accepts `id`. Read commands expose principal identities,
API keys, basic-auth records, group associations and Vault authorizations
under `principal`; passwords sit beneath `principal basic-auth`. Groups
expose `group memberships` and `group api-keys`. Singular/plural aliases work
at each resource level. The existing flat forms retain separate command
instances and continue to work with their existing flags and paths.

Existing API-key/password create, patch and delete operations use the same
nested paths. Principal identity/basic-auth adoption also has nested forms.
These are additional routes to existing handlers, not new lifecycle support.
Directory defaults, explicit parent selectors, private capture requirements,
mutation intent and ownership/deletion guards are preserved. Identities do
not gain credential children. The examples and local walkthrough use the
nested paths.

Routing tests cover read and mutation verbs, adoption, aliases, parent hooks,
required selectors, private capture and mutation intent. Modernization,
stable formatting, CGO-disabled build and full `make test-all` pass using the
worktree VCS-stamping setting described in section 71. Read-only US `.tech`
comparisons return identical collections through the flat and nested forms
for principal identities, API keys, basic-auth records, principal groups and
group keys. Vault is disabled; the principal Vault-authorization endpoint
returns HTTP 403 through both forms, so no positive Vault read is claimed.
No live resources were changed. The 33 live Identity scenarios were not rerun;
replay scenarios and recordings are unchanged. Local check output:
`.cache/identity-feature-walkthrough/principal-command-tree-checks.log`.

## 73. Principal display-name list filtering

`get/list id principal[s] --display-name NAME` now sends an exact-match
display-name filter to the API and follows all filtered pages. All matching
principals are retained because display names are not unique. No matches
produce an empty list. An explicitly empty name or combination with a
positional UUID is rejected. The flag is command-local, not persistent,
global, configuration-bound or inherited by principal child commands.
Declarative inventory reads retain their existing unfiltered behavior.

Tests verify filter serialization, special-character encoding, pagination,
multiple matches, no matches, local flag scope and argument validation.
Read-only US `.tech` checks return one result for `On-call engineer`, zero
for `On-call` and zero for a nonexistent name. Explicit Konnect get/list
routing works; a principal API-key command rejects the flag as unknown.
Modernization, formatting, CGO-disabled build and full `make test-all` pass
with the section 71 VCS-stamping setting. No live resources were changed,
and no SDK modification was required. Local check output:
`.cache/identity-feature-walkthrough/display-name-checks.log`.

## 74. Vault read-command grouping

Get/list now support `id vault keys` and `id vault providers`, with singular
forms for detail selection. The existing flat commands remain supported.
Vault is a command namespace with no independent resource ID, lifecycle or
ownership. Directory selectors retain their default and explicit overrides.
Trusted IdP paths and declarative YAML fields are unchanged.

Routing tests cover aliases, hooks, directory fallback, key/provider selector
validation and independent flat-command flags. Modernization, stable
formatting, CGO-disabled build and full `make test-all` pass with the section
71 VCS-stamping setting. Read-only US `.tech` comparisons return the same
registered key and empty provider collection through old and new paths.
Key detail lookup by UUID and fingerprint and explicit Konnect list routing
also pass. No live resources were changed and no SDK changes were needed.
Local check output:
`.cache/identity-feature-walkthrough/vault-command-tree-checks.log`.

## 75. Proposed Vault YAML grouping — design analysis only

This section records the design before implementation. Section 76 documents
the implemented format.

The manual walkthrough is paused while evaluating a directory-level `vault:`
object. No configuration-format change has been implemented. Repository
evidence supports this direction: `identity:`, `analytics:` and
`organization:` are declaration grouping objects without their own remote
lifecycle. `portals[].assets` is a closer example of a non-resource object
nested beneath a managed resource, with explicit presence handling for its
logo/favicon children.

The proposed directory block groups `enabled`, `keys` and `providers`.
`enabled` maps to the existing directory API field `vault_enabled`. Keys,
providers and static shared credentials remain their current resource kinds,
with the directory as their existing ownership ancestor. The grouping has no
ref, remote ID, independent namespace/protection or lifecycle. Trusted IdPs
remain directory-level declarations. Provider `shared_credentials` retains
its current meaning and ownership policy.

Recommended presence contract, preserving the existing collection policy:

| Declaration | Recommended behavior |
| --- | --- |
| `vault` omitted | Preserve enablement and omit its child collections from sync scope. |
| `vault: {}` | Same intent as omission; no implicit enablement or collection pruning. |
| `vault: null` | Reject; use omission to preserve or an object to declare intent. |
| `vault.enabled: true` | Set directory enablement; omitted child collections remain unselected. |
| `vault.enabled: false` | Disable the capability; do not interpret this as deleting children. |
| `vault.keys` omitted | Preserve keys during apply and sync. |
| `vault.keys: []` | In sync, select that directory's key collection for guarded retirement; apply does not prune. |
| `vault.providers` omitted | Preserve providers during apply and sync. |
| `vault.providers: []` | In sync, select provider retirement under the existing credential/dependency guards. |

An explicit collection selects existing directory-wide authority, including
resources created outside the tool: these Vault children have no metadata to
divide their collections between administrative systems. Directory adoption
and protection remain required. A grouping object does not solve Q25's
missing metadata capability. Provider removal must not implicitly authorize
deletion of its undeclared credentials or user authorizations.

The backend's disabled-Vault visibility behavior is unchanged. An existing
disabled directory must still be enabled separately before its children can
be safely observed and planned. The current planner rejects managing a child
collection while disabling Vault; the proposed format should preserve that
guard. Declaring the directory still selects it in `delete -f`; use scoped
collection sync to retire children while retaining the directory.

Implementation is more than changing YAML tags. Current declaration discovery
finds direct registered child fields on a resource. The source-scope walk
also takes the parent ref from the map containing the leaf collection; a
`vault` grouping map would not carry the directory's ref. Both need support
for a non-resource grouping between the directory and its children, or an
explicit existing-style capture policy. Portal asset handling provides prior
art for specialized presence policies. Root grouping discovery already walks
non-resource objects, so the problem is narrower than a new lifecycle type.

Typed child extraction, validation, aggregation, namespace inheritance,
deferred secret/file handling and references need to see the same normalized
resources. Dump must emit the chosen canonical shape. Explain/scaffold/load
schema, error paths, examples and relevant E2E manifests must agree with it.
Existing resource kinds and API request fields can remain stable; preserving
the normalized saved-plan representation should avoid a gratuitous plan
format change, subject to round-trip verification. No SDK changes are implied.

Migration needs an explicit choice for both nested `directories[].vault_*`
and grouped `identity.vault_*` inputs. A single canonical preview format is
preferable to indefinite parallel syntaxes. If temporary aliases are kept,
reject mixed old/new declarations rather than silently merging them. A
grouped root collection still needs an explicit directory selector; it must
not acquire an implicit directory named default. Global `enabled` at such a
root would be ambiguous across directories.

Verification should cover omitted/empty/null grouping and collections,
enablement omission/false, disabled-Vault guards, sibling preservation,
parent/protection checks, undeclared-credential deletion blockers, grouped
forms and mixed-format errors, multi-file aggregation, secret/reference
resolution, saved plans and dump/reload/no-op. Existing enabled replay
coverage must be preserved and refreshed under the repository replay policy
if any covered scenario inputs change. This analysis changes no code, SDK,
configuration acceptance or remote resources.

## 76. Canonical Vault YAML grouping implemented

The directory-level `vault:` grouping is now implemented:

```yaml
identity:
  directories:
    - ref: default-directory
      name: default
      vault:
        enabled: true
        keys:
          - ref: encryption
            name: application-encryption
            public_key: !file ./vault-public.pem
        providers:
          - ref: upstream
            name: upstream-service
            template_name: static_secret
            credential_type: shared
```

Grouped declarations use `identity.vault.keys`, `identity.vault.providers`
and `identity.vault.provider_credentials`. They retain explicit parent
selectors. Directory enablement is declared only as
`identity.directories[].vault.enabled`.

This replaces the earlier preview input keys `vault_enabled`, `vault_keys`,
`vault_providers` and `vault_provider_credentials`; old or mixed input forms
are rejected. Examples, generated explain/scaffold paths, relevant Identity
scenario manifests and local walkthrough files use the canonical format.
Native API fixture payloads and response assertions retain `vault_enabled`.
The SDK, HTTP wire fields, normalized resource kinds and saved-plan field
names are unchanged.

There is no independent Vault resource or ownership metadata. Directory
adoption, inherited protection and existing retirement/dependency guards
continue to apply. Trusted IdPs remain directory siblings. The presence
contract in section 75 is implemented: omission and an empty grouping do not
select child collections; explicit empty child lists select that directory's
corresponding sync collection. Null groupings and null child collections are
rejected. Apply does not prune. An existing disabled Vault still needs a
separate enablement before its children can be observed and planned.

Typed grouping objects preserve existing internal resource access. Shared
declaration discovery recognizes children beneath non-resource objects.
Source-scope capture retains the owning resource ref through those objects,
so one directory's empty key list cannot scope another directory's children.
Schema validation and reserved-metadata validation follow the same paths.
Dump emits nested enablement, keys and providers; fixture tests reload the
output and produce no changes. Existing saved-plan lifecycle tests continue
to exercise key/provider/credential execution and ownership checks.

The local walkthrough's registered public key, private-key location and
existing remote resources are unchanged. Read-only US `.tech` checks show
zero-change diffs for the migrated Vault key manifest, the principal-identity
manifest and a freshly dumped default directory with children. Invalid null,
legacy and mixed input examples fail at loading. No live mutations were
performed for this format change. Updated Identity E2E scenarios still need
their next live run; the prior live checkpoint does not validate these new
inputs.

Final validation passed: Go modernization, stable formatting, CGO-disabled
build, lint, race-enabled unit tests, integration tests, installer tests,
SDK-patch tests, E2E harness tests and replay-metrics tests through
`GOFLAGS=-buildvcs=false make test-all`. All 20 enabled cassette scenarios
passed local replay, covering 2,609 recorded exchanges; their inputs,
recordings and eligibility registry were unchanged. Identity Python helpers
and YAML files parse successfully, and 21 native directory fixture payloads
retain the API's flat enablement field. The final binary is rebuilt locally.

Logs are under `.cache/identity-feature-walkthrough/`:
`vault-format-final-gates.log`, `vault-format-all-replays.log` and per-scenario
logs in `vault-replays/`. The changes are local and uncommitted.

## 77. Vault provider credential commands nested

Provider credential inspection now uses
`get|list id vault provider credential[s]`, with the existing explicit
provider selector and default-directory behavior. Singular/plural provider
aliases both work. The same hierarchy exposes the existing imperative
`create|delete id vault provider credential` operations and
`create id vault provider enrollment` URL capture. Static shared credential
creation continues to use declarative configuration. These are command-path
changes; API operations, private capture requirements, mutation validation
and read sanitization remain the same. Existing flat aliases are retained.

Examples and the prepared walkthrough use the nested paths. The Identity
credential inspection and OAuth enrollment E2E helpers exercise the new
paths while retaining flat-form compatibility checks. Neither scenario is
replay-enabled; no replay registry or cassette changes were required.

The rebuilt binary successfully lists the walkthrough provider's active
shared credential in US `.tech` through the nested command. This was a
read-only check; no enrollment URLs or credentials were created or changed.

Validation passed: modernization, stable formatting, CGO-disabled build and
`GOFLAGS=-buildvcs=false make test-all` (lint, race-enabled unit and harness
tests, integration, installer, SDK-patch and replay-metrics checks). Nested
list and single-record reads both returned the expected active credential.
Updated live Identity scenarios have not been rerun for this command change.
The gate log is
`.cache/identity-feature-walkthrough/nested-vault-credentials-gates.log`.
Changes remain local and uncommitted.

## 78. Auth-server scalar extraction corrected

The Trusted IdP walkthrough exposed a CLI output bug: auth-server get/list
formatted JSON/YAML directly instead of using the shared output renderer.
Consequently, `--jq .issuer -r` and `--jq .jwks_uri -r` were accepted but
ignored. Shell command substitution captured the entire JSON object. Loading
the trust declaration then failed URI validation before any trust API write.
This was a client output defect, not an API behavior or engineering question.

Auth-server JSON/YAML output now uses the shared renderer. Command-level
regression tests run the actual command with fixture API responses, covering
issuer/JWKS extraction, name/UUID selection, quoted scalar output, get/list
collection projections and invalid expressions. The tests reproduced the
ignored-filter behavior before the fix. The auth-server E2E scenario also
checks plain issuer/JWKS extraction; it is not replay-enabled, so no cassette
or registry changes are required.

The binary is rebuilt locally. Users must rerun both export commands because
previously exported JSON objects remain in their shell environment. No
Trusted IdP or other remote resource was created or changed by this fix.

Final modernization, stable formatting, CGO-disabled build and
`GOFLAGS=-buildvcs=false make test-all` passed. The original live extraction
commands now return exactly the public issuer/JWKS strings. With those
values, the walkthrough trust manifest loads and proposes one creation.
The user subsequently applied it and confirmed matching URLs, false JIT and
Vault-access controls, and the configured claim mapping. The API's trust
creation succeeded without a backend workaround for this failure.
The gate log is `.cache/identity-feature-walkthrough/auth-server-jq-gates.log`.
The updated auth-server live E2E scenario has not been rerun. Changes are
local and uncommitted.

## 79. Trusted IdP associated-principal reads nested

Associated-principal reads now use
`get|list id trusted-idp principal[s]`, including singular/plural parent
aliases. The existing Trusted IdP get/list command retains its own reads,
selectors and aliases. Child reads still require exactly one of
`--issuer-url` or `--trusted-idp-id` and default their directory to `default`.
Selectors remain command-local; parent and flat-command flag state are not
shared. Existing flat `trusted-idp-principals` aliases remain supported.

No API operation, ownership rule or principal mutation was added. Examples,
walkthrough text and the Trusted IdP inspection E2E helper use nested paths.
The scenario retains an explicit flat compatibility check and exercises
issuer/UUID selectors, get/list, empty results, pagination and text/YAML
output. It is not replay-enabled; no registry or cassette changes are needed.

The binary is rebuilt locally. Read-only US `.tech` checks compared nested
get by issuer, nested list by UUID and the flat alias: all returned the same
existing On-call engineer. No remote resource was modified.

Modernization, stable formatting, CGO-disabled build and
`GOFLAGS=-buildvcs=false make test-all` passed. The updated inspection helper
parses successfully; its full live scenario has not been rerun. The gate log
is `.cache/identity-feature-walkthrough/nested-trusted-idp-gates.log`.
Changes remain local and uncommitted.

## 80. Organization-chart import moved under the import verb

The canonical command is now `kongctl import id org-chart`. Explicit product
selection works as `kongctl import konnect identity org-chart`; identity/id
and org-chart/org-charts aliases are supported. The earlier
`kongctl create id org-chart-import` path remains available for compatibility.

The new root verb and Konnect product route use the existing profile, API
origin, region and token configuration. Both implicit and explicit product
paths carry the import verb/product context into SDK construction. The
Identity import subtree exposes only the organization-chart operation.

Both command forms reuse the existing import runner: a whole-file upload,
explicit dry-run/execute choice, new private report file, no automatic write
retry and unchanged native provisioning/ownership rules. No principal
ownership metadata is added, and no CSV schema or SDK/API operation changed.

Focused command tests execute the real runner against fixture directory and
import APIs for both preview and execution, checking uploaded bytes,
directory defaulting, verb/product context, base URL/region/token flag
binding, counts output and private report capture. The existing create form
retains its argument validation and aliases. Examples, the manual walkthrough
and the import E2E helper use the new form; the E2E helper also performs a
legacy preview comparison. This scenario is not replay-enabled, and no
replay eligibility or recordings changed. No live import was executed by
the agent for this command reorganization.

Modernization, stable formatting, CGO-disabled build and
`GOFLAGS=-buildvcs=false make test-all` passed, including lint, race-enabled
unit/harness tests, integration, installer, SDK-patch and replay-metrics
checks. The updated live E2E helper parses successfully but has not been
rerun against Konnect. Built CLI help verifies both new paths and the legacy
path with the existing intent/report flags. The gate log is
`.cache/identity-feature-walkthrough/import-verb-gates.log`.
Changes remain local and uncommitted.
