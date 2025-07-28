# Stage 7: Testing, Documentation, and Core Improvements - Implementation Steps

## Progress Summary
**Progress**: 1/10 steps completed (10%)  
**Current Step**: Step 2 - Comprehensive Documentation Updates

## Overview
This document outlines the step-by-step implementation plan for completing 
essential testing, documentation, and core improvements for declarative 
configuration management, focusing on production readiness and user experience.

## Implementation Steps

### Step 1: Login Command Migration to Konnect-First
**Status**: Completed

Update login to be Konnect-first without requiring product specification.

**Files to modify**:
- `internal/cmd/root/verbs/login/login.go`
- `internal/cmd/root/verbs/login/konnect.go`

**Changes**:
- Update login command to work without "konnect" subcommand
- Add deprecation warning for old syntax
- Update help text to reflect new usage
- Ensure backward compatibility with `kongctl login konnect`

**Acceptance criteria**:
- `kongctl login` works directly without subcommand
- `kongctl login konnect` still works but shows deprecation warning
- Help text updated to show new usage pattern
- No breaking changes for existing scripts

---

### Step 2: Comprehensive Documentation Updates
**Status**: Not Started

Create comprehensive documentation for the declarative configuration feature.

**Files to create/modify**:
- `README.md` - Add declarative config section
- `docs/declarative-configuration.md` - Complete guide (new)
- `docs/examples/ci-cd-integration.md` - Automation patterns (new)
- `docs/troubleshooting.md` - Common issues and solutions (new)
- Command help text for apply/sync/plan/diff

**Changes**:
- Document apply vs sync command comparison
- Add best practices for declarative configuration
- Create migration guide from imperative to declarative
- Provide CI/CD integration examples
- Enhance command help text with examples

**Acceptance criteria**:
- Complete documentation covers all features
- Examples are tested and working
- CI/CD patterns documented
- Clear migration path from imperative commands
- Help text is comprehensive and helpful

---

### Step 3: Apply Command Integration Tests
**Status**: Not Started

Create thorough integration tests for apply command flows.

**Files to create**:
- `test/integration/declarative/apply_test.go`

**Test scenarios**:
- TestApplyCreateOnly
- TestApplyWithUpdates
- TestApplyRejectsPlanWithDeletes
- TestApplyDryRun
- TestApplyFromPlanFile
- TestApplyWithProtectedResources
- TestApplyIdempotency
- TestApplyOutputFormats
- TestApplyStdinSupport

**Acceptance criteria**:
- All test scenarios passing
- Good coverage of edge cases
- Tests run reliably in CI
- Clear test failure messages

---

### Step 4: Sync Command Integration Tests
**Status**: Not Started

Create thorough integration tests for sync command flows.

**Files to create**:
- `test/integration/declarative/sync_test.go`

**Test scenarios**:
- TestSyncFullReconciliation
- TestSyncDeletesUnmanagedResources
- TestSyncProtectedResourceHandling
- TestSyncConfirmationFlow
- TestSyncAutoApprove
- TestSyncOutputFormats
- TestSyncDryRun

**Acceptance criteria**:
- All test scenarios passing
- Deletion flows tested thoroughly
- Protected resource handling verified
- Output formats work correctly

---

### Step 5: Error Scenario Integration Tests
**Status**: Not Started

Create integration tests for error scenarios and edge cases.

**Files to create**:
- `test/integration/declarative/error_scenarios_test.go`

**Test scenarios**:
- TestExecutorAPIErrors
- TestExecutorPartialFailure
- TestProtectionViolations
- TestNetworkFailures
- TestInvalidConfigurations

**Acceptance criteria**:
- Error scenarios properly handled
- Clear error messages for users
- Partial failures handled gracefully
- Network resilience tested

---

### Step 6: Enhanced Error Messages
**Status**: Not Started

Improve error messages throughout the declarative configuration system.

**Files to modify**:
- `internal/declarative/executor/*.go`
- `internal/declarative/planner/*.go`
- `internal/declarative/loader/loader.go`
- `internal/declarative/state/client.go`

**Changes**:
- Add context to all API errors
- Include resource names in error messages
- Provide actionable hints for common errors
- Consistent error formatting

**Acceptance criteria**:
- Error messages include resource context
- API errors translated to user-friendly messages
- Hints provided for common issues
- Consistent formatting across all errors

---

### Step 7: Improved Plan Summary Display
**Status**: Not Started

Enhance plan summary display for better readability and information.

**Files to modify**:
- `internal/cmd/root/products/konnect/declarative/plan.go`
- `internal/cmd/root/products/konnect/declarative/apply.go`
- `internal/cmd/root/products/konnect/declarative/sync.go`

**Changes**:
- Add visual separators and formatting
- Show field-level changes for updates
- Better grouping of resources
- Include protected resource warnings
- Add total summary statistics

**Acceptance criteria**:
- Plan summaries are clear and readable
- Updates show what's changing
- Protected resources clearly marked
- Statistics help understand scope

---

### Step 8: Progress Indicators for Long Operations
**Status**: Not Started

Add progress reporting for long-running operations.

**Files to modify**:
- `internal/declarative/executor/types.go`
- `internal/declarative/executor/*_executor.go`
- `internal/cmd/root/products/konnect/declarative/apply.go`
- `internal/cmd/root/products/konnect/declarative/sync.go`

**Changes**:
- Create ProgressReporter interface
- Implement console progress reporter
- Add progress callbacks to executors
- Show progress during apply/sync operations

**Acceptance criteria**:
- Progress shown for operations taking >2 seconds
- Clear indication of current operation
- Non-intrusive for fast operations
- Works with different output formats

---

### Step 9: Migrate Dump Command to Public SDK
**Status**: Not Started

Complete the migration from internal to public Konnect SDK for dump command.

**Files to modify**:
- `internal/cmd/root/verbs/dump/dump.go`
- `internal/cmd/root/verbs/dump/konnect.go`

**Changes**:
- Replace internal SDK imports with public SDK
- Update API calls to use public SDK methods
- Ensure all functionality maintained
- Remove internal SDK dependency

**Acceptance criteria**:
- Dump command works with public SDK
- No internal SDK imports remain
- All existing functionality preserved
- Tests pass with new implementation

---

### Step 10: Code Quality and Refactoring
**Status**: Not Started

Conduct focused code review and refactoring for maintainability.

**Focus areas**:
- Reduce code duplication across resource types
- Extract common patterns to shared utilities
- Improve error handling consistency
- Simplify complex functions
- Increase test coverage to >80%

**Files to review**:
- All files in `internal/declarative/`
- Resource-specific executors and planners
- State management client
- Command implementations

**Acceptance criteria**:
- Code duplication significantly reduced
- Complex functions broken down
- Test coverage >80%
- Consistent error handling patterns
- Code is more maintainable

---

## Summary

Total steps: 10

Implementation order is designed to:
1. Start with user-facing improvements (Steps 1-2)
2. Ensure quality through testing (Steps 3-5)
3. Enhance user experience (Steps 6-8)
4. Complete technical debt (Steps 9-10)

Each step can be implemented and tested independently, allowing for incremental progress.