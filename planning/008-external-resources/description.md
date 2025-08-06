# Stage 8: External Resources

## Overview

Implement external resource references to enable kongctl to integrate with 
resources managed by other Kong declarative configuration tools (decK, Kong 
Operator, Terraform provider). This feature allows users to reference 
existing resources without taking ownership, enabling gradual migration and 
parallel workflows during transition periods.

## Problem Statement

Kong customers have existing investments in multiple declarative tools:
- **decK**: Manages Kong Gateway core entities (services, routes, plugins)
- **Kong Operator**: Kubernetes-native resource management
- **Terraform Provider**: Infrastructure-as-code for Konnect

As kongctl expands capabilities, users need:
- Migration paths that don't require abandoning existing tools immediately
- Ability to operate multiple tools in parallel
- Ways to reference externally-managed resources in kongctl configurations
- Gradual adoption strategy for comprehensive Konnect resource management

## Solution

Introduce `external_resources` blocks that:
- Define references to resources managed by external tools
- Use selectors to query and identify specific resources
- Resolve to resource IDs and data during planning
- Enable dependencies between kongctl-managed and externally-managed resources

## Important Limitations

### Phase 1 Limitations
- Only `matchFields` selectors (simple equality matching)
- No complex query expressions or operators
- Must resolve to exactly one resource (no optional or multiple matches)
- Limited to resource types with SDK support

### Not Supported
- Direct management or modification of external resources
- Cascade operations on external resources
- Import of external resources into kongctl management
- Provider-specific features or optimizations

## Key Features

1. **Direct ID Reference**: Reference resources by UUID when known
2. **Field Selectors**: Query resources by field values
3. **Parent Relationships**: Support hierarchical resource structures
4. **Implicit ID Resolution**: Automatic resolution of ID fields
5. **Type Safety**: Resource-type-aware validation and resolution

## Success Criteria

- Users can reference any supported Konnect resource managed externally
- Clear error messages for ambiguous or missing resources
- Seamless integration in dependency chains
- Intuitive configuration syntax

## Example Use Case

A team using decK for gateway configuration wants to adopt kongctl for API 
management. They can reference their existing control planes and services 
while managing APIs and API implementations through kongctl:

```yaml
external_resources:
  - ref: prod-cp
    resource_type: control_plane
    selector:
      matchFields:
        name: production-control-plane
  - ref: user-service
    resource_type: ce_service # core entity service
    control_plane: prod-cp    # Parent reference
    selector:
      matchFields:
        name: user-service

apis:
  - ref: user-api
    name: User Management API
    
api_implementations:
  - ref: impl
    api:
      ref: user-api
    service:
      control_plane_id: prod-cp  # Resolves to ID of external resource
      id: user-service           # Resolves to ID of external resource
```
