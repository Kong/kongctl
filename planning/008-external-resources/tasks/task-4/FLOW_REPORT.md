# External Resources Flow Analysis Report

## Executive Summary

This report provides a comprehensive analysis of the execution flow for external resources implementation in kongctl. The investigation reveals that Steps 1-2 have 
created solid infrastructure, and Step 3 (External Resource Resolver) needs to be implemented at a specific integration point in the planner's resolution flow.

## Current Execution Flow Analysis

### 1. Plan Command Flow

```
User Command: kongctl plan -f config.yaml
    │
    ├─ cmd/plan.go::NewPlanCmd()
    │   └─ Delegates to konnect.NewKonnectCmd(Verb)
    │
    ├─ konnect.go::NewKonnectCmd()
    │   └─ For Plan verb: calls declarative.NewDeclarativeCmd(verb)
    │
    ├─ declarative.go::newDeclarativePlanCmd()
    │   └─ RunE: runPlan()
    │
    └─ declarative.go::runPlan()
        ├─ loader.LoadFromSources() → ResourceSet
        ├─ planner.NewPlanner() → Planner instance
        └─ p.GeneratePlan() → Plan artifact
```

### 2. Configuration Loading Flow

```
File Sources (YAML/JSON)
    │
    ├─ loader.ParseSources(filenames) → []Source
    │
    ├─ ldr.LoadFromSources(sources, recursive) → ResourceSet
    │   ├─ Parse YAML/JSON files
    │   ├─ Unmarshal into resource types
    │   ├─ Validate resource schemas
    │   └─ Build ResourceSet structure
    │
    └─ ResourceSet contains:
        ├─ Portals []PortalResource
        ├─ APIs []APIResource  
        ├─ ExternalResources []ExternalResourceResource ← KEY FOR STEP 3
        └─ ... other resource types
```

### 3. Planner GeneratePlan Flow

```
planner.GeneratePlan(ctx, resourceSet, opts)
    │
    ├─ Pre-resolution Phase (LINE 86)
    │   └─ p.resolveResourceIdentities(ctx, rs) ← CRITICAL INTEGRATION POINT
    │       ├─ p.validateExternalResources(rs.ExternalResources) ← CURRENT PLACEHOLDER
    │       ├─ p.resolveAPIIdentities(ctx, rs.APIs)
    │       ├─ p.resolvePortalIdentities(ctx, rs.Portals)
    │       └─ p.resolveAuthStrategyIdentities(ctx, rs.ApplicationAuthStrategies)
    │
    ├─ Namespace Processing
    │   ├─ Extract namespaces from resources
    │   └─ Process each namespace independently
    │
    ├─ Change Generation
    │   ├─ authStrategyPlanner.PlanChanges()
    │   ├─ portalPlanner.PlanChanges()
    │   └─ apiPlanner.PlanChanges()
    │
    ├─ Reference Resolution (LINE 198)
    │   └─ p.resolver.ResolveReferences(ctx, basePlan.Changes)
    │
    ├─ Dependency Resolution (LINE 223)
    │   └─ p.depResolver.ResolveDependencies(basePlan.Changes)
    │
    └─ Return complete Plan with execution order
```

## Step 3 Integration Point Analysis

### Current Implementation Gap

**File**: `/internal/declarative/planner/planner.go`
**Method**: `validateExternalResources()` (Lines 416-441)
**Status**: Placeholder implementation that only logs external resources

```go
// Current placeholder implementation
func (p *Planner) validateExternalResources(externalResources []resources.ExternalResourceResource) {
    // For now, we just ensure external resources are structurally valid
    // TODO: In future steps, this will:
    // 1. Build dependency graph for resolution order
    // 2. Resolve parent resources first  
    // 3. Execute SDK queries to resolve IDs
    // 4. Validate exactly one match for selectors
    // 5. Cache resolved resources for reference resolution
}
```

### Required Implementation

**New Method**: `resolveExternalResources()` to replace `validateExternalResources()`

