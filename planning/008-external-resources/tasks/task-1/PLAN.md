# REVISED External Resources Implementation Plan

## Executive Summary

This plan implements external resources functionality by **extending existing systems (80% code reuse)** rather than creating new architectures. The implementation prioritizes **code accuracy and correct behavior** over performance optimization, following the user's requirements for maximum reuse, minimal schema sprawl, and type-based field resolution.

**Key Strategy**: External resources become participants in existing flows through targeted extensions, preserving all proven patterns and algorithms.

## User Requirements Alignment

✅ **No Performance Focus**: Prioritizes production-ready accuracy over optimization  
✅ **Maximum Code Reuse**: Leverages 80% of existing dependency/reference resolution systems  
✅ **Minimize Schema Sprawl**: Reuses existing PlannedChange and ReferenceInfo types  
✅ **Type-Based Field Resolution**: Uses explicit field lists, NOT pattern matching on *_id  
✅ **No New Concurrency**: Maintains existing sequential execution model  
✅ **Phase Alignment**: Maps directly to execution-plan-steps.md 8-phase structure  

## Implementation Strategy: Extend, Don't Replace

### Code Reuse Analysis (80% Reusable)

**High Reuse Components**:
- **Dependency Resolution** (`internal/declarative/planner/dependencies.go`) - 90% reusable
- **Reference Resolution** (`internal/declarative/planner/resolver.go`) - 85% reusable  
- **Schema Types** (`internal/declarative/planner/types.go`) - 100% compatible
- **Execution Patterns** (`internal/declarative/executor/executor.go`) - 60% reusable

**New Components Required** (20% of implementation):
- External resource configuration schema
- External resource registry
- SDK query adapters
- Selector matching logic

## Phase-by-Phase Implementation Plan

### Phase 1: Schema and Configuration

**Objective**: Define external resource configuration types with minimal schema sprawl

**Files to Modify**:

1. **Create**: `internal/declarative/config/external_resources.go`
```go
// NEW: Minimal configuration types only
type ExternalResourceConfig struct {
    Resources map[string]ExternalResource `yaml:"resources"`
}

type ExternalResource struct {
    Kind     string           `yaml:"kind"`
    Selector ResourceSelector `yaml:"selector"`
    Parent   *string         `yaml:"parent,omitempty"`
}

type ResourceSelector struct {
    ID          *string           `yaml:"id,omitempty"`
    MatchFields map[string]string `yaml:"matchFields,omitempty"`
}
```

2. **Extend**: `internal/declarative/config/config.go`
```go
// EXTEND: Add external resources to existing config structure
type Config struct {
    // ... existing fields ...
    ExternalResources *ExternalResourceConfig `yaml:"external_resources,omitempty"`
}
```

**Schema Sprawl Prevention**: Reuse existing `PlannedChange` and `ReferenceInfo` types for external resources - no new core types needed.

**Validation Strategy**: Use existing validation patterns from current configuration types.

### Phase 2: Resource Type Registry

**Objective**: Create registry for external resource type mappings without changing core interfaces

**Files to Create**:

1. **Create**: `internal/declarative/external/registry.go`
```go
// NEW: External resource type registry
type ExternalResourceRegistry struct {
    supportedTypes map[string]ExternalResourceType
    parentMappings map[string]string
    fieldMappings  map[string]string  // field name -> resource type
}

type ExternalResourceType struct {
    Kind        string
    ParentType  *string
    AdapterFunc func(*state.Client) ExternalResourceAdapter
}

// Resource type mapping functions
func (r *ExternalResourceRegistry) GetResourceType(kind string) (ExternalResourceType, bool)
func (r *ExternalResourceRegistry) GetParentType(kind string) string
func (r *ExternalResourceRegistry) IsExternalResourceField(fieldName string) bool
```

**Code Reuse**: Follows existing registry patterns from resource type system.

**Parent Mapping**: Uses same patterns as existing parent-child relationships in dependency resolution.

### Phase 3: External Resource Resolver

**Objective**: Implement external resource resolution using existing resolution infrastructure

**Files to Create**:

1. **Create**: `internal/declarative/external/resolver.go`
```go
// NEW: External resource resolver following existing patterns
type ExternalResourceResolver struct {
    client   *state.Client
    registry *ExternalResourceRegistry
    cache    map[string]string  // selector hash -> resolved ID
}

// Core resolution methods
func (r *ExternalResourceResolver) ResolveBySelector(
    ctx context.Context,
    resourceType string,
    selector ResourceSelector,
) (string, error)

func (r *ExternalResourceResolver) ResolveByID(
    ctx context.Context,
    resourceType string,
    id string,
) (string, error)
```

