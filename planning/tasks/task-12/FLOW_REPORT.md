# Portal Page Update Flow Analysis Report

## Executive Summary

This report maps the complete execution flow for portal page UPDATE operations in kongctl, identifying the exact failure point and architectural mismatch causing the "portal_page no longer exists" error.

**Root Cause**: BaseExecutor validation assumes all resources can be validated by name alone, but portal pages (and other child resources) require parent context (portal ID) for identification.

**Key Finding**: A working fallback mechanism exists but is artificially restricted to protection changes only.

## Complete Execution Flow Mapping

### 1. Portal Page UPDATE Operation Flow

```
User applies plan → Plan Execution → Portal Page Update
     ↓
executor.Execute() (executor.go:126)
     ↓
executeChange() (executor.go:174) - processes each PlannedChange
     ↓
updateResource() (executor.go:842) - handles ActionUpdate
     ↓
case "portal_page": (executor.go:928-950)
  - Resolves portal reference if needed
  - Calls portalPageExecutor.Update()
     ↓
BaseExecutor.Update() (base_executor.go:105) - FAILURE POINT
     ↓
validateResourceForUpdate() (base_executor.go:204) - FAILS HERE
     ↓
"portal_page no longer exists" error (base_executor.go:122)
```

### 2. BaseExecutor Validation Process (The Failure Point)

Located in `base_executor.go:204-250`, the validation has three strategies:

#### Strategy 1: Standard name-based lookup (FAILS for portal pages)
```go
// Line 210
resource, err := b.ops.GetByName(ctx, resourceName)
```

**What happens**: 
- Calls `PortalPageAdapter.GetByName()` 
- Always returns `(nil, nil)` - see portal_page_adapter.go:155-159
- Validation fails immediately

#### Strategy 2: ID-based lookup fallback (EXISTS but RESTRICTED)
```go
// Lines 216-227 - THE CRITICAL RESTRICTION
if change.ResourceID != "" && isProtectionChange(change) {
    if idLookup, ok := b.ops.(interface{ GetByID(...) }); ok {
        resource, err := idLookup.GetByID(ctx, change.ResourceID)
        // This would work, but only for protection changes!
    }
}
```

**The Problem**: This fallback is artificially restricted to `isProtectionChange(change)` only.

#### Strategy 3: Namespace lookup (ALSO RESTRICTED)
```go
// Lines 230-246 - Also only for protection changes
if isProtectionChange(change) && change.Fields != nil {
    // Additional fallback strategies
}
```

### 3. Portal Page Reference Resolution Process

Portal page updates involve complex reference resolution in the main executor:

```
updateResource() case "portal_page" (executor.go:928-950):
1. Resolve portal reference if needed (lines 930-937)
2. Resolve parent page reference if needed (lines 940-948)  
3. Call portalPageExecutor.Update() (line 950)
```

**Portal ID Resolution**:
```go
if portalRef, ok := change.References["portal_id"]; ok && portalRef.ID == "" {
    portalID, err := e.resolvePortalRef(ctx, portalRef)
    // Updates change.References["portal_id"] with resolved ID
}
```

**Context Passing**:
```go
ctx = context.WithValue(ctx, contextKeyPlannedChange, *change)
```

The PlannedChange with all resolved references is passed to the adapter via context.

### 4. PortalPageAdapter Operations

#### GetByName Implementation (Always Fails)
```go
// portal_page_adapter.go:155-159
func (p *PortalPageAdapter) GetByName(_ context.Context, _ string) (ResourceInfo, error) {
    // Portal pages don't have a direct "get by name" method
    // The planner handles this by searching through the list
    return nil, nil
}
```

#### GetPortalID Context Extraction
```go
// portal_page_adapter.go:177-192
func (p *PortalPageAdapter) getPortalID(ctx context.Context) (string, error) {
    change, ok := ctx.Value(contextKeyPlannedChange).(planner.PlannedChange)
    if portalRef, ok := change.References["portal_id"]; ok {
        if portalRef.ID != "" {
            return portalRef.ID, nil  // This works during execution
        }
    }
    return "", fmt.Errorf("portal ID is required for page operations")
}
```

#### Update Method (Would Work if Validation Passed)
```go
// portal_page_adapter.go:128-141
func (p *PortalPageAdapter) Update(ctx context.Context, id string, req UpdatePortalPageRequest, _ string) (string, error) {
    portalID, err := p.getPortalID(ctx)  // This works
    return p.client.UpdatePortalPage(ctx, portalID, id, req)  // This would work
}
```

## Planning vs Execution Comparison

### Planning Phase Success (Why It Works)

Located in `portal_child_planner.go:459-539`:

1. **Has Portal Context**: Planner receives `portalID` parameter directly
2. **Lists All Pages**: Calls `ListManagedPortalPages(ctx, portalID)` (line 466)
3. **Builds Lookup Maps**: Creates `existingByPath` and `existingByID` maps (lines 492-498)
4. **Hierarchical Matching**: Builds full slug paths including parent hierarchy (lines 500-539)
5. **Stores Resource Info**: Puts both `ResourceID` (page ID) and portal reference in `PlannedChange`

**Key Success Patterns**:
```go
// Line 466 - Direct API call with portal context
pages, err := p.client.ListManagedPortalPages(ctx, portalID)

// Lines 492-498 - Proper indexing
existingByPath := make(map[string]state.PortalPage)
existingByID := make(map[string]state.PortalPage)
for _, page := range existingPages {
    existingByID[page.ID] = page
}
```

