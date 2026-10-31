# Kong Identity compatibility tracking

Track temporary implementation restrictions here and in the resource issues.
API questions
for engineering stay in the separate
[question log](../kong-identity-api-questions.md), without client-specific
implementation requirements.

The original restrictions below derive from dated US `.com` probes. They do
not establish equivalent prerelease `.tech` limitations. Revalidate the
native API in `.tech` before deciding which restrictions remain necessary.
The October 31 native revalidation is recorded below and in the engineering
question matrix. No CLI behavior was changed during that validation pass.

## Principal type changes

Related API question: Q9. Creation supports user, agent, and unclassified/null
types, but tested PUT/PATCH requests did not change an existing type.
In `.tech`, a non-null type change now returns an explicit immutable-field
error; a requested null still returns success without clearing the type.

- Initial contract: type is create-only. Preserve it when omitted on update;
  reject requested changes, including a requested reset to null.
- Removal trigger: engineering confirms the intended contract and direct HTTP
  tests verify supported changes in the target environment.
- Required verification: user to agent, agent to user, and classified to null;
  saved plans; omitted-field preservation; no-op reapplication; dump/reload.
- Keep the guard explicitly tied to Q9 rather than treating immutability as a
  permanent API guarantee.

## Principal active state

Related API question: Q10. Earlier US `.com` responses omitted active. The
generated SDK's plain bool cannot distinguish absence from false.

US `.tech` revalidation clears the native API blocker for directly provisioned
principals: false creation/read/list, PUT activation, PATCH deactivation and
reactivation work. Unrelated PUT/PATCH preserve omitted false on user, agent
and unclassified principals. Existing SDK request models already expose
optional `*bool` fields without defaults; no new SDK field is required.
Declarative optional active is now implemented. Explicit false/true flows through
planning, saved writes and dump. Omission uses the documented PUT semantics
and preserves the current value without synthesizing a false value. Explicit
null is rejected. Fresh observed state is required for updates; deployments
omitting active fail explicit reconciliation before mutation. Create and update
readback must retain the requested boolean; a failure after writing is reported
without retry or rollback.

The specification prohibits deactivating developer-portal principals. Planner
and executor enforce that rule against fresh provisioning information. Setting
false can change provisioning provenance to konnect_user, as documented by the
API. Other provisioning paths have not been live-tested; no broader guarantee
is inferred.

- Keep presence-aware reads/dump for older deployments (Q10).
- A field omitted by the API stays absent in dumps and unknown in text.
- Remove missing-field compatibility only after the deployment contract is
  established. Other provisioning actors need dedicated lifecycle fixtures.
- Coverage includes explicit false creation, false/true/no-op/saved updates,
  omission, protection, adoption, diff, dump and sync; the live command also
  verifies explicit reconciliation fails on deployments omitting the field.

## Principal management metadata PATCH

Related API question: Q11. PUT retained management metadata updates; PATCH
returned success while leaving a requested version change unchanged.
The partial/full-map and null-removal failure is also reproduced in `.tech`.

- Initial contract: use a fresh read plus PUT for management metadata changes,
  adoption, and protection updates; preserve omitted settings.
- Removal trigger: engineering confirms PATCH semantics and direct tests verify
  value changes, omitted keys, null removal, and preservation of other keys.
- Required verification: canonical five-entry ownership metadata, protection
  changes, adoption, omitted fields, and saved plans.
- Reassess whether PATCH simplifies the implementation once verified. Working
  PUT support remains valid even if switching is unnecessary.

## Integer metadata precision

Related API question: Q31. Direct principal PUT and group POST/PUT requests
accept large integers but can return rounded digits, including inside arrays.
Fresh GET confirms the change. For example, `9007199254740993` becomes
`9007199254740992`; `1152921504606846976` becomes `1152921504606847000`.
Raw JSON was parsed with exact integers. These results match a binary64
conversion followed by decimal serialization; this is an observed model,
not a documented guarantee about the service's internal representation.
Direct `.tech` principal PUT/group-create probes reproduce the same digits.

- Initial contract: validate declared principal metadata in the loader and
  again when executing saved plan fields. Reject integer digits that change
  through the observed conversion. Keep stable integers and string values
  intact; do not silently convert numbers into strings. Values above the
  binary64 consecutive-integer range are not categorically rejected:
  `9007199254740994` and the returned `1152921504606847000` remain stable.
- Create readback and PUT responses must retain explicitly requested metadata.
  A mismatch returns an error without retry or rollback: the resource may
  already exist or have changed remotely. Omitted fields still use fresh
  hydration, preserving remote metadata without inventing configuration.
