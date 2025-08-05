# Fix API Document Update Issue #36 - Implementation Plan

## Problem Summary

**GitHub Issue**: #36 - "kongctl apply trying to CREATE api_document that already exists"

**Issue Description**: When attempting to update existing API documents, kongctl incorrectly plans them as CREATE operations instead of UPDATE operations, resulting in 409 Resource Conflict errors during execution.

**Relationship to Issue #34**: This is a **direct continuation** of the recently fixed issue #34 (portal pages). The root cause is identical - child resource adapters missing the `GetByID()` method required by the BaseExecutor's validation fallback mechanism.

**Current Impact**: 
- All API document update operations fail with "api_document no longer exists" error
- Users cannot modify existing API documentation through kongctl
- Similar issues likely affect other child resources (portal snippets, domains, API versions, publications)

## Root Cause Analysis Summary

### Technical Root Cause

The BaseExecutor uses a three-strategy validation approach for UPDATE operations:

1. **Strategy 1**: `GetByName()` - Fails for child resources that can't be looked up by name alone
2. **Strategy 2**: `GetByID()` - **MISSING** for APIDocumentAdapter and other child resource adapters
3. **Strategy 3**: Namespace-specific lookup - Not applicable

**APIDocumentAdapter.GetByName()** returns `(nil, nil)` because API documents require both API ID and document ID for lookup. The fallback to **Strategy 2** fails because APIDocumentAdapter doesn't implement the `GetByID(context.Context, string) (ResourceInfo, error)` interface.

### Flow Analysis

1. **Planning Phase** ✅ **WORKS**: Correctly identifies UPDATE needed and creates PlannedChange with proper ResourceID
2. **Execution Phase** ❌ **FAILS**: BaseExecutor validation fails even though resource exists and all required IDs are available
3. **Error Result**: "api_document no longer exists" → 409 Resource Conflict

### Affected Resources

**Child Resources Missing GetByID() (AFFECTED)**:
- `api_document` - **Issue #36** (immediate priority)
- `portal_snippet` - Same issue pattern
- `portal_domain` - Same issue pattern  
- `api_version` - Same issue pattern
- `api_publication` - Same issue pattern

**Child Resources with GetByID() (FIXED)**:
- `portal_page` - **Fixed in issue #34**

**Top-Level Resources (NOT AFFECTED)**:
- `api`, `portal`, `auth_strategy` - Can be looked up by name directly

## Proposed Solution

### Solution Pattern

Follow the **exact same pattern** used to fix issue #34 (portal pages). Implement `GetByID()` methods for all affected child resource adapters using the established template:

```go
func (a *[ResourceType]Adapter) GetByID(ctx context.Context, id string) (ResourceInfo, error) {
    // Get parent ID from context using existing method
    parentID, err := a.get[ParentType]ID(ctx)
    if err != nil {
        return nil, fmt.Errorf("failed to get [parent] ID for [resource] lookup: %w", err)
    }
    
    // Use existing client method
    resource, err := a.client.Get[ResourceType](ctx, parentID, id)
    if err != nil {
        return nil, fmt.Errorf("failed to get [resource]: %w", err)
    }
    if resource == nil {
        return nil, nil
    }
    
    return &[ResourceType]ResourceInfo{[field]: resource}, nil
}
```

### Infrastructure Availability

All required components already exist:
- ✅ **State Client Methods**: `Client.GetAPIDocument()`, etc.
- ✅ **Context Extraction**: `getAPIID()`, `getPortalID()` methods
- ✅ **Resource Info Wrappers**: `APIDocumentResourceInfo`, etc.
- ✅ **BaseExecutor Fallback**: Enhanced in issue #34 to use GetByID()

## Implementation Steps

### Phase 1: Immediate Fix (High Priority)

#### Step 1.1: Implement APIDocumentAdapter.GetByID()

**File**: `/Users/rick.spurgeon@konghq.com/go/src/github.com/Kong/kongctl-portal-page-bug/internal/declarative/executor/api_document_adapter.go`

**Action**: Add the following method after the existing `GetByName()` method (around line 127):

```go
// GetByID gets an API document by ID using API context
func (a *APIDocumentAdapter) GetByID(ctx context.Context, id string) (ResourceInfo, error) {
    // Get API ID from context using existing pattern
    apiID, err := a.getAPIID(ctx)
    if err != nil {
        return nil, fmt.Errorf("failed to get API ID for document lookup: %w", err)
    }
    
    // Use existing client method
    document, err := a.client.GetAPIDocument(ctx, apiID, id)
    if err != nil {
        return nil, fmt.Errorf("failed to get API document: %w", err)
    }
    if document == nil {
        return nil, nil
    }
    
    return &APIDocumentResourceInfo{document: document}, nil
}
```

