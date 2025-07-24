# Stage 6: Namespace-Based Resource Management - Architecture Decision Records

This document captures key architectural decisions made during the planning and implementation of namespace-based resource management.

## ADR-006-001: Namespace as a Label vs. Separate System

### Status
Accepted

### Context
We need a mechanism to isolate resources managed by different teams within a shared Konnect organization. Options considered:
1. Build a separate namespace system with its own tracking
2. Leverage the existing label system
3. Use a combination of both

### Decision
We will implement namespaces using the existing label system:
- Namespace stored as `KONGCTL-namespace` label
- Follows the same pattern as `KONGCTL-protected`
- Reuses existing label infrastructure

### Consequences
**Positive:**
- Consistent with existing patterns
- Minimal new code required
- Label filtering already implemented
- Clear audit trail in Konnect

**Negative:**
- Subject to Konnect label limitations
- Cannot enforce server-side
- No native namespace concept in Konnect

---

## ADR-006-002: Default Namespace Value

### Status
Accepted

### Context
Should namespace be optional with a default value, or required on all resources?

### Decision
Namespace will default to "default" when not specified:
- If no `kongctl.namespace` and no `_defaults.kongctl.namespace`, use "default"
- Allows minimal configuration for simple use cases
- Users can be explicit when needed for multi-team scenarios

### Consequences
**Positive:**
- Easier onboarding and simple use cases
- Minimal configuration works out of the box
- No breaking changes needed
- Gradual adoption path

**Negative:**
- Risk of accidental resource mixing if teams forget to set namespace
- "default" namespace may accumulate unintended resources
- Need clear documentation about namespace importance

---

## ADR-006-003: File-Level Defaults Structure

### Status
Accepted

### Context
How should file-level defaults be expressed in configuration files?

### Decision
Use `_defaults` section with nested structure:
```yaml
_defaults:
  kongctl:
    namespace: team-alpha
```

### Consequences
**Positive:**
- Clear separation from Konnect resources
- Extensible for future defaults
- Consistent underscore prefix convention
- Allows nested configuration

**Negative:**
- Additional parsing complexity
- New top-level section to document
- Potential for user confusion

---

## ADR-006-004: Namespace in kongctl Section

### Status
Accepted

### Context
Where should the namespace field be placed within resource definitions?

### Decision
Add namespace to the existing `kongctl` section alongside `protected`:
```yaml
kongctl:
  protected: false
  namespace: team-alpha
```

### Consequences
**Positive:**
- Consistent with `protected` field
- Clear grouping of kongctl metadata
- Natural override location
- Type-safe implementation

**Negative:**
- Only on parent resources
- Must handle nil kongctl sections
- Requires planner updates

---

## ADR-006-005: Parent-Only Namespace Labels

### Status
Accepted (Forced by Konnect limitation)

### Context
Konnect API only supports labels on top-level resources (APIs, Portals, Auth Strategies), not on child resources.

### Decision
- Only parent resources have namespace in configuration
- Child resources inherit parent's namespace implicitly
- State client filters parents, includes their children

### Consequences
**Positive:**
- Works within Konnect constraints
- Simpler configuration
- Clear parent-child relationship

**Negative:**
- Cannot namespace individual child resources
- Complexity in state client filtering
- Must track parent-child relationships

---

## ADR-006-006: Multi-Namespace Command Operations

### Status
Accepted

### Context
How should commands handle multiple namespaces in configuration files?

### Decision
Commands will process all namespaces found in loaded configurations:
- No `--namespace` CLI flag
- Each namespace planned independently
- Clear output showing namespace operations
- Sync only affects declared namespaces

### Consequences
**Positive:**
- Configuration-driven approach
- Single command for all namespaces
- Clear operational boundaries
- Safer sync operations

**Negative:**
- Cannot filter to single namespace via CLI
- Potentially longer execution times
- More complex output formatting

---

## ADR-006-007: Namespace Change Prevention

### Status
Accepted

### Context
Should resources be allowed to change namespaces?

### Decision
Namespace changes will be prevented:
- Validation error if existing resource has different namespace
- Must delete and recreate to change namespace
- `--force-namespace-change` considered for future

