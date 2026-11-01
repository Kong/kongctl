# Kong Identity compatibility tracking

Track temporary implementation restrictions here and in the resource issues.
API questions for engineering stay in the separate
[question log](../kong-identity-api-questions.md), without client-specific
implementation requirements.

The original restrictions below derive from dated US `.com` probes. They do
not establish equivalent prerelease `.tech` limitations. Revalidate the
native API in `.tech` before deciding which restrictions remain necessary.
The October 31 native revalidation is recorded below and in the engineering
question matrix. No CLI behavior was changed during that validation pass.

## Principal CSV input, public metadata policy and UUID assertions

Principal CSV is a client frontend to normal declarative resources, separate
from the native organization-chart importer. CSV rows require local refs;
optional UUID assertions require an existing matching managed principal.
They cannot create a resource with that ID, silently adopt an unowned record
or override importer provisioning. Display names supply no matching authority.
Nested identities, generated keys and basic-auth usernames reuse their existing
metadata/ownership, capture and deletion contracts. Imported key/password
material remains outside declarative enrollment under Q28/Q35.

Explicit metadata_mode:merge retains unmentioned public keys; sync replaces
the complete map. Omitted metadata preserves it in either mode. Existing YAML
without the field retains replacement behavior. Planning compares the merged
target but stores only the declared patch with metadata_merge intent.
Execution re-reads public metadata and preserves keys added after planning;
it validates the complete hydrated map and the Q31 precision boundary before
PUT. Arrays replace a key's value. Empty merge is a no-op; empty sync clears.
No instruction is sent as an API property or ownership marker.

Plan format 1.2 prevents older executors from mistaking a patch for a complete
replacement. This binary still reads 1.0/1.1 plans, but rejects merge intent
on those versions, other resource types, creates/deletes or missing patches.
The US .tech CSV lifecycle verifies fresh late-key retention, explicit
replacement/clearing and ordinary 1.1 replacement execution through direct
public GET checks. The SDK revision does not change.

CSV parsing rejects duplicate headers/refs/asserted UUIDs/JSON keys, unsupported
fields, conflicting parent selectors, invalid typed values, embedded template
controls and deferred environment/secret encodings. Reference-looking strings
remain literal. Paths reuse the !file boundary and symlink checks; oversized
or nonregular inputs are rejected instead of truncated. Offline inspection
returns structural counts only. Remote authority is still checked by planning
and execution, including saved plans. See the CSV example/tutorial.

## Unrelated resource planning

Directory planning returns early for apply/delete when no directory is
declared. Children still require their declared directory, including external
parents. Explicit directory sync scope retains its existing behavior.
Without the guard, unrelated control-plane/portal/API workflows issue an
unnecessary directory list and acquire a new API availability dependency.
The focused planner regression verifies zero directory reads for absent,
empty and auth-server-only Identity declarations. All twenty enabled replay
scenarios pass unchanged; adding unrelated HTTP exchanges is not the remedy.

## Principal-identity read-after-create observation

Related API question: Q52. One US .tech trace returned 201 for OIDC identity
creation, followed immediately by an empty parent-scoped list. Twenty direct
HTTP trials with active/inactive users did not reproduce it. The current
creation helper still reports the issued UUID and verification failure;
it does not replay POST or bypass scoped membership proof using detail GET,
whose parent-scoping issue is tracked by Q21. No new retry workaround or
strong-consistency guarantee is inferred from the successful trials.

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

The principal-group collection is the seventeenth operation in the canonical
cursor patch. Native page-size-one reads verify disjoint pages and next-link
cursor extraction; flat size is ignored. The expanded actual-SDK regression
fails only ListPrincipalGroups against the previous preview, then passes all
seventeen methods after correction. Its dedicated CLI reader and consumer pin
are being implemented under #2658. Response models do not change.

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

The initial #1417 implementation supplied scoped get/list; explicit principal
and group credential mutations are now implemented under #1417/#2659.
A dedicated public projection excludes material even if a read response includes a
credential field. Errors from the credential API are reduced to status/context
without response bodies. All pages are observed; repeated IDs/cursors, invalid
IDs, missing collections and page errors fail the entire observation.
The verified key detail route rejects a wrong existing principal with 404;
the identity-specific Q21 workaround is not copied without that discrepancy.

November 1 US `.tech` native checks cover user, agent and unclassified
principals plus two ordinary groups. Generated/imported create, expiration
changes and retirement work. Complete map replacement/null removal are ignored;
imported create rejects managed_by. Principal scalar names work, but null does
not clear them. Generated group creation retains names; imported group naming
and all group name PATCH changes are ignored. Q28/Q29 track those contracts.

Explicit create reserves a new 0600 capture before issuing one POST, with both
retry layers disabled. Material is saved before public receipt/read-back
validation; uncertain outcomes never cause automatic reissue or deletion.
Captures include directory, principal/group and known key UUIDs. Imported
sources use the existing secret-env/file resolver. Normal output is a public
projection. Whole credential bodies and body-read/transport diagnostics are
redacted on principal/group key routes, including arbitrary error bodies.

