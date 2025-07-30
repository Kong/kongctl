# Flow Report: API Publication Creation Execution Path

## Executive Summary

This report traces the complete execution flow for API publication creation from the `sync` command through to the error "API ID is required for publication operations". The analysis reveals a critical timing issue in reference resolution when API publications are defined at the root level (extracted) rather than nested within API resources.

## Command Entry Point

### 1. Sync Command Initialization
**File**: `/internal/cmd/root/verbs/sync/sync.go`
- The `sync` command delegates to the konnect command's RunE function
- Sets up context with verb="sync" and product="konnect"

### 2. Konnect Command Routing
**File**: `/internal/cmd/root/products/konnect/konnect.go`
- For sync verb, routes to `declarative.NewDeclarativeCmd()`
- Returns a command configured for declarative operations

### 3. Declarative Sync Command
**File**: `/internal/cmd/root/products/konnect/declarative/declarative.go`
- `runSync()` function (line 1033) orchestrates the sync operation
- Key steps:
  1. Loads configuration files via `loader.LoadFromSources()`
  2. Creates planner with `planner.NewPlanner()`
  3. Generates plan with `p.GeneratePlan()` using `PlanModeSync`
  4. Creates executor with `executor.New()`
  5. Executes plan with `exec.Execute()`

## Resource Loading Phase

### 4. Configuration Loader
**File**: `/internal/declarative/loader/loader.go`
- `LoadFromSources()` loads and parses YAML files
- API publications are loaded as separate resources into `target.APIPublications` (line 584)
- Resources are stored in `ResourceSet` with APIs and APIPublications as separate lists

## Planning Phase

### 5. Plan Generation
**File**: `/internal/declarative/planner/planner.go`
- `GeneratePlan()` orchestrates planning for all resource types
- Calls specialized planning functions for each resource type

### 6. API Publication Planning
**File**: `/internal/declarative/planner/api_planner.go`
- `planAPIPublicationsChanges()` (line 1259) processes extracted API publications:
  1. Groups publications by parent API ref
  2. For each API group:
     - Searches planned changes for the API
     - If API is being created (ActionCreate), sets up dependency but passes empty API ID
     - If API exists, tries to get ID from `api.GetKonnectID()`
  3. Calls `planAPIPublicationCreate()` (line 752) which:
     - Creates a PlannedChange with References["api_id"] containing:
       - Ref: The API reference
       - ID: The API ID (may be empty)
       - LookupFields: Contains API name for resolution

## Execution Phase

### 7. Plan Execution
**File**: `/internal/declarative/executor/executor.go`
- `Execute()` (line 124) processes changes in `plan.ExecutionOrder`
- For each change, calls `executeChange()`
- `executeChange()` routes to `createResource()` for CREATE actions

### 8. Resource Creation
**File**: `/internal/declarative/executor/executor.go`
- `createResource()` (line 574) handles api_publication creation:
  1. Checks if `change.References["api_id"].ID` is empty (line 614)
  2. If empty, calls `resolveAPIRef()` to find the API
  3. Updates the reference with resolved ID
  4. Passes change to `apiPublicationExecutor.Create()`

### 9. API Reference Resolution
**File**: `/internal/declarative/executor/executor.go`
- `resolveAPIRef()` (line 464):
  1. Checks `e.refToID["api"][refInfo.Ref]` for recently created APIs
  2. If not found, uses `lookupValue` (preferring name from LookupFields)
  3. Calls `GetAPIByName()` to find API in Konnect
  4. Returns error if API not found

### 10. API Publication Adapter
**File**: `/internal/declarative/executor/api_publication_adapter.go`
- `Create()` method calls `getAPIID()` (line 140)
- `getAPIID()`:
  1. Retrieves PlannedChange from context
  2. Checks `change.References["api_id"].ID`
  3. Returns "API ID is required for publication operations" if empty

## Resource ID Tracking

### 11. Reference Tracking
**File**: `/internal/declarative/executor/executor.go`
- After successful resource creation (line 277):
  - Stores ID in `e.createdResources[change.ID]`
  - Stores ID in `e.refToID[change.ResourceType][change.ResourceRef]` (line 284)
- This allows subsequent resources to resolve references to created resources

## Issue Analysis

### Root Cause
The error occurs when:
1. API publications are defined at root level (extracted)
2. The parent API is new (being created in the same plan)
3. The API hasn't been created in Konnect yet when the publication tries to resolve it

### Execution Flow Problem
1. Planner sets up correct dependencies (publication depends on API)
2. Executor respects dependencies in execution order
3. However, when the publication executes:
   - The API may have been created but the reference still has empty ID
   - `resolveAPIRef()` fails because the API doesn't exist in Konnect yet
   - The timing gap between API creation and publication reference resolution causes failure

### Critical Gap
The issue is in the reference resolution timing:
- When API is ActionCreate, the planner passes empty API ID
- The executor's `resolveAPIRef()` can't find the API because:
  - It hasn't been created in Konnect yet (if execution hasn't reached it)
  - OR it was just created but GetAPIByName might have timing/caching issues

## Potential Solutions

### 1. Enhanced Dependency Resolution
- Ensure API creation completes and ID is available before publication execution
- Update reference IDs dynamically after dependent resource creation

### 2. Pre-Planning Resource Resolution
- Implement a resolution phase that populates `konnectID` for all existing resources
- This would allow `GetKonnectID()` to return proper IDs during planning

### 3. Improved Reference Propagation
- After creating an API, update all pending changes that reference it
- Propagate the created ID to dependent resources before their execution

### 4. Retry Mechanism
- Add retry logic in `resolveAPIRef()` for recently created resources
- Account for eventual consistency in Konnect API

## Conclusion

The API publication creation failure stems from a timing issue in reference resolution when publications are defined separately from their parent APIs. The system correctly tracks dependencies but fails to properly propagate created resource IDs to dependent resources during execution. This particularly affects scenarios where both the API and its publications are being created in the same sync operation.