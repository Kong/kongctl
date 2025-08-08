# Implementation Plan: Step 4 - Reference Resolution Integration

**Date**: August 8, 2025  
**Task**: Step 4 of Stage 8 (External Resources) - Reference Resolution Integration  
**Objective**: Integrate external resource resolution with dynamic reference field detection

## Executive Summary

This plan implements Step 4 of the External Resources feature, focusing on replacing hardcoded reference field detection with dynamic resource mapping-based resolution. The foundation is solid with external resource resolution already integrated, but the reference resolver currently uses hardcoded field names instead of leveraging the existing `GetReferenceFieldMappings()` interface that each resource provides.

**Key Changes**:
- Replace hardcoded field detection in `internal/declarative/planner/resolver.go` with dynamic resource mapping queries
- Audit and complete resource field mappings in `internal/declarative/resources/*.go`
- Add enhanced validation for reference types and mixed reference scenarios
- Implement comprehensive testing for all reference resolution scenarios

## Implementation Phases

### Phase 1: Core Integration (Priority: High)
**Estimated Time**: 2-3 hours  
**Files Modified**: 1 primary file  
**Risk Level**: Medium (replacing core logic)

### Phase 2: Resource Mapping Audit (Priority: Medium) 
**Estimated Time**: 1-2 hours  
**Files Modified**: Multiple resource files  
**Risk Level**: Low (adding/completing mappings)

### Phase 3: Enhanced Validation (Priority: Medium)
**Estimated Time**: 1-2 hours  
**Files Modified**: Primarily resolver.go  
**Risk Level**: Low (additional validation)

### Phase 4: Comprehensive Testing (Priority: High)
**Estimated Time**: 2-3 hours  
**Files Modified**: Test files  
**Risk Level**: Low (testing only)

---

## Phase 1: Core Integration

### 1.1 Add Resource Registry Access to ReferenceResolver

**File**: `internal/declarative/planner/resolver.go`

**Current Constructor**:
```go
func NewReferenceResolver(externalResolver *external.ResourceResolver) *ReferenceResolver {
    return &ReferenceResolver{
        externalResolver: externalResolver,
    }
}
```

**Required Change**: Add resource registry dependency
```go
// Add to ReferenceResolver struct
type ReferenceResolver struct {
    externalResolver *external.ResourceResolver
    resourceRegistry *resources.Registry  // NEW: Add registry access
    mappingCache     map[string]map[string]string  // NEW: Cache for mappings
    cacheMutex       sync.RWMutex  // NEW: Cache synchronization
}

// Update constructor
func NewReferenceResolver(externalResolver *external.ResourceResolver, registry *resources.Registry) *ReferenceResolver {
    return &ReferenceResolver{
        externalResolver: externalResolver,
        resourceRegistry: registry,
        mappingCache:     make(map[string]map[string]string),
    }
}
```

### 1.2 Implement Dynamic Resource Mapping Retrieval

**File**: `internal/declarative/planner/resolver.go`

**Add New Method**:
```go
// getResourceMappings retrieves reference field mappings for a resource type
func (r *ReferenceResolver) getResourceMappings(resourceType string) map[string]string {
    // Check cache first
    r.cacheMutex.RLock()
    if mappings, exists := r.mappingCache[resourceType]; exists {
        r.cacheMutex.RUnlock()
        return mappings
    }
    r.cacheMutex.RUnlock()

    // Get resource instance from registry
    resource, err := r.resourceRegistry.GetResource(resourceType)
    if err != nil {
        // Log error but don't fail - fallback to empty mappings
        slog.Warn("Failed to get resource for mappings", "resource_type", resourceType, "error", err)
        return make(map[string]string)
    }

    // Check if resource implements reference mappings
    if mapper, ok := resource.(resources.ReferenceMapper); ok {
        mappings := mapper.GetReferenceFieldMappings()
        
        // Cache the result
        r.cacheMutex.Lock()
        r.mappingCache[resourceType] = mappings
        r.cacheMutex.Unlock()
        
        return mappings
    }

    // No mappings available
    return make(map[string]string)
}
```

### 1.3 Replace Hardcoded Field Detection

**File**: `internal/declarative/planner/resolver.go`

**Current Method**:
```go
func isReferenceField(fieldName string) bool {
    referenceFields := []string{
        "default_application_auth_strategy_id",
        "control_plane_id",
        "portal_id",
        "auth_strategy_ids",
    }
    // ... hardcoded check
}
```