PATCH sends only supported public fields and no managed_by changes. A fresh
strict observation supplies omitted expiration, which is explicitly sent to
avoid the API's clearing behavior. Returned fields and fresh read-back must
retain management metadata, creation identity, name and requested expiration.
Group name changes and imported group naming fail before issuing a mutation.
Null name clearing is not exposed. Mutations are single-attempt operations.
Lift group naming restrictions only after create/PATCH and fresh reads retain
the requested values. Null clearing also needs a request model that distinguishes
omission from null; the current SDK name pointer cannot express that distinction.

Explicit deletion confirms user intent, checks fresh detail and complete parent
collection membership, sends one DELETE and verifies absence. These commands
do not stamp ownership or enforce declarative namespace/protection, matching
the explicit password/secret operations. Administrative reads do not establish
runtime continuity or revocation; Q30 still needs supported integration context.
SDK preview `90596b498bcb` already includes writable nullable expiration and
all affected cursor corrections. No new SDK revision is required.

API-only Q27–Q30 and Q4 govern enrollment of mutations:

The following initial US `.com` observations are historical. The engineering
question matrix records the newer `.tech` availability results; ownership and
expiration-omission restrictions are still reproduced there.

- Keys work under `agent` and unclassified principals. A freshly readable
  `user` principal's key create/list return 404. Reads report that failure;
  they do not interpret it as an empty collection or enforce a local type ban.
- Generated creation retains the five-entry ownership map and expiration.
  Imported creation rejects `managed_by`. Both kinds ignore complete and
  partial management-map PATCH changes. PUT returns 405. Adoption, namespace
  movement and protection updates therefore remain unavailable.
- Expiration PATCH works, including explicit null, but omission clears it.
  The explicit writer now hydrates expiration from a fresh read and verifies
  public metadata retention. Runtime continuity is not established.
- Name-bearing create returns 400, while name-bearing PATCH returns success
  without making name observable. Missing/null names are omitted from the read
  projection. No name identity or uniqueness rule is inferred.
- An administration bearer token does not establish runtime mTLS context.
  Generated/imported authentication returned "mTLS headers missing"; this
  establishes no credential continuity or revocation guarantee.

Generated principal/group key declarations are implemented through resource
loading, plan/diff/apply/sync/delete, state/execution, dump and explain/scaffold.
Imported ownership/adoption has no verified atomic creation or metadata-transfer
path. Neither names nor material supply matching identity. The generated
creation marker records explicit declared intent; it is not inferred from
arbitrary public key metadata. Key adoption remains unavailable.

The generated-key implementation follows ordinary-child authority rather than
independent key protection. A managed principal or ordinary group supplies the
current namespace/protection; an external directory remains valid. External
principal/group declarations do not grant that authority. The immutable
key map contains tool, creator version, persisted ref and kongctl-create-type:v1.
The final marker records explicit generated creation intent, which public key
reads do not otherwise reveal. It does not identify imported or unowned keys.
Child namespace/protection copies are deliberately absent: parent transitions
must not require an API operation known to ignore management-map changes.

A November 1 native probe verifies all four markers under both parent types,
parent namespace/protection changes without key-map changes, expiration updates
preserving markers and duplicate-ref detection across two pages. Every fixture
is retired and the default directory object is unchanged. Evidence is
generated-key-parent-authority.json in the local cache. This establishes API
storage behavior. The separate live declarative scenario verifies the client
authority/ordering implementation against the same API contract.

The metadata engine now permits inherited-namespace resources to persist a ref,
record fixed creation intent and opt out of management-map stamping on updates.
Existing mutable resource policies are unchanged. Key creation can also accept
a capture file reserved before other planned creates; resolved parent IDs replace
pending context before the single POST. Closed/replaced/completed reservations
cannot issue another key. Full integration repeats fresh parent provisioning,
tool/ref/name, namespace/protection and key UUID/ref checks on saved writes.
Duplicate refs and legacy/incompatible map collisions fail before mutation.
Omitted collections grant no pruning authority; explicit empty scopes retire
only recognized generated keys. Foreign/imported/legacy keys are preserved.
Replacement key issuance and capture must succeed before old-key retirement.
Group names are create-only; principal scalar names and expiration updates
retain immutable markers and hydrate omitted expiration. Null expiration
survives manifests, saved plans and dump/reload.

Principal declarative delete/sync now observes basic-auth and API-key
descendants as well as identity associations. Every attached key must have an
explicitly authorized deletion in the final plan before parent retirement.
Unmanaged keys block deletion. Authorized key deletes become parent dependencies;
execution repeats complete empty checks. Failed observations, including a
legacy user-type 404, fail closed. Fixture cleanup does not extend production
deletion authority.
Q24 records the API's native descendant deletion; public metadata alone does
not provide independent protection against that operation.

