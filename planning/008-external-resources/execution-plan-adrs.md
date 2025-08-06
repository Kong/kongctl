# Architecture Decision Records (ADRs)

## ADR-001: Core External Resource Design

### Status
Accepted

### Decision
- Use `external_resources` (plural) blocks containing arrays of external resource definitions
- Each resource requires `ref` and `resource_type` fields
- Support both direct ID reference and selector-based queries
- External resources are read-only references, not managed resources

### Rationale
- Consistent with existing kongctl patterns (plural resource blocks)
- Clear separation between managed and external resources
- Flexibility for different identification methods

## ADR-002: Selector Design - Phase 1

### Status
Accepted

### Decision
Start with `matchFields` only for simple equality matching:
```yaml
selector:
  matchFields:
    name: production-cp
    environment: prod
```

### Rationale
- Simpler initial implementation
- Covers majority of use cases
- Foundation for future enhancements
- Clear, intuitive syntax

### Future Consideration
matchExpressions with operators for complex queries (deferred to Phase 2)

## ADR-003: Direct ID Reference

### Status
Accepted

### Decision
Support direct UUID reference as peer to selector:
```yaml
external_resources:
  - ref: my-resource
    resource_type: control_plane
    id: "550e8400-e29b-41d4-a716-446655440000"
```

### Rationale
- Most efficient resolution method
- Clear precedence (ID overrides selector)
- Common use case when IDs are known

## ADR-004: Selector Matching Requirements

### Status
Accepted

### Decision
- Selectors must match exactly one resource
- Zero matches: fail fast with clear error
- Multiple matches: fail fast with list of matches

### Rationale
- External resources are dependencies
- Ambiguity should block operations
- Clear error messages guide users to fix selectors

## ADR-005: Parent Relationships

### Status
Accepted

### Decision
Use typed parent fields based on resource type knowledge:
```yaml
external_resources:
  - ref: user-service
    resource_type: gw_service
    control_plane: prod-cp  # Parent reference
```

### Rationale
- Leverages resource type knowledge
- Clean syntax without unnecessary nesting
- Consistent with resource hierarchies

## ADR-006: External Resource Storage and Field Resolution

### Status
Accepted

### Decision
- External resources are fully resolved during planning
- Complete resource objects are stored in memory
- All fields are available for potential use
- For Phase 1: Only ID fields are extracted for dependencies
- Field extraction based on type knowledge, not string patterns

### Implementation
```yaml
# External resource provides complete object
external_resources:
  - ref: prod-cp
    # Resolves to complete object with id, name, config, labels, etc.

# Dependent resource extracts needed field (ID for now)
api_implementations:
  service:
    control_plane_id: prod-cp  # System knows to extract prod-cp.id
    id: user-service          # System knows to extract user-service.id
```

### Rationale
- Storing complete objects enables future field access
- Type knowledge ensures correct field extraction
- Consistent with existing reference patterns
- Extensible for future needs without breaking changes

### Future Capability
Since complete objects are stored, future phases could:
- Extract other fields (name, endpoint, labels)
- Use JQ-like syntax for complex field access
- Add more type mappings as needed

## ADR-007: Resolution Timing

### Status
Accepted

### Decision
Resolve ALL external resources during planning phase, following existing pattern:
1. Parse external_resources blocks first (before other resources)
2. Resolve in dependency order (parents before children)
3. Execute selectors via SDK to fetch complete resource data
4. Store resolved data in memory for the duration of the planning/execution
5. Populate PlannedChange.References with resolved IDs
6. Fail immediately if any external resource cannot be resolved

### Implementation Pattern
Follows existing two-phase resolution used for API publications:
- **Planning phase**: External resources fully resolved, IDs available
- **Execution phase**: Uses pre-resolved IDs, no additional lookups needed

### Rationale
- Consistent with existing reference resolution pattern
- Fail fast on missing/ambiguous external resources
- No runtime surprises during execution
- Clear error messages guide users to fix selectors
- Simplifies executor implementation (IDs always available)

## ADR-008: Caching Strategy

### Status
Accepted

### Decision
- No persistent caching between CLI invocations
- Resolved external resource data stored in memory during planning/execution only
- Each CLI command fetches fresh data from Konnect

### Rationale
- Avoids premature optimization
- Consistent with current CLI behavior (fetches data each operation)
- Prevents stale data issues
- Simplifies implementation
- Can add caching in future if performance requires

## ADR-009: No Provider Attribution

### Status
Accepted

### Decision
Do not include provider/tool attribution fields

### Rationale
- External resources are tool-agnostic
- Reduces complexity
- Avoids coupling to specific tools
- Focus on resource identity, not management tool

