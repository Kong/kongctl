# Investigation Report: Runtime Errors After PortalCustomDomainResource Change

## Executive Summary

This report documents the investigation of runtime errors that occurred after changing `PortalCustomDomainResource` to embed `CreatePortalCustomDomainRequest` from the Konnect SDK. Five distinct validation errors were identified, each with specific root causes in the codebase.

## Errors Investigated

1. **portal_custom_domain**: `"field":"ssl","reason":"must match exactly one schema in oneOf"`
2. **application_auth_strategy key-auth**: `"configs is required for key_auth strategy"`
3. **application_auth_strategy oauth2**: `"auth_methods":"must be array"`
4. **api_version**: `"content must be valid specification"`
5. **api_publication**: `"auth_strategy_ids.0":"must match format \"uuid\""`

## Detailed Analysis

### 1. Portal Custom Domain SSL Validation Error

**File**: `/internal/declarative/planner/portal_child_planner.go`
**Lines**: 396-401

**Root Cause**: The code attempts to access `domain.Ssl.DomainVerificationMethod` without checking if `domain.Ssl` is nil first. Since `PortalCustomDomainResource` now embeds `CreatePortalCustomDomainRequest`, the `Ssl` field is of type `CreatePortalCustomDomainSSL` (likely a pointer), which is nil by default.

**Problematic Code**:
```go
// Line 397: Direct access without nil check
if domain.Ssl.DomainVerificationMethod != "" {
    sslFields := make(map[string]interface{})
    sslFields["domain_verification_method"] = string(domain.Ssl.DomainVerificationMethod)
    fields["ssl"] = sslFields
}
```

**Fix Required**: Add nil check before accessing `Ssl` fields:
```go
if domain.Ssl != nil && domain.Ssl.DomainVerificationMethod != "" {
    // ... rest of the code
}
```

### 2. Application Auth Strategy Key-Auth Configs Error

**File**: `/internal/declarative/planner/auth_strategy_planner.go`
**Lines**: 187-192

**Root Cause**: The planner uses incorrect key name `"key-auth"` (with hyphen) instead of `"key_auth"` (with underscore) when setting the configs map. The Konnect API expects `"key_auth"` as the key.

**Problematic Code**:
```go
// Line 188: Wrong key name with hyphen
fields["configs"] = map[string]interface{}{
    "key-auth": map[string]interface{}{
        "key_names": strategy.AppAuthStrategyKeyAuthRequest.Configs.KeyAuth.KeyNames,
    },
}
```

**Fix Required**: Change to use underscore:
```go
fields["configs"] = map[string]interface{}{
    "key_auth": map[string]interface{}{
        "key_names": strategy.AppAuthStrategyKeyAuthRequest.Configs.KeyAuth.KeyNames,
    },
}
```

### 3. Application Auth Strategy OAuth2 Auth Methods Error

**File**: `/internal/declarative/planner/auth_strategy_planner.go`
**Lines**: 214-216, 219-222

**Root Cause**: Similar to the key-auth issue, the planner uses `"openid-connect"` (with hyphen) instead of `"openid_connect"` (with underscore) as the key in the configs map. Additionally, the error suggests `auth_methods` should be an array but the code correctly passes it as an array from the SDK type, so this might be a secondary effect of the wrong key name.

**Problematic Code**:
```go
// Line 219: Wrong key name with hyphen
fields["configs"] = map[string]interface{}{
    "openid-connect": oidcConfig,
}
```

**Fix Required**: Change to use underscore:
```go
fields["configs"] = map[string]interface{}{
    "openid_connect": oidcConfig,
}
```

### 4. API Version Content Validation Error

**File**: `/internal/declarative/resources/api_version.go`
**Lines**: 171-202

**Root Cause**: The custom UnmarshalJSON method correctly handles various spec formats and wraps them in a `content` field. However, the error "content must be valid specification" suggests the API is receiving invalid OpenAPI/AsyncAPI content. This could be due to:

1. The YAML example files having `document:` instead of `content:` in the spec field
2. The content being passed as a YAML string when the API expects JSON

**Example from** `/docs/examples/declarative/namespace/single-team/api.yaml`:
```yaml
spec:
  document: |
    openapi: 3.0.0
    # ... YAML content
```

The code correctly handles this by converting to JSON and wrapping in a `content` field, but the API might be rejecting YAML-formatted OpenAPI specs.

### 5. API Publication Auth Strategy IDs Format Error

**File**: `/internal/declarative/resources/api_publication.go`
**Line**: 162

**Root Cause**: The `auth_strategy_ids` field expects UUID-formatted strings, but the code is receiving reference names instead. In the example YAML:

```yaml
auth_strategy_ids: 
  - api-key-auth  # This is a ref name, not a UUID
```

The planner needs to resolve these reference names to actual Konnect IDs (UUIDs) before sending to the API. The resource definition correctly maps this field in `GetReferenceFieldMappings()`, but the resolution might not be happening during planning/execution.

## Additional Findings

### Portal Custom Domain Resource Structure

**File**: `/internal/declarative/resources/portal_custom_domain.go`

The resource now embeds `CreatePortalCustomDomainRequest` directly:
```go
type PortalCustomDomainResource struct {
    kkComps.CreatePortalCustomDomainRequest `yaml:",inline" json:",inline"`
    Ref    string `yaml:"ref,omitempty" json:"ref,omitempty"`
    Portal string `yaml:"portal,omitempty" json:"portal,omitempty"`
}
```

This change aligns with other resources but introduces the nil pointer issue mentioned above.

### Impact on Other Resources

The portal custom domain change itself didn't directly affect other resources. The other errors are pre-existing issues in the codebase that were revealed during testing.

## Recommendations

1. **Immediate Fixes**:
   - Add nil check for `Ssl` field in portal custom domain planning
   - Fix key names in auth strategy planner (use underscores not hyphens)
   - Ensure API version specs are converted to valid JSON format
   - Verify reference resolution is working for auth_strategy_ids

2. **Testing**:
   - Add unit tests for nil pointer scenarios
   - Add integration tests that validate against actual Konnect API
   - Test with various spec formats (YAML, JSON, inline vs file)

3. **Code Review**:
   - Review all places where SDK structs are embedded
   - Check for consistent key naming (underscore vs hyphen)
   - Verify all reference fields are properly resolved before API calls

## Conclusion

The errors are primarily caused by:
1. Missing nil checks after struct embedding changes
2. Incorrect key naming conventions (hyphens vs underscores)
3. Reference resolution not happening before API calls
4. Spec format compatibility issues

All issues have clear fixes and are localized to specific files and functions.