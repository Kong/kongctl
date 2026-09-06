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
explain/scaffold, load-schema discovery, and dump-default metadata. The
[root planner inventory][roots] drives root construction and dispatch.
[Runtime executor registration][runtime-executors] supplies action routing and
payload validation for AI Gateway and Event Gateway resources. Other executor
families, loader scope/extraction, namespace participation, relationships,
state-client wiring, and dump collection still have separate integration
points. Registering a declaration does not complete those steps automatically.

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
- For namespace-bearing declarations, inspect
  [namespace participants][namespaces] and namespace accessors in
  `ResourceSet`. Ordinary managed children inherit namespace and protection
  from their parent; do not give them independent `kongctl` configuration.
- Follow the [maturity policy](maturity.md). Resources default to GA.
  Co-locate `WithMaturity` and narrower `WithOperationMaturity` overrides
  with registration. Maturity is discovery metadata, not runtime gating or
  plan, result, or telemetry data.

### Defaults and SDK shape

Required fields for a managed operation must be explicit in manifests.
`SetDefaults()` may apply documented literal API defaults only. It must not
derive required values from `ref`, `name`, or another supplied field. Existing
name-from-ref fallbacks are compatibility behavior whose removal needs
separate scope.

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

Update [loader scope tables][load-scope] for root collections, root-level
child selectors, and nested child keys. For delete-capable singletons,
preserve scope while dropping the empty desired value during decoding.
Do not apply that deletion meaning to update-only singletons.

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
Inspect [planning caches][cache] before adding reads: namespace fanout can
share observations across a run. Preserve missing-client and error behavior
during refactoring; a failed read must not become an empty successful result.

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
- **Parents with managed children:** [API planning][api-plan] and
  [portal child planning][portal-children] demonstrate parent/child traversal.
  Preserve child planning for new, existing, and external parents.
- **Create/delete collections:** In the [planner package][planner-package],
  `control_plane_data_plane_certificate_planner.go` demonstrates fingerprint
  matching and replacement ordering. Do not manufacture an update operation
  for an immutable resource.
- **Singletons:** Portal customization uses an update-only API. Portal custom
  domains are optional and delete-capable. Both appear in
  [portal child planning][portal-children]; their empty-input behavior differs.
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
plan. Use SDK label conversion helpers so user-label removal and managed
labels survive updates.

Add AI Gateway resources in [AI Gateway registration][ai-executors] and Event
Gateway resources in [Event Gateway registration][egw-executors]. The typed
base executor supplies its resource kind and payload contract.
`crudResourceExecutor` exposes create/update/delete;
`createDeleteResourceExecutor` leaves update unsupported.
Registration rejects missing contracts, empty action sets, and duplicate kinds,
including conflicts with the legacy payload inventory. No per-kind field,
payload-list entry, or action-switch case is needed for these resources.

AI Gateway children prepare references before every supported operation;
consumer groups synchronize membership after successful create/update.
Event Gateway children prepare writes only; deletes use routing IDs in the
plan. Static keys omit update. Virtual-cluster create/update retain distinct
gateway lookup rules. Preserve empty-ID versus unknown-ID predicates and
parent/reference precedence during migration.

Use `prepareResourceExecutor` for all actions, `prepareResourceWrites` for
create/update, and `prepareResourceWrite` for action-specific preparation.
Keep ordering and failures explicit; unsupported operations must not resolve
references or perform API calls.

Other families retain [legacy executor wiring][executor]: inspect
`NewWithOptions`, the three resource action switches, and payload registration.
When migrating a family, remove its entries from every superseded inventory.
Preserve created-ID tracking, execution groups, dry-run behavior,
current-state checks, and action-specific reference/post-operation work.

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

Wire supported root/child collection and API-to-declarative conversion in
[dump][dump] and [child dump][dump-children]. Preserve valid reloadable YAML,
parent relationships, user labels, and secret omission. Update the
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
[relationships]: ../../internal/declarative/resources/relationships.go
[explain]: ../../internal/declarative/resources/explain.go
[load-schema]: ../../internal/declarative/resources/load_schema.go
[loader]: ../../internal/declarative/loader/loader.go
[load-validation]: ../../internal/declarative/loader/validator.go
[load-scope]: ../../internal/declarative/loader/sync_scope.go
[planner]: ../../internal/declarative/planner/planner.go
[roots]: ../../internal/declarative/planner/root_planners.go
[constants]: ../../internal/declarative/planner/constants.go
[reconcile]: ../../internal/declarative/planner/managed_root_reconciler.go
[auth-plan]: ../../internal/declarative/planner/auth_strategy_planner.go
[dcr-plan]: ../../internal/declarative/planner/dcr_provider_planner.go
[dashboard-plan]: ../../internal/declarative/planner/dashboard_planner.go
[api-plan]: ../../internal/declarative/planner/api_planner.go
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
[dump-children]: ../../internal/cmd/root/verbs/dump/declarative_children.go
[dump-defaults]: ../../internal/declarative/resources/dump_defaults.go
[dump-inventory]:
  ../../internal/declarative/resources/testdata/dump_defaults_inventory.yaml
[dump-default-tests]:
  ../../internal/declarative/resources/dump_defaults_test.go
[e2e]: ../../test/e2e/scenarios/README.md
