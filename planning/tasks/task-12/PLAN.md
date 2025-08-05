# Portal Page Update Issue - Comprehensive Fix Plan

## Executive Summary

This plan addresses the "portal_page no longer exists" error during UPDATE operations in kongctl. The issue is an architectural mismatch between the adapter interface design and hierarchical resource requirements. The solution involves a two-phase approach: an immediate fix to remove an artificial restriction, followed by proper architectural improvements.

**Impact**: Affects all child resources (portal pages, portal snippets, API documents)  
**Complexity**: Low to Medium  
**Risk**: Low (leverages existing infrastructure)  
**Timeline**: Phase 1 (immediate fix) - 1 day, Phase 2 (proper architecture) - 2-3 days

## Problem Analysis

### Root Cause (from Investigation Report)

The `BaseExecutor.validateResourceForUpdate()` method calls `GetByName()` on resource adapters to verify resources exist before updating them. However, portal pages and other child resources implement `GetByName()` to always return `(nil, nil)` because they cannot be uniquely identified by name alone - they require parent context (portal ID + name).

### Architectural Mismatch

The `ResourceOperations` interface assumes name-only lookup is sufficient, but hierarchical resources require parent context for identification:

- **Portal Pages**: Need portal ID + page slug
- **Portal Snippets**: Need portal ID + snippet name  
- **API Documents**: Need API ID + document name

### The Artificial Restriction (from Flow Report)

A working GetByID fallback mechanism exists in `BaseExecutor.validateResourceForUpdate()` (lines 216-227), but it's artificially restricted to protection changes only:

```go
// Line 216 in base_executor.go - THE CRITICAL RESTRICTION
if change.ResourceID != "" && isProtectionChange(change) {
    // GetByID fallback would work here, but only for protection changes
}
```

### Systemic Impact

This issue affects **all child resources** in the system:
- `PortalPageAdapter.GetByName()` - returns `(nil, nil)`
- `PortalSnippetAdapter.GetByName()` - returns `(nil, nil)`  
- `APIDocumentAdapter.GetByName()` - returns `(nil, nil)`

## Proposed Solution

### Two-Phase Approach

**Phase 1: Immediate Fix (Remove Artificial Restriction)**
- Remove the `&& isProtectionChange(change)` condition from BaseExecutor
- Leverages existing GetByID fallback mechanism
- Provides immediate relief for the reported issue

**Phase 2: Proper Architecture (Implement GetByID Methods)**
- Add GetByID methods to all affected child resource adapters
- Use existing context extraction patterns and client methods
- Provides proper architectural solution for long-term maintainability

### Why This Approach

1. **Leverages Existing Infrastructure**: All required client methods and context patterns already exist
2. **Minimal Risk**: Phase 1 is a one-line change, Phase 2 uses established patterns
3. **Backwards Compatible**: No breaking changes to existing interfaces
4. **Immediate Relief**: Phase 1 fixes the critical issue while Phase 2 provides proper architecture
5. **Future-Proof**: Sets pattern for similar hierarchical resources

## Alternative Approaches Considered

### Alternative 1: Modify ResourceOperations Interface
- **Approach**: Add GetByNameWithContext or similar methods
- **Rejected**: Breaking change affecting all adapters, much larger scope

### Alternative 2: Skip Validation for Child Resources  
- **Approach**: Bypass validation for certain resource types
- **Rejected**: Removes safety checks, inconsistent behavior, not architectural

### Alternative 3: Custom Validation Logic per Resource Type
- **Approach**: Specialized validation methods for different resource types
- **Rejected**: Complex, inconsistent patterns, harder to maintain

### Alternative 4: Restructure Planning Phase
- **Approach**: Store complete resource info in PlannedChange instead of IDs
- **Rejected**: Large architectural change, affects planning phase, increases memory usage

## Implementation Steps

### Phase 1: Remove Artificial Restriction (Critical Path)

#### Step 1.1: Modify BaseExecutor Validation Logic
**File**: `/internal/declarative/executor/base_executor.go`  
**Location**: Line 216  
**Change**:
```go
// Current:
if change.ResourceID != "" && isProtectionChange(change) {

// Change to:
if change.ResourceID != "" {
```

#### Step 1.2: Verify Build and Test
```bash
make build
make lint  
make test
```

#### Step 1.3: Manual Testing
Test the exact scenario from GitHub issue #34:
1. Create portal with page
2. Modify page content in YAML
3. Apply changes
4. **Expected**: Update succeeds (no "portal_page no longer exists" error)

### Phase 2: Implement GetByID Methods (Proper Architecture)