**Replace With**:
```go
// isReferenceFieldDynamic checks if a field is a reference field using resource mappings
func (r *ReferenceResolver) isReferenceFieldDynamic(resourceType, fieldName string) bool {
    mappings := r.getResourceMappings(resourceType)
    _, exists := mappings[fieldName]
    return exists
}

// Keep backward compatibility method for now
func isReferenceField(fieldName string) bool {
    // Fallback to hardcoded for backward compatibility during transition
    referenceFields := []string{
        "default_application_auth_strategy_id",
        "control_plane_id",
        "portal_id", 
        "auth_strategy_ids",
    }
    
    for _, field := range referenceFields {
        if field == fieldName {
            return true
        }
    }
    return false
}
```

### 1.4 Replace Hardcoded Resource Type Mapping

**File**: `internal/declarative/planner/resolver.go`

**Current Method**:
```go
func getResourceTypeForField(fieldName string) string {
    switch fieldName {
    case "default_application_auth_strategy_id", "auth_strategy_ids":
        return "application_auth_strategy"
    case "control_plane_id", "gateway_service.control_plane_id":
        return "control_plane"
    // ... more hardcoded cases
    }
    return ""
}
```

**Replace With**:
```go
// getResourceTypeForFieldDynamic gets resource type using dynamic mappings
func (r *ReferenceResolver) getResourceTypeForFieldDynamic(resourceType, fieldName string) string {
    mappings := r.getResourceMappings(resourceType)
    if targetType, exists := mappings[fieldName]; exists {
        return targetType
    }
    return ""
}

// Keep backward compatibility method for now
func getResourceTypeForField(fieldName string) string {
    // Fallback to hardcoded for backward compatibility during transition
    switch fieldName {
    case "default_application_auth_strategy_id", "auth_strategy_ids":
        return "application_auth_strategy"
    case "control_plane_id", "gateway_service.control_plane_id":
        return "control_plane"
    case "portal_id":
        return "portal"
    default:
        return ""
    }
}
```

### 1.5 Update Main Resolution Methods

**File**: `internal/declarative/planner/resolver.go`

**Update `ResolveReferences` Method**:
```go
func (r *ReferenceResolver) ResolveReferences(ctx context.Context, changes []PlannedChange) *ResolveResult {
    result := &ResolveResult{
        ChangeReferences: make(map[string]map[string]ResolvedReference),
        Errors:           []error{},
    }

    for _, change := range changes {
        changeRefs := make(map[string]ResolvedReference)
        
        // NEW: Get resource mappings for this change's resource type
        resourceMappings := r.getResourceMappings(change.ResourceType)
        
        for _, fieldChange := range change.Changes {
            fieldName := fieldChange.Field
            
            // NEW: Use dynamic field detection
            if r.isReferenceFieldDynamic(change.ResourceType, fieldName) {
                // Extract and resolve reference
                ref := r.extractReference(fieldName, fieldChange.Value)
                if ref != "" && !isUUID(ref) {
                    // NEW: Use dynamic resource type lookup
                    resourceType := r.getResourceTypeForFieldDynamic(change.ResourceType, fieldName)
                    if resourceType == "" {
                        result.Errors = append(result.Errors, 
                            fmt.Errorf("no resource type mapping for field %s in resource %s", 
                                fieldName, change.ResourceType))
                        continue
                    }
                    
                    resolvedID, err := r.resolveReference(ctx, resourceType, ref)
                    if err != nil {
                        result.Errors = append(result.Errors, 
                            fmt.Errorf("failed to resolve reference %s for field %s: %w", 
                                ref, fieldName, err))
                        continue
                    }
                    
                    changeRefs[fieldName] = ResolvedReference{
                        OriginalValue: ref,
                        ResolvedID:    resolvedID,
                        ResourceType:  resourceType,
                    }
                }
            }
        }
        
        if len(changeRefs) > 0 {
            result.ChangeReferences[change.ID] = changeRefs
        }
    }
    
    return result
}
```

### 1.6 Update Planner to Pass Resource Registry

**File**: `internal/declarative/planner/planner.go`

**Current Resolver Creation** (around line 50):
```go
resolver := NewReferenceResolver(p.externalResolver)
```

**Update To**:
```go
resolver := NewReferenceResolver(p.externalResolver, p.resourceRegistry)
```

**Note**: Ensure `p.resourceRegistry` is available in planner struct. If not available, it needs to be added to the planner constructor.

---

## Phase 2: Resource Mapping Audit

