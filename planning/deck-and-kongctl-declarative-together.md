# Deck + Kongctl Declarative Integration

## Problem Statement

**TLDR**: Can we solve a dependency issue when using kongctl + deck to manage
Konnect & Kong GW resources by isolating or ignoring resources in a given
kongctl run?

### ADR

From the below record of planning and design considerations, it is decided that the initial 
implementation will follow C.2 below: extending the `_external` block and shelling
out a command to `deck` with a constrained list of capabilities.

- Assume: the user has ran any deck file preprocessing stages prior to running
  the kongctl command which will execute the subsequent deck command
- Assume: required deck select tags are provided _within_ the deck configuration 
  allowing kongctl to execute either a deck sync or deck apply with expected results
- Assume: generally that what is required for deck via cli arg is provided within config
- Considerations
  - Users may have control plane names embedded in configuration but we need cp id as input
    to api_implementation resources. Investigate
- Pass through the `sync` or `apply` command through from `kongctl` to `deck`

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

### Option A: Ignore/Isolate Flags 

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

### Option B: Soft-Fail with Pending State 

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

### Option C: Deck Integration (Invoke Deck from Kongctl)

Run deck automatically from within kongctl to achieve single-command execution.

**Goal:** User runs one `kongctl sync` command that orchestrates deck internally.

**Pros:** Single command, best UX for kongctl+deck-centric workflows
**Cons:** More complex implementation, requires deck availability

