# External Resources Implementation Investigation Report

## Executive Summary

This investigation analyzed the current state of external resources implementation in 
kongctl, focusing on understanding what exists and what needs to be implemented for 
**Step 3: External Resource Resolver**. Steps 1 and 2 are fully completed with solid 
infrastructure in place. Step 3 is the next major implementation milestone.

## Current Implementation Status

### ✅ Completed Components (Steps 1-2)

#### Step 1: Schema and Configuration (COMPLETED)
- **File**: `/internal/declarative/resources/external_resource.go`
- **Status**: Fully implemented with comprehensive validation
- **Key Features**:
  - `ExternalResourceResource` struct with ID/selector XOR validation
  - `ExternalResourceSelector` with matchFields support
  - `ExternalResourceParent` for hierarchical resources
  - Runtime state methods (SetResolvedID, GetResolvedResource, etc.)
  - Complete validation framework with detailed error messages

#### Step 2: Resource Type Registry (COMPLETED)
- **Files**: `/internal/declarative/external/registry.go` and all adapters
- **Status**: All 13 adapters implemented and tested
- **Key Features**:
  - Registry with 13 resource types registered
  - Complete adapter factory with dependency injection
  - All adapters implement `ResolutionAdapter` interface
  - Parent-child relationship validation
  - Comprehensive test coverage

**Supported Resource Types**:
- Top-level: `portal`, `api`, `control_plane`, `application_auth_strategy`
- Core entities: `ce_service` (with control_plane parent)
- Portal children: `portal_customization`, `portal_custom_domain`, `portal_page`, `portal_snippet`
- API children: `api_version`, `api_implementation`, `api_document`, `api_publication`

#### Integration Points (COMPLETED)
- **ResourceSet**: External resources integrated in `/internal/declarative/resources/types.go`
- **Planner Validation**: Basic validation hook exists in planner
- **Adapter Infrastructure**: Base adapter with common filtering and SDK integration

### ❌ Missing Components (Step 3)

#### Step 3: External Resource Resolver (NOT IMPLEMENTED)
The core resolver component is completely missing. Based on the execution plan and 
architecture, here's what needs to be implemented:

## Step 3 Requirements Analysis

### ExternalResourceResolver Implementation

**Location**: `/internal/declarative/external/resolver.go` (new file)

**Core Responsibilities**:
1. Parse external_resources from ResourceSet
2. Build dependency graph for resolution order
3. Execute SDK queries via registry adapters
4. Validate exactly one match per selector
5. Store resolved resources in memory
6. Provide resolved IDs for planning phase

**Key Methods Needed**:
```go
type ExternalResourceResolver struct {
    registry *ResolutionRegistry
    client   *state.Client
    logger   *slog.Logger
    resolved map[string]*ResolvedExternalResource
}

// Core resolution methods
func (r *ExternalResourceResolver) ResolveExternalResources(ctx context.Context, externalResources []resources.ExternalResourceResource) error
func (r *ExternalResourceResolver) buildDependencyGraph(resources []resources.ExternalResourceResource) (*DependencyGraph, error)
func (r *ExternalResourceResolver) resolveResource(ctx context.Context, resource *resources.ExternalResourceResource) error
func (r *ExternalResourceResolver) GetResolvedResource(ref string) (*ResolvedExternalResource, bool)
```

### Integration with Planner

**File**: `/internal/declarative/planner/planner.go`
**Changes Needed**:
1. Add ExternalResourceResolver field to Planner struct
2. Integrate external resolution into `resolveResourceIdentities` method
3. Call external resolution BEFORE other resource planning
4. Update reference resolver to use external resource IDs

**Current Integration Point**:
```go
// Line 86: Pre-resolution phase
if err := p.resolveResourceIdentities(ctx, rs); err != nil {
    return nil, fmt.Errorf("failed to resolve resource identities: %w", err)
}
```

### Dependency Resolution

**New Component**: Dependency graph builder for external resources
**Purpose**: Ensure parent resources are resolved before children
**Example**: `control_plane` must be resolved before `ce_service`

### Error Handling

**Requirements from Architecture**:
- Clear messages for zero matches
- Clear messages for multiple matches  
- SDK error propagation with context
- Validation errors with field-specific details

**Example Error Format**:
```
Error: External resource 'prod-cp' selector matched 0 resources
  Resource type: control_plane
  Selector: matchFields: {name: "production-cp"}
  Suggestion: Verify the resource exists in Konnect
```

