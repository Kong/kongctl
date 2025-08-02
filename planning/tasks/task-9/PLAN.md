# Plan: API Version Limitation Fix

## Problem Summary

Kongctl's declarative configuration allows users to define multiple API versions (using a slice in the `APIResource` struct), but Konnect's API enforces a constraint that each API can have only one version. This mismatch is not validated during the loading or planning phases, resulting in a runtime error during the apply phase with the message: "At most one api specification can be provided for a api".

This creates a poor user experience where users invest time creating complex configurations with multiple API versions, only to have them fail during execution. The error occurs late in the process, potentially after partial application of changes.

## Proposed Solution Approach

The solution maintains backward compatibility while adding early validation to catch the constraint violation before execution. We will:

1. Keep the `Versions` field as a slice (avoiding breaking changes)
2. Add validation at multiple layers to enforce the single-version constraint
3. Provide clear, actionable error messages explaining the Konnect limitation
4. Design the solution to easily support future multi-version capabilities
5. Handle edge cases gracefully with helpful guidance

## Implementation Steps

### Phase 1: Immediate Validation (Priority: High)

#### 1.1 Add Validation to Loader (First Line of Defense)
**File**: `internal/declarative/loader/validator.go`
**Function**: `validateAPIs()`

Add validation after existing ref uniqueness checks:
```go
// Validate Konnect's single-version constraint
if len(api.Versions) > 1 {
    return fmt.Errorf("api %q defines %d versions, but Konnect currently supports only one version per API. Please define only one version or split into separate APIs", api.GetRef(), len(api.Versions))
}
```

#### 1.2 Add Validation to Planner (Safety Net)
**File**: `internal/declarative/planner/api_planner.go`
**Function**: `planAPIVersionChanges()`

Before planning version creates, check existing versions:
```go
// Check if API already has a version (Konnect constraint)
if len(current) > 0 && len(desired) > 0 {
    return fmt.Errorf("cannot add version to api %q: Konnect APIs support only one version. Current version: %s", apiRef, getCurrentVersionString(current))
}
```

#### 1.3 Update Error Messages
**File**: `internal/declarative/executor/api_version_adapter.go`
**Function**: `Create()`

Wrap Konnect API errors with more context:
```go
if err != nil {
    if strings.Contains(err.Error(), "At most one api specification") {
        return nil, fmt.Errorf("failed to create API version: Konnect allows only one version per API. Consider updating the existing version or creating a separate API. Original error: %w", err)
    }
    return nil, fmt.Errorf("failed to create API version: %w", err)
}
```

### Phase 2: Enhanced Validation and UX (Priority: Medium)

#### 2.1 Add Configuration-Time Warnings
**File**: `internal/declarative/loader/loader.go`
**Function**: `LoadFromSources()`

Add non-fatal warnings for future compatibility:
```go
// Warn if using single-element version arrays
if len(api.Versions) == 1 {
    log.Debug("API %q uses array syntax for single version. This is valid but consider using inline version for clarity", api.GetRef())
}
```

#### 2.2 Enhance Plan Output
**File**: `internal/declarative/planner/planner.go`
**Function**: `GeneratePlan()`

Add plan validation summary:
```go
// Add validation summary to plan metadata
if validationWarnings := validatePlanConstraints(plan); len(validationWarnings) > 0 {
    plan.Metadata["warnings"] = validationWarnings
}
```

#### 2.3 Add Dry-Run Validation
**File**: `internal/cmd/root/products/konnect/declarative/declarative.go`
**Function**: `runApply()`

Add pre-execution validation option:
```go
if dryRun {
    if err := validatePlanAgainstKonnect(ctx, plan); err != nil {
        return fmt.Errorf("plan validation failed: %w", err)
    }
}
```

### Phase 3: Future-Proofing (Priority: Low)

#### 3.1 Add Feature Flag for Multi-Version Support
**File**: `internal/declarative/config/features.go` (new file)

```go
package config

type Features struct {
    AllowMultipleAPIVersions bool `yaml:"allow_multiple_api_versions"`
}

func (f *Features) IsMultiVersionEnabled() bool {
    return f.AllowMultipleAPIVersions
}
```

#### 3.2 Conditional Validation Based on Feature Flag
**File**: `internal/declarative/loader/validator.go`

```go
if !features.IsMultiVersionEnabled() && len(api.Versions) > 1 {
    // Apply single-version constraint
}
```

## Edge Cases and Considerations

### 1. Version Identifier Changes
**Scenario**: User wants to change version from "v1" to "v1.0"
**Solution**: 
- Document that version identifiers are immutable in Konnect
- Provide clear error message suggesting delete and recreate
- Consider adding a `--force-recreate-versions` flag for apply command

