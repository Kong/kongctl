# Troubleshooting Guide - Declarative Configuration

This guide helps resolve common issues when using kongctl's declarative configuration features for API resources and multi-resource management.

## Table of Contents

- [File Loading Issues](#file-loading-issues)
- [YAML Tag Problems](#yaml-tag-problems)
- [Cross-Resource Reference Errors](#cross-resource-reference-errors)
- [API Resource Configuration](#api-resource-configuration)
- [Performance Issues](#performance-issues)
- [Security and Permissions](#security-and-permissions)
- [Command-Specific Troubleshooting](#command-specific-troubleshooting)
- [Best Practices](#best-practices)

## File Loading Issues

### File Not Found Errors

**Error message**:
```
Error: failed to process file tag: file not found: ./specs/api.yaml
```

**Common causes and solutions**:

1. **Incorrect relative path**:
   ```yaml
   # ❌ Wrong - path not relative to config file
   spec: !file specs/api.yaml
   
   # ✅ Correct - proper relative path
   spec: !file ./specs/api.yaml
   ```

2. **File doesn't exist**:
   ```bash
   # Check if file exists
   ls -la ./specs/api.yaml
   
   # Create missing file or fix path
   ```

3. **Wrong base directory**:
   ```
   project/
   ├── config/
   │   └── main.yaml       # Config file here
   └── specs/
       └── api.yaml        # Spec file here
   ```
   
   In `config/main.yaml`:
   ```yaml
   # ❌ Wrong - looks in config/specs/
   spec: !file ./specs/api.yaml
   
   # ✅ Correct - goes up one level first
   spec: !file ../specs/api.yaml
   ```

### File Permission Errors

**Error message**:
```
Error: failed to process file tag: permission denied: ./specs/api.yaml
```

**Solutions**:
```bash
# Check file permissions
ls -la ./specs/api.yaml

# Fix permissions
chmod 644 ./specs/api.yaml

# Check directory permissions
ls -la ./specs/
chmod 755 ./specs/
```

### Large File Handling

**Error message**:
```
Error: file size exceeds limit: ./large-spec.yaml (12MB > 10MB limit)
```

**Solutions**:

1. **Split large files**:
   ```yaml
   # Instead of one huge spec, split into sections
   apis:
     - ref: users-api
       versions:
         - ref: users-v1
           spec: !file ./specs/users/v1/core.yaml
         - ref: users-v2
           spec: !file ./specs/users/v2/enhanced.yaml
   ```

2. **Use value extraction**:
   ```yaml
   # Extract only needed values instead of entire file
   name: !file ./large-spec.yaml#info.title
   version: !file ./large-spec.yaml#info.version
   # Don't load: spec: !file ./large-spec.yaml
   ```

3. **Optimize file content**:
   ```bash
   # Remove comments and formatting to reduce size
   yq eval 'del(..|select(tag == "!!null"))' large-spec.yaml > optimized-spec.yaml
   ```

## YAML Tag Problems

### Invalid Extraction Path

**Error message**:
```
Error: path not found: info.nonexistent.field
```

**Debugging steps**:

1. **Inspect file structure**:
   ```bash
   # View YAML structure
   yq eval '.' ./specs/api.yaml
   
   # Check specific path
   yq eval '.info' ./specs/api.yaml
   ```

2. **Common path mistakes**:
   ```yaml
   # ❌ Wrong field names
   title: !file ./spec.yaml#info.titel  # Typo: "titel"
   
   # ✅ Correct field names
   title: !file ./spec.yaml#info.title
   
   # ❌ Wrong array syntax
   server: !file ./spec.yaml#servers[0].url  # Wrong bracket syntax
   
   # ✅ Correct array syntax
   server: !file ./spec.yaml#servers.0.url
   ```

3. **Case sensitivity**:
   ```yaml
   # YAML is case-sensitive
   # ❌ Wrong case
   version: !file ./spec.yaml#Info.Version
   
   # ✅ Correct case
   version: !file ./spec.yaml#info.version
   ```

### Malformed YAML Tag Syntax

**Error message**:
```
Error: failed to parse file reference: invalid tag format
```

**Common syntax errors**:

1. **Missing file path**:
   ```yaml
   # ❌ Empty file reference
   description: !file
   
   # ✅ Provide file path
   description: !file ./docs/description.txt
   ```

2. **Invalid hash syntax**:
   ```yaml
   # ❌ Multiple hashes
   title: !file ./spec.yaml##info.title
   
   # ❌ Hash at start
   title: !file #./spec.yaml#info.title
   
   # ✅ Single hash for extraction
   title: !file ./spec.yaml#info.title
   ```

3. **Map format errors**:
   ```yaml
   # ❌ Missing required fields
   title: !file
     extract: info.title  # Missing 'path'
   
   # ❌ Wrong field names
   title: !file
     file: ./spec.yaml     # Should be 'path'
     get: info.title       # Should be 'extract'
   
   # ✅ Correct map format
   title: !file
     path: ./spec.yaml
     extract: info.title
   ```

### File Format Issues

**Error message**:
```
Error: failed to parse YAML: yaml: line 5: mapping values are not allowed
```

**Solutions**:

1. **Validate referenced files**:
   ```bash
   # Check YAML syntax
   yq eval '.' ./specs/api.yaml
   
   # Check JSON syntax
   jq '.' ./specs/api.json
   
   # Use online validators for complex files
   ```

2. **Handle special characters**:
   ```yaml
   # Files with special characters in content
   # ❌ May cause parsing issues
   description: !file ./docs/description-with-quotes.txt
   
   # ✅ Use YAML escaping if needed
   description: !file "./docs/description-with-quotes.txt"
   ```

## Cross-Resource Reference Errors

### Unknown Resource References

**Error message**:
```
Error: resource "my-api" references unknown portal: unknown-portal (field: portal)
```

**Common causes and solutions**:

1. **Typo in resource ref**:
   ```yaml
   portals:
     - ref: developer-portal  # Note: developer-portal
   
   api_publications:
     - ref: api-pub
       api: my-api
       portal: dev-portal     # ❌ Wrong: should be developer-portal
   ```

2. **Resource not defined**:
   ```yaml
   # ❌ Portal referenced but not defined
   api_publications:
     - ref: api-pub
       portal: missing-portal  # This portal doesn't exist
   
   # ✅ Define the portal first
   portals:
     - ref: missing-portal
       name: "missing-portal"
       # ... portal configuration
   ```

3. **Resource ordering issues**:
   ```yaml
   # ❌ Wrong order - publication before portal
   api_publications:
     - ref: api-pub
       portal: my-portal
   
   portals:
     - ref: my-portal  # Defined after being referenced
   
   # ✅ Correct order - portal before publication
   portals:
     - ref: my-portal
       name: "my-portal"
   
   api_publications:
     - ref: api-pub
       portal: my-portal
   ```

### External ID vs Reference Issues

**Error message**:
```
Error: resource "my-impl" references unknown control_plane_id: my-control-plane
```

**Understanding external vs internal references**:

```yaml
api_implementations:
  # ✅ External UUID (existing resource in Kong)
  - ref: external-impl
    service:
      control_plane_id: "550e8400-e29b-41d4-a716-446655440000"  # UUID
      id: "550e8400-e29b-41d4-a716-446655440001"                # UUID
  
  # ❌ Wrong - trying to reference declarative resource
  - ref: internal-impl
    service:
      control_plane_id: "my-control-plane"  # Not a UUID - will fail
      id: "my-service"                      # Not a UUID - will fail
```

## API Resource Configuration

### API Version Issues

**Error message**:
```
Error: API version "v1" gateway_service missing required fields
```

**Required fields checklist**:

```yaml
api_versions:
  - ref: my-api-v1
    api: my-api                    # ✅ Required: parent API reference
    name: "v1.0.0"                 # ✅ Required: version name
    gateway_service:               # ✅ Required block
      control_plane_id: "uuid"     # ✅ Required: valid UUID
      id: "uuid"                   # ✅ Required: valid UUID
    spec:                          # ✅ Required: OpenAPI spec
      openapi: "3.0.0"
      # ... rest of spec
```

### API Publication Problems

**Error message**:
```
Error: API publication visibility must be one of: public, private
```

**Valid configuration**:

```yaml
api_publications:
  - ref: my-publication
    api: my-api                           # ✅ Must reference existing API
    portal: my-portal                     # ✅ Must reference existing portal
    visibility: public                    # ✅ Must be "public" or "private"
    auto_approve_registrations: false     # ✅ Must be boolean
```

### Nested vs Separate Resource Issues

**Problem**: Mixing nested and separate declarations

```yaml
# ❌ Conflicting declarations
apis:
  - ref: my-api
    name: "My API"
    versions:
      - ref: my-api-v1  # Nested version
        name: "v1.0.0"

api_versions:
  - ref: my-api-v1      # ❌ Same ref as nested version
    api: my-api
    name: "v1.0.0"
```

**Solution**: Choose one approach consistently

```yaml
# ✅ Option 1: All nested
apis:
  - ref: my-api
    name: "My API"
    versions:
      - ref: my-api-v1
        name: "v1.0.0"
    publications:
      - ref: my-api-pub
        portal: my-portal

# ✅ Option 2: All separate
apis:
  - ref: my-api
    name: "My API"

api_versions:
  - ref: my-api-v1
    api: my-api
    name: "v1.0.0"

api_publications:
  - ref: my-api-pub
    api: my-api
    portal: my-portal
```

## Performance Issues

### Slow File Loading

**Symptoms**: Long execution times with many file tags

**Solutions**:

1. **Optimize file references**:
   ```yaml
   # ❌ Loading same file multiple times without caching benefit
   apis:
     - ref: api-1
       name: !file ./specs/large-spec.yaml#info.title
       spec: !file ./specs/other-spec.yaml  # Different file
     - ref: api-2
       name: !file ./specs/large-spec.yaml#info.title  # Same file - cached
   ```

2. **Reduce file parsing**:
   ```yaml
   # ❌ Many extractions from same large file
   contact_name: !file ./large-config.yaml#contact.name
   contact_email: !file ./large-config.yaml#contact.email
   contact_phone: !file ./large-config.yaml#contact.phone
   
   # ✅ Extract parent object once, use YAML anchors
   contact: &contact !file ./large-config.yaml#contact
   contact_name: *contact.name  # Use YAML references
   ```

3. **File size optimization**:
   ```bash
   # Remove unnecessary content from loaded files
   yq eval 'del(..|select(tag == "!!null")) | del(.examples) | del(..|.description?)' \
     large-spec.yaml > optimized-spec.yaml
   ```

### Memory Usage

**Issue**: High memory usage with many large files

**Solutions**:

1. **Load only needed portions**:
   ```yaml
   # ❌ Loading entire large specifications
   spec: !file ./huge-openapi-spec.yaml
   
   # ✅ Extract only metadata, reference spec externally
   name: !file ./huge-openapi-spec.yaml#info.title
   version: !file ./huge-openapi-spec.yaml#info.version
   # Keep spec external or load smaller portions
   ```

2. **Split large configurations**:
   ```bash
   # Split large files into smaller, focused files
   mkdir -p ./apis/users ./apis/products
   
   # Move specific API configs to dedicated files
   yq eval '.apis[] | select(.ref == "users-api")' config.yaml > ./apis/users/config.yaml
   ```

## Security and Permissions

### Path Traversal Prevention

**Error message**:
```
Error: path traversal not allowed: ../../../etc/passwd
Error: absolute paths not allowed: /etc/passwd
```

**Security restrictions**:

```yaml
# ❌ These will be blocked
config: !file /etc/passwd                    # Absolute path
secret: !file ../../../sensitive/file.yaml   # Path traversal
data: !file ./safe/../unsafe/file.yaml       # Hidden traversal

# ✅ These are allowed
config: !file ./config/app.yaml              # Relative, within project
spec: !file ./specs/api.yaml                 # Relative, within project
docs: !file ./team/docs/guide.md             # Relative, within project
```

### File Permission Issues

**Problem**: Files exist but can't be read

```bash
# Check and fix permissions
ls -la ./specs/
chmod 644 ./specs/*.yaml
chmod 755 ./specs/

# Ensure parent directories are accessible
chmod 755 ./
```

## Command-Specific Troubleshooting

### Plan Command Issues

**Error**: Plan generation fails with file loading

```bash
# Debug plan generation
kongctl plan --config my-api.yaml --log-level debug

# Check specific resource loading
kongctl plan --config my-api.yaml --dry-run
```

**Common fixes**:
1. Verify all referenced files exist
2. Check file syntax before planning
3. Validate cross-resource references

### Apply Command Issues

**Error**: Apply fails after successful plan

```bash
# Re-run plan to verify current state
kongctl plan --config my-api.yaml

# Check for external changes
kongctl diff --config my-api.yaml

# Apply with verbose logging
kongctl apply --config my-api.yaml --log-level debug
```

### Diff Command Issues

**Error**: Diff shows unexpected changes

```bash
# Force refresh of current state
kongctl diff --config my-api.yaml --refresh

# Check for configuration drift
kongctl plan --config my-api.yaml --output yaml
```

## Best Practices

### File Organization

```
project/
├── main-config.yaml                 # Main configuration
├── environments/
│   ├── dev/
│   │   └── config.yaml             # Environment-specific config
│   └── prod/
│       └── config.yaml
├── teams/
│   ├── identity/
│   │   ├── apis.yaml               # Team-specific APIs
│   │   └── specs/
│   │       └── users-api.yaml
│   └── ecommerce/
│       ├── apis.yaml
│       └── specs/
│           └── products-api.yaml
└── shared/
    ├── common.yaml                 # Shared configuration
    └── docs/
        └── descriptions/
```

### Configuration Validation

```bash
# Pre-deployment validation pipeline
validate-config() {
  # 1. YAML syntax validation
  yq eval '.' config.yaml > /dev/null
  
  # 2. File reference validation
  grep -r '!file' config.yaml | while read -r line; do
    file_path=$(echo "$line" | sed 's/.*!file \([^#]*\).*/\1/')
    [[ -f "$file_path" ]] || echo "Missing file: $file_path"
  done
  
  # 3. Plan generation test
  kongctl plan --config config.yaml --dry-run
}
```

### Error Recovery

```bash
# Common recovery steps
recovery-steps() {
  # 1. Verify file structure
  find . -name "*.yaml" -exec yq eval '.' {} \; > /dev/null
  
  # 2. Check permissions
  find . -name "*.yaml" -not -perm 644 -exec chmod 644 {} \;
  
  # 3. Validate references
  kongctl plan --config config.yaml --dry-run
  
  # 4. Reset to known good state if needed
  git checkout HEAD -- config.yaml
}
```

### Monitoring and Debugging

```bash
# Enable comprehensive logging
export KONGCTL_LOG_LEVEL=trace

# Monitor file access
strace -e trace=openat kongctl plan --config config.yaml 2>&1 | grep "\.yaml"

# Profile performance
time kongctl plan --config config.yaml
```

This troubleshooting guide covers the most common issues encountered when working with kongctl's declarative configuration features. For additional help, check the examples in `docs/examples/apis/` or consult the YAML Tags Reference Guide.