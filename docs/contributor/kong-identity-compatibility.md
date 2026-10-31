# Kong Identity compatibility tracking

Track temporary implementation restrictions here and in the resource issues.
API questions
for engineering stay in the separate
[question log](../kong-identity-api-questions.md), without client-specific
implementation requirements.

## Principal type changes

Related API question: Q9. Creation supports user, agent, and unclassified/null
types, but tested PUT/PATCH requests did not change an existing type.

- Initial contract: type is create-only. Preserve it when omitted on update;
  reject requested changes, including a requested reset to null.
- Removal trigger: engineering confirms the intended contract and direct HTTP
  tests verify supported changes in the target environment.
- Required verification: user to agent, agent to user, and classified to null;
  saved plans; omitted-field preservation; no-op reapplication; dump/reload.
- Keep the guard explicitly tied to Q9 rather than treating immutability as a
  permanent API guarantee.

## Principal active state

Related API question: Q10. Tested responses omitted active. The generated
SDK's plain bool cannot distinguish absence from false.

- Initial contract: defer declarative active configuration. Inspection omits
  absent values in JSON/YAML and shows unknown in text. Never dump a fabricated
  false value or write one while preserving unrelated settings.
- Removal trigger: engineering confirms deployment and provisioning rules;
  direct create/GET/list/update tests return observable active state and verify
  true/false transitions.
- Required verification: omitted versus false, activation/deactivation,
  provisioning restrictions, saved plans, and dump/reload.
- Retain response-presence handling for compatibility with older environments
  even after enabling active configuration.

## Principal management metadata PATCH

Related API question: Q11. PUT retained management metadata updates; PATCH
returned success while leaving a requested version change unchanged.

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

The principal resource validates deferred `active` input in
`internal/declarative/resources/identity_principal.go`. Type changes are
rejected in `planner/identity_principal_planner.go` and rechecked against a
fresh observation in `executor/identity_principal_adapter.go`. These guards
reference Q9/Q10 explicitly.

`helpers.IdentityPrincipalReplacement` builds PUT requests without `active`.
The principal executor hydrates omitted fields from a fresh detail response,
then overlays only saved plan fields. Adoption uses the same replacement
builder. Reads preserve response presence independently of SDK defaults.

Coverage lives in the principal loader, planner, executor, adoption, and read
helper tests. `test/e2e/scenarios/identity/directories` exercises the live
principal lifecycle, including user/agent/null creation, repeated no-op plans,
PUT updates, omitted fields, metadata clearing, dump/reload, adoption,
protection, deletion, and scoped sync. Q9/Q10 rejection assertions should be
replaced with supported behavior when those restrictions are removed.

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
