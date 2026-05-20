# GH-32 DCR Provider Declarative Plan

## Overview

Issue `#32` should be implemented in two layers, in this order:

1. Add `dcr_providers` as a first-class declarative parent resource.
2. Update `application_auth_strategy` so `dcr_provider_id` can reference a
   declarative DCR provider resource.

This is the correct direction for `kongctl` because DCR providers are real
Konnect resources with their own CRUD lifecycle and labels. Auth strategies do
not own provider configuration. They only point at a provider by ID.

The DCR provider `verify` endpoint is explicitly out of scope for this work.

## Why This Order

- DCR providers support labels, so they fit kongctl's managed
  namespace/protection model.
- Users should not need to pre-create DCR providers outside declarative and
  hardcode raw UUIDs into auth strategy resources.
- The local API specification shows that application auth strategies still only
  support `strategy_type: key_auth | openid_connect`. DCR is not a third auth
  strategy type. It is an optional association on OIDC auth strategies through
  `dcr_provider_id`.
- This makes DCR provider the lifecycle owner and auth strategy the consumer.

## Desired UX

```yaml
_defaults:
  kongctl:
    namespace: platform

dcr_providers:
  - ref: auth0-dcr
    name: auth0-dcr
    display_name: Auth0 DCR
    provider_type: auth0
    issuer: https://example.us.auth0.com/
    dcr_config:
      initial_client_id: abc123
      initial_client_secret: secret-value
    labels:
      team: identity

application_auth_strategies:
  - ref: oidc-app-auth
    name: oidc-app-auth
    display_name: OIDC with DCR
    strategy_type: openid_connect
    configs:
      openid-connect:
        issuer: https://example.us.auth0.com/
        credential_claim: ["client_id"]
        scopes: ["openid"]
        auth_methods: ["client_credentials"]
    dcr_provider_id: !ref auth0-dcr
```

## Scope

### In Scope

- New declarative parent resource: `dcr_providers`
- Full declarative lifecycle:
  - loader/validation
  - state client CRUD
  - planner
  - executor/adapter
  - explain/scaffold
  - docs and examples
  - unit tests
  - declarative integration/e2e coverage where practical
- Auth strategy reference support:
  - allow `application_auth_strategy.dcr_provider_id`
  - allow `!ref <dcr-provider-ref>`
  - resolve provider IDs during planning/execution

### Out of Scope

- DCR provider `verify` endpoint
- New imperative CLI surfaces unless needed separately
- Expanding auth strategy strategy types beyond `key_auth` and
  `openid_connect`

## API Observations That Matter

The local API specification in `./.tmp/konnect-application-auth-strategies.json`
shows:

- DCR providers are a separate resource family with endpoints for:
  - create
  - list
  - get
  - update
  - delete
- DCR provider create is a discriminator union by `provider_type`:
  - `auth0`
  - `azureAd`
  - `curity`
  - `okta`
  - `http`
- DCR provider update does not allow changing `provider_type`.
- DCR providers support `labels`.
- OIDC auth strategy create/update accepts `dcr_provider_id`.
- Auth strategy responses include nullable `dcr_provider`.

Implementation consequence:

- `dcr_providers` should be modeled similarly to other parent resources, but
  will likely need custom union handling comparable to auth strategies.
- Planner logic must reject `provider_type` mutation and require delete/recreate
  instead.

## Implementation Strategy

## Phase 1: Add `dcr_providers` Parent Resource

### 1. Resources Layer

Add a new resource in `internal/declarative/resources/`:

- New file:
  - `internal/declarative/resources/dcr_provider.go`
- Update:
  - `internal/declarative/resources/types.go`

Expected shape:

- `DCRProviderResource`
- embeds `BaseResource`
- embeds or wraps `kkComps.CreateDcrProviderRequest`
- implements:
  - `GetType()`
  - `GetMoniker()`
  - `GetDependencies()`
  - `GetLabels()`
  - `SetLabels()`
  - `GetReferenceFieldMappings()`
  - `Validate()`
  - `SetDefaults()`
  - `GetKonnectMonikerFilter()`
  - `TryMatchKonnectResource()`

Notes:

- This resource should be registered with `registerResourceType(...)`.
- Because create is a union by `provider_type`, it will likely need custom
  `UnmarshalJSON` and `MarshalJSON`.
