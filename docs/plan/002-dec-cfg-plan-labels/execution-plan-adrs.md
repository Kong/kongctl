# Stage 2: Architecture Decision Records (ADRs)

## ADR-002-001: Label Normalization Strategy

### Status
Proposed

### Context
The SDK has inconsistent label representations:
- `CreatePortal` uses `map[string]*string` (pointer values)
- `PortalResponse` uses `map[string]string` (non-pointer values)

We need a consistent approach for comparing and managing labels.

### Decision
Normalize all labels to `map[string]string` (non-pointer values) for internal processing:
- Convert pointer values to non-pointer during read operations
- Handle nil pointers as empty strings
- Use non-pointer values for all comparisons and hash calculations

### Consequences
- **Positive**: Consistent label handling throughout the codebase
- **Positive**: Simplified comparison logic
- **Negative**: Additional conversion step when interacting with SDK
- **Negative**: Must remember to convert back to pointers for SDK calls

### Implementation
```go
func normalizeLabels(labels map[string]*string) map[string]string {
    normalized := make(map[string]string)
    for k, v := range labels {
        if v != nil {
            normalized[k] = *v
        }
    }
    return normalized
}
```

---

## ADR-002-002: Configuration Hash Algorithm

### Status
Proposed

### Context
We need to detect configuration drift efficiently without comparing every field. A hash
provides a quick way to determine if configurations differ.

### Decision
Use SHA256 hashing with canonical JSON representation:
- Sort all map keys alphabetically
- Exclude system-generated fields (ID, timestamps)
- Exclude KONGCTL-prefixed labels
- Include all user-configurable fields
- Store hash as base64-encoded string

### Consequences
- **Positive**: Fast drift detection with single string comparison
- **Positive**: Cryptographically secure against collisions
- **Positive**: Deterministic output for same configuration
- **Negative**: Requires careful field selection
- **Negative**: Hash changes even for semantically equivalent configs

### Implementation Approach
```go
type HashablePortal struct {
    Name                            string
    DisplayName                     *string
    Description                     *string
    AuthenticationEnabled           *bool
    RbacEnabled                    *bool
    DefaultAPIVisibility           *string
    DefaultPageVisibility          *string
    DefaultApplicationAuthStrategyID *string
    AutoApproveDevelopers          *bool
    AutoApproveApplications        *bool
    UserLabels                     map[string]string // Non-KONGCTL labels only
}
```

---

## ADR-002-003: Reference Resolution Timing

### Status
Proposed

### Context
Declarative configs use references (e.g., `auth_strategy: "oauth-strategy"`). These must
be resolved to Konnect IDs at some point.

### Decision
Resolve references during plan generation (not execution):
- Validate all references exist at plan time
- Store ref → ID mappings in the plan
- Detect if references change between plan and execution
- Fail fast if references cannot be resolved

### Consequences
- **Positive**: Early validation of configuration correctness
- **Positive**: Faster execution as lookups are pre-resolved
- **Positive**: Can detect reference changes as another drift indicator
- **Negative**: Plans become invalid if referenced resources are deleted
- **Negative**: Plans are environment-specific (contain IDs)

### Alternative Considered
Late resolution during execution was considered but rejected because:
- Would delay error detection until execution
- Makes plans less deterministic
- Requires lookups during execution, slowing it down

---

## ADR-002-004: Plan Structure and Versioning

### Status
Proposed

### Context
Plans need a well-defined structure that can evolve over time while maintaining 
compatibility.

### Decision
Use versioned JSON with semantic versioning:
- Version "1.0" for initial implementation
- Include metadata section with version, timestamp, generator
- Separate sections for reference mappings and changes
- Summary section for quick overview

### Consequences
- **Positive**: Clear upgrade path for future enhancements
- **Positive**: Tools can detect and handle version differences
- **Positive**: Human-readable format for debugging
- **Negative**: Version management complexity
- **Negative**: Need migration strategy for version changes

### Plan Schema
```yaml
version: 1.0
metadata:
  version: "1.0"
  generated_at: ISO8601 timestamp
  generator: kongctl version
reference_mappings:
  resource_type:
    ref: resolved_id
changes:
  - id: unique change ID
    resource_type: portal|auth_strategy|etc
    resource_ref: declarative ref
    resource_name: display name
    action: CREATE|UPDATE
    current_state: (UPDATE only)
    desired_state: full resource
    field_changes: (UPDATE only)
summary:
  total_changes: number
  by_action: map
  by_resource: map
```

---

## ADR-002-005: Client-Side Resource Filtering

### Status
Proposed

### Context
SDK doesn't support server-side filtering by labels. We need to identify KONGCTL-managed
resources among all resources in Konnect.

### Decision
Implement client-side filtering:
- Fetch all resources using pagination
- Filter for presence of "KONGCTL/managed" label
- Cache results within command execution
- Log warning if large number of resources

### Consequences
- **Positive**: Works with current SDK capabilities
- **Positive**: Simple implementation
- **Negative**: Inefficient for large resource counts
- **Negative**: Fetches unnecessary data
- **Future**: Migrate to server-side filtering when available

### Implementation Notes
```go
func filterManagedResources(portals []Portal) []Portal {
    var managed []Portal
    for _, p := range portals {
        if labels := normalizeLabels(p.Labels); labels["KONGCTL/managed"] == "true" {
            managed = append(managed, p)
        }
    }
    return managed
}
```

---

## ADR-002-006: Plan File Format

### Status
Proposed

### Context
Plans need to be saved to files for later execution. Format must be readable and portable.

### Decision
Use JSON format with .json extension:
- Single file per plan
- Human-readable with proper indentation
- Standard JSON for tool compatibility
- No compression for simplicity

### Consequences
- **Positive**: Wide tool support for JSON
- **Positive**: Git-friendly for version control
- **Positive**: Easy to inspect and debug
- **Negative**: Larger file size than binary formats
- **Negative**: No built-in compression

### Alternative Considered
YAML format was considered but JSON chosen for:
- Simpler parsing
- Better performance
- Avoiding YAML type coercion issues