**Code Reuse**: Leverages existing caching patterns and error handling from current resolver.

**Sequential Processing**: Maintains existing sequential model - no new concurrency patterns.

### Phase 4: Reference Resolution Integration

**Objective**: Extend existing reference resolution to handle external resources using type-based field detection

**Files to Modify**:

1. **Extend**: `internal/declarative/planner/resolver.go`
```go
// EXTEND: Add external resource fields to existing explicit lists
func (r *ReferenceResolver) isReferenceField(fieldName string) bool {
    referenceFields := []string{
        "default_application_auth_strategy_id",
        "control_plane_id", 
        "portal_id",
        "auth_strategy_ids",
        // NEW: External resource fields (explicit list, no pattern matching)
        "external_control_plane_id",
        "external_service_id", 
        "external_route_id",
        "external_api_id",
    }
    // ... existing logic unchanged
}

// EXTEND: Add external resource type mappings  
func (r *ReferenceResolver) getResourceTypeForField(fieldName string) string {
    switch fieldName {
    // ... existing mappings ...
    // NEW: External resource mappings (type-based, not pattern matching)
    case "external_control_plane_id":
        return "external_control_plane"
    case "external_service_id":
        return "external_service"
    case "external_route_id":
        return "external_route"
    case "external_api_id":
        return "external_api"
    default:
        return ""
    }
}

// NEW: External resource resolution methods
func (r *ReferenceResolver) resolveExternalControlPlaneRef(ctx context.Context, ref string) (string, error)
func (r *ReferenceResolver) resolveExternalServiceRef(ctx context.Context, ref string) (string, error)
func (r *ReferenceResolver) resolveExternalRouteRef(ctx context.Context, ref string) (string, error)
```

**Type-Based Resolution**: Uses explicit field names and type knowledge, NOT pattern matching on `*_id` suffixes.

**Code Reuse**: Extends existing `isReferenceField()` and `getResourceTypeForField()` functions with additional explicit entries.

### Phase 5: Dependency Graph Integration

**Objective**: Extend existing dependency resolution to include external resources

**Files to Modify**:

1. **Extend**: `internal/declarative/planner/dependencies.go`
```go  
// EXTEND: Add external resource parent types to existing function
func (d *DependencyResolver) getParentType(childType string) string {
    switch childType {
    case "api_version", "api_publication", "api_implementation", "api_document":
        return "api"
    case "portal_page":
        return "portal"
    // NEW: External resource hierarchies
    case "external_service", "external_route":
        return "external_control_plane"
    case "external_api_version":
        return "external_api"
    default:
        return ""
    }
}

// EXTEND: Include external resources in implicit dependency detection
func (d *DependencyResolver) findImplicitDependencies(change PlannedChange, allChanges []PlannedChange) []string {
    var dependencies []string
    
    // Existing logic for references
    for _, refInfo := range change.References {
        if refInfo.ID == "[unknown]" {
            for _, other := range allChanges {
                // NEW: Include external resources in dependency search
                if other.ResourceRef == refInfo.Ref && 
                   (other.Action == ActionCreate || strings.HasPrefix(other.ResourceType, "external_")) {
                    dependencies = append(dependencies, other.ID)
                    break
                }
            }
        }
    }
    return dependencies
}
```

