# GH-327 Deck Integration Plan

## Overview
Integrate decK execution into kongctl declarative workflows by attaching a **single `_deck` configuration**
per control plane. This replaces the prior `_external.requires.deck` gateway_service mechanism and
simplifies UX: **deck runs once per control plane**, and external gateway services are resolved
by selector after the deck run.

Key goals:
- Control plane owns the deck configuration.
- Deck runs once per control plane (apply/sync).
- Gateway services remain external and are resolved by selector name.
- Plans remain readable and explicit about post-external-tool resolution.

## Target UX

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

## Behavior Summary

### Plan/Diff
- Runs `deck gateway diff --json-output --no-color` **once per control plane** with `_deck`.
- If the control plane is being created in the same plan (or no ID is available), diff is skipped and
  the `_deck` change is included.
- For apply mode, delete-only diffs are ignored.

### Apply/Sync
- Ensures the control plane exists.
- Executes `deck gateway apply|sync` **once per `_deck` config**.
- Resolves external gateway services via `_external.selector.matchFields.name` after the deck run.
- Updates dependent `api_implementation` changes with resolved service ID and control plane ID.

## Validation Rules
- `_deck` is allowed only on control planes; only one `_deck` per control plane.
- `_deck.files` must include at least one state file.
- `_deck.flags` optional; cannot include Konnect auth flags or output flags; no `{{kongctl.mode}}`.
- `_external.selector.matchFields.name` is required for external gateway services and must be the
  only selector field for deck resolution.
- `_external.requires.deck` is not supported.

## Plan Representation
- The `_deck` planned change uses:
  - `resource_type: _deck`
  - `action: EXTERNAL_TOOL`
  - `fields`: control plane metadata, deck files/flags, deck_base_dir
  - `post_resolution_targets`: gateway services to resolve **after** the deck step

`post_resolution_targets` makes the dependency explicit without overloading `fields`:

```json
{
  "resource_type": "_deck",
  "action": "EXTERNAL_TOOL",
  "fields": {
    "control_plane_ref": "team-a-cp",
    "control_plane_id": "...",
    "control_plane_name": "team-a-cp",
    "deck_base_dir": "configs/prod",
    "files": ["kong.yaml"],
    "flags": ["--analytics=false"]
  },
  "post_resolution_targets": [
    {
      "resource_type": "gateway_service",
      "resource_ref": "team-a-cool-gw-svc",
      "control_plane_ref": "team-a-cp",
      "control_plane_id": "...",
      "control_plane_name": "team-a-cp",
      "selector": {"matchFields": {"name": "cool-service"}}
    }
  ]
}
```

Summary output (`summary.by_external_tools._deck`) includes the same gateway service targets.

## Implementation Plan

### 1) Resource Model & Validation
- Add `_deck` to `ControlPlaneResource` with validation rules above.
- Keep `GatewayServiceResource` external with selector. Remove `_external.requires.deck` support.

### 2) Loader & Base Dir Handling
- Record a deck base directory per control plane (resolved relative to the file that declares `_deck`).
- Enforce `--base-dir` boundary for `_deck` files.

### 3) Planner
- Generate one `_deck` change per control plane that declares `_deck`.
- Run `deck gateway diff` to decide whether to include the change.
- Dependencies:
  - `_deck` depends on control plane CREATE (if present).
  - `api_implementation` changes that reference gateway services depend on the `_deck` change.
- Populate `post_resolution_targets` with gateway service selectors and control plane identifiers.

### 4) Executor
- Execute `_deck` steps per control plane and run `deck gateway apply|sync`.
- Resolve control plane name from the plan (preferred) or Konnect lookup by ID.
- Post-run: resolve gateway services by selector for that control plane, update dependent changes.
- Capture deck stdout/stderr and log a human-readable summary (even though `--json-output` is used).

### 5) Documentation
- Update `docs/declarative.md` to document:
  - `_deck` on control planes
  - external gateway service selector usage
  - plan representation with `post_resolution_targets`
  - deck diff/apply/sync behavior
  - path resolution and base-dir constraints

### 6) Tests
- E2E scenarios under `test/e2e/scenarios/deck`:
  - `deck/basic` (happy path + invalid path + invalid state + sync delete)
  - `deck/multi-file` (multiple deck files in one config)
- Unit tests:
  - `_deck` validation
  - planner `_deck` generation and summary output
  - executor deck run + post-run resolution

## Out of Scope
- Multiple `_deck` configs per control plane.
- Additional external tool integrations.
