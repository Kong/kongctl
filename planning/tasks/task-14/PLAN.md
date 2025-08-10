# Solution Plan: Global Ref Uniqueness Implementation

## Executive Summary

This plan provides a comprehensive solution to fix the ref field uniqueness bug in the 
kongctl declarative configuration system. The current implementation allows duplicate 
ref values across different resource types, violating the intended global uniqueness 
requirement. This plan outlines a phased approach to implement global ref uniqueness 
while maintaining backward compatibility and minimizing risk.

## Problem Statement

### Current Issue
- Refs are only unique within each resource type due to separate tracking maps
- A portal with `ref: "common"` and an API with `ref: "common"` both pass validation
- This creates potential ambiguity in cross-resource references and dependency resolution
- Violates the intended design where refs should be globally unique identifiers

### Root Cause Analysis
The system uses a consistent **per-resource-type isolation pattern** at every layer:
1. **Loading:** Separate tracking maps (`portalRefs`, `apiRefs`, etc.)
2. **Validation:** Separate registry maps (`registry["portal"]`, `registry["api"]`)
3. **Resolution:** Separate created resource maps (`createdResources[resourceType][ref]`)
4. **Dependencies:** Ref matching without type consideration

## Solution Architecture

### Core Design Change
Transform from **per-resource-type ref tracking** to **global ref tracking with type metadata**.

### Key Principles
1. **Global Uniqueness:** No two resources can have the same ref value
2. **Type Awareness:** System maintains knowledge of which resource type owns each ref
3. **Backward Compatibility:** Existing valid configurations continue to work
4. **Clear Error Messages:** Users receive helpful feedback about ref conflicts

## Phase 1: Foundation Changes (Low Risk)

### 1.1 Create Global Ref Registry Data Structure

**File:** `/internal/declarative/resources/types.go`

Add new global ref tracking structure:

```go
// GlobalRefRegistry tracks refs across all resource types
type GlobalRefRegistry struct {
    refs     map[string]string // ref -> resource_type
    mutex    sync.RWMutex      // For thread safety if needed
}

func NewGlobalRefRegistry() *GlobalRefRegistry {
    return &GlobalRefRegistry{
        refs: make(map[string]string),
    }
}

func (g *GlobalRefRegistry) AddRef(ref, resourceType string) error {
    g.mutex.Lock()
    defer g.mutex.Unlock()
    
    if existingType, exists := g.refs[ref]; exists {
        return fmt.Errorf("duplicate ref '%s': already used by %s resource, cannot use for %s resource", 
            ref, existingType, resourceType)
    }
    
    g.refs[ref] = resourceType
    return nil
}

func (g *GlobalRefRegistry) HasRef(ref string) bool {
    g.mutex.RLock()
    defer g.mutex.RUnlock()
    _, exists := g.refs[ref]
    return exists
}

func (g *GlobalRefRegistry) GetResourceType(ref string) (string, bool) {
    g.mutex.RLock()
    defer g.mutex.RUnlock()
    resourceType, exists := g.refs[ref]
    return resourceType, exists
}

func (g *GlobalRefRegistry) GetAllRefs() map[string]string {
    g.mutex.RLock()
    defer g.mutex.RUnlock()
    
    result := make(map[string]string)
    for ref, resourceType := range g.refs {
        result[ref] = resourceType
    }
    return result
}
```

**Risk Level:** Low - New code, no existing functionality affected  
**Testing:** Unit tests for registry operations

### 1.2 Update Loader for Global Ref Tracking

**File:** `/internal/declarative/loader/loader.go`

**Current Lines 58-74:** Replace separate tracking maps
```go
// REMOVE: Separate per-type maps
// portalRefs := make(map[string]string)
// authStratRefs := make(map[string]string)
// cpRefs := make(map[string]string)
// apiRefs := make(map[string]string)

// ADD: Global ref tracking
globalRefRegistry := resources.NewGlobalRefRegistry()
```

