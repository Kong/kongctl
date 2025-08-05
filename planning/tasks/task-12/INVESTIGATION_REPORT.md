# Portal Page Update Issue Investigation Report

## Issue Summary

**GitHub Issue**: #34 - "Error applying 'missing' entity" for portal pages
**Error Message**: `portal_page no longer exists`
**Symptoms**: When updating portal pages, the operation fails during validation with the message that the portal page no longer exists, even though the page exists and was found during planning.

## Root Cause Analysis

### Primary Root Cause

The issue stems from an **architectural mismatch** between the adapter interface design and hierarchical resource requirements. The `BaseExecutor.validateResourceForUpdate()` method calls `GetByName()` on resource adapters to verify that resources exist before updating them. However, portal pages (and other child resources) implement `GetByName()` to always return `(nil, nil)` because they cannot be uniquely identified by name alone.

**Evidence from Code**:

1. **PortalPageAdapter.GetByName()** (`/internal/declarative/executor/portal_page_adapter.go:155-159`):
   ```go
   func (p *PortalPageAdapter) GetByName(_ context.Context, _ string) (ResourceInfo, error) {
       // Portal pages don't have a direct "get by name" method
       // The planner handles this by searching through the list
       return nil, nil
   }
   ```

2. **BaseExecutor.validateResourceForUpdate()** (`/internal/declarative/executor/base_executor.go:117-123`):
   ```go
   resource, err := b.validateResourceForUpdate(ctx, resourceName, change)
   if err != nil {
       return "", fmt.Errorf("failed to validate %s for update: %w", b.ops.ResourceType(), err)
   }
   if resource == nil {
       return "", fmt.Errorf("%s no longer exists", b.ops.ResourceType())
   }
   ```

### Systemic Pattern

This issue affects **all child resources** that require parent context for identification:

- **PortalPageAdapter**: Returns `(nil, nil)` with comment "Portal pages don't have a direct 'get by name' method"
- **PortalSnippetAdapter**: Returns `(nil, nil)` with comment "Portal snippets are looked up by the planner from the list"
- **APIDocumentAdapter**: Returns `(nil, nil)` with comment "API documents don't have a direct 'get by name' method"

### Why Planning Works but Execution Fails

**During Planning** (`/internal/declarative/planner/portal_child_planner.go`):
- The planner has access to portal IDs and can call `ListManagedPortalPages(ctx, portalID)` (line 466)
- It builds maps by full slug paths and can properly identify existing pages
- For updates, it stores both `ResourceID` (page ID) and portal reference information in the `PlannedChange`

**During Execution**:
- The executor tries to validate the resource exists by calling `GetByName()`
- Child resource adapters can't implement meaningful `GetByName()` because they need both parent ID and name
- Validation fails even though the resource exists and the executor has all the necessary IDs

## Current Fallback Mechanism

The `BaseExecutor` already has a fallback strategy for this scenario in `validateResourceForUpdate()` (lines 216-227):

```go
// Strategy 2: For protection changes, try ID-based lookup if available
if change.ResourceID != "" && isProtectionChange(change) {
    if idLookup, ok := b.ops.(interface{ GetByID(context.Context, string) (ResourceInfo, error) }); ok {
        resource, err := idLookup.GetByID(ctx, change.ResourceID)
        if err == nil && resource != nil {
            logger.Debug("Resource found via ID lookup during protection change", ...)
            return resource, nil
        }
    }
}
```

**Issue**: This fallback only applies to protection changes, but regular updates also need it.

## Available Infrastructure

The required infrastructure already exists:

1. **State Client Methods**:
   - `Client.GetPortalPage(ctx, portalID, pageID)` - line 1471 in `state/client.go`
   - `Client.GetPortalSnippet(ctx, portalID, snippetID)` - exists based on planner usage
   - `Client.GetAPIDocument(ctx, apiID, documentID)` - likely exists

2. **Context Extraction**:
   - Portal page adapter already has `getPortalID(ctx)` method (lines 177-192)
   - Similar pattern exists in other child resource adapters

## Detailed Analysis

### Planning Phase Success

The planner successfully handles portal pages by:

1. **Fetching existing state**: `ListManagedPortalPages(ctx, portalID)` with proper portal context
2. **Building lookup maps**: Creates `existingByPath` and `existingByID` maps (lines 492-546)
3. **Proper comparison**: Fetches full page details with `GetPortalPage(ctx, portalID, existingPage.ID)` (line 583)
4. **Resource identification**: Uses full slug paths and hierarchical relationships
5. **Change creation**: Stores `ResourceID` (page ID) and `References` (portal info) in `PlannedChange`

### Execution Phase Failure

The executor fails because:

