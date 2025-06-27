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
| 7.5 | Protected resource validation | Not Started | Step 7 |
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

4. Add protection detection and blocked change generation:
```go
// In PlannedChange struct
type PlannedChange struct {
    ChangeID     string          `json:"change_id"`
    Action       ActionType      `json:"action"`
    ResourceType string          `json:"resource_type"`
    ResourceName string          `json:"resource_name"`
    Blocked      bool            `json:"blocked,omitempty"`
    BlockReason  string          `json:"block_reason,omitempty"`
    Current      json.RawMessage `json:"current,omitempty"`
    Desired      json.RawMessage `json:"desired,omitempty"`
}

// During plan generation
if isProtected(currentResource) && (action == ActionUpdate || action == ActionDelete) {
    change.Blocked = true
    change.BlockReason = fmt.Sprintf("Resource is protected. Remove protection before %s.", strings.ToLower(string(action)))
}
```

5. Update plan command to accept mode flag:
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
- Protected resources generate blocked changes for UPDATE/DELETE
- Blocked changes include clear reason messages
- Protection only blocks modifications, not removal of protection itself

### Definition of Done
- [ ] Planner supports mode parameter
- [ ] Apply mode excludes DELETE operations
- [ ] Sync mode includes all operations
- [ ] Plan metadata indicates generation mode
- [ ] Protected resources generate blocked changes
- [ ] Blocked changes have clear reasons
- [ ] Tests pass for both modes and protection scenarios

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
    BlockedCount int
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
    // Handle blocked changes first
    if change.Blocked {
        e.reporter.BlockedChange(change)
        return nil // Not an error, just blocked
    }
    
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
    BlockedChange(change planner.PlannedChange)
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
Updating portal: staging-portal... ⊘ Blocked: Resource is protected
Deleting portal_page: old-docs... ✗ Error: not found
Updating portal: production-portal... ⊘ Blocked: Resource is protected

Execution complete:
- Success: 1
- Failed: 1
- Skipped: 0
- Blocked: 2
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
- Skip if change is blocked (protection check done during planning)
- For unprotected resources:
  - Update labels with new hash
  - Call update API
  - Preserve certain fields if needed
- Report blocked changes to progress reporter

4. Implement delete operation:
- Skip if change is blocked (protection check done during planning)
- For unprotected resources:
  - Verify resource is managed
  - Call delete API
  - Handle not-found gracefully
- Report blocked changes to progress reporter

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
- Executor skips blocked changes
- Progress reporter shows blocked changes correctly
- Error handling for each operation
- Blocked changes don't affect execution result

### Definition of Done
- [ ] Portal operations implemented
- [ ] Label management integrated
- [ ] Blocked changes handled correctly
- [ ] Progress reporter shows blocked status
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
    cmd.Flags().String("output", "text", "Output format (text|json|yaml)")
    
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
    
    // Show summary and confirm (only in text mode)
    outputFormat := cmd.Flag("output").Value.String()
    if outputFormat == "text" && !autoApprove {
        if !confirmExecution(plan) {
            return fmt.Errorf("apply cancelled")
        }
    }
    
    // Execute
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

4. Add output formatting:
```go
func outputResults(result *executor.ExecutionResult, err error, format string) error {
    switch format {
    case "json":
        output := map[string]interface{}{
            "execution_result": result,
        }
        if err != nil {
            output["error"] = err.Error()
        }
        return json.NewEncoder(os.Stdout).Encode(output)
    case "yaml":
        output := map[string]interface{}{
            "execution_result": result,
        }
        if err != nil {
            output["error"] = err.Error()
        }
        return yaml.NewEncoder(os.Stdout).Encode(output)
    default: // text
        if err != nil {
            return err
        }
        // Human-readable output already handled by progress reporter
        return nil
    }
}
```

### Tests Required
- Command creation and flag parsing
- Plan file loading
- Plan generation in apply mode
- Validation rejects DELETE operations
- Dry-run execution
- Auto-approve flow
- Output format handling (text, json, yaml)
- Structured output in CI/CD contexts

