# Declarative engine implementation guide

This is the primary implementation guide for agents adding resources,
extending declarative features, or refactoring the engine. It defines the
required behavior and integration points; linked code supplies current
signatures and working implementations.

Follow [repository guidance][agents] and the maintainer's task constraints.
Use [declarative usage][usage] and the [resource reference][reference] for the
user-facing contract. Imperative commands are separate scope.

## Start here

1. Define the affected contract: accepted YAML, identity, supported operations,
   parent scope, defaults, references, secrets, and observable API fields.
2. Trace that contract through loading, planning, execution, and dump. For a
   new resource, follow each implementation step below. For a field or engine
   feature, identify every affected step rather than copying a whole resource.
3. Choose existing code by lifecycle and API semantics. Check its tests and
   known compatibility behavior before using it as a migration example.
4. Establish the existing validation baseline. Keep refactoring separate from
   changes to accepted manifests, API requests, or saved-plan behavior.

### Engine map

| Layer | Owns |
| --- | --- |
| [Resources][resources] | Typed declarations, identity, schema metadata |
| [Loader][loader] | Sources, templates, tags, scope, decoding, validation |
| [Planner][planner] | Observation, differences, references, dependencies |
| [State client][state] | API access and normalized observed resources |
| [Executor][executor] | Payload checks, reference hydration, API mutations |
| [Dump][dump] | API-to-declarative conversion and export |

A resource declaration, SDK request, observed response, and planned change
are different representations. Do not use resource serialization as an API
request: it can contain `ref`, `kongctl`, children, and parent selectors.

The [resource registry][registry] drives iteration, aggregation,
explain/scaffold, load-schema discovery, namespace participation,
collection scope, registered child loading, ordered collection validation,
and dump-default metadata.
The [root planner inventory][roots] drives root construction and dispatch.
[Runtime executor registration][runtime-executors] supplies action routing and
payload validation for SDK resource operations. [Coverage checks][coverage]
tie root planners, SDK executors, and all managed dump dispositions to scope.
The [dump collector inventory][dump-collectors] supplies supported selectors,
help, and dispatch. The [API][api-child-dump], [AI Gateway][ai-child-dump],
[Event Gateway][eg-child-dump], [Portal][portal-child-dump], and
[control-plane][cp-child-dump] child inventories guard export ownership.
Grouped loading, specialized namespace selection,
relationships, pre-execution validation, state-client wiring, and other child
dump traversal remain separate steps.
Registering a declaration does not complete those steps automatically.

## 1. Define and register the resource

- Add the canonical `ResourceType` and storage in
  [`ResourceSet`][resource-types]. Include nested parent fields only where the
  manifest supports nested declarations.
- Implement the [resource interfaces][interfaces]. Use `BaseResource` and
  existing matching helpers where their behavior fits. Keep `ref`, remote ID,
  and API moniker distinct; root identity is not universally name-only.
