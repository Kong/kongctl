# Stage 4: API Resources and Multi-Resource Support - Architecture Decision Records

## ADR-004-001: Resource Interface Design

### Context
We need a common interface for all resources to enable generic operations while maintaining type safety and specific behaviors.

### Decision
Implement a `Resource` interface that all declarative resources must satisfy:
```go
type Resource interface {
    GetKind() string
    GetRef() string
    GetName() string
    GetDependencies() []ResourceRef
    Validate() error
    SetDefaults()
}
```

### Consequences
- **Positive:**
  - Enables generic resource handling in planner and executor
  - Consistent API across all resource types
  - Easy to add new resource types
  - Type-safe with compile-time checking

- **Negative:**
  - Some boilerplate for each resource type
  - May need type assertions in some cases
  - Interface changes affect all resources

### Alternatives Considered
- Empty interface{} with reflection - Too dynamic, loses type safety
- Concrete types only - No generic operations possible
- Inheritance-based approach - Go doesn't support inheritance

---

## ADR-004-002: YAML Tag System for External Content

### Context
Users need to load external content (API specs, documentation) from files and extract specific values. After evaluating multiple syntax options including template functions, special keys, and YAML tags, we need a solution that is:
- Native to YAML
- Extensible for future needs
- Familiar to users
- Clean and readable

### Decision
Implement a YAML tag-based system using YAML's native tag feature:

```yaml
# Simple file loading
spec_content: !file ./specs/openapi.yaml

# With value extraction (map format)
name: !file
  path: ./specs/openapi.yaml
  extract: info.title

# With value extraction (array shorthand)
version: !file.extract [./specs/openapi.yaml, info.version]
```

### Rationale
YAML tags are chosen because:
1. **Part of YAML specification** - Not a custom invention
2. **Familiar pattern** - Similar to CloudFormation (!Ref, !Sub) and Ansible (!vault)
3. **Parse-time handling** - Can be processed during YAML parsing
4. **Extensible** - Easy to add new tags (!env, !vault, !http)
5. **Clean syntax** - Readable for both simple and complex cases
6. **No string parsing** - Unlike template syntax ${...}

### Implementation Details
- Custom YAML unmarshaler to handle tags
- Tag resolver registry for extensibility
- Path-based value extraction using dot notation
- Security restrictions on file access

### Consequences
- **Positive:**
  - Native YAML feature, not custom syntax
  - Extensible for future sources
  - Clear intent with ! prefix
  - Works with any YAML parser that supports tags
  - Familiar to CloudFormation/Ansible users

- **Negative:**
  - Less IDE support for custom tags initially
  - Learning curve for users unfamiliar with YAML tags
  - Need careful error messages for invalid usage

### Alternatives Considered

1. **Template Function Syntax** (like Terraform)
   ```yaml
   name: ${file("./spec.yaml").info.title}
   ```
   - ❌ Requires custom template parser
   - ❌ String interpolation complexity
   - ✅ Very familiar to Terraform users

2. **Special Key Approach** (like Helm)
   ```yaml
   name:
     $file: ./spec.yaml
     $extract: info.title
   ```
   - ✅ Pure YAML, simple parsing
   - ❌ More verbose
   - ❌ Can conflict with actual data

3. **URI-style References**
   ```yaml
   name: "file://./spec.yaml#/info/title"
   ```
   - ❌ Requires string parsing
   - ❌ Less readable
   - ✅ Familiar URI pattern

### Future Extensions
The tag system enables future additions:
- `!env` - Environment variables
- `!vault` - Secret management integration  
- `!http` - Remote content loading
- `!template` - Template processing
- `!merge` - Merge multiple sources

---

## ADR-004-003: External ID References for Control Planes and Services

### Context
API implementations need to reference Gateway services and control planes. Since core entities (services, routes, etc.) are managed by decK and we want to avoid overlap, we need to support external ID references without managing these entities.

### Decision
Support external ID references for control planes and services:
```yaml
api_implementations:
  - ref: users-impl
    api: users-api
    service:
      control_plane_id: "prod-cp"  # External ID reference
      id: "users-service-v2"       # External service ID
```

### Consequences
- **Positive:**
  - No overlap with decK's responsibilities
  - Clear separation of concerns
  - Flexibility for users
  - Simpler initial implementation

- **Negative:**
  - No validation of external IDs
  - Manual coordination required
  - Potential for dangling references

### Future Consideration
If we later decide to manage control planes declaratively, the syntax can be extended to support both external IDs and references:
```yaml
# Future: reference OR external ID
service:
  control_plane: my-cp  # Reference to managed control plane
  # OR
  control_plane_id: "external-cp-id"  # External ID
```

---

## ADR-004-004: Dependency Resolution Strategy

### Context
With multiple resource types and cross-resource references, we need robust dependency resolution to ensure correct creation and deletion order.

### Decision
Extend the existing topological sort-based dependency resolver:
1. Explicit dependencies via parent-child relationships
2. Implicit dependencies via references
3. Cross-resource dependencies (e.g., API publication → portal)
4. Reverse order for deletions in sync mode

### Implementation
```go
// Dependency detection
- Parent-child: API → API Version
- References: API Publication → Portal
- External: No dependencies for external IDs
```

### Consequences
- **Positive:**
  - Correct execution order guaranteed
  - Cycle detection prevents invalid configs
  - Extensible for new dependency types
  - Reuses proven algorithm

