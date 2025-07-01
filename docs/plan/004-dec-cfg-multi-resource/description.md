# KongCtl Stage 4 - API Resources and Multi-Resource Support

## Goal
Extend declarative configuration to support API resources and their child resources (versions, publications, implementations) with dependency handling.

## Deliverables
- Support for API resource type and all child resources
- Dependency resolution between resources
- Cross-file reference validation
- Enhanced diff output showing dependencies
- Nested resource configuration support

## Implementation Details

### Extended Configuration Format
```yaml
# apis.yaml
apis:
  - ref: users-api
    name: "Users API"
    description: "User management API"
    labels:
      team: platform
    kongctl:
      protected: false
    
    # Nested child resources
    versions:
      - ref: v1
        name: "v1.0.0"
        gateway_service: users-service-v1
        
      - ref: v2
        name: "v2.0.0"
        gateway_service: users-service-v2
        deprecated: true
    
    publications:
      - ref: dev-portal-pub
        portal: developer-portal  # Reference to portal
        version: v2  # Reference to version
        
    implementations:
      - ref: users-impl
        type: proxy
        config:
          upstream: "http://users.internal"
          route_config:
            paths:
              - /users
              - /users/*

# Alternative: separate files
# api-versions.yaml
api_versions:
  - ref: users-v1
    api: users-api  # Reference by name
    name: "v1.0.0"
    gateway_service: users-service-v1

# api-publications.yaml  
api_publications:
  - ref: users-pub
    api: users-api
    portal: developer-portal
    version: users-v1
    auto_publish: true
```

### Resource Types
```go
// API resource already exists in internal/declarative/resources/api.go
// Need to add child resources:

// internal/declarative/resources/api_version.go
type APIVersionResource struct {
    kkInternalComps.CreateAPIVersionRequest `yaml:",inline" json:",inline"`
    Ref string `yaml:"ref" json:"ref"`
    API string `yaml:"api,omitempty" json:"api,omitempty"` // Parent API reference
    Kongctl *KongctlMeta `yaml:"kongctl,omitempty" json:"kongctl,omitempty"`
}

// internal/declarative/resources/api_publication.go
type APIPublicationResource struct {
    kkInternalComps.CreateAPIPublicationRequest `yaml:",inline" json:",inline"`
    Ref     string `yaml:"ref" json:"ref"`
    API     string `yaml:"api,omitempty" json:"api,omitempty"`
    Portal  string `yaml:"portal" json:"portal"` // Reference to portal
    Version string `yaml:"version" json:"version"` // Reference to version
    Kongctl *KongctlMeta `yaml:"kongctl,omitempty" json:"kongctl,omitempty"`
}

// internal/declarative/resources/api_implementation.go
type APIImplementationResource struct {
    kkInternalComps.CreateAPIImplementationRequest `yaml:",inline" json:",inline"`
    Ref     string `yaml:"ref" json:"ref"`
    API     string `yaml:"api,omitempty" json:"api,omitempty"`
    Type    string `yaml:"type" json:"type"` // proxy, mock, etc.
    Config  map[string]interface{} `yaml:"config" json:"config"`
    Kongctl *KongctlMeta `yaml:"kongctl,omitempty" json:"kongctl,omitempty"`
}
```

### Resource Interfaces
```go
// internal/declarative/resources/interfaces.go
type Resource interface {
    GetKind() string
    GetName() string
    GetRef() string
    GetDependencies() []ResourceRef
    Validate() error
}

type ResourceRef struct {
    Kind string
    Name string
}

// Example implementation for API Version
func (v APIVersionResource) GetKind() string { return "api_version" }
func (v APIVersionResource) GetRef() string { return v.Ref }
func (v APIVersionResource) GetDependencies() []ResourceRef {
    deps := []ResourceRef{}
    if v.API != "" {
        deps = append(deps, ResourceRef{Kind: "api", Name: v.API})
    }
    return deps
}

// Example for API Publication (depends on API, Portal, and Version)
func (p APIPublicationResource) GetDependencies() []ResourceRef {
    return []ResourceRef{
        {Kind: "api", Name: p.API},
        {Kind: "portal", Name: p.Portal},
        {Kind: "api_version", Name: p.Version},
    }
}
```

### Extended Resource Set Structure
```go
// internal/declarative/resources/resource_set.go
type ResourceSet struct {
    Portals               []PortalResource               `yaml:"portals,omitempty"`
    ApplicationAuthStrategies []ApplicationAuthStrategyResource `yaml:"application_auth_strategies,omitempty"`
    ControlPlanes         []ControlPlaneResource         `yaml:"control_planes,omitempty"`
    APIs                  []APIResource                  `yaml:"apis,omitempty"`
    // Note: Child resources can be nested under APIs or separate
    APIVersions           []APIVersionResource           `yaml:"api_versions,omitempty"`
    APIPublications       []APIPublicationResource       `yaml:"api_publications,omitempty"`
    APIImplementations    []APIImplementationResource    `yaml:"api_implementations,omitempty"`
}

// Extract nested resources when processing
func (rs *ResourceSet) NormalizeNestedResources() {
    for _, api := range rs.APIs {
        // Extract nested versions and add parent reference
        for _, version := range api.Versions {
            version.API = api.Ref
            rs.APIVersions = append(rs.APIVersions, version)
        }
        // Clear nested to avoid duplication
        api.Versions = nil
        
        // Same for publications and implementations
    }
}
```

