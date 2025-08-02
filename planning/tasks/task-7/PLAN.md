# Fix Runtime Errors Plan

## Overview

This plan addresses critical runtime validation errors that occur when running `kongctl apply` with the example YAML files. The errors prevent successful resource creation in Konnect and must be fixed immediately to restore functionality.

## Priority Order

1. **P0 - Portal Custom Domain SSL Error** (Blocks portal creation)
2. **P0 - Auth Strategy Key Naming Errors** (Blocks auth strategy creation)
3. **P1 - API Publication UUID Resolution** (Blocks API publication)
4. **P1 - API Version Content Validation** (Blocks API version creation)
5. **P2 - Add Validation and Testing** (Prevents regression)

## Detailed Implementation Steps

### Step 1: Fix Portal Custom Domain SSL Nil Pointer Issue

**Problem**: Attempting to access `domain.Ssl.DomainVerificationMethod` without nil check causes creation of empty SSL object that fails API validation.

**Files to Modify**:
- `/internal/declarative/planner/portal_child_planner.go`

**Code Changes**:
```go
// Lines 396-401 - Add nil check before accessing SSL fields
if domain.Ssl != nil && domain.Ssl.DomainVerificationMethod != "" {
    sslFields := make(map[string]interface{})
    sslFields["domain_verification_method"] = string(domain.Ssl.DomainVerificationMethod)
    fields["ssl"] = sslFields
}
```

**Why This Fixes It**:
- Prevents creating empty SSL object when no SSL configuration exists
- Konnect API expects either no SSL field or a fully populated SSL configuration
- The oneOf schema validation fails with empty SSL objects

**Testing**:
```bash
# Verify portal with custom domain can be created without SSL
./kongctl apply -f docs/examples/declarative/namespace/single-team/portal.yaml --pat $(cat ~/.konnect/claude.pat)
```

### Step 2: Fix Auth Strategy Key Naming Convention

**Problem**: Using hyphens ("key-auth", "openid-connect") instead of underscores ("key_auth", "openid_connect") in config keys.

**Files to Modify**:
- `/internal/declarative/planner/auth_strategy_planner.go`

**Code Changes**:

For Key-Auth (Line 188):
```go
// Change from "key-auth" to "key_auth"
fields["configs"] = map[string]interface{}{
    "key_auth": map[string]interface{}{
        "key_names": strategy.AppAuthStrategyKeyAuthRequest.Configs.KeyAuth.KeyNames,
    },
}
```

For OAuth2/OpenID Connect (Line 219):
```go
// Change from "openid-connect" to "openid_connect"
fields["configs"] = map[string]interface{}{
    "openid_connect": oidcConfig,
}
```

**Why This Fixes It**:
- Konnect API expects underscore-separated keys in the configs map
- The API validation looks for specific keys like "key_auth" and "openid_connect"
- Using hyphens causes the API to not find the required configuration

**Testing**:
```bash
# Test key-auth strategy creation
./kongctl apply -f docs/examples/declarative/namespace/single-team/auth_strategies.yaml --pat $(cat ~/.konnect/claude.pat)
```

### Step 3: Fix API Publication Reference Resolution

**Problem**: auth_strategy_ids contains reference names instead of UUIDs.

**Investigation Needed**:
1. Check if reference resolution is being called for auth_strategy_ids
2. Verify the reference mapping is working correctly

**Files to Check**:
- `/internal/declarative/planner/api_publication_planner.go`
- `/internal/declarative/planner/reference_resolver.go`

**Potential Fix**:
Ensure auth_strategy_ids references are resolved during planning phase. The resource already has correct reference field mappings:

```go
// In api_publication.go GetReferenceFieldMappings()
"auth_strategy_ids": {
    ResourceType: DeclarativeAuthStrategiesResourceType,
    SourceField:  "auth_strategy_ids",
    TargetField:  "id",
}
```

**Testing**:
```bash
# Verify auth strategies are created first, then API publication
./kongctl apply -f docs/examples/declarative/namespace/single-team -R --pat $(cat ~/.konnect/claude.pat)
```

### Step 4: Fix API Version Content Validation

**Problem**: API rejects content as invalid specification.

**Investigation Needed**:
1. Check if YAML content is properly converted to JSON
2. Verify the content structure matches API expectations

**Files to Check**:
- `/internal/declarative/resources/api_version.go` (UnmarshalJSON method)
- `/internal/declarative/planner/api_version_planner.go`
- Example YAML files using "document:" instead of "content:"

