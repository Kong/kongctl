# GH-327 Deck Integration Plan (kongctl declarative)

Goal
- Add `_external.requires.deck` for gateway_services only. Plan/diff are side-effect free. apply/sync runs deck steps, resolves gateway service by selector name, then continues execution (including api_implementation create).

Requirements (from issue + planning docs)
- `_external.requires.deck` allowed only for `gateway_services`.
- `requires.deck` is an array of steps; each step has `args: []string`.
- `{{kongctl.mode}}` placeholder allowed only in `deck gateway` steps; must appear exactly once when used.
- Only `deck gateway sync|apply` (or `deck gateway {{kongctl.mode}}`) supported; reject other gateway verbs.
- `requires.deck` requires `_external.selector.matchFields.name` (name-only for now).
- If `requires.deck` present, `_external.id` is invalid.
- Deck steps run once per resource; no dedupe.
- Deck steps do not run for plan/diff or for apply/sync dry-run.
- Deck steps run in process working dir/base-dir (same as plan time for apply-from-plan).
- If deck step includes `gateway`, kongctl injects Konnect context (token, control plane name, base URL) and errors if user supplied those flags already.

Execution behavior
- apply/sync:
  1) Ensure control plane exists (normal deps).
  2) Run deck steps for each gateway_service with requires.deck.
  3) Resolve gateway service by selector name within control plane.
  4) Update plan changes referencing that gateway service (service.id, service.control_plane_id), then continue execution.
- dry-run: skip deck steps with a clear “skipped” reason.

Plan (first change is e2e scenario)
1) E2E scenario + testdata
   - Add scenario under `test/e2e/scenarios/external/deck-requires/`.
   - Use a stub `deck` script under `test/e2e/tools/` so tests do not require a real deck install.
   - Scenario sets `PATH=./tools:/usr/bin:/bin` and runs a declarative apply/sync using configs with `_external.requires.deck` and selector name.
   - Assertions check:
     - plan metadata mode
     - api_implementation create in plan
     - stub deck was invoked with injected `--konnect-*` flags
   - Use harness templates to write stub output files into the step inputs dir.

2) Resource model/validation
   - `internal/declarative/resources/external.go`
     - Add `Requires` to ExternalBlock with nested `DeckRequires` + `DeckStep`.
     - Validation rules per requirements above.
   - `internal/declarative/resources/gateway_service.go`
     - Add helper `HasDeckRequires()` (or similar).
     - Validate “gateway_services only” in resource validation.

3) Planner updates
   - `internal/declarative/planner/types.go`
     - Add plan persistence type for deck requirements (e.g., `DeckDependency`).
     - Add `DeckDependencies []DeckDependency` to Plan.
   - `internal/declarative/planner/planner.go`
     - In `resolveGatewayServiceIdentities`, if `requires.deck` present: skip Konnect lookup, store dependency data in plan, allow unresolved service ID.
     - Update `resolveAPIImplementationServiceReferences` to tolerate unresolved gateway service IDs when deck is required (leave pending for execution update).
     - Ensure control plane name is available for deck injection; if external CP, fill name from matched Konnect resource if needed.
   - Add dependency wiring so deck step runs before api_implementation creates that reference the gateway service.

4) Executor + deck runner
   - New package `internal/declarative/deck/` with:
     - `runner.go`: validates args, injects konnect flags, runs `exec.CommandContext`.
     - `errors.go`: typed errors for missing deck, invalid args, conflicting flags.
   - `internal/declarative/executor/executor.go`
     - Add pseudo change type (e.g., `_deck`) or a pre-execution hook.
     - Execute deck steps, replace `{{kongctl.mode}}` with `apply`/`sync`.
     - Resolve gateway service via selector.name using state client ListGatewayServices.
     - Update plan changes referencing that gateway service (service.id/control_plane_id), including any api_implementation create fields.
     - Skip and mark as skipped on dry-run.

5) Docs + unit tests
   - `docs/declarative.md`: document `_external.requires.deck` and behavior (no plan/diff execution; requires selector name).
   - Unit tests:
     - Validation for requires.deck rules and placeholder constraints.
     - Planner captures deck dependencies.
     - Executor runs deck steps, updates references, skips on dry-run.

Files likely to touch
- `internal/declarative/resources/external.go`
- `internal/declarative/resources/gateway_service.go`
- `internal/declarative/planner/types.go`
- `internal/declarative/planner/planner.go`
- `internal/declarative/executor/executor.go`
- `internal/declarative/deck/*`
- `docs/declarative.md`
- `test/e2e/scenarios/external/deck-requires/scenario.yaml`
- `test/e2e/testdata/declarative/external/deck-requires/*`
- `test/e2e/tools/deck` (stub script)
- new unit tests under `internal/declarative/*_test.go`

Notes
- E2E scenario must be added first before other code changes.
- When injecting konnect flags into deck args, error if user already passed `--konnect-token`, `--konnect-control-plane-name`, or `--konnect-addr` in the step.
