# Implementation Plan: Fix Namespace Inheritance for Child Resources

## Overview

This plan addresses the systematic issue where child resources (api_version, api_publication, api_document, portal_page, etc.) are not inheriting namespaces from their parent resources. The fix ensures that child resources properly inherit their parent's namespace during the planning phase, preventing them from defaulting to the "default" namespace during execution.

## Problem Summary

- **Current Behavior**: Child resources show empty namespace in plan output and default to "default" namespace during sync
- **Expected Behavior**: Child resources should inherit their parent's namespace without explicitly storing namespace metadata
- **Root Cause**: Child resource planning functions don't set the Namespace field in PlannedChange structs

## Solution Strategy

Extract namespace from the planning context (already available) and set it in PlannedChange structs for all child resources. This approach:
- Uses existing context mechanism
- Maintains consistency with parent resource planning
- Avoids code duplication through a helper function
- Preserves the design where child resources don't have kongctl metadata

## Implementation Steps

### Step 1: Create Namespace Extraction Helper Function

**File**: `internal/declarative/planner/namespace_helper.go` (new file)

Create a helper function to extract namespace from context with proper defaulting:

```go
package planner

import "context"

// extractNamespaceFromContext extracts the namespace from the planning context.
// Returns the namespace from context or "default" if not found.
func extractNamespaceFromContext(ctx context.Context) string {
    if namespace, ok := ctx.Value(NamespaceContextKey).(string); ok && namespace != "" {
        return namespace
    }
    return DefaultNamespace
}
```

### Step 2: Update API Child Resource Planning Functions

**File**: `internal/declarative/planner/api_planner.go`

#### 2.1 Update planAPIVersionCreate (lines 511-563)

Add namespace extraction and set it in PlannedChange:

```go
func (p *Planner) planAPIVersionCreate(
    ctx context.Context, // Ensure ctx parameter is present
    apiRef string, apiID string, version resources.APIVersionResource, 
    dependsOn []string, plan *Plan,
) {
    // Extract namespace from context
    namespace := extractNamespaceFromContext(ctx)
    
    // ... existing field extraction code ...
    
    change := PlannedChange{
        ID:           p.nextChangeID(ActionCreate, "api_version", version.GetRef()),
        ResourceType: "api_version",
        ResourceRef:  version.GetRef(),
        Parent:       parentInfo,
        Action:       ActionCreate,
        Fields:       fields,
        DependsOn:    dependsOn,
        Namespace:    namespace, // Set the namespace
    }
    
    // ... rest of function ...
}
```

#### 2.2 Update planAPIVersionUpdate (lines 565-611)

Similar update to set namespace in PlannedChange:

```go
func (p *Planner) planAPIVersionUpdate(
    ctx context.Context, // Ensure ctx parameter is present
    apiRef string, apiID string, version resources.APIVersionResource,
    currentVersion resources.APIVersionResource, dependsOn []string, plan *Plan,
) {
    // Extract namespace from context
    namespace := extractNamespaceFromContext(ctx)
    
    // ... existing code ...
    
    change := PlannedChange{
        // ... existing fields ...
        Namespace: namespace, // Set the namespace
    }
    
    // ... rest of function ...
}
```

#### 2.3 Update planAPIPublicationCreate (lines 779-831)

```go
func (p *Planner) planAPIPublicationCreate(
    ctx context.Context, // Add ctx parameter if not present
    apiRef string, apiID string, pub resources.APIPublicationResource,
    versionDependsOn []string, plan *Plan,
) {
    // Extract namespace from context
    namespace := extractNamespaceFromContext(ctx)
    
    // ... existing code ...
    
    change := PlannedChange{
        // ... existing fields ...
        Namespace: namespace, // Set the namespace
    }
    
    // ... rest of function ...
}
```

#### 2.4 Update planAPIPublicationUpdate (lines 833-886)

Similar pattern - add namespace extraction and set in PlannedChange.

#### 2.5 Update planAPIImplementationCreate

Follow the same pattern for API implementation resources.

#### 2.6 Update planAPIImplementationUpdate

Follow the same pattern for API implementation updates.

#### 2.7 Update planAPIDocumentCreate (lines 1076-1108)

