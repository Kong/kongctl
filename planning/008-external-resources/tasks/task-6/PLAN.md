# External Resources Step 5 Implementation Plan: Error Handling Enhancement

**Plan Date**: 2025-08-08  
**Current Branch**: feat/008-external-resources  
**Implementation Step**: Step 5/8 - Error Handling (50% → 62.5% completion)  
**Priority**: High - Critical for production readiness

## Executive Summary

This plan implements Step 5 of the external resources execution plan, focusing on comprehensive error handling improvements. The implementation builds upon the solid foundation of Steps 1-4 (completed) to provide production-quality error messages, SDK error classification, and enhanced validation. The scope is well-defined with clear success criteria and maintains backward compatibility throughout.

## Current Context

### Completed Foundation (Steps 1-4)
- ✅ **Step 1**: Schema and Configuration with XOR validation
- ✅ **Step 2**: Resource Type Registry with 13 resource adapters
- ✅ **Step 3**: External Resource Resolver with dependency graph
- ✅ **Step 4**: Dynamic Reference Field Detection with thread-safe caching

### Current Error Handling Gaps
Based on investigation analysis, the following improvements are required:
- Basic error messages need context and actionable suggestions
- SDK errors are propagated raw without user-friendly translation
- Validation errors lack field-specific guidance
- Missing configuration file context in error messages
- Limited error scenario testing coverage

## Implementation Scope

### Primary Objectives
1. **Enhanced User Experience**: Transform technical errors into actionable user guidance
2. **SDK Error Classification**: Handle network, authentication, and API errors gracefully
3. **Comprehensive Validation**: Provide detailed validation feedback with context
4. **Production Readiness**: Ensure error handling meets production CLI standards

### Success Criteria
- All error messages include actionable context and suggestions
- SDK errors are classified and translated to user-friendly messages
- Validation errors provide specific guidance about configuration issues
- 90%+ test coverage for error scenarios
- No regressions in existing functionality
- Performance impact < 5% for error scenarios

## Detailed Implementation Tasks

### Phase 1: Core Error Message Enhancement (High Priority)

#### Task 1.1: Enhanced Zero Match Errors
**File**: `/Users/rick.spurgeon@konghq.com/go/src/github.com/Kong/kongctl/internal/declarative/external/resolver.go`

**Current Implementation** (lines 58-64):
```go
func (r *ResourceResolver) createZeroMatchError(resource Resource) error {
    return fmt.Errorf("external resource %q selector matched 0 resources\n"+
        "  Resource type: %s\n"+
        "  Selector: %s\n"+
        "  Suggestion: Verify the resource exists in Konnect and the selector fields are correct",
        resource.GetRef(), resource.GetResourceType(), selectorStr)
}
```

**Enhancement Requirements**:
1. Add context about available resources of the same type (up to 5 examples)
2. Implement fuzzy matching for similar resource names/identifiers
3. Include parent resource context when applicable
4. Provide specific selector field validation suggestions
5. Add configuration file context when available

**Implementation Approach**:
- Add helper method `getAvailableResourceContext()` to query registry for sample resources
- Implement `suggestSimilarResources()` using Levenshtein distance for name matching
- Enhance error structure to include parent hierarchy context
- Integrate with registry metadata for field suggestions

#### Task 1.2: Enhanced Multiple Match Errors  
**File**: `/Users/rick.spurgeon@konghq.com/go/src/github.com/Kong/kongctl/internal/declarative/external/resolver.go`

**Current Implementation** (lines 66-72):
```go
func (r *ResourceResolver) createMultipleMatchError(resource Resource, count int) error {
    return fmt.Errorf("external resource %q selector matched %d resources\n"+
        "  Resource type: %s\n"+
        "  Selector: %s\n"+
        "  Suggestion: Use more specific selector fields to match exactly one resource",
        resource.GetRef(), count, resource.GetResourceType(), selectorStr)
}
```

