# Stage 4: API Resources and Multi-Resource Support - Implementation Steps

## Progress Summary

| Step | Description | Status | Dependencies |
|------|-------------|---------|--------------|
| 1 | Create resource interfaces and base types | Not Started | - |
| 2 | Implement YAML tag system with value extraction | Not Started | - |
| 3 | Implement API resource type | Not Started | Step 1 |
| 4 | Implement API child resource types | Not Started | Steps 1, 3 |
| 5 | Add file loading with tag resolvers | Not Started | Step 2 |
| 6 | Extend planner for API resources | Not Started | Steps 3, 4 |
| 7 | Add API operations to executor | Not Started | Steps 3, 4 |
| 8 | Implement dependency graph enhancements | Not Started | Steps 3, 4 |
| 9 | Add cross-resource reference validation | Not Started | Step 8 |
| 10 | Create comprehensive integration tests | Not Started | Steps 5, 6, 7 |
| 11 | Add examples and documentation | Not Started | All steps |

**Current Stage**: Not Started

---

## Step 1: Create Resource Interfaces and Base Types

**Goal**: Establish common interfaces for all resources to implement.

### Implementation

1. Create `internal/declarative/resources/interfaces.go`:
```go
package resources

// Resource is the common interface for all declarative resources
type Resource interface {
    GetKind() string
    GetRef() string
    GetName() string
    GetDependencies() []ResourceRef
    Validate() error
    SetDefaults()
}

// ResourceRef represents a reference to another resource
type ResourceRef struct {
    Kind string `json:"kind" yaml:"kind"`
    Ref  string `json:"ref" yaml:"ref"`
}

// ResourceWithParent represents resources that have a parent
type ResourceWithParent interface {
    Resource
    GetParentRef() *ResourceRef
}

// ResourceWithLabels represents resources that support labels
type ResourceWithLabels interface {
    Resource
    GetLabels() map[string]string
    SetLabels(map[string]string)
}
```

2. Update existing resources to implement interfaces:
- Update `PortalResource` to implement `Resource`
- Update `ApplicationAuthStrategyResource` to implement `Resource`

### Tests Required
- Interface compliance tests
- Verify existing resources implement interfaces correctly

### Definition of Done
- [ ] Resource interfaces defined
- [ ] Existing resources updated
- [ ] Interface compliance verified
- [ ] Tests pass

---

## Step 2: Implement YAML Tag System with Value Extraction

**Goal**: Create the YAML tag processing system for external content loading.

### Implementation

1. Create `internal/declarative/tags/types.go`:
```go
package tags

import "gopkg.in/yaml.v3"

// TagResolver processes custom YAML tags
type TagResolver interface {
    Tag() string
    Resolve(node *yaml.Node) (interface{}, error)
}

// FileRef represents a file reference with optional extraction
type FileRef struct {
    Path    string
    Extract string // Optional: path to extract value
}
```

2. Create `internal/declarative/tags/resolver.go`:
```go
// ResolverRegistry manages tag resolvers
type ResolverRegistry struct {
    resolvers map[string]TagResolver
}

func NewResolverRegistry() *ResolverRegistry
func (r *ResolverRegistry) Register(resolver TagResolver)
func (r *ResolverRegistry) Process(data []byte) ([]byte, error)
```

3. Create `internal/declarative/tags/file.go`:
```go
// FileTagResolver handles !file tags
type FileTagResolver struct {
    baseDir string
}

func (f *FileTagResolver) Tag() string { return "!file" }
func (f *FileTagResolver) Resolve(node *yaml.Node) (interface{}, error)
```

4. Create `internal/declarative/tags/extractor.go`:
```go
// ExtractValue extracts a value from structured data using path notation
func ExtractValue(data interface{}, path string) (interface{}, error)
```

### YAML Tag Formats
```yaml
# Simple file loading
content: !file ./path/to/file.yaml

# With extraction (map format)
value: !file
  path: ./path/to/file.yaml
  extract: field.nested.value

# With extraction (array format)
value: !file.extract [./path/to/file.yaml, field.nested.value]
```