**Lines 354-395:** Update duplicate checking logic
```go
// REPLACE existing per-type duplicate checking with:
func checkForDuplicateRefs(resourceSet *resources.ResourceSet, sourceMap map[string]string) error {
    globalRefRegistry := resources.NewGlobalRefRegistry()
    
    // Check portals
    for _, portal := range resourceSet.Portals {
        if err := globalRefRegistry.AddRef(portal.GetRef(), "portal"); err != nil {
            source := sourceMap[portal.GetRef()]
            return fmt.Errorf("in file %s: %w", source, err)
        }
    }
    
    // Check auth strategies
    for _, authStrat := range resourceSet.ApplicationAuthStrategies {
        if err := globalRefRegistry.AddRef(authStrat.GetRef(), "application_auth_strategy"); err != nil {
            source := sourceMap[authStrat.GetRef()]
            return fmt.Errorf("in file %s: %w", source, err)
        }
    }
    
    // Check control planes
    for _, cp := range resourceSet.ControlPlanes {
        if err := globalRefRegistry.AddRef(cp.GetRef(), "control_plane"); err != nil {
            source := sourceMap[cp.GetRef()]
            return fmt.Errorf("in file %s: %w", source, err)
        }
    }
    
    // Check APIs
    for _, api := range resourceSet.APIs {
        if err := globalRefRegistry.AddRef(api.GetRef(), "api"); err != nil {
            source := sourceMap[api.GetRef()]
            return fmt.Errorf("in file %s: %w", source, err)
        }
    }
    
    // Continue for all other resource types...
    
    return nil
}
```

**Risk Level:** Low - Self-contained change with clear error boundaries  
**Testing:** Update existing loader tests to expect global uniqueness errors

### 1.3 Update Validator for Global Ref Registry

**File:** `/internal/declarative/loader/validator.go`

**Lines 12-16:** Replace resource registry creation
```go
// REPLACE:
// resourceRegistry := make(map[string]map[string]bool)

// WITH:
globalRefRegistry := resources.NewGlobalRefRegistry()
```

**Lines 56-240:** Update all validation functions to use global registry

**New validation pattern for each resource type:**
```go
func validatePortals(portals []resources.PortalResource, globalRefRegistry *resources.GlobalRefRegistry) error {
    for _, portal := range portals {
        if err := globalRefRegistry.AddRef(portal.GetRef(), "portal"); err != nil {
            return err
        }
        
        // Continue with existing portal-specific validation...
    }
    return nil
}

func validateAuthStrategies(strategies []resources.ApplicationAuthStrategyResource, globalRefRegistry *resources.GlobalRefRegistry) error {
    for _, strategy := range strategies {
        if err := globalRefRegistry.AddRef(strategy.GetRef(), "application_auth_strategy"); err != nil {
            return err
        }
        
        // Continue with existing auth strategy validation...
    }
    return nil
}

// Similar pattern for validateControlPlanes, validateAPIs, etc.
```

**Lines 242-285:** Update cross-reference validation
```go
func validateCrossReferences(resourceSet *resources.ResourceSet, globalRefRegistry *resources.GlobalRefRegistry) error {
    // Existing cross-reference validation logic remains the same
    // The globalRefRegistry.HasRef() can be used for reference existence checks
    
    for fieldPath, expectedType := range mappings {
        resourceType, exists := globalRefRegistry.GetResourceType(fieldValue)
        if !exists {
            return fmt.Errorf("resource %q references unknown resource: %s", 
                refResource.GetRef(), fieldValue)
        }
        if resourceType != expectedType {
            return fmt.Errorf("resource %q field %s references %s resource %q, expected %s resource", 
                refResource.GetRef(), fieldPath, resourceType, fieldValue, expectedType)
        }
    }
    
    return nil
}
```

**Risk Level:** Low - Validation functions are self-contained  
**Testing:** Update validation tests to expect global uniqueness errors

## Phase 2: Core Resolution Changes (Medium Risk)

### 2.1 Update Reference Resolution for Global Awareness

**File:** `/internal/declarative/planner/resolver.go`

**Lines 43-52:** Enhance created resource tracking
```go
// CURRENT:
// createdResources := make(map[string]map[string]string) // resource_type → ref → change_id

// ENHANCED:
type CreatedResourceRegistry struct {
    byType map[string]map[string]string // resource_type → ref → change_id (for compatibility)
    byRef  map[string]string            // ref → change_id (for global lookup)
    refTypes map[string]string          // ref → resource_type
}

func newCreatedResourceRegistry() *CreatedResourceRegistry {
    return &CreatedResourceRegistry{
        byType:   make(map[string]map[string]string),
        byRef:    make(map[string]string),
        refTypes: make(map[string]string),
    }
}

func (c *CreatedResourceRegistry) AddResource(resourceType, ref, changeID string) error {
    // Check for global conflicts
    if existingType, exists := c.refTypes[ref]; exists {
        return fmt.Errorf("ref conflict: %s already used by %s resource, cannot use for %s", 
            ref, existingType, resourceType)
    }
    
    // Add to type-specific map (preserve existing behavior)
    if c.byType[resourceType] == nil {
        c.byType[resourceType] = make(map[string]string)
    }
    c.byType[resourceType][ref] = changeID
    
    // Add to global maps
    c.byRef[ref] = changeID
    c.refTypes[ref] = resourceType
    
    return nil
}
```