**Enhancement Requirements**:
1. List all matching resources with their IDs and distinguishing fields
2. Suggest specific additional selector fields for disambiguation
3. Show registry metadata about available selector fields
4. Include examples of disambiguating selector patterns

**Implementation Approach**:
- Modify method signature to accept matched resources list
- Add `formatMatchedResources()` helper to display resource details
- Integrate with registry to suggest additional selector fields
- Generate example selector patterns based on matched resource differences

### Phase 2: SDK Error Classification and Translation (High Priority)

#### Task 2.1: SDK Error Classification System
**File**: `/Users/rick.spurgeon@konghq.com/go/src/github.com/Kong/kongctl/internal/declarative/external/adapters/base_adapter.go`

**Current Implementation** (lines 89-93):
```go
if err := adapter.GetBySelector(ctx, selector.GetMatchFields(), parentResource); err != nil {
    return fmt.Errorf("failed to resolve by selector: %w", err)
}
```

**Enhancement Requirements**:
1. Classify SDK errors by category (network, auth, API, validation)
2. Handle specific HTTP status codes (401, 403, 404, 500, timeout)
3. Translate technical errors to user-friendly messages
4. Maintain error chain for debugging while presenting clean user messages

**Implementation Approach**:
- Create `SDKErrorClassifier` type with classification methods
- Add `classifySDKError(error) SDKErrorType` function
- Implement `translateSDKError(error, context) error` for user-friendly messages
- Handle authentication scenarios (invalid PAT, expired tokens, permissions)

**New Error Classification Types**:
```go
type SDKErrorType int

const (
    SDKErrorNetwork SDKErrorType = iota
    SDKErrorAuthentication
    SDKErrorAuthorization
    SDKErrorNotFound
    SDKErrorValidation
    SDKErrorServerError
    SDKErrorUnknown
)
```

#### Task 2.2: Network and Connectivity Error Handling
**File**: `/Users/rick.spurgeon@konghq.com/go/src/github.com/Kong/kongctl/internal/declarative/external/adapters/base_adapter.go`

**Enhancement Requirements**:
1. Handle connection timeouts with retry suggestions
2. Handle DNS resolution failures
3. Handle connection refused scenarios
4. Provide actionable guidance for network issues

**Implementation Approach**:
- Add network error detection patterns
- Implement retry logic for transient network failures
- Provide environment-specific suggestions (VPN, proxy, firewall)

### Phase 3: Enhanced Validation and Context (Medium Priority)

#### Task 3.1: Field-Specific Validation Enhancement
**File**: `/Users/rick.spurgeon@konghq.com/go/src/github.com/Kong/kongctl/internal/declarative/resources/external_resource.go`

**Current Implementation** (lines 47-71):
```go
func (e ExternalResourceResource) Validate() error {
    if err := ValidateRef(e.Ref); err != nil {
        return fmt.Errorf("invalid external resource ref: %w", err)
    }
    // Basic XOR and type validation
}
```

**Enhancement Requirements**:
1. Validate selector field values and formats
2. Provide resource type-specific validation guidance
3. Include configuration file and line number context
4. Validate parent-child relationship compatibility

**Implementation Approach**:
- Add `validateSelectorFields()` method with registry integration
- Implement `validateResourceTypeCompatibility()` for parent-child validation
- Add configuration context structure for file/line information
- Create field-specific validation error types

#### Task 3.2: Registry Integration for Error Context
**File**: `/Users/rick.spurgeon@konghq.com/go/src/github.com/Kong/kongctl/internal/declarative/external/registry.go`

**Enhancement Requirements**:
1. Add methods to query available resources for error context
2. Provide metadata about supported selector fields per resource type
3. Enable similarity matching for error suggestions
4. Support resource hierarchy validation

**Implementation Approach**:
- Add `GetSampleResources(resourceType string) ([]interface{}, error)` method
- Implement `GetSelectorFieldMetadata(resourceType string) map[string]FieldMetadata`
- Add `FindSimilarResourceNames(resourceType, query string) []string` for suggestions

