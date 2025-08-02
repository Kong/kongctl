# Declarative Configuration Reference

This guide provides a comprehensive reference for kongctl's declarative configuration features for Kong Konnect.

## Table of Contents

- [Overview](#overview)
- [Core Concepts](#core-concepts)
- [Resource Types](#resource-types)
- [Configuration Structure](#configuration-structure)
- [Kongctl Metadata](#kongctl-metadata)
- [YAML Tags](#yaml-tags)
- [Commands Reference](#commands-reference)
- [Best Practices](#best-practices)
- [Migration Guide](#migration-guide)

## Overview

Declarative configuration enables you to manage your Konnect resources as code using YAML files. This approach is ideal for:

- Version-controlled API infrastructure
- Automated deployments via CI/CD
- Consistent environments (dev, staging, production)
- Team collaboration through code review
- Disaster recovery and backup

### Quick Start

```yaml
# api-config.yaml
apis:
  - ref: my-api
    name: "My API"
    description: "Example API"
    version: "v1.0.0"
```

```shell
# Preview changes
kongctl plan -f api-config.yaml

# Apply configuration
kongctl apply -f api-config.yaml
```

## Core Concepts

### Declarative vs Imperative

**Imperative** (traditional CLI commands):
```bash
kongctl create portal developer-portal
kongctl update portal developer-portal --display-name "Dev Portal"
```

**Declarative** (configuration as code):
```yaml
portals:
  - ref: developer-portal
    name: "developer-portal"
    display_name: "Dev Portal"
    authentication_enabled: true
```

### Key Principles

1. **Desired State**: Define what you want, not how to get there
2. **Idempotency**: Apply configurations multiple times safely
3. **State-Free**: No local state files - current state queried from Konnect
4. **Plan-Based**: Preview changes before applying

### Resource Identity

Resources have two types of identifiers:

- **id**: UUID assigned by Konnect (not used in configuration files)
- **ref**: User-defined reference identifier for cross-references

Additionally, resources have a `name` field for display:
- **name**: Display field that may or may not be unique depending on resource type
- The `name` field should not be used as an identifier

```yaml
application_auth_strategies:
  - ref: oauth-strategy              # Identifier for cross-references
    name: "OAuth 2.0 Strategy"       # Display field (not an identifier)

portals:
  - ref: developer-portal
    name: "Developer Portal"         # Display field (not an identifier)
    default_application_auth_strategy: oauth-strategy  # References the ref
```

## Resource Types

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

### Resource Relationships

```
Portal
  └── API Publication → API
                         ├── API Version
                         ├── API Document
                         └── API Implementation
```

## Configuration Structure

### Basic Structure

```yaml
# Optional defaults section
_defaults:
  kongctl:
    namespace: production
    protected: false

# Define portals
portals:
  - ref: developer-portal
    name: "developer-portal"
    display_name: "Developer Portal"
    description: "API documentation hub"

# Define APIs
apis:
  - ref: users-api
    name: "Users API"
    description: "User management"
    version: "v1.0.0"
    labels:
      team: platform

# Publish APIs to portals
api_publications:
  - ref: users-api-pub
    api: users-api
    portal: developer-portal
    visibility: public
```

### Nested vs Separate Configuration

Both approaches are supported:

**Nested Configuration**:
```yaml
apis:
  - ref: users-api
    name: "Users API"
    versions:
      - ref: v1
        name: "v1.0.0"
        spec: !file ./specs/users-v1.yaml
    publications:
      - ref: public
        portal: main-portal
        visibility: public
```

**Separate Configuration**:
```yaml
apis:
  - ref: users-api
    name: "Users API"

api_versions:
  - ref: v1
    api: users-api
    name: "v1.0.0"
    spec: !file ./specs/users-v1.yaml

api_publications:
  - ref: public
    api: users-api
    portal: main-portal
```

## Kongctl Metadata

The `kongctl` section provides tool-specific metadata for resource management. This section is **only supported on parent resources**.

### Protected Resources

The `protected` field prevents accidental deletion of critical resources:

```yaml
portals:
  - ref: production-portal
    name: "Production Portal"
    kongctl:
      protected: true  # Cannot be deleted until protection is removed
```

### Namespace Management

The `namespace` field enables multi-team resource isolation:

```yaml
apis:
  - ref: billing-api
    name: "Billing API"
    kongctl:
      namespace: finance-team  # Owned by finance team
      protected: false
```

### File-Level Defaults

Use `_defaults` to set default values for all resources in a file:

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

### Namespace Inheritance

Child resources automatically inherit the namespace of their parent resource:

```yaml
apis:
  - ref: user-api
    name: "User API"
    kongctl:
      namespace: platform-team  # ✅ Valid on parent
    
    versions:
      - ref: v1
        version: "1.0.0"
        # ❌ No kongctl section here - inherits from parent
        
    documents:
      - ref: changelog
        title: "Changelog"
        # ❌ No kongctl section here - inherits from parent
```

## YAML Tags

### Basic File Loading

Load content from external files using the `!file` tag:

```yaml
apis:
  - ref: users-api
    name: "Users API"
    description: !file ./docs/api-description.md
```

### Value Extraction

Extract specific values from YAML/JSON files:

```yaml
apis:
  - ref: users-api
    name: !file ./specs/openapi.yaml#info.title
    description: !file ./specs/openapi.yaml#info.description
    version: !file ./specs/openapi.yaml#info.version
    
    versions:
      - ref: v1
        spec: !file ./specs/openapi.yaml
```

### Map Format

For complex extractions:

```yaml
apis:
  - ref: products-api
    name: !file
      path: ./specs/products.yaml
      extract: info.title
    labels:
      contact: !file
        path: ./specs/products.yaml
        extract: info.contact.email
```

For comprehensive YAML tags documentation, see [YAML Tags Reference](declarative-yaml-tags.md).

## Commands Reference

### plan

Generate an execution plan showing what changes will be made:

```shell
# Generate plan from configuration
kongctl plan -f config.yaml

# Save plan to file
kongctl plan -f config.yaml -o plan.json

# Plan with specific namespace
kongctl plan -f config.yaml --namespace team-alpha
```

### apply

Apply configuration changes (create/update only):

```shell
# Apply directly from config
kongctl apply -f config.yaml

# Apply from saved plan
kongctl apply --plan plan.json

# Dry run
kongctl apply -f config.yaml --dry-run
```

### sync

Full synchronization including deletions:

```shell
# Preview sync changes
kongctl sync -f config.yaml --dry-run

# Sync specific namespace
kongctl sync -f team-config.yaml

# Force sync without confirmation
kongctl sync -f config.yaml --force
```

### diff

Show differences between current and desired state:

```shell
kongctl diff -f config.yaml
```

### dump

Export current Konnect state to YAML:

```shell
# Export all resources
kongctl dump > current-state.yaml

# Export specific namespace
kongctl dump --namespace team-alpha > team-state.yaml
```

## Best Practices

### File Organization

```
config/
├── _defaults.yaml        # Shared defaults
├── portals/             # Portal definitions
│   └── main.yaml
├── apis/                # API definitions
│   ├── users.yaml
│   └── products.yaml
├── publications/        # API publications
│   └── public.yaml
└── specs/              # OpenAPI specifications
    ├── users-v1.yaml
    └── products-v2.yaml
```

### Multi-Team Setup

Each team manages their own namespace:

```yaml
# team-alpha/config.yaml
_defaults:
  kongctl:
    namespace: team-alpha

apis:
  - ref: frontend-api
    name: "Frontend API"
    # Automatically in team-alpha namespace
```

### Environment Management

Use profiles for different environments:

```shell
# Development
kongctl apply -f config.yaml --profile dev

# Production with approval
kongctl plan -f config.yaml --profile prod -o prod-plan.json
# Review plan...
kongctl apply --plan prod-plan.json --profile prod
```

### Security Best Practices

1. **Protect production resources**:
   ```yaml
   apis:
     - ref: payment-api
       kongctl:
         namespace: production
         protected: true
   ```

2. **Use namespaces for isolation**:
   - One namespace per team
   - Separate namespaces for environments
   - Clear namespace ownership documentation

3. **Version control everything**:
   - Configuration files
   - OpenAPI specifications
   - Documentation

4. **Review plans before applying**:
   - Always use `plan` in production
   - Save plans for audit trail
   - Implement approval workflows

### Common Mistakes to Avoid

❌ **Setting kongctl on child resources**:
```yaml
# WRONG
apis:
  - ref: my-api
    kongctl:
      namespace: team-a
    versions:
      - ref: v1
        kongctl:  # ERROR: Not supported on child
          protected: true
```

✅ **Correct approach**:
```yaml
# RIGHT
apis:
  - ref: my-api
    kongctl:
      namespace: team-a
      protected: true  # Set on parent only
    versions:
      - ref: v1
        # No kongctl here
```

❌ **Using name as identifier**:
```yaml
# WRONG
api_publications:
  - ref: pub1
    api: "Users API"  # Using display name
```

✅ **Use ref for references**:
```yaml
# RIGHT
api_publications:
  - ref: pub1
    api: users-api  # Using ref
```

## Migration Guide

### From Imperative to Declarative

#### Step 1: Export Current State

```shell
kongctl dump > current-state.yaml
```

#### Step 2: Clean Up Export

Remove server-generated fields:
- `id` fields (except where required)
- `created_at`, `updated_at`
- System-generated labels

#### Step 3: Add References

Replace IDs with meaningful refs:

```yaml
# Before (exported)
apis:
  - id: "123e4567-e89b-12d3-a456-426614174000"
    name: "Users API"

# After (cleaned)
apis:
  - ref: users-api
    name: "Users API"
```

#### Step 4: Add Management Metadata

```yaml
apis:
  - ref: users-api
    name: "Users API"
    kongctl:
      namespace: production
      protected: true  # Protect during migration
```

#### Step 5: Test Migration

```shell
# Dry run to ensure no unexpected changes
kongctl sync -f migrated-config.yaml --dry-run

# Should show minimal changes (mainly adding labels)
```

#### Step 6: Apply Configuration

```shell
# First apply adds management labels
kongctl apply -f migrated-config.yaml

# Verify state matches
kongctl diff -f migrated-config.yaml
```

### Gradual Migration Strategy

For large deployments:

1. **Phase 1**: Export and document current state
2. **Phase 2**: Migrate non-critical resources
3. **Phase 3**: Migrate development/staging environments
4. **Phase 4**: Migrate production with protection enabled
5. **Phase 5**: Enable full management (remove protection)

## Field Validation

Kongctl uses strict YAML validation to catch configuration errors early:

```yaml
# This will cause an error
portals:
  - ref: my-portal
    name: "My Portal"
    lables:  # ❌ ERROR: Unknown field 'lables'. Did you mean 'labels'?
      team: platform
```

Common field name errors:
- `lables` → `labels`
- `descriptin` → `description`
- `displayname` → `display_name`
- `strategytype` → `strategy_type`

## Troubleshooting

For common issues and solutions, see the [Troubleshooting Guide](troubleshooting.md).

## Examples

Browse the [examples directory](examples/declarative/) for:
- Basic configurations
- Multi-resource setups
- Team collaboration patterns
- CI/CD integration

## Related Documentation

- [Getting Started Guide](declarative-getting-started.md) - Step-by-step tutorial
- [YAML Tags Reference](declarative-yaml-tags.md) - Comprehensive file loading guide
- [CI/CD Integration](declarative-ci-cd.md) - Automation examples
- [Troubleshooting Guide](troubleshooting.md) - Common issues and solutions