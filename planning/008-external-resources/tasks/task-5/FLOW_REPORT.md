# Flow Analysis Report: External Resources Reference Resolution

**Generated**: August 8, 2025  
**Task**: Step 4 of Stage 8 (External Resources) - Reference Resolution Integration  
**Focus**: Code flow analysis for implementing dynamic reference resolution with external resource integration

## Executive Summary

This report traces the complete execution flow for reference resolution in the Kong Control CLI (kongctl), identifying how external resource resolution integrates with the existing reference resolution system. The analysis reveals a well-architected foundation with clear integration points, but identifies areas where hardcoded field detection should be replaced with dynamic resource mapping-based approaches.

## Complete Execution Flow Diagram

```
┌─────────────────┐    ┌──────────────────────┐    ┌─────────────────┐
│  Configuration  │    │   External Resource  │    │   Reference     │
│     Loading     │───▶│     Resolution       │───▶│   Resolution    │
└─────────────────┘    └──────────────────────┘    └─────────────────┘
                                │                            │
                                ▼                            ▼
┌─────────────────┐    ┌──────────────────────┐    ┌─────────────────┐
│  Dependency     │    │   Plan Generation    │    │   Validation    │
│   Ordering      │◀───│  & Change Creation   │◀───│  & Errors       │
└─────────────────┘    └──────────────────────┘    └─────────────────┘
        │
        ▼
┌─────────────────┐
│   Execution     │
└─────────────────┘
```

## 1. Configuration Loading → Reference Detection Flow

### Entry Point: Plan Generation
**File**: `/internal/declarative/planner/planner.go`  
**Function**: `GeneratePlan(ctx, rs, opts)` (Line 88)

```go
// Pre-resolution phase: Resolve resource identities before planning
if err := p.resolveResourceIdentities(ctx, rs); err != nil {
    return nil, fmt.Errorf("failed to resolve resource identities: %w", err)
}
```

### Resource Identity Resolution Flow
**File**: `/internal/declarative/planner/planner.go`  
**Function**: `resolveResourceIdentities(ctx, rs)` (Line 397)

**Flow Steps**:
1. **External Resources First** (Line 405):
   ```go
   if err := p.externalResolver.ResolveExternalResources(ctx, externalResources); err != nil {
       return fmt.Errorf("failed to resolve external resources: %w", err)
   }
   ```

2. **Internal Resource Resolution** (Line 410):
   ```go
   if err := p.resolveAPIIdentities(ctx, rs.APIs); err != nil {
       return fmt.Errorf("failed to resolve API identities: %w", err)
   }
   ```

## 2. External Resource Resolution Workflow

### Main Resolution Flow
**File**: `/internal/declarative/external/resolver.go`  
**Function**: `ResolveExternalResources(ctx, externalResources)` (Line 34)

**Execution Steps**:

#### Step 1: Dependency Graph Construction
**Function**: `buildDependencyGraph(externalResources)` (Line 46)
- **File**: `/internal/declarative/external/dependencies.go`
- **Algorithm**: Two-pass dependency graph building
  - Pass 1: Create nodes for all resources (Line 17)
  - Pass 2: Build parent-child relationships (Line 41)
- **Validation**: Parent-child relationship validation via registry (Line 54)

#### Step 2: Topological Sorting
**Function**: `topologicalSort(graph)` (Line 66)
- **Algorithm**: Kahn's algorithm for cycle detection
- **Output**: `ResolutionOrder []string` - dependency-ordered resource list
- **Error Handling**: Circular dependency detection (Line 135)

#### Step 3: Sequential Resource Resolution
**Function**: `resolveResource(ctx, resource)` (Line 67)
- **Parent Resolution**: Handles parent context preparation (Line 82)
- **Dual Resolution Modes**:
  - **Direct ID** (Line 123): `adapter.GetByID(ctx, *id, parentResource)`
  - **Selector-based** (Line 130): `adapter.GetBySelector(ctx, selector, parentResource)`
- **Caching**: Stores resolved resources in `resolved` map (Line 169)

### Data Transformations

#### Resource → ResolvedResource
**Location**: `resolveResource()` (Line 153)
```go
resolvedResource := &ResolvedResource{
    ID:           resolvedID,           // Extracted from API response
    Resource:     resolved,             // Full SDK response object
    ResourceType: resource.GetResourceType(),
    Ref:          resource.GetRef(),    // Original config reference
    ResolvedAt:   time.Now(),
}
```

#### Resolution Cache Structure
**Type**: `map[string]*ResolvedResource`
- **Key**: Resource reference from configuration
- **Value**: Complete resolved resource data
- **Access**: Via `GetResolvedID(ref)` method

## 3. Reference Resolution Integration Points

