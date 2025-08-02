# kongctl diff - Extended Documentation

## Overview

The `kongctl diff` command displays detailed differences between your desired configuration and the current state in Kong Konnect. It provides a clear view of what would change without making any modifications.

## Command Syntax

```
kongctl diff [flags]
```

## Flags

### Input Flags

- `-f, --file` (string): Path to configuration file or directory
  - Can be specified multiple times
  - Use `-` to read from stdin
- `--plan` (string): Use a pre-generated plan file
- `-r, --recursive`: Process directories recursively

### Output Flags

- `--format` (string): Output format: text, json, or yaml (default: text)
- `--log-level` (string): Set logging level: trace, debug, info, warn, error

## Output Formats

### Text Format (Default)

Shows human-readable differences with color coding:
- 🟢 Green: Additions (CREATE)
- 🟡 Yellow: Modifications (UPDATE)
- 🔴 Red: Deletions (DELETE - sync mode only)

```diff
Portal "developer-portal":
  + display_name: "Developer Portal"
  + authentication_enabled: true

API "users-api":
  ~ description: "Basic user API" → "User management and authentication API"
  ~ version: "v1.0.0" → "v2.0.0"
  + labels:
    + team: "platform"
    + environment: "production"

- API "legacy-api" (would be deleted in sync mode)
```

### JSON Format

Structured output for programmatic processing:

```json
{
  "timestamp": "2024-01-15T12:00:00Z",
  "mode": "apply",
  "changes": [
    {
      "operation": "CREATE",
      "resource_type": "portal",
      "resource_ref": "developer-portal",
      "changes": {
        "display_name": {
          "old": null,
          "new": "Developer Portal"
        }
      }
    },
    {
      "operation": "UPDATE", 
      "resource_type": "api",
      "resource_ref": "users-api",
      "changes": {
        "description": {
          "old": "Basic user API",
          "new": "User management and authentication API"
        }
      }
    }
  ]
}
```

### YAML Format

Similar to JSON but in YAML syntax:

```yaml
timestamp: "2024-01-15T12:00:00Z"
mode: apply
changes:
  - operation: CREATE
    resource_type: portal
    resource_ref: developer-portal
    changes:
      display_name:
        old: null
        new: Developer Portal
```

## Examples

### Basic Diff Operations

```bash
# Show differences for a configuration
kongctl diff -f api-config.yaml

# Diff multiple files
kongctl diff -f portals.yaml -f apis.yaml

# Diff from directory
kongctl diff -f ./configs/ --recursive

# Diff from stdin
cat config.yaml | kongctl diff -f -
```

### Working with Plans

```bash
# Generate plan and show diff
kongctl plan -f config.yaml -o plan.json
kongctl diff --plan plan.json

# One-liner
kongctl plan -f config.yaml -o - | kongctl diff --plan -
```

### Output Processing

```bash
# Get JSON output for scripts
kongctl diff -f config.yaml --format json

# Filter specific operations
kongctl diff -f config.yaml --format json | jq '.changes[] | select(.operation == "UPDATE")'

# Count changes by type
kongctl diff -f config.yaml --format json | jq '.changes | group_by(.operation) | map({operation: .[0].operation, count: length})'
```

### CI/CD Integration

```bash
#!/bin/bash
# Show diff in pull request

echo "## Configuration Changes" >> pr-comment.md
echo '```diff' >> pr-comment.md
kongctl diff -f ./configs/ >> pr-comment.md
echo '```' >> pr-comment.md

# Post to GitHub PR
gh pr comment --body-file pr-comment.md
```

## Understanding Diff Output

### Field-Level Changes

Shows exactly what fields are changing:

```
API "payment-api":
  ~ description: "Payment processing" → "Payment processing with fraud detection"
  ~ labels:
    ~ version: "1.0.0" → "1.1.0"
    + compliance: "PCI-DSS"
    - deprecated: "false"
