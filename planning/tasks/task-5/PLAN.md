# Plan: Fix auth_strategy_ids Optional Field Handling

## Executive Summary

The `auth_strategy_ids` field in API publications is incorrectly enforced as required in 
the kongctl implementation, contradicting the Konnect API specification which clearly 
states it's optional. This prevents users from:
1. Omitting the field to use the portal's default auth strategy
2. Setting it to null to indicate no authentication required
3. Using empty arrays (if that's a valid use case)

## Root Cause

The issue stems from two enforcement points in `api_publication_adapter.go`:
1. The field is listed in `RequiredFields()` at line 118
2. The `Create()` method validates that the array is not empty at lines 75-77

These checks contradict the SDK definition where the field has `omitempty` tag and 
comments explicitly state omitting is valid.

## Implementation Plan

### Step 1: Remove Required Field Enforcement

**File**: `internal/declarative/executor/api_publication_adapter.go`

**Change 1**: Update `RequiredFields()` method (line 118)
```go
// Current:
func (a *APIPublicationAdapter) RequiredFields() []string {
    return []string{"portal_id", "auth_strategy_ids"}
}

// Change to:
func (a *APIPublicationAdapter) RequiredFields() []string {
    return []string{"portal_id"}
}
```

**Change 2**: Remove validation in `Create()` method (lines 75-77)
```go
// Remove these lines:
if len(req.AuthStrategyIds) == 0 {
    return "", fmt.Errorf("auth_strategy_ids is required for API publication")
}
```

### Step 2: Verify MapCreateFields Handles All Cases

**File**: `internal/declarative/executor/api_publication_adapter.go`

Verify the `MapCreateFields` method (lines 36-50) properly handles:
- Field not present in fields map (omitted case)
- Empty array value
- String value (comma-separated)
- String array value
- Reference resolution

Current implementation should work correctly once validation is removed.

### Step 3: Add Test Coverage

**File**: `internal/declarative/executor/api_publication_adapter_test.go` (create if doesn't exist)

Add unit tests for:
```go
func TestAPIPublicationAdapter_OptionalAuthStrategyIds(t *testing.T) {
    adapter := &APIPublicationAdapter{}
    
    t.Run("omitted auth_strategy_ids", func(t *testing.T) {
        fields := map[string]interface{}{
            "portal_id": "test-portal",
            // auth_strategy_ids intentionally omitted
        }
        change := &common.Change{Fields: fields}
        
        req, err := adapter.MapCreateFields(change)
        assert.NoError(t, err)
        assert.Nil(t, req.AuthStrategyIds)
    })
    
    t.Run("empty array auth_strategy_ids", func(t *testing.T) {
        fields := map[string]interface{}{
            "portal_id": "test-portal",
            "auth_strategy_ids": []string{},
        }
        change := &common.Change{Fields: fields}
        
        req, err := adapter.MapCreateFields(change)
        assert.NoError(t, err)
        assert.Empty(t, req.AuthStrategyIds)
    })
    
    t.Run("populated auth_strategy_ids", func(t *testing.T) {
        fields := map[string]interface{}{
            "portal_id": "test-portal",
            "auth_strategy_ids": []string{"strategy1", "strategy2"},
        }
        change := &common.Change{Fields: fields}
        
        req, err := adapter.MapCreateFields(change)
        assert.NoError(t, err)
        assert.Equal(t, []string{"strategy1", "strategy2"}, req.AuthStrategyIds)
    })
}
```

### Step 4: Add Integration Test

**File**: `test/integration/api_publication_test.go` (create or update)

Add integration test that verifies the full flow:
```go
func TestAPIPublication_OptionalAuthStrategy(t *testing.T) {
    t.Run("create API publication without auth_strategy_ids", func(t *testing.T) {
        config := `
api_publications:
  - ref: test-pub
    api: test-api
    portal_id: test-portal
    visibility: public
`
        // Test that this configuration successfully creates an API publication
        // without auth_strategy_ids, using the portal's default
    })
}
```

### Step 5: Update Example Configuration

**File**: `internal/declarative/loader/testdata/valid/api-publications-examples.yaml` (create)

Add examples showing all valid configurations:
```yaml
# Example 1: Omit auth_strategy_ids to use portal default
api_publications:
  - ref: uses-portal-default
    api: my-api
    portal_id: my-portal
    visibility: public
    # auth_strategy_ids omitted - will use portal's default auth strategy

# Example 2: Explicitly set auth strategies
api_publications:
  - ref: custom-auth
    api: my-api
    portal_id: my-portal
    visibility: public
    auth_strategy_ids: 
      - oauth2-strategy
      - api-key-strategy

# Example 3: No authentication required (if supported by API)
# Note: Current implementation doesn't distinguish null from omitted
# This would require changes to use pointers or custom types
```

### Step 6: Verify Existing Tests

Run all tests to ensure no regressions:
```bash
make test
make test-integration
```

Fix any tests that were expecting the old behavior.

## Testing Scenarios

### Unit Tests
1. ✅ Omitted `auth_strategy_ids` - should not error
2. ✅ Empty array `[]` - should not error  
3. ✅ Populated array - should work as before
4. ✅ String value (comma-separated) - should parse correctly
5. ✅ Reference resolution - should resolve correctly

### Integration Tests
1. ✅ Full flow with omitted field - API publication created using portal default
2. ✅ Full flow with populated field - API publication created with specified strategies
3. ✅ Plan/apply workflow - ensure plan correctly shows changes

### Manual Testing
1. Create YAML config without `auth_strategy_ids`
2. Run `kongctl plan -f config.yaml --pat $PAT`
3. Verify plan shows API publication creation
4. Run `kongctl apply --plan plan.json --pat $PAT`
5. Verify API publication created successfully
6. Check in Konnect UI that portal's default auth strategy is being used

## Documentation Updates

### 1. Code Comments

Update any comments suggesting `auth_strategy_ids` is required:
- Remove or update validation error messages
- Add comment in adapter explaining optional nature

### 2. YAML Schema Documentation

If there's schema documentation, ensure `auth_strategy_ids` is marked as optional with 
clear explanation of behavior when omitted.

## Verification Steps

1. **Build**: `make build` - Ensure code compiles
2. **Lint**: `make lint` - No linting issues  
3. **Unit Tests**: `make test` - All tests pass
4. **Integration Tests**: `make test-integration` - Full flow works
5. **Manual Test**: Create API publication without auth_strategy_ids

## Future Considerations

### Null vs Omitted Distinction

The current implementation doesn't distinguish between:
- Field omitted in YAML (should use portal default)
- Field explicitly set to null (should mean no auth required)

To support this distinction, consider:
1. Using pointer types: `*[]string` instead of `[]string`
2. Custom unmarshaling to preserve null vs omitted
3. Update planner to handle null values differently

This would be a larger change and should be considered separately if the use case 
requires it.

### Broader Pattern Review

The pattern of checking `len(slice) == 0` for optional array fields should be reviewed 
across other adapters:
- Search for similar validation patterns
- Ensure consistency with API specifications
- Consider creating guidelines for optional field handling

## Success Criteria

1. ✅ Users can create API publications without specifying `auth_strategy_ids`
2. ✅ Existing functionality with populated `auth_strategy_ids` continues to work
3. ✅ All tests pass
4. ✅ No linting issues
5. ✅ Clear examples demonstrate the optional nature of the field