- Removal trigger: engineering establishes the supported numeric contract,
  and direct create/PUT/fresh GET tests retain exact digits or explicitly
  reject unsupported values. Verify negative values, integer arrays, int64
  boundaries and downstream propagation before relaxing the guard.
- Required coverage: raw JSON integer comparisons, saved plans, omitted-field
  preservation, no-op plans and dump/reload. Client serialization tests alone
  cannot establish server retention. The live `identity/metadata-integers`
  scenario compares JSON through Python integers, avoiding floating-point
  assertion decoding.

## Current implementation and regression coverage

The principal resource accepts optional boolean active and rejects null.
Type changes are rejected in planner/identity_principal_planner.go and
rechecked against a fresh observation in executor/identity_principal_adapter.go.
The active capability/provenance guard is shared with read helpers; state writes
verify explicit active using presence-preserving GETs.

helpers.IdentityPrincipalReplacement builds hydrated PUT requests without
active. The executor overlays active only when saved plan fields request it;
adoption preserves it through omission. Coverage lives in loader, planner,
executor, state, dump and read-helper tests. The identity/directories scenario
includes active.py, which exercises the supported lifecycle or verifies Q10's
explicit missing-field error according to observed API presence. Type/null
rejection remains tied to Q9.

## Auth-server preview SDK boundaries

The public preview SDK exposes `dcr.default_access_token_duration`. Internal
DCR options remain outside the public manifest schema. PATCH writes only
declared public fields, preserving omitted backend options. Verify any future
DCR expansion against its create, partial-update, secret, and read contracts
before exposing it.

The preview includes a targeted SDK serialization patch that preserves an
explicit empty `trusted_origins` array. A nil slice still omits the field.
Keep the SDK wire regression and live clear/replan scenario when regenerating
or upgrading the SDK; remove the patch only when upstream serialization
preserves this distinction. This is a generated-client constraint, not an API
question for the service team.

Auth-server deletion always requests `force=false`. Existing children may
block deletion; no root-level cleanup or declarative delete silently cascades.
Declared child deletions run before their parent; the API still rejects
unobserved children instead of silently cascading.

## Claim names and pagination

Related API questions: Q12 and Q13. Direct HTTP probes accepted duplicate
claim names. Claim list totals exceeded the actual number of claims when
claims referenced multiple scopes.

- Initial read contract: name lookup exhausts the unfiltered collection and
  rejects duplicate matches. UUID lookup remains available. Server template
  strings are returned verbatim.
- Claim pagination continues through an empty terminal page, rather than
  trusting the inflated total as a resource count. It validates page metadata,
  rejects repeated/missing IDs, and exposes no partial result after a failure.
- Removal trigger: engineering documents name collision behavior and corrects
  totals; direct multi-scope tests verify accurate counts and terminal pages.
- Keep duplicate detection even if uniqueness becomes enforced, for records
  created before the constraint. Keep terminal-page traversal compatible with
  deployments that still report inflated totals.
- Required verification: two claims linked to three scopes each, page size one
  and one hundred, empty collections, duplicate names, exact UUID reads, and
  failed or repeated pages. Scope deletion also needs fresh claim/client
  observations because the API silently detaches associations.

The preview SDK at `dfd890eb1a03` includes a targeted claim PATCH fix for
explicit empty `include_in_scopes`. Its wire regression fails before the
patch and passes afterward; the live declarative scenario verifies clearing
and no-op replanning. Keep that regression during SDK upgrades and remove the
patch only when upstream serialization preserves nil versus empty arrays.
This serializer issue is separate from the service API questions.

## Creation retry safety

Creation of principals and claims uses a single attempt in both the HTTP
transport and generated SDK. Neither resource has a server-enforced unique
match key. After an uncertain result, generate a fresh plan to observe any
created record; duplicate matches fail before mutation. Q14 tracks the
service idempotency/recovery contract. The regression enables both retry
layers and verifies that a 503 response produces exactly one POST.
Revisit this restriction only after engineering documents idempotency and
direct contract tests verify safe recovery after uncertain outcomes. Any
future retry must use the verified deduplication mechanism; name/ref matching
alone is not proof that a POST is safe to replay.

## OAuth client update and pagination contracts

Related API questions: Q15–Q20. Client declarative lifecycle, initial-secret
capture, reads, adoption and explicit secret lifecycle are implemented.
Resource metadata lives in client labels; secret records have no ownership
field. The live client lifecycle and explicit rotation scenarios verify these
separate contracts.

- Client pagination now follows actual pages to an empty terminal page,
  ignoring totals for completion. Two clients each linked to two scopes
  reported total four. This also fixes the shared client observation used
  when planning or executing scope deletion. Missing/repeated IDs, invalid
  pagination and failed pages still reject the complete observation.
