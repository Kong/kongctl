# Step 3: External Resource Resolver - Implementation Plan

## Executive Summary

This plan provides a comprehensive implementation strategy for Step 3 of the external resources feature: the External Resource Resolver. Steps 1-2 have created solid infrastructure with schema validation and registry adapters. Step 3 implements the core resolution engine that executes SDK queries to resolve external resource references to Konnect IDs.

## Implementation Overview

### What Needs to Be Built

The ExternalResourceResolver is the core component that:
1. Parses external resources from configuration files
2. Builds dependency graphs for parent-child resolution ordering
3. Executes SDK queries via registry adapters to find resources
4. Validates exactly one match per selector
5. Stores resolved resources for reference resolution
6. Provides resolved IDs to the planning system

### Key Components and Responsibilities

1. **ExternalResourceResolver**: Core resolution engine
   - Manages resolution workflow and caching
   - Coordinates with registry adapters
   - Handles dependency ordering
   
2. **DependencyGraph**: Parent-child resolution ordering
   - Builds dependency relationships
   - Performs topological sorting
   - Detects circular dependencies

3. **ResolvedExternalResource**: Storage for resolved data
   - Caches resolved IDs and full resource objects
   - Provides metadata for debugging
   - Supports parent-child relationships

4. **Planner Integration**: Seamless integration with existing planner
   - Replaces placeholder validation with actual resolution
   - Provides resolved IDs to reference resolution system
   - Maintains existing planner workflow

### Integration Approach

The resolver integrates at the existing `resolveResourceIdentities()` method in the planner, replacing the placeholder `validateExternalResources()` implementation. It leverages the completed registry system and adapters, maintaining full backward compatibility with existing functionality.

## Detailed Implementation Steps

### Phase 1: Core Resolver Implementation

#### Step 1.1: Create Core Types and Structures

**File**: `/internal/declarative/external/types.go` (extend existing)

Add resolver-specific types:

```go
// ResolvedExternalResource holds the resolved data for an external resource
type ResolvedExternalResource struct {
    ID           string                 // Resolved Konnect ID
    Resource     interface{}           // Full SDK response object
    ResourceType string               // Resource type (portal, api, etc.)
    Ref          string               // Original reference from config
    Parent       *ResolvedExternalResource // Parent resource if applicable
    ResolvedAt   time.Time            // Resolution timestamp
}

// DependencyNode represents a node in the dependency graph
type DependencyNode struct {
    Ref          string   // External resource reference
    ResourceType string   // Resource type
    ParentRef    string   // Parent reference (empty for top-level)
    ChildRefs    []string // Child references
    Resolved     bool     // Resolution status
}

// DependencyGraph manages resolution ordering
type DependencyGraph struct {
    Nodes           map[string]*DependencyNode // All nodes by ref
    ResolutionOrder []string                   // Topologically sorted order
}
```

#### Step 1.2: Create External Resource Resolver

**File**: `/internal/declarative/external/resolver.go`

