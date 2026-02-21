# GH-462 Declarative Request Optimization

## Notes: Strategy and Tradeoffs

### Strategy

- Treat each optimization pass as one isolated experiment:
  - pick one reproducible scenario
  - collect baseline request/latency data
  - make one focused change
  - re-run and compare
- Prioritize request elimination before micro-optimizing request duration.
- Target duplicate `GET` calls first, especially list calls repeated across:
  - identity resolution
  - resource planners
  - executor pre-create validation
- Use a planner-scoped cache for managed resources:
  - cache lifetime is one `GeneratePlan` run
  - cache keys are normalized namespace filters
  - wildcard (`*`) responses are reused to derive namespace-specific views
- Keep executor behavior simple for creates:
  - avoid duplicate pre-create existence reads when planner already performed
    state comparison
  - rely on create response/API conflict behavior for final authority
- Validate each pass with `go test ./...` and command analyzer output.

### Known Tradeoffs

- Pre-create existence checks removed for selected create flows reduce requests,
  but change where duplicate-name failures surface (API create instead of local
  pre-check).
- Wildcard-derived filtering assumes label/namespace filtering logic in memory
  matches server-side behavior; this is acceptable for managed resources but
  still a behavioral assumption.
- Cached reads can become stale within very long plans if remote state changes
  concurrently during planning; this is the standard consistency/performance
  tradeoff for request deduplication.
- We optimize for fewer calls first. For mixed heavy operations (large payload
  uploads), total elapsed time can improve less than request count.

## Optimization Pass 1: Simple API apply (`basic/api.yaml`)

### Goal

Reduce redundant API calls for a single API create during declarative `apply`.

### Baseline (before)

- Command:
  ```sh
  ./scripts/command-analyzer.sh -- apply \
    -f docs/examples/declarative/basic/api.yaml \
    --auto-approve
  ```
- Log file: `/tmp/kongctl-http.xZ9w.log`
- Requests: 5 total
- Route/method counts:
  - `GET /v3/apis`: 4
  - `POST /v3/apis`: 1
- Elapsed: 1019 ms

### Changes made

- Added planner-scoped managed API cache shared across one `GeneratePlan` run.
  - Added: `internal/declarative/planner/resource_cache.go`
  - Wired into planner lifecycle:
    - `internal/declarative/planner/planner.go`
- Updated API identity resolution to use cached managed API list when resolving
  `name[eq]=...` filters.
  - `internal/declarative/planner/planner.go`
- Updated API planning list call to use planner cache.
  - `internal/declarative/planner/api_planner.go`
- Removed API CREATE pre-execution existence lookup in executor to avoid
  duplicate read calls before create.
  - `internal/declarative/executor/executor.go`

### Result (after)

- Command:
  ```sh
  ./scripts/command-analyzer.sh -- apply \
    -f docs/examples/declarative/basic/api.yaml \
    --auto-approve
  ```
- Log file: `/tmp/kongctl-http.058U.log`
- Requests: 2 total
- Route/method counts:
  - `GET /v3/apis`: 1
  - `POST /v3/apis`: 1
- Elapsed: 495 ms

### Net improvement

- Request count: `5 -> 2` (60% reduction)
- Redundant API list calls removed from planner+executor path for this case.
- Elapsed time: `1019 ms -> 495 ms` (~51% reduction in this run)

## Optimization Pass 2: Simple Portal apply (`basic/portal.yaml`)

### Goal

Reduce redundant portal list calls during declarative `apply` for a single
portal create.

### Baseline (before)

- Command:
  ```sh
  ./scripts/command-analyzer.sh -- apply \
    -f docs/examples/declarative/basic/portal.yaml \
    --auto-approve
  ```
- Log file: `/tmp/kongctl-http.MSIw.log`
- Requests: 8 total
- Route/method counts:
  - `GET /v3/portals`: 7
  - `POST /v3/portals`: 1
- Elapsed: 778 ms

### Changes made

- Added planner-scoped managed portal cache (namespace-aware, per-plan-run).
  - `internal/declarative/planner/resource_cache.go`
- Updated portal planning to use cached managed portal lists.
  - `internal/declarative/planner/portal_planner.go`
  - `internal/declarative/planner/portal_child_planner.go`
- Updated portal identity resolution to use cached managed portals for
  `name[eq]=...` filter lookups.
  - `internal/declarative/planner/planner.go`
- Updated API publication portal mapping to use cached portal lists.
  - `internal/declarative/planner/api_planner.go`
- Removed portal CREATE pre-execution existence lookup in executor.
  - `internal/declarative/executor/executor.go`

### Result (after)

- Command:
  ```sh
  ./scripts/command-analyzer.sh -- apply \
    -f docs/examples/declarative/basic/portal.yaml \
    --auto-approve
  ```
