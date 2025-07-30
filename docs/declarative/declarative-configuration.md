# Declarative Configuration Guide

This guide provides comprehensive documentation for using kongctl's declarative 
configuration management features with Kong Konnect.

## Table of Contents

- [Overview](#overview)
- [Core Concepts](#core-concepts)
- [Resource Types](#resource-types)
- [Configuration Format](#configuration-format)
- [Workflow Overview](#workflow-overview)
- [Label Management](#label-management)
- [Namespace Management](#namespace-management)
- [Best Practices](#best-practices)
- [Migration Guide](#migration-guide)

## Overview

Declarative configuration allows you to define your Kong Konnect resources as 
code using YAML files. This approach provides:

- **Version Control**: Track changes to your API infrastructure
- **Reproducibility**: Deploy identical configurations across environments
- **Automation**: Integrate with CI/CD pipelines
- **Collaboration**: Review changes through pull requests
- **Disaster Recovery**: Quickly restore from configuration files

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
    name: developer-portal
    display_name: "Dev Portal"
    authentication_enabled: true
```

### Key Principles

1. **Desired State**: Define what you want, not how to get there
2. **Idempotency**: Apply configurations multiple times safely
3. **Reconciliation**: System calculates and applies necessary changes
4. **Validation**: Changes are validated before execution

## Resource Types

### Supported Resources

| Resource | Description | Parent | Children |
|----------|-------------|--------|----------|
| Portal | Developer portal for API documentation | - | API Publications |
| API | API definition and metadata | - | Versions, Publications |
| API Version | Specific version of an API | API | - |
| API Publication | Publishing an API to a portal | API | - |
| Auth Strategy | Authentication configuration | - | - |

### Resource Relationships

```
Portal
  └── API Publication → API
                         ├── API Version
                         └── API Publication
```

## Configuration Format

### Basic Structure

```yaml
# Optional defaults section
_defaults:
  kongctl:
    namespace: production
    protected: false

# Resource definitions
portals:
  - ref: main-portal
    name: main-portal
    display_name: "Main Developer Portal"

apis:
  - ref: users-api
    name: "Users API"
    description: "User management API"
    
api_publications:
  - ref: users-publication
    api: users-api
    portal: main-portal
```

### Resource References

Resources reference each other using the `ref` field:

```yaml
apis:
  - ref: my-api  # Define reference
    name: "My API"

api_publications:
  - ref: my-publication
    api: my-api  # Reference the API
    portal: main-portal  # Reference the portal
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

## Workflow Overview

### Standard Workflow

1. **Create Configuration**
   ```yaml
   # api-config.yaml
   apis:
     - ref: payment-api
       name: "Payment API"
       description: "Process payments"
   ```

2. **Generate Plan**
   ```bash
   kongctl plan -f api-config.yaml
   ```

3. **Review Changes**
   ```bash
   kongctl diff -f api-config.yaml
   ```

4. **Apply Changes**
   ```bash
   kongctl apply -f api-config.yaml
   ```

### Advanced Workflow with Plans

```bash
# Generate plan for review
kongctl plan -f ./configs/ -o plan.json

# Share plan with team
# ... review process ...

# Apply approved plan
kongctl apply --plan plan.json
```

## Label Management

### System Labels

Kongctl uses labels to track resource state:

| Label | Purpose | Values |
|-------|---------|--------|
| `KONGCTL-managed` | Identifies managed resources | `true` |
| `KONGCTL-config-hash` | Tracks configuration changes | Hash string |
| `KONGCTL-namespace` | Resource namespace | Namespace name |
| `KONGCTL-protected` | Prevents modifications | `true` |

### Protection Mechanism

```yaml
apis:
  - ref: production-api
    name: "Production API"
    kongctl:
      protected: true  # Cannot be modified or deleted
```

Protected resources:
- Block UPDATE operations in apply/sync
- Block DELETE operations in sync
- Require explicit unprotection before changes

## Namespace Management

### Overview

Namespaces provide isolation between teams or environments:

```yaml
apis:
  - ref: frontend-api
    name: "Frontend API"
    kongctl:
      namespace: team-alpha
```

### Namespace Behavior

- Only parent resources can have namespaces
- Child resources inherit parent's namespace
- Operations only affect specified namespaces
- Default namespace is "default"

### File-Level Defaults

```yaml
_defaults:
  kongctl:
    namespace: platform-team

apis:
  - ref: api1  # Uses platform-team namespace
    name: "API 1"
    
  - ref: api2
    name: "API 2"
    kongctl:
      namespace: special-team  # Override default
```

## Best Practices

### 1. File Organization

```
configs/
├── _defaults.yaml      # Shared defaults
├── portals/           # Portal definitions
│   └── main.yaml
├── apis/              # API definitions
│   ├── users.yaml
│   └── payments.yaml
└── auth/              # Auth strategies
    └── oauth.yaml
```

### 2. Environment Management

```
environments/
├── base/              # Shared configuration
│   └── apis.yaml
├── dev/              # Development overrides
│   └── portals.yaml
├── staging/          # Staging overrides
│   └── portals.yaml
└── prod/             # Production overrides
    └── portals.yaml
```

### 3. Change Management

1. **Always use version control**
2. **Review plans before applying**
3. **Test in non-production first**
4. **Use protected resources for critical APIs**
5. **Implement gradual rollouts**

### 4. Security

1. **Never commit secrets** - Use environment variables
2. **Restrict file access** - Configuration may contain sensitive data
3. **Audit changes** - Keep plan files for audit trail
4. **Use namespaces** - Isolate team resources

## Migration Guide

### From Imperative to Declarative

#### Step 1: Export Current State

```bash
kongctl dump > current-state.yaml
```

#### Step 2: Organize Configuration

Split the dump into logical files:
```bash
# Extract portals
yq e '.portals' current-state.yaml > portals.yaml

# Extract APIs
yq e '.apis' current-state.yaml > apis.yaml
```

#### Step 3: Add Management Metadata

```yaml
apis:
  - ref: existing-api
    name: "Existing API"
    kongctl:
      namespace: production
      protected: true  # Protect during migration
```

#### Step 4: Validate Migration

```bash
# Dry run to ensure no changes
kongctl sync -f ./configs/ --dry-run

# Should show no changes if migration is correct
```

#### Step 5: Apply Configuration

```bash
# First apply adds labels
kongctl apply -f ./configs/

# Future changes now tracked
```

### Gradual Migration

For large deployments, migrate incrementally:

1. **Phase 1**: Non-critical resources
2. **Phase 2**: Development/staging APIs  
3. **Phase 3**: Production APIs (protected)
4. **Phase 4**: Remove protection, full management

## Command Reference

### Planning Commands

- `kongctl plan` - Generate execution plan
- `kongctl diff` - Show differences

### Execution Commands

- `kongctl apply` - Apply changes (create/update only)
- `kongctl sync` - Full synchronization (includes delete)

### Utility Commands

- `kongctl dump` - Export current state
- `kongctl help <command>` - Extended documentation

## Troubleshooting

See [Troubleshooting Guide](troubleshooting.md) for common issues and solutions.

## Further Reading

- [YAML Tags Reference](declarative/YAML-Tags-Reference.md)
- [CI/CD Integration](examples/ci-cd-integration.md)
- [Example Configurations](examples/declarative/)