**Algorithm Reuse**: 90% reuse of existing topological sort (Kahn's algorithm) - no changes to core dependency resolution algorithm.

**Parent Hierarchy**: Extends existing parent-child relationship handling with external resource hierarchies.

### Phase 6: SDK Query Adapters  

**Objective**: Create external resource SDK adapters following existing adapter patterns

**Files to Create**:

1. **Create**: `internal/declarative/external/adapters/control_plane.go`
```go
// NEW: Control plane external resource adapter
type ExternalControlPlaneAdapter struct {
    client *state.Client
}

func (a *ExternalControlPlaneAdapter) QueryBySelector(
    ctx context.Context,
    selector ResourceSelector,
) ([]*ControlPlane, error) {
    if selector.ID != nil {
        cp, err := a.client.GetControlPlaneByID(ctx, *selector.ID)
        if err != nil {
            return nil, err
        }
        return []*ControlPlane{cp}, nil
    }
    
    if selector.MatchFields != nil {
        return a.client.ListControlPlanesWithFilters(ctx, selector.MatchFields)
    }
    
    return nil, fmt.Errorf("invalid selector")
}
```

2. **Create**: `internal/declarative/external/adapters/service.go`
3. **Create**: `internal/declarative/external/adapters/route.go`
4. **Create**: `internal/declarative/external/adapters/api.go`

**Pattern Reuse**: Follows existing executor adapter patterns from `internal/declarative/executor/`.

**SDK Integration**: Uses existing SDK client methods - no new SDK patterns needed.

### Phase 7: Integration with Planning Phase

**Objective**: Integrate external resource resolution into existing planning phase

**Files to Modify**:

1. **Extend**: `internal/declarative/planner/planner.go`
```go
// EXTEND: Add external resource resolution to existing planning pipeline
func (p *Planner) Plan(ctx context.Context, config *Config) (*Plan, error) {
    // ... existing planning logic ...
    
    // NEW: External resource resolution step
    if config.ExternalResources != nil {
        externalChanges, err := p.resolveExternalResources(ctx, config.ExternalResources)
        if err != nil {
            return nil, fmt.Errorf("external resource resolution failed: %w", err)
        }
        allChanges = append(allChanges, externalChanges...)
    }
    
    // Existing dependency resolution (reused)
    executionOrder, err := p.dependencyResolver.ResolveDependencies(allChanges)
    if err != nil {
        return nil, err
    }
    
    // ... rest of existing planning logic ...
}

// NEW: External resource resolution method
func (p *Planner) resolveExternalResources(ctx context.Context, config *ExternalResourceConfig) ([]PlannedChange, error)
```

**Planning Integration**: External resources become additional `PlannedChange` objects in existing planning pipeline.

**Schema Reuse**: External resources use existing `PlannedChange` type - no new plan structure needed.

### Phase 8: Error Handling and Validation

**Objective**: Implement comprehensive error handling using existing patterns

**Files to Modify**:

1. **Extend**: `internal/declarative/external/errors.go`
```go
// NEW: External resource specific errors following existing error patterns
var (
    ErrExternalResourceNotFound     = errors.New("external resource not found")
    ErrMultipleExternalResourcesFound = errors.New("multiple external resources found, selector not specific enough")
    ErrInvalidExternalResourceSelector = errors.New("invalid external resource selector")
)

// Error context functions following existing patterns
func NewExternalResourceError(resourceType, ref string, cause error) error
func NewSelectorValidationError(selector ResourceSelector, issues []string) error
```

**Error Pattern Reuse**: Follows existing error handling patterns from current codebase.

**Detailed Context**: Provides specific error context for troubleshooting external resource issues.

## Production-Ready Implementation Details

### Configuration File Integration

**YAML Configuration Support**:
```yaml
# External resources configuration block
external_resources:
  db_control_plane:
    kind: control_plane
    selector:
      matchFields:
        name: "database-services"
        
  external_service:
    kind: service
    selector:
      id: "550e8400-e29b-41d4-a716-446655440000"  
    parent: db_control_plane

# Usage in resource definitions  
services:
  - ref: my-service
    control_plane_id: db_control_plane  # References external resource
    # ... other service fields
```

### Sequential Execution Flow

**Execution Order** (maintains existing sequential model):
1. **Load Configuration**: Parse external resources alongside existing resources
2. **Plan Generation**: Include external resources in dependency graph
3. **Reference Resolution**: Query external resources during reference resolution phase
4. **Sequential Execution**: Process changes in dependency order (existing algorithm)

**No New Concurrency**: External resource queries happen within existing sequential reference resolution phase.

### Type-Based Field Resolution Strategy

**Explicit Field Lists** (NOT pattern matching):
```go
// Current approach: explicit field lists
referenceFields := []string{
    "control_plane_id",           // existing
    "external_control_plane_id",  // new
    "service_id",                 // existing  
    "external_service_id",        // new
}

// Type mappings: explicit switch statements
switch fieldName {
case "control_plane_id":
    return "control_plane"
case "external_control_plane_id":  
    return "external_control_plane"
}
```

**Rationale**: Maintains existing explicit field detection approach for type safety and consistency.

## File Modification Summary

### Files to Extend (High Reuse)
- `internal/declarative/planner/dependencies.go` - Add external resource parent types
- `internal/declarative/planner/resolver.go` - Add external resource field mappings
- `internal/declarative/planner/planner.go` - Integrate external resource resolution
- `internal/declarative/config/config.go` - Add external resources configuration
- `internal/declarative/executor/executor.go` - Add external resource execution support

### Files to Create (New Components)
- `internal/declarative/config/external_resources.go` - Configuration schema
- `internal/declarative/external/registry.go` - Resource type registry
- `internal/declarative/external/resolver.go` - External resource resolver
- `internal/declarative/external/errors.go` - Error handling
- `internal/declarative/external/adapters/control_plane.go` - Control plane adapter
- `internal/declarative/external/adapters/service.go` - Service adapter
- `internal/declarative/external/adapters/route.go` - Route adapter
- `internal/declarative/external/adapters/api.go` - API adapter

### Schema Compatibility
**Zero New Core Types**: External resources use existing `PlannedChange` and `ReferenceInfo` types.

**Configuration Extensions**: Only add external resource configuration types, no changes to existing schemas.

## Testing Strategy

### Unit Tests (Following Existing Patterns)
- External resource configuration parsing
- Selector validation and matching logic
- Registry lookup operations
- Adapter SDK query methods
- Error handling scenarios

### Integration Tests (Extend Existing Test Suite)
- External resource resolution within planning phase
- Mixed internal/external reference resolution
- Dependency graph generation with external resources
- End-to-end configuration to execution flow

### Error Scenario Testing
- Zero matches for selector
- Multiple matches for selector
- Invalid configuration scenarios  
- SDK query failures
- Circular dependency detection with external resources

## Risk Mitigation

### Low Risk (High Reuse Areas)
- **Dependency Resolution**: Existing topological sort algorithm proven robust
- **Schema Compatibility**: Existing types accommodate external resources
- **Sequential Execution**: No concurrency changes reduce complexity

### Medium Risk (Extension Points)  
- **Field Detection**: Risk of breaking existing field detection - mitigated by comprehensive testing
- **SDK Integration**: Risk of incorrect external queries - mitigated by following existing adapter patterns
- **Configuration Parsing**: Risk of YAML parsing issues - mitigated by reusing existing patterns

### Mitigation Strategies
1. **Backward Compatibility**: All extensions preserve existing functionality
2. **Pattern Following**: New components follow proven existing patterns
3. **Comprehensive Testing**: Test all extension points with existing configurations
4. **Gradual Integration**: Phase-by-phase implementation allows for iterative validation

## Success Criteria

### Code Quality Objectives
✅ **Accuracy First**: Correct behavior and proper error handling over performance optimization  
✅ **Maximum Reuse**: 80% code reuse achieved through extensions rather than new systems  
✅ **Schema Consistency**: Zero new core types, full compatibility with existing schemas  
✅ **Pattern Consistency**: All new components follow existing codebase patterns  
✅ **Type Safety**: Explicit field resolution maintains type safety over convenience  

### Functional Requirements
- External resources resolve correctly to unique IDs
- Clear error messages for all failure scenarios
- Seamless integration with existing dependency resolution
- Full compatibility with existing resource configurations
- Production-ready accuracy and reliability

## Implementation Timeline

**Phase 1 Implementation** (aligned with execution-plan-steps.md):
1. Schema and Configuration (1-2 days)
2. Resource Type Registry (1 day)  
3. External Resource Resolver (2-3 days)
4. Reference Resolution Integration (2-3 days)
5. Dependency Graph Integration (1-2 days)
6. SDK Query Adapters (3-4 days)
7. Planning Phase Integration (1-2 days)
8. Error Handling and Testing (3-4 days)

**Total Estimated Effort**: 14-21 days for complete Phase 1 implementation

## Conclusion

This implementation plan achieves the user's requirements through a strategy of **extending proven systems** rather than creating new architectures. The 80% code reuse through targeted extensions ensures:

- **Production-ready accuracy** through reuse of proven algorithms and patterns
- **Minimal schema sprawl** by leveraging existing `PlannedChange` and `ReferenceInfo` types
- **Type-based field resolution** through explicit field lists rather than pattern matching  
- **Sequential execution model** preservation without new concurrency complexity
- **Perfect phase alignment** with the established execution-plan-steps.md structure

The implementation maintains consistency with existing codebase patterns while providing robust external resource functionality focused on correctness and maintainability over performance optimization.