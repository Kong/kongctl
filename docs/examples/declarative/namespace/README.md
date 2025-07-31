# Namespace Examples

This directory contains examples demonstrating namespace-based resource 
management in kongctl. Namespaces allow multiple teams to safely manage their 
own resources within a shared Kong Konnect organization.

## What are Namespaces?

Namespaces provide resource isolation between different teams or environments. 
When you apply or sync configurations, kongctl only manages resources within 
the namespaces defined in your configuration files, leaving resources in other 
namespaces untouched.

## Key Concepts

1. **Namespace Label**: Resources are tagged with a `KONGCTL-namespace` label
2. **Default Namespace**: Resources without explicit namespace use "default"
3. **Parent Resources Only**: Only parent resources (APIs, Portals, Auth 
   Strategies) can have namespaces
4. **Child Resource Inheritance**: Child resources (versions, publications, 
   etc.) inherit their parent's namespace
5. **Namespace Isolation**: Operations only affect resources in specified 
   namespaces

## Examples Overview

### 1. Single Team (`single-team/`)
Basic examples showing how a single team can use namespaces to organize their 
resources. This is the simplest use case.

**Files:**
- `api.yaml` - API with explicit namespace
- `portal.yaml` - Portal with namespace
- `auth-strategy.yaml` - Auth strategy with namespace

### 2. Multi-Team (`multi-team/`)
Demonstrates how multiple teams can manage resources in the same Konnect 
organization without interfering with each other.

**Files:**
- `team-alpha.yaml` - Team Alpha's resources (namespace: team-alpha)
- `team-beta.yaml` - Team Beta's resources (namespace: team-beta)
- `shared.yaml` - Shared resources (namespace: default)

### 3. With Defaults (`with-defaults/`)
Shows how to use the `_defaults` section to set a default namespace for all 
resources in a file, reducing repetition.

**Files:**
- `main.yaml` - File with `_defaults` section
- `additional.yaml` - Resources inheriting defaults

### 4. Protected Resources (`protected-resources/`)
Demonstrates combining namespaces with resource protection for production 
environments.

**Files:**
- `production.yaml` - Production resources with namespace and protection

## Namespace Naming Conventions

Recommended patterns for namespace names:
- Team-based: `team-alpha`, `payments-team`, `platform-team`
- Environment-based: `dev`, `staging`, `prod`
- Project-based: `project-phoenix`, `mobile-app`
- Department-based: `engineering`, `marketing`, `support`

**Rules:**
- Must start and end with alphanumeric characters
- Can contain hyphens in the middle
- Maximum 63 characters
- Lowercase only

## Best Practices

### 1. One Namespace Per Team
Each team should use their own namespace to ensure clear ownership and prevent 
accidental modifications.

```yaml
# team-payments.yaml
apis:
  - ref: payment-api
    name: "Payment Processing API"
    kongctl:
      namespace: payments-team
```

### 2. Use File-Level Defaults
When all resources in a file belong to the same namespace, use `_defaults`:

```yaml
_defaults:
  kongctl:
    namespace: platform-team

apis:
  - ref: user-api
    name: "User API"
    # Inherits namespace: platform-team
```

### 3. Protect Production Resources
Combine namespaces with protection for critical resources:

```yaml
apis:
  - ref: billing-api
    name: "Billing API"
    kongctl:
      namespace: prod
      protected: true
```

### 4. Document Namespace Ownership
Maintain clear documentation about which team owns which namespace:

```yaml
# This file is owned by the Platform Team
# Namespace: platform-team
# Contact: platform@example.com
```

## Common Patterns

### Pattern 1: Team-Based Organization
```
configs/
├── team-alpha/
│   ├── apis.yaml         # namespace: team-alpha
│   └── portals.yaml      # namespace: team-alpha
├── team-beta/
│   ├── apis.yaml         # namespace: team-beta
│   └── portals.yaml      # namespace: team-beta
└── shared/
    └── auth.yaml         # namespace: default
```

### Pattern 2: Environment-Based Organization
```
configs/
├── dev/
│   └── all.yaml          # namespace: dev
├── staging/
│   └── all.yaml          # namespace: staging
└── prod/
    └── all.yaml          # namespace: prod
```

### Pattern 3: Mixed Organization
```
configs/
├── team-alpha/
│   ├── dev.yaml          # namespace: alpha-dev
│   ├── staging.yaml      # namespace: alpha-staging
│   └── prod.yaml         # namespace: alpha-prod
└── team-beta/
    ├── dev.yaml          # namespace: beta-dev
    └── prod.yaml         # namespace: beta-prod
```

## Limitations

1. **Label Limit**: Kong Konnect has a 5-label limit per resource. Namespace 
   uses one label slot.
2. **No Server-Side Enforcement**: Namespaces are enforced by kongctl, not by 
   Konnect itself.
3. **No Hierarchical Namespaces**: Namespaces are flat, not nested.
4. **No Cross-Namespace References**: Resources can only reference other 
   resources in any namespace (no isolation for references).

## Migration Guide

### Moving from No Namespaces to Namespaces

1. **Audit Current Resources**: List all managed resources
2. **Plan Namespace Structure**: Decide on naming convention
3. **Add Namespaces Gradually**: Start with new resources
4. **Migrate Existing Resources**: Add namespace to existing configs
5. **Verify Isolation**: Test that teams can't affect each other's resources

### Example Migration

Before:
```yaml
apis:
  - ref: user-api
    name: "User API"
```

After:
```yaml
apis:
  - ref: user-api
    name: "User API"
    kongctl:
      namespace: platform-team
```

## Troubleshooting

### Resources Not Found
If resources aren't being managed, check:
1. Namespace is spelled correctly
2. Resources have the expected namespace label in Konnect
3. No typos in namespace values

### Accidental Deletion
To prevent accidental deletion during sync:
1. Always use explicit namespaces (avoid relying on "default")
2. Use `--dry-run` to preview changes
3. Protect critical resources with `protected: true`

### Namespace Conflicts
If teams accidentally use the same namespace:
1. Audit resources in the namespace
2. Coordinate namespace renaming
3. Update all configurations
4. Re-apply with correct namespaces

## Command Examples

### View Resources by Namespace
```bash
# Plan shows namespace grouping
kongctl plan -f team-configs/

# Apply affects only namespaces in configs
kongctl apply -f team-configs/

# Sync removes unmanaged resources in specified namespaces only
kongctl sync -f team-configs/
```

### Dry Run for Safety
```bash
# Always preview sync operations
kongctl sync -f team-configs/ --dry-run
```

## See Also

- [Configuration Guide](../../../declarative/Configuration-Guide.md) - Complete 
  namespace documentation
- [Basic Examples](../basic/) - Simple configurations without namespaces
- [Troubleshooting Guide](../../../declarative/Troubleshooting-Guide.md) - 
  Common issues and solutions
