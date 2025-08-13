# Executor Context Anti-Pattern Refactoring Plan

## Problem Statement
The executor package abuses context.WithValue to pass namespace, protection, and PlannedChange data through the call stack. This creates:
- Hidden dependencies (30+ context.WithValue calls in createResource and updateResource)
- Loss of type safety (unsafe type assertions in every adapter)
- Difficult testing and maintenance
- No compile-time checking of required parameters

## Solution
Create an `ExecutionContext` struct to explicitly pass required data, eliminating all context value storage in the executor package.

## Implementation Steps

### 1. Create ExecutionContext struct
**File**: `internal/declarative/executor/execution_context.go`
```go
package executor

import "github.com/kong/kongctl/internal/declarative/planner"

// ExecutionContext carries execution state that was previously stored in context
type ExecutionContext struct {
    Namespace     string
    Protection    interface{} // Will match current usage pattern
    PlannedChange *planner.PlannedChange
}
```

### 2. Update adapter interfaces
Change method signatures to accept ExecutionContext explicitly:
- `MapCreateFields(ctx context.Context, execCtx *ExecutionContext, fields map[string]interface{}, create *T) error`
- `MapUpdateFields(ctx context.Context, execCtx *ExecutionContext, fields map[string]interface{}, update *T, currentLabels map[string]string) error`

### 3. Update executor.go
- Remove all context.WithValue calls (30+ instances)
- Create ExecutionContext once at method entry points
- Pass ExecutionContext explicitly to all adapter calls

### 4. Update base_executor.go
- Remove context key constants:
  - `contextKeyNamespace`
  - `contextKeyProtection`
  - `contextKeyPlannedChange`

### 5. Update all adapters
**Files to modify**:
- portal_adapter.go
- api_adapter.go
- auth_strategy_adapter.go
- api_version_adapter.go
- api_publication_adapter.go
- api_document_adapter.go
- portal_page_adapter.go
- portal_snippet_adapter.go
- portal_domain_adapter.go

**Changes for each adapter**:
- Accept ExecutionContext parameter in MapCreateFields and MapUpdateFields
- Replace `ctx.Value(contextKeyNamespace).(string)` with `execCtx.Namespace`
- Replace `ctx.Value(contextKeyProtection)` with `execCtx.Protection`
- Replace `ctx.Value(contextKeyPlannedChange).(planner.PlannedChange)` with `*execCtx.PlannedChange`

## Benefits
- **Type safety**: No more unsafe type assertions
- **Explicit dependencies**: Function signatures show what's needed
- **Testability**: Easy to create ExecutionContext for tests
- **Performance**: No runtime type assertions
- **Maintainability**: Clear data flow

## Quality Checks
After implementation:
1. `make build` - Ensure compilation succeeds
2. `make lint` - Verify no linting issues
3. `make test` - Confirm all tests pass

## Expected Changes
- **Lines removed**: ~30 context.WithValue calls
- **Type assertions removed**: ~15 unsafe type assertions
- **Function signatures**: 11 adapter files updated with explicit dependencies
- **No behavior changes**: Pure refactoring maintaining existing functionality

## Implementation Status
- [ ] Plan created
- [ ] ExecutionContext struct created
- [ ] executor.go updated
- [ ] base_executor.go updated
- [ ] All adapters updated
- [ ] Quality checks passed
- [ ] Changes committed

## Summary
This refactoring eliminates the context anti-pattern in the executor package by replacing hidden context values with explicit parameter passing. The change improves type safety, testability, and maintainability while preserving all existing behavior.