### 2.1 Audit Current Resource Mappings

**Objective**: Ensure all `_id` fields that should be references are properly mapped.

**Process**:
1. List all resource files in `internal/declarative/resources/`
2. For each resource, compare hardcoded fields with existing mappings
3. Add missing mappings for any `_id` fields that contain references

### 2.2 Resource-by-Resource Audit

#### 2.2.1 Portal Resource
**File**: `internal/declarative/resources/portal.go`

**Current Mapping**:
```go
func (p PortalResource) GetReferenceFieldMappings() map[string]string {
    return map[string]string{
        "default_application_auth_strategy_id": "application_auth_strategy",
    }
}
```

**Status**: ✅ Complete - covers known portal reference fields

#### 2.2.2 API Publication Resource
**File**: `internal/declarative/resources/api_publication.go`

**Current Mapping**:
```go
func (p APIPublicationResource) GetReferenceFieldMappings() map[string]string {
    return map[string]string{
        "portal_id":         "portal",
        "auth_strategy_ids": "application_auth_strategy",
    }
}
```

**Status**: ✅ Complete - covers known API publication reference fields

#### 2.2.3 Check Other Resource Files

**Files to Audit**:
- `internal/declarative/resources/api.go`
- `internal/declarative/resources/api_implementation.go`
- `internal/declarative/resources/application_auth_strategy.go`
- `internal/declarative/resources/control_plane.go`
- Any other resource files with `_id` fields

**Look For**:
- Fields ending in `_id` that might contain references (non-UUID values)
- Fields mentioned in current hardcoded list but not in mappings
- Nested field paths like `service.control_plane_id`

**Example Missing Mapping** (if found in API Implementation):
```go
func (a APIImplementationResource) GetReferenceFieldMappings() map[string]string {
    return map[string]string{
        "service.control_plane_id": "control_plane",
        // Add any other missing mappings
    }
}
```

### 2.3 Handle Nested Field Paths

**Current Hardcoded Paths**:
- `"gateway_service.control_plane_id"`
- `"service.control_plane_id"`

**Implementation**: Ensure these nested paths are included in resource mappings and that the field extraction logic handles dot notation correctly.

**Example Resource Update**:
```go
func (r ResourceWithNestedFields) GetReferenceFieldMappings() map[string]string {
    return map[string]string{
        "control_plane_id":         "control_plane",
        "service.control_plane_id": "control_plane",
        "gateway_service.control_plane_id": "control_plane",
    }
}
```

---

## Phase 3: Enhanced Validation

### 3.1 Add Reference Type Validation

**File**: `internal/declarative/planner/resolver.go`

**Add Method**:
```go
// validateReferenceType validates that resolved reference matches expected type
func (r *ReferenceResolver) validateReferenceType(resolvedID, expectedType, actualType string) error {
    if expectedType != actualType {
        return fmt.Errorf("reference type mismatch: expected %s, got %s for ID %s", 
            expectedType, actualType, resolvedID)
    }
    return nil
}
```

**Update `resolveReference` Method**:
```go
func (r *ReferenceResolver) resolveReference(ctx context.Context, resourceType, ref string) (string, error) {
    // Check external resources first
    if r.externalResolver != nil {
        if resolvedID, found := r.externalResolver.GetResolvedID(ref); found {
            // NEW: Validate external resource type if available
            if resolved := r.externalResolver.GetResolvedResource(ref); resolved != nil {
                if err := r.validateReferenceType(resolvedID, resourceType, resolved.ResourceType); err != nil {
                    return "", fmt.Errorf("external resource validation failed: %w", err)
                }
            }
            return resolvedID, nil
        }
    }
    
    // Fall back to internal resolution
    // ... existing internal resolution logic
}
```

### 3.2 Handle Array Fields

**File**: `internal/declarative/planner/resolver.go`

**Update `extractReference` Method**:
```go
func (r *ReferenceResolver) extractReference(fieldName string, value interface{}) []string {
    // Handle both single values and arrays
    switch v := value.(type) {
    case string:
        if v != "" && !isUUID(v) {
            return []string{v}
        }
    case []interface{}:
        var refs []string
        for _, item := range v {
            if str, ok := item.(string); ok && str != "" && !isUUID(str) {
                refs = append(refs, str)
            }
        }
        return refs
    case []string:
        var refs []string
        for _, str := range v {
            if str != "" && !isUUID(str) {
                refs = append(refs, str)
            }
        }
        return refs
    }
    return nil
}
```

