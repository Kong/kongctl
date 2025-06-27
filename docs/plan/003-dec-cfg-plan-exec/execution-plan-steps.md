# Stage 3: Plan Execution - Implementation Steps

## Progress Summary

| Step | Description | Status | Dependencies |
|------|-------------|---------|--------------|
| 1 | Enhance planner with mode support | Not Started | - |
| 2 | Create base executor package | Not Started | Step 1 |
| 3 | Implement progress reporter | Not Started | Step 2 |
| 4 | Add portal operations to executor | Not Started | Step 2 |
| 5 | Implement apply command | Not Started | Steps 3, 4 |
| 6 | Implement sync command | Not Started | Steps 3, 4 |
| 7 | Add plan validation | Not Started | Steps 5, 6 |
| 8 | Implement confirmation prompts | Not Started | Steps 5, 6 |
| 9 | Migrate login to Konnect-first | Not Started | - |
| 10 | Add integration tests | Not Started | Steps 5, 6, 9 |
| 11 | Update documentation | Not Started | All steps |

**Current Stage**: Ready to begin implementation

---

## Step 1: Enhance Planner with Mode Support

**Status**: Not Started

### Goal
Extend the Stage 2 planner to support mode-aware plan generation.

### Changes Required

1. Update plan types in `internal/declarative/planner/plan.go`:
```go
type PlanMode string

const (
    PlanModeSync  PlanMode = "sync"
    PlanModeApply PlanMode = "apply"
)

// Update PlanMetadata
type PlanMetadata struct {
    GeneratedAt string    `json:"generated_at"`
    Version     string    `json:"version"`
    Mode        PlanMode  `json:"mode"`
    ConfigHash  string    `json:"config_hash"`
}
```

2. Update planner options in `internal/declarative/planner/planner.go`:
```go
type Options struct {
    Mode PlanMode
}

func (p *Planner) GeneratePlan(ctx context.Context, resources *resources.ResourceSet, opts Options) (*Plan, error)
```

3. Modify plan generation logic to conditionally include DELETE operations:
- When mode is "apply": Skip DELETE operation generation
- When mode is "sync": Include DELETE operations for managed resources not in config

4. Update plan command to accept mode flag:
```go
// In internal/cmd/root/verbs/plan/plan.go
var planMode string
cmd.Flags().StringVar(&planMode, "mode", "sync", "Plan generation mode (sync|apply)")
```

### Tests Required
- Planner generates apply-mode plans without DELETEs
- Planner generates sync-mode plans with DELETEs
- Plan metadata includes correct mode
- Plan command accepts and validates mode flag

### Definition of Done
- [ ] Planner supports mode parameter
- [ ] Apply mode excludes DELETE operations
- [ ] Sync mode includes all operations
- [ ] Plan metadata indicates generation mode
- [ ] Tests pass for both modes

---

## Step 2: Create Base Executor Package

**Status**: Not Started
**Dependencies**: Step 1

### Goal
Create the executor package with core execution logic.

### Implementation

1. Create `internal/declarative/executor/executor.go`:
```go
package executor

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

type ExecutionError struct {
    ChangeID     string
    ResourceType string
    ResourceName string
    Error        error
}

func New(client *state.KonnectClient, reporter ProgressReporter, dryRun bool) *Executor

func (e *Executor) Execute(ctx context.Context, plan *planner.Plan) (*ExecutionResult, error)
```

2. Implement core execution loop:
- Iterate through plan changes
- Dispatch to operation handlers
- Collect results and errors
- Support dry-run mode

3. Create operation dispatcher:
```go
func (e *Executor) executeChange(ctx context.Context, change planner.PlannedChange) error {
    if e.dryRun {
        e.reporter.SkipChange(change, "dry-run mode")
        return nil
    }

    switch change.Action {
    case planner.ActionCreate:
        return e.createResource(ctx, change)
    case planner.ActionUpdate:
        return e.updateResource(ctx, change)
    case planner.ActionDelete:
        return e.deleteResource(ctx, change)
    default:
        return fmt.Errorf("unknown action: %s", change.Action)
    }
}
```

### Tests Required
- Executor creation and configuration
- Dry-run mode skips actual operations
- Execution result tracking
- Error collection and reporting

### Definition of Done
- [ ] Executor package structure created
- [ ] Core execution loop implemented
- [ ] Dry-run mode supported
- [ ] Unit tests for executor logic

---

## Step 3: Implement Progress Reporter

**Status**: Not Started
**Dependencies**: Step 2

### Goal
Create progress reporting system for real-time execution feedback.

### Implementation

