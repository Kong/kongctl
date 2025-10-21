# kongctl apply - Extended Documentation

## Overview

The `kongctl apply` command applies declarative configuration changes to Kong Konnect. It performs CREATE and UPDATE operations only - it will never delete resources. This makes it safer for incremental updates and production use.

## Command Syntax

```
kongctl apply [flags]
```

## Flags

### Input Flags

- `-f, --file` (string): Path to configuration file or directory
  - Can be specified multiple times
  - Use `-` to read from stdin
- `--plan` (string): Path to a pre-generated plan file
- `-r, --recursive`: Process directories recursively

### Execution Flags

- `--dry-run`: Preview changes without applying them
- `--force`: Skip confirmation prompts
- `--auto-approve`: Automatically approve changes (alias for --force)

### Output Flags

- `--format` (string): Output format: json, yaml, or text (default: text)
- `--log-level` (string): Set logging level: trace, debug, info, warn, error

## Apply vs Sync

**Use `apply` when:**
- Adding new resources
- Updating existing resources
- You want to preserve unmanaged resources
- Making incremental changes
- Working in shared environments

**Use `sync` when:**
- You need full state reconciliation
- Removing obsolete resources
- Managing all resources in a namespace
- Enforcing exact configuration state

## Examples

### Basic Usage

```bash
# Apply single configuration file
kongctl apply -f api-config.yaml

# Apply multiple files
kongctl apply -f portals.yaml -f apis.yaml

# Apply from directory
kongctl apply -f ./configs/

# Apply with auto-approval (no prompts)
kongctl apply -f config.yaml --auto-approve
```

### Working with Plans

```bash
# Generate plan first
kongctl plan -f config.yaml -o my-plan.json

# Review plan
cat my-plan.json | jq '.summary'

# Apply the plan
kongctl apply --plan my-plan.json
```

### Dry Run Mode

```bash
# Preview what would be applied
kongctl apply -f config.yaml --dry-run

# Dry run with detailed output
kongctl apply -f config.yaml --dry-run --log-level debug
```

### Pipeline Integration

```bash
# Apply from stdin
cat config.yaml | kongctl apply -f -

# Apply from git
git show HEAD:configs/api.yaml | kongctl apply -f -

# Apply with environment substitution
envsubst < config.template.yaml | kongctl apply -f -
```

## Understanding Apply Output

### Standard Output

```
Applying configuration...

✓ CREATE portal "developer-portal"
✓ UPDATE api "users-api"
✓ CREATE api_version "users-api-v2"
✓ CREATE api_publication "users-public"

Summary:
- Created: 3 resources
- Updated: 1 resource
- Failed: 0 resources

Apply completed successfully.
```

### JSON Output

```json
{
  "timestamp": "2024-01-15T12:00:00Z",
  "results": [
    {
      "operation": "CREATE",
      "resource_type": "portal",
      "resource_ref": "developer-portal",
      "status": "success",
      "resource_id": "12345678-1234-1234-1234-123456789012"
    }
  ],
  "summary": {
    "created": 3,
    "updated": 1,
    "failed": 0
  }
}
```

## Protected Resources

Resources marked as protected will block apply operations:

```yaml
apis:
  - ref: production-api
    name: "Production API"
    kongctl:
      protected: true  # This prevents modifications
```

Attempting to modify protected resources results in:

```
Error: Cannot modify protected resource "production-api"
- Resource is marked as protected in current state
- Remove protection before making changes
```

## Partial Failures

Apply continues executing even if individual operations fail:

```
Applying configuration...

✓ CREATE portal "developer-portal"
✗ CREATE api "invalid-api" - Error: Invalid configuration
✓ UPDATE api "users-api"

Summary:
- Created: 1 resource
- Updated: 1 resource
- Failed: 1 resource

Apply completed with errors. See above for details.
```

## Common Workflows

### Progressive Rollout

```bash
# Stage 1: Apply portals only
kongctl apply -f portals.yaml

# Stage 2: Apply APIs
kongctl apply -f apis.yaml

# Stage 3: Apply publications
kongctl apply -f publications.yaml
```

### Multi-Environment Pattern

```bash
# Development
kongctl apply -f base/ -f overlays/dev/ --profile dev

# Staging
kongctl apply -f base/ -f overlays/staging/ --profile staging

# Production (with plan review)
kongctl plan -f base/ -f overlays/prod/ -o prod-plan.json
# ... review plan ...
kongctl apply --plan prod-plan.json --profile prod
```

### GitOps Workflow

```bash
# In CI/CD pipeline
#!/bin/bash
set -e

# Generate plan
kongctl plan -f ./configs/ -o plan.json

# Validate plan meets policies
./scripts/validate-plan.sh plan.json

# Apply if validation passes
if [ "$BRANCH" = "main" ]; then
  kongctl apply --plan plan.json --auto-approve
fi
```

## Error Handling

### Connection Errors

```bash
# Retry with exponential backoff
for i in {1..3}; do
  kongctl apply -f config.yaml && break
  echo "Retry $i/3 failed, waiting..."
  sleep $((2**i))
done
```

### Validation Errors

Enable debug mode to see detailed validation errors:

```bash
kongctl apply -f config.yaml --log-level debug
```

Common validation issues:
- Missing required fields
- Invalid references to non-existent resources
- Circular dependencies
- Schema violations

## Best Practices

1. **Always plan before apply** in production
2. **Use --dry-run** for safety
3. **Apply in stages** for large changes
4. **Monitor apply progress** in production
5. **Keep plans for rollback** scenarios
6. **Use protected resources** for critical APIs

## Performance Considerations

- Operations are executed in parallel where possible
- Large configurations may take time
- Use `--log-level debug` to monitor progress
- Network latency affects execution time

## Troubleshooting

### "Resource not found" errors
- Ensure dependencies are applied first
- Check resource references are correct
- Verify resources weren't deleted externally

### "Conflict" errors
- Resource may have been modified externally
- Use `kongctl plan` to see current state
- Consider using `sync` for full reconciliation

### Authentication failures
- Verify credentials with `kongctl get portals`
- Check token expiration
- Ensure correct profile is active

## Related Commands

- `kongctl plan` - Preview changes before applying
- `kongctl sync` - Full state synchronization
- `kongctl diff` - Show detailed differences

## See Also

- [Declarative Configuration Guide](https://github.com/Kong/kongctl/blob/main/docs/declarative-configuration.md)
- [CI/CD Integration Guide](https://github.com/Kong/kongctl/blob/main/docs/declarative-ci-cd.md)
- [Protected Resources Example](https://github.com/Kong/kongctl/tree/main/docs/examples/declarative/protected)
