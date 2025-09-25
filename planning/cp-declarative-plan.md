# Control Plane Support Implementation Plan for Declarative Configuration

## Current State Analysis

### What Exists
1. **Resource Definition**: `internal/declarative/resources/control_plane.go` contains:
   - `ControlPlaneResource` struct with `kkComps.CreateControlPlaneRequest` embedded
   - Standard resource interface implementations (GetRef, Validate, GetType, etc.)
   - `ResourceTypeControlPlane` constant defined in `types.go`
   - Control planes array in `ResourceSet` struct

2. **Documentation References**:
   - Listed as parent resource in `docs/declarative-configuration.md` (line 167)
   - Supports kongctl metadata (namespace, protected fields)

3. **Partial References**:
   - Validator includes control plane validation (`internal/declarative/validator/namespace_validator.go:294`)
   - Reference resolver has TODO stub (`internal/declarative/planner/resolver.go:222-224`)

### What's Missing (Critical Gaps)
1. No planner implementation for control planes
2. No state client methods for control plane CRUD operations
3. No executor operations/adapters
4. Not integrated into main planner's GeneratePlan method
5. No examples or documentation of usage

## Implementation Requirements

### 1. Control Plane Planner (`internal/declarative/planner/control_plane_planner.go`)

Create new file following the pattern of `portal_planner.go` and `api_planner.go`:

```go
package planner

import (
    "context"
    "fmt"

    "github.com/Kong/sdk-konnect-go/models/components"
    "github.com/Kong/kongctl/internal/declarative/resources"
)

// Key functions to implement:
// - planControlPlanes(ctx context.Context, desired []resources.ControlPlaneResource, plan *Plan) error
// - planControlPlaneCreate(namespace, ref string, cp resources.ControlPlaneResource, plan *Plan)
// - planControlPlaneUpdate(currentID string, desired resources.ControlPlaneResource, current state.ControlPlane, plan *Plan)
// - planControlPlaneDelete(ref string, current state.ControlPlane, plan *Plan)
```

Key implementation details:
- Follow namespace filtering pattern from `planPortals`
- Use `p.isResourceInScope()` for namespace checking
- Check protection status before planning deletes
- Build field comparisons for updates (Name, Description, Config fields)
- Add changes to plan with proper dependencies (control planes have no dependencies)

### 2. State Client Methods (`internal/declarative/state/client.go`)

Add these methods following the pattern of Portal/API methods:

```go
// ListManagedControlPlanes returns all KONGCTL-managed control planes in specified namespaces
func (c *Client) ListManagedControlPlanes(ctx context.Context, namespaces []string) ([]ControlPlane, error)

// GetControlPlaneByName finds a managed control plane by name
func (c *Client) GetControlPlaneByName(ctx context.Context, name string) (*ControlPlane, error)

// GetControlPlaneByFilter finds a managed control plane using filter expression
func (c *Client) GetControlPlaneByFilter(ctx context.Context, filter string) (*ControlPlane, error)

// CreateControlPlane creates a new control plane with management labels
func (c *Client) CreateControlPlane(ctx context.Context, cp components.CreateControlPlaneRequest, namespace string) (*components.ControlPlane, error)

// UpdateControlPlane updates an existing control plane
func (c *Client) UpdateControlPlane(ctx context.Context, id string, cp components.UpdateControlPlaneRequest, namespace string) (*components.ControlPlane, error)

// DeleteControlPlane deletes a control plane by ID
func (c *Client) DeleteControlPlane(ctx context.Context, id string) error
```

Also add ControlPlane type to state package:
```go
type ControlPlane struct {
    components.ControlPlane
    NormalizedLabels map[string]string
}
```

Implementation notes:
- Use `c.controlPlaneAPI` (already exists in KonnectSDK)
- Apply label management using `labels.BuildCreateLabels` and `labels.BuildUpdateLabels`
- Follow pagination pattern with `PaginateAll` helper
- Include fallback lookup strategy for protection changes

### 3. Executor Operations (`internal/declarative/executor/control_plane_operations.go`)

Create following the pattern of `portal_operations.go`:

```go
package executor

// Key functions:
// - (e *Executor) executeControlPlaneCreate(ctx context.Context, change Change) error
// - (e *Executor) executeControlPlaneUpdate(ctx context.Context, change Change) error
// - (e *Executor) executeControlPlaneDelete(ctx context.Context, change Change) error
```

Implementation details:
- Extract fields from change.Fields map
- Use state client methods for actual operations
- Include proper error handling with context
- Update progress tracking

### 4. Executor Adapter (`internal/declarative/executor/control_plane_adapter.go`)

Create adapter to convert between resource and SDK types:

```go
package executor

import (
    kkComps "github.com/Kong/sdk-konnect-go/models/components"
    "github.com/Kong/kongctl/internal/declarative/resources"
)

func adaptControlPlaneResourceToCreate(cp resources.ControlPlaneResource, namespace string, protected bool) kkComps.CreateControlPlaneRequest {
    // Build create request from resource
    // Apply labels using labels.BuildCreateLabels
}

func adaptControlPlaneResourceToUpdate(cp resources.ControlPlaneResource, existingLabels map[string]string, namespace string, protected bool) kkComps.UpdateControlPlaneRequest {
    // Build update request
    // Use labels.BuildUpdateLabels for label management
}
```

