# Comprehensive Example

This directory contains a comprehensive example showing all supported resource types and configurations.

## Structure

- `main.yaml` - Complete configuration with all resource types
- `docs/` - Documentation files referenced by API configurations
- `specs/` - OpenAPI specifications for API versions

## Features Demonstrated

- APIs with multiple versions
- Portal configurations with customization
- Application authentication strategies
- File references using YAML tags
- Labels and metadata
- Nested child resources

## Usage

Sync the complete configuration:

```bash
kongctl sync -f main.yaml
```

This example will be expanded as new features are added to kongctl.