```go
package external

import (
    "context"
    "fmt"
    "log/slog"
    "time"
    
    "github.com/Kong/kongctl/internal/declarative/resources"
    "github.com/Kong/kongctl/internal/state"
)

// ExternalResourceResolver resolves external resource references to Konnect IDs
type ExternalResourceResolver struct {
    registry *ResolutionRegistry
    client   *state.Client
    logger   *slog.Logger
    resolved map[string]*ResolvedExternalResource
}

// NewExternalResourceResolver creates a new resolver instance
func NewExternalResourceResolver(registry *ResolutionRegistry, client *state.Client, logger *slog.Logger) *ExternalResourceResolver {
    return &ExternalResourceResolver{
        registry: registry,
        client:   client,
        logger:   logger,
        resolved: make(map[string]*ResolvedExternalResource),
    }
}

// ResolveExternalResources resolves all external resources in dependency order
func (r *ExternalResourceResolver) ResolveExternalResources(ctx context.Context, externalResources []resources.ExternalResourceResource) error {
    if len(externalResources) == 0 {
        return nil
    }
    
    r.logger.Debug("Starting external resource resolution", "count", len(externalResources))
    
    // Build dependency graph for resolution ordering
    graph, err := r.buildDependencyGraph(externalResources)
    if err != nil {
        return fmt.Errorf("failed to build dependency graph: %w", err)
    }
    
    // Resolve resources in dependency order
    for _, ref := range graph.ResolutionOrder {
        if err := r.resolveResource(ctx, findResourceByRef(externalResources, ref)); err != nil {
            return fmt.Errorf("failed to resolve external resource '%s': %w", ref, err)
        }
    }
    
    r.logger.Info("External resource resolution completed", "resolved_count", len(r.resolved))
    return nil
}

// resolveResource resolves a single external resource
func (r *ExternalResourceResolver) resolveResource(ctx context.Context, resource *resources.ExternalResourceResource) error {
    // Skip if already resolved
    if _, exists := r.resolved[resource.Ref]; exists {
        return nil
    }
    
    r.logger.Debug("Resolving external resource", "ref", resource.Ref, "type", resource.ResourceType)
    
    // Get appropriate adapter from registry
    adapter, err := r.registry.GetResolutionAdapter(resource.ResourceType)
    if err != nil {
        return fmt.Errorf("failed to get adapter for resource type '%s': %w", resource.ResourceType, err)
    }
    
    // Prepare parent context if needed
    var parentResource interface{}
    if resource.Parent != nil {
        parentRef := resource.Parent.Ref
        parentResolved, exists := r.resolved[parentRef]
        if !exists {
            return fmt.Errorf("parent resource '%s' not resolved yet", parentRef)
        }
        parentResource = parentResolved.Resource
    }
    
    // Execute resolution via adapter
    var resolved interface{}
    var resolvedID string
    
    if resource.ID != "" {
        // Direct ID resolution
        resolved, err = adapter.GetByID(ctx, resource.ID, parentResource)
        if err != nil {
            return fmt.Errorf("failed to resolve by ID: %w", err)
        }
        resolvedID = resource.ID
    } else {
        // Selector-based resolution
        results, err := adapter.GetBySelector(ctx, resource.Selector, parentResource)
        if err != nil {
            return fmt.Errorf("failed to resolve by selector: %w", err)
        }
        
        // Validate exactly one match
        if len(results) == 0 {
            return r.createZeroMatchError(resource)
        }
        if len(results) > 1 {
            return r.createMultipleMatchError(resource, len(results))
        }
        
        resolved = results[0]
        resolvedID = adapter.ExtractID(resolved)
    }
    
    // Store resolved resource
    resolvedResource := &ResolvedExternalResource{
        ID:           resolvedID,
        Resource:     resolved,
        ResourceType: resource.ResourceType,
        Ref:          resource.Ref,
        ResolvedAt:   time.Now(),
    }
    
    // Set parent reference if applicable
    if resource.Parent != nil {
        if parentResolved, exists := r.resolved[resource.Parent.Ref]; exists {
            resolvedResource.Parent = parentResolved
        }
    }
    
    r.resolved[resource.Ref] = resolvedResource
    
    // Update original resource with resolved ID
    resource.SetResolvedID(resolvedID)
    resource.SetResolvedResource(resolved)
    
    r.logger.Debug("External resource resolved", "ref", resource.Ref, "id", resolvedID)
    return nil
}

// GetResolvedResource retrieves a resolved resource by reference
func (r *ExternalResourceResolver) GetResolvedResource(ref string) (*ResolvedExternalResource, bool) {
    resolved, exists := r.resolved[ref]
    return resolved, exists
}

// HasResolvedResource checks if a resource reference has been resolved
func (r *ExternalResourceResolver) HasResolvedResource(ref string) bool {
    _, exists := r.resolved[ref]
    return exists
}

// GetResolvedID returns just the resolved ID for a reference
func (r *ExternalResourceResolver) GetResolvedID(ref string) (string, bool) {
    if resolved, exists := r.resolved[ref]; exists {
        return resolved.ID, true
    }
    return "", false
}

// Helper functions for error creation
func (r *ExternalResourceResolver) createZeroMatchError(resource *resources.ExternalResourceResource) error {
    return fmt.Errorf("external resource '%s' selector matched 0 resources\n"+
        "  Resource type: %s\n"+
        "  Selector: %+v\n"+
        "  Suggestion: Verify the resource exists in Konnect",
        resource.Ref, resource.ResourceType, resource.Selector)
}

func (r *ExternalResourceResolver) createMultipleMatchError(resource *resources.ExternalResourceResource, count int) error {
    return fmt.Errorf("external resource '%s' selector matched %d resources\n"+
        "  Resource type: %s\n"+
        "  Selector: %+v\n"+
        "  Suggestion: Use more specific selector fields to match exactly one resource",
        resource.Ref, resource.ResourceType, count, resource.Selector)
}

// findResourceByRef finds an external resource by reference
func findResourceByRef(resources []resources.ExternalResourceResource, ref string) *resources.ExternalResourceResource {
    for i := range resources {
        if resources[i].Ref == ref {
            return &resources[i]
        }
    }
    return nil
}
```

#### Step 1.3: Create Dependency Graph Implementation

**File**: `/internal/declarative/external/dependencies.go`

