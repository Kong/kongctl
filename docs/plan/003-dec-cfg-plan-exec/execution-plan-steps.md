# Stage 3: Plan Execution - Implementation Steps

## Progress Summary

| Step | Description | Status | Dependencies |
|------|-------------|---------|--------------|
| 1 | Enhance planner with mode support | Completed ✅ | - |
| 2 | Create base executor package | Completed ✅ | Step 1 |
| 3 | Implement progress reporter | Completed ✅ | Step 2 |
| 4 | Add portal operations to executor | Completed ✅ | Step 2 |
| 5 | Implement apply command | Completed ✅ | Steps 3, 4 |
| 5a | Fix idempotency issue | Completed ✅ | Step 5 |
| 5b | Add configuration discovery | Not Started | Step 5a |
| 6 | Implement sync command | Not Started | Steps 3, 4 |
| 7 | Add plan validation | Not Started | Steps 5, 6 |
| 8 | Implement confirmation prompts | Partially Complete | Steps 5, 6 |
| 9 | Migrate login to Konnect-first | Not Started | - |
| 10 | Add integration tests | Not Started | Steps 5, 6, 9 |
| 11 | Update documentation | Not Started | All steps |

**Current Stage**: Step 5a completed, ready for Step 5b (Configuration Discovery)

**Note**: Step 8 is marked as "Partially Complete" because confirmation prompts are implemented in apply command but sync command doesn't exist yet.

## Recent Improvements (Beyond Original Plan)

During implementation of Steps 1-5a, several improvements were made based on real-world usage:

1. **Protection Label Enhancement**: Changed from adding/removing labels to always present with true/false value
2. **Full Portal Field Support**: Added support for all portal fields (authentication_enabled, rbac_enabled, etc.)
3. **Output Format Improvements**: Enhanced consistency and structure of JSON/YAML outputs
4. **stdin Support**: Added ability to use stdin with interactive prompts via /dev/tty
5. **User Experience**: Various fixes for edge cases and improved error messages

---

## Step 1: Enhance Planner with Mode Support

**Status**: Completed ✅

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

4. Add protection detection with fail-fast behavior:
```go
// During plan generation, check protection before adding changes
func (p *Planner) validateProtection(current, desired Resource, action ActionType) error {
    if action == ActionUpdate || action == ActionDelete {
        if isProtected(current) {
            return fmt.Errorf("resource %s %q is protected and cannot be %s",
                current.Type, current.Name, strings.ToLower(string(action)))
        }
    }
    return nil
}

// Collect all protection errors and fail fast
var protectionErrors []error
for _, resource := range resources {
    if err := p.validateProtection(current, desired, action); err != nil {
        protectionErrors = append(protectionErrors, err)
    }
}
if len(protectionErrors) > 0 {
    return nil, fmt.Errorf("cannot generate plan due to protected resources:\n%v", 
        joinErrors(protectionErrors))
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
- Protected resources cause plan generation to fail
- Error messages list all protected resources clearly
- Protection only blocks modifications, not removal of protection itself

### Definition of Done
- [x] Planner supports mode parameter
- [x] Apply mode excludes DELETE operations
- [x] Sync mode includes all operations
- [x] Plan metadata indicates generation mode
- [x] Protected resources cause planning to fail
- [x] Clear error messages for protection violations
- [x] Tests pass for both modes and protection scenarios

---

## Step 2: Create Base Executor Package

**Status**: Completed ✅
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
- [x] Executor package structure created
- [x] Core execution loop implemented
- [x] Dry-run mode supported
- [x] Unit tests for executor logic

---

## Step 3: Implement Progress Reporter

**Status**: Completed ✅
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
- [x] Progress reporter interface defined
- [x] Console reporter implemented
- [x] Executor integrated with reporter
- [x] Clear, informative output format

---

## Step 4: Add Portal Operations to Executor

**Status**: Completed ✅
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
- Validate protection status at execution time
- If protected, skip and report as error
- For unprotected resources:
  - Update labels with new hash
  - Call update API
  - Preserve certain fields if needed

4. Implement delete operation:
- Validate protection status at execution time
- If protected, skip and report as error
- For unprotected resources:
  - Verify resource is managed
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
- Protection validation at execution time
- Protected resources generate execution errors
- Error handling for each operation
- Protection changes between planning and execution detected

### Definition of Done
- [x] Portal operations implemented
- [x] Label management integrated
- [x] Protection validated at execution time
- [x] Protection errors handled correctly
- [x] Comprehensive error handling

---

## Step 5: Implement Apply Command

**Status**: Completed ✅
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
- [x] Apply command implemented
- [x] Plan validation prevents DELETEs
- [x] Confirmation prompt works
- [x] Integration with executor
- [x] Clear output and error messages
- [x] Output formats work correctly
- [x] Auto-approve enables automation

---

## Step 5a: Fix Idempotency Issue

**Status**: Completed ✅
**Dependencies**: Step 5

### Goal
Fix the issue where consecutive apply commands detect changes when no actual changes exist due to API-added default values.

### Context
The current hash-based change detection fails to achieve idempotency because:
- API adds default values not present in user configuration
- Hash comparison includes these defaults, causing false positives
- Every apply triggers unnecessary updates

### Implementation

1. **Remove hash-based comparison** from `internal/declarative/planner/planner.go`:
```go
// Remove or deprecate:
// - CalculatePortalHash and hash comparison logic
// - KONGCTL-config-hash label usage for comparison
// - Keep hash calculation only if needed for backwards compatibility
```

2. **Implement configuration-based comparison**:
```go
// New approach in planPortalChanges
func (p *Planner) shouldUpdatePortal(current state.Portal, desired resources.PortalResource) (bool, map[string]interface{}) {
    updates := make(map[string]interface{})
    
    // Only compare fields present in desired configuration
    if desired.Description != nil {
        if current.Description == nil || *current.Description != *desired.Description {
            updates["description"] = *desired.Description
        }
    }
    
    if desired.DisplayName != nil {
        if current.DisplayName != *desired.DisplayName {
            updates["display_name"] = *desired.DisplayName
        }
    }
    
    // Continue for all user-configurable fields...
    
    return len(updates) > 0, updates
}
```

3. **Update plan generation logic**:
```go
// In planPortalChanges
if exists {
    needsUpdate, updates := p.shouldUpdatePortal(current, desired)
    if needsUpdate {
        p.planPortalUpdate(current, desired, updates, plan)
    }
    // Remove hash comparison entirely
}
```

4. **Fix "no changes" confirmation**:
```go
// In apply command
if len(plan.Changes) == 0 {
    fmt.Println("No changes needed. Resources match configuration.")
    return nil // Skip confirmation
}
```

5. **Update portal operations** for sparse updates:
```go
// In updatePortal, only send fields that changed
func (e *Executor) updatePortal(ctx context.Context, change planner.PlannedChange) (string, error) {
    var updateRequest kkInternalComps.UpdatePortal
    
    // Only include fields from change.Fields that actually changed
    for field, value := range change.Fields {
        switch field {
        case "description":
            desc := value.(string)
            updateRequest.Description = &desc
        case "display_name":
            name := value.(string)
            updateRequest.DisplayName = &name
        // ... other fields
        }
    }
    
    // Send sparse update
    resp, err := e.client.UpdatePortal(ctx, change.ResourceID, updateRequest, "")
}
```

### Tests Required
- Consecutive applies with no changes show "No changes needed"
- Minimal config ignores API defaults
- Only user-specified fields trigger updates
- Sparse updates send only changed fields
- Adding new fields to config triggers appropriate updates

### Definition of Done
- [x] Hash comparison removed/deprecated
- [x] Configuration-based comparison implemented
- [x] No confirmation prompt when no changes
- [x] Sparse updates working
- [x] Tests verify idempotency

---

## Step 5b: Add Configuration Discovery

**Status**: Not Started
**Dependencies**: Step 5a

### Goal
Implement a feature to help users discover unmanaged fields and progressively build their configurations.

### Context
With configuration-based change detection, users need visibility into:
- What fields are available but not managed
- Current values of unmanaged fields
- How to add these fields to their configuration

### Implementation

1. **Add discovery logic** to planner:
```go
type UnmanagedFields struct {
    ResourceType string
    ResourceName string
    Fields       map[string]interface{}
}

