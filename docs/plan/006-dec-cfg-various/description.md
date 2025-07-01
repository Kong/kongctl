# KongCtl Stage 6 - Various Improvements and Testing

## Goal
Complete remaining improvements, UX enhancements, and comprehensive testing for declarative configuration management.

## Deliverables
- Configuration discovery feature
- Plan validation framework
- Login command migration to Konnect-first
- Comprehensive integration tests
- Complete documentation updates
- Various UX improvements
- Extensive code review and refactoring

## Sub-Tasks

### 1. Configuration Discovery Feature

Implement visibility into unmanaged fields to help users progressively build configurations.

```go
// Add to apply/sync commands
cmd.Flags().Bool("show-unmanaged", false, "Show unmanaged fields after execution")

// Discovery logic
type UnmanagedFields struct {
    ResourceType string
    ResourceName string
    Fields       map[string]interface{}
}

func DiscoverUnmanagedFields(current state.Portal, desired resources.PortalResource) UnmanagedFields {
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

Expected output:
```
Discovered unmanaged fields for portal "my-portal":
  - display_name: "Developer Portal"
  - authentication_enabled: true
  - rbac_enabled: false

To manage these fields, add them to your configuration.
```

### 2. Plan Validation Framework

Implement comprehensive validation before plan execution.

```go
// internal/declarative/validator/validator.go
type PlanValidator struct {
    client *state.KonnectClient
}

func (v *PlanValidator) ValidateForApply(plan *planner.Plan) error {
    // Verify no DELETE operations
    // Check resource states haven't changed
    // Validate protection status
    // Ensure references are valid
}

func (v *PlanValidator) ValidateForSync(plan *planner.Plan) error {
    // Check resource states
    // Validate protection status
    // Verify managed labels
}
```

### 3. Login Command Migration

Update login to be Konnect-first without requiring product specification.

```go
// Before: kongctl login konnect
// After: kongctl login

func NewCommand() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "login",
        Short: "Authenticate with Kong Konnect",
        Long:  "Authenticate with Kong Konnect using device authorization flow",
        RunE:  runLogin,
    }
    
    // Future: Add --product flag for other products
    return cmd
}

// Add deprecation warning
if len(args) > 0 && args[0] == "konnect" {
    fmt.Fprintf(os.Stderr, "Warning: 'kongctl login konnect' is deprecated. Use 'kongctl login' instead.\n")
}
```

### 4. Comprehensive Integration Tests

Create thorough integration tests for all declarative configuration flows.

#### Apply Command Tests
```go
// test/integration/apply_test.go
TestApplyCreateOnly
TestApplyWithUpdates
TestApplyRejectsPlanWithDeletes
TestApplyDryRun
TestApplyFromPlanFile
TestApplyWithProtectedResources
TestApplyIdempotency
TestApplyOutputFormats
TestApplyStdinSupport
```

#### Sync Command Tests
```go
// test/integration/sync_test.go
TestSyncFullReconciliation
TestSyncDeletesUnmanagedResources
TestSyncProtectedResourceHandling
TestSyncConfirmationFlow
TestSyncAutoApprove
TestSyncOutputFormats
TestSyncDryRun
```

#### Error Scenario Tests
```go
// test/integration/error_scenarios_test.go
TestExecutorAPIErrors
TestExecutorPartialFailure
TestPlanValidationErrors
TestProtectionViolations
TestNetworkFailures
TestInvalidConfigurations
```

### 5. Documentation Updates

#### Main README.md Updates
- Apply vs sync command comparison
- Best practices for declarative configuration
- Migration guide from imperative to declarative
- CI/CD integration examples

#### New Documentation Files
- `docs/declarative-configuration.md` - Complete guide
- `docs/examples/ci-cd-integration.md` - Automation patterns
- `docs/troubleshooting.md` - Common issues and solutions

#### Command Help Text
```bash
$ kongctl apply --help
Apply configuration changes (create/update only)

This command creates new resources and updates existing ones based on
your configuration files. It never deletes resources, making it safe
for incremental updates in production.

Usage:
  kongctl apply [flags]

Examples:
  # Apply configuration from current directory
  kongctl apply

  # Apply with specific config directory
  kongctl apply --config ./portals

  # Preview changes without applying (dry-run)
  kongctl apply --dry-run

  # Apply from a pre-generated plan
  kongctl apply --plan plan.json

  # CI/CD automation with auto-approve
  kongctl apply --auto-approve --output json

Flags:
      --auto-approve    Skip confirmation prompt
      --config string   Path to configuration directory (default ".")
      --dry-run        Preview changes without applying
  -h, --help           help for apply
      --output string  Output format (text|json|yaml) (default "text")
      --plan string    Path to existing plan file
      --show-unmanaged Show unmanaged fields after execution
```

### 6. Additional UX Improvements

#### Enhanced Error Messages
```go
// Instead of: "failed to create portal: 409"
// Show: "failed to create portal 'dev-portal': a portal with this name already exists"