```go
package external

import (
    "fmt"
    "sort"
    
    "github.com/Kong/kongctl/internal/declarative/resources"
)

// buildDependencyGraph creates a dependency graph for external resources
func (r *ExternalResourceResolver) buildDependencyGraph(externalResources []resources.ExternalResourceResource) (*DependencyGraph, error) {
    graph := &DependencyGraph{
        Nodes: make(map[string]*DependencyNode),
    }
    
    // Create nodes for all resources
    for _, resource := range externalResources {
        node := &DependencyNode{
            Ref:          resource.Ref,
            ResourceType: resource.ResourceType,
            ChildRefs:    make([]string, 0),
            Resolved:     false,
        }
        
        // Set parent reference if applicable
        if resource.Parent != nil {
            node.ParentRef = resource.Parent.Ref
            
            // Validate parent-child relationship using registry
            if !r.registry.IsValidParentChild(resource.Parent.ResourceType, resource.ResourceType) {
                return nil, fmt.Errorf("invalid parent-child relationship: %s -> %s",
                    resource.Parent.ResourceType, resource.ResourceType)
            }
        }
        
        graph.Nodes[resource.Ref] = node
    }
    
    // Build parent-child relationships
    for _, node := range graph.Nodes {
        if node.ParentRef != "" {
            parent, exists := graph.Nodes[node.ParentRef]
            if !exists {
                return nil, fmt.Errorf("parent resource '%s' not found for child '%s'",
                    node.ParentRef, node.Ref)
            }
            parent.ChildRefs = append(parent.ChildRefs, node.Ref)
        }
    }
    
    // Perform topological sort for resolution order
    order, err := r.topologicalSort(graph)
    if err != nil {
        return nil, fmt.Errorf("failed to determine resolution order: %w", err)
    }
    
    graph.ResolutionOrder = order
    return graph, nil
}

// topologicalSort performs topological sorting of dependency graph
func (r *ExternalResourceResolver) topologicalSort(graph *DependencyGraph) ([]string, error) {
    // Track in-degree (number of dependencies) for each node
    inDegree := make(map[string]int)
    for ref := range graph.Nodes {
        inDegree[ref] = 0
    }
    
    // Calculate in-degrees
    for _, node := range graph.Nodes {
        if node.ParentRef != "" {
            inDegree[node.Ref]++
        }
    }
    
    // Queue of nodes with no dependencies (in-degree 0)
    queue := make([]string, 0)
    for ref, degree := range inDegree {
        if degree == 0 {
            queue = append(queue, ref)
        }
    }
    
    // Sort queue for deterministic ordering
    sort.Strings(queue)
    
    result := make([]string, 0, len(graph.Nodes))
    
    for len(queue) > 0 {
        // Remove node from queue
        current := queue[0]
        queue = queue[1:]
        result = append(result, current)
        
        // Update dependencies for child nodes
        node := graph.Nodes[current]
        for _, childRef := range node.ChildRefs {
            inDegree[childRef]--
            if inDegree[childRef] == 0 {
                queue = append(queue, childRef)
                sort.Strings(queue) // Keep queue sorted for determinism
            }
        }
    }
    
    // Check for circular dependencies
    if len(result) != len(graph.Nodes) {
        return nil, fmt.Errorf("circular dependency detected in external resources")
    }
    
    return result, nil
}

// ValidateDependencies validates the dependency graph for consistency
func (r *ExternalResourceResolver) ValidateDependencies(graph *DependencyGraph) error {
    for _, node := range graph.Nodes {
        // Validate parent exists if specified
        if node.ParentRef != "" {
            if _, exists := graph.Nodes[node.ParentRef]; !exists {
                return fmt.Errorf("parent resource '%s' not found for '%s'",
                    node.ParentRef, node.Ref)
            }
        }
        
        // Validate children exist
        for _, childRef := range node.ChildRefs {
            if _, exists := graph.Nodes[childRef]; !exists {
                return fmt.Errorf("child resource '%s' not found for parent '%s'",
                    childRef, node.Ref)
            }
        }
    }
    
    return nil
}
```

### Phase 2: Planner Integration

#### Step 2.1: Modify Planner Structure

**File**: `/internal/declarative/planner/planner.go`

Add ExternalResourceResolver field to Planner struct:

```go
// Around line 32, add to Planner struct
type Planner struct {
    client             *state.Client
    logger             *slog.Logger
    resolver           *ReferenceResolver
    depResolver        *DependencyResolver
    externalResolver   *external.ExternalResourceResolver // NEW FIELD
    // ... existing fields
}
```

#### Step 2.2: Update Planner Constructor

**File**: `/internal/declarative/planner/planner.go`

Modify NewPlanner function (around line 47):

```go
func NewPlanner(client *state.Client, logger *slog.Logger) *Planner {
    // ... existing initialization
    
    // Initialize external resource resolver
    registry := external.GetResolutionRegistry()
    externalResolver := external.NewExternalResourceResolver(registry, client, logger)
    
    return &Planner{
        client:           client,
        logger:           logger,
        resolver:         NewReferenceResolver(client, externalResolver), // Updated
        depResolver:      NewDependencyResolver(),
        externalResolver: externalResolver, // NEW FIELD
        // ... existing fields
    }
}
```

#### Step 2.3: Replace External Resource Validation

**File**: `/internal/declarative/planner/planner.go`

Replace the placeholder implementation (lines 391-441):

```go
// Replace validateExternalResources call (around line 391) with:
if err := p.externalResolver.ResolveExternalResources(ctx, rs.ExternalResources); err != nil {
    return nil, fmt.Errorf("failed to resolve external resources: %w", err)
}
```