```go
func (p *Planner) resolveExternalResources(ctx context.Context, externalResources []resources.ExternalResourceResource) error {
    if len(externalResources) == 0 {
        return nil
    }
    
    // Create and configure external resource resolver
    resolver := external.NewExternalResourceResolver(registry, p.client, p.logger)
    
    // Perform resolution (builds dependency graph, executes SDK queries)
    return resolver.ResolveExternalResources(ctx, externalResources)
}
```

## Data Flow Analysis

### 1. Configuration to Resolution Flow

```
External Resource in YAML
    │
    ├─ Parsed into ExternalResourceResource struct
    │   ├─ Ref: "prod-portal"
    │   ├─ ResourceType: "portal"
    │   ├─ Selector: {name: "Production Portal"}
    │   └─ Parent: nil (top-level resource)
    │
    ├─ Passes through Loader validation
    │   ├─ ValidateRef()
    │   ├─ ValidateResourceType() ← Uses registry
    │   ├─ ValidateIDXORSelector()
    │   └─ ValidateParent() if present
    │
    ├─ Added to ResourceSet.ExternalResources[]
    │
    └─ Sent to ExternalResourceResolver for resolution
        ├─ Registry.GetResolutionAdapter("portal")
        │   └─ Returns PortalResolutionAdapter
        │
        ├─ Adapter.GetBySelector(ctx, {name: "Production Portal"})
        │   └─ StateClient.ListPortalsWithFilter()
        │       └─ SDK API call to Konnect
        │
        ├─ Validate exactly one match
        ├─ Extract Konnect ID from response
        └─ ExternalResource.SetResolvedID(id)
```

### 2. Dependencies and Parent-Child Resolution

```
Dependency Graph Example:
    control_plane: "prod-cp" (parent)
        │
        └─ ce_service: "prod-service" (child)

Resolution Order:
    1. Resolve "prod-cp" first
       ├─ ControlPlaneAdapter.GetBySelector()
       ├─ Validate single match
       └─ Store resolved ID
    
    2. Resolve "prod-service" with parent context
       ├─ Get resolved parent from cache
       ├─ ServiceAdapter.GetBySelector(ctx, selector, parent)
       ├─ SDK call with parent context
       └─ Store resolved ID
```

### 3. Integration with Reference Resolution

```
External Resource Resolution → Reference Resolution Integration
    │
    ├─ ExternalResourceResolver stores resolved IDs in:
    │   └─ Map[ref] → ResolvedExternalResource{ID, Resource}
    │
    ├─ ReferenceResolver.ResolveReferences() needs extension:
    │   ├─ Current: resolveReference(ctx, resourceType, ref)
    │   │   └─ Only queries SDK for existing resources
    │   │
    │   └─ Enhanced: Check external resources first
    │       ├─ if resolver.HasResolvedResource(ref) 
    │       │   └─ return resolver.GetResolvedID(ref)
    │       └─ else fallback to current SDK query
    │
    └─ PlannedChange.References populated with external IDs
```

## Component Interconnection Analysis

### 1. Registry and Adapter System

```
ResolutionRegistry (Singleton)
    │
    ├─ Metadata Storage
    │   ├─ 13 resource types registered
    │   ├─ Selector fields per type
    │   ├─ Parent-child relationships
    │   └─ Adapter instances (injected)
    │
    ├─ Adapter Factory
    │   ├─ GetResolutionAdapter(resourceType) → ResolutionAdapter
    │   └─ Dependency injection from AdapterFactory
    │
    └─ Validation Support
        ├─ IsSupported(resourceType)
        ├─ GetSupportedSelectorFields(resourceType)
        └─ IsValidParentChild(parentType, childType)

Adapters Architecture:
    BaseAdapter
        ├─ Common SDK error handling
        ├─ Parent context validation
        ├─ Selector filtering with exact match validation
        └─ State client access
    
    Concrete Adapters (13 types):
        ├─ PortalResolutionAdapter
        ├─ APIResolutionAdapter
        ├─ ControlPlaneResolutionAdapter
        ├─ ServiceResolutionAdapter
        └─ ... 9 more child resource adapters
```

