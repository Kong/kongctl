# Stage 1 Architecture Decision Records (ADRs)

These ADRs document architectural decisions made specifically for Stage 1 implementation of the declarative configuration feature. Each ADR is numbered as ADR-001-XXX to indicate it applies to Stage 1.

## ADR-001-001: Type-Specific ResourceSet vs Generic Collection

### Context
We need to decide how to structure the ResourceSet that contains all declarative resources loaded from YAML files. Two main approaches were considered:

1. **Type-specific fields**: Each resource type has its own field in ResourceSet
2. **Generic collection**: Resources stored in a generic map or slice with runtime type information

### Decision
We will use a type-specific ResourceSet with explicit fields for each resource type:

```go
type ResourceSet struct {
    Portals                   []PortalResource                    `yaml:"portals,omitempty"`
    ApplicationAuthStrategies []ApplicationAuthStrategyResource   `yaml:"application_auth_strategies,omitempty"`
    Teams                     []TeamResource                      `yaml:"teams,omitempty"`
    // Additional fields added as we support new resource types
}
```

### Rationale

**Benefits of type-specific approach:**
- **Type safety**: Compile-time guarantees about resource types prevent runtime errors
- **IDE support**: Autocomplete, go-to-definition, and refactoring tools work effectively
- **Clear API**: Users and developers immediately see what resources are supported
- **Simple validation**: Each field has a specific type to validate against
- **Easy serialization**: YAML/JSON marshaling is straightforward
- **Better user experience**: The YAML format matches user expectations from the design brief

**Why not generic?**
- Konnect has a finite, well-known set of resources (not an unbounded plugin system)
- The planning documents show clear preference for type-specific top-level keys
- Runtime type assertions would complicate the code and reduce safety
- Poor IDE support would harm developer productivity

### Consequences
- Adding new resource types requires modifying ResourceSet (acceptable given finite resource types)
- Clear, type-safe API for consumers of the configuration
- Straightforward implementation and testing

---

## ADR-001-002: SDK Type Embedding vs Duplication

### Context
We need to decide how to handle the relationship between our declarative resource types and the Kong SDK types. Options considered:

1. **Embed SDK types**: Use struct embedding to include SDK types in our wrappers
2. **Duplicate fields**: Copy all fields from SDK types into our own structs
3. **Interface abstraction**: Define interfaces and convert between types

### Decision
We will embed SDK types directly in our resource wrappers:

```go
type PortalResource struct {
    components.CreatePortal `yaml:",inline"`  // Embed SDK type
    Ref string `yaml:"ref"`                   // Reference identifier
    Kongctl *KongctlMeta `yaml:"kongctl,omitempty"`
}
```

### Rationale
- **No duplication**: Avoid maintaining copies of SDK field definitions
- **Automatic updates**: When SDK updates, we get new fields automatically
- **Type safety**: Direct use of SDK types ensures compatibility
- **Simple implementation**: No complex conversion logic needed
- **YAML inline**: The `yaml:",inline"` tag provides clean YAML structure