**Dependencies Used**:
- `a.getAPIID(ctx)` - Already exists (lines 144-159)
- `a.client.GetAPIDocument(ctx, apiID, id)` - Already exists (state/client.go:1070)
- `APIDocumentResourceInfo` - Already exists (lines 162-181)

#### Step 1.2: Quality Verification

Run quality gates in sequence:
```bash
make build    # Must succeed
make lint     # Zero issues
make test     # All pass
```

#### Step 1.3: Integration Testing

Test with real scenario:
1. Create API with document
2. Modify document content in YAML
3. Run `kongctl apply`
4. **Expected**: UPDATE succeeds (no 409 error)

### Phase 2: Comprehensive Fix (Medium Priority)

#### Step 2.1: Implement PortalSnippetAdapter.GetByID()

**File**: `/Users/rick.spurgeon@konghq.com/go/src/github.com/Kong/kongctl-portal-page-bug/internal/declarative/executor/portal_snippet_adapter.go`

**Action**: Add GetByID() method using `getPortalID()` and `client.GetPortalSnippet()`

#### Step 2.2: Implement PortalDomainAdapter.GetByID()

**File**: `/Users/rick.spurgeon@konghq.com/go/src/github.com/Kong/kongctl-portal-page-bug/internal/declarative/executor/portal_domain_adapter.go`

**Action**: Add GetByID() method using `getPortalID()` and appropriate client method

#### Step 2.3: Implement APIVersionAdapter.GetByID()

**File**: `/Users/rick.spurgeon@konghq.com/go/src/github.com/Kong/kongctl-portal-page-bug/internal/declarative/executor/api_version_adapter.go`

**Action**: Add GetByID() method using `getAPIID()` and appropriate client method

#### Step 2.4: Implement APIPublicationAdapter.GetByID()

**File**: `/Users/rick.spurgeon@konghq.com/go/src/github.com/Kong/kongctl-portal-page-bug/internal/declarative/executor/api_publication_adapter.go`

**Action**: Add GetByID() method using `getAPIID()` and appropriate client method

**Note**: Each step should be followed by individual quality verification and testing before proceeding to the next.

### Phase 3: Comprehensive Testing

#### Step 3.1: Unit Testing

For each implemented GetByID() method:
- Test successful lookup with valid parent and resource IDs
- Test failure when parent ID cannot be extracted from context
- Test failure when client method returns error
- Test proper nil handling when resource doesn't exist

#### Step 3.2: Integration Testing  

End-to-end scenarios for each resource type:
- Create parent resource with child resource
- Modify child resource configuration
- Apply configuration with `kongctl apply`
- Verify UPDATE operation succeeds
- Verify actual content is updated
- Verify no duplicate resources are created

#### Step 3.3: Regression Testing

