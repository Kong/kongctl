# Fix Plan: API Publication Creation Failure

## Problem Summary

API Publications fail during sync with error "API ID is required for publication 
operations" when the API is created in the same plan. The root cause is a timing 
bug in `executor.go` where the PlannedChange is stored in context BEFORE 
references are resolved, causing adapters to receive outdated change objects 
with empty reference IDs.

## Root Cause Analysis

The issue occurs in `executor.go:createResource()` at line 608:

1. PlannedChange is stored in context with empty API ID
2. API reference is resolved and local change updated (lines 669-677)  
3. Adapter retrieves the ORIGINAL change from context (still has empty ID)
4. Adapter fails because it cannot find the API ID

This affects all resource types with parent/reference dependencies:
- API Publications (depends on API)
- API Versions (depends on API)
- API Documents (depends on API)
- Portal Pages with parent pages

## Solution Overview

Move the context update to AFTER reference resolution in the executor. This 
ensures adapters receive the fully resolved PlannedChange with all reference 
IDs populated.

## Implementation Steps

### Step 1: Fix the Timing Issue in executor.go

**File**: `internal/declarative/executor.go`

**Changes needed in `createResource()` function (lines 606-760)**:

1. Remove the early context update at line 608
2. Add context updates after reference resolution for each resource type
3. Ensure consistent handling across all resource types

**Specific code changes**:

```go
func (e *Executor) createResource(ctx context.Context, change *planner.PlannedChange) error {
    // Remove this line (line 608):
    // ctx = context.WithValue(ctx, contextKeyPlannedChange, *change)
    
    // ... existing validation code ...
    
    switch change.ResourceType {
    case "api_publication":
        // First resolve API reference if needed
        if apiRef, ok := change.References["api_id"]; ok && apiRef.ID == "" {
            apiID, err := e.resolveAPIRef(ctx, apiRef)
            if err != nil {
                return fmt.Errorf("failed to resolve API reference: %w", err)
            }
            apiRef.ID = apiID
            change.References["api_id"] = apiRef
        }
        
        // Then resolve portal reference if needed
        if portalRef, ok := change.References["portal_id"]; ok && portalRef.ID == "" {
            portalID, err := e.resolvePortalRef(ctx, portalRef)
            if err != nil {
                return fmt.Errorf("failed to resolve portal reference: %w", err)
            }
            portalRef.ID = portalID
            change.References["portal_id"] = portalRef
        }
        
        // NOW update context with fully resolved change
        ctx = context.WithValue(ctx, contextKeyPlannedChange, *change)
        return e.apiPublicationExecutor.Create(ctx, *change)
        
    case "api_version":
        // Similar pattern for api_version
        if apiRef, ok := change.References["api_id"]; ok && apiRef.ID == "" {
            apiID, err := e.resolveAPIRef(ctx, apiRef)
            if err != nil {
                return fmt.Errorf("failed to resolve API reference: %w", err)
            }
            apiRef.ID = apiID
            change.References["api_id"] = apiRef
        }
        
        ctx = context.WithValue(ctx, contextKeyPlannedChange, *change)
        return e.apiVersionExecutor.Create(ctx, *change)
        
    case "api_document":
        // Similar pattern for api_document
        if parentRef, ok := change.References["parent_id"]; ok && parentRef.ID == "" {
            parentID, err := e.resolveParentRef(ctx, parentRef, change.Parent.Type)
            if err != nil {
                return fmt.Errorf("failed to resolve parent reference: %w", err)
            }
            parentRef.ID = parentID
            change.References["parent_id"] = parentRef
        }
        
        ctx = context.WithValue(ctx, contextKeyPlannedChange, *change)
        return e.apiDocumentExecutor.Create(ctx, *change)
        
    case "portal_page":
        // Handle parent page references if present
        if parentRef, ok := change.References["parent_id"]; ok && parentRef.ID == "" {
            parentID, err := e.resolvePortalPageRef(ctx, parentRef)
            if err != nil {
                return fmt.Errorf("failed to resolve parent page reference: %w", err)
            }
            parentRef.ID = parentID
            change.References["parent_id"] = parentRef
        }
        
        ctx = context.WithValue(ctx, contextKeyPlannedChange, *change)
        return e.portalPageExecutor.Create(ctx, *change)
        
    // For resources without references, add context before execution
    case "portal":
        ctx = context.WithValue(ctx, contextKeyPlannedChange, *change)
        return e.portalExecutor.Create(ctx, *change)
        
    case "api":
        ctx = context.WithValue(ctx, contextKeyPlannedChange, *change)
        return e.apiExecutor.Create(ctx, *change)
        
    // ... handle other resource types similarly ...
    }
}
```

