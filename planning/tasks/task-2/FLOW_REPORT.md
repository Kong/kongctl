# Flow Report: API Resource Processing in Sync Operations

## Executive Summary

This report documents the complete code flow for how API resources are processed during sync operations, 
from YAML parsing to plan generation. The analysis reveals a critical bug in the `filterResourcesByNamespace` 
function that prevents API child resources (versions, publications) from being included in the execution plan.

## Problem Statement

When running the sync command with portal and API YAML files, the plan summary shows APIs being created but 
no API versions or publications are planned, despite these being defined in the YAML configuration.

## Code Flow Analysis

### 1. Entry Point: Sync Command

**File**: `/internal/cmd/root/verbs/sync/sync.go`
- The sync command delegates to konnect implementation via `konnect.NewKonnectCmd(Verb)`

**File**: `/internal/cmd/root/products/konnect/konnect.go`
- For sync verb, creates declarative command: `declarative.NewDeclarativeCmd(verbs.Sync)`

### 2. Declarative Sync Command Execution

**File**: `/internal/cmd/root/products/konnect/declarative/declarative.go`

#### Flow in `runSync()` (line 1033):
1. Parse command flags and build helper
2. Load configuration from YAML files:
   ```go
   ldr := loader.New()
   resourceSet, err := ldr.LoadFromSources(sources, recursive) // line 1130
   ```
3. Create planner and generate plan:
   ```go
   p := planner.NewPlanner(stateClient, logger)
   plan, err := p.GeneratePlan(ctx, resourceSet, opts) // line 1172
   ```

### 3. YAML Loading and Resource Extraction

**File**: `/internal/declarative/loader/loader.go`

#### Key Functions:
- `LoadFromSources()` (line 55): Entry point for loading YAML files
- `parseYAML()` (line 183): Parses individual YAML files
- `extractNestedResources()` (line 892): **Critical function** that extracts child resources

#### Resource Extraction Process:
```go
// Extract nested API child resources (line 894)
for i := range rs.APIs {
    api := &rs.APIs[i]
    
    // Extract versions
    for j := range api.Versions {
        version := api.Versions[j]
        version.API = api.Ref // Set parent reference
        rs.APIVersions = append(rs.APIVersions, version)
    }
    
    // Extract publications
    for j := range api.Publications {
        publication := api.Publications[j]
        publication.API = api.Ref // Set parent reference
        rs.APIPublications = append(rs.APIPublications, publication)
    }
    
    // Clear nested resources from API
    api.Versions = nil
    api.Publications = nil
}
```

**Key Point**: Child resources are correctly extracted and parent references are set using the API's `Ref` 
field (e.g., "sms"), not the `Name` field (e.g., "SMS API").

### 4. Plan Generation

**File**: `/internal/declarative/planner/planner.go`

#### Flow in `GeneratePlan()` (line 81):
1. Extract namespaces from resources
2. For each namespace:
   - Filter resources by namespace: `filterResourcesByNamespace(rs, namespace)` (line 138)
   - Store filtered resources in namespace planner (lines 142-152)
   - Generate changes via resource planners

### 5. The Bug: filterResourcesByNamespace

**File**: `/internal/declarative/planner/planner.go` (line 516)

```go
func (p *Planner) filterResourcesByNamespace(rs *resources.ResourceSet, namespace string) *resources.ResourceSet {
    // ... filter parent resources ...
    
    // Build parent reference maps
    portalRefs := make(map[string]bool)
    for _, portal := range filtered.Portals {
        portalRefs[portal.Ref] = true  // Correctly uses Ref
    }
    
    apiNames := make(map[string]bool)
    for _, api := range filtered.APIs {
        apiNames[api.Name] = true  // BUG: Uses Name instead of Ref!
    }
    
    // Filter child resources based on parent presence
    for _, version := range rs.APIVersions {
        if apiNames[version.API] {  // version.API contains Ref, not Name!
            filtered.APIVersions = append(filtered.APIVersions, version)
        }
    }
}
```

**The Bug**: 
- `apiNames` map is populated with API names (e.g., "SMS API")
- Child resources reference parent by ref (e.g., version.API = "sms")
- The lookup `apiNames[version.API]` fails because "sms" != "SMS API"
- Result: All API child resources are filtered out

### 6. API Planning Without Child Resources

**File**: `/internal/declarative/planner/api_planner.go`

The API planner correctly attempts to plan child resources:
```go
func (a *apiPlannerImpl) PlanChanges(ctx context.Context, plan *Plan) error {
    // Plan API resources
    if err := a.planner.planAPIChanges(ctx, desiredAPIs, plan); err != nil {
        return err
    }
    
    // Plan child resources - but these lists are empty due to the bug!
    if err := a.planner.planAPIVersionsChanges(ctx, a.GetDesiredAPIVersions(), plan); err != nil {
        return err
    }
    if err := a.planner.planAPIPublicationsChanges(ctx, a.GetDesiredAPIPublications(), plan); err != nil {
        return err
    }
}
```

The getter methods return empty lists because filterResourcesByNamespace incorrectly filtered out all child 
resources:
```go
func (b *BasePlanner) GetDesiredAPIVersions() []resources.APIVersionResource {
    return b.planner.desiredAPIVersions  // Empty due to filtering bug
}
```

## Impact Analysis

### Current Behavior:
1. APIs are created successfully
2. API versions and publications are ignored
3. No error is reported to the user
4. The API exists but has no versions or portal publications

### Expected Behavior:
1. APIs are created
2. API versions are created as child resources
3. API publications link the versions to portals
4. Complete API structure is established

## Fix Location

The fix needs to be applied in `/internal/declarative/planner/planner.go` at line 545:

```go
// Change from:
apiNames := make(map[string]bool)
for _, api := range filtered.APIs {
    apiNames[api.Name] = true
}

// To:
apiRefs := make(map[string]bool)
for _, api := range filtered.APIs {
    apiRefs[api.Ref] = true
}
```

And update all subsequent references from `apiNames` to `apiRefs`.

## Integration Points

### Correct Integration Pattern (Portal Resources):
The portal filtering correctly uses refs:
```go
portalRefs := make(map[string]bool)
for _, portal := range filtered.Portals {
    portalRefs[portal.Ref] = true
}
```

### Failed Integration Pattern (API Resources):
The API filtering incorrectly mixes names and refs, breaking the parent-child relationship.

## Conclusion

The sync command has a complete and correct pipeline for processing API resources and their child resources. 
The loader properly extracts nested resources and sets parent references. The planner has the correct logic 
to handle child resources. However, a single bug in `filterResourcesByNamespace` breaks this entire flow by 
filtering out all API child resources before they can be planned.

This is a critical bug that prevents the declarative configuration system from properly managing API versions 
and publications, which are essential components of the API lifecycle in Kong Konnect.