Ensure existing functionality still works:
- CREATE operations for all resource types
- DELETE operations for all resource types
- UPDATE operations for top-level resources
- Portal page updates (issue #34 fix)

## Testing Approach

### Test Scenarios

#### Scenario 1: API Document Update (Issue #36)
```yaml
# Initial state
apis:
  - name: test-api
    documents:
      - title: "API Documentation" 
        content: "Initial content"
        slug: "api-doc"

# Modified state  
apis:
  - name: test-api
    documents:
      - title: "Updated API Documentation"  # Changed
        content: "Updated content"           # Changed
        slug: "api-doc"                     # Same slug
```

**Expected**: UPDATE operation succeeds, document content updated in place.

#### Scenario 2: Multiple Child Resource Updates
```yaml
portals:
  - name: test-portal
    snippets:
      - name: header-snippet
        content: "Updated header code"  # Changed
    domains:
      - name: api.example.com
        wildcard: true                  # Changed

apis:
  - name: test-api  
    versions:
      - name: v2                        # Changed from v1
    publications:
      - portal: test-portal
        published: false                # Changed from true
```

**Expected**: All UPDATE operations succeed for child resources.

### Verification Points

1. **Planning Phase**: Verify plan shows UPDATE actions (not CREATE)
2. **Execution Phase**: Verify no "resource no longer exists" errors
3. **Result Verification**: Verify actual resource content is updated
4. **Uniqueness**: Verify no duplicate resources are created
5. **Error Handling**: Verify proper error messages for invalid scenarios

### Automated Testing

**Unit Tests**: Add tests for each GetByID() implementation
**Integration Tests**: Add end-to-end update scenarios using `-tags=integration`
**CI Pipeline**: Ensure all quality gates pass in continuous integration

## Risk Assessment

### Risk Level: **LOW**

**Justification**:
- ✅ **Proven Pattern**: Follows exact same approach as issue #34 fix
- ✅ **Existing Infrastructure**: All required components already exist
- ✅ **Isolated Impact**: Only affects UPDATE operations for child resources
- ✅ **Reversible**: Easy rollback by removing GetByID() methods
- ✅ **No Breaking Changes**: No changes to existing interfaces or behavior

### Potential Risks and Mitigation

#### Risk 1: Client Method Failures
**Risk**: Client methods might not exist for all resource types
**Mitigation**: Verify client methods exist before implementation
**Status**: APIDocument client method confirmed to exist

#### Risk 2: Context Extraction Failures  
**Risk**: Parent ID extraction from context might fail
**Mitigation**: Use existing, tested context extraction methods
**Status**: API and Portal ID extraction methods already exist and tested

#### Risk 3: Resource Info Wrapper Issues
**Risk**: Resource info structures might not exist or be incomplete
**Mitigation**: Use existing resource info structures
**Status**: All required structures already exist

### Edge Cases Handled

1. **Parent Resource Deleted**: Client method will return appropriate error
2. **Invalid Child Resource ID**: Client method will return nil (not found)
3. **Network/API Failures**: Existing error handling will bubble up errors
4. **Context Missing**: Existing context extraction includes error handling

## Quality Verification

### Build Requirements
```bash
# Must pass after each change
make build
make lint  
make test
make test-integration  # When applicable
```

### Code Quality Standards
- Follow existing code patterns and conventions
- Include proper error handling and logging
- Add appropriate documentation comments
- Maintain consistent naming and structure

### Review Checklist
- [ ] GetByID() method follows established pattern
- [ ] Uses existing context extraction method
- [ ] Uses existing client method
- [ ] Uses existing resource info wrapper
- [ ] Includes proper error handling
- [ ] All quality gates pass
- [ ] Integration test scenarios work
- [ ] No regression in existing functionality

## Success Criteria

### Primary Success Criteria (Issue #36)
- [ ] API document UPDATE operations succeed without 409 errors
- [ ] Plan correctly shows UPDATE action for existing documents
- [ ] Actual document content is updated in place
- [ ] No duplicate documents are created

### Secondary Success Criteria (Comprehensive Fix)
- [ ] All child resource UPDATE operations work correctly
- [ ] Portal snippets, domains, API versions, publications can be updated
- [ ] No similar "resource no longer exists" errors occur
- [ ] All existing functionality continues to work

### Quality Criteria
- [ ] All build and test pipelines pass
- [ ] Zero linting issues introduced
- [ ] Integration tests demonstrate end-to-end success
- [ ] Code follows established patterns and conventions

## Implementation Timeline

### Phase 1 Time Estimate: 2-4 hours
- 1 hour: Implement APIDocumentAdapter.GetByID()
- 1 hour: Quality verification and unit testing
- 1-2 hours: Integration testing and validation

### Phase 2 Time Estimate: 4-6 hours  
- 1 hour each: Implement remaining 4 adapters
- 2 hours: Comprehensive testing and validation

### Total Estimate: 6-10 hours
**Priority**: Phase 1 can be implemented immediately to fix issue #36. Phase 2 can be scheduled to prevent similar issues.

## Key Files to Modify

### Immediate (Phase 1)
- `/Users/rick.spurgeon@konghq.com/go/src/github.com/Kong/kongctl-portal-page-bug/internal/declarative/executor/api_document_adapter.go`

### Follow-up (Phase 2)  
- `/Users/rick.spurgeon@konghq.com/go/src/github.com/Kong/kongctl-portal-page-bug/internal/declarative/executor/portal_snippet_adapter.go`
- `/Users/rick.spurgeon@konghq.com/go/src/github.com/Kong/kongctl-portal-page-bug/internal/declarative/executor/portal_domain_adapter.go`
- `/Users/rick.spurgeon@konghq.com/go/src/github.com/Kong/kongctl-portal-page-bug/internal/declarative/executor/api_version_adapter.go`
- `/Users/rick.spurgeon@konghq.com/go/src/github.com/Kong/kongctl-portal-page-bug/internal/declarative/executor/api_publication_adapter.go`

### Reference Files (No Changes)
- `/Users/rick.spurgeon@konghq.com/go/src/github.com/Kong/kongctl-portal-page-bug/internal/declarative/executor/portal_page_adapter.go` - Pattern reference
- `/Users/rick.spurgeon@konghq.com/go/src/github.com/Kong/kongctl-portal-page-bug/internal/declarative/executor/base_executor.go` - Validation logic
- `/Users/rick.spurgeon@konghq.com/go/src/github.com/Kong/kongctl-portal-page-bug/internal/declarative/state/client.go` - Client methods

## Conclusion

This plan provides a **comprehensive solution** to issue #36 and prevents similar issues across all child resource types. The approach is:

- ✅ **Well-Proven**: Based on successful fix for issue #34
- ✅ **Low-Risk**: Uses existing infrastructure and established patterns  
- ✅ **Phased**: Can fix immediate issue first, then address broader problem
- ✅ **Testable**: Clear verification criteria and test scenarios
- ✅ **Maintainable**: Follows consistent patterns across all adapters

The implementation leverages existing architecture and requires minimal code changes while providing maximum impact in resolving UPDATE operation failures for hierarchical resources.