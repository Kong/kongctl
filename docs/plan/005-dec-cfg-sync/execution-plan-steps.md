# Stage 5: Sync Command Implementation - Execution Steps

## Progress Summary

| Step | Description | Status | Dependencies |
|------|-------------|---------|--------------|
| 1 | Create sync command structure | Completed | None |
| 2 | Add sync mode to planner | Completed | Step 1 |
| 3 | Implement DELETE operation planning | Completed | Step 2 |
| 4 | Add portal DELETE execution | Completed | Step 3 |
| 5 | Add API resource DELETE execution | Not Started | Step 4 |
| 6 | Implement confirmation prompts | Not Started | Step 5 |
| 7 | Add integration tests | Not Started | Step 6 |

## Detailed Steps

### Step 1: Create sync command structure
**Status**: Completed

**Note**: The sync command structure already existed as part of the verb-based CLI architecture. This step involved verifying the existing implementation and ensuring it follows the correct patterns.

Create the basic sync command with flags and structure.

**Files to create/modify**:
- `internal/cmd/root/verbs/sync/sync.go`
- `internal/cmd/root/verbs/verbs.go` (register command)

**Implementation**:
```go
// internal/cmd/root/verbs/sync/sync.go
package sync

import (
    "context"
    "fmt"
    "os"
    
    "github.com/kong/kongctl/internal/cmd/common"
    "github.com/kong/kongctl/internal/cmd/root/verbs/apply"
    "github.com/kong/kongctl/internal/declarative/planner"
    "github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "sync",
        Short: "Synchronize configuration state (includes deletions)",
        Long: `Execute a plan to fully synchronize state, including deletion of resources not in configuration.

The sync command performs full state reconciliation:
- Creates new resources defined in configuration
- Updates existing resources to match configuration
- Deletes resources that exist but are not in configuration

This is the only command that performs deletions. Protected resources cannot be deleted.`,
        RunE: runSync,
    }
    
    cmd.Flags().StringSliceP("filename", "f", []string{},
        "Filename or directory to files to use to create the resource (can specify multiple)")
    cmd.Flags().BoolP("recursive", "R", false,
        "Process the directory used in -f, --filename recursively")
    cmd.Flags().StringP("plan", "p", "", "Path to existing plan file (JSON format)")
    cmd.Flags().Bool("dry-run", false, "Preview changes without applying them")
    cmd.Flags().Bool("auto-approve", false, "Skip interactive confirmation prompt")
    cmd.Flags().StringP("output", "o", "text", "Output format (text|json|yaml)")
    cmd.Flags().String("execution-report-file", "", "Save execution report as JSON to file")
    
    return cmd
}

func runSync(cmd *cobra.Command, args []string) error {
    // Implementation in later steps
    return fmt.Errorf("sync command not yet implemented")
}
```

**Tests to add**:
- Command registration test
- Flag parsing test

### Step 2: Add sync mode to planner
**Status**: Completed
**Dependencies**: Step 1

**Note**: The sync mode functionality was already implemented in the planner. All resource planners (portal, API, auth strategy) already check for `plan.Metadata.Mode == PlanModeSync` and generate DELETE operations for managed resources not in the desired state.

Extend the planner to support sync mode for DELETE operations.

**Files to modify**:
- `internal/declarative/planner/planner.go`
- `internal/declarative/planner/plan.go`

**Implementation**:
```go
// internal/declarative/planner/planner.go

// Add to existing file
type PlanMode string

const (
    PlanModeApply PlanMode = "apply"
    PlanModeSync  PlanMode = "sync"
)

// Modify BuildPlan to accept mode
func (p *Planner) BuildPlan(ctx context.Context, desired resources.ResourceSet, mode PlanMode) (*Plan, error) {
    // Existing logic...
    
    // After processing CREATE/UPDATE operations
    if mode == PlanModeSync {
        // Find resources to delete
        for _, current := range currentState {
            if !isInDesiredState(current, desired) && labels.IsManaged(current.Labels) {
                change := PlannedChange{
                    Action:       ActionDelete,
                    ResourceType: current.Type,
                    ResourceID:   current.ID,
                    ResourceName: current.Name,
                    CurrentState: current,
                    Protection:   current.Protected,
                }
                changes = append(changes, change)
            }
        }
    }
    
    // Continue with plan creation...
}
```

**Tests to add**:
- Sync mode creates DELETE operations
- Apply mode does not create DELETE operations
- Only managed resources included in DELETE

### Step 3: Implement DELETE operation planning
**Status**: Completed
**Dependencies**: Step 2

**Note**: DELETE operation validation is already implemented within the planner. Protected resources are validated during plan generation in the resource-specific planners (portal_planner.go, api_planner.go, etc.). The validation happens inline rather than in separate validation files.

Add DELETE operation support to plan generation and validation.

