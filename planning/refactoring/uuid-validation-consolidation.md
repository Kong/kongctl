# UUID Validation Consolidation Refactoring Plan

## Problem Statement
UUID validation regex patterns are duplicated across 12+ files with inconsistent implementations:
- Different regex patterns (case-sensitive vs case-insensitive)
- Different validation approaches (regex vs string checks)
- Performance impact from recompiling regex on each use

## Solution
Create a centralized UUID validation helper in `internal/util/uuid.go` with:
- Single compiled regex pattern (case-insensitive for maximum compatibility)
- Consistent validation logic
- Better performance through regex compilation once

## Implementation Steps

### 1. Create UUID Helper (internal/util/uuid.go)
```go
package util

import "regexp"

var uuidRegex = regexp.MustCompile(`^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$`)

// IsValidUUID checks if a string is a valid UUID format
func IsValidUUID(s string) bool {
    return uuidRegex.MatchString(s)
}
```

### 2. Replace Duplicated Implementations

**Files to update:**
1. `internal/declarative/resources/api_implementation.go` - Remove local `isValidUUID`, use `util.IsValidUUID`
2. `internal/declarative/planner/resolver.go` - Remove local `isUUID`, use `util.IsValidUUID`
3. `internal/declarative/executor/api_publication_operations.go` - Remove local `isUUID`, use `util.IsValidUUID`
4. `internal/cmd/root/products/konnect/portal/deletePortal.go` - Replace inline regex with `util.IsValidUUID`
5. `internal/cmd/root/products/konnect/portal/getPortal.go` - Replace inline regex with `util.IsValidUUID`
6. `internal/cmd/root/products/konnect/api/getAPI.go` - Replace inline regex with `util.IsValidUUID`
7. `internal/cmd/root/products/konnect/authstrategy/getAuthStrategy.go` - Replace inline regex with `util.IsValidUUID`
8. `internal/cmd/root/products/konnect/gateway/route/getRoute.go` - Replace inline regex with `util.IsValidUUID`
9. `internal/cmd/root/products/konnect/gateway/controlplane/getControlPlane.go` - Replace inline regex with `util.IsValidUUID`
10. `internal/cmd/root/products/konnect/gateway/consumer/getConsumer.go` - Replace inline regex with `util.IsValidUUID`
11. `internal/cmd/root/products/konnect/gateway/service/getService.go` - Replace inline regex with `util.IsValidUUID`

### 3. Add Unit Tests (internal/util/uuid_test.go)
Test cases for:
- Valid UUIDs (lowercase, uppercase, mixed case)
- Invalid formats (wrong length, missing dashes, invalid characters)
- Edge cases (empty string, nil-like UUID)

## Benefits
- **Single source of truth** for UUID validation
- **Consistent behavior** across all commands
- **Better performance** (regex compiled once)
- **Easier maintenance** (one place to update)
- **Reduced code duplication** (~30 lines removed)

## Quality Checks
After implementation:
1. `make build` - Ensure compilation succeeds
2. `make lint` - Verify no linting issues
3. `make test` - Confirm all tests pass

## Execution Status
- [x] Plan created and approved
- [x] UUID helper implementation
- [x] Replace duplicated implementations (11 files)
- [x] Add unit tests
- [x] Quality checks (build, lint, test)

## Summary
Successfully consolidated UUID validation from 12+ files into a single helper function:
- **Files refactored**: 11 source files + 2 test files
- **Lines removed**: ~30 lines of duplicated code
- **New functionality**: Case-insensitive UUID validation (improved from mixed implementations)
- **Performance**: Regex compiled once vs multiple compilations per validation
- **Test coverage**: Comprehensive test suite with 25+ test cases