- Log file: `/tmp/kongctl-http.L6Yn.log`
- Requests: 2 total
- Route/method counts:
  - `GET /v3/portals`: 1
  - `POST /v3/portals`: 1
- Elapsed: 407 ms

### Net improvement

- Request count: `8 -> 2` (75% reduction)
- Redundant portal list calls removed from planner+executor path for this case.
- Elapsed time: `778 ms -> 407 ms` (~48% reduction in this run)

## Optimization Pass 3: API + Portal + Publication (`api-with-portal-pub.yaml`)

### Goal

Reduce redundant auth strategy list calls in multi-resource apply flows.

### Baseline (before)

- Command:
  ```sh
  ./scripts/command-analyzer.sh -- apply \
    -f docs/examples/declarative/basic/api-with-portal-pub.yaml \
    --base-dir . \
    --auto-approve
  ```
- Log file: `/tmp/kongctl-http.UXPN.log`
- Requests: 10 total
- Route/method counts:
  - `GET /v2/application-auth-strategies`: 2
  - `GET /v3/apis`: 1
  - `GET /v3/portals`: 1
  - `POST /v2/application-auth-strategies`: 1
  - `POST /v3/apis`: 1
  - `POST /v3/portals`: 1
  - `PUT /v3/apis/<id>/publications/<id>`: 1
  - `PUT /v3/portals/<id>/assets/logo`: 1
  - `PUT /v3/portals/<id>/assets/favicon`: 1
- Elapsed: 1583 ms

### Changes made

- Added planner-scoped managed auth strategy cache (namespace-aware,
  per-plan-run).
  - `internal/declarative/planner/resource_cache.go`
- Updated auth strategy planner to use cached managed auth strategy listing.
  - `internal/declarative/planner/auth_strategy_planner.go`
- Updated auth strategy identity resolution to use cached managed list for
  `name[eq]=...` filter lookups.
  - `internal/declarative/planner/planner.go`

### Result (after)

- Command:
  ```sh
  ./scripts/command-analyzer.sh -- apply \
    -f docs/examples/declarative/basic/api-with-portal-pub.yaml \
    --base-dir . \
    --auto-approve
  ```
- Log file: `/tmp/kongctl-http.lFEz.log`
- Requests: 9 total
- Route/method counts:
  - `GET /v2/application-auth-strategies`: 1
  - `GET /v3/apis`: 1
  - `GET /v3/portals`: 1
  - `POST /v2/application-auth-strategies`: 1
  - `POST /v3/apis`: 1
  - `POST /v3/portals`: 1
  - `PUT /v3/apis/<id>/publications/<id>`: 1
  - `PUT /v3/portals/<id>/assets/logo`: 1
  - `PUT /v3/portals/<id>/assets/favicon`: 1
- Elapsed: 1576 ms

### Net improvement

- Request count: `10 -> 9` (10% reduction)
- Removed duplicate auth strategy list call between identity resolution and
  auth strategy planner.
- Latency impact in this run is minimal because saved request was a fast GET.

## Optimization Pass 4: Simple Control Plane apply (`control-plane/basic.yaml`)

### Goal

Reduce redundant control plane list calls during declarative `apply` for a
single control plane create.

### Baseline (before)

- Command:
  ```sh
  ./scripts/command-analyzer.sh -- apply \
    -f docs/examples/declarative/control-plane/basic.yaml \
    --auto-approve
  ```
- Log file: `/tmp/kongctl-http.XJRe.log`
- Requests: 5 total
- Route/method counts:
  - `GET /v2/control-planes`: 4
  - `POST /v2/control-planes`: 1
- Elapsed: 647 ms

### Changes made

- Added planner-scoped managed control plane cache (namespace-aware,
  per-plan-run).
  - `internal/declarative/planner/resource_cache.go`
- Updated control plane planner to use cached managed control plane listing.
  - `internal/declarative/planner/control_plane_planner.go`
- Updated control plane identity resolution to use cached managed list for
  `name[eq]=...` filter lookups.
  - `internal/declarative/planner/planner.go`
- Removed control plane CREATE pre-execution existence lookup in executor.
  - `internal/declarative/executor/executor.go`

### Result (after)

- Command:
  ```sh
  ./scripts/command-analyzer.sh -- apply \
    -f docs/examples/declarative/control-plane/basic.yaml \
    --auto-approve
  ```
- Log file: `/tmp/kongctl-http.TGhq.log`
- Requests: 2 total
- Route/method counts:
  - `GET /v2/control-planes`: 1
  - `POST /v2/control-planes`: 1
- Elapsed: 512 ms

### Net improvement

- Request count: `5 -> 2` (60% reduction)
- Redundant control plane list calls removed from planner+executor path for
  this case.
- Elapsed time: `647 ms -> 512 ms` (~21% reduction in this run)