**Lines 62-83:** Update reference resolution logic
```go
func (r *ReferenceResolver) ResolveReferences(ctx context.Context, changes []PlannedChange) error {
    createdRegistry := newCreatedResourceRegistry()
    
    // Build registry of resources being created
    for _, change := range changes {
        if change.Action == ActionCreate {
            if err := createdRegistry.AddResource(change.ResourceType, change.ResourceRef, change.ID); err != nil {
                return fmt.Errorf("reference conflict in planned changes: %w", err)
            }
        }
    }
    
    // Resolve references (existing logic with enhanced registry)
    for i := range changes {
        for fieldName, ref := range changes[i].References {
            expectedType := r.getResourceTypeForField(fieldName)
            
            // Check if this references something being created
            if changeID, exists := createdRegistry.byRef[ref]; exists {
                actualType := createdRegistry.refTypes[ref]
                if actualType != expectedType {
                    return fmt.Errorf("field %s expects %s resource, but ref %q is a %s resource", 
                        fieldName, expectedType, ref, actualType)
                }
                changes[i].References[fieldName] = ReferenceInfo{
                    Ref: ref,
                    ID:  changeID,
                }
            } else {
                // Resolve from existing resources
                id, err := r.resolveReference(ctx, expectedType, ref)
                if err != nil {
                    return fmt.Errorf("failed to resolve %s reference %q: %w", expectedType, ref, err)
                }
                changes[i].References[fieldName] = ReferenceInfo{
                    Ref: ref,
                    ID:  id,
                }
            }
        }
    }
    
    return nil
}
```

**Risk Level:** Medium - Affects planning pipeline  
**Testing:** Comprehensive tests for cross-type ref conflicts in resolution

### 2.2 Update Dependency Resolution

**File:** `/internal/declarative/planner/dependencies.go`

**Lines 101-118:** Enhance dependency matching
```go
func (d *DependencyResolver) findImplicitDependencies(change PlannedChange, allChanges []PlannedChange) []string {
    var dependencies []string
    
    for _, refInfo := range change.References {
        if refInfo.ID == "[unknown]" {
            // Find the change that creates this resource
            for _, other := range allChanges {
                // ENHANCED: Match by both ref and type compatibility
                if other.ResourceRef == refInfo.Ref && other.Action == ActionCreate {
                    // Verify this is the correct resource type for the field
                    expectedType := getResourceTypeForReference(refInfo.Ref, change.ResourceType)
                    if other.ResourceType == expectedType {
                        dependencies = append(dependencies, other.ID)
                        break
                    }
                }
            }
        } else {
            dependencies = append(dependencies, refInfo.ID)
        }
    }
    
    return dependencies
}

// New helper function for type validation in dependencies
func getResourceTypeForReference(ref, sourcResourceType string) string {
    // This would need to be implemented based on field mappings
    // Could use the existing getResourceTypeForField logic
    return "" // Implementation needed
}
```

**Risk Level:** Medium - Affects dependency graph construction  
**Testing:** Tests for correct dependency resolution with global refs

## Phase 3: Comprehensive Testing (Low Risk)

### 3.1 Create Comprehensive Test Suite

**File:** `/internal/declarative/loader/validator_test.go`

Add test cases for global ref uniqueness:

```go
func TestGlobalRefUniqueness(t *testing.T) {
    tests := []struct {
        name        string
        input       string
        expectError bool
        errorMsg    string
    }{
        {
            name: "duplicate ref across resource types should fail",
            input: `
portals:
  - ref: common
    name: "My Portal"
apis:
  - ref: common
    name: "My API"
`,
            expectError: true,
            errorMsg: "duplicate ref 'common': already used by portal resource, cannot use for api resource",
        },
        {
            name: "duplicate ref within same type should fail",
            input: `
portals:
  - ref: common
    name: "Portal 1"
  - ref: common
    name: "Portal 2"
`,
            expectError: true,
            errorMsg: "duplicate ref 'common': already used by portal resource, cannot use for portal resource",
        },
        {
            name: "unique refs across types should pass",
            input: `
portals:
  - ref: my-portal
    name: "My Portal"
apis:
  - ref: my-api
    name: "My API"
control_planes:
  - ref: my-cp
    name: "My Control Plane"
