# Stage 3: Plan Execution - Architecture Decision Records

## ADR-003-001: Mode-Aware Plan Generation

### Context
The plan generation phase needs to support two distinct execution modes:
- Apply: Safe incremental updates (CREATE/UPDATE only)
- Sync: Full reconciliation (CREATE/UPDATE/DELETE)

### Decision
Implement mode-aware plan generation where the planner accepts a mode parameter
that determines which operations to include in the generated plan.

### Consequences
- Plans are optimized for their intended use case
- Apply mode plans are smaller without DELETE operations
- Plan metadata must indicate generation mode
- Planner logic becomes slightly more complex

### Implementation
```go
type PlannerOptions struct {
    Mode PlanMode // "apply" or "sync"
}

// Plan metadata includes mode
type PlanMetadata struct {
    GeneratedAt string    `json:"generated_at"`
    Version     string    `json:"version"`
    Mode        PlanMode  `json:"mode"`
    ConfigHash  string    `json:"config_hash"`
}
```

---

## ADR-003-002: Separate Apply and Sync Commands

### Context
Users need both safe incremental updates and full state reconciliation, but
these represent fundamentally different risk profiles and use cases.

### Decision
Implement two separate commands with distinct behaviors:
- `kongctl apply`: CREATE/UPDATE only, safe for production
- `kongctl sync`: Full reconciliation including DELETE

### Consequences
- Clear separation of concerns
- Reduced risk of accidental deletions
- Commands can have different default behaviors
- Some code duplication between commands

### Rationale
Following the principle of least surprise, separating destructive and
non-destructive operations into different commands provides better safety
and clearer intent.

---

## ADR-003-003: Plan Validation and Mode Compatibility

### Context
Commands need to validate that the plan they're executing matches their
intended behavior to prevent accidents.

### Decision
- Apply command rejects plans containing DELETE operations
- Sync command accepts both plan types but warns for apply-mode plans
- Validation happens before any execution begins

### Consequences
- Strong safety guarantees
- Clear error messages for mismatched plans
- Plans are not universally interchangeable

### Example
```go
func validatePlanCompatibility(plan *Plan, expectedMode PlanMode) error {
    if expectedMode == PlanModeApply && plan.containsDeletes() {
        return fmt.Errorf("apply command cannot execute plans with DELETE operations")
    }
    return nil
}
```

---

## ADR-003-004: Protected Resource Immutability

### Context
Protected resources represent critical infrastructure that should not be
modified or deleted without explicit intent. Both UPDATE and DELETE operations
on protected resources carry risk and should be prevented.

### Decision
Implement comprehensive protection mechanisms:
1. Protected resources cannot be updated or deleted
2. Protection status detected during state retrieval
3. Plan generation includes "blocked" changes with clear reasons
4. Two-phase process required for any modifications
5. Execution skips blocked changes with appropriate reporting

### Consequences
- Complete immutability for protected resources
- Clear visibility of blocked operations during planning
- Explicit unprotection required before any changes
- Reduced risk of accidental modifications to critical resources

### Protection Modification Flow
```yaml
# Phase 1: Remove protection (only change allowed)
kongctl:
  protected: false  # Changed from true

# Phase 2: Make desired changes (update or delete)
# Resource can now be modified in subsequent operations
```

### Blocked Change Handling
```json
{
  "change_id": "portal-production-update",
  "action": "UPDATE",
  "resource_type": "portal",
  "resource_name": "production-portal",
  "blocked": true,
  "block_reason": "Resource is protected. Remove protection before updating."
}
```

---

## ADR-003-005: Executor Error Handling Strategy

### Context
Plan execution can fail at any operation, and we need a consistent strategy
for handling partial failures.

### Decision
- Default to fail-fast behavior (stop on first error)
- No automatic rollback of successful operations
- Clear error reporting with operation context
- Future: Best-effort mode as optional flag

### Consequences
- Predictable behavior on errors
- Partial state possible after failures
- Re-running can complete remaining operations
- Aligns with tools like Terraform

### Rationale
Automatic rollback is complex and can cause more issues than it solves.
Clear error reporting allows users to make informed decisions about recovery.

---

## ADR-003-006: Konnect-First Login Command Migration

### Context
Current login command requires explicit "konnect" product specification:
`kongctl login konnect`. This conflicts with our Konnect-first approach
used in declarative commands.

### Decision
Migrate login to be Konnect-first:
- `kongctl login` defaults to Konnect login
- Future products can be `kongctl login --product gateway`
- Maintain backward compatibility during transition

### Consequences
- Consistent with declarative command patterns
- Simpler default usage
- May require deprecation period for old syntax

---

## ADR-003-007: Confirmation Prompt and Output Format Patterns

### Context
Both apply and sync commands need consistent user interfaces for confirmation
and output formatting. Commands should work well in both interactive and
automated contexts.

### Decision
Implement consistent behavior across both commands:
1. Unified confirmation prompt (type 'yes' to proceed)
2. Clear plan summary before confirmation
3. Support `--auto-approve` flag for automation
4. Respect `--output` flag for structured output (json, yaml)
5. Default to human-readable text format

### Consequences
- Consistent user experience across commands
- Automation-friendly with structured output
- Clear visibility of changes before execution
- Works well in CI/CD pipelines

### Example Interactive Flow
```
# Both apply and sync show plan summary
Plan Summary:
- Create: 3 resources
- Update: 2 resources  
- Delete: 1 resource (sync only)
- Blocked: 2 resources (protected)

WARNING: This operation will DELETE resources:
- portal: staging-portal

Do you want to continue? Type 'yes' to confirm: yes
```

### Example Automated Flow
```bash
# With auto-approve and JSON output
kongctl sync --auto-approve --output json

# Output:
{
  "execution_result": {
    "success_count": 5,
    "failure_count": 0,
    "skipped_count": 0,
    "blocked_count": 2,
    "errors": []
  }
}
```

---

## ADR-003-008: Executor Architecture

### Context
The executor needs to handle multiple operation types, provide progress
feedback, and integrate with existing components.

### Decision
Single executor implementation that:
- Accepts any valid plan regardless of mode
- Delegates to operation-specific methods
- Uses strategy pattern for progress reporting
- Maintains stateless execution

### Consequences
- Single, well-tested executor component
- Flexible progress reporting
- Easy to extend for new operation types
- Clear separation from plan generation

### Structure
```go
type Executor struct {
    client   *state.KonnectClient
    reporter ProgressReporter
    dryRun   bool
}

// Executes any plan, regardless of operations
func (e *Executor) Execute(ctx context.Context, plan *Plan) (*ExecutionResult, error)
```

---

## ADR-003-009: Label Update Strategy

### Context
Successful operations need to update resource labels to maintain tracking,
but the strategy differs between operations.

### Decision
- CREATE: Add all management labels
- UPDATE: Update config-hash and last-updated
- DELETE: No label operations (resource removed)
- Failed operations: No label changes

### Consequences
- Consistent tracking of managed resources
- Config drift detection remains accurate
- Failed operations don't corrupt tracking

---

## ADR-003-010: Integration Test Approach

### Context
Stage 3 introduces complex command flows that need thorough testing without
depending on real Konnect APIs.

### Decision
Extend Stage 2's dual-mode SDK testing approach:
- Mock mode for fast, deterministic tests
- Real mode for integration validation
- Test both apply and sync flows completely

### Consequences
- Comprehensive test coverage
- Fast test execution in CI
- Ability to test error scenarios
- Validation against real API when needed