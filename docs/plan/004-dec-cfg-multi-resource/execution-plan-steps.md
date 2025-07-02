# Stage 4: API Resources and Multi-Resource Support - Implementation Steps

## Progress Summary

| Step | Description | Status | Dependencies |
|------|-------------|---------|--------------|
| 1 | Migrate to public Konnect SDK | ✅ COMPLETE | - |
| 2 | Create resource interfaces and base types | ✅ COMPLETE | Step 1 |
| 3 | Implement API resource type | ✅ COMPLETE | Steps 1, 2 |
| 4 | Implement API child resource types | ✅ COMPLETE | Steps 2, 3 |
| 5 | Create YAML tag system architecture | Not Started | Step 2 |
| 6 | Implement file tag resolver with loading | Not Started | Step 5 |
| 7 | Integrate tag system with resource loader | Not Started | Steps 4, 6 |
| 8 | Extend planner for API resources | Not Started | Steps 4, 7 |
| 9 | Add API operations to executor | Not Started | Steps 4, 7 |
| 10 | Implement dependency graph enhancements | Not Started | Steps 4, 8 |
| 11 | Add cross-resource reference validation | Not Started | Step 10 |
| 12 | Create comprehensive integration tests | Not Started | Steps 8, 9, 10 |
| 13 | Add examples and documentation | Not Started | All steps |

**Current Stage**: Steps 1-4 Completed - Ready for Step 5

---

## Step 1: Migrate to Public Konnect SDK

**Goal**: Replace usage of internal SDK with the public Kong Konnect Go SDK where possible.

### Status Update (2025-01-02)

**Completed** ✅:
- Completely removed internal SDK dependency from project
- Updated SDK from v0.3.1 to v0.6.0 (public SDK only)
- Migrated ALL operations to public SDK (Portal, API, and child resources)
- Updated all helper functions and interfaces
- Fixed SDK compatibility issues throughout codebase
- Updated all unit and integration tests to use public SDK types
- Fixed test mocks and assertions for SDK compatibility
- Updated dump command to use public SDK

**Key Changes**:
- Removed `github.com/Kong/sdk-konnect-go-internal` from go.mod entirely
- Changed all imports from internal to public SDK packages
- Simplified SDK helper interfaces by removing redundant wrapper methods
- Updated API document helper to match public SDK method signatures
- Fixed all compilation errors from SDK migration
- Net reduction of 132 lines of code due to simplified architecture

**Migration Strategy**:
- Public SDK is now the single source of truth for all API schemas and operations
- This aligns with Kong's GA API versioning strategy
- No internal SDK fallback needed - public SDK v0.6.0 has all required APIs

### Implementation

1. Add public SDK dependency:
```bash
go get github.com/Kong/sdk-konnect-go
```

2. Update imports in existing files:
```go
// Replace where possible:
// kkInternalComps "github.com/Kong/sdk-konnect-go-internal/models/components"
// With:
kkComps "github.com/Kong/sdk-konnect-go/models/components"
```

3. Update SDK client initialization:
- Check which APIs are available in public SDK
- Keep internal SDK for APIs not yet public
- Update helpers package to use public SDK where possible

4. Update existing portal operations:
- Portal CRUD operations should use public SDK
- Update type references in executor and planner
- Ensure label handling remains compatible

### Migration Scope
- ✅ Portal operations (available in public SDK)
- ✅ API operations (available in public SDK)
- ❓ Check other operations for public SDK availability
- Keep internal SDK as fallback for missing APIs

### Tests Required
- Verify all existing tests pass with new SDK
- Ensure SDK behavior is consistent
- Check for any breaking changes

### Definition of Done
- [x] Public SDK dependency added
- [x] Portal operations migrated
- [x] API operations migrated to public SDK
- [x] Internal SDK completely removed from project
- [x] All tests pass with new SDK
- [x] Documentation updated

---

## Step 2: Create Resource Interfaces and Base Types

**Goal**: Establish common interfaces for all resources to implement.

### Status Update (2025-01-02)

**Completed** ✅:
- Created comprehensive resource interfaces in `interfaces.go`
- Defined Resource, ResourceWithParent, and ResourceWithLabels interfaces
- Updated PortalResource to implement interfaces
- Updated ApplicationAuthStrategyResource to implement interfaces
- Added interface compliance tests
- All tests passing

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
- [x] Resource interfaces defined
- [x] Existing resources updated (Portal, ApplicationAuthStrategy)
- [x] Interface compliance verified with tests
- [x] All tests pass

---

