# Flow Report: API Publication Creation Failure

## Executive Summary

The "API ID is required for publication operations" error occurs because the PlannedChange object is stored in the context **before** API references are resolved, causing the APIPublicationAdapter to receive an outdated change object with empty API IDs.

## Complete Execution Flow

### 1. Sync Command Entry Point
```
sync.go:runSync() 
  → declarative.go:runSync()
    → loader.LoadFromSources() - Load YAML configs
    → planner.GeneratePlan() - Create execution plan
    → executor.Execute() - Execute the plan
```

### 2. Planning Phase - API Publication Creation

**File**: `api_planner.go:planAPIPublicationCreate()` (lines 752-831)

The planner creates a PlannedChange for API publication with:
- Parent field set with API reference and ID (if known)
- References map populated with:
  - `"api_id"`: Contains API ref, ID (may be empty), and lookup fields
  - `"portal_id"`: Contains portal ref and lookup fields

```go
// Line 811-818
change.References["api_id"] = ReferenceInfo{
    Ref: apiRef,
    ID:  apiID, // May be empty if API doesn't exist yet
    LookupFields: map[string]string{
        "name": apiName,
    },
}
```

### 3. Execution Phase - Resource Creation

**File**: `executor.go:createResource()` (lines 606-760)

Critical sequence:
1. **Line 608**: PlannedChange stored in context (WITH EMPTY API ID)
   ```go
   ctx = context.WithValue(ctx, contextKeyPlannedChange, *change)
   ```

2. **Lines 669-677**: API reference resolution for api_publication
   ```go
   if apiRef, ok := change.References["api_id"]; ok && apiRef.ID == "" {
       apiID, err := e.resolveAPIRef(ctx, apiRef)
       // ...
       apiRef.ID = apiID
       change.References["api_id"] = apiRef  // Updates local change only!
   }
   ```

3. **Line 688**: Calls executor with updated change
   ```go
   return e.apiPublicationExecutor.Create(ctx, *change)
   ```

### 4. Adapter Phase - API ID Retrieval Failure

**File**: `api_publication_adapter.go:getAPIID()` (lines 145-172)

The adapter retrieves the PlannedChange from context:
```go
change, ok := ctx.Value(contextKeyPlannedChange).(planner.PlannedChange)
```

**Problem**: This retrieves the ORIGINAL change stored at line 608, NOT the updated one with resolved references!

## Data Flow Analysis

### Successful Path (When API Already Exists)
1. Planner sets `api_id` reference with actual ID
2. Context stores change with populated ID
3. No resolution needed in executor
4. Adapter finds ID in reference → Success

### Failed Path (When API Created in Same Plan)
1. Planner sets `api_id` reference with empty ID
2. Context stores change with empty ID ← **ISSUE HERE**
3. Executor resolves reference, updates local change
4. Context still has original change with empty ID
5. Adapter retrieves from context → Finds empty ID → **FAILURE**

## Reference Resolution Mechanism

The system has a two-stage reference resolution:

1. **Planning Stage**: Sets up references with known IDs or placeholders
2. **Execution Stage**: Resolves references for newly created resources

The executor's `resolveAPIRef()` method successfully finds the API ID from previously executed changes, but this resolved ID never makes it to the adapter due to the context timing issue.

## Root Cause

The root cause is a timing issue in `executor.go:createResource()`:

```go
// Line 608 - Context set too early
ctx = context.WithValue(ctx, contextKeyPlannedChange, *change)

// Lines 669-677 - References resolved AFTER context is set
if apiRef, ok := change.References["api_id"]; ok && apiRef.ID == "" {
    // Resolution happens here, but context already has old change
}
```

## Solution

The context should be updated AFTER reference resolution:

```go
// Resolve all references first
switch change.ResourceType {
case "api_publication":
    // Resolve references...
    if apiRef, ok := change.References["api_id"]; ok && apiRef.ID == "" {
        apiID, err := e.resolveAPIRef(ctx, apiRef)
        // Update reference
    }
    // Update context with resolved change
    ctx = context.WithValue(ctx, contextKeyPlannedChange, *change)
    return e.apiPublicationExecutor.Create(ctx, *change)
}
```

## Impact Analysis

This issue affects all child resources that depend on parent resources created in the same plan:
- API Publications (depends on API)
- API Versions (depends on API)
- API Documents (depends on API)
- Portal Pages with parent pages
- Any resource with cross-references

## Testing Recommendations

1. Test creating API and publishing in same plan
2. Test updating existing API publication
3. Test with multiple APIs and publications
4. Verify other child resources don't have similar issues