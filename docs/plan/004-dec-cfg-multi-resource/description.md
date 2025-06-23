# KongCtl Stage 4 - Multi-Resource Support

## Goal
Extend to portal child resources (pages, specs) with dependency handling.

## Deliverables
- Support for portal_pages and portal_specs resource types
- Dependency resolution between resources
- Cross-file reference validation
- Enhanced diff output showing dependencies

## Implementation Details

### Suggested Extended Configuration Format
```yaml
# portals.yaml
portals:
  - name: developer-portal
    display_name: "Kong Developer Portal"
    description: "Main developer portal"
    labels:
      team: platform

# pages.yaml
portal_pages:
  - name: getting-started
    portal: developer-portal  # Reference by name
    slug: /getting-started
    title: "Getting Started Guide"
    content: |
      # Getting Started
      Welcome to our API documentation...
    visibility: public
    status: published

# specs.yaml  
portal_specs:
  - name: users-api-v1
    portal: developer-portal
    spec_file: ./openapi/users-v1.yaml
    title: "Users API v1"
    description: "User management API"
```

### Resource Interfaces
```go
// internal/declarative/resources/interfaces.go
type Resource interface {
    GetKind() string
    GetName() string
    GetDependencies() []ResourceRef
    Validate() error
}

type ResourceRef struct {
    Kind string
    Name string
}

// internal/declarative/resources/portal_page.go
type PortalPageResource struct {
    components.CreatePortalPageRequest `yaml:",inline"`
    Name   string `yaml:"name"`
    Portal string `yaml:"portal"` // Portal name reference
}

func (p PortalPageResource) GetKind() string { return "portal_page" }
func (p PortalPageResource) GetName() string { return p.Name }
func (p PortalPageResource) GetDependencies() []ResourceRef {
    return []ResourceRef{{Kind: "portal", Name: p.Portal}}
}

// internal/declarative/resources/portal_spec.go
type PortalSpecResource struct {
    Name        string `yaml:"name"`
    Portal      string `yaml:"portal"`
    SpecFile    string `yaml:"spec_file"`
    Title       string `yaml:"title"`
    Description string `yaml:"description"`
}
```

### Suggested Extended Resource Set Structure
```go
// internal/declarative/resources/resource_set.go
type ResourceSet struct {
    Portals     []PortalResource     `yaml:"portals,omitempty"`
    PortalPages []PortalPageResource `yaml:"portal_pages,omitempty"`
    PortalSpecs []PortalSpecResource `yaml:"portal_specs,omitempty"`
}

func (rs *ResourceSet) GetAllResources() []Resource {
    resources := []Resource{}
    for _, p := range rs.Portals {
        resources = append(resources, p)
    }
    for _, pp := range rs.PortalPages {
        resources = append(resources, pp)
    }
    for _, ps := range rs.PortalSpecs {
        resources = append(resources, ps)
    }
    return resources
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
    
    // Build node map
    for _, r := range resources {
        key := fmt.Sprintf("%s:%s", r.GetKind(), r.GetName())
        graph.nodes[key] = r
    }
    
    // Build edges
    for _, r := range resources {
        fromKey := fmt.Sprintf("%s:%s", r.GetKind(), r.GetName())
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

func (g *DependencyGraph) TopologicalSort() ([]Resource, error) {
    // Implementation of topological sort
    // Returns resources in order they should be created
}
```

### Suggested Enhanced Planner
```go
// internal/declarative/planner/planner.go
func (p *Planner) GeneratePlan(ctx context.Context, resourceSet *ResourceSet) (*Plan, error) {
    // Build dependency graph first
    resources := resourceSet.GetAllResources()
    depGraph, err := BuildDependencyGraph(resources)
    if err != nil {
        return nil, fmt.Errorf("dependency resolution failed: %w", err)
    }
    
    // Get resources in dependency order
    orderedResources, err := depGraph.TopologicalSort()
    
    plan := &Plan{
        Metadata: generateMetadata(),
        Changes:  []PlannedChange{},
    }
    
    // Process each resource type
    for _, resource := range orderedResources {
        switch r := resource.(type) {
        case PortalResource:
            change := p.planPortal(ctx, r)
            if change != nil {
                plan.Changes = append(plan.Changes, *change)
            }
        case PortalPageResource:
            change := p.planPortalPage(ctx, r)
            if change != nil {
                plan.Changes = append(plan.Changes, *change)
            }
        case PortalSpecResource:
            change := p.planPortalSpec(ctx, r)
            if change != nil {
                plan.Changes = append(plan.Changes, *change)
            }
        }
    }
    
    // Set execution order based on dependencies
    plan.ExecutionOrder = determineExecutionOrder(plan.Changes, depGraph)
    
    return plan, nil
}
```

### Enhanced Diff Output
```go
// internal/cmd/root/verbs/diff/display.go
func displayHumanReadableDiff(plan *Plan) error {
    // Group changes by resource type
    changesByType := groupChangesByType(plan.Changes)
    
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
// internal/declarative/executor/executor.go
func (e *Executor) Execute(ctx context.Context, plan *Plan) (*ExecutionResult, error) {
    result := &ExecutionResult{}
    
    // Execute in dependency order
    for _, changeID := range plan.ExecutionOrder {
        change := findChangeByID(plan.Changes, changeID)
        
        // Check dependencies are satisfied
        if !e.areDependenciesSatisfied(change, result.CompletedChanges) {
            return nil, fmt.Errorf("dependency not satisfied for %s", change.ResourceName)
        }
        
        if err := e.executeChange(ctx, change); err != nil {
            result.Errors = append(result.Errors, ExecutionError{
                ChangeID: change.ID,
                Error:    err,
            })
            result.FailureCount++
            
            // Fail fast on dependency failures
            if hasDownstreamDependencies(change, plan) {
                return result, fmt.Errorf("stopping execution due to failed dependency")
            }
        } else {
            result.SuccessCount++
            result.CompletedChanges = append(result.CompletedChanges, changeID)
        }
    }
    
    return result, nil
}
```

## Tests Required
- Dependency graph construction and cycle detection
- Topological sort correctness
- Cross-file reference validation
- Multi-resource plan generation
- Execution order verification
- Parent-child resource relationships

## Proof of Success
```bash
# Manage a portal with pages
$ kongctl apply
Validating configuration...
✓ Dependencies resolved successfully

Executing plan...
✓ Created portal: developer-portal
✓ Created page: getting-started (parent: developer-portal)
✓ Created spec: users-api-v1 (parent: developer-portal)
Plan applied successfully: 3 resources created

# Show dependencies in diff
$ kongctl diff
Plan Summary:
  3 resource(s) to create
  0 resource(s) to update

Changes:
+ portal: developer-portal
  
+ portal_page: getting-started
  Dependencies:
    - portal:developer-portal
    
+ portal_spec: users-api-v1
  Dependencies:
    - portal:developer-portal

# Handle missing dependencies
$ kongctl plan
Error: dependency resolution failed: resource portal_page:about depends on non-existent portal:missing-portal
```

## Dependencies
- Stage 3 completion (plan execution)
- Understanding of portal child resource APIs
- File I/O for spec file loading

## Notes
- Resources reference each other by name, not ID
- Dependency order is critical for creation
- Reverse dependency order for future deletion support
- Consider parallel execution for independent resources
- Validate spec files exist before plan execution