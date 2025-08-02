# Investigation Report: API Publications Failure Issue

## Executive Summary

API Publications fail when created in the same plan as APIs and portals because the reference resolver overwrites the entire References map, discarding the `api_id` reference that was correctly set by the planner. While the propagation mechanism works correctly, the API publication never receives the `api_id` reference to propagate to.

## Problem Description

When creating API publications alongside their parent APIs in the same plan:
- **First run**: API publications fail with "API ID is required for publication operations"
- **Second run**: API publications succeed when APIs/portal already exist

## Root Cause Analysis

### 1. The Reference Overwriting Bug

The core issue lies in how references are handled between planning and execution:

1. **Plan Creation Phase** (in `api_planner.go`):
   - The planner correctly creates both references:
     - `change.References["api_id"] = ReferenceInfo{Ref: "sms", ID: "", ...}`
     - `change.References["portal_id"] = ReferenceInfo{Ref: "getting-started", ID: "", ...}`
   - Also adds `portal_id` to Fields: `fields["portal_id"] = publication.PortalID`

2. **Reference Resolution Phase** (in `planner.go` line 198-217):
   - The ReferenceResolver only looks in the `Fields` map, not the `References` map
   - It finds `portal_id` in Fields but not `api_id` (which is only in References)
   - **Line 207 creates a new empty References map**: `basePlan.Changes[i].References = make(map[string]ReferenceInfo)`
   - This discards all existing references!
   - Only `portal_id` is restored from resolver results

3. **Execution Phase** (in `executor.go`):
   - API publication has only 1 reference (`portal_id`) instead of 2
   - The `api_id` reference is completely missing
   - APIPublicationAdapter fails with "API ID is required"

### 2. Why the Propagation Mechanism Couldn't Help

The executor has a working propagation mechanism (lines 289-317 in `executor.go`) that correctly updates pending changes with created resource IDs:
- After creating an API, it stores `refToID["api"]["sms"] = "api-id"`
- It propagates this ID to subsequent changes that reference it
- **However**, it can only propagate to references that exist!
- Since the `api_id` reference was already discarded by the resolver, there's nothing to propagate to

### 3. The Investigation Process

Through extensive debugging, we discovered:
1. The planner creates 2 references correctly (verified with debug output)
2. During execution, API publications only have 1 reference
3. The propagation mechanism works but has no `api_id` reference to update
4. The reference was lost between planning and execution

## Why It Works on Second Run

On the second run:
- APIs and portals already exist in Konnect
- The `resolveAPIRef` function successfully finds the API by name lookup
- The reference is populated with the found ID
- API publication creation proceeds successfully

## Key Code Locations

1. **Plan Creation**: `internal/declarative/planner/api_planner.go`
   - `planAPIPublicationCreate` (line 752): Sets up references with potentially empty IDs

2. **Execution**: `internal/declarative/executor/executor.go`
   - `createResource` (line 673-696): Attempts to resolve references before execution
   - `resolveAPIRef` (line 496): Resolution logic that fails for not-yet-created resources

3. **API Publication Adapter**: `internal/declarative/executor/api_publication_adapter.go`
   - `getAPIID` (line 140): Expects resolved API ID, returns error when not found

## Technical Details

### Execution Flow for API Publications

1. **Planning Phase**:
   ```
   planAPIPublicationCreate() -> Sets References["api_id"] = {Ref: "api-ref", ID: "", LookupFields: {...}}
   ```

2. **Execution Phase**:
   ```
   createResource() -> resolveAPIRef() -> Checks refToID (empty) -> Lookup by name (fails) -> Error
   ```

3. **Expected Flow**:
   ```
   Create API -> Update refToID -> Propagate to dependent changes -> Create API Publication
   ```

4. **Actual Flow**:
   ```
   Try to create API Publication -> Need API ID -> API not created yet -> Fail
   ```

## Impact

This issue affects any scenario where:
- API publications are defined in the same configuration as their parent APIs
- Resources with parent-child relationships are created in the same plan
- The child resource requires the parent's ID for creation

## The Fix

The fix was simple - modify `planner.go` line 207 to preserve existing references:

```go
// Instead of: basePlan.Changes[i].References = make(map[string]ReferenceInfo)
// Preserve existing references and merge with resolver results
if basePlan.Changes[i].References == nil {
    basePlan.Changes[i].References = make(map[string]ReferenceInfo)
}
```

This ensures that references set by the planner are preserved and merged with any references found by the resolver.

## Conclusion

The root cause was not a timing issue or propagation failure, but a simple bug where the reference resolver was discarding existing references by creating a new empty map. The fix preserves all references, allowing the propagation mechanism to work as designed.