# Plan: Deck Requires for External Gateway Services (GH-327)

## Overview
Integrate deck execution into the declarative workflow for **external gateway services** only. When a gateway service is marked external and includes `_external.requires.deck`, kongctl should run the deck steps during `apply`/`sync`, then resolve the gateway service by selector (`matchFields.name`). Planning and diff must remain side-effect free, so deck runs only during execution.

Key decisions:
- `_external.requires.deck` is allowed **only for `gateway_services`**.
- `requires.deck` is an **array of step objects** with `args` (string slice).
- `requires.deck` supports a **mode placeholder** in args that is replaced by the invoking `kongctl` verb (`sync` or `apply`).
- Only `deck gateway sync|apply` (or `deck gateway {{kongctl.mode}}`) are allowed for now; other gateway verbs are rejected.
- `requires.deck` **requires** `_external.selector.matchFields.name` (name-only for now).
- Deck commands **run once per resource** (no deduping).
- Deck commands **do not run** for `plan`, `diff`, or `apply --dry-run`/`sync --dry-run`.
- Deck commands run in the **process working dir / base-dir**; file paths remain relative to that directory.
- When a deck step includes `gateway`, kongctl injects Konnect auth/context (token, control plane name, base URL) and **errors** if the user already supplied `--konnect-token`, `--konnect-control-plane-name`, or `--konnect-addr`.

## Target UX

### Example
```yaml
gateway_services:
  - ref: foo
    control_plane: my-cp
    _external:
      requires:
        deck:
          - args: ["file", "openapi2kong", "input.yaml", "-o", "kong.yaml"]
          - args: ["gateway", "{{kongctl.mode}}", "-s", "kong.yaml"]
      selector:
        matchFields:
          name: "abc-service"
```

Behavior:
1. `kongctl plan/diff` records the deck requirements in the plan, but does not execute them.
2. `kongctl apply/sync`:
   - Ensures the control plane exists (via normal planner/executor dependencies).
   - Executes the deck steps for this gateway service.
   - Resolves the service via selector (`name`), updates plan references for `api_implementation` service fields, then continues execution.

## Implementation Plan

### 1) Resource Model and Validation
- **File**: `internal/declarative/resources/external.go`
  - Add `Requires` to `ExternalBlock` with a nested `DeckRequires` type.
  - Define `DeckRequires` with `Steps []DeckStep` where `DeckStep` has `Args []string`.
  - Update `ExternalBlock.Validate()`:
    - Allow `Selector` + `Requires` together for gateway services.
    - If `Requires.Deck` is present, require `Selector` and require `selector.matchFields.name` only.
    - Reject `ID` if `Requires.Deck` is present.
    - Validate `{{kongctl.mode}}` placeholder usage:
      - Allowed only in `deck gateway` steps.
      - Must appear exactly once in a step when present.
      - Error if `{{kongctl.mode}}` is used with any other deck subcommand.
    - Validate gateway verb:
      - Allow only `sync` or `apply` (or `{{kongctl.mode}}`).
      - Reject `diff|dump|ping|reset|validate` in this initial implementation.
    - For non-gateway-services, error if `Requires` is set.

- **File**: `internal/declarative/resources/gateway_service.go`
  - Add `HasDeckRequires()` or `IsDeckRequired()` helper.

- **Validation location**: enforce the “gateway_services only” rule in resource validation (preferably in `GatewayServiceResource.Validate()` once `Requires` is part of the external block).

### 2) Plan Types for External Deck Steps
- **File**: `internal/declarative/planner/types.go`
  - Add `DeckDependency` or `ExternalDeckStep` struct for plan persistence.
  - Add `DeckDependencies []DeckDependency` to `Plan`.
    - Fields should include: `GatewayServiceRef`, `ControlPlaneRef`, `ControlPlaneID`, `ControlPlaneName`, `SelectorName`, `Steps`.

### 3) Planner Integration
- **File**: `internal/declarative/planner/planner.go`
  - Update `resolveGatewayServiceIdentities()` to detect `requires.deck`:
    - Skip Konnect lookup for the service and record a deck dependency in the plan.
    - Allow gateway services to carry unresolved IDs during planning.
  - Update `resolveAPIImplementationServiceReferences()` to tolerate unresolved gateway service IDs when deck is required (record as pending).

### 4) Executor Integration (Deck Steps)
- **New package**: `internal/declarative/deck/`
  - `runner.go`: deck runner that accepts steps, injects Konnect flags for `gateway` commands, and executes using `exec.CommandContext`.
  - `errors.go`: deck execution errors (missing deck, invalid args, injected flag conflicts).

- **File**: `internal/declarative/executor/executor.go`
  - Add a new internal “pseudo-change” type (e.g., `resource_type = "deck_step"`) **or** add an execution hook to run deck steps before processing dependent changes.
  - Recommended: add a **planned change** for each `requires.deck` to honor dependency ordering. These changes should:
    - Depend on control plane creation/update (if CP is in plan).
    - Be depended on by api_implementation creates that reference the gateway service.
  - During execution of this change:
    - Run deck steps.
    - Replace `{{kongctl.mode}}` in args with `sync` or `apply` based on the invoking command.
    - Resolve the gateway service by selector (name) using existing `ListGatewayServices` by CP ID.
    - Update plan changes referencing the gateway service (service.id + service.control_plane_id).

### 5) Konnect Context Injection for Deck
- **Source**: `internal/cmd/root/products/konnect/common/common.go` already resolves token and base URL.
  - Reuse `GetAccessToken()` and `ResolveBaseURL()` to inject `--konnect-token` and `--konnect-addr`.
  - For control plane name, use the resolved control plane name from resource set (or fetch from Konnect if only ID is known).
  - For `{{kongctl.mode}}`, map `kongctl sync` → `sync`, `kongctl apply` → `apply`. Error on `plan/diff` (should never execute anyway) and on any other verb.

### 6) Dry-Run Behavior
- Skip all `requires.deck` execution when dry-run is enabled.
- The deck pseudo-change should be marked skipped with a clear reason.

### 7) Documentation
- **File**: `docs/declarative.md`
  - Add a section to `_external` describing `requires.deck` for gateway services.
  - Add example YAML and explain the execution order and the selector requirement.
  - Clarify that deck steps are not run in plan/diff or dry-run.

### 8) Tests
- Unit tests for:
  - `_external.requires.deck` validation (requires selector.name, rejects id, gateway services only).
  - Planner capturing deck dependencies in the plan.
  - Executor deck step execution order and reference updates (mock deck runner).

## Open Considerations (Out of Scope for MVP)
- Deduplicating deck steps across resources.
- Supporting other external “requires” command types.
- `deck diff` integration for plan/diff.

## Approval Gate
No code changes until this plan is approved.