Remove the old `validateExternalResources` method entirely.

#### Step 2.4: Enhance Reference Resolution

**File**: `/internal/declarative/planner/resolver.go`

Update ReferenceResolver constructor:

```go
// Update ReferenceResolver struct (around line 20)
type ReferenceResolver struct {
    client           *state.Client
    externalResolver *external.ExternalResourceResolver // NEW FIELD
}

// Update constructor (around line 32)
func NewReferenceResolver(client *state.Client, externalResolver *external.ExternalResourceResolver) *ReferenceResolver {
    return &ReferenceResolver{
        client:           client,
        externalResolver: externalResolver,
    }
}
```

Update reference resolution logic:

```go
// In resolveReference method (around line 50), add at the beginning:
func (r *ReferenceResolver) resolveReference(ctx context.Context, resourceType, ref string) (string, error) {
    // Check external resources first
    if r.externalResolver != nil {
        if resolvedID, found := r.externalResolver.GetResolvedID(ref); found {
            return resolvedID, nil
        }
    }
    
    // Existing logic as fallback
    switch resourceType {
    // ... existing cases remain unchanged
    }
}
```

### Phase 3: Testing Implementation

#### Step 3.1: Unit Tests for Core Resolver

**File**: `/internal/declarative/external/resolver_test.go`