## ADR-010: Namespace Handling

### Status
Accepted

### Decision
- External resources are namespace-agnostic
- No namespace field on external_resources
- Can be referenced from any namespace
- Not included in namespace filtering operations

### Rationale
- Namespaces identify sets of "managed" resources
- External resources are by definition not managed
- External resources are shared references available to all namespaces
- Simplifies cross-team resource sharing

## ADR-011: Plan Output Representation

### Status
Accepted

### Decision
Display external resources in a separate section of plan output:
```
External Resources (read-only):
  ✓ control_plane "prod-cp" (id: abc-123...)
  ✓ ce_service "user-service" (id: def-456...)

Changes to apply:
  + api_implementation "impl"
```

### Rationale
- Clear distinction between external and managed resources
- Shows successful resolution status
- Provides visibility into external dependencies

## ADR-012: SDK Error Handling

### Status
Accepted

### Decision
- Fail fast on any SDK errors during external resource resolution
- Provide clear error messages for:
  - Network failures
  - Authentication errors
  - Rate limiting
  - API errors

### Rationale
- External resources are critical dependencies
- Better to fail early with clear errors
- Users must fix connectivity/auth before proceeding

## ADR-013: Dry Run Behavior

### Status
Accepted

### Decision
- External resources are resolved during dry-run operations
- Validates configuration without making changes
- Same resolution behavior as normal planning

### Rationale
- Allows validation of external references
- Catches configuration errors early
- Consistent behavior across modes

## ADR-014: Complex Selectors (Future)

### Status
Proposed (Phase 2)

### Options Considered
1. **matchExpressions with operators**
   ```yaml
   matchExpressions:
     name:
       operator: StartsWith
       value: "api-"
   ```

2. **JQ filter expressions**
   ```yaml
   selector:
     jq: '.[] | select(.name | startswith("api-"))'
   ```

3. **JSONPath syntax**
   ```yaml
   selector:
     jsonpath: '$[?(@.name =~ /^api-/)]'
   ```

### Recommendation
Option 1 (matchExpressions) for Phase 2:
- Progressive complexity from matchFields
- Structured and validatable
- Better IDE support
- Consistent with Kubernetes patterns

## ADR-015: Core Entity Resource Naming

### Status
Accepted

### Decision
Use `ce_` prefix for Gateway core entity resource types:
- `ce_service` - Gateway service
- `ce_route` - Gateway route
- `ce_consumer` - Gateway consumer
- `ce_plugin` - Gateway plugin
- `ce_upstream` - Gateway upstream
- `ce_target` - Gateway target

### Rationale
- Maps directly to API path structure (`/core-entities/`)
- Consistent pattern across all core entities
- Distinguishes from Service Catalog "service" resource
- Technical alignment with Konnect API structure
- Acronym concern addressed through documentation and error messages

### Mitigation for Learning Curve
- Clear error messages: "Unknown resource type 'service'. Did you mean 'ce_service' (Gateway core entity)?"
- Documentation explains convention once
- Consistent usage across all examples

## ADR-016: Parent Validation

### Status
Accepted

### Decision
Validate parent field requirements at configuration parse time:
- Validate before any SDK calls
- Check required parent fields based on resource type
- Validate parent resource type compatibility

### Validation Rules
```yaml
# Required parent fields by resource type:
ce_service: requires control_plane
ce_route: requires control_plane (service parent optional)
ce_plugin: requires control_plane (service/route parent optional)
ce_consumer: requires control_plane
ce_upstream: requires control_plane
ce_target: requires control_plane and ce_upstream
```

### Error Examples
```
Error: Invalid external resource configuration
  Resource: "user-service" (type: ce_service)
  Missing required parent field: control_plane
  
Error: Invalid external resource configuration  
  Resource: "my-route" (type: ce_route)
  Invalid parent type: expected 'control_plane', got 'portal'
```

### Rationale
- Immediate feedback on configuration errors
- Prevents invalid SDK calls
- Consistent with existing validation patterns
- Better user experience with early error detection

## ADR-017: Phase 1 Resource Types

### Status
Accepted

### Decision
Phase 1 supports only essential resource types for api_implementation:
- `control_plane` - Kong Gateway control plane
- `ce_service` - Gateway service (core entity)

### Rationale
- Minimal scope for initial implementation
- Covers immediate use case (API implementations)
- Proves the pattern before expanding
- Additional types can be added incrementally

## ADR-018: Testing Strategy

### Status
Accepted

### Decision
- Unit tests with mocked SDK responses
- Integration tests with real Konnect resources
- Manual validation for edge cases

### Rationale
- Unit tests ensure logic correctness
- Integration tests validate SDK integration
- Manual testing catches user experience issues