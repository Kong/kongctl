# Troubleshooting Guide

This guide helps you diagnose and resolve common issues when using kongctl's 
declarative configuration features.

## Table of Contents

- [Common Issues](#common-issues)
- [Authentication Problems](#authentication-problems)
- [Configuration Errors](#configuration-errors)
- [Planning Issues](#planning-issues)
- [Execution Failures](#execution-failures)
- [Performance Issues](#performance-issues)
- [Debugging Techniques](#debugging-techniques)

## Common Issues

### Issue: "No changes detected" when changes exist

**Symptoms:**
- Modified configuration but plan shows no changes
- Resources appear unchanged after apply

**Causes:**
1. Resource already matches desired state
2. Invalid resource references
3. Namespace mismatch

**Solutions:**

```bash
# 1. Verify current state
kongctl dump > current-state.yaml
diff current-state.yaml your-config.yaml

# 2. Check resource references
grep "ref:" your-config.yaml

# 3. Verify namespace
kongctl get apis --format json | jq '.[] | select(.labels."KONGCTL-namespace")'
```

### Issue: "Resource not found" errors

**Symptoms:**
- Error during plan or apply
- References to non-existent resources

**Example:**
```
Error: resource "my-portal" not found
```

**Solutions:**

```bash
# 1. Check if resource exists
kongctl get portals | grep my-portal

# 2. Verify resource ref spelling
grep -n "my-portal" *.yaml

# 3. Ensure dependencies are created first
kongctl apply -f portals.yaml
kongctl apply -f apis.yaml
kongctl apply -f publications.yaml
```

### Issue: File loading errors

**Symptoms:**
```
Error: failed to process file tag: file not found: ./specs/api.yaml
```

**Solutions:**

```bash
# 1. Check file exists
ls -la ./specs/api.yaml

# 2. Verify relative paths
# Paths are relative to the YAML file, not current directory
cd $(dirname config.yaml)
ls -la ./specs/api.yaml

# 3. Check file permissions
chmod 644 ./specs/api.yaml
```

## Authentication Problems

### Issue: "Unauthorized" or "403 Forbidden"

**Symptoms:**
- API calls fail with 401/403 errors
- "Invalid token" messages

**Solutions:**

```bash
# 1. Check token expiration
kongctl get portals
# If this fails, token may be expired

# 2. Re-authenticate
kongctl login

# 3. Verify PAT (if using)
echo $KONGCTL_KONNECT_PAT
# Should start with "kpat_"

# 4. Check profile
echo $KONGCTL_PROFILE
# Ensure using correct profile
```

### Issue: Multiple authentication methods conflict

**Symptoms:**
- Unexpected authentication behavior
- Wrong credentials being used

**Resolution priority:**
1. `--pat` flag
2. `KONGCTL_<PROFILE>_KONNECT_PAT` environment variable
3. Stored token from `kongctl login`

```bash
# Clear all auth methods and start fresh
unset KONGCTL_DEFAULT_KONNECT_PAT
rm ~/.config/kongctl/.default-konnect-token.json
kongctl login
```

## Configuration Errors

### Issue: YAML parsing errors

**Symptoms:**
```
Error: yaml: unmarshal errors:
  line 10: cannot unmarshal !!str `true` into bool
```

**Solutions:**

```yaml
# BAD
authentication_enabled: "true"  # String

# GOOD  
authentication_enabled: true    # Boolean

# Validate YAML
yamllint config.yaml

# Or use online validator
cat config.yaml | python -m yaml
```

### Issue: Duplicate resource references

**Symptoms:**
```
Error: duplicate resource ref "my-api" found
```

**Solutions:**

```bash
# Find duplicates
grep -n "ref: my-api" *.yaml

# Use unique refs
apis:
  - ref: users-api-v1    # Unique
  - ref: users-api-v2    # Unique
```

### Issue: Invalid field values

**Symptoms:**
```
Error: invalid value for field "visibility": "internal"
```

**Solutions:**

```yaml
# Check allowed values in documentation
api_publications:
  - ref: my-pub
    visibility: private  # Allowed: public, private
```

## Planning Issues

### Issue: Plan generation hangs

**Symptoms:**
- `kongctl plan` doesn't complete
- No output after initial message

**Solutions:**

```bash
# 1. Enable debug logging
kongctl plan -f config.yaml --log-level debug

# 2. Check network connectivity
curl -I https://global.api.konghq.com/v2/portals

# 3. Try smaller configuration
kongctl plan -f single-resource.yaml
```

### Issue: Circular dependencies

**Symptoms:**
```
Error: circular dependency detected: api1 -> api2 -> api1
```

**Solutions:**

```yaml
# BAD - Circular reference
apis:
  - ref: api1
    depends_on: api2
  - ref: api2  
    depends_on: api1

# GOOD - Break circular dependency
apis:
  - ref: api-base
  - ref: api1
    depends_on: api-base
  - ref: api2
    depends_on: api-base
```

## Execution Failures

### Issue: Partial apply failures

**Symptoms:**
- Some resources created, others fail
- Apply completes with errors

**Example output:**
```
✓ CREATE portal "dev-portal"
✗ CREATE api "users-api" - Error: Invalid configuration
✓ UPDATE api "products-api"

Apply completed with errors.
```

**Solutions:**

```bash
# 1. Fix the failed resource configuration
vim users-api.yaml

# 2. Re-run apply (idempotent)
kongctl apply -f config.yaml

# 3. Or apply just the fixed resource
kongctl apply -f users-api.yaml
```

### Issue: Protected resource blocking changes

**Symptoms:**
```
Error: Cannot modify protected resource "production-api"
```

**Solutions:**

```yaml
# 1. Temporarily remove protection
apis:
  - ref: production-api
    kongctl:
      protected: false  # Changed from true

# 2. Apply changes
kongctl apply -f api.yaml

# 3. Re-enable protection
apis:
  - ref: production-api
    kongctl:
      protected: true
```

### Issue: Sync deleting unexpected resources

**Symptoms:**
- Resources deleted that shouldn't be
- More deletions than expected

**Prevention:**

```bash
# 1. ALWAYS dry-run first
kongctl sync -f config.yaml --dry-run

# 2. Use namespaces to limit scope
kongctl sync -f team-config.yaml
# Only affects resources in that namespace

# 3. Check managed labels
kongctl get apis -o json | jq '.[] | select(.labels."KONGCTL-managed" == "true")'
```

## Performance Issues

### Issue: Slow plan generation

**Symptoms:**
- Plans take minutes to generate
- High API latency

**Solutions:**

```bash
# 1. Enable trace logging to see API calls
kongctl plan -f config.yaml --log-level trace

# 2. Reduce configuration size
# Split into smaller files
kongctl plan -f apis-batch-1.yaml
kongctl plan -f apis-batch-2.yaml

# 3. Check for rate limiting
# Look for 429 status codes in trace logs
```

### Issue: Large file handling

**Symptoms:**
```
Error: file size exceeds limit: 10MB
```

**Solutions:**

```yaml
# 1. Split large OpenAPI specs
api_versions:
  - ref: v1
    spec: !file ./specs/api-v1-endpoints.yaml
    
  - ref: v1-schemas
    additional_specs:
      - !file ./specs/api-v1-schemas.yaml

# 2. Use value extraction
apis:
  - ref: my-api
    name: !file 
      path: ./huge-spec.yaml
      extract: info.title  # Only extract needed value
```

## Debugging Techniques

### Enable Debug Logging

```bash
# Show detailed operation logs
kongctl apply -f config.yaml --log-level debug

# Show API requests/responses
kongctl apply -f config.yaml --log-level trace
```

### Trace Log Analysis

When trace logging is enabled:

```
time=2024-01-15T12:00:00.000Z level=TRACE msg="HTTP request" method=GET url=https://global.api.konghq.com/v2/portals
time=2024-01-15T12:00:01.000Z level=TRACE msg="HTTP response" status=200 duration=1s
```

Look for:
- 4xx/5xx status codes
- Slow response times
- Unexpected response bodies

### Step-by-Step Debugging

```bash
# 1. Validate configuration
cat config.yaml | python -m yaml

# 2. Test authentication
kongctl get portals

# 3. Generate plan with debug
kongctl plan -f config.yaml --log-level debug -o plan.json

# 4. Review plan
cat plan.json | jq '.changes'

# 5. Dry run
kongctl apply --plan plan.json --dry-run

# 6. Apply with trace logging
kongctl apply --plan plan.json --log-level trace
```

### Common Debug Commands

```bash
# Check current state
kongctl dump > current.yaml

# Compare configurations
diff -u current.yaml desired.yaml

# List managed resources
kongctl get apis -o json | jq '.[] | select(.labels."KONGCTL-managed")'

# Check specific resource
kongctl get api my-api -o yaml

# Verify file paths
find . -name "*.yaml" -exec echo {} \; -exec head -1 {} \;

# Validate references
for ref in $(grep -h "ref:" *.yaml | awk '{print $2}'); do
  echo "Checking ref: $ref"
  grep -l "$ref" *.yaml
done
```

## Getting Help

### 1. Extended Documentation

```bash
# View extended help
kongctl help plan
kongctl help apply
kongctl help sync
```

### 2. Check Examples

```bash
# Review example configurations
ls docs/examples/declarative/
cat docs/examples/declarative/basic/api.yaml
```

### 3. Report Issues

If you encounter a bug:

1. Collect debug information:
   ```bash
   kongctl version --full
   kongctl plan -f config.yaml --log-level trace 2> trace.log
   ```

2. Create minimal reproduction:
   - Smallest config that shows the issue
   - Remove sensitive information

3. Report at: https://github.com/Kong/kongctl/issues

## Quick Reference

### Error Patterns

| Error | Likely Cause | Quick Fix |
|-------|--------------|-----------|
| "unauthorized" | Expired token | `kongctl login` |
| "not found" | Wrong reference | Check spelling |
| "invalid value" | Wrong type/format | Check docs |
| "file not found" | Wrong path | Use relative paths |
| "protected resource" | Protection enabled | Temporarily disable |
| "circular dependency" | Resource loop | Restructure deps |

### Useful Environment Variables

```bash
# Enable debug globally
export KONGCTL_LOG_LEVEL=debug

# Use specific profile
export KONGCTL_PROFILE=production

# Override API URL (for testing)
export KONGCTL_KONNECT_BASE_URL=https://api.konghq.tech
```

## Prevention Tips

1. **Always dry-run** in production
2. **Use version control** for configurations
3. **Test in lower environments** first
4. **Keep configurations small** and focused
5. **Use namespaces** to isolate changes
6. **Enable trace logging** when debugging
7. **Review plans** before applying