```

### Nested Resource Changes

Shows changes in nested resources:

```
API "users-api":
  ~ version: "v1" → "v2"
  
  Versions:
    + "v2.0.0":
      + spec: { ... OpenAPI spec ... }
      + gateway_service:
        + control_plane_id: "cp-123"
        + id: "svc-456"
```

### Array Changes

Shows modifications to arrays:

```
Portal "developer-portal":
  ~ auto_approve_applications: false → true
  ~ approved_domains:
    + "example.com"
    + "api.example.com"
    - "old.example.com"
```

## Diff Modes

### Apply Mode (Default)

Shows only CREATE and UPDATE operations:

```bash
# Default behavior
kongctl diff -f config.yaml

# Explicit apply mode
kongctl plan -f config.yaml
kongctl diff --plan -
```

### Sync Mode

Shows CREATE, UPDATE, and DELETE operations:

```bash
# Generate sync plan
kongctl plan -f config.yaml --sync
kongctl diff --plan -

# Shows deletions
- API "deprecated-api"
- Portal "old-portal"
```

## Common Use Cases

### Pre-deployment Review

```bash
# In CI pipeline
kongctl diff -f ./configs/ > diff-output.txt

# Check for breaking changes
if grep -q "DELETE api" diff-output.txt; then
  echo "WARNING: APIs will be deleted!"
  exit 1
fi
```

### Change Validation

```bash
# Ensure no protected resources are modified
kongctl diff -f config.yaml --format json | \
  jq '.changes[] | select(.resource_ref == "production-api")' && \
  echo "ERROR: Attempting to modify protected resource" && exit 1
```

### Drift Detection

```bash
# Compare current state with expected
kongctl dump > current-state.yaml
kongctl diff -f expected-state.yaml

# Show only unexpected changes
diff <(kongctl dump) expected-state.yaml
```

## Advanced Examples

### Namespace-Aware Diff

```bash
# Shows namespace context
kongctl diff -f team-configs/

Namespace: team-alpha
  API "frontend-api":
    + description: "Frontend API for web app"

Namespace: team-beta
  API "backend-api":
    ~ version: "v1.0.0" → "v1.1.0"
```

### Filtered Diffs

```bash
# Show only API changes
kongctl diff -f config.yaml --format json | \
  jq '.changes[] | select(.resource_type == "api")'

# Show only label changes
kongctl diff -f config.yaml --format json | \
  jq '.changes[] | select(.changes.labels != null)'
```

### Side-by-Side Comparison

```bash
# Create visual diff
kongctl diff -f config.yaml > changes.diff
code --diff current-state.yaml desired-state.yaml
```

## Interpreting Results

### No Changes

```
No differences found between configuration and current state.
```

### Protected Resources

```
Portal "main-portal":
  ⚠️  Protected resource - changes blocked
  ~ display_name: "Portal" → "Main Portal"
```

### Missing Resources

```
Unable to compare - resource not found in current state:
- API "new-api" (will be created)
```

## Best Practices

1. **Always diff before apply/sync** in production
2. **Use JSON format** for automated validation
3. **Check for breaking changes** in CI/CD
4. **Review deletions carefully** in sync mode
5. **Save diff output** for audit trails

## Troubleshooting

### Empty diff output
- Ensure configuration files are valid
- Check authentication is working
- Verify resource refs match

### Diff shows unexpected changes
- Someone may have modified resources directly
- Check for case sensitivity in refs
- Ensure latest state is fetched

### Performance issues
```bash
# Enable caching for large diffs
export KONGCTL_CACHE_TTL=300
kongctl diff -f large-config/
```

## Related Commands

- `kongctl plan` - Generate execution plan
- `kongctl apply` - Apply changes (no deletions)
- `kongctl sync` - Full synchronization

## See Also

- [Declarative Configuration Guide](https://github.com/Kong/kongctl/blob/main/docs/declarative-configuration.md)
- [Getting Started Guide](https://github.com/Kong/kongctl/blob/main/docs/declarative-getting-started.md)
- [CI/CD Integration Guide](https://github.com/Kong/kongctl/blob/main/docs/declarative-ci-cd.md)