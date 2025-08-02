# Flow Report: API Publication Creation Failure Analysis

## Executive Summary

This report traces the complete execution flow that leads to API publication creation failures when APIs and publications are created in the same plan. The root cause is that the reference resolver overwrites the References map, discarding the `api_id` reference before execution begins.

## High-Level Flow Diagram

```
┌─────────────────────────────────────────────────────────────────────────┐
│                            PLANNING PHASE                                │
├─────────────────────────────────────────────────────────────────────────┤
│ 1. api_planner.go:planAPIPublicationCreate()                           │
│    ├─> Creates PlannedChange with TWO references:                      │
│    │   • References["api_id"] = {Ref: "sms", ID: "", ...}             │
│    │   • References["portal_id"] = {Ref: "getting-started", ID: "",...}│
│    └─> Also adds to Fields: fields["portal_id"] = "getting-started"    │
│                                                                         │
│ 2. planner.go:ResolveReferences()                                      │
│    ├─> ReferenceResolver looks ONLY in Fields map                      │
│    ├─> Finds portal_id in Fields                                       │
│    └─> Does NOT see api_id (only in References, not Fields)           │
│                                                                         │
│ 3. planner.go line 207: THE BUG                                        │
│    ├─> Creates NEW empty References map                                │
│    ├─> Discards existing api_id reference!                            │
│    └─> Only restores portal_id from resolver results                   │
└─────────────────────────────────────────────────────────────────────────┘
                                    ↓
┌─────────────────────────────────────────────────────────────────────────┐
│                           EXECUTION PHASE                                │
├─────────────────────────────────────────────────────────────────────────┤
│ 4. executor.go:Execute() - Processes changes in order                  │
│                                                                         │
│    For API Creation:                                                    │
│    ├─> createResource() → Success                                      │
│    ├─> Stores ID in refToID["api"]["sms"] = "api-id"                 │
│    └─> Tries to propagate but api_id reference is GONE!               │
│                                                                         │
│    For API Publication Creation:                                        │
│    ├─> Has only 1 reference: portal_id                                │
│    ├─> No api_id reference to resolve or propagate to                 │
│    └─> APIPublicationAdapter fails: "API ID is required"              │
└─────────────────────────────────────────────────────────────────────────┘
```

## Detailed Execution Flow

### 1. Planning Phase (api_planner.go)

```go
// planAPIPublicationCreate (line 752)
func planAPIPublicationCreate(...) {
    // Line 813: Sets up reference with potentially empty ID
    change.References["api_id"] = ReferenceInfo{
        Ref: apiRef,
        ID:  apiID,  // Empty string if API doesn't exist yet
        LookupFields: map[string]string{"name": apiName},
    }
}
```

**Key Point**: The planner correctly sets up the reference structure but with an empty ID, expecting it to be resolved during execution.

### 2. Dependency Resolution

The dependency resolver correctly orders operations:
1. Create API (e.g., "1:c:api:my-api")
2. Create API Publication (e.g., "2:c:api_publication:my-api-to-portal")

### 3. Execution Phase - The Critical Path

#### 3.1 API Creation (SUCCESS)
```
executor.go:Execute()
  └─> executeChange() [line 240]
      └─> createResource() [line 246]
          └─> apiExecutor.Create()
              └─> Returns: api-id = "12345"
          └─> SUCCESS: Stores in refToID["api"]["my-api"] = "12345"
          └─> Propagates to pending changes [lines 289-317]
```

#### 3.2 API Publication Creation (FAILURE)
```
executor.go:Execute()
  └─> executeChange() [line 240]
      └─> createResource() [line 246] for api_publication
          ├─> Lines 675-682: if apiRef.ID == "" then resolveAPIRef()
          │   └─> resolveAPIRef() [line 496]
          │       ├─> Check refToID["api"]["my-api"] - EMPTY!
          │       │   (Propagation hasn't happened yet)
          │       └─> Try GetAPIByName("my-api") - NOT FOUND!
          │           (API just created, not available yet)
          │       └─> RETURNS ERROR
          └─> ERROR: "failed to resolve API reference"
              (Never reaches APIPublicationAdapter)
```

### 4. Debug Output Analysis

Our debug output revealed the critical insight:

**During Planning:**
```
[DEBUG] API Publication references: 2 references
[DEBUG]   api_id: ref='sms', id=''
[DEBUG]   portal_id: ref='getting-started', id=''
```

**During Execution:**
```
[DEBUG] Found matching change: type=api_publication, ref=sms-api-to-getting-started
[DEBUG] Change has 1 references
[DEBUG] Checking reference: key=portal_id, refType=portal, ref=getting-started
```

Only 1 reference remained! The `api_id` reference was lost.

1. **Reference Resolution Happens Too Early**: The `createResource` function for API publications attempts to resolve the API reference BEFORE checking if it's being created in the same plan.

2. **Propagation Happens Too Late**: The reference propagation mechanism (lines 289-317) only runs AFTER a resource is successfully created, but the API publication needs the reference BEFORE it can be created.

3. **Circular Dependency**: 
   - API publication creation requires resolved API ID
   - API ID only becomes available after API creation
   - But resolution is attempted before propagation occurs

## Why It Works on Second Run

On the second run:
1. API already exists in Konnect
2. `resolveAPIRef` → `GetAPIByName()` succeeds
3. Reference gets populated with found ID
4. API publication creation proceeds normally

## Code Flow Sequence

```
Time →
T0: Plan contains API and API Publication changes (correctly ordered)
T1: Execute API creation
T2: API created successfully, ID stored in refToID
T3: Execute API Publication creation
T4: ❌ Attempt to resolve API reference (TOO EARLY!)
T5: ❌ refToID check fails (propagation not done yet)
T6: ❌ Konnect lookup fails (API just created)
T7: ❌ Error returned, execution stops
T8: (Never reached) Propagation would have updated references
```

## Key Code Locations

### Planning
- `api_planner.go:752-831` - planAPIPublicationCreate
  - Line 813: Sets reference with empty ID

### Execution
- `executor.go:240-324` - executeChange main flow
  - Line 246: Calls createResource
  - Lines 289-317: Propagation mechanism (too late)

- `executor.go:673-696` - API publication creation path
  - Lines 675-682: Premature reference resolution

- `executor.go:496-544` - resolveAPIRef
  - Line 499: Checks refToID (fails)
  - Line 524: Tries Konnect lookup (fails)

### Adapter
- `api_publication_adapter.go:140-165` - getAPIID
  - Line 149: Expects resolved ID, returns error

## Design Flaw

The fundamental design flaw is that the executor assumes all references can be resolved at the time of resource creation, but this assumption breaks when:
1. Resources are created in the same plan
2. Child resources require parent IDs
3. The parent hasn't been fully processed yet

The propagation mechanism exists but executes in the wrong order relative to when the reference resolution is needed.

## Recommendations

To fix this issue, one of the following approaches could be taken:

1. **Deferred Resolution**: Allow references to remain unresolved until the actual SDK call, letting the adapter handle resolution.

2. **Pre-execution Propagation**: Run a propagation pass after each successful creation but before the next change execution.

3. **Lazy Resolution**: Only resolve references when actually needed by the adapter, not preemptively.

4. **Two-Phase Execution**: First phase creates all resources, second phase resolves all references.

The current execution model's assumption that all references must be resolved before resource creation is incompatible with creating related resources in a single plan.