func (p *Planner) DiscoverUnmanagedFields(current state.Portal, desired resources.PortalResource) UnmanagedFields {
    unmanaged := UnmanagedFields{
        ResourceType: "portal",
        ResourceName: current.Name,
        Fields:       make(map[string]interface{}),
    }
    
    // Check each field in current state
    if desired.DisplayName == nil && current.DisplayName != "" {
        unmanaged.Fields["display_name"] = current.DisplayName
    }
    
    if desired.AuthenticationEnabled == nil {
        unmanaged.Fields["authentication_enabled"] = current.AuthenticationEnabled
    }
    
    // Continue for all fields...
    
    return unmanaged
}
```

2. **Integrate with apply/sync commands**:
```go
// Add to command flags
cmd.Flags().Bool("show-unmanaged", false, "Show unmanaged fields after execution")

// After successful execution
if showUnmanaged {
    unmanaged := discoverAllUnmanagedFields(plan, currentState)
    displayUnmanagedFields(unmanaged)
}
```

3. **Create display formatting**:
```go
func displayUnmanagedFields(unmanaged []UnmanagedFields) {
    if len(unmanaged) == 0 {
        return
    }
    
    fmt.Println("\nDiscovered unmanaged fields:")
    fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
    
    for _, resource := range unmanaged {
        if len(resource.Fields) > 0 {
            fmt.Printf("\n%s: %s\n", resource.ResourceType, resource.ResourceName)
            for field, value := range resource.Fields {
                fmt.Printf("  %s: %v\n", field, value)
            }
        }
    }
    
    fmt.Println("\nTo manage these fields, add them to your configuration.")
}
```

4. **Add verbose discovery mode** (future enhancement):
```go
// With --verbose flag
if verbose {
    // Include field descriptions
    // Show which fields are required vs optional
    // Display allowed values for enums
}
```

### Tests Required
- Discovery correctly identifies unmanaged fields
- No discovery output when all fields are managed
- Discovery works across multiple resources
- Output format is clear and actionable

### Definition of Done
- [ ] Discovery logic implemented
- [ ] --show-unmanaged flag added to commands
- [ ] Clear output format for discovered fields
- [ ] Integration tests verify discovery
- [ ] User documentation updated

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