There is a separate SDK preparation issue: canonical key create/update bodies
reference a mutable nullable expiration schema, while the merged preview's
shared `ExpiresAt` is read-only. Generated write models consequently omit the
field even though direct API writes work. Resolve that targeted schema/reference
difference and verify omitted/value/null wire semantics before enabling writes;
it is not evidence that the API lacks expiration support. SDK `79bc3dbbb928`
now corrects this gap; see the credential-expiration correction below. Backend
ownership restrictions still apply independently.

The live inspection scenario uses generated keys with explicit fixture metadata
under agent/unclassified principals, captures material privately, verifies
three request pages and scans artifacts for material and its Base64 form.
Fixture cleanup opts into API-key observation explicitly, checks every marked
collection before deletion, rechecks metadata before individual deletes and
requires empty collections before principal removal. Existing scenarios do not
gain a new credential permission requirement. Earlier US `.com` group-key GET
and POST returned 501 (Q33); the newer `.tech` deployment supports them. Their
shared schema alone is not sufficient to enroll a product lifecycle.

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
Password write models originally omitted expires_at due to the shared
read-only merged schema. SDK `79bc3dbbb928` corrects this separate composition
and serialization gap; it does not resolve Q35's backend metadata behavior.

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

## Credential expiration SDK correction

Related specification question: Q36. Preview SDK `79bc3dbbb928` separates the
canonical writable credential expiration schema from the merged read-only
IAM timestamp. Six request schemas gain their documented expiration property;
principal/group generated/imported keys reuse those same request bodies.

Five optional fields use the SDK's existing OptionalNullable date-time type:
unset omits the property, a date sends a timestamp, and explicit null sends
JSON null. Required password replacement keeps a nullable time pointer and
always sends the property, including null. The global generator configuration
and all existing public SDK fields remain unchanged. The snapshot records
patch source `190e5b57`; no ordinary KongCTL build performs these patches.

The wire regression has 30 cases. Before correction, 21 fail because request
models omit expiration. Correcting the schema alone leaves nine explicit-null
failures; the targeted wrappers resolve those failures. All 30 pass after full
generation. Two preparation tests verify scope, idempotence and drift checks.
An AST audit finds no removed or changed existing exported members/signatures
across 39,350 declarations; the 1,102-operation inventory is unchanged.

Native SDK requests in US `.tech` pass 84 creation/update/replacement and
fresh-GET expiration checks, with generated/imported principal/group keys and
passwords. Fixture secrets stay in memory and all fixture resources are
removed. These are administrative expiration checks, not runtime credential
continuity evidence (Q30). Operation-specific SDK server URLs require the
same environment routing used by existing KongCTL helpers.

This removes the SDK preparation prerequisite, not the backend limitations:
imported-key ownership is still unavailable, key/password metadata PATCH is
ignored, and omitted expiration still clears it (Q4/Q28/Q35). No key/password
product writer is enabled by this dependency change. Remove the scoped schema
and wrapper corrections together only when upstream composition/generation
passes the same omitted/value/null wire cases without changing unrelated
public SDK models.

## Directory Trusted IdP inspection

Related questions: Q6, Q22, Q37 and Q38. Native US `.tech` supports multiple
IdPs with directory-unique issuer URLs and duplicate display names. Full PUT
retains supplied configuration and resets omitted fields as documented.
Associated-principal reads include manually created OIDC associations. No
runtime provisioning or authentication guarantee follows from these reads.

SDK `30c3834c9846` changes only the two list request Page tags to deepObject,
from source `280bdecc`. The ten-operation wire regression fails exactly the
two new list methods before correction and passes afterward. Schema/model
inventory and prior expiration/array/default fixes are unchanged.

Public-only get/list commands use explicit directory selection and ID/exact
issuer lookup. Raw parsing preserves false and nullable configuration without
SDK defaults; missing required identity/configuration, duplicate IDs/cursors,
invalid page metadata and failed pages produce no partial inventory. Lookup
reads fresh detail after complete duplicate detection. No ownership is stamped.
Text always includes the full issuer because display names are not unique.

The live scenario verifies three IdP pages, two associated-principal pages,
all public configuration/output formats and declaratively managed principal
associations under an external directory. Inspection leaves native IdP
configuration unchanged. Token Vault must be enabled; disabled reads fail
with the observed 403 rather than claiming an empty collection.

Directory force=false deletes attached IdPs, even while disabled Vault hides
them. Production directory deletion policy remains under review (Q37).
Fixtures opt into requireEmptyTrustedIdPs before cleanup writes and before
parent deletion. This test-only check grants no IdP mutation authority and
does not change production directory deletion. Other scenarios have no new
Vault permission requirement. Missing/nonempty/error inventories fail closed.

