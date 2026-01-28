# Deck + Kongctl Declarative Integration (Revised)

## Problem Statement
kongctl manages Konnect resources (control planes, APIs, API implementations) while deck manages Kong Gateway
entities (services, routes, plugins). API implementations depend on gateway services, so running the tools
independently introduces a temporal dependency problem.

## Final Design (Chosen)
Attach a single deck configuration directly to a control plane using the `_deck` key. This is a pseudo-resource
that kongctl executes once per control plane in apply/sync workflows.

### Example
```yaml
control_planes:
  - ref: team-a-cp
    name: "team-a-cp"
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

### Key Behaviors
- `_deck` runs once per control plane.
- `plan/diff` uses `deck gateway diff --json-output --no-color` to decide if a `_deck` change is needed.
- `apply` runs `deck gateway apply`; `sync` runs `deck gateway sync`.
- External gateway services are resolved by selector **after** the deck run.
- API implementations referencing those services wait on the `_deck` step.
- Plans represent deck resolution explicitly with a `post_resolution_targets` list on the `_deck` change.

### Constraints
- Only one `_deck` config per control plane.
- `_deck.files` required; `_deck.flags` optional.
- Flags cannot include Konnect auth or output flags; no `{{kongctl.mode}}` placeholder.
- `_external.requires.deck` is removed (breaking change on this branch).
- `_deck` can be used on managed or external control planes.

### Notes on Select Tags
When multiple deck files or multiple kongctl runs manage the same control plane, deck state files should include
`_info.select_tags` and matching entity `tags` to avoid unintended deletes in sync mode. kongctl does not inject
select tags.

### Implementation Notes (High Level)
- Add `_deck` to `ControlPlaneResource` and validate.
- Planner emits `_deck` changes per control plane and adds dependencies to API implementation changes.
- `_deck` plan changes include control plane metadata, deck file/flag info, and `post_resolution_targets`.
- Executor runs deck per control plane, resolves gateway services post-run, and updates dependent changes.
- Deck runs are skipped in dry-run.
- Deck base directories are resolved relative to the config file and stored relative in plans for portability.
- Deck stdout/stderr are captured; logs emit a readable summary while still using JSON output.
