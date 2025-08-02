# Stage 4: API Resources and Multi-Resource Support - Technical Overview

## Overview

Stage 4 extends kongctl's declarative configuration to support API resources and their child resources with full dependency handling. This stage introduces external content loading through YAML tags with value extraction, enabling users to reference external files and extract specific fields from them.

## Core Capabilities

### 1. API Resource Management
- Full CRUD operations for API resources
- Support for API specifications (OpenAPI/AsyncAPI)
- Label management and protection handling
- Configuration-based change detection

### 2. API Child Resources
- **API Versions**: Multiple versions per API with specification content
- **API Publications**: Publishing APIs to portals with visibility control
- **API Implementations**: Linking APIs to Gateway services

### 3. External Content Loading with YAML Tags
- Load content from external files
- Extract specific fields using path notation
- Support for various file formats (YAML, JSON, text)
- Extensible tag system for future sources

### 4. External ID References
- Support for control plane IDs (external references)
- Support for service IDs (external references)
- Enables integration without full core entity management

### 5. Dependency Resolution
- Automatic dependency detection
- Topological sorting for execution order
- Cross-resource reference validation
- Parent-child relationship handling

## Technical Architecture

### Resource Interface Design

All resources will implement a common interface:

```go
type Resource interface {
    GetKind() string
    GetRef() string
    GetDependencies() []ResourceRef
    Validate() error
    SetDefaults()
}

type ResourceRef struct {
    Kind string
    Ref  string
}
```

### YAML Tag System

Custom YAML tags for external content loading:

```yaml
# Simple file loading
spec_content: !file ./specs/openapi.yaml

# With value extraction
name: !file
  path: ./specs/openapi.yaml
  extract: info.title

# Shorthand for extraction
version: !file.extract [./specs/openapi.yaml, info.version]

# Environment variables (future)
api_key: !env API_KEY
```

### Tag Processing Pipeline

1. **Parse Phase**: YAML parser identifies custom tags
2. **Resolution Phase**: Tag resolvers load and process content
3. **Validation Phase**: Validate loaded content and types
4. **Integration Phase**: Merge resolved values into resources

### Dependency Graph

```
API (users-api)
├── API Version (v1)
├── API Version (v2)
├── API Publication (to developer-portal)
│   └── depends on: Portal (developer-portal)
└── API Implementation (users-impl)
    └── references: Control Plane (external)
                   Service (external)
```

## Implementation Components

### 1. Resource Types
- `internal/declarative/resources/api.go`
- `internal/declarative/resources/api_version.go`
- `internal/declarative/resources/api_publication.go`
- `internal/declarative/resources/api_implementation.go`

### 2. Tag System
- `internal/declarative/tags/resolver.go` - Main tag resolution engine
- `internal/declarative/tags/file.go` - File tag implementation
- `internal/declarative/tags/extractor.go` - Path-based value extraction

### 3. Planner Extensions
- `internal/declarative/planner/api_planner.go` - API-specific planning logic
- Extended dependency resolver for new resource types

### 4. Executor Extensions
- `internal/declarative/executor/api_operations.go` - API CRUD operations
- Resource-specific validation and error handling

## Configuration Examples

### Basic API with Inline Spec
```yaml
apis:
  - ref: users-api
    name: "Users API"
    description: "User management API"
    spec_content: |
      openapi: 3.0.0
      info:
        title: Users API
        version: 1.0.0
```

### API with External Spec
```yaml
apis:
  - ref: users-api
    name: !file
      path: ./specs/users-api.yaml
      extract: info.title
    description: !file
      path: ./specs/users-api.yaml
      extract: info.description
    spec_content: !file ./specs/users-api.yaml
```

### Complete Multi-Resource Example
```yaml
apis:
  - ref: users-api
    name: "Users API"
    spec_content: !file ./specs/users-api.yaml
    
    versions:
      - ref: v1
        version: "1.0.0"
        spec:
          content: !file ./specs/users-api-v1.yaml
      
      - ref: v2
        version: "2.0.0"
        spec:
          content: !file ./specs/users-api-v2.yaml
    
    publications:
      - ref: public-portal-pub
        portal: public-portal
        version: v2
        visibility: public
    
    implementations:
      - ref: prod-impl
        service:
          control_plane_id: "prod-cp"
          id: "users-service-v2"
```

## Error Handling

### File Loading Errors
- File not found → Clear error with path
- Parse errors → Show line/column if available
- Invalid extraction path → Show available paths

### Dependency Errors
- Missing dependencies → List all missing resources
- Circular dependencies → Show the cycle
- Invalid references → Suggest available options

## Performance Considerations

### File Loading
- Cache loaded files within a plan execution
- Validate file size limits
- Stream large files if needed

### Dependency Resolution
- Efficient graph algorithms (O(V+E))
- Early cycle detection
- Parallel execution where possible

## Security Considerations

### File Access
- Restrict to relative paths by default
- No parent directory traversal
- Validate file permissions

### Content Validation
- Size limits on loaded content
- Timeout for file operations
- Sanitize extracted values

## Future Extensibility

The YAML tag system is designed for extension:
- `!env` - Environment variables
- `!vault` - Secret management integration
- `!http` - Remote content loading
- `!template` - Template processing
- `!merge` - Merge multiple sources

## Success Metrics

- Support all API resource operations
- Handle complex dependency graphs
- Efficient file loading and caching
- Clear error messages
- Comprehensive test coverage