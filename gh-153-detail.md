# gh-153 – Planner emits duplicate `portal_custom_domain` creates

## Problem Statement

After provisioning a portal with a declarative configuration that includes a custom
domain, running `kongctl plan` immediately afterwards still produces a
`portal_custom_domain` `CREATE` change. The planned change is incorrect because the
domain already exists in Konnect.

## Observed Flow

```bash
./kongctl apply -f test/e2e/testdata/declarative/portal/custom-domain/config.yaml --auto-approve
./kongctl plan --mode apply -f test/e2e/testdata/declarative/portal/custom-domain/config.yaml
```

The second command schedules:

```
Action: CREATE
Resource: portal_custom_domain portal-1-custom-domain
```

API calls (`/v3/portals/{id}` and `/v3/portals/{id}/custom-domain`) confirm the domain
already exists with the expected hostname and verification method, so a no-op plan
should be produced.

## Root Cause

- `internal/declarative/planner/portal_child_planner.go` implements
  `planPortalCustomDomainsChanges`. The function blindly calls
  `planPortalCustomDomainCreate` for every desired domain without inspecting Konnect
  state.
- The state layer has no way to surface existing custom domains:
  - `internal/declarative/state/client.go` only exposes create/update/delete helpers –
    no `GetPortalCustomDomain`.
  - `internal/declarative/state/cache.go` contains an empty placeholder struct
    `PortalCustomDomain`, so even if state were loaded, it would carry no data.
- Because the planner cannot diff “desired vs current”, it always schedules `CREATE`.

The other singleton portal child resource (`portal_customization`) already follows
the expected pattern: fetch current state, compare, and emit an `UPDATE` only when a
diff exists.

## Desired Behaviour

For each portal:

1. If the portal does not yet exist in Konnect (new portal), plan a `CREATE` for the
   custom domain (with dependencies on the portal create change).
2. If the portal exists:
   - Fetch the existing custom domain.
   - If no domain exists, plan a `CREATE`.
   - If one exists:
     - If `hostname`, verification method, or skip-CA flag differ:
       - Plan a `DELETE` (sync mode only) followed by a `CREATE`, or force a replace
         within apply mode by always creating after deleting.
     - If only `enabled` differs, plan an `UPDATE` using the update endpoint.
     - Otherwise, emit no change.
   - In sync mode, if the desired configuration omits the custom domain but one exists
     remotely, plan a `DELETE`.

## Proposed Implementation

### 1. State Layer Enhancements

- Implement `Client.GetPortalCustomDomain(ctx, portalID)` in
  `internal/declarative/state/client.go`:
  - Validate API client (`ValidateAPIClient`).
  - Call `PortalCustomDomains.GetPortalCustomDomain`.
  - Translate the SDK response into a populated internal struct.
  - Return `(nil, nil)` when the API responds with 404 (no domain).
- Define `PortalCustomDomain` in `internal/declarative/state/cache.go` with fields:
  - `Hostname`, `Enabled`, `VerificationMethod`, `SkipCACheck`, timestamps, etc.
  - Use the type inside `CachedPortal.CustomDomain`.
- Add a helper to normalize the SDK union:
  - Map `kkComps.PortalCustomDomain.Ssl.DomainVerificationMethod` to an enum/string.
  - Extract `SkipCaCheck` only when present.

### 2. Planner Changes

- Modify `planPortalCustomDomainsChanges` to accept the portal ID when available.
- For existing portals:
  1. Retrieve the current domain via the new state helper.
  2. Compare against the desired resource:
     - `hostname` and verification method are immutable once created. For differences,
       plan a delete (if permitted) plus a create. In apply mode, we can emulate
       replace by scheduling delete + create; in sync mode the delete should depend
       on plan mode.
     - `enabled` can be toggled via the update endpoint. If the desired value differs
       from the current, emit an `ActionUpdate` change that carries only the
       `enabled` field.
     - For skip-CA, treat it as part of the immutable SSL configuration (requires
       replace).
  3. Ensure change dependencies reference the parent portal change when the portal
     is new or when a delete/create pair is being emitted.
- For new portals (portal ID unknown):
  - Continue scheduling a create, but ensure the change records the parent lookup so
    the executor can resolve the portal ID at runtime.
- When plan mode is sync:
  - If the desired config omits a custom domain and one exists remotely, add a
    `DELETE` change.
  - Only attempt delete if the domain currently exists (ignore 404s gracefully).

### 3. Executor Considerations

- The existing portal custom domain executor (`PortalDomainAdapter`) already maps
  create, update (enabled), and delete to the state client. Once the planner emits
  the right change type, no executor modification should be necessary.
- Verify that the executor sets `Parent.ID` / reference IDs correctly when the portal
  already exists; the planner should populate `Parent.ID` when the portal ID is
  known.

## Implementation Steps

1. **State**  
   - Add a real `PortalCustomDomain` struct plus mapping helpers.  
   - Implement `Client.GetPortalCustomDomain`.  
   - Update cache constructors to store the fetched domain.

2. **Planner**  
   - Refactor `planPortalCustomDomainsChanges` to fetch state and branch on existing
     vs new.  
   - Introduce helper functions:
     - `planPortalCustomDomainCreate`.
     - `planPortalCustomDomainUpdate` (enabled only).
     - `planPortalCustomDomainDelete`.  
   - Ensure dependency wiring (`DependsOn`, `Parent`, `References`) matches existing
     patterns.

3. **Tests**  
   - Extend unit tests in `internal/declarative/planner/portal_child_test.go` to cover:
     - Existing domain matches desired (no change).
     - Existing domain differs by enabled flag (update).
     - Existing domain requires replacement (delete + create).  
   - Add integration coverage to the new E2E scenario overlays (e.g. start with HTTP,
     script a plan with no changes, toggle enabled, migrate to custom certificate,
     and remove domain).  
   - Verify sync mode deletes by adding a scenario overlay that removes the custom
     domain and running plan/sync.

4. **Documentation**  
   - Update `test/e2e/COVERAGE_ANALYSIS_2025-11-01.md` to reflect the new checks once
     the tests pass.

## Testing Strategy

- **Unit Tests**: Expand existing planner tests with mocked state client responses.
- **E2E Scenario**: Extend `test/e2e/scenarios/portal/custom-domain` overlays:
  - Validate that the second overlay (plan after initial apply) emits no changes.
  - Add a step toggling `enabled` and confirm the plan schedules `UPDATE`.
  - In sync mode overlays, confirm deletions occur.
- **Manual Smoke**: Repeat the reproduction steps to confirm `kongctl plan` no longer
  emits a duplicate create after the fix.

## Risks & Mitigations

- **API Availability**: If `PortalCustomDomains.GetPortalCustomDomain` is not exposed in
  all regions, the new state call might fail. Handle “client not configured” errors by
  skipping diffing (fall back to create with a plan warning).
- **Delete Support**: Confirm the executor/state client gracefully handles deleting a
  domain that may still be pending verification.
- **Toggle Semantics**: Ensure the update path only toggles `enabled`; other fields are
  immutable and should be replaced via delete + create.

## Acceptance Criteria

- Running `kongctl plan --mode apply -f …/portal/custom-domain/config.yaml` after the
  first apply produces an empty plan.  
- Toggling `enabled` results in a single `UPDATE`.  
- Switching from HTTP to custom certificate produces a delete/create pair.  
- Sync mode removes unmanaged domains.  
- Automated tests exercise the above scenarios.
