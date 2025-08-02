# YAML Tags Reference Guide

This guide provides comprehensive documentation for YAML tags in kongctl, which enable loading content from external files and extracting specific values from structured data.

## Overview

YAML tags allow you to:
- Load entire files as string content
- Extract specific values from YAML or JSON files
- Organize large configurations across multiple files
- Enable team ownership of configuration sections
- Manage large OpenAPI specifications externally

## Syntax Formats

### 1. Simple File Loading

Load the entire content of a file as a string:

```yaml
description: !file ./path/to/file.txt
```

**Supported file types**: Any text file (`.txt`, `.md`, `.yaml`, `.json`, etc.)

**Use cases**:
- Loading plain text descriptions
- Loading markdown documentation
- Loading entire configuration files

### 2. Hash Syntax for Value Extraction

Extract specific values using hash (`#`) notation:

```yaml
name: !file ./config.yaml#api.name
version: !file ./spec.yaml#info.version
contact_email: !file ./spec.yaml#info.contact.email
```

**Path syntax**:
- Use dot notation for nested objects: `info.contact.email`
- Use array indices for arrays: `servers.0.url`
- Supports deep nesting: `paths./users.get.responses.200.description`

### 3. Map Format for Complex Extraction

Use map format for better readability with complex paths:

```yaml
api_title: !file
  path: ./openapi-spec.yaml
  extract: info.title

contact_info: !file
  path: ./openapi-spec.yaml
  extract: info.contact
```

**Benefits**:
- More readable for complex extraction paths
- Easier to document and maintain
- Clear separation of file path and extraction logic

## Supported File Types

### YAML Files (.yaml, .yml)

```yaml
# config.yaml
api_defaults:
  name: "Default API"
  version: "1.0.0"
  description: "Default API description"
environment:
  name: "production"
  region: "us-west-2"
```

**Extraction examples**:
```yaml
name: !file ./config.yaml#api_defaults.name
environment: !file ./config.yaml#environment.name
```

### JSON Files (.json)

```json
{
  "info": {
    "title": "Users API",
    "version": "2.0.0",
    "contact": {
      "email": "api@example.com"
    }
  },
  "servers": [
    {"url": "https://api.example.com"},
    {"url": "https://staging.example.com"}
  ]
}
```

**Extraction examples**:
```yaml
title: !file ./api.json#info.title
production_url: !file ./api.json#servers.0.url
staging_url: !file ./api.json#servers.1.url
```

### Plain Text Files (.txt, .md)

```markdown
# API Documentation

This API provides comprehensive user management capabilities
including authentication, authorization, and profile management.

## Features

- User registration and authentication
- Role-based access control
- Profile management
- Password reset functionality
```

**Usage**:
```yaml
description: !file ./docs/api-description.md
```

### OpenAPI Specifications

Perfect for extracting metadata and loading entire specs:

```yaml
apis:
  - ref: users-api
    # Extract metadata from OpenAPI spec
    name: !file ./specs/users-api.yaml#info.title
    description: !file ./specs/users-api.yaml#info.description
    version: !file ./specs/users-api.yaml#info.version
    
    versions:
      - ref: users-api-v1
        name: !file ./specs/users-api.yaml#info.version
        gateway_service:
          control_plane_id: "your-control-plane-id"
          id: "your-service-id"
        # Load entire specification
        spec: !file ./specs/users-api.yaml
```

## Path Resolution

### Relative Paths

All file paths are resolved relative to the directory containing the configuration file:

```
project/
├── config.yaml          # Main config file
├── specs/
│   ├── users-api.yaml
│   └── products-api.yaml
└── docs/
    └── descriptions.txt
```

In `config.yaml`:
```yaml
apis:
  - ref: users-api
    name: !file ./specs/users-api.yaml#info.title
    description: !file ./docs/descriptions.txt
```

### Nested Directories

```
project/
├── teams/
│   ├── identity/
│   │   ├── config.yaml
│   │   └── specs/
│   │       └── users.yaml
│   └── ecommerce/
│       ├── config.yaml
│       └── specs/
│           └── products.yaml
└── shared/
    └── common.yaml
```

In `teams/identity/config.yaml`:
```yaml
apis:
  - ref: users-api
    # Load from team's spec directory
    name: !file ./specs/users.yaml#info.title
    # Load from shared directory
    environment: !file ../../shared/common.yaml#environment
```

## Advanced Examples

### Array Access

Extract specific items from arrays:

```yaml
# OpenAPI spec with multiple servers
servers:
  - url: "https://api.example.com"
    description: "Production"
  - url: "https://staging.example.com"
    description: "Staging"
  - url: "https://dev.example.com"
    description: "Development"
```

```yaml
# Extract specific servers
production_server: !file ./spec.yaml#servers.0.url
staging_server: !file ./spec.yaml#servers.1.url
dev_server: !file ./spec.yaml#servers.2.url
```

### Complex Object Extraction

Extract entire objects or nested structures:

```yaml
# Extract complete contact object
contact_info: !file ./spec.yaml#info.contact

# Extract complex nested paths
auth_config: !file ./spec.yaml#components.securitySchemes.oauth2
```

### Mixed Content Loading

Combine file loading with static values:

```yaml
apis:
  - ref: mixed-api
    # Load from external file
    name: !file ./specs/api.yaml#info.title
    # Static value
    version: "1.0.0"
    # Load description from text file
    description: !file ./docs/description.txt
    # Extract from config
    labels:
      team: !file ./config.yaml#defaults.team
      environment: production  # Static value
      contact: !file ./specs/api.yaml#info.contact.email
```