- Client names are documented unique per server and a direct duplicate
  create returned 409. Positional CLI selectors are always opaque client IDs;
  exact-name selection uses `--client-name`, with full-scan duplicate and
  fresh detail checks. Name/ID ambiguity must not be guessed from UUID shape.
- A labels-only PATCH resets `token_endpoint_auth_method` to
  `client_secret_post`. All declarative updates and adoption carry the fresh
  existing method explicitly when omitted, including adoption and protection
  changes. Live credential continuity verifies preservation. Remove the
  workaround only after direct basic/none omission tests pass (Q15).
  A direct runtime probe verified the safe path: the same original secret
  minted a client-credentials token before and after a label PATCH carrying
  the existing `client_secret_basic` method. Deleting that secret changed
  authentication to 401; importing replacement material restored 200. No
  token or secret values were logged. The declarative executor and adoption now
  verify that continuity with a privately captured initial secret. Separate
  secret issuance also has
  private file capture and live rotation coverage.
- Raw HTTP accepts explicit empty grant, response, redirect and allowed-scope
  arrays, and null or empty login URI. The preview SDK at `221af2edb788` now
  preserves explicit empty grant/response/redirect/allowed-scope arrays and
  omits an unset authentication method. The wire regression failed before
  those fixes and passes afterward. This changes serialization, not Q15's
  deployed defaulting; executors still preserve the fresh method.
- Authorization-code grants were rejected in US `.com`. Keep API capability
  limits explicit and verify other grants independently; do not silently
  rewrite a requested grant or make broad support claims (Q17).
- Secrets have two total slots even when both records are disabled. An
  explicit lifecycle must never delete a record merely to make room without
  a requested retirement. The explicit create command captures material once
  in a new private file and makes a single request through both retry layers.
  No declarative secret-slot naming contract exists (Q16).
- Secret reads project onto a metadata-only type. No read command serializes
  an SDK secret item directly. Regression tests inject unexpected values and
  assert they cannot appear in JSON/YAML; live coverage checks all formats.

The patch bundle at `2489716c` also fixes accumulating multiple targeted
replacements in one file before writing. Regeneration tests cover this and
drift failures without partial file updates. Keep these regressions alongside
the SDK's existing origin/claim array fixes and other KongCTL patches.

## Explicit secret capture and rotation

Client-secret commands use explicit parent/client selection and record UUIDs.
They have no declarative ownership or automatic rotation policy. Creation
reserves an unused `0600` file, writes recovery metadata before POST, and
captures the issued value only there. Existing files and symlinks fail before
creation; replaced/disappeared output paths are detected. The successful
capture is synced and closed. Normal output uses a metadata-only projection.

Missing sources fail before creation. Imported values use existing env/file
secret resolution; parser failures do not publish source contents. SDK error
bodies can echo supplied material, so secret operations expose status only.
Synthetic tests verify a single POST even with SDK and transport retry enabled,
failed output preflight, an uncertain server response, missing returned values
and a path disappearing after remote creation.

Failures retain capture/recovery evidence and never automatically replay
creation or delete an issued record. Q19 asks engineering for the API recovery
contract. A reserved file does not prove whether creation happened. Inspect
remote metadata and explicitly retire an unusable record before retrying into
a new path. Enable/disable is explicit; deletion uses standard confirmation.

The live `identity/client-secrets` scenario covers generated and imported
material, two-slot rejection, output overwrite refusal, all mutation routes,
missing sources/flags, metadata reads and actual token authentication through
enable/disable/delete. Private files stay outside artifacts in a test-managed
directory, removed even after a deliberate stop. Secret-slot declarations
remain unavailable; initial client-create capture is covered separately below.

Revisit count traversal only after multi-scope, page-size-one tests verify a
correct resource total (Q18). Keep compatibility with older deployments and
the no-partial-inventory guarantee.

## Recording resolution

For each resolved question, record the team answer, date, environment/API
version, and direct-call evidence in the question log. Update #1415 and this
tracker, remove or revise the corresponding guard, and add the verification
above in the same change. Do not silently broaden support based solely on a
specification or SDK regeneration.

## Declarative client implementation and initial capture

`identity.auth_servers[].clients` and grouped `identity.clients` support
explicit required name/grant/response fields, all public optional settings,
and scope UUIDs or typed refs under the same server. Namespace/protection use
independent labels, including under external server parents. Omitted child
collections do not authorize pruning; explicit empty collections select their
source namespace. Auth-server pruning tracks its own source namespaces.

Client scope matching observes existing scopes even when a client's namespace
is planned before the server's. Final dependencies include parent creates,
client/claim scope detachment or deletion, and child-before-parent deletion.
Fresh execution observations still reject late references and changed parent
identity, ownership or protection. A metadata-only update is not proof that a
client detached from a scope. Ordinary client updates never send secret values.