// Add context to all errors
return fmt.Errorf("failed to %s %s %q: %w", 
    action, resourceType, resourceName, apiError)
```

#### Progress Indicators
```go
// For long-running operations
type ProgressReporter interface {
    StartChange(change PlannedChange)
    UpdateProgress(percent int, message string)
    CompleteChange(change PlannedChange, err error)
}
```

#### Improved Plan Summary Display
```
Plan Summary:
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Resources to Create (2):
  + portal: developer-portal
  + portal: staging-portal

Resources to Update (1):
  ~ portal: production-portal
    - description: "Old description" → "New description"
    - display_name: "Prod" → "Production Portal"

Resources to Delete (1):
  - portal: deprecated-portal

Protected Resources (1):
  ⚠ portal: critical-portal (skipped)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Total: 5 resources (2 create, 1 update, 1 delete, 1 protected)
```

### 7. Extensive Code Review and Refactoring

Conduct a comprehensive code review focusing on code quality, maintainability, and architectural improvements.

#### Review Focus Areas

1. **Code Duplication (DRY)**
   - Identify repeated patterns across resource types
   - Extract common functionality into shared utilities
   - Create generic interfaces where appropriate

2. **Type-Specific Functions**
   - Review adapter functions for each resource type
   - Evaluate generic solutions to reduce boilerplate
   - Consider code generation for repetitive patterns

3. **Error Handling**
   - Ensure consistent error wrapping and context
   - Verify error messages are user-friendly
   - Check for proper error propagation

4. **Code Organization**
   - Review package structure and dependencies
   - Identify circular dependencies
   - Ensure clear separation of concerns

5. **Testing Coverage**
   - Identify untested code paths
   - Review test quality and maintainability
   - Ensure edge cases are covered

#### Refactoring Targets

```go
// Example: Generic resource operations to reduce duplication
type ResourceOperations[T any] interface {
    Create(ctx context.Context, resource T) error
    Update(ctx context.Context, id string, resource T) error
    Delete(ctx context.Context, id string) error
}

// Example: Common field comparison logic
func CompareFields(current, desired map[string]interface{}) (changes map[string]interface{})

// Example: Shared validation patterns
type Validator interface {
    Validate() error
}
```

#### Code Quality Improvements

1. **Reduce Complexity**
   - Break down large functions
   - Simplify nested conditionals
   - Extract complex logic into well-named functions

2. **Improve Naming**
   - Ensure consistent naming conventions
   - Use descriptive variable and function names
   - Align with Go idioms

3. **Documentation**
   - Add missing godoc comments
   - Update outdated documentation
   - Include examples where helpful

4. **Performance**
   - Review for unnecessary allocations
   - Optimize hot paths
   - Consider concurrent operations where safe

#### Review Process

1. **Static Analysis**
   ```bash
   # Run linters with strict settings
   golangci-lint run --enable-all
   
   # Check for cyclomatic complexity
   gocyclo -over 10 .
   
   # Review code coverage
   go test -coverprofile=coverage.out ./...
   go tool cover -html=coverage.out
   ```

2. **Manual Review Checklist**
   - [ ] No code duplication across resource types
   - [ ] Consistent error handling patterns
   - [ ] Clear package boundaries
   - [ ] Adequate test coverage (>80%)
   - [ ] No TODO comments without issues
   - [ ] All exported functions have godoc
   - [ ] No magic numbers or strings

3. **Architectural Review**
   - [ ] Clear separation between layers
   - [ ] Minimal coupling between packages
   - [ ] Consistent patterns across codebase
   - [ ] Future extensibility considered

## Tests Required
- Configuration discovery accuracy
- Plan validation catches all error cases
- Login command migration works smoothly
- Integration tests cover all scenarios
- Documentation is clear and complete
- UX improvements enhance usability
- Code review findings are addressed
- Refactoring maintains functionality

## Proof of Success
```bash
# Configuration discovery helps users
$ kongctl apply --show-unmanaged
✓ Applied successfully

Discovered unmanaged fields:
portal "my-portal":
  - rbac_enabled: false
  - auto_approve_developers: true

# Login is simpler
$ kongctl login
Opening browser for authentication...
✓ Successfully authenticated to Kong Konnect

# Clear error messages
$ kongctl apply
Error: Cannot create portal "dev-portal": a portal with this name already exists.
Hint: Use 'kongctl sync' to take ownership of existing resources.

# Comprehensive test coverage
$ make test-integration
Running integration tests...
✓ 45 tests passed
Coverage: 92.3%

# Code quality improvements
$ golangci-lint run
✓ No issues found

$ gocyclo -over 10 .
✓ All functions below complexity threshold

# Review complete
✓ Removed 300+ lines of duplicated code
✓ Extracted 5 common interfaces
✓ Improved error messages across codebase
✓ Test coverage increased to 85%
```

## Dependencies
- Stages 1-5 completion
- Understanding of user workflows
- Feedback from early adopters

## Notes
- Focus on developer experience and usability
- Ensure all features are well-tested
- Documentation should cover real-world scenarios
- Consider future extensibility in all designs