PATCH jwks_uri null retains an override despite success, while PUT omission
clears it (Q38). A future declaration writer can use verified fresh full-PUT
hydration, but must establish ownership/protection, external-parent and scoped
sync rules before enrollment. IdPs expose no labels/managed_by store; matching
an issuer alone is not adoption. The engineering answers and direct omitted/
null/dependency probes must accompany any relaxation of these boundaries.

## Explicit basic-auth password lifecycle

Related questions: Q19, Q30 and Q35. Explicit create/patch/delete commands
operate separately from the username's declarative lifecycle. Generated and
imported creation require an exclusively reserved private output file. Its
public parent IDs identify the selected collection even when creation is
unconfirmed. Capture persists before public-field/read-back validation; no
automatic retry or rollback is performed after a possible issuance.

An expiration-only PATCH sends a date or null explicitly, because omission
clears it in the deployed API. Existing management metadata is observed and
verified. Supplying a material source explicitly selects full PUT: fresh GET
hydrates omitted expiration and the entire management map. Missing response
properties block a preserving PUT instead of becoming null/empty defaults.
SDK and transport retries are disabled for both secret-bearing POST and PUT.
Uncertain replacement requires inspecting the selected record and verifying
authentication before another attempt. Public read-back proves expiration and
metadata, not runtime material continuity.

These imperative operations never stamp ownership or adopt password records.
They can explicitly retire a selected record or every password under the
selected username with normal confirmation. Delete-all verifies a fresh empty
collection. Expired records still occupy slots; no automatic retirement is
inferred. Whole password-route trace bodies are suppressed, including errors
that could echo material outside a secret-named property.

The live scenario covers generated/imported private capture, file/environment
sources, exclusive file/symlink refusal, explicit expiration/null, preserving
PUT, arbitrary management maps, wrong parent scopes, two-slot limits, all
output formats, selected/all deletion and unchanged username plans. Username
declarative deletion refuses attached passwords. Runtime authentication needs
Q30's integration context; metadata-only password adoption needs Q35's API
contract. Correcting those contracts is necessary before considering a
declarative password resource; these commands do not claim that support.

## Token Vault contracts and list cursor SDK correction

Related questions: Q6, Q7, Q22 and Q37–Q40. Direct US `.tech` probes verify
Vault key/provider/template collections before declarative enrollment.
RSA/EC keys have RFC 7638 kid identity and non-unique names. Providers have
directory-unique names. Neither family has ownership metadata. Vault disabling
hides keys/providers with 403, and directory force=false cascades with an
attached key/provider in either Vault state. Existing directory deletion policy
is unchanged; ownership/deletion policy is still required before enrollment.

Provider PUT cannot replace the verified custom OAuth DCR configuration,
including when creation-time registration/DCR fields are supplied. PATCH
cannot clear scopes with an empty array. POST enabled:false is ignored, while
explicit PATCH false and PUT true work. PUT omission retains enabled but
resets a refresh buffer. Q39/Q40 capture the API-only requests and questions.
Public configuration probes do not establish stored OAuth secret preservation,
enrollment, DCR execution or runtime encryption/authentication effects.

SDK preview 2597fe249e26, from patch source bd2ee640, changes only the three
list request Page tags for Vault keys, providers and provider templates.
Bracketed queries traverse five keys, five providers and eight templates;
flat size is ignored. The actual-wire regression fails exactly those three
methods before correction and passes all thirteen Identity list operations
after generation. The SDK audit preserves prepared OpenAPI, global generation
configuration, module dependencies, response models and the 1,102-operation
inventory, including prior credential expiration/array fixes. Remove the
targeted replacements only when upstream generation emits the same documented
bracketed query contract. No product declarations or commands are enabled by
this dependency change alone.

## Directory deletion observations

Related question: Q37. Planning a directory DELETE or scoped sync requires
a fresh directory detail and observable Vault key, provider and Trusted IdP
collections. Keys must be empty or have exact authorized deletions in the plan;
provider/IdP collections must be empty. Execution requires actual empty
collections after the ordered child deletions. Unfiltered terminal observations
prove emptiness; planned key removals require complete validated inventories.
Missing APIs, malformed/incomplete observations, nonterminal emptiness and
read errors block deletion. Disabled/unknown Vault is an explicit error;
neither planning nor execution enables it automatically.

This is the existing fail-closed credential-deletion rule applied to directory
dependencies that the native API cascades even with force=false. It adds no
ownership to a key/provider/IdP. Key declarations separately establish explicit
managed-directory and collection authority; other attachments cannot cascade.
Ordinary creates, updates and external-directory principal workflows do not
require these extra observations. Tokens without the required read permissions
cannot prove safe declarative directory deletion. A deployment that cannot
retain vault_enabled:true cannot satisfy this deletion precondition.

Saved plans do not bypass the guard. A late attachment or disabled setting
blocks the operation; an already absent directory stays an idempotent success.
The SDK remains 2597fe249e26. Fixture cleanup can opt into observing all three
collections before any cleanup writes and again before parent deletion.
It never removes a remaining Vault child. Other fixtures gain no permission
requirement from that opt-in.

