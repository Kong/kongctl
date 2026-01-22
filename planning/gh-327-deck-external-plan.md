# GH-327 Deck Integration Plan (kongctl declarative)

Goal
- Add `_external.requires.deck` for gateway_services only. Plan/diff are side-effect free but use `deck gateway diff`
  to decide whether an external tool change is needed. apply/sync runs `deck gateway apply|sync`, resolves the
  gateway service by selector name, then continues execution (including api_implementation create).

Requirements (from issue + final implementation)
- `_external.requires.deck` allowed only for `gateway_services`.
- `requires.deck` is a single object with `files` and optional `flags` (no array of commands).
- `{{kongctl.mode}}` is not supported; kongctl selects `apply` or `sync` based on the command.
- `plan`/`diff` runs `deck gateway diff --json-output --no-color` and only includes a `_deck` change when diff reports
  changes. For apply mode, delete-only diffs are ignored.
- `apply` runs `deck gateway apply`, `sync` runs `deck gateway sync`.
- `requires.deck` requires `_external.selector.matchFields.name` (name-only for now).
- If `requires.deck` is present, `_external.id` is invalid.
- Deck runs once per resource; no dedupe.
- Deck runs do not execute for plan/diff or for apply/sync dry-run.
- Deck files are resolved relative to the declarative config file and must remain within the `--base-dir` boundary.
- Plan files store deck base directories relative to the plan file directory (or CWD when outputting to stdout).
- kongctl injects Konnect auth/context and output flags; user-supplied `--konnect-*` and output flags are rejected.

Execution behavior
- plan/diff:
  1) If control plane ID is known and not being created, run `deck gateway diff` to decide whether to include a
     `_deck` change.
  2) If the control plane is being created (or ID is unknown), skip diff and include the `_deck` change.
- apply/sync:
  1) Ensure control plane exists (normal deps).
  2) Run `deck gateway apply|sync` once per gateway_service with requires.deck.
  3) Resolve gateway service by selector name within control plane.
  4) Update plan changes referencing that gateway service (service.id, service.control_plane_id), then continue
     execution.
- dry-run: skip deck runs with a clear skipped reason.

Plan (first change is e2e scenario)
1) E2E scenario + testdata
   - Add scenario under `test/e2e/scenarios/external/deck-requires/`.
   - Scenario uses the real deck binary and asserts create/delete behavior by querying gateway services.

2) Resource model/validation
   - `internal/declarative/resources/external.go`
     - `DeckRequires` uses `Files` and optional `Flags`.
     - Validation rules per requirements above (no placeholder).
   - `internal/declarative/resources/gateway_service.go`
     - Add helper `HasDeckRequires()` (or similar).
     - Validate “gateway_services only” in resource validation.

3) Planner updates
   - `internal/declarative/planner/types.go`
     - Summary includes `by_external_tools` with `_deck` entries.
   - `internal/declarative/planner/deck_requirements.go`
     - Add `_deck` changes with `files`, `flags`, selector, and control plane info.
     - Run deck diff at plan time to decide whether to include the change.
   - Add dependency wiring so `_deck` runs before api_implementation creates that reference the gateway service.

4) Executor + deck runner
   - New package `internal/declarative/deck/` with:
   - `runner.go`: validates args, injects Konnect flags, runs `exec.CommandContext`, captures stdout/stderr.
     - `errors.go`: typed errors for missing deck, invalid args, conflicting flags.
   - `internal/declarative/executor/executor.go`
     - Execute `_deck` changes with `deck gateway apply|sync`.
     - Resolve gateway service via selector.name using state client ListGatewayServices.
     - Update plan changes referencing that gateway service (service.id/control_plane_id), including any api_implementation create fields.
     - Skip and mark as skipped on dry-run.

5) Docs + unit tests
   - `docs/declarative.md`: document `_external.requires.deck` and behavior (no plan/diff execution; requires selector name).
   - Unit tests:
     - Validation for requires.deck rules.
     - Planner captures `_deck` changes and summary entries.
     - Executor runs deck, updates references, skips on dry-run.

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
- When injecting Konnect flags into deck args, error if user already passed `--konnect-token`,
  `--konnect-control-plane-name`, or `--konnect-addr` in the config.