### Consequences
- Tight coupling to SDK types (acceptable as we're building for Kong specifically)
- Must handle any SDK type changes in future versions
- Clear separation between SDK fields and our additions

---

## ADR-001-003: Resource Reference Identifiers

### Context
Most Konnect resources (portals, teams, auth strategies, etc.) have a `Name` field in the SDK that represents the resource's name in the API. However, Konnect resource names can contain spaces and special characters, making them problematic for cross-references in configuration files.

Additionally, Konnect resources have UUID-based `ID` fields that are API-assigned. We need a way to identify resources in declarative configuration for cross-references that:
1. Doesn't conflict with SDK's `Name` field (can have spaces)
2. Doesn't conflict with SDK's `ID` field (UUID assigned by Konnect)
3. Is computer-friendly for use in references

### Decision
We will use a separate `ref` field for resource references:

```yaml
portals:
  - ref: dev-portal                     # Computer-friendly reference identifier
    name: "Developer Portal (Production)"  # Human-friendly Konnect name
    # ID field will be populated by Konnect (UUID)
    
application_auth_strategies:
  - ref: oauth-strategy
    name: "My OAuth 2.0 Strategy"
    
# Cross-references use the ref field
portals:
  - ref: prod-portal
    name: "Production Portal"
    default_application_auth_strategy: oauth-strategy  # References the ref field
```

This creates three distinct identifiers:
- **SDK's ID field**: UUID assigned by Konnect (never in declarative config)
- **SDK's Name field**: Human-friendly display name (can have spaces)
- **Our ref field**: Computer-friendly reference identifier (no spaces, used for cross-references)

### Rationale
- **Unambiguous**: Clear distinction from both SDK `ID` and `Name` fields
- **User-friendly**: Clean references without spaces or special characters
- **Future-proof**: Works well with type prefixes, namespaces, etc.
- **Flexible**: Users can use different values for display vs references
- **Familiar**: Similar to Kubernetes and other declarative tools

### Consequences
- Users must understand three different identifiers (mitigated by clear documentation)
- Ref field must be unique within resource type
- Documentation must clearly explain the distinction
- Validation must ensure ref field is properly set
- Cross-resource references use ref values (see ADR-001-008 for reference pattern details)

---

## ADR-001-004: Package Structure - Avoiding "Config" Naming

### Context
The term "config" is already used extensively in kongctl for CLI configuration (profiles, settings, etc.). Using "config" for declarative resource management would create confusion.

### Decision
Use `declarative` as the top-level package with sub-packages:
- `internal/declarative/resources/` - Resource type definitions
- `internal/declarative/loader/` - YAML loading and parsing
- Future: `internal/declarative/planner/`, `internal/declarative/executor/`

### Rationale
- **Clear distinction**: "Declarative" clearly indicates the feature domain
- **Avoids confusion**: No overlap with existing configuration management
- **Descriptive**: The name explains the purpose
- **Scalable**: Easy to add sub-packages for different concerns

### Consequences
- Slightly longer import paths
- Clear separation of concerns
- Easy to understand codebase organization

---

## ADR-001-005: Test Strategy - What to Test

### Context
We need to decide what aspects of the code require testing and what can be safely skipped to avoid testing third-party functionality.

### Decision
**Test:**
- Business logic (validation, merging, name resolution)
- Integration points (command execution, file loading)
- Error handling and edge cases
- Complex transformations or algorithms

**Don't test:**
- SDK functionality (already tested by Kong)
- YAML marshaling/unmarshaling (standard library)
- Simple getters/setters with no logic
- Third-party library functionality

### Rationale
- **Focus on value**: Test code we write, not code we import
- **Maintainable**: Fewer brittle tests of external behavior
- **Confidence**: Good coverage of our actual logic
- **Efficiency**: Faster test runs, easier maintenance

### Consequences
- Clear testing guidelines for contributors
- Focused test suites that provide real value
- Potential gaps if third-party libraries have bugs (acceptable risk)

---

## ADR-001-006: Command Structure - Konnect-First Approach

### Context
We need to add new commands for declarative configuration. Options:
1. Add as top-level verbs (plan, apply, sync, diff, export)
2. Add under a namespace (e.g., `kongctl declarative plan`)
3. Add under existing verbs (e.g., `kongctl get plan`)

### Decision
Add as top-level verbs following the existing pattern in kongctl, with these commands serving as aliases for Konnect operations.

### Rationale
- **Konnect-first approach**: The broader project follows a "Konnect first" approach where commands default to Konnect operations
- **Future extensibility**: Commands like `kongctl apply` are effectively aliases for `kongctl konnect apply`, allowing future expansion to `kongctl gateway apply` for on-prem configurations
- **Consistency**: Matches existing verb-noun pattern in kongctl
- **Simplicity**: Direct commands are easier to use for the primary use case (Konnect)
- **Precedent**: Tools like Terraform use top-level commands
- **Design alignment**: Planning documents show this approach

### Consequences
- More top-level commands (acceptable for core functionality)
- Clear, simple command structure for Konnect operations
- Future flexibility to add product-specific namespaces (gateway, mesh) when needed
- May need to carefully manage verb namespace as we add more products

---

## ADR-001-007: YAML File Organization

### Context
Users need flexibility in organizing their declarative configuration files. We need to decide how to handle file discovery and organization.

### Decision
Support flexible file organization:
- Process all `.yaml` and `.yml` files in specified directory
- Support nested folder structures with recursive traversal
- Allow arbitrary directory structure and naming
- Resources can be split across multiple files
- No enforced naming convention

### Rationale
- **User flexibility**: Teams can organize files as they prefer (by environment, team, resource type, etc.)
- **Simple implementation**: Use `filepath.Walk` to find and parse all YAML files
- **Scalability**: Easy to split large configurations across nested directories
- **Tool-agnostic**: Works with any file organization approach
- **Real-world usage**: Teams often need hierarchical organization for complex deployments

### Consequences
- Must handle resource merging across files and directories
- Ref uniqueness validation across all files in the directory tree
- Clear error messages must indicate which file has issues
- Recursive directory traversal may be slower for very large directory trees

---

## ADR-001-008: Resource Reference Pattern and Validation

### Context
The Konnect API uses UUID-based references between resources, but these UUIDs are not user-friendly for declarative configuration. After analyzing the SDK, we discovered several patterns:

1. **Inconsistent field naming**: `ControlPlaneID`, `control_plane_id`, `DefaultApplicationAuthStrategyID`, `auth_strategy_ids`
2. **Nested references**: API Implementation requires both `control_plane_id` and `id` (service within control plane)
3. **Array references**: API Publications can reference multiple auth strategies via `auth_strategy_ids`
4. **Context-dependent fields**: Some fields like `id` depend on their context to determine the expected type

Examples from the SDK:
```go
// API Implementation Service Input
type APIImplementationServiceInput struct {
    ControlPlaneID string  // References control plane
    ID             string  // References service within that control plane
}

// API Publication
type APIPublicationListItem struct {
    APIID           string    // References API
    PortalID        string    // References portal
    AuthStrategyIds []string  // References multiple auth strategies
}
```

### Decision
Use **Smart Field Analysis with Pattern-Based Validation** (Option B):

1. **Simple ref-based syntax**: Users provide `ref` values in all reference fields
2. **Pattern-based field mapping**: Build mappings to determine expected types from field names
3. **Validation at planning time**: Resolve and validate all references during plan generation
4. **Clear error messages**: Guide users when references are invalid

### Implementation
```go
// Field pattern mapping for reference validation
var referenceFieldMappings = map[string]string{
    "*_control_plane_id":                   "control_plane",
    "*_portal_id":                          "portal", 
    "*_api_id":                            "api",
    "auth_strategy_ids":                   "auth_strategy",
    "default_application_auth_strategy_id": "auth_strategy",
    // Context-dependent cases handled separately
}

// Example declarative configuration
api_implementations:
  - ref: my-api-impl
    service:
      control_plane_id: my-cp        # Expects control_plane ref
      id: my-service                 # Expects service ref (context: within control plane)

api_publications:
  - ref: my-publication
    api_id: my-api                   # Expects api ref
    portal_id: my-portal             # Expects portal ref
    auth_strategy_ids: 
      - oauth-strategy               # Expects auth_strategy refs
      - key-auth-strategy
```

### Rationale
- **Clean syntax**: No type prefixes needed (avoiding `application_auth_strategy.oauth-strategy`)
- **Type safety**: Validation ensures references point to correct resource types
- **Handles complexity**: Works with arrays, nested references, and inconsistent naming
- **Future-proof**: Can add type prefixes later if needed
- **Resolution timing**: All references resolved to UUIDs at planning time for safety

### UX Concerns and Mitigations
**Problem**: Field names contain `id` but expect `ref` values, potentially confusing users

**Mitigations**:
1. **Clear documentation** with extensive examples showing `ref` usage
2. **Helpful error messages**:
   ```
   Error in api_publication "my-publication":
     Field "portal_id" expects a portal reference (ref value), not a UUID
     Found: "7710d5c4-d902-410b-992f-18b814155b53" 
     Did you mean: "my-portal"?
     
   Available portal refs: my-portal, dev-portal, staging-portal
   ```
3. **UUID detection**: Warn when UUID-like values are used in reference fields
4. **Consistent terminology**: Always refer to "`ref` values" in documentation

### Consequences
- **Simple user experience**: Clean YAML syntax without type prefixes
- **Implementation complexity**: Need pattern matching and context-aware validation
- **Documentation burden**: Must clearly explain ref vs UUID distinction
- **Error handling**: Need excellent error messages to guide users
- **Future flexibility**: Can add escape hatches (type prefixes) if simple refs become insufficient