- It should follow the same pattern already used by
  `ApplicationAuthStrategyResource` for discriminated unions.
- The resource should accept declarative YAML keys that match the API shape:
  - `provider_type`
  - `issuer`
  - `display_name`
  - `dcr_config`
  - `labels`

### 2. State Client

Add DCR provider support to `internal/declarative/state/client.go`:

- Add normalized type:
  - `DCRProvider`
- Add client methods:
  - `CreateDCRProvider`
  - `ListManagedDCRProviders`
  - `GetDCRProviderByID`
  - `GetDCRProviderByName`
  - `UpdateDCRProvider`
  - `DeleteDCRProvider`
- Add normalization helpers to convert SDK responses into a stable internal
  representation used by the planner.

Suggested normalized state fields:

- `ID`
- `Name`
- `DisplayName`
- `ProviderType`
- `Issuer`
- `DcrConfig map[string]any`
- `NormalizedLabels map[string]string`

Notes:

- Managed-resource filtering should be label-based like other parent resources.
- Since DCR provider responses are unions, response normalization should flatten
  the active variant into a single internal type.

### 3. Konnect Helper Interface

Add API abstraction in `internal/konnect/helpers/`:

- New helper file:
  - `internal/konnect/helpers/dcr_providers.go`

Expected interface methods:

- `ListDcrProviders`
- `GetDcrProvider`
- `CreateDcrProvider`
- `UpdateDcrProvider`
- `DeleteDcrProvider`

This should match the existing helper style used by APIs such as auth
strategies.

### 4. Planner

Add a DCR provider planner in `internal/declarative/planner/`:

- New file:
  - `internal/declarative/planner/dcr_provider_planner.go`

Planner responsibilities:

- fetch current managed DCR providers by namespace
- compare desired vs current by name
- plan create/update/delete
- respect protection rules
- reject `provider_type` mutation as unsupported
- compare only configured fields
- compare user labels correctly using the existing labels helpers

Expected planner behavior:

- create when provider name does not exist
- update mutable fields:
  - `display_name`
  - `issuer`
  - `labels`
  - `dcr_config`
- fail planning when `provider_type` changes
- sync mode should delete managed providers not present in desired state

Potential design detail:

- `dcr_config` differs by provider type. For phase one, compare the variant as a
  map-shaped payload after normalization. That keeps planner behavior simple and
  aligned with the API union shape.

### 5. Executor

Add DCR provider execution support:

- New adapter:
  - `internal/declarative/executor/dcr_provider_adapter.go`
- Wire resource execution in:
  - `internal/declarative/executor/executor.go`

Adapter responsibilities:

- map planned create fields into `CreateDcrProviderRequest`
- map planned update fields into `UpdateDcrProviderRequest`
- create/update/delete/get by name/get by ID
- preserve label behavior through existing label helpers

Important behavior:

- create must build the correct union branch from `provider_type`
- update must not attempt to set `provider_type`
- executor should follow the existing declarative adapter pattern rather than
  adding a bespoke execution path

### 6. Planner Registration / Resource Accessors

Wire the new resource through the planner and resource registry:

- `internal/declarative/resources/types.go`
- planner construction and desired resource accessors
- any planner caches, if needed later for performance

The planner should process DCR providers before application auth strategies so
that auth strategy references can resolve newly created provider IDs.

## Phase 2: Wire Auth Strategies to DCR Providers

Once `dcr_providers` exist as first-class declarative resources, update
application auth strategies to consume them.

### 1. Resource Model Changes

Update `internal/declarative/resources/auth_strategy.go`:

- parse and preserve `dcr_provider_id` for OIDC resources
- include `dcr_provider_id` in JSON marshal output
- advertise the outbound reference in `GetReferenceFieldMappings()`

Expected mapping:

- `"dcr_provider_id": "dcr_provider"`

`GetDependencies()` should also include the DCR provider dependency when the
field is present.

### 2. State Normalization

Update `internal/declarative/state/client.go` auth strategy normalization so the
planner can detect DCR drift.

Suggested addition to `ApplicationAuthStrategy` normalized state:

- `DCRProviderID string`

Populate it from response-side `dcr_provider.id` when available.

### 3. Planner Changes

Update `internal/declarative/planner/auth_strategy_planner.go`:

- include `dcr_provider_id` in create fields for OIDC strategies
- compare current vs desired `dcr_provider_id`
- emit update fields when the association changes
- continue to treat `strategy_type` as immutable

