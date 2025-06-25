# Declarative Configuration Examples

This directory contains example declarative configuration files for kongctl. These
examples demonstrate various patterns and best practices for managing Kong/Konnect
resources declaratively.

## Directory Structure

- **`basic/`** - Simple, single-resource examples for getting started
- **`complex/`** - Advanced configurations showing real-world scenarios
- **`layouts/`** - Different organizational approaches for your configs

## Basic Examples

Start here if you're new to declarative configuration. Each example is in its own directory and can be run independently:

- `basic/portal-example/` - Basic portal configuration
- `basic/auth-strategy-example/` - Authentication strategy examples  
- `basic/control-plane-example/` - Control plane definitions
- `basic/api-with-children-example/` - API with nested resources (versions, publications, implementations)

## Complex Examples

Real-world scenarios and patterns. Each example is in its own directory:

- `complex/multi-resource-example/` - Multiple resource types in one file
- `complex/full-portal-setup-example/` - Complete portal with all dependencies
- `complex/api-lifecycle-example/` - API versioning and lifecycle management

## Layout Examples

Different ways to organize your configuration files:

### Flat Layout (`layouts/flat/`)
All resources in a single file. Good for:
- Small configurations
- Quick prototypes
- Simple deployments

### Organized Layout (`layouts/organized/`)
Resources organized by type in subdirectories. Good for:
- Large configurations
- Team collaboration
- Type-based access control

Structure:
```
organized/
├── portals/
├── auth-strategies/
├── control-planes/
└── apis/
```

### Mixed Layout (`layouts/mixed/`)
Hybrid approach with base configs and specific definitions. Good for:
- Shared base configurations
- Environment-specific overrides
- Modular configurations

## Key Concepts

### Resource References
Resources reference each other using the `ref` field:

```yaml
portals:
  - ref: my-portal  # This is the reference identifier
    name: "My Portal"
    default_application_auth_strategy_id: oauth-strategy  # References auth strategy

application_auth_strategies:
  - ref: oauth-strategy  # This ref is used above
    name: "OAuth Strategy"
```

### Parent-Child Relationships
API resources contain their child resources:

```yaml
apis:
  - ref: my-api
    name: "My API"
    versions:  # Child resources nested under parent
      - ref: my-api-v1
        name: "v1.0.0"
    publications:
      - ref: my-api-pub
        portal_id: my-portal  # References external portal
```

### External Service IDs
API implementations reference external services managed by decK:

```yaml
implementations:
  - ref: my-impl
    service:
      id: "12345678-1234-1234-1234-123456789012"  # UUID from decK
      control_plane_id: my-cp  # References control plane by ref
```

### Tool-Specific Metadata
Use `kongctl` field for tool-specific settings:

```yaml
control_planes:
  - ref: production-cp
    name: "Production"
    kongctl:
      protected: true  # Prevent accidental deletion
```

## Environment Variables

Sensitive values can use environment variables:

```yaml
openid_connect:
  client_secret: "${OAUTH_CLIENT_SECRET}"
```

## Usage

To validate these examples:

```bash
# Validate basic examples
kongctl plan --dir docs/examples/declarative/basic/portal-example

# Validate complex examples 
kongctl plan --dir docs/examples/declarative/complex/full-portal-setup-example

# Validate layout examples (multi-file)
kongctl plan --dir docs/examples/declarative/layouts/organized/

# Generate a plan from examples
kongctl plan --dir docs/examples/declarative/complex/api-lifecycle-example --output-file plan.json
```

## Best Practices

1. **Use meaningful refs**: Choose refs that clearly identify the resource
2. **Organize by team/service**: Group related resources together
3. **Version your configs**: Track changes in version control
4. **Validate before applying**: Always run `plan` before `sync`
5. **Use environment variables**: Never hardcode secrets
6. **Document your configs**: Add comments explaining complex relationships