### Definition of Done
- [ ] Apply command implemented
- [ ] Plan validation prevents DELETEs
- [ ] Confirmation prompt works
- [ ] Integration with executor
- [ ] Clear output and error messages
- [ ] Output formats work correctly
- [ ] Auto-approve enables automation

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
    cmd.Flags().String("output", "text", "Output format (text|json|yaml)")
    
    return cmd
}
```

2. Implement command logic similar to apply:
```go
func runSync(cmd *cobra.Command, args []string) error {
    // Load or generate plan
    var plan *planner.Plan
    if planFile != "" {
        plan = loadPlanFromFile(planFile)
    } else {
        plan = generatePlan(configDir, planner.PlanModeSync)
    }
    
    // Show summary and confirm (only in text mode)
    outputFormat := cmd.Flag("output").Value.String()
    if outputFormat == "text" && !autoApprove {
        if !confirmExecution(plan) {
            return fmt.Errorf("sync cancelled")
        }
    }
    
    // Execute
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

3. Key differences from apply:
- Generate plans in sync mode (includes DELETE operations)
- No special validation to reject DELETE operations
- Uses same confirmation prompt (with DELETE warning in confirmExecution)
- Same output format support for CI/CD integration

### Tests Required
- Sync mode plan generation
- DELETE operation handling
- Confirmation flow with DELETE warnings
- Protected resource warnings
- Output format handling (text, json, yaml)
- Auto-approve for automation

### Definition of Done
- [ ] Sync command implemented
- [ ] DELETE operations supported
- [ ] Consistent confirmation with warnings
- [ ] Clear warnings for destructive operations
- [ ] Output formats work correctly
- [ ] Auto-approve enables automation

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

## Step 7.5: Protected Resource Validation

**Status**: Not Started
**Dependencies**: Step 7

### Goal
Implement comprehensive validation and reporting for protected resources.

### Implementation

1. Add validation helpers in `internal/declarative/validator/protected.go`:
```go
type ProtectedResourceValidator struct {
    client *state.KonnectClient
}

// ValidateProtectedChanges checks all blocked changes in a plan
func (v *ProtectedResourceValidator) ValidateProtectedChanges(plan *planner.Plan) (*ProtectedValidationResult, error) {
    result := &ProtectedValidationResult{
        BlockedChanges: []BlockedChange{},
    }
    
    for _, change := range plan.Changes {
        if change.Blocked {
            result.BlockedChanges = append(result.BlockedChanges, BlockedChange{
                ResourceType: change.ResourceType,
                ResourceName: change.ResourceName,
                Action:       change.Action,
                Reason:       change.BlockReason,
            })
        }
    }
    
    return result, nil
}
```

2. Create summary reporting for blocked changes:
```go
type ProtectedValidationResult struct {
    BlockedChanges []BlockedChange
    HasBlockedChanges bool
}

func (r *ProtectedValidationResult) Summary() string {
    if !r.HasBlockedChanges {
        return "No protected resources affected"
    }
    
    var summary strings.Builder
    summary.WriteString("Protected resources blocking changes:\n")
    for _, blocked := range r.BlockedChanges {
        summary.WriteString(fmt.Sprintf("  - %s %s: %s\n", 
            blocked.Action, blocked.ResourceName, blocked.Reason))
    }
    return summary.String()
}
```

3. Integrate with apply and sync commands:
```go
// In apply/sync command
protectedValidator := validator.NewProtectedResourceValidator(client)
protectedResult, err := protectedValidator.ValidateProtectedChanges(plan)
if err != nil {
    return err
}

if protectedResult.HasBlockedChanges {
    fmt.Fprintf(os.Stderr, "\nWarning: %s\n", protectedResult.Summary())
}
```

### Tests Required
- Validation correctly identifies all blocked changes
- Summary formatting is clear and actionable
- Integration with commands shows warnings appropriately
- No false positives for unprotected resources

### Definition of Done
- [ ] Protected resource validator implemented
- [ ] Clear summary reporting of blocked changes
- [ ] Integration with apply and sync commands
- [ ] Users informed about protection before execution

---

## Step 8: Implement Confirmation Prompts

**Status**: Not Started
**Dependencies**: Steps 5, 6

### Goal
Create consistent confirmation prompts for both apply and sync commands.

### Implementation

1. Create `internal/cmd/common/prompts.go`:
```go
func ConfirmExecution(plan *planner.Plan) bool
func DisplayPlanSummary(plan *planner.Plan)
```

2. Implement unified confirmation:
```go
func ConfirmExecution(plan *planner.Plan) bool {
    DisplayPlanSummary(plan)
    
    // Show DELETE warning if applicable
    if plan.Summary.ByAction["DELETE"] > 0 {
        fmt.Println("\nWARNING: This operation will DELETE resources:")
        for _, change := range plan.Changes {
            if change.Action == planner.ActionDelete && !change.Blocked {
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

3. Add consistent summary display:
```go
func DisplayPlanSummary(plan *planner.Plan) {
    fmt.Println("Plan Summary:")
    
    if plan.Summary.ByAction["CREATE"] > 0 {
        fmt.Printf("- Create: %d resources\n", plan.Summary.ByAction["CREATE"])
    }
    if plan.Summary.ByAction["UPDATE"] > 0 {
        fmt.Printf("- Update: %d resources\n", plan.Summary.ByAction["UPDATE"])
    }
    if plan.Summary.ByAction["DELETE"] > 0 {
        fmt.Printf("- Delete: %d resources\n", plan.Summary.ByAction["DELETE"])
    }
    if plan.Summary.BlockedCount > 0 {
        fmt.Printf("- Blocked: %d resources (protected)\n", plan.Summary.BlockedCount)
    }
}
```

### Tests Required
- Unified confirmation flow
- DELETE warning display
- User input handling ('yes' required)
- Auto-approve bypasses prompts
- Summary display accuracy

### Definition of Done
- [ ] Single confirmation function for both commands
- [ ] Consistent prompt requiring 'yes'
- [ ] DELETE operations show clear warning
- [ ] Auto-approve support works

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