**Potential Fixes**:
1. Ensure YAML specs are converted to proper JSON format
2. Update example files to use "content:" if needed
3. Add validation for OpenAPI/AsyncAPI spec formats

**Testing**:
```bash
# Test API version creation with various spec formats
./kongctl apply -f docs/examples/declarative/namespace/single-team/api.yaml --pat $(cat ~/.konnect/claude.pat)
```

### Step 5: Add Comprehensive Testing

**Unit Tests to Add**:

1. **Portal Custom Domain Nil Check Test**:
   - File: `/internal/declarative/planner/portal_child_planner_test.go`
   - Test nil SSL field doesn't create empty object
   - Test populated SSL field is correctly mapped

2. **Auth Strategy Key Naming Test**:
   - File: `/internal/declarative/planner/auth_strategy_planner_test.go`
   - Test key_auth uses underscore in config key
   - Test openid_connect uses underscore in config key

3. **Reference Resolution Test**:
   - Verify auth_strategy_ids references are resolved to UUIDs

**Integration Tests**:
- Add integration test that runs full apply with example files
- Verify all resources are created successfully

### Step 6: Add Early Validation

**Goal**: Catch errors during planning phase instead of API call phase.

**Implementation**:
1. Add validation function in each planner to check:
   - Required fields are present
   - Field formats match API expectations (e.g., UUIDs)
   - Key naming conventions are correct

2. Create common validation utilities:
   - UUID format validator
   - Reference resolution checker
   - Key naming convention validator

**Files to Create/Modify**:
- `/internal/declarative/planner/validation.go` (new file)
- Update each planner to call validation before returning plan

### Step 7: Pattern Consistency Review

**Check for Similar Issues**:

1. **Nil Pointer Checks**:
   - Search for other embedded SDK structs
   - Add nil checks where needed
   - Pattern: `if resource.Field != nil`

2. **Key Naming Convention**:
   - Search for map keys with hyphens
   - Ensure all use underscores for API compatibility
   - Pattern: Use `key_name` not `key-name`

3. **Reference Resolution**:
   - Verify all reference fields are properly resolved
   - Check GetReferenceFieldMappings implementations

**Files to Review**:
```bash
# Find potential nil pointer issues
grep -r "\..*\." --include="*.go" internal/declarative/planner/

# Find hyphenated keys
grep -r '".*-.*":' --include="*.go" internal/declarative/

# Check reference mappings
grep -r "GetReferenceFieldMappings" --include="*.go" internal/declarative/resources/
```

## Implementation Order

1. **Immediate Fixes (Do First)**:
   - Step 1: Portal Custom Domain nil check
   - Step 2: Auth Strategy key naming
   - These are simple, low-risk fixes that unblock functionality

2. **Investigation and Fixes (Do Second)**:
   - Step 3: API Publication reference resolution
   - Step 4: API Version content validation
   - May require more investigation but critical for full functionality

3. **Quality Improvements (Do Third)**:
   - Step 5: Add comprehensive testing
   - Step 6: Add early validation
   - Step 7: Pattern consistency review
   - Prevents regression and improves reliability

## Risk Assessment

**Low Risk**:
- Portal Custom Domain nil check - Simple defensive programming
- Auth Strategy key naming - String constant changes only

**Medium Risk**:
- Reference resolution changes - May affect other resources
- API Version content handling - Complex transformation logic

**Mitigation**:
- Test each fix individually before combining
- Run full integration test suite after each change
- Keep changes minimal and focused

## Success Criteria

1. All example YAML files apply successfully without errors
2. Unit tests pass for all fixed issues
3. Integration tests verify end-to-end functionality
4. No regression in existing functionality
5. Early validation prevents runtime errors

## Decision: Portal Custom Domain Change

**Keep the embedding approach** rather than reverting because:
1. Aligns with pattern used in other resources
2. Provides better type safety from SDK
3. Fix is simple (add nil check)
4. Maintains consistency across codebase

## Testing Commands

After implementing fixes, run these commands in order:

```bash
# Build and verify compilation
make build

# Run linter to check code quality
make lint

# Run unit tests
make test

# Run integration tests
make test-integration

# Test with example files
./kongctl apply -f docs/examples/declarative/namespace/single-team -R --pat $(cat ~/.konnect/claude.pat)
```

## Notes

- All fixes maintain backward compatibility
- No changes to public APIs or CLI interface
- Focus on minimal changes to restore functionality
- Comprehensive testing prevents future regression