### Execution Phase Failure (Why It Fails)

1. **Loses Portal Context**: BaseExecutor.validateResourceForUpdate() uses generic GetByName()
2. **Can't Use Hierarchy**: No parent context available to GetByName()
3. **Interface Limitation**: ResourceOperations interface assumes name-only lookup suffices
4. **Fallback Restricted**: GetByID fallback exists but only for protection changes

## Working Infrastructure Already Exists

### Available State Client Methods
```go
// state/client.go - These methods exist and work
func (c *Client) GetPortalPage(ctx context.Context, portalID, pageID string) (*PortalPage, error)
func (c *Client) ListManagedPortalPages(ctx context.Context, portalID string) ([]PortalPage, error)
```

### Context Extraction Pattern
```go
// portal_page_adapter.go:177-192 - This pattern works
func (p *PortalPageAdapter) getPortalID(ctx context.Context) (string, error) {
    change := ctx.Value(contextKeyPlannedChange).(planner.PlannedChange)
    return change.References["portal_id"].ID, nil
}
```

### Legacy Working Code
The deprecated `updatePortalPage()` in portal_child_operations.go:395-489 shows the working pattern:
- Gets both portal ID and page ID (lines 403-429)
- Uses proper API calls (line 484)
- Handles reference resolution correctly

## Comparison with Other Child Resources

### Similar Affected Resources

All these adapters return `(nil, nil)` from GetByName():

1. **PortalSnippetAdapter**: "Portal snippets are looked up by the planner from the list"
2. **APIDocumentAdapter**: "API documents don't have a direct 'get by name' method"

### Working Parent Resources

Resources that work have meaningful GetByName implementations:

1. **APIAdapter**: Has `GetByID()` method and name-based lookup
2. **PortalAdapter**: Can be looked up by name at top level

## Decision Points Leading to Error

### Critical Decision Tree
```
BaseExecutor.validateResourceForUpdate()
├── Strategy 1: GetByName() → Returns (nil, nil) ❌
├── Strategy 2: GetByID() fallback → isProtectionChange() check ❌
│   ├── If protection change → Would work ✅  
│   └── If regular update → Skipped ❌
└── Strategy 3: Namespace fallback → Also protection-only ❌

Result: return nil → "portal_page no longer exists" error
```

### The Artificial Restriction
```go
// base_executor.go:216 - THE PROBLEM
if change.ResourceID != "" && isProtectionChange(change) {
//                            ^^^^^^^^^^^^^^^^^^^^^^^ 
//                            This restriction causes the bug
```

## Recommended Solutions

### Solution 1: Remove Protection Change Restriction (Minimal Change)
```go
// In base_executor.go:216, change from:
if change.ResourceID != "" && isProtectionChange(change) {

// To:
if change.ResourceID != "" {
```

### Solution 2: Implement GetByID Methods (Complete Solution)
```go
// Add to PortalPageAdapter
func (p *PortalPageAdapter) GetByID(ctx context.Context, id string) (ResourceInfo, error) {
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

### Solution 3: Both (Recommended)
Implement both solutions for robust coverage:
1. Remove the artificial restriction for immediate fix
2. Add GetByID methods for proper architecture

## Impact Analysis

### Currently Failing Operations
- All portal page UPDATE operations
- All portal snippet UPDATE operations  
- All API document UPDATE operations
- Any child resource UPDATE operation

### Currently Working Operations
- CREATE operations (no validation needed)
- DELETE operations (different validation path)
- UPDATE operations for parent resources (APIs, portals)
- Protection changes (use GetByID fallback)

## Test Scenarios

### Failure Scenario
```yaml
# portal.yaml
portals:
  - name: "test-portal"
    pages:
      - slug: "getting-started"
        content: "Original content"

# Modify content and apply again
portals:
  - name: "test-portal"
    pages:
      - slug: "getting-started"
        content: "Updated content"  # This fails with "portal_page no longer exists"
```

### Success After Fix
The same scenario should work with either solution implemented.

## Related Files Summary

### Core Issue Files
- `internal/declarative/executor/base_executor.go:216` - Artificial restriction
- `internal/declarative/executor/portal_page_adapter.go:155` - Missing GetByID
- `internal/declarative/executor/executor.go:928-950` - Update orchestration

### Supporting Infrastructure  
- `internal/declarative/planner/portal_child_planner.go:466` - Working planning logic
- `internal/declarative/state/client.go:1471` - GetPortalPage method
- `internal/declarative/executor/portal_child_operations.go:395` - Legacy working code

### Pattern Files (Same Issue)
- `internal/declarative/executor/portal_snippet_adapter.go` - Same GetByName pattern
- `internal/declarative/executor/api_document_adapter.go:122` - Same GetByName pattern

## Conclusion

The portal page update failure is a well-defined architectural issue with a clear fix path. The existing BaseExecutor already has the correct fallback mechanism, but it's artificially restricted to protection changes only. 

The recommended approach is to:
1. **Immediate fix**: Remove the protection change restriction in BaseExecutor
2. **Proper solution**: Implement GetByID methods for all child resource adapters
3. **Testing**: Verify that all child resource updates work correctly

This solution leverages existing infrastructure and maintains architectural consistency while fixing the core validation issue.