### Step 2: Add Reference Resolution Methods (if missing)

**File**: `internal/declarative/executor.go`

Ensure these reference resolution methods exist:

```go
func (e *Executor) resolvePortalPageRef(ctx context.Context, ref planner.ReferenceInfo) (string, error) {
    // Implementation to resolve portal page references
    // Similar to existing resolveAPIRef and resolvePortalRef
}

func (e *Executor) resolveParentRef(ctx context.Context, ref planner.ReferenceInfo, parentType string) (string, error) {
    // Generic parent resolution based on parent type
    switch parentType {
    case "api":
        return e.resolveAPIRef(ctx, ref)
    case "api_version":
        return e.resolveAPIVersionRef(ctx, ref)
    default:
        return "", fmt.Errorf("unsupported parent type: %s", parentType)
    }
}
```

### Step 3: Update deleteResource() Method

**File**: `internal/declarative/executor.go`

Apply the same pattern to `deleteResource()` to ensure consistency:

```go
func (e *Executor) deleteResource(ctx context.Context, change *planner.PlannedChange) error {
    // Remove early context update
    // Apply same pattern as createResource
}
```

### Step 4: Add Comprehensive Tests

**File**: `internal/declarative/executor_test.go`

Add test cases for:
1. Creating API and publication in same plan
2. Creating API version with API in same plan
3. Creating nested portal pages
4. Verifying context contains resolved references

```go
func TestExecutor_CreateAPIPublicationWithNewAPI(t *testing.T) {
    // Test that verifies API publication creation works when API is created
    // in the same plan
}

func TestExecutor_ContextContainsResolvedReferences(t *testing.T) {
    // Test that verifies adapters receive fully resolved changes
}
```

### Step 5: Integration Tests

**File**: `test/integration/api_publication_test.go`

Add integration test that reproduces the original issue:

```go
func TestAPIPublicationCreationInSamePlan(t *testing.T) {
    // Create config with API and publication
    // Run sync command
    // Verify both are created successfully
}
```

## Verification Steps

1. **Unit Testing**:
   ```bash
   go test ./internal/declarative/... -v
   ```

2. **Integration Testing**:
   ```bash
   make test-integration
   ```

3. **Manual Testing**:
   ```bash
   # Create a config.yaml with API and publication
   ./kongctl sync -f config.yaml --pat $(cat ~/.konnect/claude.pat)
   ```

4. **Test Scenarios**:
   - ✓ Create API and publication in same plan
   - ✓ Update existing API publication
   - ✓ Create multiple APIs with publications
   - ✓ Create API versions with new API
   - ✓ Create API documents with new API
   - ✓ Create nested portal pages

## Rollback Plan

If issues arise:
1. Revert the executor.go changes
2. Document any new edge cases discovered
3. Consider alternative approach of updating adapters to handle both patterns

## Timeline

- Step 1-3: Core fix implementation (1 hour)
- Step 4-5: Test implementation (1 hour)
- Verification: 30 minutes
- Total: ~2.5 hours

## Success Criteria

1. API publications can be created when API is in same plan
2. All existing tests continue to pass
3. No regression in other resource types
4. Clear error messages if reference resolution fails

## Additional Considerations

1. **Performance**: Moving context updates won't impact performance as it's the 
   same operation, just at a different time

2. **Backward Compatibility**: This change is backward compatible as it only 
   affects the timing of when data is stored in context

3. **Future Prevention**: Consider adding a linter rule or code review checklist 
   item to ensure context updates happen after all data mutations

## Alternative Solutions (Not Recommended)

1. **Update all adapters**: Modify adapters to check both context and receive 
   updated change as parameter. More complex and error-prone.

2. **Two-phase execution**: Resolve all references first, then execute. Would 
   require significant refactoring.

3. **Remove context usage**: Pass updated change directly. Would break existing 
   adapter patterns.

The recommended solution is the simplest and most focused fix that addresses 
the root cause without breaking existing patterns.