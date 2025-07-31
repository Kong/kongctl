# Flow Report: Portal Custom Domain Validation Error

## Executive Summary

This report documents the complete code execution flow for the portal_custom_domain validation error that occurs when running `k sync -f docs/examples/declarative/namespace/single-team -R`. The error "invalid portal_custom_domain '': invalid custom domain ref: ref cannot be empty" occurs due to a critical issue in the resource extraction process where the `ref` field from nested custom domains is not properly preserved.

## Error Reproduction

Command: `k sync -f docs/examples/declarative/namespace/single-team -R`

Error: `Error: failed to load configuration: invalid portal_custom_domain '': invalid custom domain ref: ref cannot be empty`

## Complete Execution Flow

### 1. Command Entry Point

```
internal/cmd/root/verbs/sync/sync.go:NewSyncCmd()
├─ Creates sync command wrapper
├─ Sets up context with Verb=sync and Product=konnect
└─ Delegates to konnect command's RunE function
```

### 2. Konnect Declarative Sync Command

```
internal/cmd/root/products/konnect/declarative/declarative.go:runSync()
├─ Line 1033: Entry point for sync execution
├─ Line 1122: Parses sources from filenames
├─ Line 1129: Creates new loader instance
└─ Line 1130: Calls ldr.LoadFromSources(sources, recursive)
```

### 3. Configuration Loading Process

```
internal/declarative/loader/loader.go:LoadFromSources()
├─ Line 55: Main loading entry point
├─ Line 76-125: Iterates through sources
│   └─ Line 84: For directory source, calls loadDirectorySource()
├─ Line 129: Applies SDK defaults
└─ Line 132: Calls validateResourceSet() - WHERE ERROR OCCURS
```

### 4. Directory Loading

```
internal/declarative/loader/loader.go:loadDirectorySource()
├─ Walks directory recursively
├─ Finds YAML files: api.yaml, auth-strategy.yaml, portal.yaml
└─ For each file:
    ├─ Calls loadSingleFile()
    └─ Merges into ResourceSet
```

### 5. YAML Parsing and Resource Extraction

```
internal/declarative/loader/loader.go:parseYAML()
├─ Line 183: Parses YAML content
├─ Line 236: Applies namespace defaults
├─ Line 251: Extracts references
└─ Line 255: Calls extractNestedResources()
```

### 6. Nested Resource Extraction (CRITICAL ISSUE)

```
internal/declarative/loader/loader.go:extractNestedResources()
├─ Line 924-963: Portal resource extraction
└─ Line 936-940: Custom domain extraction
    ├─ if portal.CustomDomain != nil {
    ├─     customDomain := *portal.CustomDomain  // ISSUE: Ref field not preserved
    ├─     customDomain.Portal = portal.Ref     // Only parent ref is set
    └─     rs.PortalCustomDomains = append(rs.PortalCustomDomains, customDomain)
```

### 7. Validation Phase (ERROR OCCURS)

```
internal/declarative/loader/validator.go:validateResourceSet()
├─ Line 340-345: Portal custom domain validation
└─ Line 343: Error construction
    ├─ domain.GetRef() returns "" (empty string)
    └─ Returns: "invalid portal_custom_domain '': invalid custom domain ref: ref cannot be empty"
```

### 8. Custom Domain Validation

```
internal/declarative/resources/portal_custom_domain.go:Validate()
├─ Line 23: Entry point
├─ Line 24: Calls ValidateRef(d.Ref)
└─ Returns error when Ref is empty
```

## File Interconnections

### Configuration Files
- `docs/examples/declarative/namespace/single-team/portal.yaml`
  - Defines portal with nested custom_domain
  - Contains `ref: internal-domain` in custom_domain section

### Core Components

1. **Command Layer**
   - `internal/cmd/root/verbs/sync/sync.go` - Sync verb command
   - `internal/cmd/root/products/konnect/declarative/declarative.go` - Declarative operations

2. **Loader Layer**
   - `internal/declarative/loader/loader.go` - Main configuration loading
   - `internal/declarative/loader/validator.go` - Resource validation
   - `internal/declarative/loader/source.go` - Source type definitions

3. **Resource Layer**
   - `internal/declarative/resources/portal.go` - Portal resource definition
   - `internal/declarative/resources/portal_custom_domain.go` - Custom domain resource
   - `internal/declarative/resources/validation.go` - Common validation functions

4. **SDK Integration**
   - Uses `github.com/Kong/sdk-konnect-go/models/components` for base types

## Root Cause Analysis

### The Problem

When extracting nested custom domains from portals in `extractNestedResources()`:

1. The code copies the custom domain struct: `customDomain := *portal.CustomDomain`
2. It sets the parent reference: `customDomain.Portal = portal.Ref`
3. **BUT** it fails to preserve the original `Ref` field from the nested structure

### Data Flow Issue

```yaml
# Input (portal.yaml)
custom_domain:
  ref: internal-domain      # This ref is lost
  domain: "api.internal.example.com"

# After extraction
PortalCustomDomainResource {
  Ref: ""                   # Empty - not preserved!
  Portal: "internal-portal" # Parent ref correctly set
  Domain: "api.internal.example.com"
}
```

### Why the Error Message is Confusing

1. The error shows `invalid portal_custom_domain ''` because `domain.GetRef()` returns empty string
2. No indication of which portal contains the problematic custom domain
3. The actual ref value "internal-domain" from YAML is lost during extraction

## Validation Logic Flow

```
validateResourceSet()
├─ Iterates through PortalCustomDomains slice
├─ Calls domain.Validate() for each
│   └─ ValidateRef(d.Ref) checks if Ref is empty
└─ Formats error with domain.GetRef() which returns ""
```

## Reference Resolution System

The loader maintains reference registries for cross-referencing:
- Portal refs/names
- Auth strategy refs/names  
- API refs/names
- Child resource refs

These are used during validation to ensure references point to existing resources.

## Key Design Patterns

1. **Nested Resource Extraction**: Child resources are extracted from parent resources and stored separately
2. **Reference Preservation**: Each extracted resource maintains a reference to its parent
3. **Two-Phase Processing**: Parse first, validate after all resources are loaded
4. **Namespace Inheritance**: Child resources inherit namespace from parent if not specified

## Impact Analysis

This bug affects:
- Any portal configuration with custom_domain defined
- Both single file and directory-based configurations
- Namespace-scoped and default namespace scenarios

The error prevents the entire configuration from loading, blocking all sync operations.

## Recommendations

1. **Fix the Extraction Logic**: Ensure all fields including `Ref` are properly preserved when extracting nested resources
2. **Improve Error Messages**: Include parent portal reference and file location in error messages
3. **Add Validation Tests**: Test cases for nested resource extraction with all fields populated
4. **Consider Alternative Approaches**: 
   - Validate nested resources before extraction (with better context)
   - Use deep copy instead of struct copy to preserve all fields
   - Add logging/tracing during extraction for debugging