```go
package external

import (
    "context"
    "log/slog"
    "testing"
    "time"
    
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
    
    "github.com/Kong/kongctl/internal/declarative/resources"
    "github.com/Kong/kongctl/internal/state"
)

// Mock adapter for testing
type MockAdapter struct {
    mock.Mock
}

func (m *MockAdapter) GetByID(ctx context.Context, id string, parent interface{}) (interface{}, error) {
    args := m.Called(ctx, id, parent)
    return args.Get(0), args.Error(1)
}

func (m *MockAdapter) GetBySelector(ctx context.Context, selector resources.ExternalResourceSelector, parent interface{}) ([]interface{}, error) {
    args := m.Called(ctx, selector, parent)
    return args.Get(0).([]interface{}), args.Error(1)
}

func (m *MockAdapter) ExtractID(resource interface{}) string {
    args := m.Called(resource)
    return args.String(0)
}

// Test cases
func TestNewExternalResourceResolver(t *testing.T) {
    registry := &ResolutionRegistry{}
    client := &state.Client{}
    logger := slog.Default()
    
    resolver := NewExternalResourceResolver(registry, client, logger)
    
    assert.NotNil(t, resolver)
    assert.Equal(t, registry, resolver.registry)
    assert.Equal(t, client, resolver.client)
    assert.Equal(t, logger, resolver.logger)
    assert.NotNil(t, resolver.resolved)
}

func TestResolveExternalResources_EmptyList(t *testing.T) {
    resolver := NewExternalResourceResolver(&ResolutionRegistry{}, &state.Client{}, slog.Default())
    
    err := resolver.ResolveExternalResources(context.Background(), []resources.ExternalResourceResource{})
    
    assert.NoError(t, err)
}

func TestResolveExternalResources_SingleResource(t *testing.T) {
    // Setup mock registry and adapter
    registry := &ResolutionRegistry{}
    adapter := &MockAdapter{}
    
    // Mock successful resolution
    mockResource := map[string]interface{}{"id": "portal-123", "name": "Test Portal"}
    adapter.On("GetBySelector", mock.Anything, mock.Anything, mock.Anything).
        Return([]interface{}{mockResource}, nil)
    adapter.On("ExtractID", mockResource).Return("portal-123")
    
    // Mock registry
    registry.adapters = map[string]ResolutionAdapter{"portal": adapter}
    
    resolver := NewExternalResourceResolver(registry, &state.Client{}, slog.Default())
    
    // Create test resource
    externalResource := resources.ExternalResourceResource{
        Ref:          "test-portal",
        ResourceType: "portal",
        Selector: resources.ExternalResourceSelector{
            MatchFields: map[string]interface{}{"name": "Test Portal"},
        },
    }
    
    // Test resolution
    err := resolver.ResolveExternalResources(context.Background(), []resources.ExternalResourceResource{externalResource})
    
    assert.NoError(t, err)
    assert.True(t, resolver.HasResolvedResource("test-portal"))
    
    resolvedID, found := resolver.GetResolvedID("test-portal")
    assert.True(t, found)
    assert.Equal(t, "portal-123", resolvedID)
}

func TestResolveExternalResources_ZeroMatches(t *testing.T) {
    // Setup mock registry and adapter
    registry := &ResolutionRegistry{}
    adapter := &MockAdapter{}
    
    // Mock zero matches
    adapter.On("GetBySelector", mock.Anything, mock.Anything, mock.Anything).
        Return([]interface{}{}, nil)
    
    registry.adapters = map[string]ResolutionAdapter{"portal": adapter}
    
    resolver := NewExternalResourceResolver(registry, &state.Client{}, slog.Default())
    
    // Create test resource
    externalResource := resources.ExternalResourceResource{
        Ref:          "missing-portal",
        ResourceType: "portal",
        Selector: resources.ExternalResourceSelector{
            MatchFields: map[string]interface{}{"name": "Missing Portal"},
        },
    }
    
    // Test resolution
    err := resolver.ResolveExternalResources(context.Background(), []resources.ExternalResourceResource{externalResource})
    
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "selector matched 0 resources")
    assert.Contains(t, err.Error(), "missing-portal")
}

func TestResolveExternalResources_MultipleMatches(t *testing.T) {
    // Setup mock registry and adapter
    registry := &ResolutionRegistry{}
    adapter := &MockAdapter{}
    
    // Mock multiple matches
    mockResource1 := map[string]interface{}{"id": "portal-123", "name": "Portal"}
    mockResource2 := map[string]interface{}{"id": "portal-456", "name": "Portal"}
    adapter.On("GetBySelector", mock.Anything, mock.Anything, mock.Anything).
        Return([]interface{}{mockResource1, mockResource2}, nil)
    
    registry.adapters = map[string]ResolutionAdapter{"portal": adapter}
    
    resolver := NewExternalResourceResolver(registry, &state.Client{}, slog.Default())
    
    // Create test resource
    externalResource := resources.ExternalResourceResource{
        Ref:          "ambiguous-portal",
        ResourceType: "portal",
        Selector: resources.ExternalResourceSelector{
            MatchFields: map[string]interface{}{"name": "Portal"},
        },
    }
    
    // Test resolution
    err := resolver.ResolveExternalResources(context.Background(), []resources.ExternalResourceResource{externalResource})
    
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "selector matched 2 resources")
    assert.Contains(t, err.Error(), "ambiguous-portal")
}

func TestResolveExternalResources_ParentChild(t *testing.T) {
    // Setup mock registry and adapters
    registry := &ResolutionRegistry{}
    cpAdapter := &MockAdapter{}
    serviceAdapter := &MockAdapter{}
    
    // Mock parent resolution
    cpResource := map[string]interface{}{"id": "cp-123", "name": "Test CP"}
    cpAdapter.On("GetBySelector", mock.Anything, mock.Anything, nil).
        Return([]interface{}{cpResource}, nil)
    cpAdapter.On("ExtractID", cpResource).Return("cp-123")
    
    // Mock child resolution
    serviceResource := map[string]interface{}{"id": "service-456", "name": "Test Service"}
    serviceAdapter.On("GetBySelector", mock.Anything, mock.Anything, cpResource).
        Return([]interface{}{serviceResource}, nil)
    serviceAdapter.On("ExtractID", serviceResource).Return("service-456")
    
    registry.adapters = map[string]ResolutionAdapter{
        "control_plane": cpAdapter,
        "ce_service":    serviceAdapter,
    }
    
    // Mock valid parent-child relationship
    registry.metadata = map[string]ResourceTypeMetadata{
        "control_plane": {ParentTypes: []string{}},
        "ce_service":    {ParentTypes: []string{"control_plane"}},
    }
    
    resolver := NewExternalResourceResolver(registry, &state.Client{}, slog.Default())
    
    // Create test resources
    externalResources := []resources.ExternalResourceResource{
        {
            Ref:          "test-cp",
            ResourceType: "control_plane",
            Selector: resources.ExternalResourceSelector{
                MatchFields: map[string]interface{}{"name": "Test CP"},
            },
        },
        {
            Ref:          "test-service",
            ResourceType: "ce_service",
            Selector: resources.ExternalResourceSelector{
                MatchFields: map[string]interface{}{"name": "Test Service"},
            },
            Parent: &resources.ExternalResourceParent{
                Ref:          "test-cp",
                ResourceType: "control_plane",
            },
        },
    }
    
    // Test resolution
    err := resolver.ResolveExternalResources(context.Background(), externalResources)
    
    assert.NoError(t, err)
    assert.True(t, resolver.HasResolvedResource("test-cp"))
    assert.True(t, resolver.HasResolvedResource("test-service"))
    
    // Verify parent-child relationship
    childResolved, found := resolver.GetResolvedResource("test-service")
    assert.True(t, found)
    assert.NotNil(t, childResolved.Parent)
    assert.Equal(t, "cp-123", childResolved.Parent.ID)
}
```

#### Step 3.2: Unit Tests for Dependency Graph

**File**: `/internal/declarative/external/dependencies_test.go`