`,
            expectError: false,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            resourceSet, err := parseYAMLContent([]byte(tt.input))
            require.NoError(t, err)
            
            err = validateResourceSet(resourceSet)
            
            if tt.expectError {
                require.Error(t, err)
                require.Contains(t, err.Error(), tt.errorMsg)
            } else {
                require.NoError(t, err)
            }
        })
    }
}
```

### 3.2 Create Integration Tests

**File:** `/test/integration/ref_uniqueness_test.go`

```go
//go:build integration

func TestGlobalRefUniquenessIntegration(t *testing.T) {
    // Test end-to-end pipeline with global ref conflicts
    tempDir := t.TempDir()
    
    // Create test configuration with duplicate refs
    configContent := `
portals:
  - ref: shared-ref
    name: "Test Portal"
    
apis:
  - ref: shared-ref
    name: "Test API"
`
    
    configFile := filepath.Join(tempDir, "config.yaml")
    err := os.WriteFile(configFile, []byte(configContent), 0644)
    require.NoError(t, err)
    
    // Load and validate - should fail
    _, err = loader.LoadFromSources([]string{configFile})
    require.Error(t, err)
    require.Contains(t, err.Error(), "duplicate ref 'shared-ref'")
}
```

### 3.3 Backward Compatibility Tests

**File:** `/internal/declarative/loader/compatibility_test.go`

```go
func TestBackwardCompatibility(t *testing.T) {
    // Test that existing valid configurations still work
    existingValidConfigs := []string{
        "testdata/valid-minimal.yaml",
        "testdata/valid-complex.yaml",
        "testdata/cross-references.yaml",
    }
    
    for _, configFile := range existingValidConfigs {
        t.Run(filepath.Base(configFile), func(t *testing.T) {
            resourceSet, err := loader.LoadFromSources([]string{configFile})
            require.NoError(t, err)
            require.NotNil(t, resourceSet)
            
            // Verify all refs are still unique globally
            refMap := make(map[string]string)
            // Check all resource types for ref uniqueness...
        })
    }
}
```

**Risk Level:** Low - Testing only  
**Impact:** Validates solution correctness

## Phase 4: Error Handling and User Experience (Low Risk)

### 4.1 Enhanced Error Messages

Update error messages across all components to be more helpful:

```go
// In GlobalRefRegistry.AddRef()
return fmt.Errorf("duplicate ref '%s': already used by %s resource (defined in %s), cannot use for %s resource", 
    ref, existingType, existingSource, resourceType)

// In validation
return fmt.Errorf("configuration error: ref '%s' is used by both %s and %s resources. Each ref must be unique across all resource types", 
    ref, existingType, newType)
```

### 4.2 Documentation Updates

**File:** `/docs/configuration-reference.md` (if it exists)

Add section on ref uniqueness:
```markdown
## Resource References (ref field)

The `ref` field serves as a unique identifier for resources within your configuration:

- **Global Uniqueness**: Each `ref` value must be unique across ALL resource types
- **Cross-References**: Use `ref` values to reference resources from other resources
- **Validation**: Duplicate `ref` values will cause validation errors

### Examples

✅ **Valid - unique refs across types:**
```yaml
portals:
  - ref: my-portal
    name: "Customer Portal"

apis:
  - ref: my-api
    name: "Customer API"
```

❌ **Invalid - duplicate ref across types:**
```yaml
portals:
  - ref: common
    name: "Portal"

apis:
  - ref: common  # Error: duplicate ref
    name: "API"
```
```

**Risk Level:** Low - Documentation only  
**Impact:** Improves user understanding

## Implementation Strategy

### Phase Ordering Rationale

1. **Phase 1 (Foundation)**: Low-risk infrastructure changes that don't affect existing functionality
2. **Phase 2 (Core Changes)**: Medium-risk changes to resolution logic with comprehensive testing
3. **Phase 3 (Testing)**: Validation of all changes with comprehensive test coverage
4. **Phase 4 (Polish)**: User experience improvements

### Risk Mitigation

#### Low-Risk Changes
- New data structures (GlobalRefRegistry) - no existing code affected
- Enhanced error messages - only affect error cases
- Documentation updates - no functional impact

#### Medium-Risk Changes  
- Reference resolution updates - affects planning pipeline
- Dependency resolution changes - affects execution order

**Mitigation Strategies:**
1. Comprehensive unit tests for each modified function
2. Integration tests for end-to-end workflows
3. Backward compatibility tests with existing configurations
4. Staged rollout with feature flags if needed

### Rollout Strategy