### Current Reference Resolution Flow
**File**: `/internal/declarative/planner/resolver.go`  
**Function**: `ResolveReferences(ctx, changes)` (Line 43)

**Integration Architecture**:
```go
// External resources checked first
if r.externalResolver != nil {
    if resolvedID, found := r.externalResolver.GetResolvedID(ref); found {
        return resolvedID, nil
    }
}
// Fall back to internal resolution
```

### Field Detection - Current Implementation (Hardcoded)
**Function**: `isReferenceField(fieldName)` (Line 125)
```go
referenceFields := []string{
    "default_application_auth_strategy_id",
    "control_plane_id",
    "portal_id", 
    "auth_strategy_ids",
}
```

### Resource Type Mapping - Current Implementation (Hardcoded)
**Function**: `getResourceTypeForField(fieldName)` (Line 147)
```go
switch fieldName {
case "default_application_auth_strategy_id", "auth_strategy_ids":
    return "application_auth_strategy"
case "control_plane_id", "gateway_service.control_plane_id":
    return "control_plane"
// ... hardcoded mapping
}
```

## 4. Resource Reference Field Mappings (Unused Potential)

### Dynamic Mapping Interface
**File**: Various `/internal/declarative/resources/*.go` files
**Method**: `GetReferenceFieldMappings() map[string]string`

### Examples:

#### Portal Resource Mappings
**File**: `/internal/declarative/resources/portal.go` (Line 88)
```go
func (p PortalResource) GetReferenceFieldMappings() map[string]string {
    return map[string]string{
        "default_application_auth_strategy_id": "application_auth_strategy",
    }
}
```

#### API Publication Resource Mappings  
**File**: `/internal/declarative/resources/api_publication.go` (Line 51)
```go
func (p APIPublicationResource) GetReferenceFieldMappings() map[string]string {
    return map[string]string{
        "portal_id":         "portal",
        "auth_strategy_ids": "application_auth_strategy",
    }
}
```

**Current Status**: These mappings exist but are NOT used by the reference resolver.

## 5. Dependency Resolution Workflow

### External Resource Dependencies
**System**: Sophisticated parent-child dependency graph
**Features**:
- Topological sorting with cycle detection
- Parent-child relationship validation
- Deterministic ordering for reproducible builds
- Comprehensive error reporting

### Internal Resource Dependencies  
**File**: `/internal/declarative/planner/dependencies.go`  
**System**: Basic change-level dependency tracking
**Features**:
- Explicit `DependsOn` relationships
- Implicit dependency detection based on references
- Simple topological sorting

### Integration Point
Both systems operate independently:
1. External resources resolved first (with their own dependency ordering)
2. Internal changes processed later (with reference resolution using cached external data)

## 6. Error Propagation Chains

### External Resource Resolution Errors
**Origin**: `/internal/declarative/external/resolver.go`
**Propagation Path**:
```
resolveResource() error
    ↓
ResolveExternalResources() error  
    ↓
resolveResourceIdentities() error
    ↓
GeneratePlan() error
```

### Reference Resolution Errors
**Origin**: `/internal/declarative/planner/resolver.go`
**Error Collection**:
```go
result := &ResolveResult{
    ChangeReferences: make(map[string]map[string]ResolvedReference),
    Errors:           []error{},  // Accumulated errors
}
```

### Error Types:
1. **External Resource Not Found**: Zero matches for selector
2. **Multiple Matches**: Ambiguous selector results  
3. **Parent Dependency Failures**: Parent resource not resolved
4. **Reference Type Mismatches**: Field expects different resource type
5. **Circular Dependencies**: Detected during topological sort

## 7. Key Integration Points for Step 4 Implementation

### 1. Dynamic Field Detection Integration
**Target File**: `/internal/declarative/planner/resolver.go`
**Current**: Hardcoded `isReferenceField()` and `getResourceTypeForField()`
**Required Change**: 
```go
// Replace with dynamic approach
func (r *ReferenceResolver) getResourceMappings(resourceType string) map[string]string {
    // Get resource instance and query GetReferenceFieldMappings()
}
```

### 2. Planned Change Resource Type Access
**Current Flow**: `ResolveReferences(changes []PlannedChange)`
**Required**: Access to resource type to query field mappings
**Implementation**: `change.ResourceType` already available in `PlannedChange`

### 3. Field Path Resolution
**Complexity**: Some mappings use nested paths like `"service.control_plane_id"`
**Current Support**: `FieldChange` objects already handle nested field extraction
**Required**: Extend path matching logic in `extractReference()`

### 4. UUID vs Reference Detection
**Current Implementation**: Works correctly
```go
func isUUID(s string) bool {
    return len(s) == 36 && s[8] == '-' && s[13] == '-' && s[18] == '-' && s[23] == '-'
}
```
**Status**: No changes needed

