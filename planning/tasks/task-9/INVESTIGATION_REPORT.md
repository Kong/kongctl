# Investigation Report: API Version Limitation Issue

## Executive Summary

The investigation reveals that kongctl's declarative configuration allows users to define multiple API versions, but Konnect's API restricts each API to having only a single version instance. This constraint is not validated during the planning phase, resulting in a runtime error during the apply phase with the message: "At most one api specification can be provided for a api".

## Problem Statement

When users define multiple API versions in their declarative configuration and attempt to apply it, the operation fails during execution rather than being caught during planning or validation. This creates a poor user experience as users invest time creating configurations that will inevitably fail.

## Investigation Findings

### 1. Declarative Configuration Structure

The `APIResource` struct in `internal/declarative/resources/api.go` contains a `Versions` field that is a slice, explicitly allowing multiple versions:

```go
type APIResource struct {
    // ... other fields ...
    Versions []APIVersionResource `yaml:"versions,omitempty" json:"versions,omitempty"`
    // ... other fields ...
}
```

### 2. Test Cases Demonstrate Multiple Versions

Test files show examples of APIs with multiple versions defined:

In `test/integration/declarative/comprehensive_multi_resource_test.go`:
```yaml
versions:
  - ref: users-api-v1
    name: "v1"
    spec: !file ./specs/users-api-spec.json
  - ref: users-api-v2
    name: "v2"
    spec: !file ./specs/users-api-spec.json
```

### 3. Planner Behavior

The planner in `internal/declarative/planner/api_planner.go` processes all versions without validation:

- `planAPIChildResourcesCreate()` (line 401) iterates through all versions and creates a plan change for each
- `planAPIVersionChanges()` (line 459) compares desired versions with current state and plans CREATE operations for missing versions
- No validation exists to check if an API already has a version

### 4. Executor Implementation

The executor attempts to create all planned versions:

- `internal/declarative/executor/api_version_adapter.go` implements the creation logic
- `CreateAPIVersion()` in `internal/declarative/state/client.go` makes the actual API call
- No pre-execution validation checks for existing versions

### 5. Validation Gap

The validator in `internal/declarative/loader/validator.go`:
- Only checks for duplicate refs within the configuration
- Does not validate the Konnect constraint of one version per API
- No warning or error is generated for multiple versions

### 6. Error Source

The error message "At most one api specification can be provided for a api" originates from Konnect's API, not from kongctl. This was confirmed by:
- No occurrence of this error message in the kongctl codebase
- The error format matches Konnect API error responses

## Root Cause Analysis

1. **Design Mismatch**: The declarative configuration design allows for multiple versions (slice type), but Konnect's business logic restricts APIs to a single version.

2. **Missing Validation**: No validation exists at any layer (parsing, planning, or pre-execution) to enforce Konnect's single-version constraint.

3. **Late Error Detection**: The constraint is only enforced by Konnect's API during the actual creation attempt, resulting in a poor user experience.

## Impact

1. **User Experience**: Users can spend time crafting complex configurations with multiple API versions, only to have them fail during apply.

2. **Partial Application**: If an API has multiple versions defined, the first version might be created successfully before the second fails, leaving the system in a partially applied state.

3. **Confusion**: The error message from Konnect's API may not be immediately clear to users about what "api specification" means in this context.

## Affected Code Paths

1. **Resource Definition**: `internal/declarative/resources/api.go` - Allows multiple versions
2. **Planning**: `internal/declarative/planner/api_planner.go` - Plans all versions without validation
3. **Validation**: `internal/declarative/loader/validator.go` - Missing constraint validation
4. **Execution**: `internal/declarative/executor/api_version_adapter.go` - Attempts all version creations

## Recommendations

1. **Add Validation During Loading**: Modify the validator to check that each API has at most one version defined.

2. **Improve Error Messages**: If validation is added at the planning stage, provide clear error messages explaining the constraint.

3. **Consider Design Change**: Either:
   - Change the `Versions` field from a slice to a single `Version` field (breaking change)
   - Keep the slice but enforce single-element constraint with clear documentation

4. **Add Documentation**: Clearly document this Konnect limitation in the user documentation and examples.

5. **Add Pre-execution Validation**: Before executing the plan, validate that no API will have more than one version after all changes are applied.

## Conclusion

The issue stems from a fundamental mismatch between kongctl's flexible declarative design and Konnect's business constraints. The solution requires adding validation at appropriate stages to catch this constraint violation early, improving the user experience by failing fast with clear error messages.