## Step 3: Implement API Resource Type

**Goal**: Create the API resource type using SDK models.

### Status Update (2025-01-02)

**Completed** ✅:
- Created APIResource type using public SDK (`kkComps.CreateAPIRequest`)
- Implemented Resource interface methods (GetKind, GetRef, GetName, GetDependencies)
- Implemented ResourceWithLabels interface (GetLabels, SetLabels)
- Added APIResource to ResourceSet in types.go
- Updated validator test to use public SDK
- Added interface compliance tests
- Created comprehensive label handling tests

**Key Implementation Details**:
- Public SDK uses `map[string]string` for labels (simpler than portal's pointer maps)
- No type conversion needed for labels
- Minimal test impact - only one import change needed in validator_test.go
- API resource supports nested child resources structure

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
- [x] API resource type implemented
- [x] Resource interface satisfied
- [x] Validation logic complete
- [x] Tests pass

---

## Step 4: Implement API Child Resource Types

**Goal**: Create API child resource types (versions, publications, implementations).

### Status Update (2025-01-02)

**Completed** ✅:
- Implemented dual-mode configuration support (nested and separate files)
- Added root-level arrays to ResourceSet for API child resources
- Added parent API reference field to all child resource types
- Implemented Resource interface for all child resources
- Implemented ResourceWithParent interface where applicable
- Added loader extraction logic to normalize nested resources to root level
- Updated loader merging and duplicate detection
- Created comprehensive tests for both configuration modes

**Key Implementation Details**:
- Child resources can be defined either nested within APIs or separately with parent references
- Loader extracts nested children to root level with parent references for consistent processing
- Supports team ownership model where different teams manage their resources independently
- All child resources implement standard interfaces for consistent handling

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
- [x] All child resource types implemented
- [x] Resource interfaces satisfied
- [x] Dependency logic correct
- [x] Tests pass

---

## Step 4: Create YAML Tag System Architecture

**Goal**: Create the YAML tag processing system architecture and interfaces.

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

3. Create `internal/declarative/tags/extractor.go`:
```go
// ExtractValue extracts a value from structured data using path notation
func ExtractValue(data interface{}, path string) (interface{}, error)
```

### YAML Tag Formats to Support
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
- Tag resolver registration
- Tag processing in YAML
- Value extraction logic
- Error handling

### Definition of Done
- [ ] Tag system architecture defined
- [ ] Resolver registry implemented
- [ ] Value extractor implemented
- [ ] Unit tests pass

---

## Step 5: Implement File Tag Resolver with Loading

**Goal**: Implement the actual file loading and processing logic.

### Implementation

1. Create `internal/declarative/tags/file.go`:
```go
// FileTagResolver handles !file tags
type FileTagResolver struct {
    baseDir string
    cache   map[string]interface{}
}

func NewFileTagResolver(baseDir string) *FileTagResolver
func (f *FileTagResolver) Tag() string { return "!file" }
func (f *FileTagResolver) Resolve(node *yaml.Node) (interface{}, error) {
    // Handle both simple string and map formats
    // Load file content
    // Apply extraction if specified
    // Cache results
}
```

2. Implement file loading with security:
```go
func (f *FileTagResolver) loadFile(path string) ([]byte, error) {
    // Validate path (no traversal)
    // Resolve relative to baseDir
    // Check file size limits
    // Read file
}
```

3. Implement caching for performance:
```go
func (f *FileTagResolver) getCached(path string) (interface{}, bool)
func (f *FileTagResolver) setCached(path string, data interface{})
```

### Security Measures
- Path traversal prevention
- File size limits
- Timeout on file operations
- Restricted to project directory

### Tests Required
- File loading from various paths
- Security validation tests
- Caching behavior
- Error scenarios

### Definition of Done
- [ ] File tag resolver implemented
- [ ] Security measures in place
- [ ] Caching working
- [ ] Tests pass

---

## Step 6: Integrate Tag System with Resource Loader

**Goal**: Connect the tag system with the resource loader.

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
- Integration with existing loader tests

### Definition of Done
- [ ] Tag resolution integrated with loader
- [ ] File loading with tags working end-to-end
- [ ] Error handling comprehensive
- [ ] Integration tests pass

---

## Step 7: Extend Planner for API Resources

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

## Step 8: Add API Operations to Executor

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

## Step 9: Implement Dependency Graph Enhancements

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

## Step 10: Add Cross-Resource Reference Validation

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

## Step 11: Create Comprehensive Integration Tests

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

## Step 12: Add Examples and Documentation

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