Initial creation requires `--secret-output-dir` on apply/sync. All create
outputs are reserved before API mutations; conflicts fail the entire
preflight. Unattempted reservations are removed, attempted uncertain creates
retain recovery markers, and successful files contain generated values plus
public identification metadata. File paths appear in execution results, not
plans. SDK and transport retries are disabled for initial creation.

Canonical Sliver marks `client_secret_basic` as `x-enum-dev`, so the selective
public SDK enum excludes a deployed, working method. Declarative validation
explicitly accepts that canonical value; SDK wire tests and live runtime
authentication verify passthrough. Q20 asks engineering to clarify that
annotation. Remove the extra validation case once the generated enum includes
the value. Authorization-code and refresh-token grants remain outside this
preview enum; Q17 records the deployed authorization-code denial.

The `identity/declarative-clients` live scenario covers saved plan/diff/apply,
initial capture, all public fields, dump/replan, explicit empty list updates,
scope detachment and deletion, protection, adoption, external parents, scoped
sync and explicit deletion. It authenticates using the initial secret before
and after updates/adoption, and scans artifacts for the raw secret and its
Basic-auth encoding. Private capture directories are outside artifacts and
removed after remote fixture cleanup, including early-stopped runs.

## Principal-identity inspection and parent scoping

Related questions: Q21–Q23. The four variants generate IDs and retain the
five managed_by ref/ownership entries and public labels on direct create,
read and PUT probes. Metadata PATCH is rejected; type-specific PATCH works.
An API Gateway consumer identity also returns `control_plane_type`. These
contracts now underpin the declarative lifecycle described below, alongside
get/list inspection.

GET and DELETE accept an identity through another existing principal in the
same directory. PUT rejects that path, and each collection lists only its
actual principal's identities. Scoped detail reads therefore prove membership
in a complete fresh collection before GET. Update, adoption and deletion
repeat that proof, especially before DELETE. Fixture cleanup uses
this proof, validates fresh fixture labels, observes every child collection
before starting writes, and requires empty child collections before parent
removal. Unowned or incompletely observed children stop cleanup.

Remove the extra membership workaround only after direct probes verify that
GET and DELETE both reject a wrong existing principal with 404, without
changing the actual owner's identity. Preserve saved-plan identity/ownership
checks independently of the service fix. Parent linkage alone never confers
ownership over an identity.

### Principal deletion while identities remain

Direct probes show principal DELETE returns 204 with OAuth-client and Gateway
consumer identities still attached, including an AI Gateway consumer link.
The principal and its identity collection then return 404. The schema does
not document descendant retention/deletion or an atomic nonempty-child
precondition; Q24 asks for that API contract.

Declarative principal delete and sync pruning require explicit authorized
deletions for every observed identity. Unplanned identities, including unowned,
protected or foreign-namespace associations, stop planning. Execution requires
a fresh empty collection after those deletions, so a saved plan cannot
silently cascade through later identities.
Missing/failed/incomplete observations fail closed. These are client checks;
without a server-side precondition a concurrent creation between observation
and deletion remains possible. Credentials and group memberships require
their own verified contracts as those families are enrolled.

Final plans order explicitly authorized identity deletions before their
principal. Keep the fresh empty collection check for unplanned or late
children. Prefer an atomic API
precondition if engineering supplies and verifies one; do not remove these
checks merely because the cascading behavior is documented.

### Principal-identity write foundations

Shared helpers project all four observed variants into full PUT bodies,
preserving public labels, management metadata and workspace presence while
omitting IDs, timestamps and read-only `control_plane_type`. Create disables
SDK and transport POST retries, checks retained type/metadata and reads the
new identity back through the selected parent. PUT and DELETE repeat complete
fresh membership proof before mutation (Q21); PUT rejects type changes and
checks retained metadata/ID in its response. These helpers do not establish
ownership, adopt resources or synthesize a metadata stamp. Declarative callers
and the adoption command add independently scoped ownership/protection,
collision checks, fresh omitted-field hydration, persisted refs and final
dependencies under #1416.

Direct custom/OIDC probes verify that PUT with omitted or empty labels clears
them while retaining the supplied management map. Declarative hydration
preserves labels omitted from the manifest before sending that full PUT;
explicit `labels: {}` clears them.

OAuth-client and API Gateway consumer associations can be retargeted through
PUT while retaining metadata. AI Gateway OAuth consumers also work with
`control_plane_consumer`, returning `control_plane_type: ai_gateway` and
retaining metadata on PUT. A later probe verified AI-to-API Gateway
retargeting; the reverse direction has not been verified.

## Further identity API contracts under implementation

