# Declarative Configuration Guide

This guide provides comprehensive documentation for using declarative 
configuration with kongctl to manage Kong Konnect resources.

## Table of Contents

- [Overview](#overview)
- [Configuration Structure](#configuration-structure)
- [Kongctl Metadata](#kongctl-metadata)
  - [Parent vs Child Resources](#parent-vs-child-resources)
  - [Kongctl Fields](#kongctl-fields)
  - [Namespace Inheritance](#namespace-inheritance)
  - [Default Namespace](#default-namespace)
  - [File-Level Defaults](#file-level-defaults)
  - [Namespace and Protected Field Behavior](#namespace-and-protected-field-behavior)
- [Resource Types](#resource-types)
- [Best Practices](#best-practices)
- [Common Mistakes](#common-mistakes)

## Overview

Kongctl's declarative configuration allows you to define Kong Konnect resources 
in YAML files and manage them using a plan-based workflow. Resources are 
identified by user-defined refs rather than server-assigned IDs, making 
configurations portable across environments.

## Configuration Structure

### Basic Structure

```yaml
# Top-level resource types
portals:
  - ref: developer-portal            # Identifier for cross-references
    name: "Developer Portal"         # Display field
    # ... portal configuration

apis:
  - ref: payments-api               # Identifier for cross-references  
    name: "Payments API"            # Display field
    # ... API configuration
    
application_auth_strategies:
  - ref: oauth-strategy             # Identifier for cross-references
    name: "OAuth 2.0 Strategy"      # Display field
    # ... auth strategy configuration
```

### Resource Identifiers and Display Fields

Resources have two types of identifiers:

- **id**: UUID assigned by Konnect (do not appear in configuration files, except in a small set of cases)
- **ref**: User-defined reference identifier (used for cross-references in configuration)

Additionally, resources may have a `name` field for display purposes:
- **name**: Display field that may or may not be unique depending on the resource type
- The `name` field should not be used as an identifier

### Resource References

Resources reference each other by ref:

```yaml
application_auth_strategies:
  - ref: oauth-strategy              # Identifier for cross-references
    name: "OAuth 2.0 Strategy"       # Display field (not an identifier)

portals:
  - ref: developer-portal
    name: "Developer Portal"         # Display field (not an identifier)
    default_application_auth_strategy: oauth-strategy  # References the ref
```

## Kongctl Metadata

The `kongctl` section provides tool-specific metadata for resource management. 
This section is **only supported on parent resources**.

### Parent vs Child Resources

**Parent Resources** (support kongctl metadata):
- APIs
- Portals  
- Application Auth Strategies
- Control Planes

**Child Resources** (do NOT support kongctl metadata):
- API Versions
- API Publications
- API Implementations
- API Documents
- Portal Pages
- Portal Snippets
- Portal Customizations
- Portal Custom Domains

### Kongctl Fields

#### protected

The `protected` field prevents accidental deletion of critical resources:

```yaml
portals:
  - name: production-portal
    display_name: "Production Portal"
    kongctl:
      protected: true  # This portal cannot be deleted or updated until protected is changed to false
```

#### namespace

The `namespace` field enables multi-team resource isolation:

```yaml
apis:
  - name: billing-api
    display_name: "Billing API"
    kongctl:
      namespace: finance-team  # Owned by finance team
      protected: false
```

### Namespace Inheritance

Child resources automatically inherit the namespace of their parent resource. 
You cannot set kongctl metadata on child resources:

```yaml
apis:
  - name: user-api
    kongctl:
      namespace: platform-team  # ✅ Valid on parent
    
    versions:
      - name: v1
        version: "1.0.0"
        # ❌ No kongctl section here - inherits from parent
        
    documents:
      - name: changelog
        title: "Changelog"
        # ❌ No kongctl section here - inherits from parent
```

### Default Namespace

When no namespace is specified, resources are assigned to the "default" 
namespace.

### File-Level Defaults

You can specify default values for namespace and protected fields at the file 
level using the `_defaults` section:

```yaml
_defaults:
  kongctl:
    namespace: platform-team    # Default namespace for resources in this file
    protected: true            # Default protection status

portals:
  - ref: api-portal
    name: "API Portal"
    # Inherits namespace: platform-team and protected: true
    
  - ref: test-portal
    name: "Test Portal"
    kongctl:
      namespace: qa-team      # Overrides default namespace
      protected: false        # Overrides default protected
```

### Namespace and Protected Field Behavior

The following tables show how namespace and protected values are determined 
based on file defaults and explicit resource values:

#### Namespace Field Behavior

| File Default | Resource Value | Final Result | Notes |
|-------------|----------------|--------------|-------|
| Not set | Not set | "default" | System default |
| Not set | "team-a" | "team-a" | Resource explicit |
| Not set | "" (empty) | ERROR | Empty namespace not allowed |
| "team-b" | Not set | "team-b" | Inherits default |
| "team-b" | "team-a" | "team-a" | Resource overrides |
| "team-b" | "" (empty) | ERROR | Empty namespace not allowed |
| "" (empty) | Any value | ERROR | Empty default not allowed |

#### Protected Field Behavior

| File Default | Resource Value | Final Result | Notes |
|-------------|----------------|--------------|-------|
| Not set | Not set | false | System default |
| Not set | true | true | Resource explicit |
| Not set | false | false | Explicit false |
| true | Not set | true | Inherits default |
| true | false | false | Resource overrides |
| false | true | true | Resource overrides |

**Important**: Empty namespace values are always rejected. Every resource must 
have a non-empty namespace to ensure proper resource management and isolation.

## Resource Types

### APIs with Child Resources

```yaml
apis:
  - ref: inventory-api              # Identifier for cross-references
    name: "Inventory Management API" # Display field
    description: "Internal inventory tracking API"
    labels:
      team: logistics
      tier: internal
    kongctl:
      namespace: logistics-team
      protected: true
    
    # Child resources - no kongctl section
    versions:
      - ref: inventory-v2           # Child ref
        version: "2.0.0"
        spec: !file ./specs/inventory-v2.yaml
        
    publications:
      - ref: internal-portal-pub    # Child ref
        portal_id: internal-portal  # References portal by ref
        auth_strategy_ids: ["key-auth"]  # References auth strategy by ref
        
    implementations:
      - ref: prod-impl              # Child ref
        implementation_url: "https://api.internal.example.com/v2"
        service:
          id: "12345678-1234-1234-1234-123456789012"
          control_plane_id: prod-cp  # References control plane by ref
```

### Portals with Child Resources

```yaml
portals:
  - ref: partner-portal             # Identifier for cross-references
    name: "Partner API Portal"      # Display field
    kongctl:
      namespace: partnerships
      protected: true
    
    # Child resources - no kongctl section
    pages:
      - ref: getting-started        # Child ref
        slug: "/getting-started"
        title: "Getting Started"
        content: !file ./content/getting-started.md
        
    customization:
      ref: partner-theme            # Child ref
      theme:
        mode: "light"
        colors:
          primary: "#0066CC"
```

## Best Practices

1. **Always set namespace on parent resources** in multi-team environments
2. **Use protected: true** for production-critical resources
3. **Organize by team** - each team should use their own namespace
4. **Document namespace ownership** in your team's runbooks
5. **Never attempt to set kongctl on child resources** - it will be rejected

## Common Mistakes

### ❌ Setting kongctl on child resources

```yaml
# WRONG - This will cause an error
apis:
  - ref: my-api
    name: "My API"
    kongctl:
      namespace: team-a
    
    versions:
      - ref: v1
        version: "1.0.0"
        kongctl:  # ❌ ERROR: kongctl not supported on child resources
          protected: true
```

### ❌ Forgetting namespace in multi-team setups

```yaml
# RISKY - No namespace means "default" namespace
apis:
  - ref: payment-api
    name: "Payment API"
    # Missing kongctl.namespace - will go to "default"
```

### ✅ Correct usage

```yaml
# CORRECT - Namespace on parent only
apis:
  - ref: payment-api
    name: "Payment API"
    kongctl:
      namespace: payments-team
      protected: true
    
    versions:
      - ref: v1
        version: "1.0.0"
        # No kongctl here - correctly inherits from parent
```

## Command Examples

### Plan with namespace visibility

```bash
$ kongctl plan -f team-configs/
Loading configurations...
Found 2 namespace(s): platform-team, data-team

Planning changes for namespace: platform-team
- CREATE api "user-api"
- UPDATE portal "developer-portal"

Planning changes for namespace: data-team  
- CREATE api "analytics-api"
```

### Sync with namespace isolation

```bash
$ kongctl sync -f team-configs/
# Only affects resources in namespaces found in config files
# Resources in other namespaces are not touched
```

## See Also

- [YAML Tags Reference](./YAML-Tags-Reference.md) - Loading external files
- [Troubleshooting Guide](./Troubleshooting-Guide.md) - Common issues
- [Examples](../examples/declarative/) - Complete examples
