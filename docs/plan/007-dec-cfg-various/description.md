# KongCtl Stage 7 - Testing, Documentation, and Core Improvements

## Goal
Complete essential testing, documentation, and core improvements for declarative configuration management, focusing on production readiness and user experience.

## Deliverables (in priority order)

### 1. Complete Documentation Updates

Create comprehensive documentation for the declarative configuration feature.

#### Main README.md Updates
- Apply vs sync command comparison
- Best practices for declarative configuration
- Migration guide from imperative to declarative
- CI/CD integration examples

#### New Documentation Files
- `docs/declarative-configuration.md` - Complete guide
- `docs/examples/ci-cd-integration.md` - Automation patterns
- `docs/troubleshooting.md` - Common issues and solutions

#### Enhanced Command Help Text
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

  # Apply with specific config files
  kongctl apply --filename ./portals

  # Preview changes without applying (dry-run)
  kongctl apply --dry-run

  # Apply from a pre-generated plan
  kongctl apply --plan plan.json

  # CI/CD automation with auto-approve
  kongctl apply --auto-approve --output json

Flags:
      --auto-approve              Skip confirmation prompt
      --dry-run                   Preview changes without applying
  -f, --filename strings          Path to configuration files or directories
  -h, --help                      help for apply
      --output string             Output format (text|json|yaml) (default "text")
      --plan string               Path to existing plan file (mutually exclusive with --filename)
```

### 2. Login Command Migration to Konnect-First

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

### 3. Comprehensive Integration Tests

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
TestProtectionViolations
TestNetworkFailures
TestInvalidConfigurations
```

### 4. Critical UX Improvements

#### Enhanced Error Messages
```go
// Instead of: "failed to create portal: 409"
// Show: "failed to create portal 'dev-portal': a portal with this name already exists"

// Add context to all errors
return fmt.Errorf("failed to %s %s %q: %w", 
    action, resourceType, resourceName, apiError)
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

#### Progress Indicators for Long Operations
```go
// For long-running operations
type ProgressReporter interface {
    StartChange(change PlannedChange)
    UpdateProgress(percent int, message string)
    CompleteChange(change PlannedChange, err error)
}
```

### 5. Migrate Remaining Internal SDK Usage

Complete the migration from internal to public Konnect SDK for all remaining commands.

#### Dump Command Migration
```go
// Current: internal/cmd/root/verbs/dump/dump.go uses internal SDK
// Migrate to use public SDK APIs where available

// Before (using internal SDK):
import kkInternal "github.com/Kong/sdk-konnect-go-internal"

// After (using public SDK):
import kkSDK "github.com/Kong/sdk-konnect-go"
```

### 6. Code Quality and Refactoring

Conduct focused code review and refactoring for maintainability.

#### Priority Areas
1. **Reduce Code Duplication**
   - Extract common patterns across resource types
   - Create shared utilities for repeated logic
   
2. **Improve Error Handling**
   - Consistent error wrapping
   - User-friendly error messages
   
3. **Simplify Complex Functions**
   - Break down large functions
   - Extract complex logic
   
4. **Test Coverage**
   - Target >80% coverage
   - Focus on error paths and edge cases

## Tests Required
- Login command works with both old and new syntax
- Integration tests cover all major workflows
- Documentation is accurate and helpful
- Error messages provide actionable information
- Internal SDK migration maintains functionality
- Refactoring doesn't break existing features

## Proof of Success
```bash
# Simpler login
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
Coverage: 85%

# Internal SDK migration complete
$ grep -r "sdk-konnect-go-internal" --include="*.go" .
✓ No internal SDK usage found
```

## Dependencies
- Stages 1-6 completion
- User feedback on current implementation
- Understanding of common user workflows

## Notes
- Focus on production readiness and user experience
- Prioritize documentation as it enables adoption
- Keep scope focused on essential improvements
- Consider maintenance burden in all decisions