Q25 asks for valid workspace configuration, field semantics and validation.
The schema supplies only a string. Text and UUID-shaped probe values both
returned 500 on otherwise working API and AI Gateway consumer identity PUTs;
subsequent GETs retained the original association without a workspace. These
were not known existing workspaces, so the probes cannot establish whether
a valid workspace is unsupported. Explicit workspace configuration is deferred
while the other four identity variants are implemented. Read models and
generic PUT projections preserve workspace presence. Enable configuration
only after a valid create/read/PUT fixture establishes the contract, including
metadata updates, omission and clearing. Add live coverage and remove the
Q25 validation restriction together.

Tracked implementation sites are resource validation/explain notes, the
workspace adoption guard, external export handling and the examples/lifecycle
scenario. Workspace-bearing records are inspectable and export as external
references; their remote metadata is unchanged. This avoids exporting a
managed declaration whose workspace configuration cannot yet be verified.

AI-to-API Gateway consumer retargeting works through the same identity type;
the response changes read-only `control_plane_type` to `api_gateway` while
retaining management metadata. Q26 records a separate lifecycle distinction:
deleting an OAuth client makes its identity disappear from both GET and list,
but deleting an API Gateway consumer and then its control plane leaves the
identity readable with the original IDs. No cross-family cleanup policy is
inferred for AI consumers. Do not equate an identity's independent metadata
with an API-enforced guarantee against deletion of an associated resource.

## Declarative principal identity lifecycle

All four variants use independent namespace/protection and persisted refs in
five-entry managed_by maps. Nested and grouped declarations, external principal
and identity UUIDs, same-plan parent/client/control-plane references, saved
plans, adoption and dump use this contract. External membership is verified
against complete scoped collections before planning. Existing Identity IDs
are resolved before protected payload comparison, including unchanged targets
in other namespaces. This prevents unresolved label/association references
from becoming false updates on protected resources.

Principal deletion checks the final cross-namespace plan for explicit identity
deletions, orders those first, and still requires a fresh empty collection at
execution. Referenced OAuth-client/server and Gateway deletions similarly
require observed associations to be deleted or retargeted first. Full PUT
fields establish detachment; metadata-only changes do not. Saved plans retain
the observed directory/principal scopes and repeat complete reference checks
at execution, including late foreign or unowned associations. A principal
removed by a preceding planned deletion makes that scoped collection absent;
other observation failures block the referenced-resource deletion.

These checks cover collections observed from declarations. They do not add
organization-wide reverse scans or require directory permissions for an
auth-server-only manifest. OAuth client deletion warns about native cascade
effects outside the inspected scopes. An old plan without captured reference
scopes cannot gain those checks retroactively; regenerate it when introducing
identity associations. API-level atomic preconditions remain Q24/Q26 work.

The live scenario covers all four types under a protected principal, distinct
ownership namespaces, same-plan directory/principal/client dependencies,
idempotent saved plans, association updates, public-label removal, diff,
dump/replan, adoption, protected updates, omitted versus explicit empty scope,
foreign/unowned preservation and child-before-parent deletion. Q25 is the
explicit field limitation rather than a guessed workspace contract.

## Directory cursor request serialization

The principal API-key reader is now included in the cursor correction under
Q22, in preview SDK `70a2818dde13` from patch bundle `0103edc5`. Actual SDK wire
tests fail before its request-tag correction and pass afterward. A live
page-size-one scenario verifies three key pages from request traces, not only
the result count. Response models and API operation inventory are unchanged.

The shared CursorPageQuery parameter omits an explicit serialization style.
Default form/explode generation emits flat `size` and `after`; the deployed
API ignores them and reports its default page size 20. This was hidden in
earlier small live collections. Direct bracketed size/cursor probes verify
the actual contract. Q22 asks engineering to correct the common definition.

Preview SDK `6ca9d1f8b309`, using patch source `ef232ca0`, changes only the
request tags for directory, principal and principal-identity list operations
to deepObject. No operations or response fields are added; all earlier
KongCTL patches remain. The wire regression fails on all three old request
shapes and passes with bracketed names, including opaque cursor encoding.
Future directory resource readers must verify their own generated query
parameters before claiming complete pagination; this patch enrolls three
implemented lists rather than altering unrelated SDK resources. Remove it
when upstream generation yields the same wire contract.

The principal-identity live scenario uses ten identities and page size one,
checks all four response shapes and explicitly verifies trace evidence of
actual cursor traversal. A larger initial fixture hit a maximum-ten quota
(Q23); no client-side fixed quota is inferred from that observation.

## Principal API-key inspection and write boundaries

