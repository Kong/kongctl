# External Resources Example

This example demonstrates the use of external resources and `!ref` tags in kongctl, 
showcasing how different teams can manage their own APIs while referencing shared 
infrastructure managed by other teams.

## Overview

The external resources feature allows you to reference resources (like portals) that are 
managed by other teams or external tools, without having to duplicate their definitions 
in your configuration files.

## Example Structure

```
external/
├── platform/          # Platform team manages the shared portal
│   ├── portal.yaml     # Portal definition with pages and customization
│   ├── api.yaml        # Platform API published to their portal
│   ├── pages/          # Portal content pages
│   ├── snippets/       # Reusable portal content snippets
│   ├── specs/          # OpenAPI specifications
│   └── docs/           # API documentation
├── team-a/             # Team A manages customer analytics
│   ├── api.yaml        # API + external portal reference
│   ├── specs/          # API specifications
│   └── docs/           # API documentation
├── team-b/             # Team B manages payment processing
│   ├── api.yaml        # API + external portal reference
│   ├── specs/          # API specifications
│   └── docs/           # API documentation
└── README.md           # This file
```

## Key Concepts

### External Resource Definition

Teams that reference external resources define them with the `_external` block:

```yaml
# In team-a/api.yaml and team-b/api.yaml
portals:
  - ref: shared-developer-portal
    _external:
      selector:
        matchFields:
          name: "Shared Developer Portal"
```

This tells kongctl:
1. This portal is managed externally (by another team)
2. Find it by searching for a portal with name "Shared Developer Portal"
3. Don't try to create/update/delete this portal during planning

### Reference Usage with !ref Tags

Once defined as external, teams can reference the portal using `!ref` tags:

```yaml
# In API publications
publications:
  - ref: customer-analytics-to-shared-portal
    portal_id: !ref shared-developer-portal#id
    visibility: public
```

The `!ref` tag resolves to the actual Konnect ID of the external portal at runtime.

## Team Responsibilities

### Platform Team (`platform/`)
- **Owns**: The shared developer portal infrastructure
- **Manages**: Portal configuration, branding, pages, and navigation
- **Publishes**: Their own platform APIs to the portal
- **Provides**: Stable portal name for other teams to reference

### Team A (`team-a/`)
- **Owns**: Customer Analytics API
- **References**: Platform team's portal as external resource
- **Publishes**: Their API to the shared portal
- **Manages**: Their own API specs and documentation

### Team B (`team-b/`)
- **Owns**: Payment Processing API  
- **References**: Platform team's portal as external resource
- **Publishes**: Their API to the shared portal
- **Manages**: Their own API specs and documentation

## Deployment Scenarios

### Scenario 1: Platform Team Deploys First

```bash
# Platform team deploys their portal and API
cd platform/
kongctl apply portal.yaml api.yaml

# Team A can now reference the external portal
cd ../team-a/
kongctl apply api.yaml

# Team B can also reference the same portal
cd ../team-b/
kongctl apply api.yaml
```

### Scenario 2: Teams Deploy Independently

```bash
# Team A tries to deploy first, but portal doesn't exist
cd team-a/
kongctl apply api.yaml
# Result: Error - external portal "Shared Developer Portal" not found

# Platform team deploys
cd ../platform/
kongctl apply portal.yaml api.yaml

# Now Team A can deploy successfully
cd ../team-a/
kongctl apply api.yaml
# Result: Success - portal found and API published

# Team B can also deploy
cd ../team-b/  
kongctl apply api.yaml
# Result: Success - references same external portal
```

## Benefits of External Resources

### 1. **Separation of Concerns**
- Platform team manages portal infrastructure
- API teams focus on their specific APIs
- Clear ownership boundaries

### 2. **Reduced Duplication**
- No need to copy portal definitions across teams
- Single source of truth for shared resources
- Easier maintenance and updates

### 3. **Flexible References**
- Reference by name (using selector)
- Reference by direct ID (if known)
- Runtime resolution allows forward references

### 4. **Team Autonomy**
- Teams can deploy independently
- API teams don't need portal management permissions
- Decoupled deployment workflows

## Best Practices

### For Platform Teams

1. **Stable Naming**: Use consistent, descriptive names for shared resources
2. **Documentation**: Clearly document what resources are available for external reference
3. **Backwards Compatibility**: Avoid breaking changes to resource names/structure
4. **Access Control**: Ensure API teams can read but not modify shared resources

### For API Teams

1. **Descriptive References**: Use clear `ref` names that indicate external resources
2. **Error Handling**: Handle cases where external resources don't exist yet
3. **Documentation**: Document external dependencies in your deployment procedures
4. **Testing**: Test deployment scenarios with and without external resources

### General

1. **Coordination**: Establish clear communication between teams about shared resources
2. **Validation**: Use `kongctl plan` to validate configurations before applying
3. **Monitoring**: Monitor for external resource availability in deployment pipelines

## Advanced Usage

### Alternative Selector Methods

Reference by Konnect ID if known:

```yaml
portals:
  - ref: shared-developer-portal
    _external:
      id: "portal-uuid-12345"
```

### Multiple External Resources

```yaml
# Reference multiple external resources
portals:
  - ref: shared-developer-portal
    _external:
      selector:
        matchFields:
          name: "Shared Developer Portal"

  - ref: admin-portal  
    _external:
      selector:
        matchFields:
          name: "Admin Portal"

# Use in publications
publications:
  - ref: api-to-public-portal
    portal_id: !ref shared-developer-portal#id
    visibility: public
    
  - ref: api-to-admin-portal
    portal_id: !ref admin-portal#id
    visibility: private
```

## Testing the Example

1. **Set up environment**: Ensure you have kongctl configured with appropriate Konnect credentials

2. **Deploy platform resources**:
   ```bash
   cd platform/
   kongctl apply portal.yaml api.yaml
   ```

3. **Deploy team APIs**:
   ```bash
   cd ../team-a/
   kongctl apply api.yaml
   
   cd ../team-b/
   kongctl apply api.yaml
   ```

4. **Verify in Konnect**: Check that all APIs are published to the same shared portal

5. **Test references**: Verify that the `!ref` tags correctly resolve to portal IDs

## Troubleshooting

### Common Issues

**Error: External portal not found**
- Ensure the platform team has deployed the portal first
- Verify the name in the `matchFields` selector matches exactly
- Check Konnect permissions for reading portals

**Error: Portal name is ambiguous**
- Multiple portals have the same name
- Use more specific selectors or direct ID references
- Consider using labels for better matching

**Error: Permission denied**
- API team lacks permission to publish to the portal
- Platform team needs to grant appropriate access
- Check Konnect RBAC settings

### Validation Commands

```bash
# Check if external resources can be found
kongctl plan --dry-run api.yaml

# Validate configuration syntax
kongctl validate api.yaml

# Debug reference resolution
kongctl apply --log-level debug api.yaml
```

## Related Documentation

- [YAML !ref Tags Guide](../../../planning/refactoring/external-resources-ref-tag-implementation.md)
- [Portal Configuration Guide](../portal/README.md)  
- [Multi-team Workflows](../namespace/README.md)

---

This example demonstrates real-world usage patterns for external resources and cross-team 
collaboration in kongctl configurations.
