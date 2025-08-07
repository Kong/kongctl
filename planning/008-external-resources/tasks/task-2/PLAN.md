# External Resources Schema and Configuration Implementation Plan

## Executive Summary

This plan implements Stage 8 Step 1: Schema and Configuration for external resources in kongctl. The implementation adds support for referencing existing Konnect resources through both direct IDs and selector patterns, with hierarchical parent-child relationships. This foundational step enables external resource resolution that will be used by subsequent stages for cross-resource references and dependency management.

**Core Deliverables:**
- External resource schema definition in ResourceSet
- Validation framework for external resource blocks
- Support for direct ID and selector patterns  
- Parent field support for hierarchical resources
- Extensible resource type registry
- Complete test coverage for all new functionality

## Implementation Phases

### Phase 1: Core Schema and Types Foundation

This phase establishes the fundamental data structures without breaking existing functionality.

#### 1.1 Modify ResourceSet Structure

**File:** `internal/declarative/resources/types.go`

**Change:** Add external_resources field to ResourceSet struct

```go
// BEFORE
type ResourceSet struct {
    Portals                   []PortalResource                   `yaml:"portals,omitempty" json:"portals,omitempty"`
    ApplicationAuthStrategies []ApplicationAuthStrategyResource `yaml:"application_auth_strategies,omitempty" json:"application_auth_strategies,omitempty"`
    ControlPlanes             []ControlPlaneResource             `yaml:"control_planes,omitempty" json:"control_planes,omitempty"`
    APIs                      []APIResource                      `yaml:"apis,omitempty" json:"apis,omitempty"`
    // ... other resources
}

// AFTER  
type ResourceSet struct {
    ExternalResources         []ExternalResourceResource         `yaml:"external_resources,omitempty" json:"external_resources,omitempty"` // NEW
    Portals                   []PortalResource                   `yaml:"portals,omitempty" json:"portals,omitempty"`
    ApplicationAuthStrategies []ApplicationAuthStrategyResource `yaml:"application_auth_strategies,omitempty" json:"application_auth_strategies,omitempty"`
    ControlPlanes             []ControlPlaneResource             `yaml:"control_planes,omitempty" json:"control_planes,omitempty"`
    APIs                      []APIResource                      `yaml:"apis,omitempty" json:"apis,omitempty"`
    // ... other resources
}
```

#### 1.2 Create ExternalResourceResource Struct

**File:** `internal/declarative/resources/external_resource.go` (NEW FILE)

**Content:** Complete external resource type definition

