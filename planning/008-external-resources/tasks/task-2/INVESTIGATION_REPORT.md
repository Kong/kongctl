# External Resources Schema and Configuration Investigation Report

## Overview

This report provides a comprehensive analysis of the kongctl codebase to understand the 
current resource configuration architecture and identify what needs to be implemented for 
Stage 8 Step 1: Schema and Configuration for external resources.

## Step 1 Requirements (from execution-plan-steps.md)

- Define external_resources schema in configuration types
- Add validation for external resource blocks
- Support both direct ID and selector patterns
- Add parent field support for hierarchical resources

## Current State Analysis

### 1. Configuration Type System Architecture

**Location**: `internal/declarative/resources/types.go`

The current system uses a centralized `ResourceSet` struct that contains all supported resources:

```go
type ResourceSet struct {
    Portals []PortalResource `yaml:"portals,omitempty" json:"portals,omitempty"`
    ApplicationAuthStrategies []ApplicationAuthStrategyResource `yaml:"application_auth_strategies,omitempty" json:"application_auth_strategies,omitempty"`
    ControlPlanes []ControlPlaneResource `yaml:"control_planes,omitempty" json:"control_planes,omitempty"`
    APIs []APIResource `yaml:"apis,omitempty" json:"apis,omitempty"`
    // ... other resources
}
```

**Key Insights:**
- Resources are defined as slice fields with YAML/JSON tags
- Each resource type has its own Go struct
- Common metadata patterns via `KongctlMeta` struct
- Validation via `ResourceValidator` interface

### 2. Resource Structure Patterns

**Location**: `internal/declarative/resources/*.go`

Each resource follows a consistent pattern (e.g., `APIResource`):

```go
type APIResource struct {
    kkComps.CreateAPIRequest `yaml:",inline" json:",inline"`  // SDK integration
    Ref     string       `yaml:"ref" json:"ref"`                // Declarative reference
    Kongctl *KongctlMeta `yaml:"kongctl,omitempty" json:"kongctl,omitempty"` // Tool metadata
    
    // Nested child resources
    Versions        []APIVersionResource        `yaml:"versions,omitempty" json:"versions,omitempty"`
    Publications    []APIPublicationResource    `yaml:"publications,omitempty" json:"publications,omitempty"`
    
    // Runtime state (not serialized)
    konnectID string `yaml:"-" json:"-"`
}
```

**Key Patterns:**
- SDK integration via inline embedding of SDK structs
- Declarative `ref` field for cross-resource references
- Nested child resources support
- Runtime state separation
- Common interfaces: `Resource`, `ResourceWithParent`, `ResourceWithLabels`

### 3. Validation System

**Location**: `internal/declarative/resources/validation.go`

Current validation includes:
- Ref pattern validation (alphanumeric, hyphens, underscores)
- Length constraints (1-63 characters)
- Reserved character restrictions (no colons, spaces)
- Interface-based validation via `ResourceValidator`

**Validation Pattern:**
```go
func (a APIResource) Validate() error {
    if err := ValidateRef(a.Ref); err != nil {
        return fmt.Errorf("invalid API ref: %w", err)
    }
    return nil
}
```

### 4. Reference Resolution System

**Location**: `internal/declarative/planner/resolver.go`

Current reference resolution:
- Resolves declarative refs to Konnect IDs during planning
- Supports both existing resources and resources being created in same plan
- Field-based detection of reference patterns
- Resource type mapping for different field names

**Resolution Pattern:**
```go
type ReferenceResolver struct {
    client *state.Client
}

func (r *ReferenceResolver) ResolveReferences(ctx context.Context, changes []PlannedChange) (*ResolveResult, error)
```

### 5. SDK Integration Patterns

**Location**: `internal/declarative/state/client.go`

The system integrates with Konnect SDK through:
- API client abstraction in `ClientConfig`
- Normalized resource structs that wrap SDK types
- Filtering for managed resources via labels
- Pagination support
- Error handling with context

**SDK Integration Pattern:**
```go
type Client struct {
    portalAPI  helpers.PortalAPI
    apiAPI     helpers.APIAPI
    // ... other APIs
}

func (c *Client) ListManagedPortals(ctx context.Context, namespaces []string) ([]Portal, error)
```

### 6. Configuration Loading System

**Location**: `internal/declarative/tags/file.go`

Current configuration loading supports:
- YAML/JSON file parsing
- External file references via `!file` tags
- Content extraction from external files
- Caching for performance

## Implementation Requirements for External Resources

### 1. Schema Definition in ResourceSet

**Required Changes**: Add external_resources field to `ResourceSet`:

```go
type ResourceSet struct {
    // ... existing fields
    ExternalResources []ExternalResourceResource `yaml:"external_resources,omitempty" json:"external_resources,omitempty"`
}
```

### 2. ExternalResourceResource Struct

**New File**: `internal/declarative/resources/external_resource.go`

**Required Structure**:
```go
type ExternalResourceResource struct {
    Ref     string       `yaml:"ref" json:"ref"`
    Kongctl *KongctlMeta `yaml:"kongctl,omitempty" json:"kongctl,omitempty"`
    
    // Resource identification
    ResourceType string `yaml:"resource_type" json:"resource_type"`
    
    // Direct ID pattern
    ID *string `yaml:"id,omitempty" json:"id,omitempty"`
    
    // Selector pattern
    Selector *ExternalResourceSelector `yaml:"selector,omitempty" json:"selector,omitempty"`
    
    // Parent field for hierarchical resources
    Parent *ExternalResourceParent `yaml:"parent,omitempty" json:"parent,omitempty"`
    
    // Runtime state
    resolvedID string `yaml:"-" json:"-"`
    resolvedResource interface{} `yaml:"-" json:"-"`
}

type ExternalResourceSelector struct {
    MatchFields map[string]string `yaml:"match_fields" json:"match_fields"`
}

type ExternalResourceParent struct {
    ResourceType string `yaml:"resource_type" json:"resource_type"`
    ID           string `yaml:"id" json:"id"`
}
```

### 3. Validation Requirements

**Required Validations**:
- Exactly one of `id` or `selector` must be specified
- Valid `resource_type` values
- Selector `match_fields` validation
- Parent relationship validation

### 4. Resource Type Registry

**New Component**: Resource type registry to map resource types to:
- Supported field names for selectors
- Parent-child relationships  
- SDK operation mappings
- Validation rules

### 5. Integration Points

**Planner Integration**: 
- Pre-resolution phase to resolve external resources before planning
- Integration with existing `ReferenceResolver`
- Dependency graph building

**State Client Integration**:
- New methods for external resource queries
- Generic query interface for different resource types
- Error handling for resolution failures

## Potential Challenges

### 1. Resource Type Extensibility
**Challenge**: Supporting new resource types without code changes
**Approach**: Registry-based system with configuration-driven mapping

### 2. Parent-Child Resolution Order
**Challenge**: Resolving hierarchical resources in correct dependency order
**Approach**: Dependency graph analysis and topological sorting

### 3. SDK Query Standardization
**Challenge**: Different SDK APIs have different query patterns
**Approach**: Adapter pattern with resource-specific query implementations

### 4. Error Handling Complexity
**Challenge**: Clear error messages for resolution failures
**Approach**: Structured error types with context information

### 5. Performance Impact
**Challenge**: Multiple SDK calls for resolution
**Approach**: Caching and batch operations where possible

## Key Files to Modify

### Core Schema Files
1. `internal/declarative/resources/types.go` - Add external_resources field
2. `internal/declarative/resources/external_resource.go` - New resource type
3. `internal/declarative/resources/validation.go` - Add validation functions

### Integration Files  
4. `internal/declarative/planner/planner.go` - Add external resource resolution
5. `internal/declarative/planner/resolver.go` - Extend reference resolution
6. `internal/declarative/state/client.go` - Add external resource queries

### New Components
7. `internal/declarative/external/registry.go` - Resource type registry
8. `internal/declarative/external/resolver.go` - External resource resolver
9. `internal/declarative/external/types.go` - External resource type definitions

## Existing Patterns to Follow

### Resource Definition Pattern
- Embed SDK types where applicable
- Use `ref` field for declarative references
- Include `Kongctl` metadata field
- Implement common interfaces (`Resource`, `ResourceValidator`)

### Validation Pattern  
- Interface-based validation with `Validate() error`
- Centralized validation functions for common patterns
- Clear, actionable error messages

### SDK Integration Pattern
- Client abstraction with interface-based APIs
- Normalized resource structs wrapping SDK types
- Pagination support for list operations
- Enhanced error handling with context

### Reference Resolution Pattern
- Context-aware resolution during planning phase
- Support for both existing and creating resources
- Field-based reference detection
- Caching for performance

## Conclusion

The kongctl codebase has a well-structured architecture for resource management that can be 
extended to support external resources. The key implementation points are:

1. **Schema Extension**: Add external_resources to ResourceSet following existing patterns
2. **Validation Framework**: Leverage existing validation interfaces and patterns
3. **Resolution Integration**: Extend current reference resolution system
4. **SDK Integration**: Follow established client and query patterns

The implementation should maintain consistency with existing patterns while providing the
flexibility needed for external resource resolution across different resource types and 
relationship hierarchies.