# Planner Context Refactoring

## Overview
This refactoring addresses the context anti-pattern in the planner package where `context.WithValue` is abused to pass namespace data through the call stack. This follows the same pattern as the successful executor context refactoring.

## Problem Statement

### Context Anti-Pattern Usage
The planner package currently uses `context.WithValue` to pass namespace information:

1. **planner.go:166** - Sets namespace in context:
   ```go
   plannerCtx := context.WithValue(ctx, NamespaceContextKey, actualNamespace)
   ```

2. **Multiple planners extract namespace from context** (5 locations):
   - api_planner.go (2 times)
   - portal_planner.go (1 time) 
   - auth_strategy_planner.go (1 time)
   - portal_child_planner.go (1 time)

### Issues with Current Approach
- **Runtime type assertions**: `ctx.Value(NamespaceContextKey).(string)`
- **Hidden dependencies**: Functions don't declare namespace dependency
- **Type safety**: No compile-time guarantees
- **Testing complexity**: Requires context setup in tests
- **Maintenance burden**: Easy to forget context setup

## Solution Approach

### Create PlannerContext Struct
Replace context values with explicit struct containing:
- Namespace field (typed as string)
- Future extensibility for additional planner data

### Update Interface Signatures
Modify `ResourcePlanner.PlanChanges` to accept `PlannerContext` parameter alongside the existing context and plan parameters.

### Type-Safe Parameter Passing
Remove all `context.WithValue` and `ctx.Value()` calls, replacing with explicit struct field access.

## Implementation Plan

### Phase 1: Create Core Infrastructure
1. Create `planner_context.go` with `PlannerContext` struct
2. Update `interfaces.go` to modify `ResourcePlanner.PlanChanges` signature
3. Remove context key definitions from `planner.go`

### Phase 2: Update All Planners
4. Modify `planner.go` to create `PlannerContext` instead of using `context.WithValue`
5. Update all planner implementations:
   - `auth_strategy_planner.go`
   - `portal_planner.go` 
   - `api_planner.go`
   - `portal_child_planner.go`

### Phase 3: Quality Assurance
6. Run quality checks (build, lint, test)
7. Commit changes

## Expected Benefits

### Type Safety
- Compile-time namespace field validation
- No runtime type assertions
- Clear dependency declaration in method signatures

### Code Quality
- Explicit parameter passing
- Easier testing (no context setup required)
- Better IDE support and refactoring safety

### Consistency
- Aligns with executor context refactoring pattern
- Consistent architectural approach across codebase

## Files to Modify

### Core Files
- `internal/declarative/planner/planner_context.go` (NEW)
- `internal/declarative/planner/interfaces.go`
- `internal/declarative/planner/planner.go`

### Planner Implementations  
- `internal/declarative/planner/auth_strategy_planner.go`
- `internal/declarative/planner/portal_planner.go`
- `internal/declarative/planner/api_planner.go`
- `internal/declarative/planner/portal_child_planner.go`

## Risk Assessment

### Low Risk
- Following proven pattern from executor refactoring
- Single namespace field simplifies implementation
- Clear interface boundaries

### Mitigations
- Incremental implementation with quality checks
- Follows established refactoring pattern
- No test file modifications (as requested)

## Success Criteria
- All context.WithValue and ctx.Value calls removed
- All quality checks pass (build, lint, test)
- Type-safe namespace parameter passing
- No functional behavior changes