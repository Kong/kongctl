# Stage 2: Plan Generation with Label Management - Technical Overview

## Overview

Stage 2 implements the plan generation functionality that compares current Konnect state 
with desired declarative configuration, producing an execution plan with CREATE and UPDATE 
operations. This stage introduces label management for tracking kongctl-managed resources 
and implements change detection through configuration hashing.

## Key Components

### 1. Label Management System

Labels are used to identify and track kongctl-managed resources in Konnect:

- **KONGCTL/managed**: Identifies resources managed by kongctl
- **KONGCTL/config-hash**: Stores hash of resource configuration for drift detection
- **KONGCTL/last-updated**: Tracks when resource was last modified by kongctl
- **KONGCTL/protected**: Prevents accidental deletion of critical resources

User-provided labels are preserved and merged with KONGCTL labels.

### 2. Extended API Interfaces

Multiple existing API interfaces need to be extended to support full CRUD operations:

```go
// PortalAPI interface extensions
type PortalAPI interface {
    // Existing operations
    ListPortals(ctx context.Context, request kkInternalOps.ListPortalsRequest) (*kkInternalOps.ListPortalsResponse, error)
    GetPortal(ctx context.Context, id string) (*kkInternalOps.GetPortalResponse, error)
    
    // New operations for Stage 2
    CreatePortal(ctx context.Context, portal kkInternalComps.CreatePortal) (*kkInternalOps.CreatePortalResponse, error)
    UpdatePortal(ctx context.Context, id string, portal kkInternalComps.UpdatePortal) (*kkInternalOps.UpdatePortalResponse, error)
}

// AppAuthStrategiesAPI interface extensions
type AppAuthStrategiesAPI interface {
    // Existing
    ListAppAuthStrategies(ctx context.Context, request kkOps.ListAppAuthStrategiesRequest) (*kkOps.ListAppAuthStrategiesResponse, error)
    
    // New operations for Stage 2
    GetAppAuthStrategy(ctx context.Context, id string) (*kkOps.GetAppAuthStrategyResponse, error)
    CreateAppAuthStrategy(ctx context.Context, strategy kkComps.CreateApplicationAuthStrategy) (*kkOps.CreateAppAuthStrategyResponse, error)
    UpdateAppAuthStrategy(ctx context.Context, id string, strategy kkComps.UpdateApplicationAuthStrategy) (*kkOps.UpdateAppAuthStrategyResponse, error)
}

// Additional interfaces (APIAPI, ControlPlaneAPI) will be extended as needed
```

### 3. Konnect State Client

A wrapper around the SDK that:
- Fetches all resources with pagination support
- Filters to only KONGCTL/managed resources client-side
- Normalizes label representations (pointer to non-pointer)
- Provides consistent error handling

### 4. Configuration Hash Strategy

The config hash enables fast drift detection by creating a deterministic hash of resource 
configuration:

- **Algorithm**: SHA256 for cryptographic strength and wide support
- **Included fields**: All user-configurable fields
- **Excluded fields**: System-generated fields (ID, timestamps), KONGCTL labels
- **Deterministic**: Sorted field ordering, normalized values
- **Purpose**: Quick comparison without field-by-field checks

### 5. Reference Resolution

References in declarative config (e.g., auth strategy names) are resolved to Konnect IDs 
during plan generation:

- Enables validation that referenced resources exist
- Detects if references have changed between plan and execution
- Speeds up plan execution by pre-resolving lookups
- Stores both ref and resolved ID in the `references` field for debugging
- Uses `<unknown>` placeholder for resources being created in the same plan
- Handles nested references with dot notation (e.g., `gateway_service.control_plane_id`)

### 6. Plan Structure

Plans are versioned JSON documents designed for minimal size and maximum clarity:

```json
{
  "metadata": {
    "version": "1.0",
    "generated_at": "2025-01-26T10:30:00Z",
    "generator": "kongctl/0.1.0"
  },
  "changes": [
    {
      "id": "1-c-oauth-strategy",
      "resource_type": "application_auth_strategy",
      "resource_ref": "oauth-strategy",
      "action": "CREATE",
      "fields": {
        "name": "OAuth 2.0 Strategy",
        "display_name": "OAuth 2.0",
        "strategy_type": "oauth2",
        "configs": { ... }
      },
      "config_hash": "sha256:abc123",
      "protection": true,
      "depends_on": []
    },
    {
      "id": "2-u-developer-portal",
      "resource_type": "portal",
      "resource_ref": "developer-portal",
      "resource_id": "2c5e8a7f-9b3d-4f6e-a1c8-7d5b2a8f3c9e",
      "action": "UPDATE",
      "fields": {
        "description": {
          "old": "Public portal",
          "new": "Public developer portal"
        },
        "default_application_auth_strategy_id": {
          "old": null,
          "new": "456e7890-1234-5678-9abc-def012345678"
        }
      },
      "references": {
        "default_application_auth_strategy_id": {
          "ref": "oauth-strategy",
          "id": "456e7890-1234-5678-9abc-def012345678"
        }
      },
      "config_hash": "sha256:def456",
      "depends_on": ["1-c-oauth-strategy"]
    },
    {
      "id": "3-c-payment-api-impl",
      "resource_type": "api_implementation",
      "resource_ref": "payment-api-impl",
      "parent": {
        "ref": "payment-api",
        "id": "<unknown>"
      },
      "action": "CREATE",
      "fields": {
        "implementation_type": "gateway_service",
        "implementation_url": "https://api.example.com/payments/v2",
        "gateway_service": {
          "service_id": "d125e0a1-b305-4ae2-9fa8-3a57f9df85e1",
          "control_plane_id": "<unknown>"
        }
      },
      "references": {
        "gateway_service.control_plane_id": {
          "ref": "prod-cp",
          "id": "<unknown>"
        }
      },
      "depends_on": ["4-c-payment-api", "5-c-prod-cp"]
    }
  ],
  "execution_order": ["1-c-oauth-strategy", "2-u-developer-portal", "3-c-payment-api-impl"],
  "summary": {
    "total_changes": 3,
    "by_action": {"CREATE": 2, "UPDATE": 1},
    "by_resource": {"application_auth_strategy": 1, "portal": 1, "api_implementation": 1}
  },
  "warnings": [
    {
      "change_id": "3-c-payment-api-impl",
      "message": "Parent and control plane references will be resolved during execution"
    }
  ]
}
```