See [Detailed Design: Option C](#detailed-design-option-d-deck-integration) below

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
- [ ] Prototype Option C if single-command UX is prioritized

---

## Detailed Design: Option C (Deck Integration)

This section provides implementation details for running deck from within kongctl.

### Bundling Options Analysis

#### Option C.1: Use `go-database-reconciler` Library (NOT recommended)

Kong maintains [go-database-reconciler](https://github.com/Kong/go-database-reconciler), a
library extracted from deck specifically for programmatic use. This is what deck uses
internally.

**Key API** (from [pkg/types](https://pkg.go.dev/github.com/kong/go-database-reconciler/pkg/types)):

```go
type EntityOpts struct {
    CurrentState  *state.KongState
    TargetState   *state.KongState
    KonnectClient *konnect.Client  // For Konnect
    IsKonnect     bool             // true for Konnect
}

// Create entity and perform diff/sync
entity, err := types.NewEntity(types.Service, opts)
differ := entity.Differ()
differ.CreateAndUpdates(func(event crud.Event) error { ... })
```

**Pros:**
- Native Go integration, no subprocess spawning
- Same code deck uses internally
- Type-safe, compile-time checks
- Better error handling and control
- No binary distribution complexity

**Cons:**
- Additional dependency in kongctl's go.mod
- Would need to keep in sync with go-database-reconciler releases
- Learning curve for the library API
- May need to handle auth token passing between kongctl and the library
- Suspect that some functionality is in the deck package itself vs go-database-reconciler

**Implementation Complexity:** High

#### Option C.2: Shell Out to Deck Binary (Recommended for MVP)

Execute deck as a subprocess and parse its output.

```go
cmd := exec.Command("deck", "gateway", "sync",
    "--konnect-token", token,
    "--konnect-control-plane-name", cpName,
    "--state", deckFile)
output, err := cmd.CombinedOutput()
```

**Pros:**
- Simplest implementation
- Deck maintained separately
- No new Go dependencies in kongctl

**Cons:**
- Requires deck binary on PATH
- Subprocess overhead
- `--json-output` format not well documented
- Error handling less granular
- Users use `deck file` functionality to pipleine behaviors, 
  it's unclear how this would fit into a kongctl managed workflow

**Implementation Complexity:** Low 

#### Option C.3: Embed Deck Binary (NOT Recommended)

Ship deck binary as an embedded resource or alongside kongctl.

**Pros:**
- No user installation of deck required
- Version consistency guaranteed

**Cons:**
- Significantly larger kongctl binary (~3x size increase)
- Platform-specific binaries needed (darwin/amd64, darwin/arm64, linux/amd64, etc.)
- Build complexity increases substantially
- Still subprocess communication overhead

**Implementation Complexity:** Medium (complex build/release process)

#### Recommendation

**Start with C.2 (shell out)**

1. MVP: Shell out to deck with required deck on PATH
2. V2: Import go-database-reconciler for native integration
3. Design `GatewayReconciler` interface to abstract the backend from the start

---

### YAML Syntax: Extend `_external` with Deck Provider

```yaml
# kongctl-config.yaml
control_planes:
  - ref: my-cp
    _external:
      selector:
        matchFields:
          name: "production-cp"

gateway_services:
  - ref: my-gw-service
    control_plane: my-cp
    _external:
      deck:                              # NEW: deck provider
        file: ./deck-state.yaml          # Relative to this config file
        # How do we identify the service in deck file or in Konnect to relate to this resource?
        # deck, like kongctl, supports both a sync and apply command. That's not expressed in the 
        #       declarative config, so how would we pass this through to the dependent deck command?
        # If deck sync is the desired command, it's a full CRD operation, so if there is a partial
        # file provided, it would delete resources. would we have to deal with 'select tags' inside here? 
        # they are cumbersome for the user and a different paradigm than kongctl's approach.

apis:
  - ref: my-api
    name: Users API

api_implementations:
  - ref: my-impl
    api: my-api
    service:
      id: !ref my-gw-service#id          # Resolved after deck sync
      control_plane_id: !ref my-cp#id
```

```yaml
# deck-state.yaml (standard deck format, unchanged)
_format_version: "3.0"
services:
  - name: users-service
    url: http://users.internal:8080
    routes:
      - name: users-route
        paths:
          - /users
```

---

### Execution Flow

```
1. LOADER/PARSER
   └─> Parse YAML with _external.deck blocks
   └─> Validate DeckProvider configuration
   └─> Path resolution: relative to config file (same as !file tag)

2. PLANNER
   └─> resolveControlPlaneIdentities() - resolve CP first (needed for deck)
   └─> resolveGatewayServiceIdentities() - detect deck-managed, skip Konnect query
   └─> Track DeckDependency in Plan.DeckDependencies[]
   └─> Generate Plan with pending references (ID = "[pending-deck]")

3. PRE-EXECUTION (NEW: DeckSynchronizer)
   └─> Check Plan.DeckDependencies
   └─> For each unique deck file:
       └─> Run: deck gateway sync --konnect-control-plane-name <cp> -s <file>
       └─> On failure: return error, abort entire operation
   └─> Query Konnect API for created services (by name)
   └─> Match services, update Plan.Changes[].References with resolved IDs

4. EXECUTION
   └─> Normal executor loop
   └─> api_implementation now has resolved service.id
```

---

### New Types

#### DeckProvider in `resources/external.go`

```go
// DeckProvider specifies deck-managed resource resolution
type DeckProvider struct {
    File    string `yaml:"file" json:"file"`       // Path to deck state file
    Service string `yaml:"service" json:"service"` // Service name in deck file
}

func (d *DeckProvider) Validate() error {
    if d.File == "" {
        return fmt.Errorf("deck provider requires 'file' field")
    }
    if d.Service == "" {
        return fmt.Errorf("deck provider requires 'service' field")
    }
    return nil
}
```

#### Extended ExternalBlock

```go
type ExternalBlock struct {
    ID       string            `yaml:"id,omitempty" json:"id,omitempty"`
    Selector *ExternalSelector `yaml:"selector,omitempty" json:"selector,omitempty"`
    Deck     *DeckProvider     `yaml:"deck,omitempty" json:"deck,omitempty"` // NEW
}

func (e *ExternalBlock) IsDeckManaged() bool {
    return e != nil && e.Deck != nil
}

func (e *ExternalBlock) Validate() error {
    // Ensure exactly one provider is set
    count := 0
    if e.ID != "" { count++ }
    if e.Selector != nil { count++ }
    if e.Deck != nil { count++ }

    if count == 0 {
        return fmt.Errorf("_external must have one of 'id', 'selector', or 'deck'")
    }
    if count > 1 {
        return fmt.Errorf("_external can only have one of 'id', 'selector', or 'deck'")
    }
    // ... validate individual providers
}
```

#### DeckDependency in `planner/types.go`

```go
// DeckDependency tracks a deck-managed resource needing synchronization
type DeckDependency struct {
    GatewayServiceRef string `json:"gateway_service_ref"`
    ControlPlaneID    string `json:"control_plane_id"`
    ControlPlaneName  string `json:"control_plane_name,omitempty"`
    DeckFile          string `json:"deck_file"`
    ServiceName       string `json:"service_name"`
    ResolvedServiceID string `json:"-"` // Populated after deck sync
}

// Plan struct - extended
type Plan struct {
    Metadata         PlanMetadata     `json:"metadata"`
    Changes          []PlannedChange  `json:"changes"`
    ExecutionOrder   []string         `json:"execution_order"`
    Summary          PlanSummary      `json:"summary"`
    Warnings         []PlanWarning    `json:"warnings,omitempty"`
    DeckDependencies []DeckDependency `json:"deck_dependencies,omitempty"` // NEW
}
```

---

### New Package: `internal/declarative/deck/`

#### reconciler.go - Interface Abstraction

```go
package deck

// GatewayReconciler abstracts gateway resource synchronization
// Allows MVP shell-out and future go-database-reconciler migration
type GatewayReconciler interface {
    Sync(ctx context.Context, opts SyncOptions) (*SyncResult, error)
}

type SyncOptions struct {
    ControlPlaneName string
    StateFile        string
    KonnectToken     string
    KonnectRegion    string
    DryRun           bool
}

type SyncResult struct {
    Services []SyncedService
    Output   string
}

type SyncedService struct {
    Name string
    ID   string
}
```

#### cli_reconciler.go - MVP Implementation

```go
// CLIReconciler implements GatewayReconciler by shelling out to deck binary
type CLIReconciler struct {
    logger *slog.Logger
}

func (r *CLIReconciler) Sync(ctx context.Context, opts SyncOptions) (*SyncResult, error) {
    deckPath, err := exec.LookPath("deck")
    if err != nil {
        return nil, &DeckNotFoundError{}
    }

    args := []string{
        "gateway", "sync",
        "--konnect-control-plane-name", opts.ControlPlaneName,
        "--konnect-token", opts.KonnectToken,
        "-s", opts.StateFile,
    }

    if opts.KonnectRegion != "" {
        args = append(args, "--konnect-addr",
            fmt.Sprintf("https://%s.api.konghq.com", opts.KonnectRegion))
    }

    cmd := exec.CommandContext(ctx, deckPath, args...)
    var stdout, stderr bytes.Buffer
    cmd.Stdout = &stdout
    cmd.Stderr = &stderr

    if err := cmd.Run(); err != nil {
        return nil, &DeckSyncError{
            File:     opts.StateFile,
            ExitCode: cmd.ProcessState.ExitCode(),
            Stderr:   stderr.String(),
        }
    }

    return &SyncResult{Output: stdout.String()}, nil
}
```

#### synchronizer.go - Coordination

```go
// DeckSynchronizer coordinates deck execution before kongctl
type DeckSynchronizer struct {
    reconciler GatewayReconciler
    client     *state.Client
    logger     *slog.Logger
    token      string
    region     string
}

func (s *DeckSynchronizer) SyncBeforeExecution(
    ctx context.Context,
    plan *planner.Plan,
    dryRun bool,
) error {
    if len(plan.DeckDependencies) == 0 {
        return nil
    }

    // 1. Group by deck file (avoid redundant syncs)
    fileToCP := make(map[string]string)
    for _, dep := range plan.DeckDependencies {
        fileToCP[dep.DeckFile] = dep.ControlPlaneName
    }

    // 2. Run deck sync for each unique file
    for file, cpName := range fileToCP {
        _, err := s.reconciler.Sync(ctx, SyncOptions{
            ControlPlaneName: cpName,
            StateFile:        file,
            KonnectToken:     s.token,
            KonnectRegion:    s.region,
            DryRun:           dryRun,
        })
        if err != nil {
            return err  // Abort on failure
        }
    }

    if dryRun {
        return nil
    }

    // 3. Resolve service IDs from Konnect API
    return s.resolveServiceIDs(ctx, plan)
}

func (s *DeckSynchronizer) resolveServiceIDs(ctx context.Context, plan *planner.Plan) error {
    // Query Konnect for services, match by name, update plan references
    // ...
}
```

#### errors.go - Error Types

```go
type DeckSyncError struct {
    File     string
    ExitCode int
    Stderr   string
}

func (e *DeckSyncError) Error() string {
    return fmt.Sprintf("deck sync failed for %s (exit %d): %s",
        e.File, e.ExitCode, e.Stderr)
}

type DeckNotFoundError struct{}

func (e *DeckNotFoundError) Error() string {
    return "deck binary not found on PATH"
}

type ServiceNotFoundError struct {
    ServiceName    string
    ControlPlaneID string
}
```

---

### Files to Modify

| File | Changes |
|------|---------|
| `internal/declarative/resources/external.go` | Add `DeckProvider`, update `ExternalBlock` |
| `internal/declarative/resources/gateway_service.go` | Add `IsDeckManaged()` method |
| `internal/declarative/planner/types.go` | Add `DeckDependency`, extend `Plan` struct |
| `internal/declarative/planner/planner.go` | Detect deck-managed in identity resolution |
| `internal/declarative/executor/executor.go` | Inject `DeckSynchronizer`, call pre-execution |
| **NEW:** `internal/declarative/deck/reconciler.go` | Interface definition |
| **NEW:** `internal/declarative/deck/cli_reconciler.go` | Shell-out implementation |
| **NEW:** `internal/declarative/deck/synchronizer.go` | Coordination logic |
| **NEW:** `internal/declarative/deck/errors.go` | Error types |

---

### Design Decisions

1. **Deck file path resolution:** Relative to the kongctl config file (consistent with
   `!file` tag behavior)

2. **Activation:** Automatic - if `_external.deck` is present, kongctl runs deck

3. **Error handling:** Abort entire operation on deck failure (fail-fast)

4. **Scope:** Konnect-only initially (use `--konnect-*` flags with deck)

5. **Multiple control planes:** Require one deck file per control plane for MVP

---

### Implementation Phases

**Phase 1: Core Types (1-2 days)**
- Add `DeckProvider` to `resources/external.go`
- Extend `ExternalBlock` validation
- Add `DeckDependency` to `planner/types.go`

**Phase 2: Deck Package (2-3 days)**
- Create `internal/declarative/deck/` package
- Implement `GatewayReconciler` interface
- Implement `CLIReconciler` (shell out to deck)
- Implement `DeckSynchronizer`

**Phase 3: Planner Integration (1-2 days)**
- Update `resolveGatewayServiceIdentities()` to detect deck-managed
- Track `DeckDependencies` during planning

**Phase 4: Executor Integration (1-2 days)**
- Inject `DeckSynchronizer` into `Executor`
- Add pre-execution deck sync call

**Phase 5: Testing (2-3 days)**
- Unit tests for new types
- Integration tests with mock deck
- E2E test with real deck binary

---

### References

- [deck gateway sync docs](https://developer.konghq.com/deck/gateway/sync/)
- [go-database-reconciler](https://github.com/Kong/go-database-reconciler)
- [pkg/types API](https://pkg.go.dev/github.com/kong/go-database-reconciler/pkg/types)
- [deck issue #1060 - library extraction](https://github.com/Kong/deck/issues/1060)