### 2. State Client Integration

```
State Client Architecture
    │
    ├─ SDK Wrapper Layer
    │   ├─ Portal API methods
    │   ├─ API API methods
    │   ├─ Control Plane API methods  
    │   ├─ Core Entity Services API methods
    │   └─ Child resource API methods
    │
    ├─ Client Configuration
    │   └─ Created in declarative.go::createStateClient()
    │       ├─ Injects all 13+ SDK APIs
    │       └─ Used by both planner and adapters
    │
    └─ Usage Pattern in Adapters
        ├─ adapter.GetClient().GetPortalByID(ctx, id)
        ├─ adapter.GetClient().ListPortalsWithFilter(ctx, selector)
        └─ SDK calls return structured objects
```

### 3. Error Flow and Propagation

```
Error Handling Chain:
    SDK API Error
        │
        ├─ Adapter.GetBySelector() catches and wraps
        │   └─ "failed to resolve portals by selector %v: %w"
        │
        ├─ ExternalResourceResolver.resolveResource() adds context
        │   └─ "external resource 'prod-portal': %w"
        │
        ├─ Planner.resolveResourceIdentities() adds planning context
        │   └─ "failed to resolve resource identities: %w"
        │
        └─ GeneratePlan() returns to command level
            └─ "failed to generate plan: %w"
```

## Key Integration Points for Step 3

### 1. Planner Integration

**File**: `/internal/declarative/planner/planner.go`
**Required Changes**:

```go
type Planner struct {
    client             *state.Client
    logger             *slog.Logger
    resolver           *ReferenceResolver
    depResolver        *DependencyResolver
    externalResolver   *external.ExternalResourceResolver // NEW FIELD
    // ... existing fields
}

func NewPlanner(client *state.Client, logger *slog.Logger) *Planner {
    // ... existing initialization
    
    // NEW: Initialize external resource resolver
    registry := external.GetResolutionRegistry()
    externalResolver := external.NewExternalResourceResolver(registry, client, logger)
    
    p.externalResolver = externalResolver
    return p
}
```

### 2. Resolution Method Replacement

**Current** (Line 391):
```go
p.validateExternalResources(rs.ExternalResources)
```

**New Implementation**:
```go
if err := p.externalResolver.ResolveExternalResources(ctx, rs.ExternalResources); err != nil {
    return fmt.Errorf("failed to resolve external resources: %w", err)
}
```

### 3. Reference Resolver Enhancement

**File**: `/internal/declarative/planner/resolver.go`
**Required Changes**:

```go
type ReferenceResolver struct {
    client           *state.Client
    externalResolver *external.ExternalResourceResolver // NEW FIELD
}

func (r *ReferenceResolver) resolveReference(ctx context.Context, resourceType, ref string) (string, error) {
    // NEW: Check external resources first
    if r.externalResolver != nil {
        if resolvedResource, found := r.externalResolver.GetResolvedResource(ref); found {
            return resolvedResource.ID, nil
        }
    }
    
    // Existing logic as fallback
    switch resourceType {
    // ... existing cases
    }
}
```

## Implementation Sequence Analysis

### Phase 1: Core Resolver Implementation

**New Files to Create**:
```
/internal/declarative/external/resolver.go
/internal/declarative/external/resolver_test.go
/internal/declarative/external/dependencies.go
/internal/declarative/external/dependencies_test.go
```

**Key Components**:
1. `ExternalResourceResolver` struct
2. `ResolveExternalResources()` method  
3. `buildDependencyGraph()` for parent-child ordering
4. `resolveResource()` for individual resolution
5. `GetResolvedResource()` for reference lookup

### Phase 2: Planner Integration

**Files to Modify**:
```
/internal/declarative/planner/planner.go
/internal/declarative/planner/resolver.go
```