## 8. Data Flow Transformations

### Configuration → Planned Changes
**Input**: YAML/JSON configuration with string references
**Output**: `PlannedChange` objects with `FieldChange` data
**Location**: Plan generation process

### Reference Extraction
**Input**: `FieldChange` objects with nested field data
**Output**: Extracted string references for resolution
**Function**: `extractReference(fieldName, value)`

### Resolution Results
**Input**: String references
**Output**: `ResolvedReference` objects with UUIDs
**Storage**: Organized by change ID and field name
```go
ChangeReferences map[string]map[string]ResolvedReference
```

## 9. Architecture Strengths

### 1. Clean Separation of Concerns
- External resolution: Handles Konnect API integration
- Internal resolution: Handles declarative resource cross-references  
- Integration: Well-defined interface via `GetResolvedID()`

### 2. Sophisticated Dependency Management
- External: Full dependency graph with parent-child relationships
- Internal: Change-level dependency tracking
- Error handling: Comprehensive cycle detection and validation

### 3. Interface-Based Design
- Resource mappings: Each resource defines its reference fields
- Resolution adapters: Pluggable SDK integration
- Caching: Efficient resolved resource storage

### 4. Error Recovery
- Accumulated error collection rather than fail-fast
- Detailed error context with resource types and references
- Clear error propagation chains

## 10. Step 4 Implementation Strategy

### Phase 1: Core Integration (High Priority)
**File**: `/internal/declarative/planner/resolver.go`

1. **Replace `isReferenceField()`**:
   ```go
   // OLD: Hardcoded field list
   // NEW: Dynamic query from resource mappings
   func (r *ReferenceResolver) isReferenceField(resourceType, fieldName string) bool
   ```

2. **Replace `getResourceTypeForField()`**:
   ```go
   // OLD: Switch statement
   // NEW: Lookup in resource mappings
   func (r *ReferenceResolver) getResourceTypeForField(resourceType, fieldName string) string
   ```

3. **Enhance `extractReference()`**:
   - Accept resource type parameter
   - Query resource mappings dynamically
   - Support nested field paths

### Phase 2: Resource Mapping Audit (Medium Priority)
**Files**: `/internal/declarative/resources/*.go`

1. **Audit Existing Mappings**: Ensure all `_id` fields are covered
2. **Add Missing Mappings**: Any reference fields not currently mapped  
3. **Validate Accuracy**: Field paths match actual resource structure

### Phase 3: Enhanced Validation (Low Priority)
1. **Type Validation**: Resolved references match expected resource types
2. **Mixed Reference Validation**: Handle external + internal references
3. **Array Field Support**: Handle `auth_strategy_ids` and similar arrays

## 11. Risk Assessment

### Low Risk
- **External resolver integration**: Already working correctly
- **UUID detection**: Current implementation is robust
- **Error propagation**: Well-established patterns

### Medium Risk  
- **Dynamic field detection**: Replacing hardcoded logic requires careful testing
- **Nested field paths**: Complex field access patterns need validation
- **Resource mapping completeness**: May discover missing mappings

### High Risk
- **Array field handling**: Current system may not handle arrays correctly
- **Performance impact**: Dynamic mapping queries could be slower
- **Backward compatibility**: Changes might break existing reference resolution

## 12. Success Metrics

### Functional Requirements
- [ ] All `_id` fields automatically detected without hardcoding
- [ ] External + internal references resolve seamlessly  
- [ ] Reference type validation works correctly
- [ ] Nested field paths (e.g., `service.control_plane_id`) supported
- [ ] Array fields (e.g., `auth_strategy_ids`) handled properly

### Quality Requirements
- [ ] Zero regression in existing reference resolution
- [ ] Performance comparable to current hardcoded approach
- [ ] Comprehensive error messages for resolution failures
- [ ] Full test coverage for mixed reference scenarios

## 13. Next Steps

1. **Immediate**: Implement dynamic field detection in `resolver.go`
2. **Short-term**: Audit and complete resource field mappings
3. **Medium-term**: Add comprehensive validation and array support  
4. **Long-term**: Consider caching resource mappings for performance

## Conclusion

The external resources reference resolution integration has a solid architectural foundation with clear separation of concerns and well-defined integration points. The primary implementation work involves replacing hardcoded field detection with dynamic resource mapping queries, leveraging the existing `GetReferenceFieldMappings()` interface that is currently underutilized.

The external resource resolver is sophisticated and production-ready, with comprehensive dependency management and error handling. The integration points are clean and the data flow is well-established. Step 4 implementation primarily involves enhancing the existing reference resolver to be more dynamic and comprehensive while maintaining backward compatibility.