# Plan: Deck Integration for Control Planes (GH-327)

## Overview
Integrate deck execution into the declarative workflow by attaching a **single `_deck` configuration** to a
control plane. This replaces the previous `_external.requires.deck` gateway_service mechanism. The deck run is
an implicit dependency of the control plane and executes **once per control plane** when `_deck` is present.

The goal is a simple, explicit UX:
- Control plane owns the deck configuration.
- Deck runs once per control plane (apply/sync).
- Gateway services remain external and are resolved by selector after the deck run.

## Target UX

### Example
```yaml
control_planes:
  - ref: team-a-cp
    name: "team-a-cp"
    description: "Team A Control Plane"
    cluster_type: "CLUSTER_TYPE_SERVERLESS"
    _deck:
      files:
        - "kong.yaml"
      flags:
        - "--analytics=false"

    gateway_services:
      - ref: team-a-cool-gw-svc
        _external:
          selector:
            matchFields:
              name: "cool-service"
```

Behavior:
1. `kongctl plan/diff` runs `deck gateway diff` **once per control plane** with `_deck` and only includes a
   `_deck` change when diff reports changes. If the control plane is being created in the same plan or the
   control plane ID is unknown, diff is skipped and the `_deck` change is included.
2. `kongctl apply/sync`:
   - Ensures the control plane exists (normal planner/executor dependencies).
   - Executes `deck gateway apply|sync` once per `_deck` config.
   - Resolves external gateway services for that control plane via selector (`matchFields.name`).
   - Updates any dependent `api_implementation` changes with the resolved service ID and control plane ID.

## Key Decisions
- `_deck` is a **pseudo-resource** scoped to a control plane (one config per control plane).
- `_deck` is allowed on managed **or external** control planes.
- `_external.requires.deck` is removed (breaking change; this branch is not released).
- `plan`/`diff` uses `deck gateway diff --json-output --no-color` to decide if a `_deck` change is needed.
  For apply mode, delete-only diffs are ignored.
- `apply` runs `deck gateway apply` and `sync` runs `deck gateway sync`.
- Deck files are resolved relative to the declarative config file and constrained by `--base-dir`.
- Plan files store deck base directories relative to the plan file location (or CWD when emitting to stdout).
- kongctl injects Konnect auth/context and output flags; user-supplied `--konnect-*` and output flags are rejected.
- Selector for external gateway services remains `_external.selector.matchFields.name` (name-only for now).

## Implementation Summary

### 1) Resource Model and Validation
- **ControlPlaneResource** gains a `_deck` field (single config).
  - `files` required; `flags` optional.
  - Validate: no auth/output flags, no `{{kongctl.mode}}` placeholder.
  - Only one `_deck` entry per control plane.
- **GatewayServiceResource** remains external with selector; no `requires.deck`.

### 2) Loader + Base Dir Handling
- Record deck base directory per control plane (similar to prior gateway service deck handling).
- Resolve deck file paths relative to the file that declares the control plane.

### 3) Planner Changes
- Add `_deck` planned changes (`ResourceType: _deck`, `Action: EXTERNAL_TOOL`) **per control plane**.
- Fields include: `control_plane_ref`, `control_plane_id` (when known), `control_plane_name` (resolved when possible),
  `deck_base_dir`, `files`, and `flags`.
- `plan`/`diff`: run `deck gateway diff` once per `_deck` config if control plane is available; otherwise include the change.
- Dependencies:
  - `_deck` depends on control plane CREATE if present in the plan.
  - `api_implementation` changes that reference gateway services in that control plane depend on the `_deck` change.

### 4) Executor Changes
- Execute `_deck` changes by running `deck gateway apply|sync` once per control plane.
- Resolve control plane name from plan changes (preferred) or Konnect lookup by ID when needed.
- After deck run, resolve external gateway services **for that control plane** and update dependent changes
  (service ID + control plane ID). If a referenced service cannot be resolved, fail the dependent change.

### 5) Documentation
- Update `docs/declarative.md` to document the `_deck` control plane field and remove `_external.requires.deck`.
- Document that `_deck` runs once per control plane and requires selector names for external gateway services.

### 6) Tests
- Update e2e scenarios to use control plane `_deck` instead of `_external.requires.deck`.
- Add scenario for multiple deck files via a single `_deck` config.
- Add negative tests for missing deck files and invalid deck content.
- Unit tests for:
  - `_deck` validation
  - planner `_deck` change generation and summary output
  - executor deck run + post-run gateway service resolution

## Open Considerations (Out of Scope)
- Multiple `_deck` configs per control plane (explicitly not supported).
- Support for additional external tool integrations.