### Phase 4: Error Type System Enhancement (Medium Priority)

#### Task 4.1: Structured Error Types
**File**: `/Users/rick.spurgeon@konghq.com/go/src/github.com/Kong/kongctl/internal/declarative/external/types.go`

**Enhancement Requirements**:
1. Create structured error types for different error categories
2. Implement error interfaces with context methods
3. Support error categorization and user-friendly formatting
4. Maintain error chain for debugging

**New Error Types**:
```go
type ExternalResourceValidationError struct {
    Ref           string
    ResourceType  string
    Field         string
    Value         string
    Message       string
    Suggestions   []string
    ConfigContext *ConfigurationContext
    Cause         error
}

type ExternalResourceResolutionError struct {
    Ref            string
    ResourceType   string
    Selector       map[string]string
    MatchedCount   int
    MatchedDetails []ResourceSummary
    Suggestions    []string
    ParentContext  *ParentResourceContext
    Cause          error
}

type ExternalResourceSDKError struct {
    Ref          string
    ResourceType string
    Operation    string
    SDKErrorType SDKErrorType
    HTTPStatus   int
    Message      string
    UserMessage  string
    Suggestions  []string
    Cause        error
}
```

## Quality Assurance Strategy

### Testing Requirements

#### Unit Test Coverage
**Files to Create/Enhance**:
- `/Users/rick.spurgeon@konghq.com/go/src/github.com/Kong/kongctl/internal/declarative/external/resolver_errors_test.go`
- `/Users/rick.spurgeon@konghq.com/go/src/github.com/Kong/kongctl/internal/declarative/external/adapters/base_adapter_errors_test.go`
- `/Users/rick.spurgeon@konghq.com/go/src/github.com/Kong/kongctl/internal/declarative/resources/external_resource_validation_test.go`

**Test Scenarios**:
1. **Zero Match Scenarios**: Empty Konnect environment, invalid selectors, non-existent resources
2. **Multiple Match Scenarios**: Ambiguous selectors, similar resource names
3. **SDK Error Scenarios**: Network failures, authentication errors, API errors
4. **Validation Scenarios**: Invalid configuration, malformed selectors, incompatible relationships
5. **Edge Cases**: Circular dependencies, missing parents, malformed YAML

#### Integration Test Coverage
**Files to Create**:
- `/Users/rick.spurgeon@konghq.com/go/src/github.com/Kong/kongctl/internal/declarative/external/integration_errors_test.go`

**Integration Scenarios**:
1. Mock SDK failures for all error types
2. End-to-end error propagation testing
3. Error message formatting validation
4. Performance impact measurement for error scenarios

### Quality Gates

#### Build Requirements
- ✅ `make build` must pass with no warnings
- ✅ `make lint` must pass with zero issues  
- ✅ `make test` must pass with 90%+ error path coverage
- ✅ `make test-integration` must pass with mocked error scenarios

#### Performance Requirements
- Error handling overhead < 5% of normal operation time
- Memory allocation for error context < 1MB per error
- No memory leaks in error path execution

#### User Experience Requirements
- All error messages must be actionable (include "what to do next")
- Error messages must include relevant context (resource type, selector, etc.)
- Technical details available in debug/trace logging mode
- Error format consistent across all external resource error types

## Risk Management

### Backward Compatibility
**Risk**: Breaking existing error handling workflows  
**Mitigation**: 
- Preserve existing error message structure as fallback
- Add feature flag for enhanced error messages if needed
- Comprehensive regression testing with existing configurations

### Performance Impact  
**Risk**: Error context gathering impacts normal operation performance  
**Mitigation**:
- Lazy evaluation of error context (only when errors occur)
- Caching of registry metadata used in error messages
- Performance benchmarks for error scenario code paths

