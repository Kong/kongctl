# Future Work: Configuration Discovery Feature

## Overview

Implement visibility into unmanaged fields to help users progressively build configurations.

## Problem Statement

When adopting declarative configuration, users often start with minimal configs and gradually add more fields. Currently, there's no easy way to discover which fields are being managed by Konnect but not declared in their configuration files.

## Proposed Solution

Add a `--show-unmanaged` flag to apply/sync commands that displays fields present in Konnect but not in the user's configuration.

## Technical Approach

```go
// Add to apply/sync commands
cmd.Flags().Bool("show-unmanaged", false, "Show unmanaged fields after execution")

// Discovery logic
type UnmanagedFields struct {
    ResourceType string
    ResourceName string
    Fields       map[string]interface{}
}

func DiscoverUnmanagedFields(current state.Portal, desired resources.PortalResource) UnmanagedFields {
    unmanaged := UnmanagedFields{
        ResourceType: "portal",
        ResourceName: current.Name,
        Fields:       make(map[string]interface{}),
    }
    
    // Check each field in current state
    if desired.DisplayName == nil && current.DisplayName != "" {
        unmanaged.Fields["display_name"] = current.DisplayName
    }
    
    if desired.AuthenticationEnabled == nil {
        unmanaged.Fields["authentication_enabled"] = current.AuthenticationEnabled
    }
    
    // Continue for all fields...
    return unmanaged
}
```

## Expected Output

```
Discovered unmanaged fields for portal "my-portal":
  - display_name: "Developer Portal"
  - authentication_enabled: true
  - rbac_enabled: false

To manage these fields, add them to your configuration.
```

## Use Cases

1. **Progressive Configuration Building**: Start with minimal config and discover additional fields to manage
2. **Migration from UI/API**: Understand what fields were set outside of declarative config
3. **Configuration Completeness**: Ensure all important fields are explicitly managed

## Implementation Considerations

- Performance impact of field-by-field comparison
- Handling nested objects and arrays
- Differentiating between unset and default values
- Output format for large numbers of unmanaged fields

## Priority

Low - Nice to have feature for improving user experience, but not critical for core functionality.