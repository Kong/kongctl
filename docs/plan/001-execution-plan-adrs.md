# Stage 1 Architecture Decision Records (ADRs)

## ADR-001: Type-Specific ResourceSet vs Generic Collection

### Status
Accepted

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

## ADR-002: SDK Type Embedding vs Duplication

### Status
Accepted

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

## ADR-003: Resource Reference Identifiers

### Status
Accepted

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

---

## ADR-004: Package Structure - Avoiding "Config" Naming

### Status
Accepted

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

## ADR-005: Test Strategy - What to Test

### Status
Accepted

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

## ADR-006: Command Structure - Konnect-First Approach

### Status
Accepted

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

## ADR-007: YAML File Organization

### Status
Accepted

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