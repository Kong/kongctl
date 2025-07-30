# Planner Code Duplication Analysis and Refactoring Plan

## Overview

This document analyzes code duplication patterns in the planner layer and
provides a refactoring strategy to reduce duplication and improve
maintainability.

## Identified Duplication Patterns

### 1. Create Operation Pattern (High Impact)

All resource planners implement nearly identical create methods:

**Current Pattern:**
- Create empty fields map
- Map resource fields to the map
- Create PlannedChange with standard fields
- Handle protection, namespace, and labels identically

**Files Affected:**
- `api_planner.go`: `planAPICreate()`
- `portal_planner.go`: `planPortalCreate()`
- `auth_strategy_planner.go`: `planAuthStrategyCreate()`

**Duplication Score:** ~90% identical code across implementations

### 2. Update Operation Pattern (High Impact)

The `shouldUpdate*` and `plan*UpdateWithFields` methods follow identical
patterns:

**Current Pattern:**
- Create updates map
- Compare each field only if present in desired
- Handle label comparison with same logic
- Pass current labels for removal handling

**Files Affected:**
- `api_planner.go`: `shouldUpdateAPI()`, `planAPIUpdateWithFields()`
- `portal_planner.go`: `shouldUpdatePortal()`, `planPortalUpdateWithFields()`
- `auth_strategy_planner.go`: `shouldUpdateAuthStrategy()`,
  `planAuthStrategyUpdateWithFields()`

**Duplication Score:** ~85% identical code

### 3. Protection Change Pattern (Medium Impact)

Protection change methods are nearly identical:

**Current Pattern:**
- Include field updates when unprotecting
- Create PlannedChange with ProtectionChange
- Extract namespace from kongctl meta

**Files Affected:**
- `api_planner.go`: `planAPIProtectionChangeWithFields()`
- `portal_planner.go`: `planPortalProtectionChangeWithFields()`
- `auth_strategy_planner.go`: `planAuthStrategyProtectionChangeWithFields()`

**Duplication Score:** ~95% identical code

### 4. Delete Operation Pattern (Medium Impact)

Delete methods follow the same pattern:

**Current Pattern:**
- Create PlannedChange with DELETE action
- Extract namespace from labels or use default
- Include name in fields

**Files Affected:**
- `api_planner.go`: `planAPIDelete()`
- `portal_planner.go`: `planPortalDelete()`
- `auth_strategy_planner.go`: `planAuthStrategyDelete()`

**Duplication Score:** ~90% identical code

### 5. Label Handling Pattern (Medium Impact)

Label handling is repeated across all planners:

**Current Pattern:**
- Convert label maps to interface{} maps
- Compare user labels for changes
- Pass current labels for removal handling

**Duplication Score:** ~100% identical logic

## Refactoring Solution

### Generic Operations Implementation

Created `generic_operations.go` with:

1. **GenericPlanner** - Provides common planning operations
2. **Configuration structs** - Type-safe configuration for each operation:
   - `GenericCreateConfig`
   - `GenericUpdateConfig`
   - `GenericProtectionChangeConfig`
   - `GenericDeleteConfig`
3. **Generic methods**:
   - `PlanGenericCreate()` - Handles all create operations
   - `PlanGenericUpdate()` - Handles all update operations
   - `PlanGenericProtectionChange()` - Handles protection changes
   - `PlanGenericDelete()` - Handles all delete operations
   - `CompareLabels()` - Generic label comparison
   - `CompareLabelsWithPointers()` - For portal resources

### Migration Strategy

1. **Phase 1: Add Generic Operations**
   - ✅ Create `generic_operations.go`
   - ✅ Add generic planner to main Planner struct
   - ✅ Expose via BasePlanner

2. **Phase 2: Incremental Migration**
   - Start with one resource type (recommend API)
   - Replace method internals to use generic operations
   - Keep existing method signatures for compatibility
   - Test thoroughly before moving to next resource

3. **Phase 3: Complete Migration**
   - Migrate all resource planners
   - Remove duplicated helper methods
   - Update tests

### Example Refactoring

See `api_planner_refactored_example.go` for examples of how to refactor
existing methods to use generic operations.

**Before (api_planner.go):**
```go
func (p *Planner) planAPICreate(api resources.APIResource, plan *Plan) string {
    fields := make(map[string]interface{})
    fields["name"] = api.Name
    if api.Description != nil {
        fields["description"] = *api.Description
    }
    // ... 40+ lines of boilerplate ...
}
```

**After:**
```go
func (p *Planner) planAPICreate(api resources.APIResource, plan *Plan) string {
    fields := make(map[string]interface{})
    if api.Description != nil {
        fields["description"] = *api.Description
    }
    
    return p.genericPlanner.PlanGenericCreate(GenericCreateConfig{
        ResourceType: "api",
        ResourceRef:  api.GetRef(),
        Name:         api.Name,
        Fields:       fields,
        Labels:       api.Labels,
        Kongctl:      api.Kongctl,
        DependsOn:    []string{},
    }, plan)
}
```

## Benefits

1. **Code Reduction**: ~60-70% reduction in planner code
2. **Consistency**: All resources follow exact same patterns
3. **Maintainability**: Bug fixes and improvements in one place
4. **Testability**: Can test generic operations once
5. **Type Safety**: Configuration structs ensure correct usage

## Implementation Priority

1. **High Priority** (Most duplication, highest impact):
   - Create operations
   - Update operations
   - Label handling

2. **Medium Priority**:
   - Protection change operations
   - Delete operations

3. **Lower Priority**:
   - Child resource planning (more unique per resource type)

## Testing Strategy

1. Create comprehensive tests for generic operations
2. Ensure existing planner tests pass after refactoring
3. Add integration tests to verify end-to-end behavior
4. Use table-driven tests for generic operations

## Notes

- The generic operations maintain exact same behavior as current code
- Resource-specific logic (field mapping) stays in resource planners
- Generic operations handle all boilerplate and common patterns
- This follows the same pattern successfully used in executor refactoring