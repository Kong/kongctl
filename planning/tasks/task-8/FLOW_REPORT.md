# Flow Report: Namespace Processing for Child Resources in kongctl

## Executive Summary

This report traces the execution flow of namespace processing in kongctl, specifically focusing on why child resources (e.g., `api_version`) are not properly inheriting namespaces from their parent resources. The analysis reveals that while parent resources correctly extract and use namespace information, child resource planning functions fail to set the namespace field in their `PlannedChange` structs, causing them to default to the "default" namespace during execution.

## Issue Overview

- **Parent Resource**: API with namespace "logistics-team" correctly shows namespace in plan
- **Child Resources**: api_version, api_publication, api_document show empty namespace in plan
- **During Execution**: Child resources default to "default" namespace instead of inheriting parent's namespace

## Command Flow Analysis

### 1. Plan Command Execution Flow

```
User Input: kongctl plan -f api.yaml
    ↓
internal/cmd/root/verbs/plan/plan.go::NewPlanCmd()
    ↓
internal/cmd/root/products/konnect/konnect.go::NewKonnectCmd(verbs.Plan)
    ↓
internal/cmd/root/products/konnect/declarative/declarative.go::NewDeclarativeCmd(verbs.Plan)
    ↓
declarative.go::runPlan()
    ├─ Loads configuration files via loader.LoadFromSources()
    ├─ Creates planner instance: planner.NewPlanner()
    └─ Generates plan: planner.GeneratePlan()
```

### 2. Sync Command Execution Flow

```
User Input: kongctl sync -f api.yaml
    ↓
internal/cmd/root/verbs/sync/sync.go::NewSyncCmd()
    ↓
[Similar flow to plan command]
    ↓
declarative.go::runSync()
    ├─ Loads/generates plan
    ├─ Creates executor: executor.New()
    └─ Executes plan: executor.Execute()
```

## Namespace Processing Flow

### 1. Namespace Extraction (planner.go)

```go
// Line 479-507: getResourceNamespaces()
- Extracts namespaces from parent resources ONLY:
  - Portals: via portal.Kongctl.Namespace
  - APIs: via api.Kongctl.Namespace  
  - AuthStrategies: via strategy.Kongctl.Namespace
- Child resources are NOT checked (by design - they inherit from parent)
- Returns sorted list of unique namespaces
```

### 2. Per-Namespace Planning (planner.go)

```go
// Line 112-187: GeneratePlan()
for _, namespace := range namespaces {
    // Create namespace-specific context
    plannerCtx := context.WithValue(ctx, NamespaceContextKey, actualNamespace)
    
    // Filter resources for this namespace
    namespaceResources = p.filterResourcesByNamespace(rs, namespace)
    
    // Plan changes for each resource type
    namespacePlanner.apiPlanner.PlanChanges(plannerCtx, namespacePlan)
}
```

### 3. Resource Filtering (planner.go)

```go
// Line 519-603: filterResourcesByNamespace()
- Filters parent resources by namespace
- Includes child resources if their parent is in the filtered set
- Example: api_version included if its parent API is in namespace
```

## API Resource Planning Flow

### 1. Parent API Planning (api_planner.go)

```go
// Line 59-73: PlanChanges()
- Extracts namespace from context: ctx.Value(NamespaceContextKey)
- Uses namespace to filter current APIs

// Line 201-240: planAPICreate()  
- Extracts namespace from api.Kongctl.Namespace (line 210-214)
- Passes namespace to CreateConfig (line 224)
- Results in PlannedChange with correct namespace
```

### 2. Child Resource Planning - THE BUG

#### API Version Creation (api_planner.go)
```go
// Line 511-563: planAPIVersionCreate()
change := PlannedChange{
    ID:           p.nextChangeID(ActionCreate, "api_version", version.GetRef()),
    ResourceType: "api_version",
    ResourceRef:  version.GetRef(),
    Parent:       parentInfo,
    Action:       ActionCreate,
    Fields:       fields,
    DependsOn:    dependsOn,
    // BUG: Namespace field is NOT set here!
}
```

