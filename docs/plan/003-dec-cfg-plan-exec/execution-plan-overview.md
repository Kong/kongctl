# Stage 3: Plan Execution - Technical Overview

## Overview

Stage 3 implements the execution phase of the declarative configuration workflow,
taking plans generated in Stage 2 and applying them to Konnect. This stage
introduces mode-aware plan generation and separate `apply` and `sync` commands
with distinct behaviors.

## Key Design Concepts

### Mode-Aware Plan Generation

Plans are generated differently based on the intended execution mode:

- **Apply Mode**: Generates plans containing only CREATE and UPDATE operations
- **Sync Mode**: Generates plans including CREATE, UPDATE, and DELETE operations

Both modes detect protected resources during plan generation:
- Protected resources that would be updated or deleted are marked as "blocked"
- Blocked changes are included in the plan with clear explanations
- Plans remain valid and executable despite containing blocked changes
- Blocked changes are skipped during execution with appropriate reporting

This distinction ensures plans are optimized for their intended use and prevents
accidental deletions when using the safer `apply` command.

### Command Separation

Two distinct commands provide different levels of state management:

1. **`kongctl apply`**: Safe incremental updates
   - Creates new resources
   - Updates existing managed resources
   - Never deletes resources
   - Ideal for production changes

2. **`kongctl sync`**: Full state reconciliation
   - Creates new resources
   - Updates existing managed resources
   - Deletes managed resources not in configuration
   - Ideal for CI/CD and environment replication

## Architecture Components

### Enhanced Planner (Stage 2 Extension)

The existing planner from Stage 2 requires enhancement to support mode-aware
generation:

```go
type PlanMode string

const (
    PlanModeSync  PlanMode = "sync"
    PlanModeApply PlanMode = "apply"
)

type PlannerOptions struct {
    Mode PlanMode // Determines which operations to include
}
```

### Plan Executor

Central component responsible for executing plans:

```go
type Executor struct {
    client   *state.KonnectClient
    reporter ProgressReporter
    dryRun   bool
}

type ExecutionResult struct {
    SuccessCount int
    FailureCount int
    SkippedCount int
    Errors       []ExecutionError
}
```

### Progress Reporting

Real-time feedback during execution:

```go
type ProgressReporter interface {
    StartExecution(plan *Plan)
    StartChange(change PlannedChange)
    CompleteChange(change PlannedChange, err error)
    FinishExecution(result *ExecutionResult)
}
```

## Command Flow

### Apply Command Flow

```
1. Load configuration files
2. Generate plan in apply mode (CREATE/UPDATE only)
3. Display plan summary
4. Confirm execution (unless --auto-approve)
5. Execute plan with progress reporting
6. Update resource labels with management metadata
7. Report results
```

### Sync Command Flow

```
1. Load configuration files
2. Generate plan in sync mode (CREATE/UPDATE/DELETE)
3. Display plan summary with DELETE warnings
4. Confirm execution with extra safety for DELETEs
5. Execute plan with progress reporting
6. Update/remove resource labels as appropriate
7. Report results
```

### Plan File Usage

Both commands support pre-generated plan files:

```bash
# Generate plan first
kongctl plan --mode=apply -o apply-plan.json

# Execute later
kongctl apply --plan apply-plan.json
```

## Safety Mechanisms

### Plan Validation

- Apply command rejects plans containing DELETE operations
- Sync command warns when using apply-mode plans
- Plan metadata includes mode for validation

### Protected Resources

Resources marked with `kongctl.protected: true` are fully immutable:
- Cannot be updated or deleted while protected
- Changes are blocked during plan generation with clear explanations
- Require explicit two-phase modification process:
  1. First: Update resource to set `protected: false`
  2. Then: Apply desired changes (update or delete)
- Blocked changes are included in plans but marked as non-executable
- Execution reports show blocked changes separately from failures

### Confirmation Prompts

- Apply: Confirms if updates will modify existing resources
- Sync: Always confirms if DELETE operations are present
- Both: Support `--auto-approve` to skip confirmations

## Error Handling

### Execution Strategies

1. **Fail-Fast** (default): Stop on first error
2. **Best-Effort**: Continue despite errors, report all at end

### Rollback Considerations

- No automatic rollback (follows Terraform model)
- Failed executions leave partial state
- Subsequent runs can complete remaining changes
- Clear error reporting for manual intervention

## Integration Points

### With Existing Components

- Extends Stage 2 planner with mode support
- Reuses loader from Stage 1 for configuration reading
- Leverages state package for Konnect API operations
- Maintains label management patterns from Stage 2

### Konnect-First Login Migration

As part of Stage 3, the login command will be migrated to follow the
Konnect-first pattern:

```bash
# Current
kongctl login konnect

# New (Konnect-first)
kongctl login
```

## Performance Considerations

### Parallel Execution

Where possible, independent operations execute in parallel:
- Resources without dependencies
- Same-type resources
- Respecting API rate limits

### Progress Reporting

- Real-time updates without buffering
- Minimal overhead on execution
- Clear indication of current operation

## Testing Strategy

### Unit Tests

- Executor logic with mocked clients
- Plan validation rules
- Error handling scenarios
- Progress reporting

### Integration Tests

- Full command execution with mock SDK
- Apply mode verification (no DELETEs)
- Sync mode with all operations
- Plan file loading and validation
- Protected resource handling

## Future Considerations

### Stage 4 Preparation

- Executor designed to handle multiple resource types
- Dependency resolution supports complex relationships
- Progress reporting scales to many operations

### Potential Enhancements

- Parallel execution optimization
- Plan diff visualization
- Execution history tracking
- Partial execution resumption