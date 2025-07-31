# Investigation Report: API Publication Creation Failure

## Issue Summary
API Publications are failing with the error "API ID is required for publication operations" during sync operations in kongctl. This error occurs when creating api_publication resources, even though APIs are created successfully before publications attempt.

## Root Cause
The issue is a mismatch between how the planner sets up API parent references and how the executor expects to receive them:

1. **Planner Behavior** (`planAPIPublicationCreate` in `api_planner.go`):
   - Sets the parent API information in the `Parent` field of the PlannedChange
   - Does NOT create an "api_id" reference in the `References` map

2. **Executor Expectation** (`createResource` in `executor.go`):
   - Looks for `change.References["api_id"]` to get the API ID
   - Falls back to resolving the API reference if the ID is empty
   - Never checks the `Parent` field

3. **Adapter Implementation** (`APIPublicationAdapter.getAPIID`):
   - Only checks `change.References["api_id"]`
   - Returns the error "API ID is required for publication operations" when not found
   - Comment indicates "Parent field fallback removed" (line 167)

## Detailed Analysis

### 1. Planner Sets Parent but Not Reference
In `planAPIPublicationCreate` (api_planner.go:656-689):
```go
parentInfo := &ParentInfo{Ref: apiRef}
if apiID != "" {
    parentInfo.ID = apiID
}

change := PlannedChange{
    // ...
    Parent: parentInfo,  // Parent is set here
    // ...
}
// References map is NOT populated with api_id
```

### 2. Executor Expects api_id Reference
In `createResource` for api_publication (executor.go:667-688):
```go
case "api_publication":
    // First resolve API reference if needed
    if apiRef, ok := change.References["api_id"]; ok && apiRef.ID == "" {
        // ... resolution logic
    }
```

### 3. Adapter Cannot Find API ID
In `APIPublicationAdapter.getAPIID` (api_publication_adapter.go:145-172):
```go
// Get API ID from references
if apiRef, ok := change.References["api_id"]; ok {
    if apiRef.ID != "" {
        return apiRef.ID, nil
    }
}
// No fallback to Parent field
return "", fmt.Errorf("API ID is required for publication operations")
```

### 4. Reference Resolver Limitation
The reference resolver (`resolver.go`) only knows how to map certain field names to resource types:
```go
func (r *ReferenceResolver) getResourceTypeForField(fieldName string) string {
    switch fieldName {
    case "portal_id":
        return ResourceTypePortal
    // No case for "api_id"
    }
}
```

## Why Portal References Work but API References Don't

Portal references work because:
1. The portal_id is stored as a field in the APIPublicationResource
2. The reference resolver knows how to map "portal_id" to portal resources
3. The executor properly resolves portal references

API references fail because:
1. The API reference is only stored in the Parent field, not as a field
2. There's no "api_id" field that the resolver can process
3. The executor expects an "api_id" reference that was never created

## Solution Options

### Option 1: Fix the Planner (Recommended)
Modify `planAPIPublicationCreate` to add an api_id reference:
```go
change := PlannedChange{
    // ... existing fields ...
    References: map[string]ReferenceInfo{
        "api_id": {
            Ref: apiRef,
            ID: apiID,
        },
    },
}
```

### Option 2: Fix the Adapter
Modify `APIPublicationAdapter.getAPIID` to check the Parent field:
```go
// Check Parent field as fallback
if change.Parent != nil && change.Parent.ID != "" {
    return change.Parent.ID, nil
}
```

### Option 3: Fix the Resolver
Add "api_id" to the reference resolver's field mappings and ensure the planner sets it as a field.

## Recommended Fix
Option 1 is recommended because:
- It aligns with how other child resources (api_version, api_document) work
- It maintains consistency with the executor's expectations
- It requires minimal changes and doesn't break existing patterns

## Impact
This issue affects all API publication operations:
- Creating new API publications
- Syncing configurations with API publications
- Any workflow that involves publishing APIs to portals

## Testing Considerations
After fixing, ensure to test:
1. Creating API publications when the API already exists
2. Creating API publications when the API is created in the same plan
3. Syncing configurations with existing API publications
4. Deleting API publications (which also uses the API ID)