```go
package resources

import (
    "fmt"
    "strings"
)

// ExternalResourceResource represents a reference to an existing resource in Konnect
// that is not managed by this configuration but needs to be referenced by managed resources.
// Note: External resources do not have Kongctl metadata as they cannot be protected
// or namespaced (they are owned by an external system).
type ExternalResourceResource struct {
    // Declarative reference identifier
    Ref string `yaml:"ref" json:"ref"`
    
    // Resource type identifier (e.g., "portal", "api", "control_plane")
    ResourceType string `yaml:"resource_type" json:"resource_type"`
    
    // Direct ID specification (mutually exclusive with Selector)
    ID *string `yaml:"id,omitempty" json:"id,omitempty"`
    
    // Selector-based specification (mutually exclusive with ID)
    Selector *ExternalResourceSelector `yaml:"selector,omitempty" json:"selector,omitempty"`
    
    // Parent resource for hierarchical resources
    Parent *ExternalResourceParent `yaml:"parent,omitempty" json:"parent,omitempty"`
    
    // Runtime state (not serialized to YAML/JSON)
    resolvedID       string      `yaml:"-" json:"-"`
    resolvedResource interface{} `yaml:"-" json:"-"`
    resolved         bool        `yaml:"-" json:"-"`
}

// ExternalResourceSelector defines criteria for finding a resource by field matching
type ExternalResourceSelector struct {
    // Map of field names to expected values for matching
    MatchFields map[string]string `yaml:"match_fields" json:"match_fields"`
}

// ExternalResourceParent defines a parent resource for hierarchical resolution
type ExternalResourceParent struct {
    // Parent resource type
    ResourceType string `yaml:"resource_type" json:"resource_type"`
    
    // Parent resource ID (must be resolved before child)
    ID string `yaml:"id,omitempty" json:"id,omitempty"`
    
    // Alternative: reference to another external resource
    Ref string `yaml:"ref,omitempty" json:"ref,omitempty"`
}

// Interface implementations

// GetRef returns the declarative reference identifier
func (e ExternalResourceResource) GetRef() string {
    return e.Ref
}

// GetResourceType returns the resource type
func (e ExternalResourceResource) GetResourceType() string {
    return e.ResourceType
}

// Validate implements ResourceValidator interface
func (e ExternalResourceResource) Validate() error {
    // Validate ref field using common validation
    if err := ValidateRef(e.Ref); err != nil {
        return fmt.Errorf("invalid external resource ref: %w", err)
    }
    
    // Validate resource type
    if err := ValidateResourceType(e.ResourceType); err != nil {
        return fmt.Errorf("invalid resource_type in external resource %q: %w", e.Ref, err)
    }
    
    // Validate ID XOR Selector requirement
    if err := ValidateIDXORSelector(e.ID, e.Selector); err != nil {
        return fmt.Errorf("invalid external resource %q: %w", e.Ref, err)
    }
    
    // Validate selector if present
    if e.Selector != nil {
        if err := ValidateSelector(e.ResourceType, e.Selector); err != nil {
            return fmt.Errorf("invalid selector in external resource %q: %w", e.Ref, err)
        }
    }
    
    // Validate parent if present
    if e.Parent != nil {
        if err := ValidateParent(e.ResourceType, e.Parent); err != nil {
            return fmt.Errorf("invalid parent in external resource %q: %w", e.Ref, err)
        }
    }
    
    return nil
}

// Runtime state methods

// SetResolvedID sets the resolved Konnect ID
func (e *ExternalResourceResource) SetResolvedID(id string) {
    e.resolvedID = id
    e.resolved = true
}

// GetResolvedID returns the resolved Konnect ID
func (e *ExternalResourceResource) GetResolvedID() string {
    return e.resolvedID
}

// SetResolvedResource sets the resolved resource object
func (e *ExternalResourceResource) SetResolvedResource(resource interface{}) {
    e.resolvedResource = resource
}

// GetResolvedResource returns the resolved resource object
func (e *ExternalResourceResource) GetResolvedResource() interface{} {
    return e.resolvedResource
}

// IsResolved returns whether this external resource has been resolved
func (e *ExternalResourceResource) IsResolved() bool {
    return e.resolved
}
```

### Phase 2: Validation Framework

This phase implements comprehensive validation for external resource configurations.

#### 2.1 Add Validation Functions

**File:** `internal/declarative/resources/validation.go`

**Changes:** Add validation functions for external resources

