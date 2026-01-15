# Deck + Kongctl Declarative Integration

## Problem Statement

**TLDR**: Can we solve a dependency issue when using kongctl + deck to manage
Konnect & Kong GW resources by isolating or ignoring resources in a given
kongctl run?

### Context

When using kongctl and deck together for Konnect + Kong Gateway declarative
configuration:

- **kongctl** manages Konnect resources (APIs, Portals, Control Planes, API
  Implementations)
- **deck** manages Kong Gateway entities (Services, Routes, Plugins)

The division is unambiguous and easy to describe. The issue arises when there
are dependencies between them and running the tools independently causes a
temporal issue.

### The Temporal Dependency Problem

`api_implementations` is the current main use case - a kongctl-managed resource
with dependency linkage to:
- A Control Plane (kongctl-managed)
- A Gateway Service (deck-managed)

When using `kongctl sync` (CreateUpdateDelete operation):

| Step | Command | Result |
|------|---------|--------|
| 1 | `kongctl sync -f configs/*` | FAILS - api_implementation CREATE fails because Gateway Service doesn't exist |
| 2 | `deck sync deck-file.yaml` | Creates Gateway Service |
| 3 | `kongctl sync -f configs/*` | Now succeeds |

### Why Simple Workarounds Don't Work

1. **Omit api_implementation from first sync**: On subsequent syncs, sync mode
   would DELETE it (not in input = delete)

2. **Split configs across runs**: The declarative engine needs ALL resources to
   properly calculate plans. Can't pass only api_implementation in follow-up
   sync.

3. **Run kongctl sync twice**: Inefficient and error-prone

---

## Solution Options Analyzed

### Option A: Ignore/Isolate Flags (Recommended)

Add `--ignore-refs` and `--isolate-refs` flags to control which resources are
planned:

```shell
# Step 1: Create everything except api_implementation
kongctl sync -f configs/* --ignore-refs my-api-implementation

# Step 2: Create Gateway Service
deck sync deck-file.yaml

# Step 3: Create only api_implementation
kongctl sync -f configs/* --isolate-refs my-api-implementation
```

**Behavior:**
- `--ignore-refs`: Load all resources, resolve refs, but skip planning for
  specified refs. Resource data available for `!ref` resolution but no changes
  planned. In sync mode, ignored resources NOT deleted.
- `--isolate-refs`: Load all resources, resolve refs, plan ONLY specified refs.
  Parent IDs must be resolvable (parent exists in Konnect).

**Pattern syntax:**
- `my-api-impl` - Match by exact ref name
- `type:api_implementation` - Match all resources of a type
- `type:api_implementation,my-portal` - Mix types and refs

**Design decision: Support both refs and types.** For users with large resource
configuration sets, filtering by type is valuable. For example, ignoring ALL
api_implementations in one command rather than listing each ref individually.

**Pros:**
- Simple, explicit control
- Works with any external tool (not just deck)
- No magic or implicit behavior
- User controls timing
- Can ignore ANY resource for any reason

**Cons:**
- Requires 3 commands (can be scripted)
- User must understand dependency order
- Parent resources must still resolve for isolated children

### Option B: Soft-Fail with Pending State (Future Enhancement)

When external dependencies are unresolvable, mark resources as "pending"
instead of failing:

```shell
kongctl sync -f configs/* --allow-pending-external

# Output:
# Planning...
# ✓ API 'users-api' - CREATE
# ✓ Portal 'dev-portal' - CREATE
# ⏸ API Implementation 'users-api-impl' - PENDING (external: gateway_service not found)
#
# Plan: 2 changes, 1 pending
```

**How it works:**
1. During identity resolution, when external reference cannot be found, mark
   resource as "pending" instead of failing
2. Pending resources excluded from plan but tracked in metadata
3. Sync mode: pending resources NOT deleted
4. Subsequent runs re-evaluate; if dependency exists, resource moves to planned

**Implementation notes:**
- Codebase already has partial infrastructure: `ResolveResult.Errors` collects
  reference errors (but not currently checked), `[unknown]` pattern for forward
  references
- Would need: `PendingResource` type, `--allow-pending-external` flag, planner
  exclusion logic

**Pros:**
- Automatic detection of what can/can't proceed
- Natural idempotent workflow
- No user input needed to specify what to skip

**Cons:**
- More implicit behavior
- More complex implementation
- Only handles external dependency case (not general filtering)

### Option C: Two-Phase Plan Generation

Generate plan in explicit phases based on dependency analysis:

```shell
kongctl plan -f configs/* --output-phases
# Output: phase-1.json, phase-2.json
```

**Pros:** Explicit output, reviewable phases
**Cons:** More complex output format, user still runs deck between phases

### Option D: Deck Integration (Invoke Deck from Kongctl)