This should remain OIDC-only behavior.

### 4. Executor Changes

Update `internal/declarative/executor/auth_strategy_adapter.go`:

- set `DcrProviderID` on OIDC create requests
- set `DcrProviderID` on update requests

This is the current missing behavior that makes the documented field inert.

### 5. Reference Resolution Gaps

This part is important: reference support is not fully generic today.

The following areas currently hardcode known reference fields and resource
types:

- `internal/declarative/planner/resolver.go`
- `internal/declarative/executor/executor.go`

`dcr_provider_id` must be added there, otherwise `GetReferenceFieldMappings()`
alone will not be enough.

Required additions:

- planner reference field detection for `dcr_provider_id`
- planner field-to-resource-type mapping:
  - `dcr_provider_id -> dcr_provider`
- executor resolution before auth strategy create/update

### 6. Explain / Scaffold / Docs

Update declarative explain/scaffold outputs and docs:

- `docs/declarative-resource-reference.md`
- resource examples under `docs/examples/declarative/...`

Docs should:

- add a new `DCR Providers` section
- update `Application Auth Strategies` to prefer:
  - `dcr_provider_id: !ref <dcr-provider-ref>`
- describe supported `provider_type` variants

## Proposed Resource Name

Use resource type name:

- `dcr_provider`

Use top-level YAML collection:

- `dcr_providers`

This is consistent with the API terminology and existing resource naming in
`kongctl`.

## Test Plan

### Unit Tests

- resource unmarshaling/marshaling for each provider type
- resource validation
- planner create/update/delete decisions
- planner protection handling
- planner rejection of `provider_type` changes
- executor create/update request mapping for each provider type
- state normalization from SDK response unions
- auth strategy planner/executor handling of `dcr_provider_id`
- reference validation and resolution for `!ref` on `dcr_provider_id`

### Integration / E2E

Minimum useful scenarios:

1. Create a managed DCR provider declaratively.
2. Update mutable DCR provider fields declaratively.
3. Sync-delete a managed DCR provider.
4. Create an OIDC auth strategy that references a declarative DCR provider.
5. Update the auth strategy to point to a different DCR provider.

If the harness setup makes end-to-end DCR provider creation expensive, unit
coverage plus one focused integration flow is acceptable as an initial landing.

## Acceptance Criteria

The work is complete when all of the following are true:

- `dcr_providers` is a supported declarative parent resource.
- Managed DCR providers can be create/update/delete planned and executed.
- DCR providers participate in namespace/protection label management.
- `kongctl explain` and `kongctl scaffold` include DCR providers.
- Declarative docs include DCR providers and the updated auth strategy
  reference form.
- `application_auth_strategy.dcr_provider_id` accepts `!ref` to a DCR provider.
- Planner and executor resolve `dcr_provider_id` correctly.
- Auth strategy drift detection includes the DCR provider association.
- `provider_type` mutation is rejected with a clear delete/recreate message.

## Suggested Delivery Sequence

1. Implement DCR provider helper + state client normalization.
2. Implement DCR provider resource model and validation.
3. Implement planner and executor for `dcr_providers`.
4. Add docs/explain/scaffold for the new resource.
5. Wire auth strategy `dcr_provider_id` parsing and state normalization.
6. Add reference-resolution support for `dcr_provider_id`.
7. Add unit tests, then targeted integration/e2e coverage.

## Risks

### Union Complexity

DCR providers are a `provider_type`-discriminated union on create and a
variant-shaped `dcr_config` on update/response. This is the main implementation
risk.

Mitigation:

- keep a normalized internal state shape for planner comparisons
- centralize variant conversion in resource/adaptor/state helper functions

### Reference Plumbing Is Partly Hardcoded

Reference handling is not fully metadata-driven today.

Mitigation:

- treat `dcr_provider_id` reference support as an explicit part of the task
- update both planner and executor hardcoded field maps

### Null / Unset Semantics

If later we need to support explicitly clearing `dcr_provider_id`, presence
tracking may matter.

Mitigation:

- for the first pass, support omitted or resolved string values cleanly
- evaluate explicit null clearing separately if the API and tests require it

## Recommendation

Proceed with DCR providers as a new declarative parent resource first. After
that, wire `application_auth_strategy.dcr_provider_id` as a standard
cross-resource reference to `dcr_provider`.