```go
func (p *Planner) planAPIDocumentCreate(
    ctx context.Context, // Add ctx parameter if not present
    apiRef string, apiID string, doc resources.APIDocumentResource,
    dependsOn []string, plan *Plan,
) {
    // Extract namespace from context
    namespace := extractNamespaceFromContext(ctx)
    
    // ... existing code ...
    
    change := PlannedChange{
        // ... existing fields ...
        Namespace: namespace, // Set the namespace
    }
    
    // ... rest of function ...
}
```

#### 2.8 Update planAPIDocumentUpdate (lines 1110-1136)

Follow the same pattern for document updates.

### Step 3: Update Portal Child Resource Planning Functions

**File**: `internal/declarative/planner/portal_child_planner.go`

Apply the same pattern to all portal child resource planning functions:

#### 3.1 Update planPortalPageCreate
#### 3.2 Update planPortalPageUpdate
#### 3.3 Update planPortalCustomizationCreate
#### 3.4 Update planPortalCustomizationUpdate
#### 3.5 Update planPortalCustomDomainCreate
#### 3.6 Update planPortalCustomDomainUpdate
#### 3.7 Update planPortalSnippetCreate
#### 3.8 Update planPortalSnippetUpdate

Each function should:
1. Accept context as parameter (if not already)
2. Extract namespace using `extractNamespaceFromContext(ctx)`
3. Set the Namespace field in PlannedChange

### Step 4: Update Function Calls to Pass Context

**Files**: Various planner files

Ensure all calls to child resource planning functions pass the context parameter. The context is already available in parent functions, so this should be straightforward.

For example, in `planAPIVersionChanges` (line 434):
```go
// Existing calls like:
p.planAPIVersionCreate(apiRef, apiID, version, dependsOn, plan)

// Should become:
p.planAPIVersionCreate(ctx, apiRef, apiID, version, dependsOn, plan)
```

### Step 5: Add Integration Tests

**File**: `test/integration/declarative/namespace_inheritance_test.go` (new file)

Create comprehensive tests to verify namespace inheritance:

```go
func TestChildResourceNamespaceInheritance(t *testing.T) {
    // Test cases:
    // 1. API with namespace, verify api_version inherits
    // 2. Portal with namespace, verify portal_page inherits
    // 3. Multiple namespaces with proper isolation
    // 4. Default namespace when parent has no namespace
}
```

### Step 6: Update Existing Tests

**File**: `test/integration/declarative/api_test.go`

Remove or update the TODO comment about the namespace bug (lines mentioning the known bug).

## Testing Strategy

1. **Unit Tests**: Test the `extractNamespaceFromContext` helper function
2. **Integration Tests**: 
   - Test namespace inheritance for all child resource types
   - Test multiple namespaces with proper isolation
   - Test default namespace behavior
3. **Manual Testing**:
   - Use example file: `docs/examples/declarative/namespace/single-team/api.yaml`
   - Verify plan output shows correct namespaces for child resources
   - Verify sync execution places resources in correct namespaces

## Validation Checklist

- [ ] All child resource planning functions extract namespace from context
- [ ] All child resource PlannedChange structs have Namespace field set
- [ ] Context is properly passed to all child planning functions
- [ ] Helper function handles missing namespace gracefully
- [ ] Integration tests pass for namespace inheritance
- [ ] Plan output shows correct namespaces for child resources
- [ ] Sync execution places child resources in correct namespaces
- [ ] No regression in existing functionality

## Risk Assessment

- **Low Risk**: Changes are localized to planner functions
- **No Breaking Changes**: Maintains backward compatibility
- **Performance Impact**: Minimal - only adds context value extraction

## Alternative Approaches Considered

1. **Pass parent namespace as parameter**: More explicit but requires more code changes
2. **Lookup parent namespace from resources**: More complex and potentially slower
3. **Store namespace in child resource metadata**: Would break the design principle

The chosen approach (context extraction) is the most elegant and consistent with existing patterns.

## Implementation Order

1. Create helper function (5 minutes)
2. Update API child resource functions (30 minutes)
3. Update Portal child resource functions (20 minutes)
4. Update function calls to pass context (15 minutes)
5. Write and run tests (30 minutes)
6. Manual validation (15 minutes)

Total estimated time: ~2 hours

## Success Criteria

1. Plan output shows correct namespace for all child resources
2. Sync execution places child resources in their parent's namespace
3. No child resources default to "default" namespace unless parent is in default
4. All tests pass
5. No performance degradation