There is still no atomic observation/delete guarantee. Do not remove these
guards merely because a server returns success: require documented and live
verified conditional deletion across all dependent families and non-mutating
visibility of disabled-Vault state. Any alternate explicit cascade policy
needs separate ownership/deletion semantics and coverage.

## Credential Provider public inspection

Public provider get/list and global template discovery use the existing SDK
`2597fe249e26` cursor correction. Provider UUID/name resolution is directory
scoped, fully paginated and duplicate checked. Custom public response models
preserve false/empty values and reject missing required identity, settings or
pagination rather than synthesizing SDK defaults. Client secrets, credential
values and enrollment/state capabilities cannot enter these output models.
Ordinary inspection calls no credential or enrollment endpoint.

Direct `.tech` provider DELETE succeeds with an active shared credential, whose
following GET returns 404. Declarations must check that dependency before
mutation (Q41). The tested public PATCH response and a subsequent fresh GET
retain visible DCR registration configuration, but client-secret retention
requires runtime enrollment verification. The signed enrollment resolve route
returns 400 without mTLS context (Q30). No internal context headers were
fabricated. Configuration inspection does not prove stored-secret state.

Q42 records scope/endpoint clearing boundaries: create scopes:[] expands to
template defaults, PATCH scopes:[] fails, endpoint null fails, and endpoint
empty strings become actual empty effective endpoints. Do not interpret these
values as a verified reset-to-template operation. Q39/Q40 retain replacement
and initial-disable limits. Provider declarations use the separate inherited
authority model described below; public get/list grants no ownership.

ListVaultProviderCredentials still has flat SDK Page serialization. Its
complete cursor traversal needs a targeted correction before product credential
pagination can rely on it. A deletion guard can use the first unfiltered page
without pagination arguments, requiring explicitly terminal empty observations.

## Credential Provider write foundations

Internal helpers now supply validated create, PATCH and guarded DELETE for
declarative integration. They do not add declaration loading or new CLI write
commands. Create and writes disable both SDK and HTTP-client retries. Errors
omit backend bodies, and response projections contain public configuration
only. Create validates supplied public fields against the response; PATCH
checks fresh identity, requested settings and preservation of omitted public
fields in both its response and a subsequent GET. No PUT hydration is used.

The deletion helper repeats a fresh name/UUID observation and requires a
terminal empty credential collection. Its request has no filters, page or sort
arguments; missing, malformed, nonterminal or failed observations block the
write. It never reads credential detail or issues enrollment capabilities.
This is an observation guard, not an atomic guarantee against concurrent
attachments (Q41). The declarative executor separately enforces directory
ownership, namespace, protection and collection authority as described below.

Direct SDK-backed `.tech` verification covers persisted DCR/public PATCH,
explicit refresh zero, blocking deletion with an active shared credential and
successful guarded fixture cleanup. The public RSA fixture and all providers/
credentials were removed and default directory settings restored. A separate
49-request native probe verifies static-secret optional-setting persistence;
Q43 asks which settings have runtime meaning. Neither probe proves runtime
OAuth enrollment or stored-secret continuity.

Empty explicit scopes and endpoint strings are rejected rather than silently
interpreted as inheritance or clearing. `enabled: false` at creation still
needs a separately documented PATCH step; no atomic disabled create is
claimed. Provider declarations, write-only sources, collection pruning, dump,
examples and lifecycle E2E are now integrated as described below. SDK version
remains `2597fe249e26`.

## Credential Provider declarative lifecycle

The write foundation above now backs nested directory.vault_providers and
grouped identity.vault_providers declarations. Providers inherit a declared
managed directory's namespace and protection; external parent references do
not authorize writes. A complete directory-scoped name inventory determines
matching. Ref is local, names must be explicit, and no independent metadata is
invented. A name change creates a different resource; apply preserves the old
one and explicitly scoped sync may retire it after guards.

Create supports every declared create field and configured deferred
client_secret sources. An initial enabled:false is POST followed by PATCH,
with fresh parent checks; it is not atomic. A failed second write leaves a
created provider that must be inspected/replanned. Ordinary updates use PATCH
and omit write-only fields unless selected through the shared secret-source
framework. Secret-only updates retain directory ID, namespace and inherited
protection context. Plan and saved execution reject unsupported create-field
changes instead of silently dropping them: template, client ID, credential
type, DCR mode and registration endpoint. This client boundary does not assert
that those fields are immutable in every API operation.

Deletion requires fresh complete provider identity and terminal empty
credential observations during planning and execution. Directory deletion may
order exact planned provider retirements first; its final dependency checks
still require actual empty collections. Saved plans reject late namespace,
protection, provider-context and credential changes. No credential is
automatically deleted. The check cannot close Q41's concurrent-attachment
window without API support.

