# Flow Report: API Version Limitation Issue

## Executive Summary

This report traces the complete execution flow for the API version limitation issue where kongctl allows multiple API versions during planning but fails during apply with the error: "At most one api specification can be provided for a api". The analysis reveals that the constraint is enforced by Konnect's API, not by kongctl, resulting in late error detection during execution rather than early validation during planning.

## Flow Analysis

### 1. Loading Declarative Configuration Files

#### Entry Point: `loader.LoadFromSources()`
- **File**: `internal/declarative/loader/loader.go`
- **Key Function**: `LoadFromSources()` (line 55)

The loading process:
1. Parses YAML files containing declarative configuration
2. The `APIResource` struct explicitly supports multiple versions:
   ```go
   type APIResource struct {
       Versions []APIVersionResource `yaml:"versions,omitempty"`
   }
   ```
3. Nested API versions are extracted to root level during loading:
   - `extractNestedResources()` (line 892) processes nested structures
   - `planAPIVersionChanges()` extracts versions from APIs (lines 895-922)
   - Sets parent references (`version.API = api.Ref`)

**Issue**: The data structure design inherently supports multiple versions per API.

### 2. Validation During Loading

#### Validation Function: `validateResourceSet()`
- **File**: `internal/declarative/loader/validator.go`
- **Key Function**: `validateAPIs()` (line 159)

The validation process:
1. Validates individual resource fields
2. Checks for duplicate refs within the configuration
3. Validates cross-references between resources
4. **MISSING**: No validation for Konnect's single-version constraint

Example from validation (lines 196-206):
```go
// Validate nested versions
for j := range api.Versions {
    version := &api.Versions[j]
    if err := version.Validate(); err != nil {
        return fmt.Errorf("invalid api_version %q in api %q: %w", version.GetRef(), api.GetRef(), err)
    }
    if versionRefs[version.GetRef()] {
        return fmt.Errorf("duplicate api_version ref: %s", version.GetRef())
    }
    versionRefs[version.GetRef()] = true
}
```

**Issue**: Only validates refs are unique, not that each API has at most one version.

### 3. Plan Command Execution

#### Command Handler: `runPlan()`
- **File**: `internal/cmd/root/products/konnect/declarative/declarative.go`
- **Function**: `runPlan()` (line 78)

The planning process:
1. Loads configuration using the loader
2. Creates a planner instance
3. Calls `p.GeneratePlan(ctx, resourceSet, opts)`
4. No validation of API version constraints

### 4. Planning Phase

#### Planner Implementation: `planAPIChanges()`
- **File**: `internal/declarative/planner/api_planner.go`
- **Function**: `planAPIChanges()` (line 58)

The planning flow for APIs:
1. Fetches current APIs from Konnect
2. Compares desired vs current state
3. For new APIs, calls `planAPIChildResourcesCreate()` (line 401)
4. For existing APIs, calls `planAPIChildResourceChanges()` (line 424)

#### Version Planning: `planAPIVersionChanges()`
- **Function**: `planAPIVersionChanges()` (line 459)

The version planning logic:
```go
// Compare desired versions
for _, desiredVersion := range desired {
    versionStr := ""
    if desiredVersion.Version != nil {
        versionStr = *desiredVersion.Version
    }

    if _, exists := currentByVersion[versionStr]; !exists {
        // CREATE - versions don't support update
        p.planAPIVersionCreate(parentNamespace, apiRef, apiID, desiredVersion, []string{}, plan)
    }
}
```

**Issue**: No check if the API already has a version before planning to create another.

#### Version Creation Planning: `planAPIVersionCreate()`
- **Function**: `planAPIVersionCreate()` (line 536)

Creates a `PlannedChange` for each version without validation:
```go
change := PlannedChange{
    ID:           p.nextChangeID(ActionCreate, "api_version", version.GetRef()),
    ResourceType: "api_version",
    ResourceRef:  version.GetRef(),
    Parent:       parentInfo,
    Action:       ActionCreate,
    Fields:       fields,
    DependsOn:    dependsOn,
    Namespace:    parentNamespace,
}
```

### 5. Apply Command Execution