### Conditional File Loading

Use different files based on environment or team:

```yaml
# Environment-specific configuration
apis:
  - ref: environment-api
    name: "Environment API"
    # Load environment-specific settings
    labels:
      environment: !file ./envs/${ENVIRONMENT:-dev}/config.yaml#environment.name
      region: !file ./envs/${ENVIRONMENT:-dev}/config.yaml#environment.region
```

### Team Ownership Pattern

Enable teams to manage their own configurations:

```yaml
# main-config.yaml
apis:
  # Each team maintains their own API configuration
  - !file ./teams/identity/users-api.yaml
  - !file ./teams/ecommerce/products-api.yaml
  - !file ./teams/payments/billing-api.yaml

# teams/identity/users-api.yaml
ref: users-api
name: !file ./specs/users.yaml#info.title
description: !file ./specs/users.yaml#info.description
labels:
  team: identity
  owner: !file ./team-config.yaml#team.owner
  contact: !file ./team-config.yaml#team.contact
```

## Security Features

### Path Traversal Prevention

Absolute paths and path traversal attempts are blocked:

```yaml
# ❌ These will fail with security errors
description: !file /etc/passwd
config: !file ../../../sensitive/file.yaml
data: !file ./safe/../../../etc/hosts
```

```yaml
# ✅ These are allowed
description: !file ./docs/description.txt
config: !file ./config/settings.yaml
data: !file ./subdir/data.yaml
```

### File Size Limits

Files are limited to 10MB for security and performance:

```yaml
# ✅ Small to medium files are fine
spec: !file ./openapi-spec.yaml  # Typical size: 50KB - 2MB

# ❌ Very large files will be rejected
large_data: !file ./huge-dataset.json  # If > 10MB
```

### Supported Base Directories

Files must be within the project directory structure:

```yaml
# ✅ Allowed: files within project
config: !file ./config/app.yaml
spec: !file ./specs/api.yaml
docs: !file ./docs/guide.md

# ❌ Not allowed: files outside project
system: !file /etc/config.yaml
home: !file ~/private/data.yaml
```

## Performance Features

### File Caching

Files are cached during a single execution to improve performance:

```yaml
apis:
  - ref: api-1
    name: !file ./common.yaml#api.name        # File loaded and cached
    description: !file ./common.yaml#api.desc # Uses cached version
  - ref: api-2
    team: !file ./common.yaml#team.name       # Uses cached version
```

### Thread Safety

File loading is thread-safe for concurrent operations:

```yaml
# Multiple resources can safely reference the same files
apis:
  - ref: users-api
    name: !file ./specs/users.yaml#info.title
  - ref: products-api
    name: !file ./specs/users.yaml#info.title  # Same file, safe
```

## Error Handling

### Common Errors and Solutions

**File not found**:
```
Error: failed to process file tag: file not found: ./specs/missing.yaml
```
- Verify the file path is correct
- Check that the file exists
- Ensure proper relative path from config file location

**Invalid extraction path**:
```
Error: path not found: nonexistent.field
```
- Verify the extraction path exists in the file
- Check YAML/JSON structure and field names
- Use a YAML/JSON viewer to inspect file structure

**Parse errors**:
```
Error: failed to parse YAML: yaml: line 5: mapping values are not allowed in this context
```
- Validate YAML/JSON syntax
- Check for proper indentation
- Use a validator tool

**Security violations**:
```
Error: absolute paths not allowed: /etc/passwd
Error: path traversal not allowed: ../../../sensitive/file
```
- Use only relative paths within the project
- Remove `../` path traversal attempts
- Ensure files are within allowed directories

## Best Practices

### Organization

1. **Group related files**: Keep specs, configs, and docs in organized directories
2. **Use descriptive filenames**: Make file purposes clear (`users-api-v2.yaml`)
3. **Maintain consistent structure**: Use standard directory layouts across teams

### Performance

1. **Minimize file loading**: Extract only needed values rather than loading entire files
2. **Cache-friendly patterns**: Reference common files multiple times to benefit from caching
3. **Reasonable file sizes**: Keep individual files under 1MB when possible

### Maintainability

1. **Document file dependencies**: Comment which external files are required
2. **Use meaningful extraction paths**: Choose clear, stable field names
3. **Version control**: Track all referenced files in version control

### Team Collaboration

1. **Clear ownership**: Define which teams own which files
2. **Stable interfaces**: Maintain consistent extraction paths across versions
3. **Validation**: Test file loading in CI/CD pipelines

## Migration from Inline Content

### Before (Inline Configuration)

```yaml
apis:
  - ref: users-api
    name: "Users API"
    description: "User management and authentication API with comprehensive features including registration, login, profile management, and role-based access control."
    version: "v3.0.0"
    spec:
      openapi: 3.0.0
      info:
        title: Users API
        version: 3.0.0
        description: User management API
      # ... hundreds of lines of OpenAPI spec ...
```

### After (File-Based Configuration)

```yaml
apis:
  - ref: users-api
    name: !file ./specs/users-api.yaml#info.title
    description: !file ./docs/users-api-description.txt
    version: !file ./specs/users-api.yaml#info.version
    spec: !file ./specs/users-api.yaml
```

**Benefits**:
- Cleaner main configuration
- Reusable OpenAPI specs
- Better version control diffs
- Team ownership of specs
- IDE support for OpenAPI editing

This approach scales much better for large API platforms with multiple teams and hundreds of endpoints.