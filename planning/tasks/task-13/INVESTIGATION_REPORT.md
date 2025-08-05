# API Document Update Issue Investigation Report

## Issue Summary

**GitHub Issue**: #36 - "kongctl apply trying to CREATE api_document that already exists"
**Error**: 409 Resource Conflict when attempting to CREATE an api_document that already exists on the server
**Symptoms**: The planner detects an api_document for creation instead of update, even though the document exists and should be updated.

## Root Cause Analysis

### Primary Root Cause

The issue is **identical to the recently fixed issue #34 (portal pages)**. The problem stems from the same architectural mismatch between the adapter interface design and hierarchical resource requirements.

**APIDocumentAdapter is missing the `GetByID()` method** that was implemented for PortalPageAdapter as part of the fix for issue #34.

### Evidence from Code

1. **APIDocumentAdapter.GetByName()** (`/Users/rick.spurgeon@konghq.com/go/src/github.com/Kong/kongctl-portal-page-bug/internal/declarative/executor/api_document_adapter.go:122-126`):
   ```go
   func (a *APIDocumentAdapter) GetByName(_ context.Context, _ string) (ResourceInfo, error) {
       // API documents don't have a direct "get by name" method
       // The planner handles this by searching through the list
       return nil, nil
   }
   ```

2. **Missing GetByID() method**: APIDocumentAdapter does not implement `GetByID()`, unlike PortalPageAdapter which was fixed.

3. **BaseExecutor validation fails** (`/Users/rick.spurgeon@konghq.com/go/src/github.com/Kong/kongctl-portal-page-bug/internal/declarative/executor/base_executor.go:117-123`):
   ```go
   resource, err := b.validateResourceForUpdate(ctx, resourceName, change)
   if err != nil {
       return "", fmt.Errorf("failed to validate %s for update: %w", b.ops.ResourceType(), err)
   }
   if resource == nil {
       return "", fmt.Errorf("%s no longer exists", b.ops.ResourceType())
   }
   ```

### How the Fix for Issue #34 Works

