# External Resources Step 5 Error Handling - Investigation Report

**Investigation Date**: 2025-08-08  
**Current Branch**: feat/008-external-resources  
**Current Progress**: 4/8 steps completed (50%) - Step 4 completed with dynamic reference resolution

## Executive Summary

The external resources feature implementation has successfully completed Steps 1-4 of the execution plan, with Step 5 (Error Handling) identified as the next implementation priority. The investigation reveals a solid foundation with basic error handling already implemented, but significant gaps remain in providing user-friendly error messages and comprehensive validation.

## Current Implementation Status

### Completed Steps (1-4)

#### Step 1: Schema and Configuration ✅ COMPLETED
- Implemented external resource schema with Resolution naming theme
- Added validation for external resource blocks in `internal/declarative/resources/external_resource.go`
- Support for both direct ID and selector patterns with XOR validation
- Parent field support for hierarchical resources

#### Step 2: Resource Type Registry ✅ COMPLETED 
- Created complete registry system in `internal/declarative/external/registry.go`
- Implemented all 13 resource type adapters with base adapter pattern
- Factory pattern for adapter injection and dependency management
- Full SDK integration with GetByID and GetBySelector methods

#### Step 3: External Resource Resolver ✅ COMPLETED
- Core resolver implementation in `internal/declarative/external/resolver.go` 
- Dependency graph with topological sorting (Kahn's algorithm)
- Integration with planner for pre-resolution phase
- Resource caching mechanism and interface-based design

#### Step 4: Reference Resolution Integration ✅ COMPLETED
- Dynamic reference field detection replacing hardcoded approach
- Added `getResourceMappings()` with thread-safe caching
- Resources self-declare reference fields via `GetReferenceFieldMappings()`
- Comprehensive test coverage in `resolver_dynamic_test.go`

### Current Error Handling Implementation

#### Existing Error Handling Patterns

1. **Basic Validation Errors** (Already Implemented):
   ```go
   // From external_resource.go
   func (e ExternalResourceResource) Validate() error {
       if err := ValidateRef(e.Ref); err != nil {
           return fmt.Errorf("invalid external resource ref: %w", err)
       }
       // XOR validation, resource type validation, parent validation
   }
   ```

2. **Basic Resolution Errors** (Already Implemented):
   ```go
   // From resolver.go
   func (r *ResourceResolver) createZeroMatchError(resource Resource) error {
       return fmt.Errorf("external resource %q selector matched 0 resources\n"+
           "  Resource type: %s\n"+
           "  Selector: %s\n"+
           "  Suggestion: Verify the resource exists in Konnect and the selector fields are correct",
           resource.GetRef(), resource.GetResourceType(), selectorStr)
   }
   ```

3. **SDK Error Propagation** (Basic Implementation):
   ```go
   // From resolver.go  
   if err := adapter.GetBySelector(ctx, selector.GetMatchFields(), parentResource); err != nil {
       return fmt.Errorf("failed to resolve by selector: %w", err)
   }
   ```

#### Error Handling Gaps for Step 5

Based on the execution plan and current implementation, the following error handling improvements are needed:

## Step 5: Error Handling - Requirements Analysis

### Required Improvements

1. **Clear error messages for zero matches** (Partially Implemented)
   - ✅ Basic structure exists in `createZeroMatchError()`
   - ❌ Missing context about available resources
   - ❌ No suggestions for similar resources
   - ❌ Missing parent context when applicable

2. **Error messages for multiple matches** (Partially Implemented)
   - ✅ Basic structure exists in `createMultipleMatchError()`
   - ❌ No listing of matching resources for user review
   - ❌ Missing guidance on which fields to add for specificity

3. **Validation errors for invalid configurations** (Partially Implemented)
   - ✅ Basic XOR validation exists
   - ❌ Missing field-specific validation messages
   - ❌ No validation for selector field values
   - ❌ Missing context about supported fields per resource type

4. **Handle SDK errors gracefully** (Not Implemented)
   - ❌ Raw SDK errors are propagated without context
   - ❌ No handling of network timeouts
   - ❌ No handling of authentication errors
   - ❌ No handling of authorization (403) errors
   - ❌ No handling of resource not found (404) errors

5. **Add detailed error context** (Not Implemented)
   - ❌ No resource hierarchy context in error messages
   - ❌ No configuration file location references
   - ❌ No YAML line number information for validation errors

## Detailed Code Analysis

### Current Error Handling Architecture

#### Error Types and Structures
```go
// Existing in external_resource.go
type ExternalResourceError struct {
    Ref          string
    ResourceType string
    Field        string
    Message      string
    Cause        error
}
```

#### Base Adapter Error Handling
```go
// From base_adapter.go
func (b *BaseAdapter) FilterBySelector(resources []interface{}, selector map[string]string, 
    getField func(interface{}, string) string) (interface{}, error) {
    
    if len(matches) == 0 {
        return nil, fmt.Errorf("no resources found matching selector: %v", selector)
    }
    if len(matches) > 1 {
        return nil, fmt.Errorf("selector matched %d resources, expected 1: %v", len(matches), selector)
    }
}
```

#### Resolver Error Handling
```go
// From resolver.go - shows current implementation
func (r *ResourceResolver) resolveResource(ctx context.Context, resource Resource) error {
    if len(results) == 0 {
        return r.createZeroMatchError(resource)
    }
    if len(results) > 1 {
        return r.createMultipleMatchError(resource, len(results))
    }
}
```

### Integration Points

#### Planner Integration
- External resolution occurs in `planner.go:405` via `p.externalResolver.ResolveExternalResources(ctx, externalResources)`
- Errors bubble up to planning phase: `return fmt.Errorf("failed to resolve external resources: %w", err)`
- Planning errors are surfaced to user through command execution

#### Test Coverage Status
- Tests passing: `ok  	github.com/kong/kongctl/internal/declarative/external	2.226s`
- Base adapter tests exist: `base_adapter_test.go`
- Registry tests exist: `registry_test.go`
- Missing: Comprehensive error scenario testing

## Technical Dependencies

### Current Dependencies
- **State Client**: `internal/declarative/state` - for SDK operations
- **Resource Types**: `internal/declarative/resources` - for external resource definitions
- **Registry System**: `internal/declarative/external/registry.go` - for adapter management
- **Planning System**: `internal/declarative/planner` - for integration

### Implementation Files Key to Step 5

1. **Primary Files for Error Handling**:
   - `/Users/rick.spurgeon@konghq.com/go/src/github.com/Kong/kongctl/internal/declarative/external/resolver.go` - Core error handling logic
   - `/Users/rick.spurgeon@konghq.com/go/src/github.com/Kong/kongctl/internal/declarative/external/adapters/base_adapter.go` - SDK error handling
   - `/Users/rick.spurgeon@konghq.com/go/src/github.com/Kong/kongctl/internal/declarative/resources/external_resource.go` - Validation errors

2. **Supporting Files**:
   - `/Users/rick.spurgeon@konghq.com/go/src/github.com/Kong/kongctl/internal/declarative/external/types.go` - Error type definitions
   - `/Users/rick.spurgeon@konghq.com/go/src/github.com/Kong/kongctl/internal/declarative/planner/planner.go` - Error propagation

## Quality Gates Status

### Current Build Status
- ✅ Build: `make build` passes
- ✅ Lint: `make lint` passes (4 minor naming convention warnings acknowledged)
- ✅ Tests: `make test` passes with all external packages testing successfully
- ✅ Integration: All quality gates from previous steps maintained

### Test Coverage Analysis
- **Unit Tests**: Present for base adapters and registry
- **Integration Tests**: Missing for error scenarios
- **Error Path Testing**: Minimal coverage identified

## Edge Cases and Considerations

### Critical Edge Cases for Step 5

1. **Network and Connectivity Issues**
   - SDK timeout handling
   - Connection refused scenarios
   - DNS resolution failures

2. **Authentication and Authorization**
   - Invalid PAT tokens
   - Expired tokens
   - Insufficient permissions (403 errors)

3. **Resource State Issues**
   - Resources deleted between discovery and resolution
   - Parent resources not found when child resolution attempted
   - Circular dependency detection and reporting

4. **Configuration Edge Cases**
   - Malformed selector values (regex, special characters)
   - Invalid parent references
   - Empty match fields

5. **SDK Response Edge Cases**
   - Unexpected response formats
   - Missing expected fields in SDK responses
   - Union type handling errors

## Recommended Implementation Approach

### Phase 1: Enhanced Error Messages (High Priority)
1. Improve `createZeroMatchError()` and `createMultipleMatchError()` with:
   - Context about available resources
   - Suggestions for fixing selectors
   - Parent resource context when applicable

2. Add SDK error classification and friendly messages
3. Include configuration context (file paths, line numbers when available)

### Phase 2: Validation Improvements (Medium Priority)
1. Field-specific validation with detailed messages
2. Resource type compatibility validation
3. Selector value format validation

### Phase 3: Comprehensive Error Testing (Medium Priority)
1. Unit tests for all error scenarios
2. Integration tests with mock SDK failures
3. End-to-end error scenario testing

## Next Implementation Step

**Step 5: Error Handling** is ready for implementation with:

- **Clear scope**: Enhance existing error handling foundation
- **Well-defined requirements**: From execution plan and gap analysis
- **Solid foundation**: Core functionality working and tested
- **Integration points**: Already established with planner
- **Quality gates**: All passing and ready for incremental improvements

The implementation can proceed immediately, focusing on user experience improvements while maintaining the existing solid architectural foundation.

## Conclusion

The external resources feature has a strong foundation with Steps 1-4 completed successfully. Step 5 represents a critical user experience improvement that will make the feature production-ready. The existing error handling provides a good starting point, but significant enhancements are needed to provide the clear, actionable error messages required for a production-quality CLI tool.

The implementation path is clear, dependencies are resolved, and all quality gates are passing, making this the optimal next step for implementation.