### Integration Complexity
**Risk**: Changes affect planner error propagation  
**Mitigation**:
- Maintain existing error interface contracts
- Add integration tests for planner error flow
- Gradual rollout of enhancements by error type

### Test Coverage Gaps
**Risk**: Insufficient testing of error scenarios  
**Mitigation**:
- Dedicated error scenario test suite
- Mock SDK integration for failure testing
- Manual testing with real Konnect environment edge cases

## Implementation Timeline

### Phase 1: Core Error Messages (Days 1-3)
- **Day 1**: Implement enhanced zero match and multiple match errors
- **Day 2**: Add registry integration for error context
- **Day 3**: Unit testing and validation

### Phase 2: SDK Error Classification (Days 4-6)  
- **Day 4**: Implement SDK error classification system
- **Day 5**: Add network and authentication error handling
- **Day 6**: Integration testing with mocked SDK failures

### Phase 3: Validation Enhancement (Days 7-8)
- **Day 7**: Enhanced validation with field-specific messages
- **Day 8**: Configuration context integration and testing

### Phase 4: Testing and Polish (Days 9-10)
- **Day 9**: Comprehensive error scenario testing
- **Day 10**: Performance validation and documentation

## Success Metrics

### Functional Metrics
- ✅ All identified error scenarios handled with user-friendly messages
- ✅ SDK errors classified and translated appropriately
- ✅ Validation errors provide actionable guidance
- ✅ Configuration context included where applicable

### Quality Metrics  
- ✅ 90%+ test coverage for error code paths
- ✅ Zero regressions in existing functionality
- ✅ All quality gates passing (build, lint, test, integration)
- ✅ Performance impact < 5% for error scenarios

### User Experience Metrics
- ✅ Error messages are actionable and clear
- ✅ Users can resolve configuration issues from error guidance
- ✅ Debug information available for troubleshooting
- ✅ Consistent error format across all external resource operations

## Integration Points

### Planner Integration
**File**: `/Users/rick.spurgeon@konghq.com/go/src/github.com/Kong/kongctl/internal/declarative/planner/planner.go` (line 405)
- Ensure enhanced errors propagate correctly through planning phase
- Maintain existing error handling interface
- Add context for configuration file information when available

### Command Integration  
**File**: `/Users/rick.spurgeon@konghq.com/go/src/github.com/Kong/kongctl/internal/cmd/root/products/konnect/declarative/declarative.go`
- Ensure user-friendly errors reach command level properly
- Maintain existing error reporting mechanisms
- Add trace-level logging for enhanced error debugging

### State Client Integration
**Files**: State client interaction through base adapter
- Ensure SDK error classification works with all state client operations
- Handle authentication and authorization error scenarios
- Maintain existing retry logic and connection handling

## Post-Implementation Considerations

### Step 6 Preparation
- Error handling improvements provide foundation for advanced features
- User feedback on error message quality informs future enhancements
- Performance monitoring establishes baseline for additional features

### Maintenance Requirements  
- Documentation updates for new error types and messages
- Monitoring of error message effectiveness through user feedback
- Regular review of SDK error patterns for classification updates

### Future Enhancements
- Interactive error resolution (suggest fixes, apply automatically)
- Error message localization support  
- Advanced similarity matching for resource suggestions
- Integration with Kong documentation for error-specific help links

## Conclusion

This implementation plan transforms the external resources feature from a technically sound system into a production-ready CLI tool with exceptional user experience. The focus on error handling ensures users can successfully configure and troubleshoot external resource dependencies, completing the foundation for Steps 6-8 of the execution plan.

The phased approach minimizes risk while ensuring comprehensive coverage of error scenarios. Quality gates and testing requirements ensure reliability and maintainability. Upon completion, the external resources feature will provide industry-standard error handling that guides users to successful configuration resolution.

**Next Action**: Begin implementation with Phase 1 (Core Error Messages) while maintaining all existing quality gates and backward compatibility requirements.