**Key Changes**:
1. Add ExternalResourceResolver field to Planner
2. Initialize resolver in NewPlanner()
3. Replace validateExternalResources() with actual resolution
4. Enhance ReferenceResolver to check external resources

### Phase 3: End-to-End Testing

**Test Strategy**:
1. Unit tests for resolver components
2. Integration tests with real adapters
3. End-to-end tests with plan generation
4. Error handling validation

## Performance and Caching Considerations

### Resolution Caching Strategy

```
ExternalResourceResolver
    │
    ├─ In-Memory Cache
    │   └─ Map[ref] → ResolvedExternalResource
    │       ├─ ID: resolved Konnect ID
    │       ├─ Resource: full SDK response object
    │       └─ Metadata: resolution timestamp, parent info
    │
    ├─ Cache Lifecycle
    │   ├─ Populate: During ResolveExternalResources()
    │   ├─ Access: During reference resolution phase
    │   └─ Scope: Single plan generation cycle
    │
    └─ Performance Benefits
        ├─ No duplicate SDK calls for same external resource
        ├─ Fast lookup during reference resolution
        └─ Immediate availability for multiple references
```

### SDK Call Optimization

```
Batch Operation Strategy:
    1. Group by resource type and parent context
    2. Single SDK call per group where possible
    3. Filter results by multiple selectors locally
    4. Validate exact matches per external resource

Parent-Child Optimization:
    1. Resolve parents first (dependency graph ordering)
    2. Pass parent context to child resolution
    3. Minimize SDK calls through context reuse
```

## Error Scenarios and Recovery

### Common Error Patterns

1. **Zero Matches**:
   ```
   Error: External resource 'prod-portal' selector matched 0 resources
     Resource type: portal
     Selector: matchFields: {name: "Production Portal"}
     Suggestion: Verify the resource exists in Konnect
   ```

2. **Multiple Matches**:
   ```
   Error: External resource 'prod-portal' selector matched 3 resources
     Resource type: portal  
     Selector: matchFields: {name: "Portal"}
     Suggestion: Use more specific selector fields to match exactly one resource
   ```

3. **Parent Resolution Failure**:
   ```
   Error: Failed to resolve parent for external resource 'prod-service'
     Parent: control_plane 'prod-cp' 
     Child: ce_service 'prod-service'
     Cause: Parent resource not found or access denied
   ```

4. **SDK Connection Errors**:
   ```
   Error: Failed to query Konnect API for external resource resolution
     Resource: portal 'prod-portal'
     Cause: network timeout / authentication failure / API error
   ```

## Security and Access Control

### Authentication Flow

```
Command Execution → SDK Client → Konnect APIs
    │
    ├─ PAT Token from config/flags
    ├─ Base URL configuration  
    ├─ SDK client initialization
    └─ API calls with authentication headers
```

### Access Control Considerations

- External resource resolution requires read access to all resource types
- Parent-child resolution may span different Konnect endpoints
- Error messages should not leak sensitive information
- Failed authentication should fail fast with clear error

## Conclusion

The external resources implementation has a solid foundation with Steps 1-2 completed. Step 3 implementation has a clear integration point at `planner.resolveResourceIdentities()` and leverages existing infrastructure:

- **Registry System**: Complete with 13 adapters ready for use
- **Schema Validation**: Comprehensive validation framework in place
- **State Client**: Full SDK integration for all resource types
- **Error Handling**: Consistent patterns established

The ExternalResourceResolver will integrate seamlessly into the existing planner flow and provide resolved IDs for the reference resolution system.

## Flow Report Location

This comprehensive flow analysis report has been saved to:
`/Users/rick.spurgeon@konghq.com/go/src/github.com/Kong/kongctl/planning/008-external-resources/tasks/task-4/FLOW_REPORT.md`

The analysis shows clear execution paths, dependencies, and integration points for implementing Step 3 of the external resources feature. The existing infrastructure provides a solid foundation for the ExternalResourceResolver implementation.