1. **Generic validation**: `validateResourceForUpdate()` uses generic `GetByName()` approach
2. **Missing context**: Child resources need parent IDs but `GetByName()` doesn't provide them
3. **Interface limitation**: `ResourceOperations` interface assumes name-only lookup is sufficient
4. **Incomplete fallback**: `GetByID()` fallback only applies to protection changes

## Impact Assessment

### Affected Operations

- **Portal page updates**: All updates fail with "portal_page no longer exists"
- **Portal snippet updates**: Likely affected (same pattern)
- **API document updates**: Likely affected (same pattern)
- **Protection changes**: May work due to existing fallback, but limited scope

### Not Affected Operations

- **CREATE operations**: Work because no validation is needed
- **DELETE operations**: Use different validation logic
- **Resources with top-level names**: APIs, portals, auth strategies work fine

## Recommended Solutions

### Solution 1: Implement GetByID Methods (Recommended)

**For PortalPageAdapter** (`portal_page_adapter.go`):

```go
// GetByID gets a portal page by ID
func (p *PortalPageAdapter) GetByID(ctx context.Context, id string) (ResourceInfo, error) {
    // Get portal ID from context
    portalID, err := p.getPortalID(ctx)
    if err != nil {
        return nil, err
    }
    
    page, err := p.client.GetPortalPage(ctx, portalID, id)
    if err != nil {
        return nil, err
    }
    if page == nil {
        return nil, nil
    }
    
    return &PortalPageResourceInfo{page: page}, nil
}
```

**Benefits**:
- Leverages existing fallback mechanism in `BaseExecutor`
- Minimal code changes required
- Uses existing infrastructure
- Maintains architectural consistency

### Solution 2: Extend Fallback to All Updates

**Modify `base_executor.go`** to apply GetByID fallback to all updates, not just protection changes:

```go
// Strategy 2: Try ID-based lookup if available (removed protection change restriction)
if change.ResourceID != "" {
    if idLookup, ok := b.ops.(interface{ GetByID(context.Context, string) (ResourceInfo, error) }); ok {
        resource, err := idLookup.GetByID(ctx, change.ResourceID)
        if err == nil && resource != nil {
            return resource, nil
        }
    }
}
```

### Solution 3: Enhanced Context-Aware Interface (Future)

Create specialized interfaces for child resources:

```go
type HierarchicalResourceOperations[TCreate, TUpdate any] interface {
    ResourceOperations[TCreate, TUpdate]
    GetByIDWithContext(ctx context.Context, id string) (ResourceInfo, error)
}
```

## Implementation Priority

### High Priority (Immediate Fix)

1. **PortalPageAdapter.GetByID()**: Implement for portal pages (most critical)
2. **Extend fallback scope**: Remove protection change restriction from GetByID fallback
3. **Test with real scenarios**: Verify fix works for reported use cases

### Medium Priority (Follow-up)

1. **PortalSnippetAdapter.GetByID()**: Same pattern for snippets
2. **APIDocumentAdapter.GetByID()**: Same pattern for API documents
3. **Integration tests**: Add tests covering update scenarios for child resources

### Low Priority (Future Enhancement)

1. **Interface redesign**: Consider specialized interfaces for hierarchical resources
2. **Documentation**: Update adapter development guidelines
3. **Validation improvements**: Enhanced error messages for debugging

## Related Files

### Core Issue Files
- `/internal/declarative/executor/portal_page_adapter.go` - GetByName() returns nil
- `/internal/declarative/executor/base_executor.go` - validateResourceForUpdate() logic
- `/internal/declarative/executor/portal_child_operations.go` - Legacy operations

### Supporting Files
- `/internal/declarative/planner/portal_child_planner.go` - Planning logic that works correctly
- `/internal/declarative/state/client.go` - Client methods for resource lookup
- `/internal/declarative/executor/portal_snippet_adapter.go` - Same pattern, likely affected
- `/internal/declarative/executor/api_document_adapter.go` - Same pattern, likely affected

## Test Scenarios

### Scenario 1: Portal Page Update
1. Create portal with page
2. Modify page content in YAML
3. Apply changes
4. **Expected**: Update succeeds
5. **Actual**: "portal_page no longer exists" error

### Scenario 2: Cross-Reference Validation
1. Portal page references another page as parent
2. Update child page
3. **Expected**: Validation finds existing page
4. **Actual**: Validation fails due to GetByName() returning nil

## Conclusion

The issue is a well-defined architectural problem with a clear solution path. The existing fallback mechanism in `BaseExecutor` provides a solid foundation for the fix. Implementing `GetByID()` methods for child resource adapters will resolve the immediate issue while maintaining architectural consistency.

The recommended approach is to implement `GetByID()` methods for affected adapters and extend the fallback mechanism to cover all update operations, not just protection changes. This solution is minimal, leverages existing infrastructure, and fixes the core issue without major architectural changes.