Dump emits public settings beneath enabled managed directories, omits secrets,
and pins effective endpoint values because stored overrides are not exposed.
Empty scopes are omitted only if the template also has no defaults; otherwise
dump fails as unreproducible. Invalid empty endpoints also fail export. Explicit
empty scopes/endpoints remain rejected in declarations under Q42.

Examples cover the existing and missing default-directory cases, static-secret,
static OAuth and custom DCR configurations, with deferred source syntax and
replacement/clearing limits. The live provider scenario covers public fields,
initial disablement, adoption, saved/secret-only updates, diff, scoped sync,
dump/reload, dependency guards and parent bootstrap/retirement. Public-state
and request tests do not prove stored-secret continuity, OAuth consent or
runtime dynamic registration; Q30/Q39–Q43 retain those API boundaries. SDK
version remains `2597fe249e26`.

## Trusted IdP write foundations

Public create, hydrated full PUT and guarded delete helpers are verified
against US `.tech`. PUT preserves every supplied public field, supports
clearing nullable display/JWKS overrides, and normalizes an empty JIT mapping
to no mapping. Omitted declaration fields must be hydrated from fresh state;
the PUT defaults must not reset omitted booleans. No secret hydration is needed.

Directory-scoped collection membership and the exact issuer are checked before
updates/deletion because detail responses have no directory ID. Deletion also
requires an explicitly terminal empty associated-principal observation. An
inactive principal still blocks deletion. Explicit identity retargeting or
deletion detaches the association. Trust deletion never authorizes mutation
of independently owned principals or identities.

Native deletion accepts associated records and leaves principals/identities
readable; Q44 asks about runtime behavior and conditional deletion. The client
cannot eliminate the race between a fresh empty observation and DELETE.
Write helpers make one mutation attempt and check public receipts/readback.
They establish no independent ownership metadata or declarative authority.

Evidence consists of 47 native requests and a separate 36-request probe using
the actual SDK-backed helpers. Fixtures were removed and the original default
directory settings restored. SDK version remains `2597fe249e26`; full trust
declarations, examples and lifecycle E2E remain the next integration work.

## Trusted IdP declarative lifecycle

Trust configurations now use nested `identity.directories[].trusted_idps` or
grouped `identity.trusted_idps`. Exact directory/issuer matching and explicit
managed-directory authority follow the Vault child pattern. No child metadata
is invented; external directories grant no write authority. Parent protection
blocks updates/deletion, and omitted collections grant no sync authority.

Public full PUT hydrates omitted settings from a fresh read. Nullable display
and JWKS declarations retain explicit-null presence through loading and saved
execution; `jit: {}` clears mapping and normalizes to no mapping. Supplied
mapping objects replace their entries. Fresh membership/issuer and directory
ownership/Vault checks repeat before writes; uncertain mutations are not
retried automatically. Q38 is handled through verified PUT, not PATCH clearing.

Final plans can order independently authorized OIDC issuer retargeting or
identity deletion before trust retirement, including across namespaces.
Principal deletion also counts only when explicitly authorized in its own
plan. Every matching identity must detach; ownership-only changes, unknown
associations or new attachments do not prove detachment. Saved execution
requires actual terminal empty associations, including inactive principals.
The root's final deletion requires actual empty Vault collections after exact
authorized key/provider/trust retirements. Q37/Q44 retain atomicity limits.

Dump exports public trust settings only under enabled managed directories,
including explicit nullable overrides and empty mappings. Examples cover
default-directory adoption/enablement and missing-default bootstrap. Runtime
JIT provisioning, token exchange and principal/group ownership remain separate
from administrative trust configuration. SDK remains `2597fe249e26`.

## Provider and principal Vault credential observation

Public get/list now covers provider credential inventories and a principal's
Vault authorizations. Provider reads use collection selection even for a UUID:
the API's shared OAuth credential detail GET issues an enrollment capability.
Narrow reader interfaces expose only list methods. Public projections retain
shared selector/priority/status or user principal/expiry metadata and exclude
secrets/capabilities. JSON/YAML and text are explicit; malformed, failed,
repeated or incomplete inventories yield no partial result.

Provider credential routes redact whole trace bodies, including errors or
unexpected capability fields. This follows existing secret/password route
handling. Principal Vault reads project provider/template/status/expiry fields;
they do not authorize adoption, sync or revocation.

Native `.tech` probes establish multiple shared credentials for static and
OAuth providers. Priorities/default slots are unique in the tested matrix;
selectors can repeat. Static PUT requires a secret, and omitted routing resets
to default. OAuth GET/PUT issue enrollment URLs. Q45/Q46 ask for the formal
cardinality and material-preserving update contracts. Actual user enrollment,
active OAuth continuity and revocation still need Q30's runtime setup.