1. Define interface in `internal/declarative/executor/progress.go`:
```go
type ProgressReporter interface {
    StartExecution(plan *planner.Plan)
    StartChange(change planner.PlannedChange)
    CompleteChange(change planner.PlannedChange, err error)
    SkipChange(change planner.PlannedChange, reason string)
    FinishExecution(result *ExecutionResult)
}
```

2. Create console reporter implementation:
```go
type ConsoleReporter struct {
    writer io.Writer
}

func NewConsoleReporter(w io.Writer) *ConsoleReporter

// Implement interface methods with formatted output
```

3. Add progress reporting to executor:
- Call reporter methods at appropriate points
- Handle nil reporter gracefully
- Ensure reporter doesn't affect execution flow

### Output Examples
```
Executing plan...
Creating portal: developer-portal... ✓
Updating portal: staging-portal... ✓
Deleting portal_page: old-docs... ✗ Error: not found

Execution complete:
- Success: 2
- Failed: 1
- Skipped: 0
```

### Tests Required
- Reporter interface compliance
- Console output formatting
- Progress tracking accuracy
- Nil reporter handling

### Definition of Done
- [ ] Progress reporter interface defined
- [ ] Console reporter implemented
- [ ] Executor integrated with reporter
- [ ] Clear, informative output format

---

## Step 4: Add Portal Operations to Executor

**Status**: Not Started
**Dependencies**: Step 2

### Goal
Implement resource-specific operations for portals in the executor.

### Implementation

1. Create `internal/declarative/executor/portal_operations.go`:
```go
func (e *Executor) createPortal(ctx context.Context, change planner.PlannedChange) error
func (e *Executor) updatePortal(ctx context.Context, change planner.PlannedChange) error
func (e *Executor) deletePortal(ctx context.Context, change planner.PlannedChange) error
```

2. Implement create operation:
- Extract portal from desired state
- Add management labels
- Call Konnect API
- Handle errors appropriately

3. Implement update operation:
- Check protection status
- Update labels with new hash
- Call update API
- Preserve certain fields if needed

4. Implement delete operation:
- Verify resource is managed
- Check protection status
- Call delete API
- Handle not-found gracefully

5. Update main executor to dispatch to portal operations

### Label Management
```go
// Add labels during create
portal.Labels = labels.AddManagedLabels(portal.Labels, configHash)

// Update hash during update
portal.Labels[labels.LabelConfigHash] = newConfigHash
portal.Labels[labels.LabelLastUpdated] = time.Now().Format(time.RFC3339)
```

### Tests Required
- Create portal with labels
- Update portal preserves protection
- Delete checks protection status
- Error handling for each operation

### Definition of Done
- [ ] Portal operations implemented
- [ ] Label management integrated
- [ ] Protection status respected
- [ ] Comprehensive error handling

---

## Step 5: Implement Apply Command

**Status**: Not Started
**Dependencies**: Steps 3, 4

### Goal
Create the apply command that executes CREATE/UPDATE operations only.

### Implementation

1. Create `internal/cmd/root/verbs/apply/apply.go`:
```go
func NewCommand() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "apply",
        Short: "Apply configuration changes (create/update only)",
        Long:  "Execute a plan to create new resources and update existing ones. Never deletes resources.",
        RunE:  runApply,
    }
    
    // Add flags
    cmd.Flags().String("plan", "", "Path to existing plan file")
    cmd.Flags().String("config", "", "Path to configuration directory")
    cmd.Flags().Bool("dry-run", false, "Preview changes without applying")
    cmd.Flags().Bool("auto-approve", false, "Skip confirmation prompt")
    
    return cmd
}
```

2. Implement command logic:
```go
func runApply(cmd *cobra.Command, args []string) error {
    // Load or generate plan
    var plan *planner.Plan
    if planFile != "" {
        plan = loadPlanFromFile(planFile)
    } else {
        plan = generatePlan(configDir, planner.PlanModeApply)
    }
    
    // Validate plan compatibility
    if err := validateApplyPlan(plan); err != nil {
        return err
    }
    
    // Show summary and confirm
    displayPlanSummary(plan)
    if !autoApprove && !confirmExecution(plan) {
        return fmt.Errorf("apply cancelled")
    }
    
    // Execute
    executor := executor.New(client, reporter, dryRun)
    result, err := executor.Execute(ctx, plan)
    
    // Report results
    return reportResults(result, err)
}
```

3. Add plan validation:
```go
func validateApplyPlan(plan *planner.Plan) error {
    if plan.ContainsDeletes() {
        return fmt.Errorf("apply command cannot execute plans with DELETE operations")
    }
    if plan.Metadata.Mode == planner.PlanModeSync {
        // Warning only
        fmt.Fprintf(os.Stderr, "Warning: Plan was generated in sync mode\n")
    }
    return nil
}
```