#### API Publication Creation (api_planner.go)
```go
// Line 779-831: planAPIPublicationCreate()
change := PlannedChange{
    // ... other fields ...
    // BUG: Namespace field is NOT set here!
}
```

#### API Document Creation (api_planner.go)
```go
// Line 1076-1108: planAPIDocumentCreate()
change := PlannedChange{
    // ... other fields ...
    // BUG: Namespace field is NOT set here!
}
```

### 3. Context Availability

The namespace IS available in the context when planning child resources:

```go
// Line 133: planAPIChildResourceChanges called with ctx
if err := p.planAPIChildResourceChanges(ctx, current, desiredAPI, plan); err != nil

// Line 434-436: Context passed to child planning methods
func (p *Planner) planAPIVersionChanges(
    ctx context.Context, apiID string, apiRef string, desired []resources.APIVersionResource, plan *Plan,
) error {
```

## Executor Namespace Defaulting

### Progress Reporter (progress.go)

When the executor processes changes with empty namespaces:

```go
// Line 83-86: StartChange()
namespace := change.Namespace
if namespace == "" {
    namespace = "default"  // Child resources hit this path
}

// Line 111-114: CompleteChange()
namespace := change.Namespace
if namespace == "" {
    namespace = "default"  // Same defaulting
}

// Line 136-139: SkipChange()
namespace := change.Namespace
if namespace == "" {
    namespace = "default"  // Same defaulting
}
```

## Root Cause Summary

1. **Parent resources** correctly set namespace because:
   - They have `kongctl.namespace` metadata in YAML
   - Planner extracts this and sets it in PlannedChange

2. **Child resources** have empty namespace because:
   - They don't have `kongctl` metadata (inherit from parent by design)
   - Planner functions don't extract namespace from context
   - PlannedChange.Namespace field is never set

3. **During execution**:
   - Empty namespace defaults to "default" in progress reporter
   - Child resources appear under wrong namespace in output

## Solution Path

The fix requires modifying child resource planning functions to:

1. Extract namespace from context OR from parent resource
2. Set the Namespace field when creating PlannedChange

Example fix for `planAPIVersionCreate`:
```go
func (p *Planner) planAPIVersionCreate(
    ctx context.Context, // Add context parameter
    apiRef string, apiID string, version resources.APIVersionResource, 
    dependsOn []string, parentNamespace string, plan *Plan,
) {
    change := PlannedChange{
        // ... existing fields ...
        Namespace: parentNamespace, // Set namespace from parent
    }
}
```

## Affected Files

1. **Planner Implementation**:
   - `internal/declarative/planner/api_planner.go` - Child resource planning functions
   - `internal/declarative/planner/portal_child_planner.go` - Portal child resources

2. **Executor/Reporter**:
   - `internal/declarative/executor/progress.go` - Namespace defaulting logic

3. **Test Files**:
   - `test/integration/declarative/api_test.go` - Contains TODO comment about namespace bug

## Execution Trace Example

```
1. User runs: kongctl plan -f api.yaml
2. YAML contains API with namespace: "logistics-team"
3. Planner extracts namespace "logistics-team" from API
4. Creates namespace-specific context with "logistics-team"
5. Plans API creation with namespace: "logistics-team" ✓
6. Plans api_version creation WITHOUT namespace ✗
7. Plan output shows:
   - API: namespace: "logistics-team"
   - api_version: namespace: ""
8. During sync execution:
   - API created in "logistics-team" namespace
   - api_version defaults to "default" namespace
```

## Conclusion

The namespace inheritance issue is a systematic problem in the planner implementation where child resource planning functions fail to propagate the parent's namespace to the `PlannedChange` struct. The namespace information is available (via context and parent resource), but is not being utilized when creating child resource changes. This causes all child resources to default to the "default" namespace during execution, breaking the intended namespace isolation.