SDK `90596b498bcb` extends the Q22 cursor fix to provider credentials, principal
Vault credentials and group keys. All sixteen wire methods pass with no skips;
the preceding SDK fails exactly these three methods. Generated-code changes
are limited to their three Page query tags. Snapshot/provenance and tests are
updated from KongCTL patch source `c775a9c4`. Native provider/group inventories
traverse five/three records at page size one. Empty principal Vault queries
honor size one; populated user pagination has not been live exercised.

## Static shared Vault credential write foundations

Guarded helpers now support create, explicit material replacement and deletion
for verified `static_secret`/`shared` providers. They recheck directory/provider
UUIDs, exact provider name and mode, then observe every credential page before
writing. Duplicate priority/default slots, unexpected user/pending records,
missing targets and stale routing block mutation. No credential detail GET is
used. These helpers do not establish directory ownership or protection; the
declarative executor must supply that separate authority before calling them.

Replacement always receives explicitly supplied material. Omitted routing in
the helper call preserves the freshly observed selector/priority; an explicit
empty routing pair requests the default slot. Both the planned routing and
target UUID must still match. Routing collisions fail before PUT. Every
mutation makes one attempt through both retry layers; public receipt and
collection readback must agree. Deletion verifies the UUID is absent afterward.
Public observations cannot prove the value of stored material or its runtime
usability, and no such guarantee is reported.

Actual helper calls in US `.tech` pass three creates, three replacements and
three deletions, with three conflicts rejected before mutation. This includes
default and priority-zero slots, repeated selectors at different priorities,
secret-only routing preservation, selector changes and a priority move. No
credential detail GET occurs. A registered encryption key is required for this
fixture: creation returned HTTP 400 without one, and succeeded after adding
it. All fixtures are removed and original default-directory settings restored.

No declarations or new imperative write commands are exposed by this slice.
Remaining integration includes deferred `!secret` sources, explicit replacement
selection, fresh inherited directory authority/protection, key dependencies,
scoped sync/parent retirement ordering, dump and examples/live E2E. SDK
`90596b498bcb` already supplies these mutation methods; no SDK change is needed.
Q45/Q46 still ask for the formal routing and material-preserving API contracts.

## Static shared Vault credential declarative lifecycle

The write foundations above are now integrated as
`identity.vault_provider_credentials` or nested provider `shared_credentials`.
They accept only explicitly declared static_secret/shared providers beneath a
managed directory. Matching uses the default or nonnegative priority slot;
selectors may repeat and refs are local. Duplicate observed slots or unexpected
user/pending variants fail before mutation. No ownership metadata is invented.

Create requires deferred `!secret`. Existing material is never compared or
implicitly rewritten. A changed selector at an existing priority requires
explicit secret-write selection because native PUT requires material. Priority
changes create another declared record, with scoped sync creating the new slot
before deleting omitted slots. Secret-only saved changes retain complete fresh
parent/routing context. Missing or stale authority, protection, provider mode,
routing, source or encryption-key availability blocks execution. Large routing
integers retain precision through saved-plan decoding.

Directory, provider and encryption-key creates precede credential writes.
Planning requires a registered or planned retained key; execution verifies an
actual key exists. Provider retirement can depend on exact explicitly planned
credential deletions; saved execution still requires actual emptiness. A newly
attached credential blocks the parent. This cannot close the native final
attachment/delete race (Q41). Omitted collections preserve credentials; explicit
nested empties scope retirement. External lookups remain read-only for them.

Dump exports public static routing under explicitly managed enabled directories
and omits material/source bindings. Existing slots reconcile without a source;
recreation requires adding one. OAuth/user authorizations are not enrolled or
pruned by static declarations. Q30/Q45/Q46 still bound runtime and formal API
contracts. SDK remains 90596b498bcb; no SDK revision is required.

The live CLI lifecycle covers deferred file/environment sources, missing-source
preflight, public/default/priority-zero settings, local-ref changes, grouped
loading, explicit replacement, stale namespace/protection/routing/key guards,
selector updates, priority replacement ordering, dump/reload, omission/scoped
sync, late-credential parent guards, ordered delete and bootstrap from no
directory. Public output/trace/artifacts exclude material and enrollment URLs.
Early scenario failures corrected explain discovery and test execution context;
no new API defect was inferred. See the dated report for final verification.

If a verified metadata-only routing operation becomes available under Q46,
reconsider selector-only updates requiring secret selection. Verify preservation
against an actual supported runtime fixture before changing that contract;
ordinary material replacement must still require explicit intent. Formal
confirmation of Q45 can resolve the documented uniqueness caveat without
changing duplicate detection. New ownership fields would require a separately
reviewed migration, not automatic metadata stamping on existing authorizations.

## Explicit shared OAuth enrollment and retirement

Shared OAuth credential creation and deliberate URL reissuance now follow the
existing private capture pattern: reserve a new 0600 file before the capability
request, retain uncertainty evidence, never overwrite or automatically retire
on capture failure, and return public metadata only. A separate narrow API
exposes detail GET exclusively to deliberate enrollment issuance; normal reads
still select through complete lists. Both POST and capability-producing GET
disable SDK and transport retries. The private record uses enrollment_url,
not secret, and carries directory/provider/credential IDs for recovery.

