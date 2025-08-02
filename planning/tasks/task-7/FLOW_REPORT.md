# Flow Report: Runtime Errors in kongctl Execution Path

## Executive Summary

This report maps the complete execution flow in kongctl from loading YAML files to making API calls, focusing on where runtime validation errors occur for the following resources:
- `portal_custom_domain` - SSL field validation error
- `application_auth_strategy` - configs and auth_methods errors  
- `api_version` - content validation error
- `api_publication` - UUID format error

## Error Reproduction

Command: `./kongctl apply -f docs/examples/declarative/namespace/single-team -R --pat $(cat ~/.konnect/claude.pat)`

Multiple validation errors occur during API calls to Konnect.

## Execution Flow

### 1. Command Entry Point

**File**: `/internal/cmd/root/verbs/apply/apply.go`
- The `apply` command delegates to the konnect declarative command
- Sets up context with Verb=apply and Product=konnect
- Entry point: `runApply()` function in `/internal/cmd/root/products/konnect/declarative/declarative.go:641`

### 2. Configuration Loading Phase

**File**: `/internal/declarative/loader/loader.go`

#### 2.1 Source Parsing
- `LoadFromSources()` (line 55) - Main entry point for loading configuration
- `loadSingleFile()` (line 161) - Loads individual YAML files
- `parseYAML()` (line 183) - Parses YAML content into ResourceSet

#### 2.2 Resource Extraction
- `extractNestedResources()` (line 241) - **Critical function** that extracts child resources from parent structures
- For portal custom domains (lines in extract function):
  ```go
  if portal.CustomDomain != nil {
      customDomain := *portal.CustomDomain
      customDomain.Portal = portal.Ref // Set parent reference
      rs.PortalCustomDomains = append(rs.PortalCustomDomains, customDomain)
  }
  ```

### 3. Planning Phase

**File**: `/internal/declarative/planner/planner.go`
- `GeneratePlan()` - Creates execution plan from ResourceSet
- Delegates to resource-specific planners

#### 3.1 Portal Custom Domain Planning Error

**File**: `/internal/declarative/planner/portal_child_planner.go`
**Lines**: 396-401

**Root Cause**: Nil pointer dereference
```go
// Line 397: Direct access without nil check
if domain.Ssl.DomainVerificationMethod != "" {
    sslFields := make(map[string]interface{})
    sslFields["domain_verification_method"] = string(domain.Ssl.DomainVerificationMethod)
    fields["ssl"] = sslFields
}
```

**Data Flow**:
1. YAML has no SSL field: `custom_domain: { hostname: "api.internal.example.com" }`
2. Resource struct has `Ssl *CreatePortalCustomDomainSSL` (nil by default)
3. Planner tries to access `domain.Ssl.DomainVerificationMethod` without nil check
4. Results in panic or error

#### 3.2 Application Auth Strategy Planning Errors

**File**: `/internal/declarative/planner/auth_strategy_planner.go`

**Key-Auth Error** (Lines 187-192):
```go
// Line 188: Wrong key name with hyphen
fields["configs"] = map[string]interface{}{
    "key-auth": map[string]interface{}{  // Should be "key_auth"
        "key_names": strategy.AppAuthStrategyKeyAuthRequest.Configs.KeyAuth.KeyNames,
    },
}
```

**OAuth2 Error** (Lines 219-222):
```go
// Line 219: Wrong key name with hyphen
fields["configs"] = map[string]interface{}{
    "openid-connect": oidcConfig,  // Should be "openid_connect"
}
```

### 4. Execution Phase

**File**: `/internal/declarative/executor/executor.go`

#### 4.1 Execution Flow
1. `Execute()` (line 126) - Main execution entry point
2. `executeChange()` (line 235) - Executes individual changes
3. Routes to resource-specific executors based on ResourceType

#### 4.2 Portal Custom Domain Execution

**File**: `/internal/declarative/executor/portal_domain_adapter.go`

**MapCreateFields()** (Lines 23-49):
- Correctly handles SSL field if present
- Creates proper SDK request structure
```go
if sslData, ok := fields["ssl"].(map[string]interface{}); ok {
    ssl := kkComps.CreatePortalCustomDomainSSL{}
    if method, ok := sslData["domain_verification_method"].(string); ok {
        ssl.DomainVerificationMethod = kkComps.PortalCustomDomainVerificationMethod(method)
    }
    create.Ssl = ssl
}
```