### 2. Existing API with Version
**Scenario**: API already has a version, user tries to add another
**Solution**:
- Planner detects existing version and provides specific error
- Suggest using update instead of create
- Provide example of correct configuration

### 3. Migration from Multi-Version Config
**Scenario**: User has existing config with multiple versions
**Solution**:
- Validation provides clear migration path
- Suggest splitting into separate APIs with naming convention
- Provide migration script/example in documentation

### 4. Partial Apply State
**Scenario**: Apply fails after creating API but before version
**Solution**:
- Detect APIs without versions in current state
- Provide helpful message about completing the configuration
- Handle gracefully in subsequent applies

### 5. Cross-Reference Validation
**Scenario**: Other resources reference specific API versions
**Solution**:
- Ensure validation considers dependent resources
- Provide clear error messages about broken references
- Suggest valid alternatives

## Testing Strategy

### Unit Tests

1. **Validator Tests** (`internal/declarative/loader/validator_test.go`)
   - Test single version: should pass
   - Test multiple versions: should fail with specific error
   - Test empty versions: should pass
   - Test with feature flag enabled: should pass multiple versions

2. **Planner Tests** (`internal/declarative/planner/api_planner_test.go`)
   - Test planning with existing version: should fail
   - Test planning version update: should handle gracefully
   - Test version deletion and recreation scenarios

3. **Error Message Tests**
   - Verify enhanced error messages are returned
   - Test error wrapping preserves original error
   - Verify context is helpful and actionable

### Integration Tests

1. **End-to-End Validation** (`test/integration/declarative/api_version_validation_test.go`)
   ```go
   func TestAPIVersionConstraintValidation(t *testing.T) {
       // Test early validation failure
       // Test clear error messages
       // Test successful single-version config
   }
   ```

2. **Migration Scenarios** (`test/integration/declarative/api_version_migration_test.go`)
   - Test config with multiple versions fails appropriately
   - Test migration path suggestions
   - Test version update workflows

3. **Edge Case Tests**
   - Test partial apply recovery
   - Test version identifier changes
   - Test cross-reference validation

### Documentation Updates

1. **User Guide** (`docs/declarative-configuration.md`)
   - Add section on API version limitations
   - Provide examples of correct single-version configuration
   - Document workarounds for multiple version needs

2. **Migration Guide** (`docs/migration/api-versions.md`)
   - Step-by-step guide for splitting multi-version APIs
   - Examples of before/after configurations
   - Common patterns and best practices

3. **Error Reference** (`docs/errors/api-version-errors.md`)
   - Document all new error messages
   - Provide solutions for each error
   - Link to relevant documentation

## Future-Proofing Considerations

### 1. API Evolution
- Design validation to be easily disabled when Konnect supports multiple versions
- Use feature flags to control behavior
- Ensure error messages indicate this is a current limitation, not a design choice

### 2. Configuration Migration
- When multi-version support is added, provide automatic migration
- Maintain backward compatibility with single-version configs
- Design configuration format to naturally extend to multiple versions

### 3. Version Management Features
- Plan for version promotion/demotion workflows
- Consider version aliasing (latest, stable, beta)
- Design for version-specific configurations

### 4. Monitoring and Metrics
- Add metrics for validation failures
- Track usage patterns to inform future design
- Monitor for attempts to use multiple versions

## Implementation Priority and Timeline

### Week 1: Phase 1 Implementation
- Day 1-2: Implement loader validation
- Day 3-4: Add planner validation and error enhancement
- Day 5: Write unit tests and documentation

### Week 2: Phase 2 Enhancement
- Day 1-2: Add configuration warnings and plan validation
- Day 3-4: Implement dry-run validation
- Day 5: Integration testing and documentation

### Week 3: Phase 3 Future-Proofing
- Day 1-2: Design and implement feature flag system
- Day 3-4: Add conditional validation
- Day 5: Final testing and documentation review

## Success Criteria

1. **Early Failure**: Validation catches multiple versions during load/plan phase
2. **Clear Messaging**: Error messages explain the limitation and provide solutions
3. **No Breaking Changes**: Existing valid configurations continue to work
4. **Future Ready**: Solution can be easily adapted when Konnect adds multi-version support
5. **Comprehensive Testing**: All scenarios covered with appropriate tests
6. **User Documentation**: Clear guidance on limitations and workarounds

## Risk Mitigation

1. **Breaking Changes**: Maintain slice type, add validation only
2. **Performance Impact**: Validation is lightweight, O(n) complexity
3. **User Confusion**: Clear documentation and error messages
4. **Future Compatibility**: Feature flag system allows smooth transition
5. **Edge Cases**: Comprehensive testing and validation at multiple layers