### Consequences
**Positive:**
- Prevents accidental namespace moves
- Clear ownership boundaries
- Safer operations
- Audit trail preservation

**Negative:**
- Less flexibility
- Manual migration required
- Potential user frustration

---

## ADR-006-008: Namespace Terminology

### Status
Accepted

### Context
Multiple terms were considered: scope, namespace, tenant, owner, workspace, domain, context.

### Decision
Use "namespace" as the term throughout:
- Familiar from Kubernetes
- Clear isolation implications
- Industry-standard terminology
- Aligns with segregation concept

### Consequences
**Positive:**
- Familiar to users
- Clear purpose
- Good documentation exists
- Natural pluralization

**Negative:**
- Might imply stronger isolation than provided
- Kubernetes associations may set expectations
- "Namespace" is a longer word than some alternatives

---

---

## ADR-006-009: Remove KONGCTL-managed and KONGCTL-last-updated Labels

### Status
Accepted

### Context
Konnect has a strict limit of 5 labels per resource. Currently kongctl adds 3 
labels:
- `KONGCTL-managed: true` - Identifies resources managed by kongctl
- `KONGCTL-last-updated: <timestamp>` - Tracks last update time
- `KONGCTL-protected: true/false` - Prevents deletion of critical resources

With the addition of `KONGCTL-namespace`, we would use 4 of 5 allowed labels, 
leaving only 1 for user labels. Additionally:
- Konnect resources already have native timestamp fields
- The namespace label can serve as the management indicator

### Decision
Remove `KONGCTL-managed` and `KONGCTL-last-updated` labels:
- Any resource with `KONGCTL-namespace` label is considered managed
- Use Konnect's native timestamp fields instead of custom label
- Keep only `KONGCTL-namespace` and `KONGCTL-protected` labels
- This leaves 3 label slots for users

### Consequences
**Positive:**
- Frees up 2 label slots for user use (60% of limit)
- Simpler label management code
- Namespace serves dual purpose (ownership + management)
- No redundant timestamp tracking
- Cleaner resource representation

**Negative:**
- Breaking change for existing deployments
- Need migration path for existing resources
- Loss of custom timestamp format
- Temporary backwards compatibility complexity

**Migration Strategy:**
- During transition, check for either old `KONGCTL-managed` or new namespace
- First sync will update resources to new label scheme
- Document migration clearly in release notes

---

---

## ADR-006-010: Remove KongctlMeta from Child Resources

### Status
Accepted

### Context
Child resources (API versions, publications, implementations, documents, portal 
pages, customizations, domains, snippets) currently have a `Kongctl *KongctlMeta` 
field in their structs. However, Konnect API doesn't support labels on child 
resources, making this field misleading and useless.

### Decision
Remove the `Kongctl *KongctlMeta` field from all child resource types:
- API child resources: versions, publications, implementations, documents
- Portal child resources: pages, customizations, custom domains, snippets
- Add validation to reject kongctl sections in child resource YAML
- Child resources inherit namespace behavior from their parent

### Consequences
**Positive:**
- Configuration matches reality (no false promises)
- Simpler code and data model
- Clear error messages prevent user confusion
- Less memory usage and cleaner structs

**Negative:**
- Breaking change for any existing configs with child resource kongctl sections
- Need to update examples and documentation
- Slightly less flexibility if Konnect adds child labels in future

---

## ADR-006-011: Only Add Protected Label When True

### Status
Accepted

### Context
Currently we add `KONGCTL-protected: false` to all resources by default. With 
Konnect's 5-label limit, every label counts. We can infer that a resource is 
not protected if the label is absent.

### Decision
Only add the `KONGCTL-protected: true` label when a resource is explicitly 
protected:
- If `kongctl.protected: true` → Add label
- If `kongctl.protected: false` or not specified → No label
- Absence of label means not protected

