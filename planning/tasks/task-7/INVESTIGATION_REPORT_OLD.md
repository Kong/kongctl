# Investigation Report: Portal Custom Domain Validation Error

## Issue Summary

When running `k sync -f docs/examples/declarative/namespace/single-team -R`, the command fails with:
```
Error: failed to load configuration: invalid portal_custom_domain '': invalid custom domain ref: ref cannot be empty
```

## Investigation Findings

### 1. Configuration Files

The directory `docs/examples/declarative/namespace/single-team` contains three YAML files:
- `api.yaml` - API configuration with namespace
- `auth-strategy.yaml` - Authentication strategies configuration
- `portal.yaml` - Portal configuration with custom domain

### 2. Portal Configuration Structure

In `portal.yaml`, the portal has a nested custom_domain section:
```yaml
custom_domain:
  ref: internal-domain
  domain: "api.internal.example.com"
```

The ref field is clearly defined with value "internal-domain".

### 3. Error Source

The error originates from:
- **File**: `internal/declarative/loader/validator.go`
- **Line**: 343
- **Function**: `validateResourceSet`

The error message is constructed as:
```go
return fmt.Errorf("invalid portal_custom_domain %q: %w", domain.GetRef(), err)
```

Where `domain.GetRef()` returns an empty string, causing the %q format to show ''.

### 4. Validation Chain

1. `PortalCustomDomainResource.Validate()` is called (portal_custom_domain.go:23)
2. It calls `ValidateRef(d.Ref)` (portal_custom_domain.go:24)
3. `ValidateRef` checks if ref is empty (validation.go:23-24)
4. Returns "ref cannot be empty" error

### 5. Root Cause

The issue occurs during the extraction of nested resources in `loader.go`:

```go
// Extract custom domain (single resource)
if portal.CustomDomain != nil {
    customDomain := *portal.CustomDomain
    customDomain.Portal = portal.Ref // Set parent reference
    
    rs.PortalCustomDomains = append(rs.PortalCustomDomains, customDomain)
}
```

**The problem**: When the loader extracts the nested custom_domain from the portal resource, it creates a copy using `customDomain := *portal.CustomDomain`. However, the ref field is not being properly copied from the nested structure to the extracted `PortalCustomDomainResource`.

### 6. Why the Error Message is Not Helpful

The error message shows `invalid portal_custom_domain ''` because:
1. The validator calls `domain.GetRef()` which returns an empty string
2. The %q format in the error message shows this as '' (empty quotes)
3. There's no indication of which portal contains the problematic custom domain
4. The actual ref value from the YAML ("internal-domain") is lost during extraction

## Recommendations

1. **Fix the extraction logic**: Ensure the ref field is properly copied when extracting nested custom domains from portals.

2. **Improve error messages**: 
   - Include the parent portal ref in the error message
   - Show the file path where the error occurred
   - If ref is empty, mention that it might be a parsing issue

3. **Add validation earlier**: Validate nested resources before extraction to catch issues with better context.

4. **Debug logging**: Add trace logging during resource extraction to help diagnose similar issues.

## Technical Details

### Call Stack
1. `sync` command execution
2. `loader.LoadFromSources()` 
3. `loader.loadFile()`
4. `loader.extractNestedResources()`
5. Portal custom domain extraction (loses ref)
6. `loader.validateResourceSet()`
7. `PortalCustomDomainResource.Validate()` fails

### Affected Code Paths
- `internal/cmd/root/verbs/sync/sync.go` - Sync command entry point
- `internal/declarative/loader/loader.go:937` - Resource extraction
- `internal/declarative/loader/validator.go:343` - Validation error
- `internal/declarative/resources/portal_custom_domain.go:24` - Ref validation
- `internal/declarative/resources/validation.go:24` - Empty ref check