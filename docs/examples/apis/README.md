# API Resource Examples

This directory contains example configurations for Kong Konnect API resources.
These examples demonstrate the declarative configuration format for managing APIs
through kongctl.

## Current Support (Stages 1-3)

Currently, kongctl supports basic API resource creation with the following
fields:

- `ref`: Unique reference identifier for the API
- `name`: API name (must be unique when combined with version)
- `description`: Optional description visible in portals
- `version`: Optional version string
- `slug`: Optional URL-friendly identifier (auto-generated if not provided)
- `labels`: Key-value pairs for filtering and organization
- `attributes`: Multi-value attributes for categorization
- `spec_content`: Inline API specification (OpenAPI/AsyncAPI)
- `kongctl.protected`: Protection flag to prevent accidental deletion

## Examples

### [basic-api.yaml](basic-api.yaml)
Demonstrates the minimum configuration needed to create an API with essential
fields like name, description, version, and labels.

### [api-with-versions.yaml](api-with-versions.yaml)
Shows how to define an API that will support multiple versions. Currently shows
the basic API structure, with comments indicating how nested versions will work
in Stage 4.

### [api-with-external-spec.yaml](api-with-external-spec.yaml)
Demonstrates including an OpenAPI specification directly in the configuration.
Includes comments showing how Stage 4 will support loading specs from external
files using YAML tags.

### [multi-resource.yaml](multi-resource.yaml)
Complete example showing APIs working together with portals. Includes comments
showing how API publications will work in Stage 4 to publish APIs to developer
portals.

## Future Features (Stage 4)

Stage 4 will add support for:

1. **API Child Resources**:
   - API Versions with separate specs
   - API Publications to portals
   - API Implementations with service mappings

2. **YAML Tags for External Content**:
   ```yaml
   # Load entire file
   spec_content: !file ./specs/api.yaml
   
   # Extract specific values
   name: !file
     path: ./specs/api.yaml
     extract: info.title
   
   # Compact extraction syntax
   version: !file.extract [./specs/api.yaml, info.version]
   ```

3. **Cross-Resource References**:
   - APIs can reference portals for publication
   - Publications can reference specific API versions
   - Support for external IDs (control planes, services)

4. **Dependency Management**:
   - Automatic ordering of resource creation
   - Validation of references between resources
   - Clear error messages for missing dependencies

## Usage

To apply these configurations:

```bash
# Apply a single API configuration
kongctl apply --config basic-api.yaml

# Apply multiple resources
kongctl apply --config multi-resource.yaml

# Preview changes without applying
kongctl plan --config api-with-external-spec.yaml

# Show differences between current and desired state
kongctl diff --config api-with-versions.yaml
```

## Best Practices

1. **Use meaningful refs**: Choose ref values that clearly identify the API
2. **Apply labels consistently**: Use labels for team ownership, environment,
   and other organizational needs
3. **Protect production APIs**: Set `kongctl.protected: true` for critical APIs
4. **Include API specs**: Provide OpenAPI/AsyncAPI specs for better
   documentation
5. **Version appropriately**: Use semantic versioning for API versions

## Field Reference

### Required Fields
- `ref`: Unique identifier within your configuration
- `name`: Human-readable name for the API

### Optional Fields
- `description`: Detailed description of the API
- `version`: Version string (e.g., "v1.0.0")
- `slug`: URL-friendly identifier (auto-generated from name+version if omitted)
- `labels`: Map of string key-value pairs for filtering
- `attributes`: Map of string keys to string arrays for categorization
- `spec_content`: Inline API specification in YAML or JSON format
- `kongctl.protected`: Boolean flag to prevent deletion (default: false)

## Notes

- The `name` + `version` combination must be unique across all APIs
- Labels cannot start with "kong", "konnect", "mesh", "kic", or "_"
- Slugs are used in generated URLs and should be URL-friendly
- API specs can be OpenAPI 2.0, OpenAPI 3.x, or AsyncAPI formats