The initial #1417 implementation is scoped get/list only. A dedicated public
projection excludes material even if an unexpected read response includes a
credential field. Errors from the credential API are reduced to status/context
without response bodies. All pages are observed; repeated IDs/cursors, invalid
IDs, missing collections and page errors fail the entire observation.
The verified key detail route rejects a wrong existing principal with 404;
the identity-specific Q21 workaround is not copied without that discrepancy.

API-only Q27–Q30 and Q4 govern enrollment of mutations:

- Keys work under `agent` and unclassified principals. A freshly readable
  `user` principal's key create/list return 404. Reads report that failure;
  they do not interpret it as an empty collection or enforce a local type ban.
- Generated creation retains the five-entry ownership map and expiration.
  Imported creation rejects `managed_by`. Both kinds ignore complete and
  partial management-map PATCH changes. PUT returns 405. Adoption, namespace
  movement and protection updates therefore remain unavailable.
- Expiration PATCH works, including explicit null, but omission clears it.
  A future metadata writer must hydrate expiration from a fresh read, then
  verify actual credential continuity and metadata retention.
- Name-bearing create returns 400, while name-bearing PATCH returns success
  without making name observable. Missing/null names are omitted from the read
  projection. No name identity or uniqueness rule is inferred.
- An administration bearer token does not establish runtime mTLS context.
  Generated/imported authentication returned "mTLS headers missing"; this
  establishes no credential continuity or revocation guarantee.

Before declarative writes/adoption, verify metadata changes and removal,
duplicate/ref matching, preserved material/expiration, type applicability,
atomic imported creation and uncertainty recovery. Add corresponding lifecycle
and runtime tests. Declarative lifecycle and adoption remain pending those
verified ownership contracts.

Principal declarative delete/sync currently guards identity associations, not
API-key descendants. Key enrollment must add production parent-deletion
observations and ordering, including a verified contract for the user-type 404.
The fixture cleanup checks below do not extend production deletion authority.
Q24 records the API's native descendant deletion; public metadata alone does
not provide independent protection against that operation.

There is a separate SDK preparation issue: canonical key create/update bodies
reference a mutable nullable expiration schema, while the merged preview's
shared `ExpiresAt` is read-only. Generated write models consequently omit the
field even though direct API writes work. Resolve that targeted schema/reference
difference and verify omitted/value/null wire semantics before enabling writes;
it is not evidence that the API lacks expiration support. The current SDK
increment fixes only pagination and adds no write-model fields.

The live inspection scenario uses generated keys with explicit fixture metadata
under agent/unclassified principals, captures material privately, verifies
three request pages and scans artifacts for material and its Base64 form.
Fixture cleanup opts into API-key observation explicitly, checks every marked
collection before deletion, rechecks metadata before individual deletes and
requires empty collections before principal removal. Existing scenarios do not
gain a new credential permission requirement. Subsequent direct group-key GET
and POST requests returned 501 (Q33); their shared schema is not sufficient to
enroll them.

## Group and direct membership inspection

The preview SDK fixes bracketed pagination for `ListGroups` and
`ListGroupMembership`, extending the same Q22 workaround to six operations.
Wire tests fail before the patch and pass afterward; the complete SDK unit
suite also passes. Keep these replacements until upstream generation produces
the documented query shape. No unrelated internal SDK model is introduced.

Group UUID/name lookup observes the complete directory collection before a
fresh detail GET. Membership detail likewise proves entry-ID association
through the complete selected group collection. Missing/malformed data,
pagination, repeated IDs or duplicate members fail without partial output.
Reads retain actual provisioning/team-manager presence instead of applying
generated SDK defaults. They do not expand transitive relationships.

Direct manual group/user membership removal works. Principal GET reports
group IDs as links are added/removed, and principal deletion removes its
membership entries. Cross-directory behavior with two existing parents is not
verified: the test organization allows only one directory. Current collection
proofs avoid relying on a detail route alone for parent association.
Principal delete/sync now guards identity associations and incoming direct
group memberships. Fresh directory-wide group/membership observations prevent
principal ownership from authorizing implicit removal of another group's link.
Explicit authorized membership deletions now supply detachment dependencies
before principal/group deletion; execution repeats fresh empty checks.

The initial inspection increment did not expose writes. Ordinary group and
direct principal membership lifecycle below adds explicit parent/collection
authority, reference ordering and descendant deletion checks. Q32 tracks
group-to-group rejection and imported-team boundaries; Q33 tracks unavailable
group key endpoints. Working manual membership deletion does not establish
authority over imported or inherited relationships.

The later `.tech` revalidation changes that availability assessment: group
keys support list/create/read/PATCH/delete, while team creation returns 400
and accepts only type group. Group-to-group membership is still rejected.
Generated group-key creation retains ownership, but PATCH ignores ownership
changes; imported creation rejects that map. Group-key name behavior also
differs from principal keys (Q29). Current group lifecycle observes key descendants in `.tech`; it does not
infer mutable ownership or runtime credential continuity from endpoint
availability alone.