**Files to modify**:
- `internal/declarative/planner/validation.go` (not needed - validation is inline)
- `internal/declarative/planner/change.go` (not needed - using existing types)

**Implementation**:
```go
// internal/declarative/planner/validation.go

// Add DELETE validation
func validateDeleteOperation(change PlannedChange) error {
    // Check protection status
    if change.Protection {
        return fmt.Errorf("%s %q is protected and cannot be deleted", 
            change.ResourceType, change.ResourceName)
    }
    
    // Verify managed status
    current := change.CurrentState
    if !labels.IsManaged(current.GetLabels()) {
        return fmt.Errorf("%s %q is not managed by kongctl", 
            change.ResourceType, change.ResourceName)
    }
    
    return nil
}

// Update validatePlan to include DELETE validation
func validatePlan(plan *Plan) error {
    for _, change := range plan.Changes {
        if change.Action == ActionDelete {
            if err := validateDeleteOperation(change); err != nil {
                return err
            }
        }
    }
    return nil
}
```

**Tests to add**:
- Protected resources fail validation
- Unmanaged resources not included
- DELETE operations properly ordered

### Step 4: Add portal DELETE execution
**Status**: Completed
**Dependencies**: Step 3

**Note**: The portal DELETE execution was already implemented as part of the executor infrastructure. The implementation follows the established patterns from earlier stages rather than the exact specification in this planning document. Key differences:
- Uses `portal.NormalizedLabels` for protection checking (consistent with UPDATE operations)
- Reporter calls are handled by the parent `executeChange` method
- Protection validation happens in both `validateChangePreExecution` and the operation itself
- No `CurrentState` field in PlannedChange; uses Fields map instead

Implement DELETE operation for portal resources in executor.

**Files to modify**:
- `internal/declarative/executor/portal_operations.go`
- `internal/declarative/executor/executor.go`

**Implementation**:
```go
// internal/declarative/executor/portal_operations.go

func (e *Executor) deletePortal(ctx context.Context, change planner.PlannedChange) error {
    // Validate at execution time
    if change.Protection {
        return fmt.Errorf("portal %q is protected and cannot be deleted", change.ResourceName)
    }
    
    // Cast to proper type
    current, ok := change.CurrentState.(*state.Portal)
    if !ok {
        return fmt.Errorf("invalid state type for portal deletion")
    }
    
    // Verify managed
    if !labels.IsManaged(current.Labels) {
        return fmt.Errorf("portal %q is not managed by kongctl", change.ResourceName)
    }
    
    // Report start
    if e.reporter != nil {
        e.reporter.StartChange(change)
    }
    
    // Skip in dry-run
    if e.dryRun {
        if e.reporter != nil {
            e.reporter.SkipChange(change, "dry-run mode")
        }
        return nil
    }
    
    // Call API
    _, err := e.client.Portals.DeletePortal(ctx, change.ResourceID)
    if err != nil {
        // Handle not found
        if sdkerrors.IsNotFoundError(err) {
            if e.reporter != nil {
                e.reporter.SkipChange(change, "resource already deleted")
            }
            return nil
        }
        return fmt.Errorf("failed to delete portal: %w", err)
    }
    
    // Report success
    if e.reporter != nil {
        e.reporter.CompleteChange(change)
    }
    
    return nil
}

// Update Execute to handle DELETE
func (e *Executor) Execute(ctx context.Context, plan *planner.Plan) (*ExecutionResult, error) {
    // Existing logic...
    
    case planner.ActionDelete:
        switch change.ResourceType {
        case "portal":
            err = e.deletePortal(ctx, change)
        // Add other resource types in Step 5
        default:
            err = fmt.Errorf("DELETE not implemented for %s", change.ResourceType)
        }
}
```

**Tests to add**:
- Successful portal deletion
- Not-found handled gracefully
- Protected portal blocks deletion
- Dry-run skips deletion

### Step 5: Add API resource DELETE execution
**Status**: Not Started
**Dependencies**: Step 4

Implement DELETE operations for API resources and their children.

**Files to modify**:
- `internal/declarative/executor/api_operations.go`

**Implementation**:
```go
// internal/declarative/executor/api_operations.go

func (e *Executor) deleteAPI(ctx context.Context, change planner.PlannedChange) error {
    // Similar structure to deletePortal
    current, ok := change.CurrentState.(*state.API)
    if !ok {
        return fmt.Errorf("invalid state type for API deletion")
    }
    
    if !labels.IsManaged(current.Labels) {
        return fmt.Errorf("API %q is not managed by kongctl", change.ResourceName)
    }
    
    if e.reporter != nil {
        e.reporter.StartChange(change)
    }
    
    if e.dryRun {
        if e.reporter != nil {
            e.reporter.SkipChange(change, "dry-run mode")
        }
        return nil
    }
    
    // Delete API (children are handled by backend cascade)
    _, err := e.client.APIs.DeleteAPI(ctx, change.ResourceID)
    if err != nil {
        if sdkerrors.IsNotFoundError(err) {
            if e.reporter != nil {
                e.reporter.SkipChange(change, "resource already deleted")
            }
            return nil
        }
        return fmt.Errorf("failed to delete API: %w", err)
    }
    
    if e.reporter != nil {
        e.reporter.CompleteChange(change)
    }
    
    return nil
}

// Add similar methods for:
// - deleteAPIVersion
// - deleteAPIImplementation
// - deleteAPIPublication
```

