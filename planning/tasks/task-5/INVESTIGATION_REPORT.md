# Investigation Report: auth_strategy_ids Enforcement Issue

## Executive Summary

The `auth_strategy_ids` field is being incorrectly enforced as required in the API publication adapter when the API specification clearly states it's optional. This mismatch prevents users from creating API publications that rely on the portal's default authentication strategy or that don't require authentication.

## Key Findings

### 1. API Specification vs Implementation Mismatch

**API Specification** (from SDK comments):
```go
// The auth strategy the API enforces for applications in the portal.
// Omitting this property means the portal's default application auth strategy will be used.
// Setting to null means the API will not require application authentication.
```

**Current Implementation**:
- `api_publication_adapter.go` line 75-77: Enforces `auth_strategy_ids` as required
- `api_publication_adapter.go` line 118: Lists `auth_strategy_ids` in `RequiredFields()`

### 2. SDK Definition

The SDK correctly defines the field as optional:
```go
// From /Kong/sdk-konnect-go/models/components/apipublication.go
AuthStrategyIds []string `json:"auth_strategy_ids,omitempty"`
```

The `omitempty` tag indicates this field should be omitted from JSON when empty.

### 3. Declarative Config Examples

Several valid configuration examples omit `auth_strategy_ids`:
- `/internal/declarative/loader/testdata/valid/api-publications-separate.yaml` - completely omits the field
- Examples show the field is meant to be optional

### 4. Field Handling in Adapter

The adapter's `MapCreateFields` method (lines 36-50) correctly handles three cases:
1. Reference resolution through `change.References`
2. String value (comma-separated IDs)
3. String array value

However, it doesn't handle the case where the field is intentionally omitted.

### 5. Comparison with Other Adapters

Other adapters handle optional fields differently:
- `api_version_adapter.go`: Returns empty `RequiredFields()` with comment "all are pointers"
- Most adapters only enforce truly required fields like `name`, `slug`, or `content`

## Root Cause Analysis

The issue stems from two locations:

1. **Validation Check** (line 75-77):
   ```go
   if len(req.AuthStrategyIds) == 0 {
       return "", fmt.Errorf("auth_strategy_ids is required for API publication")
   }
   ```

2. **Required Fields Declaration** (line 118):
   ```go
   return []string{"portal_id", "auth_strategy_ids"}
   ```

## Impact

This enforcement prevents three valid use cases:
1. **Omitting the field** - Should use portal's default auth strategy
2. **Setting to null** - Should mean no authentication required
3. **Setting to empty array** - Unclear semantics, but shouldn't fail

## Recommendations

1. **Remove the validation check** at lines 75-77
2. **Remove `auth_strategy_ids` from RequiredFields()** - only keep `portal_id`
3. **Consider explicit null handling** - The current implementation doesn't distinguish between omitted and null

## Similar Issues to Check

The pattern of checking `len(slice) == 0` for optional array fields should be reviewed across other adapters to ensure consistency with API specifications.

## Code References

- **Adapter**: `/internal/declarative/executor/api_publication_adapter.go`
- **SDK Model**: `/Kong/sdk-konnect-go/models/components/apipublication.go`
- **Resource**: `/internal/declarative/resources/api_publication.go`
- **Example Config**: `/internal/declarative/loader/testdata/valid/api-publications-separate.yaml`

## Conclusion

The current implementation incorrectly treats `auth_strategy_ids` as a required field, contradicting the API specification and SDK definition. The fix is straightforward: remove the validation check and the field from the required fields list.