#### Pre-Deployment
1. Run full test suite including new global uniqueness tests
2. Validate against existing configuration corpus
3. Performance testing to ensure no regression

#### Deployment
1. Deploy to internal/staging environment first
2. Validate with known good configurations
3. Monitor for any unexpected validation failures
4. Deploy to production with monitoring

#### Post-Deployment
1. Monitor error logs for validation failures
2. Track user feedback on error message clarity
3. Document any edge cases discovered

## Testing Strategy

### Unit Tests
- `GlobalRefRegistry` operations (Add, Get, HasRef)
- Individual validation functions
- Reference resolution with conflicts
- Dependency resolution correctness

### Integration Tests
- End-to-end configuration loading with ref conflicts
- Cross-resource reference resolution
- Planning pipeline with global refs
- State resolution with unique refs

### Regression Tests
- All existing valid configurations must still pass
- All existing invalid configurations must still fail (with potentially different error messages)
- Performance benchmarks for large configurations

### Edge Case Tests
- Empty ref values (should be handled by existing validation)
- Special characters in refs
- Very long ref values
- Maximum number of resources with unique refs

## Success Criteria

### Functional Requirements ✅
- [ ] No duplicate refs allowed across any resource types
- [ ] Clear, actionable error messages for ref conflicts
- [ ] All existing valid configurations continue to work
- [ ] Cross-resource references work correctly with global uniqueness
- [ ] Dependency resolution works correctly

### Non-Functional Requirements ✅
- [ ] No performance regression in configuration loading
- [ ] Memory usage remains acceptable for large configurations
- [ ] Error messages are clear and actionable
- [ ] Backward compatibility maintained

### Quality Gates ✅
- [ ] `make build` succeeds
- [ ] `make lint` passes with zero issues
- [ ] `make test` passes all tests
- [ ] `make test-integration` passes
- [ ] New test coverage >= 90% for modified code
- [ ] No breaking changes to public APIs

## Complexity and Risk Assessment

### Overall Complexity: **Medium**
- **Data Structure Changes**: Low complexity (new registry design)
- **Validation Logic**: Low complexity (straightforward replacement)
- **Resolution Logic**: Medium complexity (affects planning pipeline)
- **Testing Requirements**: Medium complexity (comprehensive coverage needed)

### Risk Assessment by Component

| Component | Risk Level | Mitigation |
|-----------|------------|------------|
| GlobalRefRegistry | Low | New code, comprehensive unit tests |
| Loader updates | Low | Self-contained, clear error boundaries |
| Validator updates | Low | Existing validation pattern, enhanced registry |
| Reference resolver | Medium | Core planning component, extensive testing |
| Dependency resolver | Medium | Affects execution order, careful validation |
| Cross-reference validation | Low | Enhancement of existing logic |

### Critical Path Dependencies
1. GlobalRefRegistry must be implemented first
2. Loader and Validator can be updated in parallel
3. Reference resolver depends on registry implementation
4. Dependency resolver depends on reference resolver changes
5. Testing can be developed in parallel with implementation

## Implementation Timeline

### Phase 1: Foundation (1-2 days)
- Implement GlobalRefRegistry
- Update loader for global tracking
- Update validator for global registry
- Basic unit tests

### Phase 2: Resolution (2-3 days)  
- Update reference resolution logic
- Update dependency resolution
- Integration with planning pipeline
- Comprehensive testing

### Phase 3: Testing (1-2 days)
- Comprehensive test suite
- Integration tests
- Backward compatibility validation
- Performance testing

### Phase 4: Polish (1 day)
- Enhanced error messages
- Documentation updates
- Final validation

**Total Estimated Timeline: 5-8 days**

## Conclusion

This plan provides a systematic approach to fixing the ref field uniqueness bug while maintaining backward compatibility and minimizing risk. The phased approach allows for incremental validation and testing, ensuring system stability throughout the implementation process.

The solution transforms the system from per-resource-type ref isolation to global ref uniqueness while preserving all existing functionality. The comprehensive testing strategy ensures that the fix works correctly and doesn't introduce regressions.

Key benefits of this approach:
- **Global Uniqueness**: Enforces the intended ref uniqueness across all resource types
- **Clear Errors**: Users receive helpful feedback about ref conflicts
- **Backward Compatibility**: Existing valid configurations continue to work
- **Future-Proof**: Enables cross-resource referencing features
- **Risk Management**: Phased implementation with comprehensive testing

The implementation follows kongctl's established patterns and maintains consistency with existing code organization and error handling approaches.
