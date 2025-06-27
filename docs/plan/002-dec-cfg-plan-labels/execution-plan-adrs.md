# Stage 2: Architecture Decision Records (ADRs)

## Status: ✅ All ADRs Implemented

All architecture decisions documented here have been successfully implemented in Stage 2.

## ADR-002-001: Label Normalization Strategy

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

### Context
Declarative configs use references (e.g., `auth_strategy: "oauth-strategy"`). These must
be resolved to Konnect IDs at some point.

### Decision
Resolve references during plan generation where possible:
- Validate existing references at plan time
- Store both ref and resolved ID in the `references` field for debugging
- Use `<unknown>` placeholder for resources being created in same plan
- Support nested references with dot notation (e.g., `gateway_service.control_plane_id`)
- Fail fast if existing references cannot be resolved

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
```json
{
  "metadata": {
    "version": "1.0",
    "generated_at": "ISO8601 timestamp",
    "generator": "kongctl version"
  },
  "changes": [
    {
      "id": "{number}-{action}-{ref}",
      "resource_type": "portal|auth_strategy|etc",
      "resource_ref": "declarative ref",
      "resource_id": "UUID (UPDATE only)",
      "action": "CREATE|UPDATE",
      "fields": {
        "field_name": "value (CREATE)",
        "field_name": {"old": "x", "new": "y"} // UPDATE
      },
      "references": {
        "field_name": {"ref": "name", "id": "UUID or <unknown>"}
      },
      "parent": {
        "ref": "parent-ref",
        "id": "UUID or <unknown>"
      },
      "protection": true | {"old": false, "new": true},
      "config_hash": "sha256:...",
      "depends_on": ["change-id", ...]
    }
  ],
  "execution_order": ["ordered-change-ids"],
  "summary": {
    "total_changes": "number",
    "by_action": {"CREATE": "n", "UPDATE": "m"},
    "by_resource": {"portal": "x", "api": "y"},
    "protection_changes": {"protecting": "n", "unprotecting": "m"}
  },
  "warnings": [
    {"change_id": "id", "message": "warning text"}
  ]
}
```

---

## ADR-002-005: Client-Side Resource Filtering

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

---

## ADR-002-007: Semantic Change IDs

### Context
Change IDs need to be unique within a plan but also provide useful information for debugging
and human readability.

### Decision
Use semantic IDs with format `{number}-{action}-{ref}`:
- **Number**: Sequential ordering (1, 2, 3...)
- **Action**: Single letter (c=create, u=update, d=delete)
- **Ref**: Resource reference name

Example: `1-c-oauth-strategy`, `2-u-developer-portal`

### Consequences
- **Positive**: Human-readable IDs show action and resource at a glance
- **Positive**: Sequential numbers make execution order clear
- **Positive**: Easier debugging and plan review
- **Negative**: IDs are longer than simple numbers
- **Negative**: Must parse ID to extract components

### Alternative Considered
- UUIDs: Rejected for lack of human readability
- Simple numbers: Rejected for lack of context
- Hash-based: Rejected for complexity

---

## ADR-002-008: No Global Reference Mappings

### Context
Initial design included a global `reference_mappings` section at the plan root to store all
ref → ID mappings. Analysis showed this was redundant with data in individual changes.

### Decision
Remove global reference mappings:
- Each change stores its own reference data in the `references` field
- Resolved IDs are stored directly in the `fields` section
- No separate global mapping needed

### Consequences
- **Positive**: Eliminates redundancy (30-50% size reduction)
- **Positive**: Single source of truth per change
- **Positive**: Simpler plan structure
- **Negative**: Must scan changes to build complete mapping if needed
- **Negative**: No central lookup table

### Rationale
Every reference that needs resolution appears in some change. The change already stores:
1. The resolved ID in its fields
2. The original ref in its references field
Global mappings added no value while increasing plan size.

---

## ADR-002-009: Protection Change Isolation

### Context
Protected resources cannot be deleted. Changing protection status is a sensitive operation
that should be explicit and deliberate.

### Decision
Protection changes must be isolated:
- When changing protection status, no other fields can be modified
- Requires separate change entry just for protection
- Other field updates must be in a subsequent change with dependency

Example:
```json
{
  "id": "3-u-api-unprotect",
  "action": "UPDATE",
  "fields": {},  // Empty - no field changes allowed
  "protection": {"old": true, "new": false}
},
{
  "id": "4-u-api",
  "action": "UPDATE",
  "fields": {"deprecated": {"old": false, "new": true}},
  "depends_on": ["3-u-api-unprotect"]
}
```

### Consequences
- **Positive**: Makes protection changes explicit and auditable
- **Positive**: Prevents accidental protection removal
- **Positive**: Clear separation of concerns
- **Negative**: Requires two changes for protect+update scenarios
- **Negative**: More complex plan generation logic

---

## ADR-002-010: Minimal Field Storage for Updates

### Context
UPDATE operations could store complete current and desired states, but this creates large
plans with mostly redundant data.

### Decision
Store only changed fields for UPDATE operations:
- CREATE: Store all fields being set
- UPDATE: Store only fields that differ with old/new values
- Reduces plan size by 50-70% for typical updates

### Consequences
- **Positive**: Significantly smaller plan files
- **Positive**: Easier to see what's actually changing
- **Positive**: Less data to transmit and store
- **Negative**: Cannot see full resource state in plan
- **Negative**: Must fetch current state to apply updates

### Implementation
```json
// CREATE - all fields
"fields": {
  "name": "API Gateway",
  "description": "Main API Gateway",
  "enabled": true
}

// UPDATE - only changes
"fields": {
  "description": {
    "old": "Main API Gateway",
    "new": "Primary API Gateway v2"
  },
  "enabled": {
    "old": true,
    "new": false
  }
}
```

---

## ADR-002-011: Enhanced Reference Tracking

### Context
When references are resolved to IDs, we lose the connection between the original reference
name and the resolved ID, making debugging difficult.

### Decision
Store both reference name and resolved ID in the `references` field:
```json
"references": {
  "default_application_auth_strategy_id": {
    "ref": "oauth-strategy",
    "id": "456e7890-1234-5678-9abc-def012345678"
  }
}
```

Use `<unknown>` for resources being created in the same plan.

### Consequences
- **Positive**: Complete audit trail of reference resolution
- **Positive**: Easy debugging of reference issues
- **Positive**: Can validate resolution was correct
- **Negative**: Slight redundancy with field values
- **Negative**: Additional data in plan

### Rationale
The redundancy is minimal and the debugging value is significant. Being able to trace
"this UUID came from this reference" is valuable for troubleshooting.