- **Negative:**
  - Complexity grows with resource types
  - Must maintain dependency rules
  - Performance overhead for large graphs

---

## ADR-004-005: Nested vs Separate File Configuration

### Context
Users need flexibility in how they organize their configuration files. Some prefer everything in one file, others prefer separation by resource type.

### Decision
Support both nested and separate file configurations:

**Nested (everything together):**
```yaml
apis:
  - ref: users-api
    name: "Users API"
    versions:
      - ref: v1
        version: "1.0.0"
    publications:
      - ref: portal-pub
        portal: dev-portal
```

**Separate files:**
```yaml
# apis.yaml
apis:
  - ref: users-api
    name: "Users API"

# api-versions.yaml
api_versions:
  - ref: v1
    api: users-api
    version: "1.0.0"
```

### Implementation
- Extract nested resources during loading
- Add parent reference to extracted resources
- Validate parent references exist

### Consequences
- **Positive:**
  - Maximum flexibility for users
  - Natural organization options
  - Easy migration between styles
  - Supports team preferences

- **Negative:**
  - More complex loading logic
  - Duplicate parent references
  - Potential for inconsistency

---

## ADR-004-006: File Path Security

### Context
Loading external files introduces security risks like path traversal attacks or accessing sensitive system files.

### Decision
Implement strict file access controls:
1. Relative paths only (no absolute paths by default)
2. No parent directory traversal (`../` not allowed)
3. Configurable base directory
4. File size limits
5. Timeout for file operations

### Implementation
```go
func validatePath(path string) error {
    if filepath.IsAbs(path) {
        return errors.New("absolute paths not allowed")
    }
    if strings.Contains(path, "..") {
        return errors.New("parent directory traversal not allowed")
    }
    return nil
}
```

### Consequences
- **Positive:**
  - Prevents directory traversal attacks
  - Limits access to project files
  - Predictable file locations
  - Safe by default

- **Negative:**
  - May limit some legitimate use cases
  - Users might need workarounds
  - Additional configuration needed

### Future Consideration
Could add opt-in flags for advanced users:
- `--allow-absolute-paths`
- `--file-base-dir`

---

## ADR-004-007: Value Extraction Path Notation

### Context
When extracting values from loaded files, we need a notation for specifying paths within structured data (JSON/YAML).

### Decision
Use dot notation for path extraction, similar to JSONPath but simpler:
```yaml
# Object field access
extract: info.title

# Nested fields
extract: info.contact.email

# Array access (future)
extract: servers[0].url

# Map key with dots (future)
extract: ["x-api-properties"]["rate-limit"]
```

### Implementation
Start with simple dot notation, extend as needed:
```go
func extractValue(data interface{}, path string) (interface{}, error) {
    parts := strings.Split(path, ".")
    current := data
    
    for _, part := range parts {
        // Navigate through maps/structs
    }
    
    return current, nil
}
```

### Consequences
- **Positive:**
  - Simple and intuitive
  - Covers most use cases
  - Familiar to developers
  - Extensible syntax

- **Negative:**
  - Limited compared to full JSONPath
  - Array access needs special handling
  - Keys with dots need escaping

---

## ADR-004-008: Error Handling for External Content

### Context
Loading external content can fail in various ways. We need clear, actionable error messages.

### Decision
Implement comprehensive error handling with context:
1. File not found → Show attempted path and working directory
2. Parse errors → Show line/column if available
3. Invalid extraction path → Show available paths
4. Type mismatches → Show expected vs actual type

Example errors:
```
Error: Cannot load file "./specs/api.yaml"
  File not found: /project/specs/api.yaml
  Working directory: /project
  
Error: Cannot extract value "info.titel" from "./specs/api.yaml"
  Path not found: info.titel
  Did you mean: info.title
  Available paths: info.title, info.version, info.description
```

### Consequences
- **Positive:**
  - Users can quickly fix issues
  - Reduces support burden
  - Better developer experience
  - Self-documenting errors

- **Negative:**
  - More complex error handling
  - Performance cost for suggestions
  - Larger error messages

---

## ADR-004-009: Caching Strategy for External Files

### Context
The same external file might be referenced multiple times in a configuration. Loading and parsing repeatedly is inefficient.

### Decision
Implement a simple cache for the duration of a single command execution:
1. Cache by absolute file path
2. Cache parsed content, not raw bytes
3. Clear cache between commands
4. No persistent cache

### Implementation
```go
type FileCache struct {
    entries map[string]interface{}
    mu      sync.RWMutex
}
```

### Consequences
- **Positive:**
  - Better performance
  - Consistent data within execution
  - Simple implementation
  - No cache invalidation issues

- **Negative:**
  - Memory usage for large files
  - No benefit across commands
  - Reloads on every command

---

## ADR-004-010: Type-Specific Adapter Functions

### Context
Each resource type requires specific adapter functions for conversion between internal and SDK types. This creates maintenance burden but provides type safety.

### Decision
Continue with type-specific adapters for now, but structure them for future generalization:
1. Consistent naming patterns
2. Similar function signatures
3. Document patterns for future refactoring
4. Consider code generation in Stage 6

### Consequences
- **Positive:**
  - Type safety maintained
  - Clear, explicit conversions
  - Easy to debug
  - Compile-time checking

- **Negative:**
  - Boilerplate code
  - Maintenance burden
  - Duplication of patterns

### Future Consideration
Stage 6's code review task will evaluate generic solutions or code generation to reduce this boilerplate.