### 5. Main Planner Integration (`internal/declarative/planner/planner.go`)

In `GeneratePlan` method, add control plane processing after auth strategies and before APIs:

```go
// Plan control planes
if err := p.planControlPlanes(ctx, resourceSet.ControlPlanes, plan); err != nil {
    return nil, fmt.Errorf("failed to plan control planes: %w", err)
}
```

### 6. Executor Integration (`internal/declarative/executor/executor.go`)

In `Execute` method's switch statement, add control plane cases:

```go
case resources.ResourceTypeControlPlane:
    switch change.Action {
    case ActionCreate:
        return e.executeControlPlaneCreate(ctx, change)
    case ActionUpdate:
        return e.executeControlPlaneUpdate(ctx, change)
    case ActionDelete:
        return e.executeControlPlaneDelete(ctx, change)
    }
```

### 7. Reference Resolver Update (`internal/declarative/planner/resolver.go`)

Update the TODO stub at line 222:

```go
func (r *ReferenceResolver) resolveControlPlaneRef(ctx context.Context, ref string) (string, error) {
    cp, err := r.stateClient.GetControlPlaneByName(ctx, ref)
    if err != nil {
        return "", fmt.Errorf("failed to resolve control plane ref '%s': %w", ref, err)
    }
    if cp == nil {
        return "", fmt.Errorf("control plane with ref '%s' not found", ref)
    }
    return cp.ID, nil
}
```

### 8. Documentation Updates

Create `docs/examples/declarative/control-plane/control-plane.yaml`:

```yaml
_defaults:
  kongctl:
    namespace: infrastructure
    protected: false

control_planes:
  - ref: prod-cp
    name: "production-control-plane"
    description: "Production Kong Gateway control plane"
    cluster_type: "CLUSTER_TYPE_CONTROL_PLANE"
    config:
      control_plane_endpoint: "https://cp.example.com"
      telemetry_endpoint: "https://telemetry.example.com"
    kongctl:
      namespace: production
      protected: true

  - ref: staging-cp
    name: "staging-control-plane"
    description: "Staging environment control plane"
    cluster_type: "CLUSTER_TYPE_CONTROL_PLANE"
    config:
      control_plane_endpoint: "https://staging-cp.example.com"
```

Update `docs/declarative-configuration.md` to include control plane examples in the resource types section.

### 9. Testing Requirements

#### Unit Tests

1. `internal/declarative/planner/control_plane_planner_test.go`:
   - Test plan generation for create/update/delete
   - Test namespace filtering
   - Test protection checking
   - Test field comparison logic

2. `internal/declarative/state/client_control_plane_test.go`:
   - Mock SDK responses
   - Test CRUD operations
   - Test label management
   - Test pagination

3. `internal/declarative/executor/control_plane_operations_test.go`:
   - Test execution of each operation type
   - Test error handling
   - Test progress updates

#### Integration Tests

Create `test/integration/declarative/control_plane_test.go`:
- Test full workflow: load config -> plan -> execute
- Test sync mode with existing resources
- Test namespace isolation
- Test protection flags

## Implementation Order

1. State client methods (foundation)
2. Control plane planner
3. Executor operations and adapter
4. Main planner/executor integration
5. Reference resolver update
6. Documentation and examples
7. Unit tests
8. Integration tests

## Critical Implementation Notes

1. **SDK Integration**: The SDK already has `ControlPlanes` API in `KonnectSDK.GetControlPlaneAPI()`. Use existing operations.

2. **Label Management**: Control planes must use the same label pattern as other resources:
   - `KONGCTL-NAMESPACE`: For namespace isolation
   - `KONGCTL-PROTECTED`: For deletion protection
   - `KONGCTL-MANAGED`: To identify managed resources

3. **Field Mapping**: Based on `CreateControlPlaneRequest` in SDK:
   - Name (string)
   - Description (*string)
   - ClusterType (*components.ClusterType)
   - Config (*components.Config with ControlPlaneEndpoint, TelemetryEndpoint)
   - Labels (map[string]*string)

4. **Namespace Behavior**: Control planes are parent resources, so they:
   - Support kongctl metadata section
   - Can have namespace and protected fields
   - Don't inherit namespace from parents (they ARE parents)

5. **Dependencies**: Control planes have no dependencies on other resources, making them good to process early in the plan.

6. **Error Handling**: Follow enhanced error patterns from API/Portal implementations with proper context and hints.

## Validation Checklist

After implementation, verify:
- [ ] Control planes can be created via `apply`
- [ ] Control planes can be updated with field changes
- [ ] Control planes can be deleted via `sync` (respecting protection)
- [ ] Namespace filtering works correctly
- [ ] Protection flags prevent deletion
- [ ] Reference resolution works for resources referencing control planes
- [ ] Plan output shows correct operations
- [ ] Diff command shows control plane changes
- [ ] Examples work end-to-end

## Known Limitations

1. SDK may not support all control plane configuration fields
2. API implementations that reference control planes are commented out (TODO in code)
3. Control plane groups are not yet modeled