#### Step 2.1: Implement PortalPageAdapter.GetByID()
**File**: `/internal/declarative/executor/portal_page_adapter.go`  
**Add Method**:
```go
// GetByID gets a portal page by ID using portal context
func (p *PortalPageAdapter) GetByID(ctx context.Context, id string) (ResourceInfo, error) {
    // Get portal ID from context using existing pattern
    portalID, err := p.getPortalID(ctx)
    if err != nil {
        return nil, fmt.Errorf("failed to get portal ID for page lookup: %w", err)
    }
    
    // Use existing client method
    page, err := p.client.GetPortalPage(ctx, portalID, id)
    if err != nil {
        return nil, fmt.Errorf("failed to get portal page: %w", err)
    }
    if page == nil {
        return nil, nil
    }
    
    return &PortalPageResourceInfo{page: page}, nil
}
```

#### Step 2.2: Implement PortalSnippetAdapter.GetByID()
**File**: `/internal/declarative/executor/portal_snippet_adapter.go`  
**Add Method**:
```go
// GetByID gets a portal snippet by ID using portal context
func (p *PortalSnippetAdapter) GetByID(ctx context.Context, id string) (ResourceInfo, error) {
    // Get portal ID from context (similar pattern to portal pages)
    portalID, err := p.getPortalID(ctx)
    if err != nil {
        return nil, fmt.Errorf("failed to get portal ID for snippet lookup: %w", err)
    }
    
    // Use existing client method
    snippet, err := p.client.GetPortalSnippet(ctx, portalID, id)
    if err != nil {
        return nil, fmt.Errorf("failed to get portal snippet: %w", err)
    }
    if snippet == nil {
        return nil, nil
    }
    
    return &PortalSnippetResourceInfo{snippet: snippet}, nil
}
```

#### Step 2.3: Implement APIDocumentAdapter.GetByID()
**File**: `/internal/declarative/executor/api_document_adapter.go`  
**Add Method**:
```go
// GetByID gets an API document by ID using API context
func (a *APIDocumentAdapter) GetByID(ctx context.Context, id string) (ResourceInfo, error) {
    // Get API ID from context (similar pattern to getPortalID)
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

#### Step 2.4: Verify Context Extraction Methods Exist
**Files to Check**:
- Portal adapters should have `getPortalID(ctx)` method (already exists in portal_page_adapter.go:177-192)
- API document adapter should have similar `getAPIID(ctx)` method
- If missing, implement following the same pattern as `getPortalID`

#### Step 2.5: Verify Client Methods Exist
**File**: `/internal/declarative/state/client.go`  
**Required Methods** (should already exist):
- `GetPortalPage(ctx context.Context, portalID, pageID string) (*PortalPage, error)`
- `GetPortalSnippet(ctx context.Context, portalID, snippetID string) (*PortalSnippet, error)`
- `GetAPIDocument(ctx context.Context, apiID, documentID string) (*APIDocument, error)`

#### Step 2.6: Quality Gates After Each Method
After implementing each GetByID method:
```bash
make build
make lint
make test
```

## Risk Assessment

### Phase 1 Risks

**Risk**: Removing protection change restriction might affect other operations  
**Mitigation**: The restriction was artificial - GetByID fallback is safe for all update operations  
**Probability**: Low  

**Risk**: Existing functionality regression  
**Mitigation**: Comprehensive testing of all operation types (CREATE, UPDATE, DELETE)  
**Probability**: Low  

### Phase 2 Risks

**Risk**: Context extraction methods might fail in some scenarios  
**Mitigation**: Use existing patterns that already work in other adapter methods  
**Probability**: Low  

**Risk**: Client methods might not exist or have different signatures  
**Mitigation**: Verify method signatures before implementation, refer to working planner code  
**Probability**: Low  

**Risk**: New methods introduce bugs  
**Mitigation**: Follow exact patterns from existing working code, comprehensive testing  
**Probability**: Low  

### Overall Risk Assessment: **LOW**
- Leverages existing infrastructure and patterns
- Minimal changes to well-tested code paths
- Clear rollback strategy available

## Testing Strategy

### Unit Testing
```bash
# After each change
make test

# Specific package testing if needed  
go test -v ./internal/declarative/executor/
```

### Integration Testing
```bash
# Full integration test suite
make test-integration

# Specific scenarios
go test -v -tags=integration ./test/integration/...
```

### Manual Testing Scenarios

#### Scenario 1: Portal Page Update (Primary Issue)
```yaml
# Initial state
portals:
  - name: "test-portal"
    pages:
      - slug: "getting-started"
        content: "Original content"

# Modified state (should work after fix)
portals:
  - name: "test-portal"  
    pages:
      - slug: "getting-started"
        content: "Updated content"
```

#### Scenario 2: Portal Snippet Update
```yaml
# Test similar pattern for snippets
portals:
  - name: "test-portal"
    snippets:
      - name: "header-snippet"
        content: "Original snippet"
```

#### Scenario 3: API Document Update
```yaml
# Test similar pattern for API documents
apis:
  - name: "test-api"
    documents:
      - slug: "api-guide"
        content: "Original documentation"