```go
// Add these functions to existing validation.go file

// ValidateResourceType validates that the resource type is supported
func ValidateResourceType(resourceType string) error {
    if resourceType == "" {
        return fmt.Errorf("resource_type is required")
    }
    
    // Get supported resource types from resolution registry
    registry := external.GetResolutionRegistry()
    if !registry.IsSupported(resourceType) {
        supported := registry.GetSupportedTypes()
        return fmt.Errorf("unsupported resource_type %q, supported types: %s", 
            resourceType, strings.Join(supported, ", "))
    }
    
    return nil
}

// ValidateIDXORSelector validates that exactly one of ID or Selector is specified
func ValidateIDXORSelector(id *string, selector *ExternalResourceSelector) error {
    hasID := id != nil && *id != ""
    hasSelector := selector != nil && len(selector.MatchFields) > 0
    
    if !hasID && !hasSelector {
        return fmt.Errorf("either 'id' or 'selector' must be specified")
    }
    
    if hasID && hasSelector {
        return fmt.Errorf("'id' and 'selector' are mutually exclusive, specify only one")
    }
    
    return nil
}

// ValidateSelector validates selector configuration for the given resource type
func ValidateSelector(resourceType string, selector *ExternalResourceSelector) error {
    if selector == nil {
        return fmt.Errorf("selector cannot be nil")
    }
    
    if len(selector.MatchFields) == 0 {
        return fmt.Errorf("selector.match_fields cannot be empty")
    }
    
    // Get supported fields from resolution registry
    registry := external.GetResolutionRegistry()
    supportedFields := registry.GetSupportedSelectorFields(resourceType)
    
    if supportedFields == nil {
        return fmt.Errorf("no supported selector fields defined for resource_type %q", resourceType)
    }
    
    for field := range selector.MatchFields {
        if !contains(supportedFields, field) {
            return fmt.Errorf("field %q is not supported for selector on resource_type %q, supported fields: %s",
                field, resourceType, strings.Join(supportedFields, ", "))
        }
    }
    
    return nil
}

// ValidateParent validates parent resource configuration
func ValidateParent(childResourceType string, parent *ExternalResourceParent) error {
    if parent == nil {
        return fmt.Errorf("parent cannot be nil")
    }
    
    // Validate parent resource type
    if err := ValidateResourceType(parent.ResourceType); err != nil {
        return fmt.Errorf("invalid parent resource_type: %w", err)
    }
    
    // Validate that exactly one of ID or Ref is specified
    hasID := parent.ID != ""
    hasRef := parent.Ref != ""
    
    if !hasID && !hasRef {
        return fmt.Errorf("parent must specify either 'id' or 'ref'")
    }
    
    if hasID && hasRef {
        return fmt.Errorf("parent 'id' and 'ref' are mutually exclusive")
    }
    
    // Validate parent-child relationship using resolution registry
    registry := external.GetResolutionRegistry()
    if !registry.IsValidParentChild(parent.ResourceType, childResourceType) {
        return fmt.Errorf("resource_type %q cannot have parent of type %q",
            childResourceType, parent.ResourceType)
    }
    
    return nil
}

// Helper function
func contains(slice []string, item string) bool {
    for _, s := range slice {
        if s == item {
            return true
        }
    }
    return false
}
```

### Phase 3: Supporting Infrastructure

This phase creates the external resource registry and supporting components.

#### 3.1 Create External Resource Type Registry

**File:** `internal/declarative/external/types.go` (NEW FILE)

```go
package external

import (
    "context"
)

// ResolutionMetadata contains metadata needed to resolve external resources from Konnect
type ResolutionMetadata struct {
    // Human-readable name
    Name string
    
    // Supported fields for selector matching
    SelectorFields []string
    
    // Supported parent resource types
    SupportedParents []string
    
    // Supported child resource types  
    SupportedChildren []string
    
    // Adapter for resolving resources via SDK
    ResolutionAdapter ResolutionAdapter
}

// ResolutionAdapter defines the interface for resolving external resources via SDK
type ResolutionAdapter interface {
    // GetByID retrieves a resource by its Konnect ID
    GetByID(ctx context.Context, id string, parent *ResolvedParent) (interface{}, error)
    
    // GetBySelector retrieves resources matching selector criteria
    GetBySelector(ctx context.Context, selector map[string]string, parent *ResolvedParent) ([]interface{}, error)
}

// ResolvedParent contains information about a resolved parent resource
type ResolvedParent struct {
    ResourceType string
    ID           string
    Resource     interface{}
}
```

**File:** `internal/declarative/external/registry.go` (NEW FILE)

