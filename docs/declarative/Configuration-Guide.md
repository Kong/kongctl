# Declarative Configuration Guide

This guide provides comprehensive documentation for using declarative 
configuration with kongctl to manage Kong Konnect resources.

## Table of Contents

- [Overview](#overview)
- [Configuration Structure](#configuration-structure)
- [Kongctl Metadata](#kongctl-metadata)
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
  - ref: developer-portal            # Reference identifier
    name: "Developer Portal"         # Display name
    # ... portal configuration

apis:
  - ref: payments-api               # Reference identifier  
    name: "Payments API"            # Display name
    # ... API configuration
    
application_auth_strategies:
  - ref: oauth-strategy             # Reference identifier
    name: "OAuth 2.0 Strategy"      # Display name
    # ... auth strategy configuration
```

### Resource Identifiers

Each resource has three potential identifiers:

- **id**: UUID assigned by Konnect (never appears in configuration files)
- **name**: Human-friendly display name (can contain spaces)
- **ref**: Computer-friendly reference identifier (used for cross-references)

### Resource References

Resources reference each other by ref:

```yaml
application_auth_strategies:
  - ref: oauth-strategy              # Reference identifier
    name: "OAuth 2.0 Strategy"       # Display name

portals:
  - ref: developer-portal
    name: "Developer Portal"
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
      protected: true  # This portal cannot be deleted via sync
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
namespace. File-level defaults will be supported in a future release.

## Resource Types

### APIs with Child Resources

```yaml
apis:
  - ref: inventory-api              # Reference identifier
    name: "Inventory Management API" # Display name
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
  - ref: partner-portal             # Reference identifier
    name: "Partner API Portal"      # Display name
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