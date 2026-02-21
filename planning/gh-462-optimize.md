# GH-462 Declarative Request Optimization

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
