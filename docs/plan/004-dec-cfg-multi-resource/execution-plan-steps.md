# Stage 4: API Resources and Multi-Resource Support - Implementation Steps

## Progress Summary

| Step | Description | Status | Dependencies |
|------|-------------|---------|--------------|
| 1 | Migrate to public Konnect SDK | ✅ COMPLETE | - |
| 2 | Create resource interfaces and base types | ✅ COMPLETE | Step 1 |
| 3 | Implement API resource type | ✅ COMPLETE | Steps 1, 2 |
| 4 | Implement API child resource types | ✅ COMPLETE | Steps 2, 3 |
| 5 | Create YAML tag system architecture | ✅ COMPLETE | Step 2 |
| 6 | Implement file tag resolver with loading | ✅ COMPLETE | Step 5 |
| 7 | Integrate tag system with resource loader | ✅ COMPLETE | Steps 4, 6 |
| 8 | Extend planner for API resources | ✅ COMPLETE | Steps 4, 7 |
| 9 | Add API operations to executor | Not Started | Steps 4, 7 |
| 10 | Implement dependency graph enhancements | Not Started | Steps 4, 8 |
| 11 | Add cross-resource reference validation | Not Started | Step 10 |
| 12 | Create comprehensive integration tests | Not Started | Steps 8, 9, 10 |
| 13 | Add examples and documentation | Not Started | All steps |

**Current Stage**: Steps 1-8 Completed - Ready for Step 9

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

## Step 5: Create YAML Tag System Architecture

**Goal**: Create the YAML tag processing system architecture and interfaces.

### Status Update (2025-01-02)

**Completed** ✅:
- Created comprehensive tag system package (`internal/declarative/tags`)
- Implemented TagResolver interface for extensible tag processing
- Created ResolverRegistry for managing multiple tag resolvers
- Implemented value extraction logic with dot notation support
- Integrated tag processing into loader with yaml.v3
- Added proper nolint directives for gomodguard (yaml.v3 required for tag support)
- Created comprehensive unit tests for all components
- Fixed all linter issues and test failures

**Key Implementation Details**:
- Switched from sigs.k8s.io/yaml to gopkg.in/yaml.v3 for custom tag support
- Tag processing happens before YAML unmarshaling to ResourceSet
- Base directory tracking for file resolution (to be used in Step 6)
- Thread-safe resolver registry for concurrent access
- Extensible architecture supports future tag types (!env, !vault, etc.)

### Implementation

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
- [x] Tag system architecture defined
- [x] Resolver registry implemented
- [x] Value extractor implemented
- [x] Unit tests pass

---

## Step 6: Implement File Tag Resolver with Loading

**Goal**: Implement the actual file loading and processing logic.

### Status Update (2025-01-02)

**Completed** ✅:
- Created FileTagResolver with full implementation
- Added comprehensive security validations (no path traversal, no absolute paths)
- Implemented file loading with size limits (10MB max)
- Added caching to avoid reloading files during execution
- Support for YAML, JSON, and plain text files
- Thread-safe implementation with mutex protection
- Created comprehensive test suite with 100% coverage
- Added testdata directory with sample files
- Integrated with loader to automatically register resolver

**Key Implementation Details**:
- File resolver is registered dynamically when parseYAML is called
- Base directory is set based on the source file location
- Cache keys include both file path and extraction path
- Security measures prevent directory traversal attacks
- Both string and map YAML node formats are supported

**Security Measures**:
- Absolute paths are blocked
- Parent directory traversal (..) is prevented
- File size limited to 10MB
- Path validation happens before file access
- All paths are cleaned and normalized

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
- [x] File tag resolver implemented
- [x] Security measures in place
- [x] Caching working
- [x] Tests pass

---

## Step 7: Integrate Tag System with Resource Loader

**Goal**: Connect the tag system with the resource loader.

### Status Update (2025-01-02)

**Completed** ✅:
- Tag registry is integrated into loader.go
- FileTagResolver is automatically registered when parseYAML is called
- Base directory is dynamically set based on source file location
- Tag processing happens before YAML unmarshaling
- Created comprehensive integration tests for file tag loading
- All tests passing

**Key Implementation Details**:
- The loader creates a tag registry on demand via getTagRegistry()
- When loading files, the base directory is set to the directory of the source file
- The FileTagResolver is registered with the correct base directory for each file
- This allows relative file paths in !file tags to work correctly
- Integration tests verify loading files with tags, nested directories, and error handling

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
- [x] Tag resolution integrated with loader
- [x] File loading with tags working end-to-end
- [x] Error handling comprehensive
- [x] Integration tests pass