```go
package external

import (
    "testing"
    
    "github.com/stretchr/testify/assert"
    
    "github.com/Kong/kongctl/internal/declarative/resources"
)

func TestBuildDependencyGraph_NoResources(t *testing.T) {
    resolver := &ExternalResourceResolver{
        registry: &ResolutionRegistry{},
    }
    
    graph, err := resolver.buildDependencyGraph([]resources.ExternalResourceResource{})
    
    assert.NoError(t, err)
    assert.NotNil(t, graph)
    assert.Empty(t, graph.Nodes)
    assert.Empty(t, graph.ResolutionOrder)
}

func TestBuildDependencyGraph_SingleResource(t *testing.T) {
    registry := &ResolutionRegistry{
        metadata: map[string]ResourceTypeMetadata{
            "portal": {ParentTypes: []string{}},
        },
    }
    
    resolver := &ExternalResourceResolver{registry: registry}
    
    resources := []resources.ExternalResourceResource{
        {
            Ref:          "test-portal",
            ResourceType: "portal",
        },
    }
    
    graph, err := resolver.buildDependencyGraph(resources)
    
    assert.NoError(t, err)
    assert.Len(t, graph.Nodes, 1)
    assert.Equal(t, []string{"test-portal"}, graph.ResolutionOrder)
    
    node := graph.Nodes["test-portal"]
    assert.Equal(t, "test-portal", node.Ref)
    assert.Equal(t, "portal", node.ResourceType)
    assert.Empty(t, node.ParentRef)
    assert.Empty(t, node.ChildRefs)
}

func TestBuildDependencyGraph_ParentChild(t *testing.T) {
    registry := &ResolutionRegistry{
        metadata: map[string]ResourceTypeMetadata{
            "control_plane": {ParentTypes: []string{}},
            "ce_service":    {ParentTypes: []string{"control_plane"}},
        },
    }
    
    resolver := &ExternalResourceResolver{registry: registry}
    
    resources := []resources.ExternalResourceResource{
        {
            Ref:          "test-service",
            ResourceType: "ce_service",
            Parent: &resources.ExternalResourceParent{
                Ref:          "test-cp",
                ResourceType: "control_plane",
            },
        },
        {
            Ref:          "test-cp",
            ResourceType: "control_plane",
        },
    }
    
    graph, err := resolver.buildDependencyGraph(resources)
    
    assert.NoError(t, err)
    assert.Len(t, graph.Nodes, 2)
    
    // Parent should be resolved before child
    assert.Equal(t, []string{"test-cp", "test-service"}, graph.ResolutionOrder)
    
    // Verify parent node
    parent := graph.Nodes["test-cp"]
    assert.Equal(t, "test-cp", parent.Ref)
    assert.Empty(t, parent.ParentRef)
    assert.Equal(t, []string{"test-service"}, parent.ChildRefs)
    
    // Verify child node
    child := graph.Nodes["test-service"]
    assert.Equal(t, "test-service", child.Ref)
    assert.Equal(t, "test-cp", child.ParentRef)
    assert.Empty(t, child.ChildRefs)
}

func TestBuildDependencyGraph_InvalidParentChild(t *testing.T) {
    registry := &ResolutionRegistry{
        metadata: map[string]ResourceTypeMetadata{
            "portal":     {ParentTypes: []string{}},
            "ce_service": {ParentTypes: []string{"control_plane"}}, // Only allows control_plane parent
        },
    }
    
    resolver := &ExternalResourceResolver{registry: registry}
    
    resources := []resources.ExternalResourceResource{
        {
            Ref:          "test-service",
            ResourceType: "ce_service",
            Parent: &resources.ExternalResourceParent{
                Ref:          "test-portal",
                ResourceType: "portal", // Invalid parent type
            },
        },
        {
            Ref:          "test-portal",
            ResourceType: "portal",
        },
    }
    
    graph, err := resolver.buildDependencyGraph(resources)
    
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "invalid parent-child relationship")
    assert.Nil(t, graph)
}

func TestTopologicalSort_CircularDependency(t *testing.T) {
    // This is a theoretical test - in practice, circular dependencies 
    // shouldn't be possible with our parent-child model, but we should
    // handle it gracefully
    
    graph := &DependencyGraph{
        Nodes: map[string]*DependencyNode{
            "a": {Ref: "a", ParentRef: "b", ChildRefs: []string{"c"}},
            "b": {Ref: "b", ParentRef: "c", ChildRefs: []string{"a"}},
            "c": {Ref: "c", ParentRef: "a", ChildRefs: []string{"b"}},
        },
    }
    
    resolver := &ExternalResourceResolver{}
    order, err := resolver.topologicalSort(graph)
    
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "circular dependency")
    assert.Empty(t, order)
}
```

#### Step 3.3: Integration Tests

**File**: `/internal/declarative/external/integration_test.go`

