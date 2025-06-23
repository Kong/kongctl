# KongCtl Stage 3 - Plan Execution

## Goal
Execute plans to create and update portals with proper error handling.

## Deliverables
- Plan executor with API operations
- Apply command with dry-run support
- Progress reporting and error handling
- Update label management on successful operations

## Implementation Details

### Plan Executor
```go
// internal/declarative/executor/executor.go
type Executor struct {
    client *state.KonnectClient
    dryRun bool
}

type ExecutionResult struct {
    SuccessCount int
    FailureCount int
    Errors       []ExecutionError
}

type ExecutionError struct {
    ChangeID string
    Error    error
}

func (e *Executor) Execute(ctx context.Context, plan *Plan) (*ExecutionResult, error) {
    result := &ExecutionResult{}
    
    for _, change := range plan.Changes {
        if err := e.executeChange(ctx, change); err != nil {
            result.Errors = append(result.Errors, ExecutionError{
                ChangeID: change.ID,
                Error:    err,
            })
            result.FailureCount++
            // Decide whether to continue or fail fast
        } else {
            result.SuccessCount++
        }
    }
    
    return result, nil
}

func (e *Executor) executeChange(ctx context.Context, change PlannedChange) error {
    if e.dryRun {
        // Log what would happen
        return nil
    }
    
    switch change.Action {
    case ActionCreate:
        return e.createResource(ctx, change)
    case ActionUpdate:
        return e.updateResource(ctx, change)
    default:
        return fmt.Errorf("unknown action: %s", change.Action)
    }
}
```

### Resource Creation/Update
```go
// internal/declarative/executor/portal_ops.go
func (e *Executor) createPortal(ctx context.Context, change PlannedChange) error {
    portal := change.DesiredState.(PortalResource)
    
    // Add managed labels
    configHash := calculateConfigHash(portal)
    portal.Labels = AddManagedLabels(portal.Labels, configHash)
    
    // Create via SDK
    createReq := components.CreatePortal{
        Name:                    portal.Name,
        DisplayName:             portal.DisplayName,
        Description:             portal.Description,
        AutoApproveDevelopers:   portal.AutoApproveDevelopers,
        AutoApproveApplications: portal.AutoApproveApplications,
        Labels:                  portal.Labels,
        // ... other fields
    }
    
    resp, err := e.client.sdk.Portals.CreatePortal(ctx, createReq)
    if err != nil {
        return fmt.Errorf("failed to create portal %s: %w", portal.Name, err)
    }
    
    // Store ID for future reference if needed
    return nil
}

func (e *Executor) updatePortal(ctx context.Context, change PlannedChange) error {
    current := change.CurrentState.(components.Portal)
    desired := change.DesiredState.(PortalResource)
    
    // Preserve protected status if set
    if current.Labels[LabelProtected] == "true" && desired.Kongctl.Protected {
        return fmt.Errorf("portal %s is protected, cannot update", desired.Name)
    }
    
    // Update labels with new hash
    configHash := calculateConfigHash(desired)
    desired.Labels = AddManagedLabels(desired.Labels, configHash)
    
    updateReq := components.UpdatePortal{
        Name:                    &desired.Name,
        DisplayName:             &desired.DisplayName,
        Description:             &desired.Description,
        AutoApproveDevelopers:   &desired.AutoApproveDevelopers,
        AutoApproveApplications: &desired.AutoApproveApplications,
        Labels:                  desired.Labels,
        // ... other fields
    }
    
    _, err := e.client.sdk.Portals.UpdatePortal(ctx, current.ID, updateReq)
    return err
}
```

### Suggested Apply Command Implementation
```go
// internal/cmd/root/verbs/apply/apply.go
func Execute(cmd *cobra.Command, args []string) error {
    var plan *Plan
    
    if planFile != "" {
        // Load existing plan
        plan, err = loadPlan(planFile)
    } else {
        // Generate new plan
        resources, err := loadResources(configDir)
        planner := planner.New(konnectClient)
        plan, err = planner.GeneratePlan(ctx, resources)
    }
    
    // Show what will happen
    if err := displayPlanSummary(plan); err != nil {
        return err
    }
    
    if dryRun {
        fmt.Println("Dry run mode - no changes will be made")
        return nil
    }
    
    // Confirm if not auto-approve
    if !autoApprove && !confirmExecution(plan) {
        return fmt.Errorf("execution cancelled")
    }
    
    // Execute
    executor := executor.New(konnectClient, dryRun)
    result, err := executor.Execute(ctx, plan)
    
    // Report results
    displayExecutionResult(result)
    
    if result.FailureCount > 0 {
        return fmt.Errorf("execution completed with %d errors", result.FailureCount)
    }
    
    return nil
}
```

### Progress Reporting
```go
// internal/declarative/executor/progress.go
type ProgressReporter interface {
    StartChange(change PlannedChange)
    CompleteChange(change PlannedChange, err error)
    Summary(result *ExecutionResult)
}

type ConsoleReporter struct{}

func (r *ConsoleReporter) StartChange(change PlannedChange) {
    action := strings.Title(strings.ToLower(string(change.Action)))
    fmt.Printf("%s %s: %s...", action, change.ResourceType, change.ResourceName)
}

func (r *ConsoleReporter) CompleteChange(change PlannedChange, err error) {
    if err != nil {
        fmt.Printf(" ✗ %s\n", err)
    } else {
        fmt.Printf(" ✓\n")
    }
}
```

## Tests Required
- Executor with mock API client
- Dry-run mode verification
- Error handling and partial failures
- Label updates during creation/update
- Protection flag enforcement
- Progress reporting

## Proof of Success
```bash
# Create a new portal from configuration
$ kongctl apply
Executing plan...
✓ Created portal: developer-portal (id: abc-123)
✓ Updated labels for tracking
Plan applied successfully: 1 resource created

# Dry run mode
$ kongctl apply --dry-run
Plan summary:
- Create portal: developer-portal
Dry run mode - no changes will be made

# Apply with existing plan
$ kongctl apply --plan previous-plan.json
Executing plan from previous-plan.json...
✓ Updated portal: developer-portal
Plan applied successfully: 1 resource updated

# Handle errors gracefully
$ kongctl apply
Executing plan...
✗ Failed to create portal: developer-portal
  Error: portal with name 'developer-portal' already exists
Plan execution failed: 1 error(s)
```

## Dependencies
- Stage 2 completion (plan generation)
- Kong SDK for API operations
- Terminal output formatting

## Notes
- Always update labels after successful operations
- Consider implementing retry logic for transient failures
- Dry-run mode must not make any API calls
- Progress reporting should be real-time, not batched
- Handle protected resources appropriately