The live group scenario uses manually created, credential-free fixtures.
Cleanup opts into `principalGroups: true`, observes all group/member scopes,
requires exact management markers, direct-user provenance and marked principal
targets, rechecks identity/protection/membership before deletion, then requires
empty collections before removing parents. This test policy grants no product
authority over group keys or importer-managed teams. Existing scenarios gain
no new group permissions.


## Ordinary group declarative lifecycle

Ordinary groups reconcile by directory/name with independent four-entry
managed_by ownership. Names are explicit ASCII alphanumeric values; refs are
local and are not a substitute for the required name. Canonical metadata
normalization, adoption and protection follow the existing owned-child
pattern. Parent directory ownership/protection does not authorize or block
independently owned group changes.

Full PUT hydrates omitted description, labels and runtime metadata from a
fresh read, overlays saved fields, and verifies ownership/public settings.
Loader and saved execution both enforce Q31's integer restrictions.
Only directly provisioned konnect_user groups are managed; missing provenance,
teams and team-manager state are rejected. Omitted/null type is ordinary
group according to the schema; reads preserve presence rather than adding it.
Unsupported/importer group kinds are preserved during ordinary sync.

Group deletion requires fresh complete membership observations plus an empty,
terminal unfiltered first key page. Any key, missing/malformed pagination,
unavailable endpoint or failed observation blocks deletion. Because any key
blocks, no key-page query or traversal is needed; this does not add group-key
pagination support or a new SDK patch. Legacy 501 (Q33) is a bounded deletion
limitation, not an empty collection. Native directory force=false also rejects
a directory containing a group (verified 409).

Native group PUT preserves an observed membership entry and key record in
US .tech. This establishes administrative continuity only; runtime key
authentication still needs Q30's supported mTLS context. Client emptiness
checks cannot close a race with a concurrent attachment; Q24 requests an
atomic service precondition.

The live declarative-groups scenario covers a new protected directory plus
group/principal, saved plans, all public fields, clear/omit, dump/replan,
external name lookup, adoption, protection, membership deletion guards and
namespace-scoped sync/delete. Legacy key 501 is asserted as a specific
fail-closed boundary. Credential-free fixture cleanup explicitly opts into
generated-name prefixes and a namespace allowlist; this grants no product
authority over unknown credentials or imported membership.


Before full group-key pagination is added, verify the preview SDK listGroupKeys
wire names: its page field still uses the flat form/explode annotation.
The existing cursor patches do not cover this operation. The
current unfiltered empty-page guard sends no page parameter and needs no SDK
revision. Full key writers also need the missing expires_at request field
and omitted/value/null serialization work already tracked under #1417/#2659.

## Direct principal membership lifecycle

Direct principal memberships inherit ordinary-group ownership and protection.
They store no independent metadata. The parent must be a declared managed
group with verified konnect_user provenance; external groups are read-only.
Match the directory-scoped group plus principal ID, and use entry IDs only
for route selection. Refs remain local. Same-directory collection proofs,
duplicate checks and fresh parent/entry observations precede writes.

Nested/grouped declarations, saved creation and deletion, add-only apply,
explicit collection sync, dump, explain and scaffold are implemented. Omission
preserves links; an explicit empty collection selects principal memberships
for pruning. Sync also retains the existing parent-group collection scope.
Changed member IDs add different relationships; sync removes omitted links.
There is no relationship update or independent adoption operation. Explicit
group adoption preserves links and establishes authority over its collection.

New memberships may be added to a protected group under the existing
protected-parent creation policy. Removal requires a separately unprotected
group; a simultaneous protection change cannot authorize removal. Saved
execution rechecks fresh group name, namespace, provenance and protection,
and verifies entry identity before deletion. Creation is single-attempt in
both retry layers and checks scoped readback without rollback or blind retry.

Final cross-namespace plans require explicit authorized detachments before
deleting a principal or group. Unplanned direct links block those deletions.
Fresh execution still requires empty collections, and group keys must be
observably empty. A 404 while observing memberships is not an empty collection
unless a fresh directory-scoped observation proves that group disappeared.
If the entire group collection returns 404, fresh directory absence is
required before treating it as gone; an existing or unreadable directory
preserves the failure. This also supports replay of saved deletes after
successful parent-directory removal.
Client checks cannot supply the atomic precondition requested in Q24.

Nested group members, importer/team writes and batch operations remain
deferred under Q32. Unsupported nested state blocks authoritative pruning
and managed membership export. No inherited relationship is removed through
the principal's computed groups field. These boundaries need native tests and
lifecycle coverage before expansion, rather than inference from the schema.

