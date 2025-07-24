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

## Summary of Decisions

1. **Implementation**: Use existing label system
2. **Default**: Namespace defaults to "default" when not specified
3. **Configuration**: `_defaults.kongctl.namespace` for file defaults
4. **Placement**: In `kongctl` section of resources
5. **Scope**: Parent resources only (Konnect limitation)
6. **Operations**: Process all namespaces in config
7. **Mutability**: Namespaces cannot be changed
8. **Terminology**: "namespace" over other terms

These decisions balance safety and explicit configuration with ease of use, supporting both simple single-team use cases and complex multi-team scenarios.