```go
package external

import (
    "fmt"
    "sync"
)

// ResolutionRegistry manages resolution metadata for external resource types
type ResolutionRegistry struct {
    mu    sync.RWMutex
    types map[string]*ResolutionMetadata
}

var (
    registry     *ResolutionRegistry
    registryOnce sync.Once
)

// GetResolutionRegistry returns the singleton registry instance
func GetResolutionRegistry() *ResolutionRegistry {
    registryOnce.Do(func() {
        registry = &ResolutionRegistry{
            types: make(map[string]*ResolutionMetadata),
        }
        // Initialize with built-in resource types
        registry.initializeBuiltinTypes()
    })
    return registry
}

// Register adds resolution metadata for a resource type to the registry
func (r *ResolutionRegistry) Register(resourceType string, info *ResolutionMetadata) {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.types[resourceType] = info
}

// IsSupported returns true if the resource type is supported for resolution
func (r *ResolutionRegistry) IsSupported(resourceType string) bool {
    r.mu.RLock()
    defer r.mu.RUnlock()
    _, exists := r.types[resourceType]
    return exists
}

// GetSupportedTypes returns a list of all resource types that can be resolved
func (r *ResolutionRegistry) GetSupportedTypes() []string {
    r.mu.RLock()
    defer r.mu.RUnlock()
    
    types := make([]string, 0, len(r.types))
    for t := range r.types {
        types = append(types, t)
    }
    return types
}

// GetSupportedSelectorFields returns supported fields for selector-based resolution
func (r *ResolutionRegistry) GetSupportedSelectorFields(resourceType string) []string {
    r.mu.RLock()
    defer r.mu.RUnlock()
    
    if info, exists := r.types[resourceType]; exists {
        return info.SelectorFields
    }
    return nil
}

// IsValidParentChild returns true if the parent-child relationship is valid for resolution
func (r *ResolutionRegistry) IsValidParentChild(parentType, childType string) bool {
    r.mu.RLock()
    defer r.mu.RUnlock()
    
    parentInfo, parentExists := r.types[parentType]
    if !parentExists {
        return false
    }
    
    for _, supportedChild := range parentInfo.SupportedChildren {
        if supportedChild == childType {
            return true
        }
    }
    
    return false
}

// GetResolutionAdapter returns the resolution adapter for a resource type
func (r *ResolutionRegistry) GetResolutionAdapter(resourceType string) (ResolutionAdapter, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    
    info, exists := r.types[resourceType]
    if !exists {
        return nil, fmt.Errorf("unsupported resource type: %s", resourceType)
    }
    
    if info.ResolutionAdapter == nil {
        return nil, fmt.Errorf("no resolution adapter configured for resource type: %s", resourceType)
    }
    
    return info.ResolutionAdapter, nil
}

// GetResolutionMetadata returns the full resolution metadata for a resource type
func (r *ResolutionRegistry) GetResolutionMetadata(resourceType string) (*ResolutionMetadata, bool) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    
    info, exists := r.types[resourceType]
    return info, exists
}

// initializeBuiltinTypes registers the built-in resource types with their resolution metadata
func (r *ResolutionRegistry) initializeBuiltinTypes() {
    // Portal resource type
    r.Register("portal", &ResolutionMetadata{
        Name:              "Portal",
        SelectorFields:    []string{"name", "description"},
        SupportedParents:  nil, // Portals are top-level
        SupportedChildren: []string{"portal_customization", "portal_custom_domain", "portal_page", "portal_snippet"},
        ResolutionAdapter: nil, // Will be set in future steps
    })
    
    // API resource type
    r.Register("api", &ResolutionMetadata{
        Name:              "API",
        SelectorFields:    []string{"name", "description"},
        SupportedParents:  nil, // APIs are top-level
        SupportedChildren: []string{"api_version", "api_publication", "api_implementation", "api_document"},
        ResolutionAdapter: nil, // Will be set in future steps
    })
    
    // Control Plane resource type
    r.Register("control_plane", &ResolutionMetadata{
        Name:              "Control Plane",
        SelectorFields:    []string{"name", "description"},
        SupportedParents:  nil, // Control planes are top-level
        SupportedChildren: nil, // No child resources supported yet
        ResolutionAdapter: nil, // Will be set in future steps
    })
    
    // API Version resource type (child of API)
    r.Register("api_version", &ResolutionMetadata{
        Name:             "API Version", 
        SelectorFields:   []string{"name", "version"},
        SupportedParents: []string{"api"},
        SupportedChildren: nil,
        ResolutionAdapter: nil, // Will be set in future steps
    })
    
    // Additional child resource types would be added here...
}
```

### Phase 4: Integration Points

This phase integrates external resources with the existing validation and processing pipeline.

#### 4.1 Integrate External Resource Validation

**File:** `internal/declarative/resources/validation.go`

**Changes:** Add external resource validation to resource set validation

```go
// Add to existing ValidateResourceSet function (or create if it doesn't exist)

// ValidateResourceSet validates all resources in a resource set
func ValidateResourceSet(rs *ResourceSet) error {
    var errors []error
    
    // Validate external resources first (they provide context for other validations)
    for i, ext := range rs.ExternalResources {
        if err := ext.Validate(); err != nil {
            errors = append(errors, fmt.Errorf("external_resources[%d]: %w", i, err))
        }
    }
    
    // Continue with existing resource validation...
    for i, portal := range rs.Portals {
        if err := portal.Validate(); err != nil {
            errors = append(errors, fmt.Errorf("portals[%d]: %w", i, err))
        }
    }
    
    // ... other resource validations
    
    if len(errors) > 0 {
        return fmt.Errorf("validation failed: %v", errors)
    }
    
    return nil
}
```