## Basic-auth credential contracts under evaluation

Direct US `.tech` probes verify basic-auth records/passwords for user, agent
and unclassified principals. Username conflicts return 409 within/across
principals in one directory. Basic-auth full PUT retains a supplied management
map and clears omitted metadata. PATCH ignores management changes. PUT and
username rename preserve observed password IDs; no runtime continuity is
claimed without Q30's supported context. Metadata writes must hydrate the
username from fresh state and verify retained ownership/public fields.

Generated/imported password creation retains ownership and explicit expiry.
Read responses omit secret material. PATCH ignores ownership changes/removal
and clears omitted expiry. PUT requires explicit secret material and can
retain new metadata/expiry when supplied. Two total password slots remain
occupied even after expiration; a third create returns 403. Do not implicitly
retire a password to make room or resend a creation after an uncertain result.
Q35 asks for metadata updates that preserve secret/expiration. Password
metadata-only adoption is unavailable; declarative enrollment still requires
an explicit secret-source/rotation policy rather than an assumed preserve.

Wrong-parent basic-auth GET/DELETE and wrong-principal/wrong-basic-auth
password GET return 404. Principal DELETE succeeds with attached records and
passwords; later administrative reads return 404. Add credential observations
before principal deletion, and password observations before basic-auth
deletion, without inferring runtime revocation or cascading ownership.

Preview SDK 995778a1c448 corrects bracketed pagination for basic-auth and
password lists, extending the existing six list corrections to eight. The
wire regression fails on both old request shapes and passes after correction;
full SDK tests pass. Existing write models suffice for username records.
Password write models separately omit expires_at due to the shared read-only
merged schema; that is SDK preparation work, not proof of an unavailable API.

### Implemented read and username-write foundations

Basic-auth username and password get/list commands now use dedicated public
response projections, sanitized errors and complete raw-presence-aware
pagination. Missing data/page metadata, repeated IDs/cursors and failed pages
return no partial result. ID arguments are always UUIDs; exact username
selection is a separate flag so UUID-shaped usernames remain usable.
Detail and list text output both use tables.

Shared username-record helpers provide create, GET/full PUT/readback and
password-aware deletion. They do not grant ownership, stamp management maps,
or expose imperative mutation commands. A full PUT body contains only the
explicit username and management map; callers must hydrate omitted fields
from fresh state. Requested identity/metadata must survive both the write
response and a subsequent detail read. An uncertain or inconsistent result
returns an error without automatic rollback. POST disables both retry layers.
Delete requires a complete empty password observation; it never retires
password slots. This client check cannot close a concurrent creation race.

Declarative username registration/planning/execution/dump/adoption and
principal credential-deletion guards are now implemented as described below.
Password metadata writes/secret rotation remain outside this slice under Q35
and the separate SDK request-model gap. The live identity/basic-auth scenario verifies reads
for user, agent and unclassified principals, five username/password records,
all output formats, actual bracketed cursor traversal and wrong-parent 404s.
Generated/imported fixture material stays in private captures; observed
credential descendants are removed explicitly before fixture principals.

### Declarative username lifecycle and credential deletion guards

Username records use independent four-entry managed_by ownership, exact
username matching and local refs, under managed or external principals.
Nested/grouped declarations, saved plans, protection, scoped sync/delete,
adoption, external UUID/username lookup, dump and explain/scaffold are
implemented. Parent directory/principal protection does not grant authority
over these independently owned credentials. Changing a username creates a
different record; no automatic credential move or password rotation occurs.

Metadata updates use the verified full PUT/readback path. Saved execution
checks fresh scoped identity, namespace and protection. Dump uses UUID local
refs and exports unowned usernames as external; passwords are excluded.
Adoption does not stamp password metadata or send password values.

Any password blocks username deletion. Principal deletion requires explicit
authorized username deletions and fresh empty username/key collections;
any API key blocks it. Final merged plans order cross-namespace child deletes
first. A newly attached credential or failed/malformed observation stops saved
execution. Principal key checks use an unfiltered empty terminal first page,
not an assumed resource count or an unsupported filter. Legacy user-key 404
is an error, not proof that the collection is empty. These client observations
cannot close an attachment race; Q24 still asks for an atomic API contract.

The identity/declarative-basic-auth live scenario verifies same-plan protected
parents, independent ownership, local-ref changes, protection/unprotection,
adoption, external username lookup, dump/replan, explicit versus omitted sync,
foreign/unowned retention, stale ownership/password guards and cross-namespace
child-before-principal deletion. Password/key fixtures use private capture,
secret/base64 artifact scans and verified explicit retirement. No additional
SDK changes were required beyond preview 995778a1c448's cursor corrections.