### Tests Required
- Command creation and flag parsing
- Plan file loading
- Plan generation in apply mode
- Validation rejects DELETE operations
- Dry-run execution
- Auto-approve flow

### Definition of Done
- [ ] Apply command implemented
- [ ] Plan validation prevents DELETEs
- [ ] Confirmation prompt works
- [ ] Integration with executor
- [ ] Clear output and error messages

---

## Step 6: Implement Sync Command

**Status**: Not Started
**Dependencies**: Steps 3, 4

### Goal
Create the sync command that performs full reconciliation including DELETEs.

### Implementation

1. Create `internal/cmd/root/verbs/sync/sync.go`:
```go
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
    
    return cmd
}
```

2. Implement sync-specific confirmation:
```go
func confirmSyncExecution(plan *planner.Plan) bool {
    if !plan.ContainsDeletes() {
        // Same as apply confirmation
        return confirmExecution(plan)
    }
    
    // Special handling for DELETEs
    fmt.Println("WARNING: This will DELETE the following resources:")
    for _, change := range plan.Changes {
        if change.Action == planner.ActionDelete {
            fmt.Printf("  - %s: %s\n", change.ResourceType, change.ResourceName)
        }
    }
    fmt.Println()
    
    fmt.Print("Type 'yes' to confirm deletion: ")
    var response string
    fmt.Scanln(&response)
    return response == "yes"
}
```

3. Implement command logic similar to apply but:
- Generate plans in sync mode
- Allow DELETE operations
- Enhanced confirmation for destructive changes

### Tests Required
- Sync mode plan generation
- DELETE operation handling
- Enhanced confirmation flow
- Protected resource warnings

### Definition of Done
- [ ] Sync command implemented
- [ ] DELETE operations supported
- [ ] Enhanced confirmation for DELETEs
- [ ] Clear warnings for destructive operations

---

## Step 7: Add Plan Validation

**Status**: Not Started
**Dependencies**: Steps 5, 6

### Goal
Implement comprehensive plan validation for both commands.

### Implementation

1. Create `internal/declarative/validator/validator.go`:
```go
type PlanValidator struct {
    client *state.KonnectClient
}

func (v *PlanValidator) ValidateForApply(plan *planner.Plan) error
func (v *PlanValidator) ValidateForSync(plan *planner.Plan) error
```

2. Implement validation rules:
- Mode compatibility
- Resource state verification
- Protection status checks
- Reference resolution validation
- Dependency order verification

3. Add pre-execution validation:
```go
// Check if resources still exist
// Verify resources haven't changed since plan generation
// Validate protection status hasn't changed
// Ensure references are still valid
```

### Tests Required
- Mode compatibility validation
- State drift detection
- Protection status validation
- Invalid plan rejection

### Definition of Done
- [ ] Validation package created
- [ ] Apply-specific validation rules
- [ ] Sync-specific validation rules
- [ ] Integration with commands

---

## Step 8: Implement Confirmation Prompts

**Status**: Not Started
**Dependencies**: Steps 5, 6

### Goal
Create user-friendly confirmation prompts with appropriate detail levels.

### Implementation

1. Create `internal/cmd/common/prompts.go`:
```go
func ConfirmApply(plan *planner.Plan) bool
func ConfirmSync(plan *planner.Plan) bool
func DisplayPlanSummary(plan *planner.Plan)
```

2. Implement tiered prompts based on risk:
- Simple prompt for CREATE only
- Detailed prompt for UPDATE operations
- Warning prompt for DELETE operations

3. Add summary display:
```go
func DisplayPlanSummary(plan *planner.Plan) {
    fmt.Printf("Plan Summary:\n")
    fmt.Printf("  Mode: %s\n", plan.Metadata.Mode)
    fmt.Printf("  Total changes: %d\n", plan.Summary.TotalChanges)
    
    if plan.Summary.ByAction["CREATE"] > 0 {
        fmt.Printf("  Create: %d resources\n", plan.Summary.ByAction["CREATE"])
    }
    // ... similar for UPDATE and DELETE
}
```

### Tests Required
- Prompt formatting
- User input handling
- Auto-approve bypasses prompts
- Summary display accuracy

### Definition of Done
- [ ] Confirmation prompts implemented
- [ ] Risk-appropriate detail levels
- [ ] Clear summary display
- [ ] Auto-approve support

---

## Step 9: Migrate Login to Konnect-First

**Status**: Not Started
**Dependencies**: None

### Goal
Update login command to default to Konnect without requiring product specification.

### Implementation

