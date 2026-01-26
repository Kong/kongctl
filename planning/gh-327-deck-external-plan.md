# GH-327 Deck Integration Plan (revised)

## Goal
Integrate deck with kongctl declarative by attaching a **single `_deck` config** to each control plane.
This replaces `_external.requires.deck` on gateway services. Deck runs once per control plane and
external gateway services are resolved after the deck run.

## Proposed UX
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

Notes:
- `_deck` is a pseudo-resource configuration; only one per control plane.
- `_deck` is allowed on managed or external control planes.
- `gateway_services` remain external with selector; no `_external.requires.deck`.

## Behavior Summary
- `plan/diff`:
  - Runs `deck gateway diff --json-output --no-color` once per control plane with `_deck`.
  - If control plane is being created or no ID is available, skip diff and include the `_deck` change.
  - For apply mode, delete-only diffs are ignored.
- `apply/sync`:
  - Ensure control plane exists.
  - Run `deck gateway apply|sync` once per `_deck`.
  - Resolve external gateway services (selector name) and update dependent `api_implementation` changes.

## Validation Rules
- `_deck.files` must include at least one state file.
- `_deck.flags` optional; cannot include Konnect auth flags or output flags; no `{{kongctl.mode}}`.
- Only one `_deck` block per control plane.
- External gateway services must include `_external.selector.matchFields.name`.
- `_external.requires.deck` is no longer supported.

## Implementation Plan (first change is e2e scenario)

1) **E2E scenarios + testdata**
   - Update existing `external/deck-requires` scenario to use control plane `_deck`.
   - Add multi-file scenario where a single `_deck` config references multiple deck files.
   - Add negative scenarios: invalid/missing deck files, invalid deck content.

2) **Resource model / validation**
   - `internal/declarative/resources/control_plane.go`
     - Add `_deck` field and validation.
   - `internal/declarative/resources/external.go`
     - Remove `_external.requires.deck` support.

3) **Loader updates**
   - Record `_deck` base directory per control plane (resolved relative to the source file).
   - Enforce `--base-dir` boundary for `_deck` files.

4) **Planner updates**
   - `internal/declarative/planner/deck_requirements.go`
     - Generate one `_deck` change per control plane with `_deck`.
     - Run deck diff to decide inclusion.
   - Add dependencies: `_deck` depends on CP create; API implementations depending on CP services depend on `_deck`.
   - Update plan summary `by_external_tools` to list `_deck` entries with CP info + files/flags.

5) **Executor updates**
   - Execute `_deck` steps per control plane.
   - Resolve control plane name via plan or Konnect lookup if needed.
   - Post-run, resolve gateway services by selector for that control plane and update dependent changes.
   - Capture deck stdout/stderr and log appropriately.

6) **Docs**
   - `docs/declarative.md`: document `_deck` under control planes; remove `_external.requires.deck`.

## Files likely to change
- `internal/declarative/resources/control_plane.go`
- `internal/declarative/resources/external.go`
- `internal/declarative/loader/*` (deck base dir handling)
- `internal/declarative/planner/deck_requirements.go`
- `internal/declarative/executor/deck_step.go`
- `docs/declarative.md`
- `test/e2e/scenarios/external/deck-requires/**`
- `test/e2e/testdata/declarative/external/deck-requires/**`