#### 4.2 Update Resource Set Processing

**File:** `internal/declarative/planner/planner.go`

**Changes:** Add external resource awareness to planning

```go
// Add imports
import (
    "github.com/Kong/kongctl/internal/declarative/external"
)

// Modify plan generation to handle external resources
func (p *Planner) GeneratePlan(ctx context.Context, resourceSet *resources.ResourceSet) (*Plan, error) {
    // TODO: This will be expanded in subsequent steps
    // For now, we validate that external resources are properly structured
    
    // Validate resource set including external resources
    if err := resources.ValidateResourceSet(resourceSet); err != nil {
        return nil, fmt.Errorf("resource set validation failed: %w", err)
    }
    
    // Continue with existing plan generation...
    // The external resource resolution will be implemented in later steps
    
    return p.generatePlanInternal(ctx, resourceSet)
}
```

## Detailed Code Specifications

### ExternalResourceResource Interface Implementation

The `ExternalResourceResource` struct implements the following interfaces:

```go
// Resource interface (common to all resources)
// Note: ExternalResourceResource only implements GetRef(), not GetKongctlMeta()
// as external resources cannot be protected or namespaced

// ResourceValidator interface (for validation)
type ResourceValidator interface {
    Validate() error
}

// ExternalResourceInterface (interface specific to external resources)
type ExternalResourceInterface interface {
    GetRef() string
    GetResourceType() string
    Validate() error
    IsResolved() bool
    GetResolvedID() string
    SetResolvedID(id string)
    GetResolvedResource() interface{}
    SetResolvedResource(resource interface{})
}
```

### Validation Error Types

Define structured error types for better error handling:

```go
// ExternalResourceError represents validation errors for external resources
type ExternalResourceError struct {
    Ref         string
    ResourceType string
    Field       string
    Message     string
    Cause       error
}

func (e *ExternalResourceError) Error() string {
    return fmt.Sprintf("external resource %q (%s): %s in field %s", 
        e.Ref, e.ResourceType, e.Message, e.Field)
}

func (e *ExternalResourceError) Unwrap() error {
    return e.Cause
}
```

## Testing Strategy

### Unit Tests

#### 4.1 ExternalResourceResource Validation Tests

**File:** `internal/declarative/resources/external_resource_test.go` (NEW FILE)

```go
func TestExternalResourceResource_Validate(t *testing.T) {
    tests := []struct {
        name    string
        resource ExternalResourceResource
        wantErr bool
        errMsg  string
    }{
        {
            name: "valid with ID",
            resource: ExternalResourceResource{
                Ref:          "my-portal",
                ResourceType: "portal", 
                ID:           stringPtr("portal-123"),
            },
            wantErr: false,
        },
        {
            name: "valid with selector",
            resource: ExternalResourceResource{
                Ref:          "my-portal",
                ResourceType: "portal",
                Selector: &ExternalResourceSelector{
                    MatchFields: map[string]string{
                        "name": "My Portal",
                    },
                },
            },
            wantErr: false,
        },
        {
            name: "invalid - both ID and selector",
            resource: ExternalResourceResource{
                Ref:          "my-portal",
                ResourceType: "portal",
                ID:           stringPtr("portal-123"),
                Selector: &ExternalResourceSelector{
                    MatchFields: map[string]string{
                        "name": "My Portal",
                    },
                },
            },
            wantErr: true,
            errMsg:  "mutually exclusive",
        },
        {
            name: "invalid - neither ID nor selector",
            resource: ExternalResourceResource{
                Ref:          "my-portal", 
                ResourceType: "portal",
            },
            wantErr: true,
            errMsg:  "must be specified",
        },
        {
            name: "invalid resource type",
            resource: ExternalResourceResource{
                Ref:          "my-resource",
                ResourceType: "invalid_type",
                ID:           stringPtr("resource-123"),
            },
            wantErr: true,
            errMsg:  "unsupported resource_type",
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := tt.resource.Validate()
            if tt.wantErr {
                assert.Error(t, err)
                if tt.errMsg != "" {
                    assert.Contains(t, err.Error(), tt.errMsg)
                }
            } else {
                assert.NoError(t, err)
            }
        })
    }
}

func stringPtr(s string) *string {
    return &s
}
```