The fix for portal pages (issue #34) implemented a two-part solution:

1. **BaseExecutor Enhancement** (`base_executor.go:215-227`): Extended the fallback mechanism to use `GetByID()` for **ALL** update operations (not just protection changes):
   ```go
   // Strategy 2: Try ID-based lookup if available (useful for child resources)
   if change.ResourceID != "" {
       if idLookup, ok := b.ops.(interface{ GetByID(context.Context, string) (ResourceInfo, error) }); ok {
           resource, err := idLookup.GetByID(ctx, change.ResourceID)
           if err == nil && resource != nil {
               logger.Debug("Resource found via ID lookup", ...)
               return resource, nil
           }
       }
   }
   ```

2. **PortalPageAdapter Enhancement** (`portal_page_adapter.go:194-212`): Implemented the `GetByID()` method:
   ```go
   func (p *PortalPageAdapter) GetByID(ctx context.Context, id string) (ResourceInfo, error) {
       portalID, err := p.getPortalID(ctx)
       if err != nil {
           return nil, fmt.Errorf("failed to get portal ID for page lookup: %w", err)
       }
       
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

### Why Planning Works but Execution Fails

**During Planning** (`/Users/rick.spurgeon@konghq.com/go/src/github.com/Kong/kongctl-portal-page-bug/internal/declarative/planner/api_planner.go:1240-1309`):
- The planner correctly calls `ListAPIDocuments(ctx, apiID)` with proper API context (line 1245)
- It indexes documents by slug and properly identifies existing documents (lines 1250-1267)
- For existing documents, it fetches full content and calls UPDATE (lines 1275-1286)
- The planner **correctly identifies that the document should be UPDATED, not CREATED**

**During Execution**:
- The executor tries to validate the resource exists by calling `GetByName()` (base_executor.go:210)
- APIDocumentAdapter's `GetByName()` returns `(nil, nil)` because it needs both API ID and document ID
- The validation fails even though the resource exists and the executor has the necessary IDs in the planned change
- The fallback to `GetByID()` doesn't work because APIDocumentAdapter doesn't implement it

## Affected Resources Analysis

### Child Resources Missing GetByID() (AFFECTED by same issue):

1. **APIDocumentAdapter** (`api_document_adapter.go:122-126`) - **Issue #36**
2. **PortalSnippetAdapter** (`portal_snippet_adapter.go:133-137`)
3. **PortalDomainAdapter** (`portal_domain_adapter.go:99-103`)
4. **APIVersionAdapter** (`api_version_adapter.go:80-84`)
5. **APIPublicationAdapter** (`api_publication_adapter.go:102-106`)

### Child Resources with GetByID() (FIXED):

1. **PortalPageAdapter** (`portal_page_adapter.go:194-212`) - **Fixed in issue #34**

### Top-Level Resources (NOT AFFECTED):

1. **APIAdapter** - Can fetch by name directly
2. **PortalAdapter** - Can fetch by name directly  
3. **AuthStrategyAdapter** - Can fetch by name directly

## Available Infrastructure for API Documents

The required infrastructure already exists for implementing the fix:

1. **State Client Method**: `Client.GetAPIDocument(ctx, apiID, documentID)` exists (line 1070 in `state/client.go`)
2. **Context Extraction**: APIDocumentAdapter already has `getAPIID(ctx)` method (lines 144-159)
3. **Resource Info Structure**: `APIDocumentResourceInfo` already exists (lines 162-181)

## Comparison with Portal Page Fix

The fix for APIDocumentAdapter should follow the **exact same pattern** as PortalPageAdapter:

### Required Implementation for APIDocumentAdapter:

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

## Impact Assessment

### Currently Affected Operations
- **API document updates**: All fail with "api_document no longer exists" (Issue #36)
- **Portal snippet updates**: Likely affected (same pattern)
- **Portal domain updates**: Likely affected (same pattern)
- **API version updates**: Likely affected (same pattern)
- **API publication updates**: Likely affected (same pattern)

### Working Operations
- **CREATE operations**: Work because no validation is needed
- **DELETE operations**: Use different validation logic
- **Portal page updates**: Work (fixed in issue #34)
- **Top-level resource updates**: Work (APIs, portals, auth strategies)

## Implementation Solution

### Immediate Fix (High Priority)

1. **Implement APIDocumentAdapter.GetByID()**: Use the same pattern as PortalPageAdapter
   - Extract API ID from context using existing `getAPIID()` method
   - Call `client.GetAPIDocument(ctx, apiID, id)`
   - Return `APIDocumentResourceInfo` wrapper

### Follow-up Fixes (Medium Priority)

Implement `GetByID()` methods for other affected child resource adapters:

2. **PortalSnippetAdapter.GetByID()**: Similar pattern with `getPortalID()` and `client.GetPortalSnippet()`
3. **PortalDomainAdapter.GetByID()**: Similar pattern with `getPortalID()` and appropriate client method
4. **APIVersionAdapter.GetByID()**: Similar pattern with `getAPIID()` and appropriate client method
5. **APIPublicationAdapter.GetByID()**: Similar pattern with `getAPIID()` and appropriate client method

### Verification Strategy

1. **Test with real scenarios**: Create API document, modify it, run `kongctl apply`
2. **Integration tests**: Add tests covering update scenarios for all child resources
3. **Error handling**: Verify proper error messages when resources don't exist

## Key Files

### Core Issue Files
- `/Users/rick.spurgeon@konghq.com/go/src/github.com/Kong/kongctl-portal-page-bug/internal/declarative/executor/api_document_adapter.go` - Missing GetByID()
- `/Users/rick.spurgeon@konghq.com/go/src/github.com/Kong/kongctl-portal-page-bug/internal/declarative/executor/base_executor.go` - Validation logic with fallback

### Reference Implementation
- `/Users/rick.spurgeon@konghq.com/go/src/github.com/Kong/kongctl-portal-page-bug/internal/declarative/executor/portal_page_adapter.go` - GetByID() implementation pattern

### Supporting Infrastructure
- `/Users/rick.spurgeon@konghq.com/go/src/github.com/Kong/kongctl-portal-page-bug/internal/declarative/state/client.go` - Client methods for resource lookup
- `/Users/rick.spurgeon@konghq.com/go/src/github.com/Kong/kongctl-portal-page-bug/internal/declarative/planner/api_planner.go` - Planning logic (works correctly)

### Other Affected Adapters
- `/Users/rick.spurgeon@konghq.com/go/src/github.com/Kong/kongctl-portal-page-bug/internal/declarative/executor/portal_snippet_adapter.go`
- `/Users/rick.spurgeon@konghq.com/go/src/github.com/Kong/kongctl-portal-page-bug/internal/declarative/executor/portal_domain_adapter.go`
- `/Users/rick.spurgeon@konghq.com/go/src/github.com/Kong/kongctl-portal-page-bug/internal/declarative/executor/api_version_adapter.go`
- `/Users/rick.spurgeon@konghq.com/go/src/github.com/Kong/kongctl-portal-page-bug/internal/declarative/executor/api_publication_adapter.go`

## Test Scenarios

### Scenario 1: API Document Update (Issue #36)
1. Create API with document
2. Modify document content in YAML
3. Run `kongctl apply`
4. **Expected**: Update succeeds
5. **Actual**: "api_document no longer exists" error → 409 Resource Conflict

### Scenario 2: Other Child Resource Updates
1. Create portal with snippet/domain
2. Create API with version/publication  
3. Modify child resources in YAML
4. Run `kongctl apply`
5. **Expected**: Updates succeed
6. **Likely Actual**: Similar "resource no longer exists" errors

## Conclusion

Issue #36 is a **direct continuation of issue #34**. The root cause is identical - child resource adapters missing `GetByID()` methods that the BaseExecutor fallback mechanism requires.

The fix is straightforward and well-established:
1. **APIDocumentAdapter needs GetByID() implementation** (immediate fix for #36)
2. **Other child resource adapters need the same fix** (prevent similar issues)

The solution leverages existing infrastructure:
- BaseExecutor fallback mechanism is already in place
- State client methods exist for all resource types
- Context extraction patterns are already established
- Resource info wrapper structures exist

This is a **minimal, low-risk fix** that follows the proven pattern from issue #34. The implementation should take the PortalPageAdapter.GetByID() method as a direct template, substituting API-specific client calls and context extraction methods.