1. Update `internal/cmd/root/verbs/login/login.go`:
```go
func NewCommand() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "login",
        Short: "Authenticate with Kong Konnect",
        Long:  "Authenticate with Kong Konnect using device authorization flow",
        RunE:  runLogin,
    }
    
    // Future: Add --product flag for other products
    // cmd.Flags().String("product", "konnect", "Product to login to")
    
    return cmd
}
```

2. Update command registration in root:
- Remove subcommands under login
- Make login directly execute Konnect auth

3. Add deprecation notice for old syntax:
```go
// If args[0] == "konnect", show deprecation warning
if len(args) > 0 && args[0] == "konnect" {
    fmt.Fprintf(os.Stderr, "Warning: 'kongctl login konnect' is deprecated. Use 'kongctl login' instead.\n")
}
```

### Tests Required
- Login command works without product specification
- Deprecation warning for old syntax
- Authentication flow unchanged

### Definition of Done
- [ ] Login defaults to Konnect
- [ ] Old syntax shows deprecation warning
- [ ] Tests updated for new behavior
- [ ] Documentation updated

---

## Step 10: Add Integration Tests

**Status**: Not Started
**Dependencies**: Steps 5, 6, 9

### Goal
Comprehensive integration tests for plan execution flows.

### Implementation

1. Create apply command tests in `test/integration/apply_test.go`:
- Test CREATE operations
- Test UPDATE operations
- Verify no DELETE operations executed
- Test plan file loading
- Test dry-run mode

2. Create sync command tests in `test/integration/sync_test.go`:
- Test full reconciliation
- Test DELETE operations
- Test protected resource handling
- Test confirmation prompts
- Test auto-approve mode

3. Create executor tests in `test/integration/executor_test.go`:
- Test with mock SDK
- Test error handling
- Test partial execution recovery
- Test progress reporting

4. Update test utilities:
- Helper to create test plans
- Helper to verify execution results
- Mock confirmation responses

### Test Scenarios
```go
// Apply scenarios
TestApplyCreateOnly
TestApplyWithUpdates
TestApplyRejectsPlanWithDeletes
TestApplyDryRun
TestApplyFromPlanFile

// Sync scenarios
TestSyncFullReconciliation
TestSyncDeletesUnmanagedResources
TestSyncProtectedResources
TestSyncConfirmationFlow

// Error scenarios
TestExecutorAPIErrors
TestExecutorPartialFailure
TestPlanValidationErrors
```

### Definition of Done
- [ ] Apply command fully tested
- [ ] Sync command fully tested
- [ ] Error scenarios covered
- [ ] Mock and real SDK modes supported

---

## Step 11: Update Documentation

**Status**: Not Started
**Dependencies**: All steps

### Goal
Update user documentation and examples for new commands.

### Implementation

1. Update main README.md:
- Add apply and sync command examples
- Explain difference between commands
- Update login command syntax

2. Create `docs/declarative-execution.md`:
- Detailed explanation of apply vs sync
- Plan generation and execution workflow
- Safety features and protection
- Troubleshooting guide

3. Update command help text:
- Clear descriptions of command behaviors
- Flag documentation
- Example usage in help output

4. Add example scenarios:
```bash
# Example: First-time setup
kongctl apply --config ./portals

# Example: CI/CD pipeline
kongctl sync --config ./config --auto-approve

# Example: Review changes before applying
kongctl plan --config ./config -o plan.json
kongctl apply --plan plan.json
```

### Definition of Done
- [ ] README updated with new commands
- [ ] Detailed execution guide created
- [ ] Command help text is clear
- [ ] Example scenarios documented

---

## Testing Strategy

### Unit Tests
Each step should include unit tests for:
- New functions and methods
- Error handling paths
- Edge cases

### Integration Tests
Step 10 provides comprehensive integration testing, but each step should be
manually testable:

```bash
# After each step
make build && make lint && make test

# Manual testing commands
./kongctl plan --mode apply --config test/fixtures/portals
./kongctl apply --dry-run --config test/fixtures/portals
./kongctl sync --config test/fixtures/portals
```

### Test Data
Use existing test fixtures from Stage 2, extended with:
- Resources for deletion testing
- Protected resource examples
- Invalid plan files for error testing

---

## Notes for Implementers

### Code Quality
- Follow existing patterns from Stages 1 and 2
- Maintain consistent error handling
- Add appropriate logging at debug level
- Keep user-facing messages clear and actionable

### Performance
- Operations should be as parallel as possible
- Progress reporting should not slow execution
- Plan validation should be efficient

### Safety
- Never execute DELETE without explicit sync command
- Always validate plans before execution
- Protected resources require extra confirmation
- Dry-run must not make any API calls

### Future Extensibility
- Executor should handle new resource types easily
- Progress reporter interface allows different implementations
- Validation can be extended with new rules
- Commands structured for future enhancements