#### Key Plan Features:

1. **Semantic Change IDs**: Format `{number}-{action}-{ref}` (e.g., "1-c-oauth-strategy")
   - Number: Sequential ordering
   - Action: c=create, u=update, d=delete
   - Ref: Resource reference for readability

2. **Minimal Field Storage**:
   - CREATE: Only fields being set
   - UPDATE: Only changed fields with old/new values
   - Reduces plan size by 50-70%

3. **Enhanced References**: 
   - Shows both original ref and resolved ID
   - Enables debugging and validation
   - Uses `<unknown>` for resources created in same plan

4. **Parent Relationships**:
   - For nested resources (API implementations, versions, etc.)
   - Includes both ref and ID for API URL construction

5. **Protection Management**:
   - Boolean for CREATE (protected from start)
   - Old/new object for UPDATE when protection changes
   - Protection changes must be isolated (no other field changes)

6. **Dependencies and Ordering**:
   - Explicit `depends_on` array per change
   - `execution_order` provides topological sort result
   - Handles complex multi-resource dependencies

7. **No Global Reference Mappings**:
   - Eliminated redundant top-level mappings
   - All reference data stored within changes

## Implementation Flow

### Plan Generation Process

1. **Load Configuration**: Read declarative YAML files using existing loader
2. **Fetch Current State**: Get all KONGCTL/managed resources from Konnect
3. **Resolve References**: Convert resource refs to Konnect IDs where possible
4. **Compare States**: For each desired resource:
   - Find matching current resource by name
   - If not found → CREATE action
   - If found and config hash differs → UPDATE action
   - If protection status changing → Separate UPDATE action
5. **Calculate Dependencies**: Build dependency graph based on references and parent relationships
6. **Generate Execution Order**: Topological sort of dependency graph
7. **Generate Plan**: Assemble changes with semantic IDs into plan document
8. **Add Warnings**: Include any execution-time considerations
9. **Save Plan**: Write to specified output file

### Diff Command Flow

1. **Load Plan**: Read plan from file or generate new one
2. **Format Output**: Display changes in human-readable or JSON format
3. **Show Summary**: Display counts by action and resource type

## Design Decisions

### Client-Side Filtering

Since SDK doesn't support label filtering, we fetch all resources and filter locally:
- Implement efficient pagination to handle large resource counts
- Cache results within command execution
- Future: Add server-side filtering when available

### Label Normalization

SDK inconsistency requires normalization:
- Convert `map[string]*string` to `map[string]string`
- Handle nil pointers gracefully
- Preserve user labels while adding KONGCTL labels

### Hash Calculation

Configuration hashing provides efficient change detection:
- Include only user-configurable fields
- Sort fields for deterministic output
- Use canonical JSON representation
- Store as base64-encoded string in label

### Plan Structure Rationale

The plan structure is designed to balance several concerns:

**Minimal Size**: 
- Only store changed fields for UPDATE operations
- No redundant data (removed global reference mappings)
- Reduces storage and transmission costs

**Debuggability**:
- Semantic change IDs show action and resource at a glance
- References field preserves original ref names alongside resolved IDs
- Clear parent relationships for nested resources

**Safety**:
- Protection changes isolated in separate changes
- Explicit dependencies prevent incorrect ordering
- Warnings for runtime considerations

**Execution Efficiency**:
- Pre-calculated execution order
- All needed IDs resolved (or marked as `<unknown>`)
- Direct mapping to SDK operations

## Testing Strategy

### Unit Tests
- Label manipulation functions
- Hash calculation with various inputs
- Reference resolution logic
- Plan generation scenarios

### Integration Tests
- Plan generation with mock Konnect responses
- Diff output formatting
- Plan serialization/deserialization
- Reference validation

## Future Considerations

### Stage 3 Dependencies
- Plan execution will consume these plans
- Plan validation before execution
- Rollback strategy using stored state

### Extensibility
- Plan format supports additional actions (DELETE)
- Resource dependencies can be added
- Dry-run execution mode