**Update Resolution Logic** to handle multiple references per field:
```go
// In ResolveReferences method, update field processing
for _, fieldChange := range change.Changes {
    fieldName := fieldChange.Field
    
    if r.isReferenceFieldDynamic(change.ResourceType, fieldName) {
        refs := r.extractReference(fieldName, fieldChange.Value)
        if len(refs) > 0 {
            resourceType := r.getResourceTypeForFieldDynamic(change.ResourceType, fieldName)
            
            var resolvedIDs []string
            for _, ref := range refs {
                resolvedID, err := r.resolveReference(ctx, resourceType, ref)
                if err != nil {
                    result.Errors = append(result.Errors, err)
                    continue
                }
                resolvedIDs = append(resolvedIDs, resolvedID)
            }
            
            // Handle single vs array in ResolvedReference
            changeRefs[fieldName] = ResolvedReference{
                OriginalValue: refs,
                ResolvedIDs:   resolvedIDs,
                ResourceType:  resourceType,
            }
        }
    }
}
```

### 3.3 Update ResolvedReference Structure

**File**: `internal/declarative/planner/types.go`

**Current Structure**:
```go
type ResolvedReference struct {
    OriginalValue string
    ResolvedID    string
    ResourceType  string
}
```

**Enhanced Structure**:
```go
type ResolvedReference struct {
    OriginalValue interface{} // Can be string or []string
    ResolvedID    string      // For single values
    ResolvedIDs   []string    // For array values
    ResourceType  string
    IsArray       bool        // Indicates if this was an array field
}
```

---

## Phase 4: Comprehensive Testing

### 4.1 Unit Tests for Dynamic Resolution

**File**: `internal/declarative/planner/resolver_test.go`

**Test Cases to Add**:

```go
func TestDynamicFieldDetection(t *testing.T) {
    // Test that resource mappings are queried correctly
    // Test that fields are detected dynamically, not hardcoded
}

func TestMixedReferenceResolution(t *testing.T) {
    // Test scenarios with both external and internal references
    // Verify external resources are checked first
}

func TestNestedFieldPaths(t *testing.T) {
    // Test fields like "service.control_plane_id"
    // Verify nested path extraction works
}

func TestArrayFieldResolution(t *testing.T) {
    // Test fields like "auth_strategy_ids" with multiple values
    // Verify array handling and multiple resolution
}

func TestResourceTypeValidation(t *testing.T) {
    // Test that resolved references match expected types
    // Test error cases for type mismatches
}

func TestMappingCaching(t *testing.T) {
    // Test that resource mappings are cached correctly
    // Test cache invalidation if needed
}

func TestBackwardCompatibility(t *testing.T) {
    // Test that existing configurations still work
    // Test fallback to hardcoded approach when needed
}
```

### 4.2 Integration Tests

**File**: `test/integration/external_resources_test.go`

**Test Scenarios**:

```go
func TestExternalInternalReferenceIntegration(t *testing.T) {
    // End-to-end test with real external and internal resources
    // Test complex dependency scenarios
}

func TestComplexReferenceScenarios(t *testing.T) {
    // Test multiple resource types with interdependencies
    // Test nested references and array fields together
}

func TestPerformanceComparison(t *testing.T) {
    // Compare performance of dynamic vs hardcoded approach
    // Ensure no significant performance degradation
}
```

### 4.3 Test Data Setup

**Files**: Test configuration files with mixed reference scenarios

**Example Test Data**:
```yaml
# test/testdata/mixed_references.yaml
portals:
  - ref: "test-portal"
    name: "Test Portal"
    default_application_auth_strategy_id: "external-auth-strategy"  # External reference

api_publications:
  - ref: "test-publication"
    portal_id: "test-portal"  # Internal reference
    auth_strategy_ids:        # Array of mixed references
      - "external-auth-strategy"  # External
      - "internal-auth-strategy"  # Internal
```

### 4.4 Error Handling Tests

**Test Error Scenarios**:
- Missing resource mappings
- Invalid resource types
- Reference resolution failures
- Type validation failures
- Array field edge cases

---

## Edge Cases and Error Handling

### 1. Resource Without Mappings
**Scenario**: Resource doesn't implement `GetReferenceFieldMappings()`
**Handling**: Log warning, return empty mappings, continue processing

### 2. Invalid Field Paths
**Scenario**: Mapping includes field path that doesn't exist in resource
**Handling**: Skip field, log warning, continue processing

