# Flow Report: auth_strategy_ids Handling in API Publication Operations

## Executive Summary

The `auth_strategy_ids` field follows a complex flow from YAML configuration through multiple layers before reaching the Konnect API. The issue stems from a validation check in the executor layer that enforces the field as required, contradicting both the API specification and SDK definition which mark it as optional.

## Flow Diagram

```
YAML Config
    ↓
Resource Parsing (api_publication.go)
    ↓
Planner (api_planner.go)
    ↓
Executor (base_executor.go)
    ↓
Adapter (api_publication_adapter.go)
    ↓
SDK Model (apipublication.go)
    ↓
Konnect API
```

## Detailed Flow Analysis

### 1. YAML Configuration Input

**File**: `api-publications-separate.yaml`
```yaml
api_publications:
  - ref: users-api-public-pub
    api: users-api
    portal_id: test-portal
    visibility: public
    auto_approve_registrations: true
    # Note: auth_strategy_ids is completely omitted
```

**Key Point**: Users can omit `auth_strategy_ids` entirely, expecting the portal's default auth strategy to be used.

### 2. Resource Definition & Parsing

**File**: `internal/declarative/resources/api_publication.go`

The resource struct embeds the SDK model:
```go
type APIPublicationResource struct {
    kkComps.APIPublication `yaml:",inline" json:",inline"`
    // ... other fields
}
```

**UnmarshalJSON** (lines 130-172):
- Line 137: `AuthStrategyIDs []string json:"auth_strategy_ids,omitempty"`
- Line 162: Maps to SDK field: `p.AuthStrategyIds = temp.AuthStrategyIDs`
- If omitted in YAML, the field remains nil

**Validate()** method (lines 59-68):
- Only enforces `ref` and `portal_id` as required
- No validation for `auth_strategy_ids`

### 3. Planning Phase

**File**: `internal/declarative/planner/api_planner.go`

**planAPIPublicationCreate** (lines 752-831):
```go
fields := make(map[string]interface{})
fields["portal_id"] = publication.PortalID
if publication.AuthStrategyIds != nil {  // Line 757
    fields["auth_strategy_ids"] = publication.AuthStrategyIds
}
```

**Critical Behavior**:
- `auth_strategy_ids` is only added to the fields map if NOT nil
- Omitted fields → not in fields map
- Empty array `[]` → present in fields map
- This distinction becomes important during validation

### 4. Execution Phase

**File**: `internal/declarative/executor/base_executor.go`

**Create** method (line 78):
```go
if err := common.ValidateRequiredFields(change.Fields, b.ops.RequiredFields()); err != nil {
    return "", common.WrapWithResourceContext(err, b.ops.ResourceType(), "")
}
```

This calls the adapter's `RequiredFields()` method before any field mapping occurs.

### 5. Validation Logic

**File**: `internal/declarative/common/fields.go`

**ValidateRequiredFields** (lines 98-111):
```go
for _, field := range requiredFields {
    value, exists := fields[field]
    if !exists {
        return fmt.Errorf("required field '%s' is missing", field)
    }
    // Check for empty string values
    if strValue, ok := value.(string); ok && strValue == "" {
        return fmt.Errorf("required field '%s' cannot be empty", field)
    }
}
```

**Validation Behavior**:
- Checks if field exists in the map
- For strings, also checks if not empty
- Does NOT distinguish between omitted, null, or empty array

### 6. Adapter Processing

**File**: `internal/declarative/executor/api_publication_adapter.go`

**RequiredFields** (line 118):
```go
return []string{"portal_id", "auth_strategy_ids"}
```

**MapCreateFields** (lines 35-50):
- Handles three cases for `auth_strategy_ids`:
  1. Reference resolution
  2. String value (comma-separated)
  3. String array value
- But only processes if field exists in the map

**Create** method (lines 75-77):
```go
if len(req.AuthStrategyIds) == 0 {
    return "", fmt.Errorf("auth_strategy_ids is required for API publication")
}
```

**Double Enforcement**: The field is checked twice:
1. In `ValidateRequiredFields` (field must exist)
2. In `Create` method (array must not be empty)

### 7. SDK Model

**File**: `Kong/sdk-konnect-go/models/components/apipublication.go`

```go
// The auth strategy the API enforces for applications in the portal.
// Omitting this property means the portal's default application auth strategy will be used.
// Setting to null means the API will not require application authentication.
AuthStrategyIds []string `json:"auth_strategy_ids,omitempty"`
```

**SDK Behavior**:
- `omitempty` tag: field omitted from JSON when empty/nil
- Comments explicitly state omitting is valid
- Supports three states: omitted, null, populated

### 8. API Call

The final SDK struct is serialized to JSON and sent to Konnect API, which accepts:
- Omitted field → use portal default
- Null → no authentication required
- Array of IDs → use specified strategies

## Comparison with Other Optional Fields

### API Version Adapter Pattern

**File**: `internal/declarative/executor/api_version_adapter.go`

```go
func (a *APIVersionAdapter) RequiredFields() []string {
    return []string{} // No required fields (all are pointers)
}
```

This adapter correctly treats all fields as optional, matching the SDK model where all fields are pointers.

### Portal Adapter Pattern

Most adapters only enforce truly required fields like `name` or `slug`, not optional array fields.

## Issue Root Cause

The issue occurs because:

1. **Planner**: Only adds `auth_strategy_ids` to fields map if not nil
2. **Validator**: Expects all "required" fields to exist in the map
3. **Adapter**: Lists `auth_strategy_ids` as required despite SDK marking it optional
4. **Create Method**: Additionally checks for empty array

When a user omits the field:
- Planner doesn't add it to fields map
- Validator fails because field doesn't exist
- User can't rely on portal's default auth strategy

## Data Flow Summary

| Stage | Omitted Field | Empty Array `[]` | Populated Array |
|-------|---------------|------------------|-----------------|
| YAML | Not present | `auth_strategy_ids: []` | `auth_strategy_ids: [id1]` |
| Resource | `AuthStrategyIds = nil` | `AuthStrategyIds = []` | `AuthStrategyIds = [id1]` |
| Planner Fields | Not in map | In map as `[]` | In map as `[id1]` |
| Validation | ❌ Fails (missing) | ❌ Fails (empty check) | ✅ Passes |
| Adapter | Never reached | Processed | Processed |
| SDK JSON | Field omitted | `"auth_strategy_ids": []` | `"auth_strategy_ids": ["id1"]` |

## Conclusion

The enforcement of `auth_strategy_ids` as required happens at two points that should be removed:
1. In `RequiredFields()` declaration
2. In the `Create()` method's empty array check

This would align the implementation with the API specification and allow users to omit the field to use portal defaults.