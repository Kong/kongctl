# Plan: Deck Requires for External Gateway Services (GH-327)

## Overview
Integrate deck execution into the declarative workflow for **external gateway services** only. When a gateway
service is marked external and includes `_external.requires.deck`, kongctl runs a deck gateway apply/sync during
execution, then resolves the gateway service by selector (`matchFields.name`). Planning is side-effect free, but
`plan`/`diff` uses `deck gateway diff` to decide whether an external tool change is needed.

Key decisions:
- `_external.requires.deck` is allowed **only for `gateway_services`**.
- `requires.deck` is a single object with `files` and optional `flags` (no array of commands).
- `{{kongctl.mode}}` is not supported; kongctl selects `apply` or `sync` based on the command.
- `plan`/`diff` runs `deck gateway diff --json-output --no-color` to decide whether to include an external tool change.
  For apply mode, delete-only diffs are ignored.
- `apply` runs `deck gateway apply` and `sync` runs `deck gateway sync`.
- `requires.deck` **requires** `_external.selector.matchFields.name` (name-only for now).
- If `requires.deck` is present, `_external.id` is invalid.
- Deck runs **once per resource** (no deduping).
- Deck runs **do not execute** for `plan`, `diff`, or `apply --dry-run`/`sync --dry-run`.
- Deck files are resolved relative to the declarative config file and constrained by `--base-dir`.
- Plan files store deck base directories relative to the plan file directory (or CWD when outputting to stdout).
- kongctl injects Konnect auth/context and output flags; user-supplied `--konnect-*` and output flags are rejected.

## Target UX

### Example
```yaml
gateway_services:
  - ref: foo
    control_plane: my-cp
    _external:
      requires:
        deck:
          files:
            - "kong.yaml"
          flags:
            - "--select-tag=kongctl"
      selector:
        matchFields:
          name: "abc-service"
```

Behavior:
1. `kongctl plan/diff` runs `deck gateway diff` and only includes an external tool change when diff reports changes.
   If the control plane is being created in the same plan (or no control plane ID is known), diff is skipped and an
   external tool change is included.
2. `kongctl apply/sync`:
   - Ensures the control plane exists (via normal planner/executor dependencies).
   - Executes `deck gateway apply|sync` once for this gateway service.
   - Resolves the service via selector (`name`), updates plan references for `api_implementation` service fields,
     then continues execution.

## Implementation Summary

### 1) Resource Model and Validation
- **Resource model** (`internal/declarative/resources/external.go`)
  - `ExternalRequires.Deck` stores `Files` and optional `Flags`.
  - Validation requires selector.matchFields.name (name-only), rejects `_external.id` and output/auth flags.

- **File**: `internal/declarative/resources/gateway_service.go`
  - Add `HasDeckRequires()` or `IsDeckRequired()` helper.

- **Validation location**: enforce the “gateway_services only” rule in resource validation (preferably in `GatewayServiceResource.Validate()` once `Requires` is part of the external block).

### 2) Plan Types for External Deck Steps
- **Planner** (`internal/declarative/planner/deck_requirements.go`)
  - Adds `_deck` planned changes with `ActionExternalTool`, selector info, control plane info, deck base dir,
    and `files`/`flags` fields.
  - Summary includes `by_external_tools` entries for `_deck`.
  - Runs `deck gateway diff` during plan/diff to decide whether to include the change; skips diff when the control
    plane is being created in the same plan.

### 3) Planner Integration
- **Planner identity resolution**
  - Best-effort resolves gateway service IDs for deck-managed services when possible.
  - API implementation references are updated after deck execution if needed.

### 4) Executor Integration (Deck Steps)
- **Executor**
  - Executes `_deck` changes and always runs `deck gateway apply|sync` with `--json-output` and `--no-color`.
  - Captures deck stdout/stderr and logs stdout at debug, stderr at error on failures.
  - Resolves gateway service IDs after deck runs and updates dependent changes.

- **Deck runner**
  - Executes `deck` via `exec.CommandContext`, captures stdout/stderr, and injects Konnect flags for gateway commands.

### 5) Konnect Context Injection for Deck
- Uses Konnect token and base URL from the CLI config.
- Resolves control plane name via config or Konnect when only the ID is known.

### 6) Dry-Run Behavior
- Skip all `requires.deck` execution when dry-run is enabled.

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
  - Placeholder validation (only `{{kongctl.mode}}`, only in `deck gateway` steps, sync/apply only).
  - Dry-run skipping behavior for deck steps.

## Open Considerations (Out of Scope for MVP)
- Deduplicating deck runs across resources.
- Supporting other external “requires” command types.

## Approval Gate
Implemented.