**Tests to add**:
- API deletion with cascade
- Child resource deletion
- Deletion order validation

### Step 6: Implement confirmation prompts
**Status**: Not Started
**Dependencies**: Step 5

Add interactive confirmation with DELETE warnings.

**Files to modify**:
- `internal/cmd/root/verbs/sync/sync.go`
- `internal/declarative/executor/confirmation.go` (new file)

**Implementation**:
```go
// internal/declarative/executor/confirmation.go
package executor

import (
    "bufio"
    "fmt"
    "os"
    "strings"
    
    "github.com/kong/kongctl/internal/declarative/planner"
)

func ConfirmExecution(plan *planner.Plan) bool {
    DisplayPlanSummary(plan)
    
    // Show DELETE warning
    deleteCount := plan.Summary.ByAction[planner.ActionDelete]
    if deleteCount > 0 {
        fmt.Fprintf(os.Stderr, "\nWARNING: This operation will DELETE %d resources:\n", deleteCount)
        for _, change := range plan.Changes {
            if change.Action == planner.ActionDelete {
                fmt.Fprintf(os.Stderr, "  - %s: %s\n", change.ResourceType, change.ResourceName)
            }
        }
        fmt.Fprintln(os.Stderr, "\nDeleted resources cannot be recovered!")
    }
    
    fmt.Fprint(os.Stderr, "\nDo you want to continue? Type 'yes' to confirm: ")
    
    reader := bufio.NewReader(os.Stdin)
    response, _ := reader.ReadString('\n')
    response = strings.TrimSpace(response)
    
    return response == "yes"
}

// Update sync command to use confirmation
func runSync(cmd *cobra.Command, args []string) error {
    // Load/generate plan...
    
    outputFormat, _ := cmd.Flags().GetString("output")
    autoApprove, _ := cmd.Flags().GetBool("auto-approve")
    
    // Confirm in text mode
    if outputFormat == "text" && !autoApprove && !dryRun {
        if !executor.ConfirmExecution(plan) {
            return fmt.Errorf("sync cancelled by user")
        }
    }
    
    // Execute plan...
}
```

**Tests to add**:
- Confirmation prompt appears for DELETE
- Auto-approve skips confirmation
- Non-text output skips confirmation
- "yes" accepts, anything else cancels

### Step 7: Add integration tests
**Status**: Not Started
**Dependencies**: Step 6

Create comprehensive integration tests for sync workflow.

**Files to create**:
- `test/integration/sync_test.go`

**Implementation**:
```go
// test/integration/sync_test.go
// +build integration

package integration

import (
    "testing"
    "github.com/stretchr/testify/require"
)

func TestSyncCommand(t *testing.T) {
    t.Run("full sync with mixed operations", func(t *testing.T) {
        // Setup: Create initial state
        // - Portal A (will be updated)
        // - Portal B (will be deleted)
        // Config defines:
        // - Portal A (modified)
        // - Portal C (new)
        
        // Run sync
        output := runCommand(t, "sync", "--auto-approve")
        
        // Verify:
        // - Portal A updated
        // - Portal B deleted
        // - Portal C created
    })
    
    t.Run("protected resource blocks sync", func(t *testing.T) {
        // Setup: Create protected portal
        // Config: Empty (would delete all)
        
        // Run sync - should fail
        _, err := runCommandExpectError(t, "sync", "--auto-approve")
        require.Contains(t, err.Error(), "protected and cannot be deleted")
    })
    
    t.Run("dry run shows deletions", func(t *testing.T) {
        // Setup: Create resources
        // Config: Empty
        
        // Run sync --dry-run
        output := runCommand(t, "sync", "--dry-run")
        
        // Verify output shows DELETE operations
        require.Contains(t, output, "Delete:")
        // Verify no actual deletions
    })
}
```

**Tests to add**:
- Full sync workflow
- Protected resource handling
- Dry-run behavior
- Output format tests
- Confirmation prompt tests
- Partial failure handling

## Completion Criteria

Each step is complete when:
1. Implementation matches specification
2. All listed tests pass
3. `make build && make lint && make test` passes
4. Integration tests pass (Step 7)
5. Documentation is updated

## Notes

- DELETE operations must validate protection and managed status
- Not-found errors should be handled gracefully (resource already gone)
- Clear user warnings for destructive operations
- Maintain consistency with apply command structure
- Focus on safety - sync is the only destructive command