#### Command Handler: `runApply()`
- **File**: `internal/cmd/root/products/konnect/declarative/declarative.go`
- **Function**: `runApply()` (line 641)

The apply process:
1. Loads or uses existing plan
2. Creates executor instance
3. Calls `exec.Execute(ctx, plan)`

### 6. Execution Phase

#### Executor: `Execute()`
- **File**: `internal/declarative/executor/executor.go`
- **Function**: `Execute()` (line 89)

Execution flow:
1. Processes changes in dependency order
2. For each change, calls `executeChange()`
3. For CREATE actions, calls `createResource()`

#### API Version Creation: `APIVersionAdapter`
- **File**: `internal/declarative/executor/api_version_adapter.go`
- **Function**: `Create()` (line 44)

The adapter:
1. Maps fields to `CreateAPIVersionRequest`
2. Gets API ID from context
3. Calls `client.CreateAPIVersion()`

### 7. API Call to Konnect

#### State Client: `CreateAPIVersion()`
- **File**: `internal/declarative/state/client.go`
- **Function**: `CreateAPIVersion()` (line 619)

The final API call:
```go
resp, err := c.apiVersionAPI.CreateAPIVersion(ctx, apiID, version)
if err != nil {
    return nil, fmt.Errorf("failed to create API version: %w", err)
}
```

**This is where Konnect's API returns the error**: "At most one api specification can be provided for a api"

### 8. Error Handling

#### Error Recording
- **File**: `internal/declarative/executor/executor.go`
- **Function**: `executeChange()` (line 257)

When the API call fails:
```go
if err != nil {
    execError := ExecutionError{
        ChangeID:     change.ID,
        ResourceType: change.ResourceType,
        ResourceName: resourceName,
        ResourceRef:  change.ResourceRef,
        Action:       string(change.Action),
        Error:        err.Error(),
    }
    result.Errors = append(result.Errors, execError)
    result.FailureCount++
}
```

## Execution Flow Diagram

```
1. User creates config.yaml with multiple API versions
   ↓
2. kongctl plan -f config.yaml
   ├─→ Loader: Parses YAML, extracts nested versions
   ├─→ Validator: Only checks ref uniqueness
   └─→ Planner: Creates PlannedChange for each version
   ↓
3. Plan generated successfully (no validation errors)
   ↓
4. kongctl apply --plan plan.json
   ├─→ Executor: Processes changes in order
   ├─→ APIVersionAdapter: Maps fields
   ├─→ StateClient: Calls Konnect API
   └─→ Konnect API: Returns error
   ↓
5. Error: "At most one api specification can be provided for a api"
```

## Key Decision Points

### 1. Data Structure Design
- **Location**: `internal/declarative/resources/api.go`
- **Decision**: Use slice for versions allowing multiple
- **Impact**: Enables invalid configurations

### 2. Missing Validation
- **Location**: `internal/declarative/loader/validator.go`
- **Decision**: No business rule validation
- **Impact**: Invalid configurations pass validation

### 3. Planning Without Constraints
- **Location**: `internal/declarative/planner/api_planner.go`
- **Decision**: Create changes for all versions
- **Impact**: Invalid plans are generated

### 4. Late Error Detection
- **Location**: Konnect API call
- **Decision**: Rely on API validation
- **Impact**: Poor user experience

## Dependencies and Ordering

### Resource Creation Order
1. API must be created before its versions
2. Planner correctly sets dependencies:
   - Version changes depend on API change ID
   - Executor respects dependency order

### Reference Resolution
- Version creation needs API ID
- If API is new, ID is resolved after API creation
- Executor propagates created IDs to dependent changes

## Root Cause Summary

The issue stems from a fundamental mismatch between:
1. **kongctl's design**: Supports multiple versions per API
2. **Konnect's constraint**: Allows only one version per API

This mismatch is not validated at any point before execution:
- Loader allows multiple versions
- Validator doesn't check the constraint
- Planner creates changes for all versions
- Only Konnect's API enforces the constraint

## Recommendations

1. **Add Validation in Loader**: Check that each API has at most one version
2. **Add Validation in Planner**: Verify constraint before creating changes
3. **Improve Error Messages**: Provide clear guidance when constraint is violated
4. **Consider Design Change**: Either support only single version or document limitation clearly