#### 4.2 Registry Tests

**File:** `internal/declarative/external/registry_test.go` (NEW FILE)

```go
func TestResolutionRegistry_IsSupported(t *testing.T) {
    registry := GetResolutionRegistry()
    
    tests := []struct {
        resourceType string
        want         bool
    }{
        {"portal", true},
        {"api", true},
        {"control_plane", true},
        {"invalid_type", false},
        {"", false},
    }
    
    for _, tt := range tests {
        t.Run(tt.resourceType, func(t *testing.T) {
            got := registry.IsSupported(tt.resourceType)
            assert.Equal(t, tt.want, got)
        })
    }
}

func TestResolutionRegistry_GetSupportedSelectorFields(t *testing.T) {
    registry := GetResolutionRegistry()
    
    fields := registry.GetSupportedSelectorFields("portal")
    assert.Contains(t, fields, "name")
    assert.Contains(t, fields, "description")
    
    fields = registry.GetSupportedSelectorFields("invalid_type")
    assert.Nil(t, fields)
}

func TestResolutionRegistry_IsValidParentChild(t *testing.T) {
    registry := GetResolutionRegistry()
    
    // Valid relationships
    assert.True(t, registry.IsValidParentChild("api", "api_version"))
    assert.True(t, registry.IsValidParentChild("portal", "portal_page"))
    
    // Invalid relationships
    assert.False(t, registry.IsValidParentChild("api_version", "api"))
    assert.False(t, registry.IsValidParentChild("portal", "control_plane"))
    assert.False(t, registry.IsValidParentChild("invalid", "api"))
}

func TestResolutionRegistry_GetResolutionAdapter(t *testing.T) {
    registry := GetResolutionRegistry()
    
    // Test that resolution adapters return nil for now (will be implemented in future steps)
    adapter, err := registry.GetResolutionAdapter("portal")
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "no resolution adapter configured")
    assert.Nil(t, adapter)
}
```

### Integration Tests

#### 4.3 YAML Configuration Parsing Tests

**File:** `internal/declarative/resources/integration_test.go`

```go
func TestResourceSet_ParseExternalResources(t *testing.T) {
    yamlContent := `
external_resources:
  - ref: existing-portal
    resource_type: portal
    id: portal-123
    
  - ref: found-portal  
    resource_type: portal
    selector:
      match_fields:
        name: "My Portal"
        
  - ref: child-api-version
    resource_type: api_version
    selector:
      match_fields:
        name: "v1.0"
    parent:
      resource_type: api
      id: api-456
      
portals:
  - ref: new-portal
    name: "New Portal"
    # This portal might reference existing-portal in some way
`

    var rs resources.ResourceSet
    err := yaml.Unmarshal([]byte(yamlContent), &rs)
    assert.NoError(t, err)
    
    assert.Len(t, rs.ExternalResources, 3)
    
    // Test first external resource
    ext1 := rs.ExternalResources[0]
    assert.Equal(t, "existing-portal", ext1.Ref)
    assert.Equal(t, "portal", ext1.ResourceType)
    assert.NotNil(t, ext1.ID)
    assert.Equal(t, "portal-123", *ext1.ID)
    assert.Nil(t, ext1.Selector)
    
    // Test second external resource  
    ext2 := rs.ExternalResources[1]
    assert.Equal(t, "found-portal", ext2.Ref)
    assert.Equal(t, "portal", ext2.ResourceType)
    assert.Nil(t, ext2.ID)
    assert.NotNil(t, ext2.Selector)
    assert.Equal(t, "My Portal", ext2.Selector.MatchFields["name"])
    
    // Test third external resource (with parent)
    ext3 := rs.ExternalResources[2]
    assert.Equal(t, "child-api-version", ext3.Ref)
    assert.Equal(t, "api_version", ext3.ResourceType)
    assert.NotNil(t, ext3.Parent)
    assert.Equal(t, "api", ext3.Parent.ResourceType)
    assert.Equal(t, "api-456", ext3.Parent.ID)
    
    // Validate the entire resource set
    err = resources.ValidateResourceSet(&rs)
    assert.NoError(t, err)
}
```

