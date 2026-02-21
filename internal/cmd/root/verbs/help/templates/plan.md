# kongctl plan - Extended Documentation

## Overview

The `kongctl plan` command generates an execution plan that shows what changes will be made to your Kong Konnect resources. This is a critical step in the declarative configuration workflow, allowing you to preview changes before applying them.

## Command Syntax

```
kongctl plan [flags]
```

## Flags

### Required Flags

- `-f, --file` (string): Path to configuration file or directory containing YAML configurations
  - Can be specified multiple times to load multiple files
  - Directories are processed non-recursively by default

### Optional Flags

- `-r, --recursive`: Process directories recursively (default: false)
- `--output-file` (string): Save the generated plan to a file
- `--log-level` (string): Set logging level: trace, debug, info, warn, error

## How Planning Works

1. **Load Configuration**: Reads all specified YAML files
2. **Fetch Current State**: Queries Konnect API for existing resources
3. **Compare States**: Identifies differences between desired and current state
4. **Generate Plan**: Creates ordered list of operations (CREATE, UPDATE, DELETE)
5. **Validate Dependencies**: Ensures resources are created in correct order

## Examples

### Basic Planning

```bash
# Generate plan for a single file
kongctl plan -f api-config.yaml

# Generate plan for multiple files
kongctl plan -f portals.yaml -f apis.yaml -f auth.yaml

# Generate plan from directory
kongctl plan -f ./configs/

# Generate plan recursively from directory tree
kongctl plan -f ./configs/ --recursive
```

### Working with Plan Files

```bash
# Save plan to file for review
kongctl plan -f config.yaml --output-file plan.json

# Use saved plan with apply
kongctl apply --plan plan.json
```

### Pipeline Examples

```bash
# Generate plan from stdin
cat config.yaml | kongctl plan -f -

# Generate plan and pipe to jq for processing
kongctl plan -f config.yaml | jq '.changes[]'

# Generate plan and review specific changes
kongctl plan -f config.yaml | jq '.changes[] | select(.operation == "CREATE")'
```

## Understanding Plan Output

### JSON Format (Default)

The `plan` command always outputs JSON. Example output:

```json
{
  "plan_id": "plan_20240115_120000_abc123",
  "timestamp": "2024-01-15T12:00:00Z",
  "changes": [
    {
      "operation": "CREATE",
      "resource_type": "portal",
      "resource_ref": "developer-portal",
      "fields": {
        "name": "developer-portal",
        "display_name": "Developer Portal"
      }
    }
  ],
  "summary": {
    "total": 2,
    "create": 1,
    "update": 1,
    "delete": 0
  }
}
```

## Resource Dependencies

The planner automatically handles dependencies:

1. **Portals** are created before API publications
2. **APIs** are created before their versions
3. **API versions** are created before publications
4. **Referenced resources** are validated

Example dependency chain:
```
portal → api → api_version → api_publication
```

## Namespace Support

When using namespaces, the plan shows namespace context:

```bash
# Plan shows namespace grouping
kongctl plan -f team-configs/

Planning changes for namespace: team-alpha
- CREATE api "frontend-api"

Planning changes for namespace: team-beta  
- CREATE api "backend-api"
```

## Common Workflows

### Development Workflow

```bash
# 1. Edit configuration
vim api-config.yaml

# 2. Generate and review plan
kongctl plan -f api-config.yaml

# 3. If changes look good, apply
kongctl apply -f api-config.yaml

# 4. Or save plan for team review
kongctl plan -f api-config.yaml --output-file proposed-changes.json
```

### CI/CD Workflow

```bash
# In CI pipeline
kongctl plan -f ./configs/ --output-file plan.json

# Review plan (automated checks)
./scripts/validate-plan.sh plan.json

# In CD pipeline
kongctl apply --plan plan.json --auto-approve
```

## Troubleshooting

### Common Issues

**No changes detected**
- Verify your configuration files are valid YAML
- Check that resource refs match existing resources
- Ensure you have proper authentication

**Validation errors**
- Check for circular dependencies
- Verify all referenced resources exist
- Ensure required fields are present

**Large plans**
- Break configuration into smaller files
- Use namespaces to isolate changes
- Consider phased rollouts

### Debug Mode

Enable debug logging to see detailed plan generation:

```bash
kongctl plan -f config.yaml --log-level debug
```

This shows:
- API requests and responses
- Resource comparison details
- Dependency resolution steps

## Best Practices

1. **Always review plans** before applying changes
2. **Save plans** for audit trails and rollback
3. **Use version control** for configuration files
4. **Test in non-production** environments first
5. **Break large changes** into smaller plans

## Related Commands

- `kongctl diff` - Show detailed differences
- `kongctl apply` - Apply changes (create/update only)
- `kongctl sync` - Full synchronization (includes deletes)

## See Also

- [Declarative Configuration Guide](https://github.com/Kong/kongctl/blob/main/docs/declarative-configuration.md)
- [YAML Tags Reference](https://github.com/Kong/kongctl/blob/main/docs/declarative-yaml-tags.md)
- [CI/CD Integration Guide](https://github.com/Kong/kongctl/blob/main/docs/declarative-ci-cd.md)