### Dependency Resolution
```go
// internal/declarative/planner/dependencies.go
type DependencyGraph struct {
    nodes map[string]Resource
    edges map[string][]string // from -> to
}

func BuildDependencyGraph(resources []Resource) (*DependencyGraph, error) {
    graph := &DependencyGraph{
        nodes: make(map[string]Resource),
        edges: make(map[string][]string),
    }
    
    // Build node map using ref as key
    for _, r := range resources {
        key := fmt.Sprintf("%s:%s", r.GetKind(), r.GetRef())
        graph.nodes[key] = r
    }
    
    // Build edges
    for _, r := range resources {
        fromKey := fmt.Sprintf("%s:%s", r.GetKind(), r.GetRef())
        for _, dep := range r.GetDependencies() {
            toKey := fmt.Sprintf("%s:%s", dep.Kind, dep.Name)
            
            // Validate dependency exists
            if _, exists := graph.nodes[toKey]; !exists {
                return nil, fmt.Errorf("resource %s depends on non-existent %s", 
                    fromKey, toKey)
            }
            
            graph.edges[fromKey] = append(graph.edges[fromKey], toKey)
        }
    }
    
    // Check for cycles
    if graph.hasCycles() {
        return nil, fmt.Errorf("circular dependency detected")
    }
    
    return graph, nil
}
```

### Enhanced Planner
```go
// Extend planner to handle API resources
func (p *Planner) planAPIChanges(ctx context.Context, desired []resources.APIResource, plan *Plan) error {
    // Fetch current APIs
    currentAPIs, err := p.client.ListAPIs(ctx)
    if err != nil {
        return fmt.Errorf("failed to list current APIs: %w", err)
    }
    
    // Similar logic to portal planning
    // Handle CREATE, UPDATE based on presence and changes
    // Check protection status
    // Build planned changes
}

// Add handlers for child resources
func (p *Planner) planAPIVersionChanges(ctx context.Context, desired []resources.APIVersionResource, plan *Plan) error
func (p *Planner) planAPIPublicationChanges(ctx context.Context, desired []resources.APIPublicationResource, plan *Plan) error
func (p *Planner) planAPIImplementationChanges(ctx context.Context, desired []resources.APIImplementationResource, plan *Plan) error
```

### Enhanced Diff Output
```go
func displayHumanReadableDiff(plan *Plan) error {
    fmt.Println("Plan Summary:")
    fmt.Printf("  %d resource(s) to create\n", plan.Summary.ByAction[ActionCreate])
    fmt.Printf("  %d resource(s) to update\n", plan.Summary.ByAction[ActionUpdate])
    fmt.Println()
    
    // Show changes with dependencies
    for _, change := range plan.Changes {
        displayChange(change)
        
        // Show dependencies if any
        if len(change.Dependencies) > 0 {
            fmt.Printf("  Dependencies:\n")
            for _, dep := range change.Dependencies {
                fmt.Printf("    - %s\n", dep)
            }
        }
        fmt.Println()
    }
    
    return nil
}
```

### Extended Executor
```go
// Add API operations to executor
func (e *Executor) createAPI(ctx context.Context, change planner.PlannedChange) error
func (e *Executor) updateAPI(ctx context.Context, change planner.PlannedChange) error
func (e *Executor) deleteAPI(ctx context.Context, change planner.PlannedChange) error

// Add child resource operations
func (e *Executor) createAPIVersion(ctx context.Context, change planner.PlannedChange) error
func (e *Executor) createAPIPublication(ctx context.Context, change planner.PlannedChange) error
func (e *Executor) createAPIImplementation(ctx context.Context, change planner.PlannedChange) error
```

## Tests Required
- API resource CRUD operations
- Nested resource extraction and normalization
- Dependency graph with multi-level dependencies
- Cross-resource reference validation
- Execution order with complex dependencies
- Protection handling for APIs and child resources

## Proof of Success
```bash
# Create API with nested resources
$ kongctl apply
Validating configuration...
✓ Dependencies resolved successfully

Executing plan...
✓ Created api: users-api
✓ Created api_version: v1 (parent: users-api)
✓ Created api_version: v2 (parent: users-api)
✓ Created api_publication: dev-portal-pub (dependencies: users-api, developer-portal, v2)
✓ Created api_implementation: users-impl (parent: users-api)
Plan applied successfully: 5 resources created

# Show dependencies in diff
$ kongctl diff
Plan Summary:
  5 resource(s) to create
  0 resource(s) to update

Changes:
+ api: users-api
  
+ api_version: v1
  Dependencies:
    - api:users-api
    
+ api_version: v2
  Dependencies:
    - api:users-api
    
+ api_publication: dev-portal-pub
  Dependencies:
    - api:users-api
    - portal:developer-portal
    - api_version:v2
    
+ api_implementation: users-impl
  Dependencies:
    - api:users-api

# Handle missing dependencies
$ kongctl plan
Error: dependency resolution failed: resource api_publication:dev-portal-pub depends on non-existent portal:developer-portal
```

## Dependencies
- Stage 3 completion (plan execution infrastructure)
- Understanding of API resource model in Konnect
- SDK support for API operations

## Notes
- Resources reference each other by ref field, not ID
- Support both nested and separate file configurations
- Dependency order is critical for creation
- Reverse dependency order for deletion in sync
- Consider parallel execution for independent resources
- API implementations may have complex configuration schemas