### Consequences
**Positive:**
- Saves a label slot in the common case (most resources aren't protected)
- Default case uses only 1 label (namespace), leaving 4 for users (80%)
- Protected resources use 2 labels, leaving 3 for users (60%)
- Cleaner resource representation

**Negative:**
- Slight code change to check for label presence vs checking value
- Cannot distinguish between "explicitly not protected" vs "default not protected"
- Need to update existing protected checking logic

---

## ADR-006-012: Use Pointer Types for KongctlMeta Fields

### Status
Accepted

### Context
The initial implementation mixed pointer and non-pointer types:
- `KongctlDefaults` used `string` for Namespace and `*bool` for Protected
- `KongctlMeta` used `string` for Namespace and `bool` for Protected

This inconsistency caused issues:
- Could not distinguish between "not set" and "explicitly set to false" for Protected
- Explicit `protected: false` would be overridden by `protected: true` defaults
- Different behavior patterns for the two fields

### Decision
Use pointer types consistently in both structs:
```go
type KongctlMetaDefaults struct {
    Namespace *string `yaml:"namespace,omitempty" json:"namespace,omitempty"`
    Protected *bool   `yaml:"protected,omitempty" json:"protected,omitempty"`
}

type KongctlMeta struct {
    Protected *bool   `yaml:"protected,omitempty" json:"protected,omitempty"`
    Namespace *string `yaml:"namespace,omitempty" json:"namespace,omitempty"`
}
```

### Consequences
**Positive:**
- Consistent behavior between Namespace and Protected fields
- Can distinguish: nil (not set), empty/false (explicit), value (explicit)
- Proper override semantics - explicit values always win
- More intuitive API - both fields work the same way
- Fixes the protected override bug

**Negative:**
- Breaking change for existing code
- Must handle nil checks throughout codebase
- Slightly more complex code with pointer dereferencing
- Potential nil pointer panics if not careful

---

## ADR-006-013: Rename KongctlDefaults to KongctlMetaDefaults

### Status
Accepted

### Context
The struct name `KongctlDefaults` was inconsistent with `KongctlMeta`. Since 
these defaults are specifically for the metadata fields, the naming should 
reflect this relationship.

### Decision
Rename the struct from `KongctlDefaults` to `KongctlMetaDefaults` to match 
the `KongctlMeta` struct it provides defaults for.

### Consequences
**Positive:**
- Clear naming relationship: `KongctlMeta` ← `KongctlMetaDefaults`
- Better code clarity and intent
- Consistent naming patterns
- Self-documenting code

**Negative:**
- Breaking change for any code referencing the old name
- Need to update all references

---

## ADR-006-014: Reject Empty Namespace Values

### Status
Accepted

### Context
With pointer types, we can now have:
- `nil` - namespace not specified
- `""` - empty string namespace
- `"value"` - actual namespace value

Empty namespaces could cause issues with resource management and filtering.

### Decision
Reject empty namespace values at all levels:
- Empty namespace in `_defaults.kongctl.namespace` → Error
- Empty namespace in resource `kongctl.namespace` → Error  
- Every resource must have a non-empty namespace (default or explicit)

Implementation validates during loading and returns clear error messages.

### Consequences
**Positive:**
- Every resource guaranteed to have meaningful namespace
- No ambiguity in resource ownership
- Clear error messages guide users
- Prevents accidental misconfigurations
- Namespace labels always have meaningful values

**Negative:**
- Cannot use empty string as a namespace (unlikely use case)
- Additional validation code required
- Must handle validation errors in loader

---

## Summary of Decisions

1. **Implementation**: Use existing label system
2. **Default**: Namespace defaults to "default" when not specified
3. **Configuration**: `_defaults.kongctl.namespace` for file defaults
4. **Placement**: In `kongctl` section of resources
5. **Scope**: Parent resources only (Konnect limitation)
6. **Operations**: Process all namespaces in config
7. **Mutability**: Namespaces cannot be changed
8. **Terminology**: "namespace" over other terms
9. **Label Optimization**: Remove managed/last-updated labels, use namespace as indicator
10. **Child Resources**: Remove KongctlMeta from child resource types
11. **Protected Label**: Only add when resource is actually protected
12. **Pointer Types**: Use pointers consistently for both Namespace and Protected fields
13. **Struct Naming**: Rename KongctlDefaults to KongctlMetaDefaults for consistency
14. **Empty Values**: Reject empty namespace values with clear error messages

These decisions balance safety and explicit configuration with ease of use, while maximizing available labels for users within Konnect's strict 5-label limit. The pointer types enable proper nil detection and override semantics, while the validation ensures every resource has a meaningful namespace.