# API with External Control Plane Example

This example demonstrates the dual support for `control_plane_id` in API 
implementations, allowing both declarative references and external UUIDs.

## Use Cases

1. **Gradual Migration**: Teams can start using declarative configuration for 
   new control planes while keeping existing ones external
2. **Hybrid Deployments**: APIs can be deployed across both managed and 
   unmanaged control planes
3. **Legacy Integration**: Existing control planes can be referenced without 
   requiring full migration

## Key Concepts

### Managed Control Planes (Using References)
```yaml
control_plane_id: managed-cp  # References a control plane defined in the config
```

### External Control Planes (Using UUIDs)
```yaml
control_plane_id: "f9e8d7c6-b5a4-3210-9876-fedcba098765"  # External UUID
```

## Important Notes

- The `service.id` field ALWAYS requires a UUID (services are managed by decK)
- The `control_plane_id` field accepts EITHER a reference OR a UUID
- This dual support is temporary until core Kong Gateway entities are supported 
  in kongctl's declarative configuration

## Running this Example

```bash
# Generate a plan to see what would be created
kongctl plan -f api-external-cp.yaml

# Apply the configuration (only the managed control plane will be created)
kongctl apply -f api-external-cp.yaml
```

The external control planes (referenced by UUID) must already exist in your Kong 
environment for the API implementations to work correctly.