### Tests Required
- File loading from various paths
- Value extraction with different path formats
- Error handling for missing files/paths
- Security tests (path traversal prevention)

### Definition of Done
- [ ] Tag resolver system implemented
- [ ] File tag with extraction working
- [ ] Path-based value extraction functional
- [ ] Security measures in place
- [ ] Comprehensive tests pass

---

## Step 3: Implement API Resource Type

**Goal**: Create the API resource type using SDK models.

### Implementation

1. Update `internal/declarative/resources/api.go`:
```go
type APIResource struct {
    kkInternalComps.CreateAPIRequest `yaml:",inline" json:",inline"`
    Ref     string       `yaml:"ref" json:"ref"`
    Kongctl *KongctlMeta `yaml:"kongctl,omitempty" json:"kongctl,omitempty"`
    
    // Nested child resources
    Versions        []APIVersionResource        `yaml:"versions,omitempty"`
    Publications    []APIPublicationResource    `yaml:"publications,omitempty"`
    Implementations []APIImplementationResource `yaml:"implementations,omitempty"`
}

// Implement Resource interface
func (a APIResource) GetKind() string { return "api" }
func (a APIResource) GetRef() string { return a.Ref }
func (a APIResource) GetName() string { return a.Name }
func (a APIResource) GetDependencies() []ResourceRef { return []ResourceRef{} }
func (a APIResource) Validate() error
func (a *APIResource) SetDefaults()
```

2. Update `internal/declarative/resources/types.go`:
```go
// Add APIs to ResourceSet
type ResourceSet struct {
    // ... existing fields ...
    APIs []APIResource `yaml:"apis,omitempty" json:"apis,omitempty"`
}
```

### Tests Required
- API resource creation and validation
- Default value application
- Nested resource handling

### Definition of Done
- [ ] API resource type implemented
- [ ] Resource interface satisfied
- [ ] Validation logic complete
- [ ] Tests pass

---

## Step 4: Implement API Child Resource Types

**Goal**: Create API child resource types (versions, publications, implementations).

### Implementation

1. Create `internal/declarative/resources/api_version.go`:
```go
type APIVersionResource struct {
    kkInternalComps.CreateAPIVersionRequest `yaml:",inline" json:",inline"`
    Ref string `yaml:"ref" json:"ref"`
    API string `yaml:"api,omitempty"` // Parent API reference
}

func (v APIVersionResource) GetKind() string { return "api_version" }
func (v APIVersionResource) GetDependencies() []ResourceRef {
    if v.API != "" {
        return []ResourceRef{{Kind: "api", Ref: v.API}}
    }
    return []ResourceRef{}
}
```

2. Create `internal/declarative/resources/api_publication.go`:
```go
type APIPublicationResource struct {
    kkInternalComps.APIPublication `yaml:",inline" json:",inline"`
    Ref     string `yaml:"ref" json:"ref"`
    API     string `yaml:"api,omitempty"`
    Portal  string `yaml:"portal"` // Reference to portal
    Version string `yaml:"version"` // Reference to API version
}

func (p APIPublicationResource) GetDependencies() []ResourceRef {
    deps := []ResourceRef{}
    if p.API != "" {
        deps = append(deps, ResourceRef{Kind: "api", Ref: p.API})
    }
    if p.Portal != "" {
        deps = append(deps, ResourceRef{Kind: "portal", Ref: p.Portal})
    }
    if p.Version != "" {
        deps = append(deps, ResourceRef{Kind: "api_version", Ref: p.Version})
    }
    return deps
}
```

3. Create `internal/declarative/resources/api_implementation.go`:
```go
type APIImplementationResource struct {
    Ref     string `yaml:"ref" json:"ref"`
    API     string `yaml:"api,omitempty"`
    Service struct {
        ControlPlaneID string `yaml:"control_plane_id"`
        ID            string `yaml:"id"`
    } `yaml:"service"`
}
```

