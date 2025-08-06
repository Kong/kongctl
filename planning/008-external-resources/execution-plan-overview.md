# External Resources Execution Plan Overview

## Objective

Implement external resource references in kongctl to enable integration with 
resources managed by other Kong declarative tools (decK, Terraform, Kong 
Operator) without taking ownership.

## Scope

### In Scope (Phase 1)
- Basic external resource definition syntax
- Direct ID references
- Simple field-based selectors (matchFields)
- Parent relationship handling
- Implicit ID resolution for dependencies
- Planning-phase resolution

### Out of Scope (Future Phases)
- Complex query expressions (matchExpressions)
- Resource import/adoption
- Modification of external resources
- Cross-namespace external references

## Technical Approach

### 1. Configuration Schema

#### External Resource Definition
```yaml
external_resources:
  # Direct ID reference
  - ref: specific-cp
    resource_type: control_plane
    id: "550e8400-e29b-41d4-a716-446655440000"
    
  # Selector-based reference
  - ref: prod-cp
    resource_type: control_plane
    selector:
      matchFields:
        name: production-control-plane
        
  # Core entity with parent (ce_ prefix for Gateway core entities)
  - ref: user-service
    resource_type: ce_service  # Gateway service (core entity)
    control_plane: prod-cp      # Required parent reference
    selector:
      matchFields:
        name: user-api-service
```

#### Using External Resources
```yaml
api_implementations:
  - ref: impl
    api:
      ref: user-api
    service:
      control_plane_id: prod-cp  # Implicit ID resolution
      id: user-service          # Implicit ID resolution
```

### 2. Resolution Process

```
PLANNING PHASE:
┌─────────────────────┐
│ Parse Configuration │
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│ Parse External      │ ◄── Process external_resources first
│ Resources           │
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│ Resolve Parents     │ ◄── Resolve in dependency order
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│ Execute Selectors   │ ◄── Query via SDK
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│ Validate Matches    │ ◄── Fail if != 1 match
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│ Store in Memory     │ ◄── Keep for planning duration
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│ Process Other       │ ◄── APIs, Portals, etc.
│ Resources           │
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│ Build PlannedChanges│ ◄── References contain resolved IDs
└──────────┬──────────┘
           │
           ▼
EXECUTION PHASE:
┌─────────────────────┐
│ Execute Changes     │ ◄── Uses pre-resolved IDs
└─────────────────────┘
```

### 3. Resource Type Registry

Maintain registry of supported external resource types:
```go
type ExternalResourceType struct {
    Name            string
    RequiredParents []string  // e.g., ["control_plane"] for ce_service
    OptionalParents []string  // e.g., ["service"] for ce_route
    SDKListFunc     func()    // SDK function to list resources
    SDKGetFunc      func()    // SDK function to get by ID
}

// Example registry entries:
registry["control_plane"] = ExternalResourceType{
    Name:            "control_plane",
    RequiredParents: nil,  // Top-level resource
}

registry["ce_service"] = ExternalResourceType{
    Name:            "ce_service",
    RequiredParents: []string{"control_plane"},
}

registry["ce_route"] = ExternalResourceType{
    Name:            "ce_route",
    RequiredParents: []string{"control_plane"},
    OptionalParents: []string{"ce_service"},
}
```

### 4. Implementation Components

#### ExternalResourceResolver
- Parses external_resources blocks before other resources
- Manages resolution order (parents first)
- Executes SDK queries based on resource type
- Validates exactly one match per selector
- **Stores complete resource objects in memory** (not just IDs)
- Provides full resource data to planner

#### Resource Storage
```go
type ResolvedExternalResource struct {
    Ref          string
    ResourceType string
    FullObject   interface{}  // Complete resource from SDK
    ID           string        // Extracted for convenience
}

// In-memory storage during planning/execution
externalResources map[string]ResolvedExternalResource
```

#### Integration with Planner
- Planner calls ExternalResourceResolver first
- When building PlannedChange for resources with external refs:
  - Uses type knowledge to determine which field to extract
  - For `_id` fields, extracts the ID from stored object
  - Populates References map with extracted values
  - No runtime SDK calls needed (already resolved)

#### Field Extraction Logic
- **Phase 1**: Type-based knowledge for ID extraction
  - `control_plane_id` → extract `control_plane.id`
  - `service.id` → extract `ce_service.id`
- **Future**: Could add field mapping or JQ-like syntax

#### Executor Changes
- Minimal changes required
- External references already resolved in PlannedChange.References
- Uses existing reference resolution logic

#### Validation
- Schema validation for external_resources
- Resource type validation
- Parent relationship validation
- Match uniqueness validation

### 5. Error Handling

#### Resolution Failures (Planning Phase)
```
Error: External resource 'prod-cp' selector matched 0 resources
  Resource type: control_plane
  Selector: 
    matchFields:
      name: "production-cp"
  Suggestion: Verify the resource exists in Konnect
  
Error: External resource 'user-service' selector matched 3 resources:
  Resource type: ce_service (Gateway core entity)
  Parent: control_plane 'prod-cp'
  Matched resources:
    1. service "api-service-1" (id: abc123...)
    2. service "api-service-2" (id: def456...)
    3. service "api-service-3" (id: ghi789...)
  Suggestion: Refine selector to match exactly one resource

Error: External resource 'user-service' parent 'prod-cp' not found
  Resource type: ce_service
  Parent reference: prod-cp
  Suggestion: Ensure parent external resource is defined first
```

#### Validation Errors (Parse Time)
```
Error: Unknown resource type 'service'
  Did you mean 'ce_service' (Gateway core entity)?
  Valid resource types: control_plane, portal, api, ce_service, ce_route, ce_consumer, ce_plugin

Error: Invalid external resource configuration
  Resource: "user-service" (type: ce_service)
  Missing required parent field: control_plane
  
Error: Invalid parent type for external resource
  Resource: "my-route" (type: ce_route)
  Parent field 'portal' is not valid for resource type 'ce_route'
  Valid parent types: control_plane, ce_service
  
Error: Circular dependency detected
  Resource dependency chain: A → B → C → A
```

## Success Metrics

1. **Functionality**
   - All external resource types resolvable
   - Parent relationships properly handled
   - ID resolution working for all dependency types

2. **Reliability**
   - Clear error messages for all failure cases
   - No silent failures
   - Consistent resolution behavior

3. **Performance**
   - Minimal SDK calls (batch where possible)
   - Efficient caching
   - Fast resolution for direct ID references

4. **Usability**
   - Intuitive configuration syntax
   - Clear documentation
   - Helpful error messages

## Migration Strategy

### Phase 1: Core Implementation (Current)
- Basic external_resources support
- matchFields selectors only
- Two resource types only:
  - `control_plane`
  - `ce_service`
- ID field extraction only
- Complete object storage for future use

### Phase 2: Extended Support (Future)
- Additional core entity types (ce_route, ce_plugin, etc.)
- Additional Konnect resource types (portal, api, etc.)
- matchExpressions for complex queries
- Field extraction beyond IDs (if needed)

### Phase 3: Advanced Features (Future)
- Resource adoption/import via export command
- Bulk external resource operations
- Cross-organization references
- JQ-like field extraction syntax