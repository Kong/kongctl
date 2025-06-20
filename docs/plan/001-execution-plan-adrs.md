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
    DeclarativeName string `yaml:"name"`      // Add our fields
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

## ADR-003: Declarative Name vs API Name Field Handling

### Status
Accepted

### Context
The Kong SDK's Portal type already contains a `Name` field that represents the portal's name in the API. We need a way to identify resources in declarative configuration for cross-references. This creates a naming conflict.

### Decision
We will use separate fields:
- `name` (yaml tag) / `DeclarativeName` (Go field): Used for references between resources
- SDK's `Name` field: The actual portal name sent to the API

If the SDK's Name field is not explicitly set, we'll default it to the declarative name.

### Rationale
- **Clear separation**: Declarative identity vs API representation
- **Flexibility**: Users can have different reference names vs display names
- **Future-proof**: Supports namespaced references later (e.g., `team/resource-name`)
- **User-friendly**: Simple `name` in YAML for references

### Consequences
- Users must understand the distinction (mitigated by good defaults)
- Documentation must clearly explain both fields
- Validation must ensure both fields are properly set

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

## ADR-006: Command Structure - New Verbs vs Sub-commands

### Status
Accepted

### Context
We need to add new commands for declarative configuration. Options:
1. Add as top-level verbs (plan, apply, sync, diff, export)
2. Add under a namespace (e.g., `kongctl declarative plan`)
3. Add under existing verbs (e.g., `kongctl get plan`)

### Decision
Add as top-level verbs following the existing pattern in kongctl.

### Rationale
- **Consistency**: Matches existing verb-noun pattern
- **Simplicity**: Direct commands are easier to use
- **Precedent**: Tools like Terraform use top-level commands
- **Design alignment**: Planning documents show this approach

### Consequences
- More top-level commands (acceptable for core functionality)
- Clear, simple command structure
- May need to carefully manage verb namespace in future

---

## ADR-007: YAML File Organization

### Status
Accepted

### Context
Users need flexibility in organizing their declarative configuration files. We need to decide how to handle file discovery and organization.

### Decision
Support flexible file organization:
- Process all `.yaml` and `.yml` files in specified directory
- Allow arbitrary directory structure
- Resources can be split across multiple files
- No enforced naming convention

### Rationale
- **User flexibility**: Teams can organize files as they prefer
- **Simple implementation**: Just find and parse all YAML files
- **Scalability**: Easy to split large configurations
- **Tool-agnostic**: Works with any file organization approach

### Consequences
- Must handle resource merging across files
- Name uniqueness validation across all files
- Clear error messages must indicate which file has issues