### 5. API Call Phase

**File**: `/internal/declarative/state/client.go`

**CreatePortalCustomDomain()** (Lines ~305-318):
```go
func (c *Client) CreatePortalCustomDomain(
    ctx context.Context,
    portalID string,
    req kkComps.CreatePortalCustomDomainRequest,
) error {
    _, err := c.portalCustomDomainAPI.CreatePortalCustomDomain(ctx, portalID, req)
    if err != nil {
        return fmt.Errorf("failed to create portal custom domain: %w", err)
    }
    return nil
}
```

## Error Analysis

### 1. Portal Custom Domain SSL Error

**Error**: `"field":"ssl","reason":"must match exactly one schema in oneOf"`

**Flow**:
1. Planner creates fields with empty SSL object due to nil check bug
2. Executor creates request with empty SSL struct
3. Konnect API rejects empty SSL as it doesn't match any oneOf schemas
4. API expects either no SSL field or a fully populated SSL configuration

### 2. Application Auth Strategy Errors

**Error**: `"configs is required for key_auth strategy"`

**Flow**:
1. Planner uses "key-auth" (hyphen) as map key
2. API expects "key_auth" (underscore)
3. API doesn't find required configs under correct key
4. Validation fails

### 3. API Version Content Error

**Error**: `"content must be valid specification"`

**File**: `/internal/declarative/resources/api_version.go`
- Custom UnmarshalJSON handles various spec formats
- Issue may be YAML content being passed as string instead of JSON object

### 4. API Publication UUID Error

**Error**: `"auth_strategy_ids.0":"must match format \"uuid\""`

**Flow**:
1. YAML contains reference names: `auth_strategy_ids: [api-key-auth]`
2. Reference resolution not happening during planning
3. API receives reference name instead of UUID
4. Validation fails expecting UUID format

## Data Transformation Summary

1. **YAML → ResourceSet**: Loader parses and extracts nested resources
2. **ResourceSet → Plan**: Planner creates change objects with fields
3. **Plan → API Requests**: Executor maps fields to SDK request types
4. **API Requests → HTTP**: State client calls SDK methods

## Key Files and Functions

- **Entry**: `/internal/cmd/root/products/konnect/declarative/declarative.go:runApply()`
- **Loading**: `/internal/declarative/loader/loader.go:parseYAML()`
- **Extraction**: `/internal/declarative/loader/loader.go:extractNestedResources()`
- **Planning**: `/internal/declarative/planner/portal_child_planner.go:planPortalCustomDomainCreate()`
- **Execution**: `/internal/declarative/executor/portal_domain_adapter.go:MapCreateFields()`
- **API Call**: `/internal/declarative/state/client.go:CreatePortalCustomDomain()`

## Example Data Flow

### Portal Custom Domain
```yaml
# Input YAML
custom_domain:
  ref: internal-domain
  hostname: "api.internal.example.com"
  enabled: true
```

```go
// After extraction (ResourceSet)
PortalCustomDomainResource{
  CreatePortalCustomDomainRequest{
    Hostname: "api.internal.example.com",
    Enabled: true,
    Ssl: nil,  // No SSL in YAML
  },
  Ref: "internal-domain",
  Portal: "internal-portal",
}
```

```go
// Planner creates fields map
fields := map[string]interface{}{
  "hostname": "api.internal.example.com",
  "enabled": true,
  "ssl": map[string]interface{}{},  // BUG: Empty SSL added due to nil check issue
}
```

```go
// Executor creates SDK request
CreatePortalCustomDomainRequest{
  Hostname: "api.internal.example.com",
  Enabled: true,
  Ssl: CreatePortalCustomDomainSSL{},  // Empty struct sent to API
}
```

## Recommendations

1. **Add nil checks** in portal_child_planner.go before accessing SSL fields
2. **Fix key naming** in auth_strategy_planner.go (use underscores not hyphens)
3. **Ensure reference resolution** happens before API calls
4. **Validate spec content format** in api_version processing
5. **Add comprehensive tests** for all transformation steps