## File Modification Matrix

| File | Modification Type | Key Changes |
|------|------------------|-------------|
| `internal/declarative/resources/types.go` | Modify | Add `ExternalResources []ExternalResourceResource` field to ResourceSet |
| `internal/declarative/resources/external_resource.go` | Create | Complete ExternalResourceResource implementation with validation |
| `internal/declarative/resources/validation.go` | Modify | Add external resource validation functions |
| `internal/declarative/external/types.go` | Create | Resource type info and query adapter interfaces |
| `internal/declarative/external/registry.go` | Create | Resource type registry with built-in types |
| `internal/declarative/planner/planner.go` | Modify | Add external resource validation to plan generation |

## Integration Points with Existing Code

### 1. Configuration Loading Integration
- **No changes required** - YAML parsing automatically handles new fields
- External resources will be parsed into `ResourceSet.ExternalResources` slice
- Existing `tags/file.go` external file loading works without modification

### 2. Validation Integration
- External resources validated as part of `ValidateResourceSet()`
- Validation occurs before planning phase
- Validation errors include context about which external resource failed

### 3. Interface Compatibility
- `ExternalResourceResource` implements standard `Resource` interface
- Compatible with existing resource processing patterns
- Can be used with common utility functions

### 4. Future Extension Points  
- Registry supports adding new resource types without code changes
- Query adapter interface allows resource-specific SDK integration
- Validation framework extensible for new validation rules

## Risk Mitigation

### 1. Breaking Changes Prevention
- **Risk:** New fields break existing configurations
- **Mitigation:** All new fields use `omitempty` YAML tags, fully backward compatible

### 2. Validation Performance
- **Risk:** Complex validation slows down configuration parsing  
- **Mitigation:** Registry uses efficient maps, validation short-circuits on errors

### 3. Resource Type Extension
- **Risk:** Hard to add new resource types later
- **Mitigation:** Registry-based design allows runtime registration of new types

### 4. Error Message Clarity
- **Risk:** Validation errors are confusing to users
- **Mitigation:** Structured error messages with field context and suggestions

## Success Criteria

### Phase 1 Success Criteria
- [ ] ResourceSet parses external_resources from YAML without errors
- [ ] ExternalResourceResource struct compiles and implements required interfaces
- [ ] Basic validation prevents malformed configurations
- [ ] All existing functionality unchanged

### Phase 2 Success Criteria  
- [ ] All validation rules correctly identify valid/invalid configurations
- [ ] Error messages provide actionable feedback to users
- [ ] XOR validation prevents conflicting ID/selector specifications
- [ ] Resource type validation uses registry correctly

### Phase 3 Success Criteria
- [ ] Registry correctly identifies supported resource types and fields
- [ ] Parent-child relationship validation works across different resource types
- [ ] Built-in resource types (portal, api, control_plane) fully supported
- [ ] Registry is extensible for future resource types

### Phase 4 Success Criteria
- [ ] External resources integrate with existing validation pipeline
- [ ] Planning phase acknowledges external resources (validation only for now)
- [ ] No regressions in existing resource processing
- [ ] Complete test coverage for all new functionality

## Implementation Order

1. **Start with Phase 1.1** - Modify ResourceSet to add the field (lowest risk)
2. **Implement Phase 1.2** - Create ExternalResourceResource struct with basic validation
3. **Phase 2.1** - Add comprehensive validation functions  
4. **Phase 3.1 & 3.2** - Create registry and type system (can be parallel)
5. **Phase 4.1** - Integrate validation with existing pipeline
6. **Phase 4.2** - Add planning phase awareness
7. **All Testing** - Implement tests throughout development, not just at the end

Each phase should be completed and tested before moving to the next to ensure stability and enable early detection of issues.

## Next Steps

After completing this implementation, the foundation will be ready for:
- **Stage 8 Step 2:** External resource resolution and SDK integration
- **Stage 8 Step 3:** Integration with reference resolution system  
- **Stage 8 Step 4:** Dependency graph building and resolution ordering
- **Stage 8 Step 5:** Error handling and recovery mechanisms

This implementation provides the necessary schema, validation, and infrastructure foundation that subsequent steps will build upon for complete external resource functionality.