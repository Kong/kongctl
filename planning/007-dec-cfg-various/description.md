# KongCtl Stage 7 - Testing, Documentation, and Core Improvements

## Goal
Complete essential testing, documentation, and core improvements for declarative configuration management, focusing on production readiness and user experience. Expand imperative command support for resource parity with declarative commands and implement a complete "Konnect-First" approach across all commands.

## Deliverables (in priority order)

### 1. Login Command Migration to Konnect-First (COMPLETED)

Update login to be Konnect-first without requiring product specification.

```bash
# Before: kongctl login konnect
# After: kongctl login
```

### 2. Rename Gateway Product to On-Prem

Rename the 'gateway' product to 'on-prem' to disambiguate between Konnect gateway resources and on-premises Kong Gateway resources.

```bash
# Current (ambiguous):
kongctl get konnect gateway control-planes  # Konnect-hosted
kongctl get gateway services                # On-prem (but unclear)

# After (clear):
kongctl get konnect gateway control-planes  # Konnect-hosted
kongctl get on-prem services                # On-premises
```

**Implementation**:
- Rename `/internal/cmd/root/products/gateway/` to `/internal/cmd/root/products/on-prem/`
- Update product constant from `gateway` to `on-prem`
- Update all i18n keys and examples
- Add comment noting this naming may change in the future
- Integrate with verb commands (currently missing)

### 3. Imperative Command Expansion

Extend imperative `get` command support to achieve parity with declarative configuration resources.

#### Get Command for Portals
```bash
# List all portals
kongctl get portals

# Get specific portal
kongctl get portal developer-portal

# Output formats
kongctl get portals -o json
kongctl get portals -o yaml
```

#### Get Command for APIs
```bash
# List all APIs
kongctl get apis

# Get specific API
kongctl get api users-api

# Include child resources
kongctl get api users-api --include-versions
kongctl get api users-api --include-publications
```

#### Get Command for Auth Strategies
```bash
# List all auth strategies
kongctl get auth-strategies

# Get specific auth strategy
kongctl get auth-strategy oauth2-strategy

# Filter by type
kongctl get auth-strategies --type oauth2
```

**Implementation Guidelines**:
- Follow existing patterns from control-planes, services, routes
- Ensure consistent behavior and UI/UX
- Support standard output formats (text, json, yaml)
- Use same authentication and error handling patterns
- Implement proper pagination for list operations

### 4. Complete Konnect-First Migration

Apply the Konnect-first pattern to ALL verb commands, making Konnect the default product.

```bash
# Examples of Konnect-first behavior:
kongctl get gateway control-planes      # Defaults to Konnect
kongctl list gateway services           # Defaults to Konnect
kongctl create gateway route            # Defaults to Konnect
kongctl delete gateway service          # Defaults to Konnect

# Explicit product specification still works:
kongctl get konnect gateway control-planes
kongctl get on-prem services
```

**Implementation Pattern**:
- Apply same pattern used for login and declarative commands
- Maintain backward compatibility
- Update help text and examples
- Ensure consistent behavior across all verbs

### 5. Complete Documentation Updates

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

### 6. Comprehensive Integration Tests

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

### 7. Critical UX Improvements

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

### 8. Migrate Remaining Internal SDK Usage

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

### 9. Code Quality and Refactoring

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
- Gateway → on-prem rename works correctly
- New imperative commands work for portals, APIs, auth strategies
- All commands support Konnect-first behavior
- Backward compatibility maintained
- Login command works with both old and new syntax
- Integration tests cover all major workflows
- Documentation is accurate and helpful
- Error messages provide actionable information
- Internal SDK migration maintains functionality
- Refactoring doesn't break existing features

## Proof of Success
```bash
# Clear product distinction
$ kongctl get on-prem services
✓ Listed 5 services from on-premises Kong Gateway

$ kongctl get gateway control-planes
✓ Listed 3 control planes from Kong Konnect

# New imperative commands working
$ kongctl get portals
NAME                DISPLAY NAME           AUTHENTICATION
developer-portal    Developer Portal       enabled
partner-portal      Partner Portal         enabled

$ kongctl get apis
NAME         VERSION    LABELS
users-api    v2.0.0     team=identity
products-api v1.5.0     team=ecommerce

# Konnect-first behavior
$ kongctl get gateway services
✓ Defaulting to Konnect (use 'kongctl get on-prem services' for on-premises)

# Comprehensive test coverage
$ make test-integration
Running integration tests...
✓ 75 tests passed
Coverage: 85%
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
- The gateway → on-prem rename may change in future based on product decisions