```go
//go:build integration
// +build integration

package external

import (
    "context"
    "log/slog"
    "testing"
    
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    
    "github.com/Kong/kongctl/internal/declarative/resources"
    "github.com/Kong/kongctl/internal/state"
)

func TestExternalResourceResolver_Integration(t *testing.T) {
    // Skip if not running integration tests
    if testing.Short() {
        t.Skip("Skipping integration test in short mode")
    }
    
    // Setup real state client (requires auth setup)
    client := setupIntegrationClient(t)
    registry := GetResolutionRegistry()
    logger := slog.Default()
    
    resolver := NewExternalResourceResolver(registry, client, logger)
    
    // Test with real external resource configuration
    externalResources := []resources.ExternalResourceResource{
        {
            Ref:          "integration-portal",
            ResourceType: "portal",
            Selector: resources.ExternalResourceSelector{
                MatchFields: map[string]interface{}{
                    "name": "Integration Test Portal",
                },
            },
        },
    }
    
    // This test requires a real Konnect environment with test data
    err := resolver.ResolveExternalResources(context.Background(), externalResources)
    
    // Expect success if portal exists, or specific error if not
    if err != nil {
        assert.Contains(t, err.Error(), "matched 0 resources")
    } else {
        assert.True(t, resolver.HasResolvedResource("integration-portal"))
    }
}

func setupIntegrationClient(t *testing.T) *state.Client {
    // Setup requires PAT token and base URL configuration
    // Implementation depends on test environment setup
    t.Skip("Integration test setup not implemented")
    return nil
}
```

#### Step 3.4: Planner Integration Tests

**File**: `/internal/declarative/planner/external_integration_test.go`

```go
package planner

import (
    "context"
    "log/slog"
    "testing"
    
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    
    "github.com/Kong/kongctl/internal/declarative/external"
    "github.com/Kong/kongctl/internal/declarative/resources"
    "github.com/Kong/kongctl/internal/state"
)

func TestPlannerWithExternalResources(t *testing.T) {
    // Setup mock client and registry
    client := &state.Client{}
    logger := slog.Default()
    
    // Create planner with external resolver
    planner := NewPlanner(client, logger)
    
    // Create resource set with external resources
    rs := &resources.ResourceSet{
        ExternalResources: []resources.ExternalResourceResource{
            {
                Ref:          "test-portal",
                ResourceType: "portal",
                ID:           "portal-123", // Direct ID for testing
            },
        },
        APIs: []resources.APIResource{
            {
                Name: "Test API",
                PortalRef: "test-portal", // Reference to external resource
            },
        },
    }
    
    // Test plan generation
    plan, err := planner.GeneratePlan(context.Background(), rs, GeneratePlanOptions{})
    
    // Verify external resource resolution was called
    // (This test would need mock adapters for full verification)
    assert.NoError(t, err)
    assert.NotNil(t, plan)
}
```

### Phase 4: Error Handling and Edge Cases

#### Step 4.1: Comprehensive Error Handling

Ensure all error scenarios are covered with helpful messages:

1. **Zero Matches**: Clear message with suggestions
2. **Multiple Matches**: Specific count with guidance
3. **Parent Resolution Failures**: Context about parent-child relationships
4. **SDK Errors**: Network, authentication, and API errors
5. **Configuration Errors**: Invalid selectors, missing fields
6. **Circular Dependencies**: Clear detection and reporting

#### Step 4.2: Performance Optimizations

1. **Caching**: In-memory cache for resolved resources
2. **Batch Operations**: Group similar queries where possible
3. **Parallel Resolution**: Resolve independent resources concurrently
4. **Timeout Handling**: Proper context cancellation

#### Step 4.3: Logging and Debugging

Add comprehensive logging at different levels:

```go
// Debug: Detailed resolution steps
r.logger.Debug("Resolving external resource", "ref", resource.Ref, "type", resource.ResourceType)

// Info: Summary information
r.logger.Info("External resource resolution completed", "resolved_count", len(r.resolved))

// Warn: Performance concerns
r.logger.Warn("Large number of external resources may impact performance", "count", len(externalResources))

// Error: Resolution failures
r.logger.Error("External resource resolution failed", "ref", resource.Ref, "error", err)
```

## Technical Design Details

### Core Data Structures

#### ExternalResourceResolver
- **Purpose**: Central coordination of external resource resolution
- **Dependencies**: ResolutionRegistry, state.Client, logger
- **Cache**: In-memory map for resolved resources
- **Lifecycle**: Single plan generation cycle

#### ResolvedExternalResource
- **ID**: Resolved Konnect ID for references
- **Resource**: Full SDK response object for metadata
- **Parent**: Parent resource reference for hierarchy
- **Metadata**: Debugging and audit information

#### DependencyGraph
- **Nodes**: Map of all external resources
- **ResolutionOrder**: Topologically sorted order
- **Validation**: Parent-child relationship checking

### Resolution Algorithm

1. **Parse**: Extract external resources from ResourceSet
2. **Graph**: Build dependency graph with parent-child relationships
3. **Sort**: Topological sort for resolution order
4. **Resolve**: Iterate through ordered list:
   - Get appropriate adapter from registry
   - Prepare parent context if needed
   - Execute SDK query (ID or selector)
   - Validate exactly one match for selectors
   - Store resolved resource in cache
   - Update original resource with resolved ID
5. **Cache**: Make resolved IDs available for reference resolution

### Error Handling Strategy

#### Error Categories
1. **Configuration Errors**: Invalid selectors, missing fields
2. **Resolution Errors**: Zero/multiple matches, SDK failures
3. **Dependency Errors**: Missing parents, circular dependencies
4. **Infrastructure Errors**: Network, authentication, API errors

