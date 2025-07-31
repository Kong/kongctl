# Plan: Fix Portal Custom Domain Configuration Error

## Problem Summary

When running `k sync -f docs/examples/declarative/namespace/single-team -R`, the command fails with:
```
Error: failed to load configuration: invalid portal_custom_domain '': invalid custom domain ref: ref cannot be empty
```

The root cause is that the `ref` field from nested custom domains is not preserved during the extraction process in `extractNestedResources()`.

## Root Cause Analysis

In `internal/declarative/loader/loader.go` (lines 936-940), when extracting nested custom domains:
```go
if portal.CustomDomain != nil {
    customDomain := *portal.CustomDomain  // Shallow copy loses the Ref field
    customDomain.Portal = portal.Ref     // Only parent ref is set
    rs.PortalCustomDomains = append(rs.PortalCustomDomains, customDomain)
}
```

The `Ref` field from the original custom domain is not explicitly preserved, causing validation to fail.

## Implementation Plan

### Phase 1: Immediate Bug Fix (High Priority)

#### 1.1 Fix Custom Domain Extraction
**File**: `internal/declarative/loader/loader.go`
**Location**: Lines 936-940 in `extractNestedResources()` function
**Change**:
```go
if portal.CustomDomain != nil {
    customDomain := *portal.CustomDomain
    // Preserve the original ref if it exists
    if portal.CustomDomain.Ref != "" {
        customDomain.Ref = portal.CustomDomain.Ref
    }
    customDomain.Portal = portal.Ref
    rs.PortalCustomDomains = append(rs.PortalCustomDomains, customDomain)
}
```

#### 1.2 Add Unit Test for Extraction
**File**: `internal/declarative/loader/loader_test.go` (create if doesn't exist)
**Test Case**:
- Test that nested custom domain extraction preserves all fields including Ref
- Test with various combinations of fields populated
- Test edge cases (empty ref, missing fields)

### Phase 2: Improve Error Messages (Medium Priority)

#### 2.1 Enhance Validation Error Message
**File**: `internal/declarative/loader/validator.go`
**Location**: Line 343 in `validateResourceSet()` function
**Change**:
```go
// Current:
return fmt.Errorf("invalid portal_custom_domain %q: %w", domain.GetRef(), err)

// Improved:
var errorMsg string
if domain.GetRef() == "" {
    errorMsg = fmt.Sprintf("invalid portal_custom_domain (ref is empty, parent portal: %q): %w", 
        domain.Portal, err)
} else {
    errorMsg = fmt.Sprintf("invalid portal_custom_domain %q (parent portal: %q): %w", 
        domain.GetRef(), domain.Portal, err)
}

// Add file context if available
if domain.GetMeta() != nil && domain.GetMeta().FilePath != "" {
    errorMsg = fmt.Sprintf("%s [file: %s]", errorMsg, domain.GetMeta().FilePath)
}

return fmt.Errorf(errorMsg)
```

#### 2.2 Add Context to Resource Metadata
**File**: `internal/declarative/loader/loader.go`
**Location**: In `parseYAML()` function, after creating resources
**Change**:
- Store the source file path in resource metadata for better error reporting
- Pass this context through the extraction process

### Phase 3: Add Trace Logging (Medium Priority)

#### 3.1 Add Extraction Logging
**File**: `internal/declarative/loader/loader.go`
**Location**: Throughout `extractNestedResources()` function
**Changes**:
```go
// At the start of portal custom domain extraction
l.logger.Trace("extracting custom domain from portal", 
    "portal_ref", portal.Ref, 
    "has_custom_domain", portal.CustomDomain != nil)

// After extraction
if portal.CustomDomain != nil {
    l.logger.Trace("extracted custom domain", 
        "portal_ref", portal.Ref,
        "custom_domain_ref", customDomain.Ref,
        "domain", customDomain.Domain)
}
```

### Phase 4: Comprehensive Testing (High Priority)

#### 4.1 Integration Test
**File**: `test/integration/declarative/portal_custom_domain_test.go` (create new)
**Test Cases**:
1. Test successful sync with portal containing custom domain
2. Test error handling when ref is missing
3. Test with multiple portals having custom domains
4. Test namespace inheritance for custom domains

#### 4.2 Example Configuration Test
**File**: Add test that specifically loads `docs/examples/declarative/namespace/single-team`
- Ensure the example configuration works correctly after the fix

### Phase 5: Review Similar Patterns (Medium Priority)

#### 5.1 Audit Other Nested Resource Extractions
**Files to Review**:
- `internal/declarative/loader/loader.go` - Check all extraction patterns:
  - API Publications extraction (lines ~950-960)
  - Auth Strategy associations
  - Any other nested resource extractions

**Action**: Ensure all nested resource extractions preserve original fields correctly

#### 5.2 Create Helper Function
**File**: `internal/declarative/loader/loader.go`
**New Function**:
```go
// ensureFieldPreservation performs a defensive copy ensuring critical fields are preserved
func ensureFieldPreservation[T any](original, extracted *T, preserveFields ...string) {
    // Implementation to use reflection to ensure specified fields are preserved
    // This prevents similar bugs in the future
}
```

### Phase 6: Documentation Updates

#### 6.1 Code Documentation
**Files**: Add comments to extraction code explaining the importance of field preservation

#### 6.2 Debugging Guide
**File**: `docs/debugging/declarative-config-errors.md` (create if doesn't exist)
**Content**:
- Common configuration errors and their solutions
- How to enable trace logging for debugging
- Understanding validation error messages

## Testing Strategy

### Before Fix Verification
1. Run `k sync -f docs/examples/declarative/namespace/single-team -R`
2. Confirm error: `invalid portal_custom_domain ''`

### After Fix Verification
1. Run the same command
2. Verify successful sync
3. Check that portal custom domain is created correctly in Konnect
4. Run all new tests: `make test`
5. Run integration tests: `make test-integration`

### Edge Cases to Test
1. Portal with custom_domain but no ref specified
2. Portal with custom_domain ref containing special characters
3. Multiple portals with custom domains in same file
4. Custom domain with very long ref names

## Rollback Plan

If the fix causes unexpected issues:
1. The changes are minimal and can be reverted easily
2. The fix only affects the extraction logic, not the data model
3. Existing configurations without custom domains are unaffected

## Future Improvements

1. **Validation Timing**: Consider validating nested resources before extraction to catch issues earlier with better context
2. **Generic Extraction**: Implement a generic nested resource extraction pattern that ensures field preservation
3. **Schema Validation**: Add JSON schema validation for configuration files to catch issues before parsing
4. **Better Error Recovery**: Allow partial configuration loading with warnings for invalid resources

## Success Criteria

1. The example configuration in `docs/examples/declarative/namespace/single-team` loads successfully
2. Portal custom domains are created with correct references
3. Error messages clearly indicate the source of validation failures
4. All existing tests continue to pass
5. New tests cover the fixed functionality