## Technical Architecture Analysis

### Existing Infrastructure That Can Be Leveraged

#### 1. Adapter System (READY)
- **Location**: `/internal/declarative/external/adapters/`
- **Status**: All 13 adapters implemented
- **Usage**: Resolver can directly use registry.GetResolutionAdapter()

#### 2. State Client Integration (READY)
- **Location**: All adapters use `*state.Client` from constructor
- **Status**: SDK integration completed in adapters
- **Usage**: Resolver uses same client instance

#### 3. Validation Framework (READY)
- **Location**: `/internal/declarative/resources/external_resource.go`
- **Status**: Complete XOR validation, parent validation, field validation
- **Usage**: Resolver calls existing Validate() methods

#### 4. Reference Resolution Pattern (REFERENCE)
- **Location**: `/internal/declarative/planner/resolver.go`
- **Status**: Existing pattern for internal references
- **Usage**: External resolver should follow similar pattern

### Key Files That Need Creation

#### 1. External Resource Resolver (NEW)
```
/internal/declarative/external/resolver.go
/internal/declarative/external/resolver_test.go
```

#### 2. Dependency Graph (NEW)
```
/internal/declarative/external/dependencies.go
/internal/declarative/external/dependencies_test.go
```

#### 3. Resolution Types (EXTEND)
```
/internal/declarative/external/types.go (add resolver types)
```

### Key Files That Need Modification

#### 1. Planner Integration (MODIFY)
```
/internal/declarative/planner/planner.go
- Add ExternalResourceResolver field
- Integrate into resolveResourceIdentities()
- Update constructor
```

#### 2. Reference Resolution (MODIFY)
```  
/internal/declarative/planner/resolver.go
- Add external resource ID resolution
- Update field mapping for external IDs
```

## Implementation Sequence for Step 3

### Phase 1: Core Resolver
1. Create ExternalResourceResolver struct
2. Implement basic resolution flow
3. Add dependency graph building
4. Integrate with existing adapters

### Phase 2: Planner Integration  
1. Add resolver to Planner struct
2. Call external resolution in resolveResourceIdentities
3. Update reference resolution for external IDs

### Phase 3: Error Handling & Validation
1. Implement detailed error messages
2. Add match validation (exactly one)
3. Handle SDK errors gracefully

### Phase 4: Testing
1. Unit tests for resolver components
2. Integration tests with mock adapters
3. End-to-end tests with planning

## Risk Assessment

### Low Risk
- **Adapter Integration**: All adapters ready and tested
- **Schema Validation**: Complete validation framework exists
- **Registry System**: Solid foundation with metadata

### Medium Risk
- **Dependency Resolution**: Complex parent-child relationships need careful ordering
- **Performance**: Multiple SDK calls need optimization/caching
- **Error Messages**: Need comprehensive coverage of failure scenarios

### High Risk
- **Planning Integration**: Must not break existing planning flow
- **Reference Resolution**: Complex interaction between internal and external refs

## Dependencies and Prerequisites

### Internal Dependencies (READY)
- ✅ External resource schema and validation
- ✅ Resolution registry and adapters
- ✅ State client and SDK integration
- ✅ Base planner infrastructure

### External Dependencies (READY)
- ✅ Konnect SDK integration
- ✅ Authentication and API client setup
- ✅ Error handling patterns

## Success Criteria for Step 3

1. **Functionality**
   - External resources resolve correctly via SDK
   - Parent-child dependencies handled properly
   - Resolved IDs available for planning phase

2. **Error Handling**
   - Clear messages for all failure scenarios
   - No silent failures or incorrect behavior
   - Helpful troubleshooting guidance

3. **Integration**
   - Seamless integration with existing planner
   - No breaking changes to current functionality
   - Consistent behavior across resource types

4. **Performance**
   - Efficient resolution with minimal SDK calls
   - Proper caching during planning/execution cycle
   - Fast resolution for direct ID references

## Recommended Next Steps

1. **Start with Core Resolver**: Implement ExternalResourceResolver struct
2. **Build Dependency Graph**: Create dependency resolution system
3. **Integrate with Planner**: Add to existing resolution flow
4. **Comprehensive Testing**: Unit + integration tests
5. **Error Handling**: Implement detailed error messages
6. **Performance Optimization**: Add caching and batch operations

The foundation is solid and well-architected. Step 3 implementation should be 
straightforward given the quality of the existing infrastructure.