#### Error Format
```
Error: [Brief description]
  Context: [Specific details]
  Resource: [Resource information]
  Suggestion: [Actionable guidance]
```

### Caching Mechanism

#### Cache Scope
- **Lifecycle**: Single plan generation cycle
- **Storage**: In-memory map by reference
- **Content**: Resolved ID + full resource object

#### Cache Benefits
- **Performance**: No duplicate SDK calls
- **Consistency**: Same resource resolved once
- **References**: Fast lookup for reference resolution

## Implementation Sequence

### Week 1: Core Implementation
1. **Day 1-2**: Create core types and ExternalResourceResolver struct
2. **Day 3-4**: Implement resolution logic and adapter integration
3. **Day 5**: Dependency graph implementation

### Week 2: Integration and Testing
1. **Day 1-2**: Planner integration and reference resolution
2. **Day 3-4**: Unit tests for all components
3. **Day 5**: Integration tests and error handling

### Week 3: Polish and Optimization
1. **Day 1-2**: Comprehensive error handling and logging
2. **Day 3-4**: Performance optimization and caching
3. **Day 5**: Documentation and final testing

## Risk Mitigation Strategies

### Medium Risks

#### Dependency Resolution Complexity
- **Mitigation**: Comprehensive validation using registry metadata
- **Testing**: Extensive unit tests with complex parent-child scenarios
- **Monitoring**: Clear error messages for invalid relationships

#### Performance with Multiple Resources
- **Mitigation**: Efficient caching and batch operations where possible
- **Testing**: Performance tests with realistic resource counts
- **Optimization**: Parallel resolution for independent resources

### High Risks

#### Breaking Changes to Planner
- **Mitigation**: Minimal changes to existing planner flow
- **Testing**: Comprehensive integration tests
- **Rollback**: Feature flag for external resource resolution

#### Reference Resolution Integration
- **Mitigation**: Clear separation between internal and external resolution
- **Testing**: End-to-end tests with mixed reference types
- **Fallback**: Existing logic preserved as fallback

## Success Criteria

### Functional Requirements
- [ ] All 13 resource types resolve correctly via adapters
- [ ] Parent-child dependencies handled properly
- [ ] Direct ID references work without SDK calls
- [ ] Selector-based resolution with exact match validation
- [ ] Resolved IDs available for reference resolution
- [ ] Integration with existing planner flow

### Error Handling Requirements
- [ ] Clear messages for zero matches with suggestions
- [ ] Clear messages for multiple matches with guidance
- [ ] Parent resolution failure handling
- [ ] SDK error propagation with context
- [ ] No silent failures or incorrect behavior

### Performance Requirements
- [ ] Resolution completes in under 10 seconds for 50 external resources
- [ ] No duplicate SDK calls for same resource
- [ ] Efficient memory usage for resolved resource cache
- [ ] Proper context cancellation and timeout handling

### Integration Requirements
- [ ] No breaking changes to existing planner functionality
- [ ] Seamless reference resolution for external resources
- [ ] Consistent behavior across all resource types
- [ ] Backward compatibility maintained

### Testing Requirements
- [ ] 100% test coverage for new resolver components
- [ ] Integration tests with mock adapters
- [ ] End-to-end tests with planner flow
- [ ] Error scenario coverage
- [ ] Performance test validation

## File Checklist

### New Files to Create
- [ ] `/internal/declarative/external/resolver.go`
- [ ] `/internal/declarative/external/resolver_test.go`
- [ ] `/internal/declarative/external/dependencies.go`
- [ ] `/internal/declarative/external/dependencies_test.go`
- [ ] `/internal/declarative/external/integration_test.go`
- [ ] `/internal/declarative/planner/external_integration_test.go`

### Files to Modify
- [ ] `/internal/declarative/external/types.go` (add resolver types)
- [ ] `/internal/declarative/planner/planner.go` (integration)
- [ ] `/internal/declarative/planner/resolver.go` (reference enhancement)

### Quality Gates
- [ ] `make build` succeeds
- [ ] `make lint` passes with zero issues
- [ ] `make test` passes all tests
- [ ] `make test-integration` passes (when applicable)

## Implementation Notes

### Development Guidelines
- Follow existing error handling patterns with wrapped errors
- Use structured logging with consistent field names
- Maintain backward compatibility with existing functionality
- Write comprehensive tests with good coverage
- Document complex algorithms and integration points

### Testing Strategy
- Unit tests for all public methods and error scenarios
- Integration tests with mock adapters for controlled testing
- End-to-end tests with real planner flow
- Performance tests for scalability validation
- Error scenario coverage for all failure modes

### Code Review Focus Areas
- Error handling comprehensiveness and clarity
- Performance implications of resolution algorithm
- Integration points with existing systems
- Test coverage and quality
- Documentation and code clarity

This implementation plan provides a comprehensive roadmap for implementing Step 3 of the external resources feature. The plan is detailed enough for direct implementation while maintaining flexibility for implementation details and optimizations.