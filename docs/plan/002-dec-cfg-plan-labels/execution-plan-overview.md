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

### 2. Extended PortalAPI Interface

The existing `PortalAPI` interface will be extended to support full CRUD operations:

```go
type PortalAPI interface {
    // Existing operations
    ListPortals(ctx context.Context, request kkInternalOps.ListPortalsRequest) (*kkInternalOps.ListPortalsResponse, error)
    GetPortal(ctx context.Context, id string) (*kkInternalOps.GetPortalResponse, error)
    
    // New operations for Stage 2
    CreatePortal(ctx context.Context, portal kkInternalComps.CreatePortal) (*kkInternalOps.CreatePortalResponse, error)
    UpdatePortal(ctx context.Context, id string, portal kkInternalComps.UpdatePortal) (*kkInternalOps.UpdatePortalResponse, error)
}
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
- Maintains mapping of ref → ID for all resource types

### 6. Plan Structure

Plans are versioned JSON documents containing:

```json
{
  "metadata": {
    "version": "1.0",
    "generated_at": "2024-01-20T10:30:00Z",
    "generator": "kongctl/v0.1.0"
  },
  "reference_mappings": {
    "application_auth_strategy": {
      "oauth-strategy": "uuid-1234"
    }
  },
  "changes": [
    {
      "id": "change-001",
      "resource_type": "portal",
      "resource_ref": "developer-portal",
      "resource_name": "Developer Portal",
      "action": "CREATE",
      "desired_state": { ... }
    }
  ],
  "summary": {
    "total_changes": 1,
    "by_action": {"CREATE": 1, "UPDATE": 0},
    "by_resource": {"portal": 1}
  }
}
```

## Implementation Flow

### Plan Generation Process

1. **Load Configuration**: Read declarative YAML files using existing loader
2. **Fetch Current State**: Get all KONGCTL/managed resources from Konnect
3. **Resolve References**: Convert resource refs to Konnect IDs
4. **Compare States**: For each desired resource:
   - Find matching current resource by name
   - If not found → CREATE action
   - If found and config hash differs → UPDATE action
5. **Generate Plan**: Assemble changes into plan document
6. **Save Plan**: Write to specified output file

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