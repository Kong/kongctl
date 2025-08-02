# Investigation Report: API Publication Creation Failing with "API ID is required"

## Problem Statement

API publication creation is failing with the error "API ID is required for publication 
operations" during sync execution. This issue occurs when API publications are defined 
at the root level (extracted) rather than nested within API resources.

## Root Cause Analysis

### 1. Error Origin

The error originates from `api_publication_adapter.go` at line 154 in the `getAPIID` 
function:

```go
func (a *APIPublicationAdapter) getAPIID(ctx context.Context) (string, error) {
    // Get the planned change from context
    change, ok := ctx.Value(contextKeyPlannedChange).(planner.PlannedChange)
    if !ok {
        return "", fmt.Errorf("planned change not found in context")
    }

    // Get API ID from references
    if apiRef, ok := change.References["api_id"]; ok {
        if apiRef.ID != "" {
            return apiRef.ID, nil
        }
    }

    return "", fmt.Errorf("API ID is required for publication operations")
}
```

### 2. Reference Resolution Flow

The system follows this flow for API publication creation:

1. **Planning Phase** (`api_planner.go`):
   - `planAPIPublicationCreate` (line 752) sets up references correctly
   - Creates reference with API ref, ID (may be empty), and lookup fields

2. **Execution Phase** (`executor.go`):
   - Lines 614-621 attempt to resolve API reference if ID is empty
   - Calls `resolveAPIRef` which tries to find the API by name

3. **Adapter Phase** (`api_publication_adapter.go`):
   - Expects the API ID to be available in the reference
   - Fails if reference ID is still empty

### 3. Issue with Extracted Publications

The problem occurs specifically with extracted (root-level) API publications processed 
by `planAPIPublicationsChanges` (line 1259):

```go
// For each API, plan publication changes
for apiRef, publications := range publicationsByAPI {
    // Find the API ID from existing changes or state
    apiID := ""
    for _, change := range plan.Changes {
        if change.ResourceType == "api" && change.ResourceRef == apiRef {
            if change.Action == ActionCreate {
                // API is being created, use dependency
                for _, pub := range publications {
                    p.planAPIPublicationCreate(apiRef, "", pub, []string{change.ID}, plan)
                }
                continue // <-- Skips further processing
            }
            apiID = change.ResourceID
            break
        }
    }

    // If API not in changes, use the resolved ID from pre-resolution phase
    if apiID == "" {
        // Find the API resource by ref to get its resolved ID
        for _, api := range p.GetDesiredAPIs() {
            if api.GetRef() == apiRef {
                resolvedID := api.GetKonnectID()
                if resolvedID != "" {
                    apiID = resolvedID
                }
                break
            }
        }
    }
```

### 4. Key Findings

1. **Task-2 Fix Applied**: The filtering issue from task-2 has been fixed. The code 
   now correctly uses `apiRefs` instead of `apiNames` in `filterResourcesByNamespace`.

2. **Resolution Timing Issue**: The problem occurs when:
   - API publications are defined at root level (extracted)
   - The parent API doesn't exist in Konnect yet
   - The `GetKonnectID()` method returns empty because resources haven't been resolved

3. **Dependency Handling**: When the API is being created (`ActionCreate`), the code 
   correctly sets up dependencies but passes an empty API ID. The executor should 
   resolve this during execution, but the resolution might fail if:
   - The API hasn't been created yet in the execution order
   - The API name lookup fails
   - There's a race condition in the reference resolution

### 5. Specific Scenarios

The error occurs in these scenarios:

1. **New API with Publications**: Creating a new API with publications defined at 
   root level (not nested)
2. **Existing API Not in Plan**: API exists in Konnect but isn't being modified, 
   and publications are defined separately
3. **Resolution Failure**: The `GetKonnectID()` returns empty, indicating the 
   resource resolution phase hasn't populated the ID

## Impact

This issue prevents proper declarative management of API publications when they are 
defined at the root level, which is the recommended pattern for managing complex 
configurations with cross-references.

## Potential Solutions

### 1. Improve Resource Resolution (Recommended)

Ensure all resources are properly resolved before planning:
- Add a pre-planning resolution phase that populates `konnectID` for all resources
- Fetch existing resources from Konnect and match them with desired resources
- Populate the `konnectID` field before planning begins

### 2. Enhanced Reference Resolution in Executor

Improve the executor's ability to resolve references:
- If API reference resolution fails, check if the API is being created in the same plan
- Wait for dependent resources to be created before proceeding
- Add retry logic for reference resolution

### 3. Fallback in Adapter

Add a fallback mechanism in the adapter:
- If API ID is not in references, check the parent field
- Try to resolve using the API ref/name directly
- Provide more detailed error messages

## Related Code Locations

1. **Planner**: `/internal/declarative/planner/api_planner.go`
   - `planAPIPublicationsChanges` (line 1259)
   - `planAPIPublicationCreate` (line 752)

2. **Executor**: `/internal/declarative/executor/executor.go`
   - API publication creation handling (lines 612-633)
   - `resolveAPIRef` function (line 464)

3. **Adapter**: `/internal/declarative/executor/api_publication_adapter.go`
   - `getAPIID` function (line 140)
   - `Create` function (line 61)

4. **Resources**: `/internal/declarative/resources/api.go`
   - `GetKonnectID` method (line 79)

## Next Steps

1. Investigate how and when the `konnectID` field is populated in resources
2. Add logging to trace the reference resolution flow
3. Implement a proper resource resolution phase before planning
4. Test with various scenarios (new API, existing API, mixed configurations)