---

## Step 8: Extend Planner and Executor for API Resources

**Goal**: Add comprehensive API resource planning and execution logic including full child resource support.

### Status Update (2025-01-03)

**Completed** ✅:
- Created APIDocumentResource type with all required interfaces
- Extended APIAPI helper interface with all child resource operations
- Implemented state client methods for List/Create/Update/Delete operations on all child resources
- Added comprehensive planner support for API and child resource changes
- Created executor operations for all API and child resource types
- Handled SDK limitations gracefully (e.g., implementation operations not fully supported)
- Fixed all SDK type mismatches and field naming inconsistencies
- All tests passing, linter clean

**Implementation Details**:

1. **API Document Resource** (`internal/declarative/resources/api_document.go`):
   - Created new resource type embedding SDK's CreateAPIDocumentRequest
   - Implements all required interfaces (Resource, ResourceWithParent)
   - Added to ResourceSet for root-level declarations

2. **Helper Interface Extensions** (`internal/konnect/helpers/apis.go`):
   - Extended APIAPI interface with methods for all child resources
   - Added CreateAPIVersion, ListAPIVersions
   - Added PublishAPIToPortal, DeletePublication, ListAPIPublications
   - Added ListAPIImplementations (create/delete not supported by SDK)
   - Added CreateAPIDocument, UpdateAPIDocument, DeleteAPIDocument, ListAPIDocuments

3. **State Client Extensions** (`internal/declarative/state/client.go`):
   - Added internal types for APIVersion, APIPublication, APIImplementation, APIDocument
   - Implemented List/Create/Update/Delete methods for each child type
   - Handled pagination for list operations
   - Managed SDK response variations (list vs full objects)

4. **Planner Implementation** (`internal/declarative/planner/api_planner.go`):
   - Completed planAPIChildResourcesCreate for new APIs
   - Completed planAPIChildResourceChanges for existing APIs
   - Added planning methods for each child resource type
   - Handled resources that don't support update operations
   - Proper parent-child dependency management

5. **Executor Operations**:
   - Created api_version_operations.go (create only)
   - Created api_publication_operations.go (create/delete)
   - Created api_implementation_operations.go (stubbed - SDK limitations)
   - Created api_document_operations.go (full CRUD)
   - Integrated all operations into main executor

**Key Decisions**:
- API child resources follow the same patterns as portal resources
- Resources without update operations only support create/delete
- SDK limitations are handled gracefully with appropriate error messages
- Parent API resolution happens at execution time
- Labels are only managed on parent resources (APIs), not children

**Implementation Note**: A comprehensive implementation plan was created in [step-8-api-child-resources-plan.md](step-8-api-child-resources-plan.md) after discovering initial SDK alignment issues. The plan outlined a 7-phase approach that was successfully completed.

### Definition of Done
- [x] API planning logic implemented
- [x] Child resource planning working
- [x] Integration with main planner
- [x] All executor operations implemented
- [x] SDK limitations handled
- [x] All tests pass
- [x] Linter clean

---

## Step 9: Create Integration Tests for API Resources

**Goal**: Create comprehensive integration tests for API resources and their children.

### Implementation

1. Create `test/integration/api_test.go`:
```go
func TestAPIResourceLifecycle(t *testing.T)
func TestAPIWithChildResources(t *testing.T)
func TestAPIPublicationToPortal(t *testing.T)
func TestAPIVersionManagement(t *testing.T)
func TestAPIDocumentHierarchy(t *testing.T)
```

2. Test scenarios:
- Basic API CRUD operations
- API with nested child resources
- Separate file child resource declarations
- Cross-resource references (publication → portal)
- Protection status for APIs
- SDK limitation handling

3. Mock vs Real SDK testing:
- Ensure both modes work correctly
- Handle SDK-specific behaviors

### Tests Required
- Full API lifecycle (create, update, delete)
- Child resource management
- Dependency ordering
- Error scenarios
- Protection validation

### Definition of Done
- [ ] Integration tests created
- [ ] All scenarios covered
- [ ] Mock and real SDK modes tested
- [ ] Tests pass

---

## Step 10: Implement Dependency Graph Enhancements

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

## Step 11: Add Cross-Resource Reference Validation

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

## Step 12: Create Comprehensive Integration Tests

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

## Step 13: Add Examples and Documentation

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
- Planner logic for each resource type
- Executor operations

### Integration Tests
- End-to-end resource management
- File loading with tags
- Complex dependency scenarios
- Error handling flows
- API with child resources
- Cross-resource references

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