- Register in the resource file's `init()` with `registerResourceType` and
  `AutoExplain[...]`. External-capable resources use
  `registerExternalResourceType`; see [references](#references-and-lookups).
  Use slice-accessor variants when storage is behind a grouping object.
- Add planner resource aliases and new `Field*` identifiers in
  [planner constants][constants]. Use typed `ResourceRef.Kind` values and
  constants for plan fields, references, required fields, and executor access.
  Keep API keys and JSON/YAML tags literal unless also internal identifiers.
- Implement validation, dependencies, and supported label access. Prefer
  shared matching/normalization helpers over new reflection or type switches.
  Registry-driven aggregation must not acquire another resource inventory.
- Namespace-bearing declarations add `WithNamespace` beside registration;
  see [namespace capabilities](#namespace-capabilities). Ordinary managed
  children inherit namespace and protection from their parent; do not give
  them independent `kongctl` configuration.
- Follow the [maturity policy](maturity.md). Resources default to GA.
  Co-locate `WithMaturity` and narrower `WithOperationMaturity` overrides
  with registration. Maturity is discovery metadata, not runtime gating or
  plan, result, or telemetry data.

### Namespace capabilities

[`WithNamespace`][namespaces] supplies a typed `NamespaceParticipant` accessor
using the resource's registered slice. Return the actual `Kongctl` field's
address, `ref`, diagnostic label, external state, and protected support.
Keep traversal order stable: defaulting and validation use it for diagnostics.
Each order must be unique; leave gaps when adding participants.

Supply supplemental grouped locations when values exist there before nested
extraction; dashboard and organization team registrations demonstrate this.
`ForEachNamespaceParticipant` visits flattened then grouped values per kind.
`NamespaceValues` reads post-extraction values for loader validation without
revisiting those groups. Organization users/system accounts use
`registerNamespaceSelector`: they carry namespace without protection and do
not enter the ordinary resource registry.

Defaulting, namespace enforcement, and planner discovery share participation
but retain their different external-resource policies. `WithNamespace` also
supplies [desired-resource namespace selection][selection] for
managed roots, using their registered flattened collection.

Use `GetResourcesByNamespace[APIResource](rs, namespace)` to select typed
resources. It returns shallow copies in source order, with nil for no matches;
supplemental grouping locations are excluded. External roots belong to
`NamespaceExternal`; other roots use their declared or default namespace.
Existing root namespace getters delegate to this shared policy.

API children and Portal pages use `WithNamespaceFrom` beside registration.
Supply a typed owner lookup, as in [API versions][api-version]. The child
inherits the owner's registered selection policy. Preserve exact reference
matching and first-match behavior; missing owners exclude the child. Namespace
ownership need not match structural parenthood or sync ownership.
The registry contract checks that every namespace-owner chain reaches a
registered namespace root without cycles. Call the generic selector only for
kinds with selection; using an unsupported kind is a programming error.

Organization assignment selection retains its specialized rules. Resources
already selected within a parent collection do not need an additional
namespace getter; avoid adding unused accessors or planner wrappers.

### Defaults and SDK shape

Required fields for a managed operation must be explicit in manifests.
`SetDefaults()` may apply documented literal API defaults only. It must not
derive required values from `ref`, `name`, or another supplied field. Existing
name-from-ref fallbacks are compatibility behavior whose removal needs
separate scope.

AI Gateway roots and name-bearing children require explicit API names in
both nested and root-level declarations; a local `ref` never supplies a
missing `name`. Keep this contract in resource validation, explain/scaffold,
and dump output, including every SDK union variant. Supported generation of
a local `ref` from an explicit name is a separate behavior.

Embed generated SDK types where the declarative shape matches, using inline
JSON/YAML tags. Handle optional pointers, enum values, unions, and label
conversions explicitly. A custom unmarshaller must preserve accepted fields
and reject unsupported input; SDK unmarshallers can silently discard keys.

## 2. Wire loading and discovery

Inspect [loader parsing/extraction][loader] and
[resource-set validation][load-validation].
Wire nested extraction, parent selectors, ref generation where supported,
resource validation, and cross-reference checks for the new shape. Preserve
the running duplicate-ref index across files.

The loader carries execution and placeholder-preserving representations.
Keep defaults, extraction, and template handling consistent across both.
Capture sync scope before extraction loses YAML key presence. Shape
validation must run before resolving ordinary environment values so invalid
input cannot disclose a secret in an error.

### Collection validation

Register root collection rules with [`withCollectionValidation`][collections].
Supply a positive phase and a typed validator factory. The factory creates
fresh uniqueness state per pass; its visitor receives each resource through
registered storage, including grouped accessors. Use
`namedCollectionValidation` for type-wide name uniqueness with the resource's
actual name accessor. AI gateways and dashboards retain namespace-aware
rules; APIs add nested checks after each root's identity checks.

Managed roots must register validation or an explicit omission reason.
Event Gateway roots retain their existing validation outside this collection
pass. Do not add earlier validation as part of registering that exception.

[`ValidateRegisteredCollections`][dispatch] derives dispatch from
the resource and selector registrations. Phases sort numerically; roots and
selectors run at order zero, followed by children in `validateOrder`.
Families register a default with `withChildValidationPhase`; selector handles
bind their phase with `withValidationPhase`. A child can set `validatePhase`
to preserve interleaving with another family without requiring a family
default. Missing inherited phases, missing selector phases, and duplicate
positions fail when the dispatch is first assembled after init;
the result is cached. No additional loader invocation is required.

The loader normalizes organization team selectors before this pass, then
checks cross-references and namespaces after it. Preserve first-error order
across phases and within each collection. Existing loader validation methods
needed by tests delegate through `ValidateResourceCollection`.
The [collection-validation contract test][validation-contract] pins the full
diagnostic sequence and exercises registration guards.

### Child loading capabilities

For parents implementing `Resource`, register child loading beside declarations
through [`registerChildResourceType`][child-load], reusing root storage.
Children with external resolution compose `withChildLoad` with
`registerExternalResourceType`, sharing the same storage accessor;
[gateway services][gateway-service] show this path.
Supply `nested`/`setParent` with an optional `beforeAppend`, or a
typed `extract` handler for exceptional sources. Custom extraction receives
the registered destination; preserve each source's copying and storage rules.

All 16 AI Gateway child kinds pair extraction with validation. Direct children
use [`registerAIGatewayChildResource`][ai-child-load] for gateway-local moniker
uniqueness; data-plane certificates retain `title` diagnostics.

API versions, publications, implementations, and documents register in that
order. [`apiChildLoad`][api-child-load] shares ordinary extraction and both
validation phases. [Documents][api-document] use custom preorder flattening:
nested documents remain under their API; `FlattenRootAPIDocuments` separately
normalizes root documents. Preserve parent-document selectors, explicit child
API overrides, and allocated empty document slices in both locations.

Portal registers fourteen extraction paths and its twelve existing child
validators. [Singleton extraction][portal-child-load] copies a present value,
sets its portal, appends it, and clears the source pointer. [Pages][portal-page]
retain preorder flattening; [email templates][portal-template] retain
map-key defaults. [Teams][portal-team] extract roles and group mappings before
appending the team, carrying both team and portal selectors. Only teams nested
under portals enter that extraction phase; root-declared teams do not.

Event Gateway's [`eventGatewayChildLoad`][eg-child-load] registers extraction
for static keys, TLS trust bundles, schema registries, backend clusters,
listeners, and data-plane certificates, in that order. Extraction stays before
deferred-value indexing, preserving attribution to each child ref. Virtual
clusters and policy placement retain their existing storage and traversal.

Control planes extract gateway services before data-plane certificates.
Validation keeps services in the family phase (90), audit-log destinations
in phase 100, and certificates in their explicit phase 110. Preserve this
interleaving when assigning a new child's phase; registration supplies its
invocation automatically.
Gateway services retain external lookup metadata and their accepted inline
forms; [certificates][cp-certificate] retain parent lookup, external-placeholder
handling, and per-control-plane certificate identity checks.

Organization team roles use ordinary child loading, invoked only for teams
under `organization.teams`; teams already in flattened root storage retain
their nested roles.
Users and system accounts remain selectors outside the resource registry.
Their [typed selector loaders][selector-load] bind each source and selector
validator through `registerSelectorLoader`. Pass that handle to
`registerSelectorChildResourceType` when registering an assignment, with its
storage, extraction, and validation; [user assignments][org-user-load] provide
an example. No per-assignment loader branch is needed.
`ExtractRegisteredSelectorChildren` visits selectors in source order, extracting
memberships before roles for each. Registered validation checks all selectors,
then memberships, then roles. Team roles precede the user family, then the
system-account family. Preserve that order,
the missing-group diagnostics, and team-selector normalization before resource
validation, including sync-scope parent rebinding.

Portal teams, team roles, and these Event Gateway children record
`validationOmittedReason` with zero `validateOrder` to preserve their lack of
family-level loader validation. Other registrations require a validator and a
positive order; these exceptions do not authorize skipping new-resource
validation. Portal assets retain separate handling outside these registrations.

Extraction order is per immediate parent; validation order is per family,
including grandchildren. Participating orders must be positive and unique
within their group, independently of scope-capture order. Registration rejects
conflicting extraction forms, missing validation dispositions, and order
clashes.

The loader still owns extraction sequencing. Ordinary
`ExtractRegisteredChildren` handlers copy children, overwrite their parent
selector, append after existing
root values, and clear nested fields. Recursion is explicit: Config Stores
extract secrets before appending the store, ahead of secrets from root-declared
stores. Secret defaults still run before source indexing; consumer credentials
are extracted later. Preserve these phases in both loader representations.

`ValidateRegisteredNestedChildren` runs optional per-parent callbacks in
extraction order. API root validation invokes it after each API's identity
checks, preserving parent-context errors and cross-kind ref checks. Ordinary
API child callbacks retain validation before extraction; normal loading leaves
only documents nested. The registered collection pass then checks
root child collections in phase/order sequence, stopping at the first error.
API and ordinary Portal ref validation both validate each resource before
checking later siblings for duplicate refs.
Portal child validation follows API-child validation. Grouped root extraction,
cross-references, and namespaces retain explicit loader paths.

### Explain, scaffold, and load schema

Every resource must support `kongctl explain` and `kongctl scaffold`.
Its [explain metadata][explain] also defines the
[runtime load schema][load-schema],
so it must describe accepted declarative YAML, including kongctl-only fields,
parent selectors, nested children, and supported unions.

- Prefer `AutoExplain[...]` with narrow hints. Include required fields and
  useful commented optional fields in scaffolds. Review canonical resource
  paths and supported nested paths.
- `WithExplainSchemaBuilder` replaces the schema completely. Prefer deriving
  SDK branches with `autoExplainConcreteNode` and applying small overlays.
  Document unavoidable replacements and validate recursive parity for fields,
  requiredness, object/array shapes, unions, and `additionalProperties`.
  Name intentional differences so SDK drift is reviewable.
- Objects are closed by default; maps remain open. Use `LoadOpaque` only for
  a field whose custom unmarshaller intentionally accepts an opaque value.
  It disables shape traversal for that field.
- Use `ExplainNode.rejectLoadField` for recognized fields intentionally
  rejected with migration or branch-specific guidance. Ordinary unknown
  fields should remain unknown-field errors.
- Schema diagnostics identify paths without including input values.

### Sync scope

Sync deletion follows explicit manifest scope:

| Input | Meaning |
| --- | --- |
| Omitted collection or singleton key | Ignore that scope |
| Root collection `[]` | Desired count zero in selected namespaces |
| Nested child collection `[]` or map collection `{}` | Zero for that parent |
| Root-level empty child collection | Reject: no parent identifies scope |
| Non-empty singleton object | Manage that singleton |
| Singleton `null` | Reject; no implicit reset/delete |
| Optional, delete-capable singleton `{}` | Zero for that parent |

Co-locate [scope capabilities][scope-capabilities] with registration:

- `WithRootSyncScope()` handles root collections, including grouped
  `organization.teams` and `analytics.dashboards` declarations.
- `WithChildSyncScope(ownerType, options...)` handles child collections whose
  sync owner matches `GetParentRef()` and a root-only parent relationship.
  Owners may themselves be children; nested paths follow that ownership chain.
- `WithChildSyncScopeFrom(ownerType, accessor, options...)` supplies a typed
  owner accessor when the sync owner differs from `GetParentRef()` or that
  interface is absent. Portal team roles use their portal as sync owner while
  retaining the team as their structural parent.

The shared [declaration structure][declaration-structure] supplies root and
nested YAML keys for scope and explain. Relationship descriptors supply the
parent selector. Do not duplicate these facts in loader or planner inventories.
Scope descriptors are derived and checked once, after resource initialization.
`SyncCollections` returns copies, including root and nested paths. The loader
decodes key presence and delegates to [resource scope capture][scope-capture].
Planner fallback infers scope from populated slices and retains any explicit
`SyncScope`; neither path needs another resource inventory.

Child options preserve compatibility policies beside registration:

- `WithEmptyRootCollectionError(order, message)` retains AI Gateway's
  loader-stage rejection of empty root child collections. Error orders must
  be positive and unique. Other families retain planner-stage validation.
- `WithNestedScopeWithin(rootType)` limits nested capture to that root's
  declaration paths. Event Gateway policies retain capture under
  `event_gateways`, while AI credentials/secrets also capture scope under
  root-declared consumers/config stores. Root-level policy declarations and
  planner inference still use their immediate owner.
- `WithNestedCoScope(kind)` adds a related kind during nested presence capture
  and fallback inference. Portal teams also scope portal team roles. Root
  `portal_teams` presence alone does not capture role scope in the loader.

All ten roots and 53 child kinds register scope. Exceptional shapes retain
explicit policies:

- `WithNestedSyncScopeCapture` attaches a root's presence policy. The
  [Portal policy][portal-scope] derives direct child and singleton/map keys
  from declarations, including shared schema diagnostics. It preserves
  per-portal null checking, asset handling, and portal-owned team group
  mappings. New shapes must preserve those diagnostics and scope boundaries.
- Organization users/system accounts use `registerSyncSelector` beside their
  declarations, with a typed source and group marker. Their assignments use
  `WithSelectorAssignmentSyncScope`. Loader presence captures flat assignment
  parent keys and explicit selector groups; fallback inference marks populated
  selector groups instead of inventing per-parent child scope. These selectors
  do not enter the ordinary resource lifecycle.

Review [parent-scope validation][plan-scope] and external-parent support for
new owners. Structural containment alone does not define sync ownership.

Keep specialized empty-input diagnostics at their existing phase. For
delete-capable singletons, preserve scope while dropping the empty desired
value during decoding. Do not apply that meaning to update-only singletons.

Root dispatch applies `shouldPlanRoot`. Parent planners must apply
`shouldPlanChild` before child observation and pruning. An external parent
can have managed child scope while its own resource remains unmanaged.

A directly constructed `ResourceSet` cannot distinguish an omitted collection
from an explicit empty collection through its slices alone. Set `SyncScope`
explicitly when exercising empty-collection sync behavior.

## 3. Implement observation

Use [state-client configuration][state] and resource-specific state files to
wire the API dependency, normalized response type, pagination, and supported
operations. Check `ClientConfig`, `NewClient`, SDK helper interfaces, and the
[CLI integration][cli] that supplies clients.

Managed listing must filter by the intended namespace and normalize managed
and user labels. External lookup uses unrestricted observation instead.
Inspect [planning caches][cache] before adding reads. All ten managed-root
families use [`observationCache[T]`][observation-cache]: a typed fetch function
and namespace accessor supply resource-specific behavior; the cache owns
namespace queries, reuse, and filtering. Compatible managed-root observations
should use this boundary. Initialize their typed cache in
`newPlanningResourceCache` and keep planner listing methods as thin adapters.
Portal child caches remain separate and keyed by portal ID.

Namespace planners share observations within a run; `GeneratePlan` resets the
cache on every invocation. Preserve request counts and order, wildcard reuse,
fanout behavior, result order, and nil/empty slices. An empty namespace query
returns an empty result without fetching. Successful empty observations are
cached; failed reads are not. A nil cache preserves uncached reads, including
returning the full wildcard result during fanout. Keep missing-client behavior
and error propagation unchanged. Observation does not choose lifecycle actions.

Return errors with operation/resource context. Let callers report errors.
Use existing structured HTTP logging context and useful debug metadata;
do not add raw payload or secret values to diagnostics.

## 4. Plan the lifecycle

Choose identity and operation semantics before selecting a reusable strategy:

- **Managed roots matched by name:** [auth strategies][auth-plan] and
  [DCR providers][dcr-plan] use [`reconcileManagedRoots`][reconcile].
  Their adapters fetch and normalize typed values; the reconciler owns
  create/update/delete selection, protection transitions, error accumulation,
  and sync pruning. Diffing and change construction remain resource-specific.
  Its existing `FieldError` handling applies to ordinary updates; changing
  protection-transition validation is separate compatibility work.
- **Roots with other matching rules:** [dashboard planning][dashboard-plan]
  demonstrates explicit-ID/name matching. Preserve identity precedence,
  ambiguity handling, and matching scope.
- **Name-matched children with detail lookup:** AI Gateway model providers,
  auth strategies, policies, agents, models, vaults, consumer groups, and MCP
  servers use [`reconcileNameMatchedChildren`][child-reconcile] within an
  existing parent.
  Match only by the declared API name; UUID refs and cached IDs cannot
  override it. A changed name declares a different resource.
  Missing detail responses schedule creation; read or comparison errors stop
  planning. Sync retains declared names and stops at the first protected
  deletion. Pruning defaults to observed order; an optional `pruneOrder`
  adapter reorders the same observations only when pruning begins. Matching
  always indexes original observation order, with the last duplicate winning.
  Typed adapters own detail reads, comparison, payloads, and dependencies;
  [agents][agent-plan], policies, and [consumer groups][consumer-group-plan]
  also bind the observed ID before the detail read. Consumer groups fetch
  membership only when declared and after a successful detail read, using the
  listed ID. Their typed observation carries detail and membership together;
  policy/consumer creation dependencies remain in the adapter.
  Callers retain scope checks and new-parent creation. This strategy
  does not add child delete-mode dispatch, update protection, or child
  traversal. Collection matching and pruning delegate to the strategy below.
- **Dependency-ordered children:** [AI Gateway MCP servers][mcp-plan] pass
  sources before listeners to the shared reconciler and provide `pruneOrder`
  to delete listeners before sources. Write adapters derive dependencies from
  changes already planned. Keep the original observations as matching input;
  sorting them for deletion before indexing changes duplicate-name matching.
- **Name-matched collections with resource-specific reconciliation:**
  [Consumers][consumer-plan] and [config stores][config-store-plan] use
  [`reconcileNameMatchedCollection`][collection-reconcile]. It owns indexing,
  desired-order traversal, sync retention/pruning, and optional deletion
  protection. The typed `reconcile` callback receives the listed match or nil;
  it owns parent actions and nested child planning, including after a no-op.
  Errors stop traversal before subsequent desired resources or pruning.
  Consumer adapters bind IDs before detail reads and recreate missing details
  with credential creation dependencies. Stores use list results, sparse
  display-name updates, and scoped secret planning; they have no deletion
  protection. Keep new-parent creation and child scope checks with callers.
  The `currentName` boolean controls matching eligibility only: consumers
  exclude empty observed names from matching; stores retain their existing
  empty-name behavior. Both still consider those observations during pruning.
  Matching uses the last eligible observation per name; pruning uses original
  order unless explicitly overridden. The detail-read strategy above delegates
  to this collection traversal; prefer it when its narrower contract fits.
- **Replacement-only credentials:** Credentials match names within their
  consumer. Refs and cached IDs do not override names or retain other names
  during sync. Keep ordered delete/create replacement and its dependencies
  explicit; this is not a mutable detail-read lifecycle.
- **AI Gateway certificate families:** Data-plane certificates match by
  required title; runtime certificates, CA certificates, and SNIs match by
  required name within their gateway. Refs and cached IDs do not select or
  retain another title/name. Preserve first-match duplicate-title warnings
  and data-plane replacement ordering. TLS orchestration keeps SNI
  dependencies on certificate creation/updates and delays certificate
  deletion until referencing SNIs are removed or repointed; reject deletion
  while an unchanged SNI or a planned SNI create/update in the same gateway
  still references it. Private keys remain deferred secret writes, outside
  comparison and ordinary plan fields.
- **Parents with managed children:** [API planning][api-plan] and
  [portal child planning][portal-children] demonstrate parent/child traversal.
  Preserve child planning for new, existing, and external parents.
- **Create/delete collections:** In the [planner package][planner-package],
  `control_plane_data_plane_certificate_planner.go` demonstrates fingerprint
  matching and replacement ordering. Do not manufacture an update operation
  for an immutable resource.
- **Optional Portal singletons:** Custom domains, email configuration, and
  audit-log webhooks use
  [`reconcileOptionalPortalSingleton`][singleton-reconcile]. It selects the
  first declaration without an existing plan change, skips observation for a
  new parent, and deletes an absent declaration only in sync. Only the
  resource's matching `APIClientError` permits assuming creation with a
  warning; other read errors remain fatal. Typed adapters normalize empty
  webhook responses and retain hostname replacement and field comparison.
  Callers own scope, payloads, references, and parent dependencies.
  Portal customization remains update-only; do not apply optional-singleton
  deletion semantics to it.
- **Assignments and selectors:** [organization planning][organization-plan]
  includes role/membership operations and broader organization scope.
- **Tool-local configuration:** `control_planes[]._deck` is validated by its
  parent and planned through [deck integration][deck-plan] with
  `ActionExternalTool`, dependencies, and external-tool summary accounting.

For a new root entry point, add one entry in
[`rootPlanners()`][roots]. It supplies the type, planner, error label, and
optional scope predicate. Preserve the inventory order: it affects planning
dependencies and change IDs. Organization assignments use broader scope than
the team root. Root dispatch supplies namespace error context and HTTP log
components; add child orchestration to the owning parent planner.
Assembly checks that this inventory covers every registered managed root
exactly once. Do not add a second list of expected planner kinds.

AI Gateway's [child traversal][ai-child-plan] is shared by new, existing,
and resolved external parents. Add child dispatch there once, preserving
scope checks, empty-sync handling, and order: config stores, vaults,
data-plane certificates, TLS, providers, auth strategies, policies, agents,
consumers, consumer groups, models, then MCP servers. Provider and policy
dependency snapshots follow their respective planning steps; consumers
precede groups. TLS retains its own certificate/SNI scope and ordering.
The root planner supplies parent IDs or create-change dependencies and skips
unresolved external parents with a warning. Child lifecycles remain separate.

Portal's [child traversal][portal-child-plan] is shared by new, existing,
and external parents. Add collection filtering, scope checks, and dispatch
there once. Keep identity providers before auth settings, and teams before
group mappings and roles. New or unresolved external parents use create-only
policy: child errors are logged and later children still run. Existing parents
return the first child error; resolved external parents outside sync only
visit declared collections. Preserve scoped empty collections during sync.
Nested assets skip external parents, compare remote content only for existing
parents, and avoid duplicating already-planned asset changes.

When comparing fields, distinguish omission, explicit empty values, and
literal defaults. Normalize equivalent API representations. Compare only
observable fields; [write-only secrets](#write-only-secrets) have separate
selection rules. Preserve PATCH sparsity and any API-specific full-update
requirements rather than applying one update policy universally.

Use `PlannedChange.Parent` and `References` for routing and
`DependsOn` for operation ordering. Children created with a new parent need
its creation dependency and a resolvable parent reference. Preserve delete
dependency ordering and API cascade semantics. Let the planner's dependency
resolver produce execution order and groups.

AI Gateway child serialization follows a topological order of semantic
dependencies, not raw planning order. Provider deletion inspects all observed
models, including unscoped models: reject retained references and wait for
model deletion or updates that release them. Provider names occur in both
targets and semantic-balancer embeddings. Replacement providers must exist
before model updates. Add these edges before serialization so deletion and
runtime-version dependencies cannot be reversed by the gateway's chain.

Preserve namespace and protection behavior, including inherited protection.
Planner validation accumulates protection failures before execution.
[Inherited protection planning][plan-protection] records protecting parents;
[execution revalidation][execute-protection] checks remote protection again.
Unprotecting and changing ordinary fields must follow the established
transition rules, not a generic bypass.

## 5. Map and execute API requests

Choose the existing executor contract for supported operations:

- [`BaseExecutor`][base-executor] handles CRUD-style operations.
  Managed-label resources use `NewManagedLabelBaseExecutor` and
  `ManagedLabelOperations`, including protection-only updates.
- [`BaseCreateDeleteExecutor`][base-operations] handles create/delete APIs.
- [`BaseSingletonExecutor`][base-operations] handles update-only singletons.

Implement typed field mapping and API calls. Use `ExecutionContext` for
namespace, protection, parent, and reference information. Resolve parent IDs
through the current execution path, including parents created in the same
plan. API child adapters read the prepared references from `ExecutionContext`;
their [runtime registration][api-execs] supplies reference preparation.
Use SDK label conversion helpers so user-label removal and managed
labels survive updates.

Add resources in their family's runtime registration:

| Family | Registration |
| --- | --- |
| AI Gateway | [AI Gateway][ai-executors] |
| Event Gateway | [Event Gateway][egw-executors] |
| Control planes and certificates | [Control planes][cp-executors] |
| Organization teams and assignments | [Organization][org-executors] |
| Auth, DCR, catalog, dashboards | [Managed roots][managed-execs] |
| Portals and children | [Portals][portal-execs] |
| APIs and children | [APIs][api-execs] |

The typed base executor supplies its resource kind and payload contract.
`crudResourceExecutor` exposes create/update/delete;
`createDeleteResourceExecutor` leaves update unsupported.
Registration rejects missing contracts, empty action sets, and duplicate kinds.
Executor construction also checks exact coverage of kinds with managed sync
scope. Gateway services use decK/external lookup; audit-log destinations are
external-only. Neither has SDK execution scope. Organization selectors remain
outside the resource registry; their assignments and Portal assets do have
managed scope and executors. Coverage checks do not imply CRUD support:
supported actions and their payload contracts remain executor-local.
No per-kind executor field, payload-list entry, or action-switch case is needed.
Use an explicit `resourceExecutor` for custom actions such as the update-only
portal team-group mapping executor. Keep unsupported actions nil.

AI Gateway children prepare references before every supported operation;
consumer groups synchronize membership after successful create/update.
Event Gateway children prepare writes only; deletes use routing IDs in the
plan. Static keys omit update. Virtual-cluster create/update retain distinct
gateway lookup rules. Preserve empty-ID versus unknown-ID predicates and
parent/reference precedence during migration.

Control-plane groups synchronize membership after writes and detach members
before delete. Organization assignments omit update; team-role deletion
resolves the team but does not re-resolve the role entity.

Portal singleton dispatch retains legacy create-to-update aliases; its
`BaseSingletonExecutor` payload contract still accepts only update plans.
API publication create/update both use the create operation and
`upsertPayloadContract`; its PUT semantics apply to both actions. Keep action
aliases and payload validation coupled in the same registration.

Use `prepareResourceExecutor` for all actions, `prepareResourceWrites` for
create/update, and `prepareResourceWrite` for action-specific preparation.
Use `afterResourceWrite` for work following a successful write.
Keep ordering and failures explicit; unsupported operations must not resolve
references or perform API calls.

[`NewWithOptions`][executor] calls registration entry points in a fixed order.
Add one call there for a new family; existing families extend their own file.
Preserve created-ID tracking, execution groups, dry-run behavior,
current-state checks, and action-specific reference/post-operation work.
Registration does not define ID requirements or inherited protection. Review
`validateChangePreExecution` and [inherited protection][exec-protection] when
adding a lifecycle, especially resources without their own remote ID.

### Payload and saved-plan contracts

[`payload_contract.go`][payloads] validates the complete plan before mutations.
Create and update are separate mappings even when their SDK types resemble
one another.

- `Fields` contains request-body fields plus explicitly registered internal
  fields. `Parent`, `References`, and identity carry routing information.
- `kongctl_parent_selector` relationships never belong in request bodies.
  An `api_foreign_key` can belong in the action-specific SDK request.
- Report unsupported mappings. Every intentionally consumed or transformed
  internal field needs a central payload-contract disposition; silently
  dropping a field is a contract violation.
- Managed-label mapping runs in both preflight and execution.
  `current_labels` is internal context, not an API body field.
- Preserve [plan compatibility validation][plan-compatibility]. Saved plans
  accept the current version and payload contract; incompatible plans must
  receive regeneration guidance rather than silent migration.

## References and lookups

[`RelationshipDescriptor`][relationships] defines cross-resource YAML fields.
Describe the target type or discriminator, scalar/list cardinality, result
field, parent scope, and root-only placement. Distinguish API foreign keys
from kongctl parent selectors. Metadata drives inference and explain; keep
execution dependencies separate.

Children implementing `ResourceWithParent` supply `GetParentRef`. Where the
loader uses `ReferenceMapping`, keep `GetReferenceFieldMappings` aligned with
the descriptors. Inspect identity resolution, planner reference resolution,
and executor hydration for any remaining resource-specific dispatch.

`!ref resource#field` defaults to `#id`. [Tag parsing][tags] produces a
placeholder; the loader resolves locally available values, the
[planner resolver][resolver] materializes unresolved references, and the
executor hydrates them using remote state or earlier execution results.
Preserve requested fields, list references, and nested/scoped paths.

`!external` and `!lookup` are aliases. Their tag resolver validates syntax
and emits an opaque placeholder without making Konnect calls.
[Planner external lookup][external] resolves identity before managed matching.

External-capable resource types must implement `ExternallyResolvableResource`,
use external registration, declare selectors and parent scope, and supply
exactly one unrestricted lookup adapter. Every relationship target needs
external capability or a specific `WithExternalUnsupportedReason`.
Registration supplies materialization; avoid per-type construction switches.
Validate the `_external` block and expose it through `GetExternalBlock`.
Override the base `IsExternal` behavior where needed so matching and lifecycle
code recognize the resource as external.

[The nested-tag allowlist][tag-registry] supports `!env` directly inside
external/lookup mapping selectors, and `!env`/`!file` inside `!secret`.
External selectors from environment values retain sensitivity metadata:
cache keys use real selectors, while diagnostics redact them.
For new compositions, define resolution phase, location, result type,
both loader representations, disclosure, and saved-plan semantics. Do not
infer support from compatible YAML shapes; control fields such as `var`,
`extract`, and `path` need explicit support.

## Write-only secrets

Register accepted-but-unreturned fields and supported operations in the
[secret catalog][secret-catalog]. New manifests must use explicit `!secret`.
Sources can be deferred `!env` or `!file`, including `parts` compositions.
A file-backed value uses the wrapper:

```yaml
key: !secret {source: !file ./certs/runtime.key}
```

A bare `!file` on a reviewed write-only field is invalid: ordinary file
resolution is eager and could put contents in a saved plan.
[Secret source loading][secret-loading] checks file scope, symlinks, and size
and preserves deferred sources. Saved paths are relative to the plan;
execution binds them to the actual plan directory, not a serialized boundary.

[Planner write selection][secret-planning] and
[executor secret handling][secret-execution] keep values out of plan fields,
change details, diagnostics, dumps, and artifacts. Preserve supported sources,
per-operation selection, and redaction at executor and HTTP boundaries.
If a full-update API requires a secret, require explicit write selection and
return a value-free error when it is missing.

## 6. Export, document, and validate

Register each managed root in the [dump collector inventory][dump-collectors]
with a supported selector and collector, or an explicit omission reason.
Catalog services retain their existing omission. Registration checks both
root coverage and exact coverage of all managed kinds, including children and
selector assignments. Missing/duplicate kinds, duplicate selectors, and
conflicting dispositions fail assembly. Supported selectors, CLI help, and
dispatch derive from it; adding a declaration alone does not add dump support.

Use `rootCollector` for typed collection, optional child population, and
append-to-output behavior. Supply child coverage through `childExportKinds`
from the family's actual inventory, including explicit omissions; do not copy
its kind list. Child coverage requires a population function. Grouped
destinations must preserve allocation even for empty results. Preserve request
order, fatal Portal child errors, and other families' warning/skip policies.

APIs, AI Gateways, Event Gateways, Portals, and control planes use ordered
managed-child inventories.
Use `childCollection` for slices, `childSingleton` for pointers, and `childMap`
for named entries. Each connects a builder, typed destination, and warning
message. The [shared adapter and guard][child-collectors] derive the root
and child kinds from those types, and check all managed descendants against
registered sync ownership. Missing/duplicate kinds, incorrect owners, and
incomplete collectors fail assembly. Explicit omissions require a reason and
cannot overlap exported kinds. No separate expected-kind list is needed.

Declare nested kinds on the collector that actually exports them: AI consumers
own credentials and config stores own secrets; Event virtual clusters own
cluster/produce/consume policies and listeners own listener policies.
Inventory order controls requests. Nested Event policy failures warn and leave
the parent in output; malformed individual resources may be skipped by their
builder. Keep these partial-export boundaries in the builders.

API inventory order is versions, documents, publications, then implementations.
Version specs and document content still require detail reads; missing or
failed details warn and skip that item. Document nesting preserves observed
order and promotes children to roots when their parent is unavailable.
Recursion stays in the document builder, not in another registration.
Preserve publication ref generation and implementation reference mapping.

Portal's inventory marks page-builder errors as fatal: wrap the original error
with the Portal name and ID using `%w`, then stop before subsequent collectors or
Portals. Other collectors warn and continue. Detail-read failures and partial
team-role/asset exports remain builder-local. Use `withCoScopedChildren` for
exported kinds sharing the root sync owner: Portal team roles nest under teams,
and logo/favicon share an asset bundle. Do not change sync ownership to match
export nesting.
Team group mappings remain explicitly omitted until dump support is added.

Control-plane certificates use the shared child adapter. Gateway services
remain an explicit earlier export step because they are outside managed sync
scope. Do not change sync ownership or add services to managed coverage merely
to fit the adapter.

Organization collection remains explicit: team roles precede membership-derived
users and system accounts. `organizationCollector` accounts for its managed
assignment kinds beside that traversal. Users/system accounts are
selectors, not managed roots, and their assignments are not team-owned children.
Preserve membership filtering, request order, sorting, and partial-error
behavior in the existing builders. When adding an assignment, update its
builder and this coverage declaration; global validation detects omissions.

Builders retain conversion, sorting, and nested reads. The shared adapters
assign only non-empty collections/maps or non-nil singleton results; empty
results and errors retain existing values. Nil clients and blank parent
refs skip traversal. Preserve empty parent selectors for nested output and
the existing secret omission rules.

Keep API-to-declarative conversion in [dump][dump] and child traversal in
[child dump][dump-children]. Preserve valid reloadable YAML, parent
relationships, user labels, and secret omission. Update the
[resource reference][reference] for parent and child fields, constraints,
API-specification/example links, and supported operations. Update
[usage documentation][usage], help, and examples when behavior changes.
Review explain/scaffold output for canonical and supported nested paths.
Add imperative `get` or `view` support only when explicitly in scope;
follow repository command and view-identifier conventions.

### Dump defaults

[`--skip-defaults`][dump-defaults] lazily derives defaults from SDK tags
reachable through registered resources in `ResourceSet`. No separate runtime
catalog is needed, and the walk is bypassed without the flag. Only literal API
defaults qualify. Preserve explicit `null`, untagged fields, and client-derived
conveniences.

Missing/unsafe SDK metadata can use exactly one co-located rule per path:
`WithDumpDefaultOverride(path, value, reason)` or
`WithDumpDefaultExclusion(path, reason)`. Rules require reasons and are
mutually exclusive. Remove an override when SDK metadata becomes authoritative.

The [default inventory][dump-inventory] is a reviewed golden artifact, not
runtime input. `TestDumpDefaultInventory` detects SDK/resource/default drift.
Regenerate with `UPDATE_GOLDEN=1` only after reviewing the change and only
when test-artifact edits are authorized; the command is in the
[inventory test][dump-default-tests].

### Validation and test constraints

Use the existing suites as the baseline for refactoring. Select coverage for
the actual lifecycle; a short implementation or passing suite does not prove
that update, protection, and deletion branches were exercised. Keep
test-facing entry points as delegating wrappers when necessary.

Maintainer restrictions override test-edit and regeneration workflows.
When tests are frozen, do not add or modify test packages, test files, mocks,
fixtures, or goldens. Do not regenerate them to make a refactor pass. Record
coverage gaps and baseline failures; additional tests require permission.

For authorized new behavior, cover the affected contracts: load shape and
omission/empty scope, identity and external lookup, diff/protection, payload
mapping, dependencies, saved plans, and secret selection/redaction.
Use integration flows and [E2E scenarios][e2e] for behavior crossing phases.
Custom schema replacements need recursive parity checks with named exceptions;
SDK defaults and relationships have existing conformance checks to inspect.

Follow the repository's quality gates in order: modernization, formatting,
CGO-disabled build, lint, unit tests, and applicable integration tests.
With tests frozen, inspect `CGO_ENABLED=0 go fix -diff ./...`, apply only
authorized production changes, and restrict formatting writes to those files.
Use `make build` or `CGO_ENABLED=0 go build`; use the configured Go cache
and temp directories. Run `make lint`, `make test`, and
`make test-integration` as applicable. Identify baseline failures separately
and verify the final diff contains only authorized files.

Documentation-only edits require link/anchor and content checks; rerun code
gates when code changes warrant them.

## Keeping this guide authoritative

Update this guide in the same change that alters an integration point or
engine contract. Each refactoring migration should:

- Replace obsolete instructions and links, including later checklists.
- State which integration points became shared and which remain manual.
- Keep each invariant in one section and link to it from related steps.
- Prefer current implementations over copied CRUD templates. Keep snippets
  short, valid, and limited to syntax that needs illustration.
- Document implemented behavior here; keep proposed architecture, migration
  history, and session-specific validation results in issues or PRs.
- Check that an agent can trace a new resource or field from declaration
  through load, plan, execution, and export without guessing omitted wiring.

[agents]: ../../AGENTS.md
[usage]: ../declarative.md
[reference]: ../declarative-resource-reference.md
[resources]: ../../internal/declarative/resources
[resource-types]: ../../internal/declarative/resources/types.go
[interfaces]: ../../internal/declarative/resources/interfaces.go
[registry]: ../../internal/declarative/resources/registry.go
[namespaces]: ../../internal/declarative/resources/namespace_participants.go
[selection]: ../../internal/declarative/resources/namespace_selection.go
[api-version]: ../../internal/declarative/resources/api_version.go
[scope-capabilities]: ../../internal/declarative/resources/sync_capabilities.go
[scope-capture]: ../../internal/declarative/resources/sync_capture.go
[portal-scope]: ../../internal/declarative/resources/portal_sync_scope.go
[declaration-structure]:
  ../../internal/declarative/resources/declaration_structure.go
[relationships]: ../../internal/declarative/resources/relationships.go
[explain]: ../../internal/declarative/resources/explain.go
[load-schema]: ../../internal/declarative/resources/load_schema.go
[loader]: ../../internal/declarative/loader/loader.go
[load-validation]: ../../internal/declarative/loader/validator.go
[child-load]: ../../internal/declarative/resources/child_load.go
[collections]: ../../internal/declarative/resources/collection_validation.go
[dispatch]: ../../internal/declarative/resources/validation_dispatch.go
[coverage]: ../../internal/declarative/resources/capability_coverage.go
[validation-contract]:
  ../../internal/declarative/resources/collection_validation_contract_test.go
[selector-load]: ../../internal/declarative/resources/selector_load.go
[org-user-load]: ../../internal/declarative/resources/organization_user.go
[api-child-load]: ../../internal/declarative/resources/api_child_load.go
[api-document]: ../../internal/declarative/resources/api_document.go
[portal-child-load]: ../../internal/declarative/resources/portal_child_load.go
[portal-page]: ../../internal/declarative/resources/portal_page.go
[portal-template]: ../../internal/declarative/resources/portal_email_template.go
[portal-team]: ../../internal/declarative/resources/portal_team.go
[ai-child-load]: ../../internal/declarative/resources/ai_gateway_child_load.go
[eg-child-load]:
  ../../internal/declarative/resources/event_gateway_child_load.go
[gateway-service]: ../../internal/declarative/resources/gateway_service.go
[cp-certificate]:
  ../../internal/declarative/resources/control_plane_data_plane_certificate.go
[plan-scope]: ../../internal/declarative/planner/sync_scope.go
[planner]: ../../internal/declarative/planner/planner.go
[roots]: ../../internal/declarative/planner/root_planners.go
[constants]: ../../internal/declarative/planner/constants.go
[reconcile]: ../../internal/declarative/planner/managed_root_reconciler.go
[collection-reconcile]:
  ../../internal/declarative/planner/name_matched_collection.go
[consumer-plan]:
  ../../internal/declarative/planner/ai_gateway_consumer_planner.go
[config-store-plan]:
  ../../internal/declarative/planner/ai_gateway_config_store_planner.go
[child-reconcile]:
  ../../internal/declarative/planner/name_matched_child_reconciler.go
[mcp-plan]: ../../internal/declarative/planner/ai_gateway_mcp_server_planner.go
[ai-child-plan]: ../../internal/declarative/planner/ai_gateway_child_planner.go
[agent-plan]: ../../internal/declarative/planner/ai_gateway_agent_planner.go
[consumer-group-plan]:
  ../../internal/declarative/planner/ai_gateway_consumer_group_planner.go
[auth-plan]: ../../internal/declarative/planner/auth_strategy_planner.go
[dcr-plan]: ../../internal/declarative/planner/dcr_provider_planner.go
[dashboard-plan]: ../../internal/declarative/planner/dashboard_planner.go
[api-plan]: ../../internal/declarative/planner/api_planner.go
[singleton-reconcile]:
  ../../internal/declarative/planner/optional_singleton_reconciler.go
[portal-children]: ../../internal/declarative/planner/portal_child_planner.go
[planner-package]: ../../internal/declarative/planner
[organization-plan]:
  ../../internal/declarative/planner/organization_team_planner.go
[deck-plan]: ../../internal/declarative/planner/deck_requirements.go
[plan-protection]:
  ../../internal/declarative/planner/protection_inheritance.go
[execute-protection]:
  ../../internal/declarative/executor/protection_inheritance.go
[cache]: ../../internal/declarative/planner/resource_cache.go
[observation-cache]: ../../internal/declarative/planner/observation_cache.go
[resolver]: ../../internal/declarative/planner/resolver.go
[external]: ../../internal/declarative/planner/external_lookup.go
[state]: ../../internal/declarative/state/client.go
[cli]:
  ../../internal/cmd/root/products/konnect/declarative/declarative.go
[executor]: ../../internal/declarative/executor/executor.go
[runtime-executors]:
  ../../internal/declarative/executor/resource_executors.go
[ai-executors]: ../../internal/declarative/executor/ai_gateway_executors.go
[egw-executors]: ../../internal/declarative/executor/event_gateway_executors.go
[cp-executors]: ../../internal/declarative/executor/control_plane_executors.go
[org-executors]: ../../internal/declarative/executor/organization_executors.go
[managed-execs]: ../../internal/declarative/executor/managed_root_executors.go
[portal-execs]: ../../internal/declarative/executor/portal_executors.go
[api-execs]: ../../internal/declarative/executor/api_executors.go
[exec-protection]: ../../internal/declarative/executor/protection_inheritance.go
[base-executor]: ../../internal/declarative/executor/base_executor.go
[base-operations]: ../../internal/declarative/executor/base_operations.go
[payloads]: ../../internal/declarative/executor/payload_contract.go
[plan-compatibility]: ../../internal/declarative/planner/plan_compatibility.go
[tags]: ../../internal/declarative/tags
[tag-registry]: ../../internal/declarative/tags/resolver.go
[secret-catalog]: ../../internal/declarative/secrets/catalog.go
[secret-loading]: ../../internal/declarative/loader/secret_sources.go
[secret-planning]: ../../internal/declarative/planner/secret_writes.go
[secret-execution]: ../../internal/declarative/executor/secret_writes.go
[dump]: ../../internal/cmd/root/verbs/dump/declarative.go
[dump-collectors]:
  ../../internal/cmd/root/verbs/dump/declarative_collectors.go
[dump-children]: ../../internal/cmd/root/verbs/dump/declarative_children.go
[dump-defaults]: ../../internal/declarative/resources/dump_defaults.go
[dump-inventory]:
  ../../internal/declarative/resources/testdata/dump_defaults_inventory.yaml
[dump-default-tests]:
  ../../internal/declarative/resources/dump_defaults_test.go
[e2e]: ../../test/e2e/scenarios/README.md
[ai-child-dump]:
  ../../internal/cmd/root/verbs/dump/declarative_ai_child_collectors.go
[eg-child-dump]:
  ../../internal/cmd/root/verbs/dump/declarative_event_child_collectors.go
[child-collectors]:
  ../../internal/cmd/root/verbs/dump/declarative_child_collectors.go
[api-child-dump]:
  ../../internal/cmd/root/verbs/dump/declarative_api_child_collectors.go
[portal-child-dump]:
  ../../internal/cmd/root/verbs/dump/declarative_portal_child_collectors.go
[cp-child-dump]:
  ../../internal/cmd/root/verbs/dump/declarative_control_plane_child_collectors.go

[portal-child-plan]:
  ../../internal/declarative/planner/portal_child_traversal.go
