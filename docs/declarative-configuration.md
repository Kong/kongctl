# Declarative Configuration Reference

This guide provides a comprehensive reference for kongctl's declarative configuration features for Kong Konnect.

## Overview

Declarative configuration enables you to manage your Konnect resources as code using YAML files. This approach is ideal for:

- Version-controlled API infrastructure
- Automated deployments via CI/CD
- Consistent environments (dev, staging, production)
- Team collaboration through code review
- Disaster recovery and backup

## Quick Start

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

## Supported Resources

### Parent Resources
- **APIs**: Core API definitions
- **Portals**: Developer portal instances
- **Application Auth Strategies**: Authentication methods for applications

### Child Resources
- **API Versions**: Different versions of an API
- **API Publications**: Publishing APIs to portals
- **API Implementations**: Linking APIs to Kong Gateway services
- **API Documents**: Additional API documentation

## Configuration Structure

### Basic Structure

```yaml
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

### Resource References

Resources reference each other using the `ref` field:

```yaml
apis:
  - ref: payment-api        # This ref is used by other resources
    name: "Payment API"

api_publications:
  - ref: payment-pub
    api: payment-api        # References the API above
    portal: main-portal     # References a portal ref
```

## YAML Tags for External Content

### Loading Files

Load content from external files using the `!file` tag:

```yaml
apis:
  - ref: users-api
    name: "Users API"
    description: !file ./docs/api-description.md
```

### Extracting Values from OpenAPI

Extract specific values from OpenAPI specifications:

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

For complex extractions, use the map format:

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

## Multi-Resource Configurations

### Complete Example

```yaml
# Complete API platform setup
portals:
  - ref: public-portal
    name: "public-portal"
    display_name: "Public APIs"
    authentication_enabled: true

  - ref: partner-portal  
    name: "partner-portal"
    display_name: "Partner APIs"
    authentication_enabled: true
    rbac_enabled: true

apis:
  - ref: users-api
    name: "Users API"
    description: "User management and authentication"
    version: "v2.0.0"
    labels:
      team: identity
      tier: public
    
    # Nested versions
    versions:
      - ref: users-v2
        name: "v2.0.0"
        gateway_service:
          control_plane_id: "cp-123"
          id: "service-456"
        spec: !file ./specs/users-v2.yaml
    
    # Nested publications
    publications:
      - ref: users-public
        portal: public-portal
        visibility: public
      
      - ref: users-partner
        portal: partner-portal
        visibility: private

# Additional publications (alternative to nested)
api_publications:
  - ref: billing-api-pub
    api: billing-api
    portal: partner-portal
    visibility: private
```

## Team Organization Pattern

Organize configurations by team:

```yaml
# platform/main.yaml
portals:
  - ref: main-portal
    name: "main-portal"
    display_name: "Developer Portal"

# Load team configurations
apis:
  - !file ./teams/identity/apis.yaml
  - !file ./teams/payments/apis.yaml
  - !file ./teams/shipping/apis.yaml
```

```yaml
# teams/identity/apis.yaml
ref: users-api
name: "Users API"
description: "Identity and authentication"
labels:
  team: identity
  owner: alice@company.com
```

## Namespace Management

Use namespaces to isolate team resources:

```yaml
# Set namespace for all resources in file
_defaults:
  kongctl:
    namespace: payments-team

apis:
  - ref: payment-api
    name: "Payment API"
    # Inherits namespace: payments-team
    
  - ref: billing-api
    name: "Billing API"
    kongctl:
      namespace: billing-team  # Override namespace
```

### Namespace Benefits

- Prevents accidental cross-team modifications
- Enables safe multi-team collaboration
- Allows team-specific sync operations

## Commands

### Plan Command

Preview changes before applying:

```shell
# Generate plan
kongctl plan -f config.yaml

# Save plan to file
kongctl plan -f config.yaml -o plan.json

# Plan with specific profile
kongctl plan -f config.yaml --profile production
```

### Apply Command

Apply configuration changes:

```shell
# Apply directly from config
kongctl apply -f config.yaml

# Apply from saved plan
kongctl apply --plan plan.json

# Dry run
kongctl apply -f config.yaml --dry-run
```

### Sync Command

Ensure Konnect matches your configuration exactly:

```shell
# Preview sync changes
kongctl sync -f config.yaml --dry-run

# Sync specific namespace
kongctl sync -f team-config.yaml  # Only affects that namespace

# Force sync (skip confirmations)
kongctl sync -f config.yaml --force
```

### Diff Command

Compare current state with desired configuration:

```shell
kongctl diff -f config.yaml
```

## Best Practices

### 1. File Organization

```
config/
├── portals.yaml          # Portal definitions
├── apis/
│   ├── users-api.yaml
│   ├── products-api.yaml
│   └── billing-api.yaml
├── publications.yaml     # API publications
└── specs/               # OpenAPI specifications
    ├── users-v1.yaml
    └── products-v2.yaml
```

### 2. Use Version Control

```shell
git add config/
git commit -m "Add users API v2"
git push
```

### 3. Environment Separation

Use profiles for different environments:

```shell
# Development
kongctl apply -f config.yaml --profile dev

# Production (with plan review)
kongctl plan -f config.yaml --profile prod -o prod-plan.json
# Review plan...
kongctl apply --plan prod-plan.json --profile prod
```

### 4. Protect Critical Resources

```yaml
apis:
  - ref: payment-api
    name: "Payment API"
    kongctl:
      protected: true  # Prevents accidental deletion
```

### 5. Use Namespaces for Teams

```yaml
_defaults:
  kongctl:
    namespace: platform-team
    protected: false
```

## Migration from Imperative

### Step 1: Export Current State

```shell
kongctl dump > current-state.yaml
```

### Step 2: Clean Up Export

Remove server-generated fields:
- `id` fields
- `created_at`, `updated_at`
- System labels

### Step 3: Add References

Replace IDs with meaningful refs:

```yaml
# Before
apis:
  - id: "123e4567-e89b-12d3-a456-426614174000"
    name: "Users API"

# After  
apis:
  - ref: users-api
    name: "Users API"
```

### Step 4: Test Migration

```shell
# Dry run to verify
kongctl plan -f migrated-config.yaml
kongctl apply -f migrated-config.yaml --dry-run
```

## Troubleshooting

See the [Troubleshooting Guide](troubleshooting.md) for common issues and solutions.

## Examples

Browse the [examples directory](examples/declarative/) for:
- Basic configurations
- Multi-resource setups
- Team patterns
- CI/CD integration

## Related Documentation

- [Getting Started Guide](getting-started.md) - Step-by-step tutorial
- [Configuration Guide](declarative/Configuration-Guide.md) - Detailed configuration reference
- [YAML Tags Reference](declarative/YAML-Tags-Reference.md) - External file loading
- [CI/CD Integration](declarative/ci-cd-integration.md) - Automation examples