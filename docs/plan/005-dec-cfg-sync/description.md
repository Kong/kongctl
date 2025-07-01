# KongCtl Stage 5 - Sync Command Implementation

## Goal
Implement the sync command for full state reconciliation including DELETE operations.

## Deliverables
- Sync command with full CREATE/UPDATE/DELETE support
- Mode-aware plan generation in sync mode
- Consistent confirmation prompts and auto-approve
- Output format support (text/json/yaml)
- Protected resource handling

## Implementation Details

### Sync Command Structure
```go
// internal/cmd/root/verbs/sync/sync.go
func NewCommand() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "sync",
        Short: "Synchronize configuration state (includes deletions)",
        Long:  "Execute a plan to fully synchronize state, including deletion of resources not in configuration.",
        RunE:  runSync,
    }
    
    // Similar flags to apply
    cmd.Flags().String("plan", "", "Path to existing plan file")
    cmd.Flags().String("config", "", "Path to configuration directory")
    cmd.Flags().Bool("dry-run", false, "Preview changes without applying")
    cmd.Flags().Bool("auto-approve", false, "Skip confirmation prompt")
    cmd.Flags().String("output", "text", "Output format (text|json|yaml)")
    cmd.Flags().Bool("show-unmanaged", false, "Show unmanaged fields after execution")
    
    return cmd
}
```

### Plan Generation in Sync Mode
```go
func runSync(cmd *cobra.Command, args []string) error {
    // Load or generate plan
    var plan *planner.Plan
    if planFile != "" {
        plan = loadPlanFromFile(planFile)
    } else {
        // Generate plan in sync mode (includes DELETE operations)
        plan = generatePlan(configDir, planner.PlanModeSync)
    }
    
    // Show summary and confirm (only in text mode)
    outputFormat := cmd.Flag("output").Value.String()
    if outputFormat == "text" && !autoApprove {
        if !confirmExecution(plan) {
            return fmt.Errorf("sync cancelled")
        }
    }
    
    // Execute using same executor as apply
    var reporter executor.ProgressReporter
    if outputFormat == "text" {
        reporter = executor.NewConsoleReporter(os.Stderr)
    }
    
    exec := executor.New(client, reporter, dryRun)
    result, err := exec.Execute(ctx, plan)
    
    // Output results based on format
    return outputResults(result, err, outputFormat)
}
```

### DELETE Operation Implementation
```go
// internal/declarative/executor/portal_operations.go
func (e *Executor) deletePortal(ctx context.Context, change planner.PlannedChange) error {
    // Validate protection status at execution time
    if change.Protection {
        return fmt.Errorf("portal %q is protected and cannot be deleted", change.ResourceName)
    }
    
    // Verify resource is managed
    current := change.CurrentState.(state.Portal)
    if !labels.IsManaged(current.Labels) {
        return fmt.Errorf("portal %q is not managed by kongctl", change.ResourceName)
    }
    
    // Call delete API
    err := e.client.DeletePortal(ctx, change.ResourceID)
    if err != nil {
        // Handle not-found gracefully
        if isNotFoundError(err) {
            e.reporter.SkipChange(change, "resource already deleted")
            return nil
        }
        return fmt.Errorf("failed to delete portal %q: %w", change.ResourceName, err)
    }
    
    return nil
}
```

### Confirmation Prompt with DELETE Warning
```go
func ConfirmExecution(plan *planner.Plan) bool {
    DisplayPlanSummary(plan)
    
    // Show DELETE warning if applicable
    if plan.Summary.ByAction["DELETE"] > 0 {
        fmt.Println("\nWARNING: This operation will DELETE resources:")
        for _, change := range plan.Changes {
            if change.Action == planner.ActionDelete {
                fmt.Printf("- %s: %s\n", change.ResourceType, change.ResourceName)
            }
        }
    }
    
    fmt.Print("\nDo you want to continue? Type 'yes' to confirm: ")
    var response string
    fmt.Scanln(&response)
    return response == "yes"
}
```

## Tests Required
- Sync mode plan generation includes DELETE operations
- DELETE operation execution and error handling
- Protected resource blocking for DELETEs
- Confirmation prompt shows DELETE warnings
- Managed resource validation
- Not-found errors handled gracefully
- Output format support (text, json, yaml)
- Auto-approve for CI/CD automation
- Integration tests for full sync workflow

## Proof of Success
```bash
# Full synchronization
$ kongctl sync
Plan Summary:
- Create: 2 resources
- Update: 1 resource
- Delete: 1 resource

WARNING: This operation will DELETE resources:
- portal: old-portal

Do you want to continue? Type 'yes' to confirm: yes

Executing plan...
✓ Created portal: new-portal
✓ Updated portal: existing-portal
✓ Deleted portal: old-portal
Plan applied successfully: 2 created, 1 updated, 1 deleted

# Dry run to preview deletions
$ kongctl sync --dry-run
Plan Summary:
- Create: 0 resources
- Update: 0 resources
- Delete: 3 resources

Dry run mode - no changes will be made

# CI/CD automation
$ kongctl sync --auto-approve --output json
{
  "execution_result": {
    "success_count": 4,
    "failure_count": 0,
    "skipped_count": 0,
    "errors": []
  }
}

# Protected resources block deletion
$ kongctl sync
Error: Cannot generate plan due to protected resources:
- portal "production-portal" is protected and cannot be deleted

To proceed, first update this resource to set protected: false
```

## Dependencies
- Stage 3 completion (executor infrastructure)
- Stage 2 planner with mode support
- DELETE operation support in executor

## Notes
- Sync is the only command that performs deletions
- Same executor infrastructure as apply command
- Protected resources must be explicitly unprotected first
- Clear warnings about destructive operations
- Support for automation with structured output