### Tests Required
- Child resource validation
- Dependency detection
- Reference validation

### Definition of Done
- [ ] All child resource types implemented
- [ ] Resource interfaces satisfied
- [ ] Dependency logic correct
- [ ] Tests pass

---

## Step 5: Add File Loading with Tag Resolvers

**Goal**: Integrate tag resolvers with resource loading.

### Implementation

1. Update `internal/declarative/loader/loader.go`:
```go
func (l *Loader) LoadFiles(filenames []string) (*resources.ResourceSet, error) {
    // Create tag resolver registry
    registry := tags.NewResolverRegistry()
    registry.Register(tags.NewFileTagResolver(l.baseDir))
    
    // Process files with tag resolution
    for _, filename := range filenames {
        data, err := l.loadFileWithTags(filename, registry)
        // ... rest of loading logic
    }
}

func (l *Loader) loadFileWithTags(filename string, registry *tags.ResolverRegistry) ([]byte, error) {
    data, err := os.ReadFile(filename)
    if err != nil {
        return nil, err
    }
    
    // Process custom tags
    return registry.Process(data)
}
```

2. Handle tag resolution errors gracefully
3. Add validation for loaded content

### Tests Required
- Loading files with tags
- Tag resolution in various contexts
- Error handling for invalid tags
- Performance with large files

### Definition of Done
- [ ] Tag resolution integrated with loader
- [ ] File loading with tags working
- [ ] Error handling comprehensive
- [ ] Tests pass

---

## Step 6: Extend Planner for API Resources

**Goal**: Add API resource planning logic.

### Implementation

1. Create `internal/declarative/planner/api_planner.go`:
```go
func (p *Planner) planAPIChanges(ctx context.Context, desired []resources.APIResource, plan *Plan) error {
    // Fetch current APIs
    currentAPIs, err := p.client.ListAPIs(ctx)
    
    // For each desired API
    for _, api := range desired {
        // Check if exists
        // Plan CREATE or UPDATE
        // Handle nested resources
    }
    
    // For sync mode, plan DELETE for unmanaged APIs
}

func (p *Planner) planAPIVersionChanges(ctx context.Context, apiID string, desired []resources.APIVersionResource, plan *Plan) error
func (p *Planner) planAPIPublicationChanges(ctx context.Context, apiID string, desired []resources.APIPublicationResource, plan *Plan) error
func (p *Planner) planAPIImplementationChanges(ctx context.Context, apiID string, desired []resources.APIImplementationResource, plan *Plan) error
```

2. Update main planner to call API planning functions
3. Handle parent-child relationships

### Tests Required
- API plan generation
- Nested resource handling
- Mode-aware planning (apply vs sync)

### Definition of Done
- [ ] API planning logic implemented
- [ ] Child resource planning working
- [ ] Integration with main planner
- [ ] Tests pass

---

## Step 7: Add API Operations to Executor

**Goal**: Implement CRUD operations for API resources.

### Implementation

1. Create `internal/declarative/executor/api_operations.go`:
```go
func (e *Executor) createAPI(ctx context.Context, change planner.PlannedChange) (string, error) {
    // Extract API from fields
    // Add management labels
    // Call SDK to create
    // Return ID
}

func (e *Executor) updateAPI(ctx context.Context, change planner.PlannedChange) (string, error)
func (e *Executor) deleteAPI(ctx context.Context, change planner.PlannedChange) error

// Similar for child resources
func (e *Executor) createAPIVersion(ctx context.Context, change planner.PlannedChange) (string, error)
func (e *Executor) createAPIPublication(ctx context.Context, change planner.PlannedChange) (string, error)
func (e *Executor) createAPIImplementation(ctx context.Context, change planner.PlannedChange) (string, error)
```

2. Update executor's resource switch statements
3. Add proper error handling

### Tests Required
- API CRUD operations
- Label management
- Error handling
- Protection validation