Provider UUID/name/shared mode, authorization_code template, complete collection
membership and routing uniqueness are checked first. Creation has no secret
input and must return pending metadata. Capture means the URL was saved, not
consent completed. Pending create/detail GET work without a registered key in
US .tech; this does not prove consent/runtime retrieval works without one.
Reissuance for active records is described by the API and unit-tested, but the
live fixture remains pending. OAuth routing PUT stays deferred under Q30/Q46.

Explicit shared record deletion supports static and OAuth providers, proves
membership without detail GET, rejects user authorizations and verifies UUID
absence. It does not prove upstream token invalidation. Like existing explicit
password/client-secret commands, these operations do not enforce declarative
namespace/protection or stamp ownership. Static declarations and their scoped
sync remain separate; OAuth/user records are not implicitly adopted or pruned.

Native pending-create/reissuance observations are recorded in
shared-oauth-enrollment-contracts.json. Q47 requests URL lifetime, reuse and
invalidation rules; Q30 still requires a supported runtime integration for
actual consent, authorization continuity and user revocation. Existing SDK
90596b498bcb already exposes all operations, so no SDK revision is required.

## Importer adoption versus retained principal ownership

Native org-chart import can adopt an existing user through its reserved email
identity. It retains UUID, managed_by and unrelated metadata while changing
display name and provisioned_by to imported. Ordinary principal PUT/DELETE
then return 409. Retained KongCTL metadata therefore does not establish that
the principal is still writable through the normal declarative lifecycle.

Explicit managed declarations, scoped pruning and adoption now reject that
provisioning state, including matching records whose fields otherwise need no
change. Saved update/delete execution repeats the fresh check. Omission without
collection authority leaves such records alone; ordinary unowned imported
records remain outside reconciliation. Dump emits imported principals as
external UUID references even when old ownership metadata survives. Their
independently owned child resources retain their existing scope rules.

This restriction is specific to the observed imported state, not an inferred
policy for all provisioning types. Native explicit deactivation transitions an
imported principal to konnect_user and permits deletion; the fixture verifies
that operational release path without silently applying it in declarative
management. Q48/Q49 request canonical import/matching/release contracts. Q50
records the public enum/adoption-count schema gaps. The Go SDK's open string
enum preserves imported observations, so no SDK compatibility change is
required for this guard.

Revisit the restriction only with a documented, verified administrative
ownership-transfer contract. Do not turn rejection into an automatic active
change or treat retained managed_by as permission to release imported users.
The explicit import workflow below does not define an import declaration or
complete the broader arbitrary-principal CSV request #1419.

## Explicit organization-chart import and private reports

Create identity org-chart-import uploads one complete CSV through the existing
SDK operation. Exactly one dry-run or execute flag is required, with explicit
directory selection and a new private report file. UTF-8, CSV syntax and the
observed 1–50000 data-row bound are checked locally; native canonical/hierarchy
validation remains authoritative. The bytes are read once and hashed for local
correlation. The operation does not expand declarative tags or upsert rows
through an unrelated client loop.

Reports reserve an exclusive 0600 path before the POST. Complete successful
native reports, including unknown fields and counts.users.adopted missing from
the typed SDK, are captured before interpretation. Available API error bodies
or SDK/transport diagnostics are also private. Unexpected non-JSON bodies are
stored as base64; unrecoverable/read failures may leave only request context.
The reserved file is retained on uncertainty. Capture never triggers a second
upload, rollback, directory cascade or automatic resource retirement.

Both retry layers are disabled for preview and execution. Public output shows
counts and value-free diagnostic rows/types/fields. Only canonical column names
and known warning codes are exposed; unknown values become other. Arbitrary
diagnostic messages stay in the private file. Import request/response bodies
and body-read/transport error text are redacted in traces. HTTP 200 with row
errors returns a safe summary and nonzero exit status after capture; no general
runtime rollback guarantee is inferred from those reports.

Preview is not a saved plan or transaction reservation. Each execution applies
its complete file to current state. Imports may adopt existing users and
deactivate absent imported users without changing their opaque managed_by maps.
They use explicit API authorization rather than declarative ownership rules.
Normal principal guards continue to reject current imported provisioning.

Q51 records a verified return behavior: a previously omitted user keeps its ID
and remains inactive/konnect_user when reintroduced. The API emits
inactive_not_reactivated and counts that record as unchanged. The command
surfaces the warning without automatically setting active or claiming that a
completed report means every user can authenticate. Revisit only with a
documented and verified API contract. Q48/Q49 still cover source isolation and
ownership transfer; Q50 covers the public enum/adoption-count schema gaps.
No SDK revision is needed: the existing multipart operation is wire-tested and
the complete response is projected from raw JSON.
