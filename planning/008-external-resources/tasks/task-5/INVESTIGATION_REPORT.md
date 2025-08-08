# Investigation Report: Step 4 - Reference Resolution Integration

**Investigation Date**: August 8, 2025  
**Task**: Step 4 of Stage 8 (External Resources) - Reference Resolution Integration  
**Status**: Investigation Complete

## Executive Summary

Step 4 requires integrating external resource resolution with the existing reference resolution system to handle mixed internal/external references and implement implicit ID resolution for _id fields. The investigation reveals that the integration foundation is already partially in place, but the current system uses hardcoded field names instead of leveraging the resource-specific reference field mappings.

## Current State Analysis

### 1. External Resource Resolver (Step 3 - Completed)

**Location**: `internal/declarative/external/resolver.go`

The external resource resolver is fully implemented with:
- **ResourceResolver struct**: Main resolver with registry, state client, and resolved resource cache
- **Dependency graph resolution**: Topological sorting for proper resolution order
- **Dual resolution modes**: Direct ID and selector-based matching
- **Caching mechanism**: Resolved resources stored in `map[string]*ResolvedResource`
- **Integration ready**: Already integrated into planner workflow

Key methods:
- `ResolveExternalResources()`: Main resolution entry point
- `GetResolvedID()`: Retrieves resolved ID for a reference
- `HasResolvedResource()`: Checks if reference is resolved

### 2. Current Reference Resolution System

**Location**: `internal/declarative/planner/resolver.go`

Current limitations:
- **Hardcoded field detection**: `isReferenceField()` uses hardcoded field names
- **Limited field coverage**: Only handles specific known fields
- **Manual mapping**: `getResourceTypeForField()` manually maps field names to resource types

```go
// Current hardcoded approach
referenceFields := []string{
    "default_application_auth_strategy_id",
    "control_plane_id", 
    "portal_id",
    "auth_strategy_ids",
}
```

**Existing Integration**: The resolver already checks external resources first:
```go
func (r *ReferenceResolver) resolveReference(ctx context.Context, resourceType, ref string) (string, error) {
    // Check external resources first
    if r.externalResolver != nil {
        if resolvedID, found := r.externalResolver.GetResolvedID(ref); found {
            return resolvedID, nil
        }
    }
    // Fall back to internal resolution
    // ...
}
```

### 3. Resource Reference Field Mappings

**Location**: `internal/declarative/resources/`

Each resource implements `GetReferenceFieldMappings()` which returns field-to-resource-type mappings:

```go
// Example: Portal references
func (p PortalResource) GetReferenceFieldMappings() map[string]string {
    return map[string]string{
        "default_application_auth_strategy_id": "application_auth_strategy",
    }
}

// Example: API Publication references  
func (p APIPublicationResource) GetReferenceFieldMappings() map[string]string {
    return map[string]string{
        "portal_id":         "portal",
        "auth_strategy_ids": "application_auth_strategy",
    }
}
```

**Current Usage**: These mappings are used by the loader for validation but NOT by the planner for reference resolution.

### 4. Planning Integration

**Location**: `internal/declarative/planner/planner.go`

External resources are resolved early in the planning process:
```go
// Line 405: External resources resolved before plan generation
if err := p.externalResolver.ResolveExternalResources(ctx, externalResources); err != nil {
    return fmt.Errorf("failed to resolve external resources: %w", err)
}
```

## Step 4 Requirements Analysis

Based on `execution-plan-steps.md`, Step 4 needs to:

1. **ReferenceResolver for dependency handling** ✅ - Already exists
2. **Detect external resource references in configurations** ✅ - Integration point exists  
3. **Implement implicit ID resolution for _id fields** ❌ - Needs implementation
4. **Handle mixed internal/external references** ✅ - Framework exists
5. **Add reference validation** ✅ - Basic validation exists

## Key Integration Points for Step 4

### 1. Replace Hardcoded Field Detection

**Current Problem**: `isReferenceField()` uses hardcoded field names
**Solution**: Use resource-specific `GetReferenceFieldMappings()` 

**Files to Modify**:
- `internal/declarative/planner/resolver.go`: Replace hardcoded field detection
- Integration with resource mappings from planned changes

### 2. Implement Dynamic Reference Detection

**Requirement**: "Implicit ID resolution for _id fields"
**Implementation**: Use resource mappings to automatically detect all _id fields that contain references (non-UUIDs)

**Algorithm**:
1. Get resource type from planned change
2. Query resource for reference field mappings
3. Check each mapped field in change data
4. Resolve non-UUID values as references

### 3. Enhanced Reference Validation