### 3. Mixed Array Types
**Scenario**: Array field contains mix of UUIDs and references
**Handling**: Process only non-UUID values as references

### 4. Performance Considerations
**Scenario**: Repeated mapping queries impact performance
**Handling**: Cache mappings per resource type with thread-safe access

### 5. Registry Access Failures
**Scenario**: Resource registry doesn't have requested resource type
**Handling**: Fallback to hardcoded approach, log warning

---

## Validation and Success Criteria

### Functional Requirements Checklist
- [ ] All `_id` fields automatically detected without hardcoding
- [ ] External and internal references resolve seamlessly
- [ ] Reference type validation works correctly  
- [ ] Nested field paths (e.g., `service.control_plane_id`) supported
- [ ] Array fields (e.g., `auth_strategy_ids`) handled properly
- [ ] Resource mapping caching functions correctly
- [ ] Error messages are clear and actionable

### Quality Requirements Checklist
- [ ] Zero regression in existing reference resolution
- [ ] Performance comparable to hardcoded approach
- [ ] Comprehensive error handling for all failure modes
- [ ] Full test coverage for all scenarios
- [ ] Backward compatibility maintained during transition
- [ ] Clear logging for debugging and monitoring

### Integration Tests Pass
- [ ] Build: `make build` succeeds
- [ ] Lint: `make lint` passes with zero issues
- [ ] Tests: `make test` passes all unit tests
- [ ] Integration: `make test-integration` passes all integration tests

---

## Implementation Timeline

### Day 1: Core Integration (Phase 1)
- **Hours 1-2**: Add resource registry access to ReferenceResolver
- **Hours 3-4**: Implement dynamic mapping retrieval and caching
- **Hours 5-6**: Replace hardcoded field detection methods
- **Hours 7-8**: Update main resolution methods

### Day 2: Resource Audit and Validation (Phases 2-3)
- **Hours 1-2**: Audit all resource files for missing mappings
- **Hours 3-4**: Add missing reference field mappings
- **Hours 5-6**: Implement enhanced validation (type checking, arrays)
- **Hours 7-8**: Update data structures for array support

### Day 3: Testing and Validation (Phase 4)
- **Hours 1-4**: Write comprehensive unit tests
- **Hours 5-6**: Create integration test scenarios  
- **Hours 7-8**: Test error handling and edge cases

### Quality Gates
After each phase:
1. **Build Check**: `make build` must succeed
2. **Lint Check**: `make lint` must pass with zero issues
3. **Test Check**: `make test` must pass all tests
4. **Integration Check**: `make test-integration` when applicable

---

## Risk Mitigation

### High-Risk Changes
1. **Replacing core resolution logic** - Implement incrementally with fallbacks
2. **Performance impact from dynamic queries** - Use caching and benchmarking
3. **Breaking existing configurations** - Maintain backward compatibility

### Risk Mitigation Strategies
1. **Incremental Implementation**: Keep hardcoded methods as fallbacks initially
2. **Extensive Testing**: Cover all existing and new scenarios thoroughly
3. **Performance Monitoring**: Benchmark before/after changes
4. **Clear Error Messages**: Provide actionable feedback for failures
5. **Rollback Plan**: Keep ability to disable dynamic resolution if issues arise

### Monitoring and Observability
- Add trace-level logging for dynamic resolution decisions
- Log cache hit/miss rates for performance monitoring
- Log fallback usage to track transition progress
- Monitor error rates for resolution failures

---

## Post-Implementation Tasks

### 1. Documentation Updates
- Update developer documentation for reference resolution
- Document new dynamic mapping approach
- Update troubleshooting guides

### 2. Performance Optimization
- Monitor cache effectiveness
- Consider pre-warming cache for common resource types
- Optimize mapping query performance if needed

### 3. Future Enhancements
- Consider configuration-based field mapping overrides
- Implement mapping validation at startup
- Add metrics for resolution performance

---

## Conclusion

This implementation plan provides a comprehensive, step-by-step approach to implementing Step 4: Reference Resolution Integration. The plan leverages the existing solid architecture while replacing hardcoded field detection with dynamic resource mapping queries.

**Key Benefits**:
- Eliminates hardcoded field maintenance burden
- Leverages existing resource mapping infrastructure
- Maintains backward compatibility during transition
- Provides enhanced validation and error handling
- Supports complex scenarios (arrays, nested paths, mixed references)

**Implementation is ready to begin** - all dependencies are in place and the approach is well-defined with clear success criteria and risk mitigation strategies.