```yaml
external_tools:
  deck:
    file: deck-file.yaml
    run_before: api_implementations
```

**Pros:** Single command
**Cons:** Significant complexity, tight coupling, configuration overhead

---

## Option Comparison

| Aspect | Option A (Ignore/Isolate) | Option B (Pending) |
|--------|---------------------------|-------------------|
| **User control** | Explicit (user specifies) | Implicit (auto-detected) |
| **Commands needed** | 3 total | 3 total |
| **User input** | Must specify refs to filter | None - auto-detected |
| **Learning curve** | Must understand deps | Just keep running |
| **Implementation** | Simpler (filtering only) | More complex |
| **Flexibility** | Can ignore ANY resource | Only external deps |

**Note:** Both options require 3 commands total (2 kongctl + 1 deck). The
difference is whether the user specifies what to filter (A) or kongctl
auto-detects (B).

---

## Recommended Approach

**Start with Option A (Ignore/Isolate Flags)** because:
1. Simple, explicit, predictable
2. No implicit behavior or magic
3. Works with any external tool (not just deck)
4. Minimal implementation complexity
5. Users already understand the kongctl/deck split
6. Foundation for Option B later (pending = auto-ignored)

**Consider Option B as future enhancement** if:
- Users find the 3-command workflow cumbersome
- Deck integration becomes the dominant use case
- Demand for "just make it work" UX increases

---

## Implementation Design (Option A)

### CLI Flags

```
--ignore-refs=<pattern1,pattern2,...>    Skip planning for matching resources
--isolate-refs=<pattern1,pattern2,...>   Plan ONLY matching resources
```

Flags are mutually exclusive. Accept comma-separated or repeated flags.

**Supported type values:**
- `api`, `api_version`, `api_publication`, `api_implementation`, `api_document`
- `portal`, `portal_page`, `portal_snippet`, `portal_customization`, etc.
- `control_plane`, `gateway_service`
- `application_auth_strategy`, `catalog_service`

### Integration Point

Best location: After validation, before planner in `runPlan/runSync`:

```
1. LoadFromSources() → Full ResourceSet
2. ValidateNamespaceRequirement() → Unchanged
3. [NEW] ApplyResourceFilter(ignoreRefs, isolateRefs)
4. Planner.GeneratePlan() with filter applied
5. Plan JSON output
```

### Filter Behavior Details

**Ignore Mode:**
- Resource loaded into ResourceSet (available for `!ref` resolution)
- Resource NOT considered during planning (no CREATE/UPDATE/DELETE)
- Parent resources of ignored children planned normally
- Sync mode: Ignored resources not deleted even if "missing"

**Isolate Mode:**
- All resources loaded into ResourceSet
- ONLY specified refs are planned
- Parent IDs must be resolvable (parent exists in Konnect)
- If parent needs creation but not in isolate list: clear error message

### Dependency Handling

**For ignored resources:**
- Children of ignored parents: Also implicitly ignored
- Resources referencing ignored resources: Plan proceeds (ref in ResourceSet)

**For isolated resources:**
- Parent must exist in Konnect OR be in isolate list
- If parent doesn't exist: Clear error message
- If parent needs creation: Must be in isolate list

### Namespace Interaction

- `--require-namespace` validation runs on FULL ResourceSet (before filtering)
- Filtered resources still must pass namespace validation
- Prevents accidentally ignoring resources in wrong namespace

### Key Files to Modify

1. `internal/cmd/root/products/konnect/declarative/declarative.go` - Add flags
2. `internal/declarative/planner/planner.go` - Add filter to Options, apply in
   GeneratePlan
3. `internal/declarative/planner/api_planner.go` (and others) - Check filter
   before planning
4. `internal/declarative/planner/filter.go` (new) - Filter types and logic

---

## Architecture Context

### Current Pipeline Flow

```
Loader → ResourceSet → Validator → Planner → Plan JSON → Executor
```

### Key Observations from Codebase Analysis

1. **External resources exist**: `_external` blocks allow referencing
   deck-managed resources by ID or selector, but they must exist at planning
   time

2. **Namespace filtering**: Already implemented at planner level (per-namespace
   planning)

3. **Reference resolution**: Two-phase - identity resolution at load time,
   reference validation before planning

4. **Dependency tracking**: `DependsOn` and implicit dependencies via
   `References` with `ID="[unknown]"`

5. **Error handling**: Identity resolution failures abort planning immediately;
   reference resolution errors are collected but not currently checked
   (`ResolveResult.Errors`)

---

## Open Items

- [ ] Implement Option A (ignore/isolate flags)
- [ ] Consider file-based pattern input (`--ignore-file`) for complex CI/CD
- [ ] Evaluate Option B based on user feedback after Option A ships