**Current**: Basic existence checking
**Needed**: Validate reference types match expected resource types from mappings

## Technical Implementation Plan

### Phase 1: Core Integration

**File**: `internal/declarative/planner/resolver.go`

1. **Modify `extractReference()`**: Remove hardcoded field list, accept field mappings as parameter
2. **Update `ResolveReferences()`**: Get resource mappings for each planned change
3. **Replace `isReferenceField()`**: Use dynamic mapping-based detection
4. **Update `getResourceTypeForField()`**: Use mappings instead of hardcoded switch

### Phase 2: Resource Integration

**Files**: Various resource files in `internal/declarative/resources/`

1. **Audit existing mappings**: Ensure all _id fields are covered
2. **Add missing mappings**: Any _id fields not currently mapped
3. **Validate mapping accuracy**: Ensure field paths match actual resource structure

### Phase 3: Validation Enhancement

1. **Type validation**: Ensure resolved references match expected types
2. **Error messaging**: Clear errors for type mismatches
3. **Mixed reference handling**: Validate combination of external and internal references

## Files Requiring Modification

### Primary Files

1. **`internal/declarative/planner/resolver.go`**
   - Replace hardcoded field detection with dynamic mapping-based approach
   - Enhance `extractReference()` and `ResolveReferences()` methods
   - Remove hardcoded `isReferenceField()` and `getResourceTypeForField()`

2. **`internal/declarative/resources/*.go`** (Various resource files)
   - Audit and complete `GetReferenceFieldMappings()` implementations
   - Ensure all _id fields are properly mapped

### Secondary Files

3. **`internal/declarative/planner/types.go`**
   - May need interface updates for resource mapping integration

4. **`internal/declarative/resources/interfaces.go`**
   - May need to formalize `ReferenceMapping` interface usage

## Current Architecture Strengths

1. **Clean separation**: External and internal resolution are well separated
2. **Interface-based design**: Resource mapping interface exists and is used
3. **Integration foundation**: External resolver already integrated in planner
4. **Caching mechanism**: Resolved resources are cached effectively
5. **Dependency handling**: External resource dependency graph handles complex scenarios

## Edge Cases and Considerations

### 1. Field Path Complexity

Some mappings use nested field paths like `"service.control_plane_id"`:
```go
// API Implementation mapping
"service.control_plane_id": "control_plane"
```

**Solution**: Current `extractReference()` method already handles `FieldChange` objects, extend for nested paths.

### 2. UUID vs Reference Detection

Current system uses UUID pattern matching:
```go
func isUUID(s string) bool {
    return len(s) == 36 && s[8] == '-' && s[13] == '-' && s[18] == '-' && s[23] == '-'
}
```

**Requirement**: Non-UUID strings in _id fields should be treated as references
**Current State**: This logic exists and works correctly

### 3. Mixed Internal/External References

Resources might reference both internal (declarative) and external (Konnect) resources.

**Current State**: System already handles this by checking external resolver first, then falling back to internal resolution.

### 4. Array Fields

Some fields contain arrays of references:
```go
"auth_strategy_ids": "application_auth_strategy"  // Array of IDs
```

**Consideration**: Implementation needs to handle array fields containing references.

## Success Criteria for Step 4

1. **Dynamic field detection**: All _id fields automatically detected via resource mappings
2. **External integration**: External resources resolved seamlessly with internal ones  
3. **Validation**: References validated against expected types
4. **Backward compatibility**: Existing hardcoded field resolution continues to work
5. **Error handling**: Clear error messages for resolution failures

## Risk Mitigation

### Risk: Breaking existing reference resolution
**Mitigation**: Implement changes incrementally, keep fallback to hardcoded approach

### Risk: Performance impact from dynamic mapping
**Mitigation**: Cache resource mappings per resource type

### Risk: Complex field paths
**Mitigation**: Extend existing field extraction logic gradually

## Dependencies for Implementation

1. **External Resource Registry**: ✅ Complete (Step 2)
2. **External Resource Resolver**: ✅ Complete (Step 3)  
3. **Resource mapping interfaces**: ✅ Already implemented
4. **Planner integration**: ✅ Foundation exists

## Recommendation

Step 4 is ready for implementation. The foundation is solid, and the changes required are primarily in the reference resolution logic to make it more dynamic and comprehensive. The key insight is leveraging existing resource mapping infrastructure instead of hardcoded field detection.

**Implementation Priority**: 
1. Start with `resolver.go` modifications
2. Audit and complete resource mappings  
3. Add comprehensive tests for mixed reference scenarios
4. Validate edge cases with nested fields and arrays

The external resource integration is architecturally sound and ready to be fully utilized through improved reference resolution.