# kongctl sync - Extended Documentation

## Overview

The `kongctl sync` command performs full state synchronization between your declarative configuration and Kong Konnect. Unlike `apply`, sync will CREATE, UPDATE, and DELETE resources to ensure the target state exactly matches your configuration.

⚠️ **WARNING**: Sync can delete resources. Always review changes before executing.

## Command Syntax

```
kongctl sync [flags]
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

## How Sync Works

1. **Identifies managed resources** using `KONGCTL-managed` labels
2. **Compares** desired state (your config) with current state
3. **Plans operations**:
   - CREATE for new resources
   - UPDATE for changed resources
   - DELETE for resources not in config
4. **Executes** all operations in dependency order

## Managed Resources

Only resources with the `KONGCTL-managed: true` label are considered for deletion:

```yaml
labels:
  KONGCTL-managed: "true"  # Resource is managed by kongctl
  KONGCTL-namespace: "production"  # Resource namespace
```

Resources without this label are ignored during sync.

## Examples

### Basic Synchronization

```bash
# Sync single configuration file
kongctl sync -f api-config.yaml

# Sync with auto-approval (dangerous in production!)
kongctl sync -f config.yaml --auto-approve

# Dry run to see what would be deleted
kongctl sync -f config.yaml --dry-run
```

### Namespace-Based Sync

```bash
# Sync only affects resources in specified namespaces
kongctl sync -f team-alpha-config.yaml
# Only syncs namespace: team-alpha

# Multi-namespace sync
kongctl sync -f team-configs/
# Syncs each namespace independently
```

### Empty Configuration Sync

```bash
# Delete all managed resources in namespace
echo 'apis: []' | kongctl sync -f -

# Safer: dry run first
echo 'apis: []' | kongctl sync -f - --dry-run
```

## Confirmation Prompts

Sync shows detailed deletion warnings:

```
Planning sync operation...

The following resources will be DELETED:
- api "deprecated-api" (id: 12345678-1234-1234-1234-123456789012)
- api_version "deprecated-api-v1" (id: 87654321-4321-4321-4321-210987654321)
- portal "old-portal" (id: 11111111-2222-3333-4444-555555555555)

Summary:
- Create: 2 resources
- Update: 3 resources
- Delete: 3 resources ⚠️

Do you want to proceed? (yes/no):
```

## Protected Resources

Protected resources block sync operations:

```yaml
apis:
  - ref: critical-api
    kongctl:
      protected: true  # Prevents deletion
```

Sync output with protected resources:

```
Error: Cannot delete protected resources:
- api "critical-api" is marked as protected
- Remove protection or exclude from sync
```

## Common Workflows

### Full Environment Sync

```bash
# Development environment - full sync
kongctl sync -f environments/dev/ --recursive --auto-approve

# Production - careful sync with review
kongctl plan -f environments/prod/ --recursive -o prod-plan.json
# ... review plan carefully ...
kongctl sync --plan prod-plan.json
```

### Namespace Isolation

```bash
# Each team syncs their namespace only
# Team Alpha
kongctl sync -f team-alpha/ --profile alpha

# Team Beta  
kongctl sync -f team-beta/ --profile beta

# Platform team manages shared resources
kongctl sync -f platform/ --profile platform
```

### Staged Synchronization

```bash
# Stage 1: Sync portals (less risky)
kongctl sync -f portals.yaml

# Stage 2: Sync APIs (review deletions)
kongctl sync -f apis.yaml --dry-run
# ... review ...
kongctl sync -f apis.yaml

# Stage 3: Sync everything
kongctl sync -f ./ --recursive
```

### Disaster Recovery

```bash
# Restore from backup
kongctl sync -f backup/2024-01-15/ --recursive --force

# Clone environment
kongctl dump > prod-backup.yaml
kongctl sync -f prod-backup.yaml --profile staging
```

## Understanding Sync Output

### Deletion Warnings

```
⚠️  WARNING: Sync will DELETE the following resources:

API Resources:
- "legacy-api-v1" (Users: 1,234 daily)
- "deprecated-service" (Last updated: 2023-12-01)

Portal Resources:
- "old-developer-portal" (Published APIs: 5)

This operation cannot be undone.
```

### Execution Output

```
Executing sync...

Phase 1: Deletions
✓ DELETE api_publication "old-pub"
✓ DELETE api_version "legacy-v1"
✓ DELETE api "legacy-api"

Phase 2: Creates
✓ CREATE api "new-api"
✓ CREATE api_version "new-api-v1"

Phase 3: Updates
✓ UPDATE portal "main-portal"

Summary:
- Deleted: 3 resources
- Created: 2 resources
- Updated: 1 resource
```

## Safety Measures

### 1. Always Dry Run First

```bash
# See what would be deleted
kongctl sync -f config.yaml --dry-run | grep DELETE
```

### 2. Use Protected Resources

```yaml
# Mark production resources as protected
apis:
  - ref: production-api
    kongctl:
      protected: true
      namespace: production
```

### 3. Backup Before Sync

```bash
# Dump current state
kongctl dump --namespace production > backup.yaml

# Perform sync
kongctl sync -f new-config.yaml

# Restore if needed
kongctl sync -f backup.yaml
```

### 4. Use Namespaces

```bash
# Limit sync scope with namespaces
kongctl sync -f team-config.yaml
# Only affects resources in that team's namespace
```

## Troubleshooting

### "Resource not found" during deletion
- Resource was already deleted externally
- Sync continues with remaining operations

### "Cannot delete resource with dependencies"
- Child resources must be deleted first
- Sync handles this automatically

### Partial sync failures
```bash
# Enable debug logging
kongctl sync -f config.yaml --log-level debug

# Shows:
# - Dependency resolution
# - API requests/responses
# - Detailed error messages
```

### Recovering from failed sync
```bash
# Re-run sync - it's idempotent
kongctl sync -f config.yaml

# Or restore from backup
kongctl sync -f last-known-good.yaml
```

## Best Practices

1. **Always use namespaces** to limit sync scope
2. **Dry run before production sync**
3. **Protect critical resources**
4. **Maintain backups** before major syncs
5. **Monitor sync operations** in production
6. **Use version control** for configurations
7. **Test in non-production** first

## Performance Notes

- Deletions are processed in reverse dependency order
- Operations are parallelized where possible
- Large syncs may take several minutes
- Use `--log-level info` to monitor progress

## Related Commands

- `kongctl apply` - Safer alternative (no deletions)
- `kongctl plan` - Preview all changes
- `kongctl dump` - Backup current state

## See Also

- [Namespace Management](https://docs.konghq.com/kongctl/namespaces)
- [Protected Resources](https://docs.konghq.com/kongctl/protected-resources)
- [Disaster Recovery](https://docs.konghq.com/kongctl/disaster-recovery)