### Definition of Done
- [ ] All API operations implemented
- [ ] Integrated with executor
- [ ] Error handling complete
- [ ] Tests pass

---

## Step 8: Implement Dependency Graph Enhancements

**Goal**: Extend dependency resolver for new resource types.

### Implementation

1. Update `internal/declarative/planner/dependencies.go`:
```go
func (d *DependencyResolver) getParentType(childType string) string {
    switch childType {
    case "api_version", "api_publication", "api_implementation":
        return "api"
    case "portal_page":
        return "portal"
    default:
        return ""
    }
}
```

2. Handle cross-resource dependencies (e.g., publication → portal)
3. Ensure proper deletion order for sync mode

### Tests Required
- Complex dependency graphs
- Cross-resource dependencies
- Cycle detection
- Deletion ordering

### Definition of Done
- [ ] Dependency resolution extended
- [ ] Cross-resource dependencies working
- [ ] Deletion order correct
- [ ] Tests pass

---

## Step 9: Add Cross-Resource Reference Validation

**Goal**: Validate references between resources.

### Implementation

1. Extend reference resolver for API resources
2. Validate portal references from API publications
3. Handle external ID references (control planes, services)
4. Provide helpful error messages for invalid references

### Tests Required
- Valid reference scenarios
- Invalid reference detection
- External ID handling
- Error message clarity

### Definition of Done
- [ ] Reference validation complete
- [ ] External IDs supported
- [ ] Clear error messages
- [ ] Tests pass

---

## Step 10: Create Comprehensive Integration Tests

**Goal**: Test complete multi-resource scenarios.

### Implementation

1. Create `test/integration/api_test.go`:
```go
func TestAPIResourceLifecycle(t *testing.T)
func TestAPIWithChildResources(t *testing.T)
func TestAPIPublicationToPortal(t *testing.T)
func TestExternalFileLoading(t *testing.T)
func TestYAMLTagProcessing(t *testing.T)
```

2. Test complex dependency scenarios
3. Test file loading edge cases
4. Test error scenarios

### Tests Required
- Full resource lifecycle
- Multi-resource plans
- File loading scenarios
- Tag processing edge cases
- Error conditions

### Definition of Done
- [ ] Integration tests comprehensive
- [ ] Edge cases covered
- [ ] Mock and real SDK modes tested
- [ ] All tests pass

---

## Step 11: Add Examples and Documentation

**Goal**: Provide clear examples and documentation.

### Implementation

1. Create example configurations:
- `docs/examples/apis/basic-api.yaml`
- `docs/examples/apis/api-with-versions.yaml`
- `docs/examples/apis/api-with-external-spec.yaml`
- `docs/examples/apis/multi-resource.yaml`

2. Update README with API resource examples
3. Document YAML tag usage
4. Add troubleshooting guide

### Definition of Done
- [ ] Examples cover common scenarios
- [ ] Documentation clear and complete
- [ ] YAML tag usage documented
- [ ] Troubleshooting guide added

---

## Testing Strategy

### Unit Tests
- Resource type validation
- Tag resolver functionality
- Dependency detection
- Value extraction

### Integration Tests
- End-to-end resource management
- File loading with tags
- Complex dependency scenarios
- Error handling flows

### Manual Testing
```bash
# Test basic API creation
./kongctl apply --config examples/apis/basic-api.yaml

# Test with external files
./kongctl apply --config examples/apis/api-with-external-spec.yaml

# Test complex dependencies
./kongctl plan --config examples/apis/multi-resource.yaml
./kongctl apply --plan plan.json

# Test sync mode with deletions
./kongctl sync --config examples/apis/
```

## Notes for Implementers

### Code Quality
- Follow existing patterns from portal implementation
- Maintain consistent error messages
- Add debug logging for tag resolution
- Keep tag system extensible

### Performance
- Cache loaded files within execution
- Validate file sizes before loading
- Use streaming for large spec files

### Security
- Prevent path traversal in file loading
- Validate file permissions
- Sanitize extracted values
- Set reasonable size limits