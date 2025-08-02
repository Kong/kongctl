# Investigation Report: Namespace Processing Issue for Child Resources

## Issue Summary

Child resources (e.g., `api_version`) are not properly inheriting namespaces from their parent resources. This manifests in two ways:
1. During `plan` command: child resources show `namespace: ""`
2. During `sync` command: child resources default to the "default" namespace instead of inheriting their parent's namespace

## Root Cause Analysis

### 1. Missing Namespace Assignment in Child Resource Planning

The root cause is in the planner implementation where child resources are created without setting the namespace field in the `PlannedChange` struct.

**Location**: `internal/declarative/planner/api_planner.go`

In the `planAPIVersionCreate` function (line 513), the `PlannedChange` is created without setting the namespace:

```go
change := PlannedChange{
    ID:           p.nextChangeID(ActionCreate, "api_version", version.GetRef()),
    ResourceType: "api_version",
    ResourceRef:  version.GetRef(),
    Parent:       parentInfo,
    Action:       ActionCreate,
    Fields:       fields,
    DependsOn:    dependsOn,
    // NOTE: Namespace field is NOT set here
}
```

This pattern is consistent across all child resource planners:
- `planAPIVersionCreate` (line 513)
- `planAPIPublicationCreate` (line 779)
- `planAPIDocumentCreate` (line 1076)
- `planPortalPageCreate` (observed in portal_child_planner.go)

### 2. Executor Default Behavior

When the executor processes changes with empty namespace, it defaults to "default":

**Location**: `internal/declarative/executor/progress.go`

```go
namespace := change.Namespace
if namespace == "" {
    namespace = "default"
}
```

This happens in multiple places (lines 125, 152, 194) when tracking namespace statistics.

### 3. Namespace Flow

1. **Parent resources** (API, Portal, AuthStrategy) correctly extract namespace from their `kongctl.namespace` field
2. **Namespace context** is properly set when planning: `context.WithValue(ctx, NamespaceContextKey, actualNamespace)`
3. **Child resources** don't have `kongctl` metadata (by design - they inherit from parent)
4. **Missing step**: Child resource planners don't retrieve parent's namespace when creating PlannedChange

## Impact

1. **Plan output**: Shows empty namespace for child resources, making it unclear which namespace they belong to
2. **Sync execution**: Child resources are incorrectly grouped under "default" namespace instead of their parent's namespace
3. **Namespace isolation**: Potential issues with namespace-based filtering and resource management

## Affected Resources

All child resources that inherit from parent resources:
- API child resources:
  - api_version
  - api_publication
  - api_implementation
  - api_document
- Portal child resources:
  - portal_page
  - portal_customization
  - portal_custom_domain
  - portal_snippet

## Solution Recommendations

### Option 1: Pass Parent Namespace During Planning (Recommended)

Modify child resource planning functions to accept and use the parent's namespace:

```go
func (p *Planner) planAPIVersionCreate(
    apiRef string, apiID string, version resources.APIVersionResource, 
    dependsOn []string, parentNamespace string, plan *Plan,
) {
    // ... existing code ...
    
    change := PlannedChange{
        ID:           p.nextChangeID(ActionCreate, "api_version", version.GetRef()),
        ResourceType: "api_version",
        ResourceRef:  version.GetRef(),
        Parent:       parentInfo,
        Action:       ActionCreate,
        Fields:       fields,
        DependsOn:    dependsOn,
        Namespace:    parentNamespace, // Set namespace from parent
    }
    
    // ... rest of function ...
}
```

### Option 2: Extract Namespace from Context

Use the context to pass namespace information:

```go
// Get namespace from context (already available in parent planner)
namespace, ok := ctx.Value(NamespaceContextKey).(string)
if !ok {
    namespace = DefaultNamespace
}
```

### Option 3: Lookup Parent Namespace

When creating child resources, lookup the parent's namespace from the desired resources or existing state.

## Test Case

The issue can be reproduced using the example file at:
`docs/examples/declarative/namespace/single-team/api.yaml`

```yaml
apis:
  - ref: inventory-api
    name: "Inventory Management API"
    kongctl:
      namespace: logistics-team  # Parent has namespace
    versions:
      - ref: inventory-v1        # Child should inherit namespace
        version: "1.0.0"
```

## Related Code Comments

There's an existing comment acknowledging a related bug in:
`test/integration/declarative/api_test.go`

```go
// Due to a known bug in filterResourcesByNamespace, child resources are not being planned
// when they reference parents by ref (the filter uses parent names)
// TODO: Fix this bug in the planner
```

## Conclusion

The namespace inheritance issue for child resources is a systematic problem in the planner implementation. Child resources are created without namespace information, causing them to default to the "default" namespace during execution. The fix requires passing the parent's namespace to child resource planning functions and setting it in the PlannedChange struct.