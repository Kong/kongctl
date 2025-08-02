# Stage 1 Execution Plan: Configuration Format & Basic CLI

## Overview

This document outlines the detailed execution plan for implementing Stage 1 of the declarative configuration feature for kongctl. The implementation follows the requirements in `001-dec-cfg-cfg-format-basic-cli.md` with careful attention to maintaining a compilable, testable codebase at each step.

## Goals

1. Establish YAML configuration format using SDK types
2. Add declarative command stubs to kongctl
3. Implement configuration loading and validation
4. Create foundation for future stages

## Key Design Decisions

### 1. Resource Wrapper Pattern

We wrap SDK types rather than duplicating them to:
- Avoid maintenance burden of keeping types in sync
- Add declarative-specific fields (name for references, kongctl metadata)
- Maintain clear separation between API representation and declarative configuration

### 2. Resource Reference Handling

Most Konnect resources have both `ID` (UUID) and `Name` fields in the SDK. Our approach:
- Use a separate `ref` field for cross-resource references
- SDK's `ID` field: UUID assigned by Konnect (never in declarative config)
- SDK's `Name` field: Human-friendly display name (can have spaces)
- Our `ref` field: Computer-friendly reference identifier
  ```yaml
  portals:
    - ref: dev-portal                    # Used for references
      name: "Developer Portal (Production)"  # Display name in Konnect
      # ID field populated by Konnect (UUID)
  ```

### 3. Package Structure

```
internal/
├── cmd/root/verbs/         # Command implementations
│   ├── plan/
│   ├── apply/
│   ├── sync/
│   ├── diff/
│   └── export/
└── declarative/
    ├── resources/          # Resource type definitions
    │   ├── types.go        # Core types (ResourceSet, KongctlMeta)
    │   └── portal.go       # Portal resource wrapper
    └── loader/             # Configuration loading
        └── loader.go       # YAML file loading and parsing
```

### 4. Explicit ResourceSet

Rather than a generic container, ResourceSet explicitly lists supported resource types:
```go
type ResourceSet struct {
    Portals []PortalResource `yaml:"portals,omitempty"`
    // Future: Teams, ApplicationAuthStrategies, etc.
}
```

Benefits:
- Type safety and IDE support
- Clear documentation of supported resources
- Easy validation and processing

### 5. Reference Pattern Examples

Our per-resource reference system handles complex scenarios found in the Konnect API:

```yaml
# Top-level control planes
control_planes:
  - ref: prod-cp
    name: "Production Control Plane"
    cluster_type: "cluster_type_hybrid"

# Top-level services (with control plane reference)
services:
  - ref: users-service
    control_plane_id: prod-cp      # References control plane
    name: "Users Service"
    url: "http://users.internal"

# Simple references
application_auth_strategies:
  - ref: oauth-strategy
    name: "OAuth 2.0 Strategy"
    auth_type: openid_connect
    configs:
      issuer: "https://auth.example.com"

# Portal referencing auth strategy
portals:
  - ref: dev-portal
    name: "Developer Portal"
    default_application_auth_strategy_id: oauth-strategy  # References auth strategy

# APIs with nested child resources (following API endpoint structure)
apis:
  - ref: users-api
    name: "Users API"
    description: "User management API"
    
    # Nested API publications (child of API)
    publications:
      - ref: users-api-dev-publication
        portal_id: dev-portal              # References portal
        auth_strategy_ids:                 # Array of auth strategy references
          - oauth-strategy
          - key-auth-strategy
        visibility: public
        
    # Nested API versions (child of API)
    versions:
      - ref: users-api-v1
        version: "1.0.0"
        spec_content: |
          openapi: 3.0.0
          info:
            title: Users API
            version: 1.0.0
            
    # Nested API implementations (child of API)
    # Uses qualified field names to resolve ambiguity
    implementations:
      - ref: users-api-impl
        service:
          control_plane_id: prod-cp      # References control plane
          id: users-service              # External UUID (managed by decK)
```

Each resource type implements `ReferenceMapping` interface to define its own reference semantics, eliminating ambiguity and making validation self-contained.

### 6. Test Strategy

Following test-first approach for:
- Business logic (validation, merging, reference resolution)
- Integration points (command execution, file loading)
- Error handling and edge cases
- Cross-resource reference validation

Not testing:
- SDK functionality (already tested)
- YAML marshaling/unmarshaling (library functionality)
- Simple getters/setters

## Implementation Steps

Each step results in a working, compilable project with comprehensive tests where appropriate.

### Step 1: Add Verb Constants
**File**: `internal/cmd/root/verbs/verbs.go`
- Add Plan, Sync, Diff, Export constants
- Maintains consistency with existing verb pattern

### Step 2: Create Command Stubs
**Files**: New command files in `internal/cmd/root/verbs/`
- Each command returns "not yet implemented"
- Follows existing command structure pattern
- Registered with root command

### Step 3: Define Core Types
**File**: `internal/declarative/resources/types.go`
- ResourceSet struct (container for all resources)
- KongctlMeta struct (tool-specific metadata)
- Common interfaces if needed

### Step 4: Define Portal Resource
**File**: `internal/declarative/resources/portal.go`
- PortalResource wrapper type
- Embeds SDK's CreatePortal type
- Adds declarative name and kongctl metadata

### Step 5: Implement YAML Loader
**File**: `internal/declarative/loader/loader.go`
- Single file loading
- YAML parsing with validation
- Name uniqueness checking

### Step 6: Add Multi-file Support
**File**: `internal/declarative/loader/loader.go`
- Directory traversal for .yaml/.yml files
- Resource merging from multiple files
- Conflict detection

### Step 7: Integrate with Plan Command
**File**: `internal/cmd/root/verbs/plan/plan.go`
- Connect loader to plan command
- Display summary of loaded resources
- Error handling and user feedback

## Success Criteria

1. All commands are registered and accessible via CLI
2. YAML configuration files can be loaded and validated
3. Portal resources are properly parsed with SDK types
4. Name uniqueness is enforced
5. Multi-file configurations work correctly
6. Clear error messages for common issues
7. All tests pass and coverage is appropriate

## Future Considerations

This foundation supports:
- Additional resource types (teams, auth strategies)
- Reference resolution between resources
- Plan generation and execution (Stage 2)
- Label management and drift detection (Stage 3)