```

#### Scenario 4: Regression Testing
- Verify CREATE operations still work for all resource types
- Verify DELETE operations still work for all resource types
- Verify UPDATE operations for parent resources (APIs, portals) still work
- Verify protection changes still work

#### Scenario 5: Edge Cases
- Test with missing parent resources (should fail gracefully)
- Test with invalid resource IDs (should fail gracefully)
- Test with network failures during validation (should fail gracefully)

### Success Criteria Verification

For each test scenario:
1. **No "no longer exists" errors** for legitimate update operations
2. **Proper error messages** for actual failure cases
3. **All existing functionality** continues to work
4. **Quality gates pass**: build, lint, test, integration tests

## Success Criteria

### Phase 1 Success Criteria
- [ ] Portal page updates work without "portal_page no longer exists" error
- [ ] All existing functionality continues to work (CREATE, DELETE, parent updates)
- [ ] Build, lint, and test quality gates pass
- [ ] Manual testing of primary scenario succeeds

### Phase 2 Success Criteria  
- [ ] All three child resource types (pages, snippets, documents) have working GetByID methods
- [ ] Update operations work for all child resource types
- [ ] Comprehensive test coverage for all scenarios
- [ ] Code follows established patterns and conventions
- [ ] Integration tests pass

### Overall Success Criteria
- [ ] GitHub issue #34 is resolved
- [ ] Solution is architectural, not a workaround
- [ ] No regression in existing functionality
- [ ] Pattern established for future hierarchical resources
- [ ] Documentation updated with implementation patterns

## Rollback Plan

### Phase 1 Rollback
If issues arise after Phase 1:
```go
// Revert base_executor.go line 216 back to:
if change.ResourceID != "" && isProtectionChange(change) {
```

**Rollback Triggers**:
- Any existing functionality stops working
- Quality gates fail
- Integration tests fail

### Phase 2 Rollback  
If issues arise after adding GetByID methods:
- Remove the specific GetByID method causing issues
- Each method can be independently rolled back
- Phase 1 fix remains in place for basic functionality

**Rollback Testing**:
After any rollback, run full test suite to ensure system returns to previous working state.

## Future Considerations

### Long-term Architectural Improvements

#### Enhanced Interface Design
Consider creating specialized interfaces for hierarchical resources:
```go
type HierarchicalResourceOperations[TCreate, TUpdate any] interface {
    ResourceOperations[TCreate, TUpdate]
    GetByIDWithContext(ctx context.Context, id string) (ResourceInfo, error)
}
```

#### Documentation and Guidelines
- Update adapter development guidelines with hierarchical resource patterns
- Document the GetByID pattern for future child resources
- Add examples of proper context extraction methods

#### Monitoring and Observability
- Add trace-level logging for validation strategy usage
- Add metrics for GetByName vs GetByID fallback usage
- Improve error messages with more context

### Potential Future Child Resources
This pattern will apply to any future hierarchical resources:
- Portal domains
- Portal customizations  
- Service versions
- Route configurations
- Plugin configurations

### Performance Considerations
The GetByID methods add one additional API call during validation, but:
- Only occurs during UPDATE operations (not CREATE or DELETE)
- Uses existing client infrastructure with proper error handling
- Minimal performance impact compared to failed operations

## Implementation Files Summary

### Files Requiring Changes

1. **`/internal/declarative/executor/base_executor.go`** (Line 216)
   - Remove `&& isProtectionChange(change)` condition
   - One-line change for immediate fix

2. **`/internal/declarative/executor/portal_page_adapter.go`**
   - Add `GetByID` method using existing `getPortalID` and `client.GetPortalPage`
   - ~15 lines of new code following established patterns

3. **`/internal/declarative/executor/portal_snippet_adapter.go`**
   - Add `GetByID` method using portal context and `client.GetPortalSnippet`
   - ~15 lines of new code following established patterns

4. **`/internal/declarative/executor/api_document_adapter.go`**
   - Add `GetByID` method using API context and `client.GetAPIDocument`  
   - ~15 lines of new code following established patterns
   - May need to implement `getAPIID(ctx)` if not already present

### Files for Reference (No Changes Needed)

- `/internal/declarative/state/client.go` - Client method signatures
- `/internal/declarative/planner/portal_child_planner.go` - Working patterns
- `/internal/declarative/executor/portal_child_operations.go` - Legacy working code

### Testing Files

- Integration tests for child resource updates
- Unit tests for new GetByID methods
- Regression tests for existing functionality

## Conclusion

This plan provides a comprehensive solution to the portal page update issue while addressing the underlying architectural problem. The two-phase approach minimizes risk while ensuring both immediate relief and proper long-term architecture.

The solution leverages existing infrastructure, follows established patterns, and provides a template for handling similar hierarchical resource issues in the future. With proper testing and quality gates, this fix should resolve the issue completely while maintaining system stability and performance.