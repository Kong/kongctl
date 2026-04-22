# GH-239 Pagination Improvements

## Scope

This plan combines the work for:

- Issue `#239`: pagination consistency, refactoring, and pagination health
  across the codebase
- Issue `#39`: prevent silent truncation in shared pagination logic

This document records the current findings, the known defects, the test-first
strategy, and the implementation plan needed to close both issues with high
confidence.

## Goals

1. Fix pagination correctness before doing consistency refactors.
2. Follow a strict test-first workflow:
   - add failing tests first
   - commit tests before internal implementation changes
   - update internal code to conform to the tests
3. Add live Konnect E2E coverage for multi-page behavior.
4. Leave a clear record of what is intentionally shared, what must remain
   resource-specific, and where the SDK itself forces different patterns.

## Executive Summary

The repository does not have one pagination problem. It has several different
pagination families, implemented at multiple layers, with different levels of
quality:

- page-number pagination
- cursor pagination via `page[after]`
- offset pagination via `offset`
- SDK list operations that are not paginated at all

The largest active correctness bug is not the original off-by-one formula from
`#239`. The largest active correctness bug is that
`internal/declarative/state/PaginateAll` currently stops when the filtered
result slice for a page is empty. Several declarative state listers return
filtered results instead of raw page results, which means pagination can stop
early and silently miss later managed resources.

Issue `#39` is still valid, but it is too narrow by itself. The
`PaginateAllFiltered` helper has a hardcoded `>10` page cutoff and can
silently truncate large data sets, but the deeper shared-helper problem is
already present in active production code through `PaginateAll`.

There are also additional underfetch bugs in the Terraform import dump path.
Several helper functions claim to fetch "all" child resources, but only issue
one SDK list request even though the pinned SDK supports pagination for those
resources.

The correct closure plan is:

1. land failing tests first
2. fix shared helper semantics
3. fix the remaining stale manual loops
4. fix underfetch in `dump tf-import`
5. normalize cursor and offset handling
6. update the implementation guide so future code does not reintroduce the
   same bugs

## Investigation Facts

### SDK and API Facts

The repository is pinned to `github.com/Kong/sdk-konnect-go v0.31.0`.

The SDK is not uniform. It exposes different pagination capabilities depending
on the API family:

- Page-number APIs:
  - `ListAPIVersionsRequest`
  - `ListAPIPublicationsRequest`
  - `ListAPIImplementationsRequest`
  - `ListPortalSnippetsRequest`
  - `ListPortalTeamsRequest`
  - `ListPortalTeamRolesRequest`
- Cursor APIs using `page[after]`:
  - event gateway control planes and children
  - control plane group memberships
  - several other newer v3 list operations
- Offset APIs using `offset`:
  - gateway services
  - gateway routes
  - gateway plugins
  - gateway consumers
  - gateway upstreams
- SDK list operations without page-number or cursor parameters:
  - `ListPortalPagesRequest`
  - `ListAPIDocumentsRequest`

This means a single pagination helper cannot safely cover every API in the
repository. The implementation must stay family-aware.

### Repository Surface Area

Pagination logic currently exists in several layers:

- `internal/declarative/state`
- `internal/konnect/helpers`
- `internal/cmd/root/products/konnect/...`
- `internal/cmd/root/verbs/dump/...`

The command layer generally has the healthiest page-number implementations.
The helper layer and declarative layer contain the highest-risk defects.

### Direct HTTP Facts

Most pagination in `kongctl` goes through `sdk-konnect-go`.

The main custom raw HTTP paginated path currently in scope is
`internal/konnect/helpers/dcr_providers.go`. That code already forwards
`page[size]` and `page[number]` correctly and is not the main risk area.

I did not find another custom raw HTTP pagination stack comparable to the SDK
paths above. Audit log reads appear to be single-shot requests, not paginated
list walkers in the current code.

## Confirmed Defects

### 1. Active correctness bug: `PaginateAll` stops on filtered-empty pages

Evidence:

- `internal/declarative/state/pagination.go`
- `internal/declarative/state/client.go`
- `planning/DECLARATIVE_RESOURCE_IMPLEMENTATION_GUIDE.md`

Current `PaginateAll` behavior:

- appends `pageResults`
- breaks when `meta == nil || len(pageResults) == 0`
- otherwise uses `meta.Total` and `pageNumber`

This is incorrect for listers that return filtered results instead of the raw
page contents.

Affected active declarative listers include:

- `ListManagedPortals`
- `ListManagedControlPlanes`
- `ListManagedAPIs`
- `ListManagedCatalogServices`
- `ListManagedAuthStrategies`
- `ListManagedDCRProviders`
- `ListManagedOrganizationTeams`

These methods fetch a raw page from Konnect, filter out unmanaged resources,
then return the filtered slice to `PaginateAll`. If page 1 contains no managed
resources but page 2 does, pagination will stop after page 1 and silently miss
page 2.

This is a real correctness bug, not just a style issue.

### 2. Active documentation bug: the implementation guide teaches the broken
pattern

Evidence:

- `planning/DECLARATIVE_RESOURCE_IMPLEMENTATION_GUIDE.md`

The current guide recommends:

- fetch a page
- filter down to managed resources
- return the filtered slice
- call `PaginateAll`

That is the exact pattern that triggers the bug above.

If `#239` is addressed by refactoring more listers to the current helper
without first fixing the helper contract, the repository will spread the bug
to more resources.

### 3. Latent correctness bug: `PaginateAllFiltered` silently truncates after
10 pages

Evidence:

- `internal/declarative/state/pagination.go`
- `internal/declarative/state/pagination_test.go`

`PaginateAllFiltered` currently increments `pageNumber` and silently `break`s
when `pageNumber > 10`.

That is the bug tracked by `#39`.

Current impact:

- appears unused in production code today
- still unsafe as shared infrastructure
- still misleading as a helper because it reports success while truncating

### 4. Remaining off-by-one page-number loop

Evidence:

- `internal/declarative/state/client.go`

`ListPortalTeamRoles` still uses:

```go
if float64(pageSize*(pageNumber-1)) >= total { ... }
```

That is the stale formula pattern originally called out by `#239`.

Most of the originally reported instances have already been corrected, but this
one remains.

### 5. Underfetch bug in `dump tf-import` child resource helpers

Evidence:

- `internal/konnect/helpers/api_versions.go`
- `internal/konnect/helpers/api_publications.go`
- `internal/konnect/helpers/api_implementations.go`
- `internal/konnect/helpers/portals.go`
- `internal/cmd/root/verbs/dump/tfimport.go`

The following helpers claim to fetch all child resources but only make one
request:

- `GetVersionsForAPI`
- `GetPublicationsForAPI`
- `GetImplementationsForAPI`
- `GetSnippetsForPortal`

These helpers are used by `dump tf-import --include-child-resources`.

With enough child resources, Terraform import output can be incomplete even
though the CLI reports success.

This is a separate correctness issue from `#239` and `#39`, but it is clearly
part of pagination health in the repository and should be fixed in the same
effort.

### 6. Mixed offset semantics

Evidence:

- `internal/konnect/helpers/services.go`
- `internal/konnect/helpers/routes.go`
- `internal/konnect/helpers/plugins.go`
- `internal/declarative/state/client.go`
- `internal/cmd/root/products/konnect/gateway/route/interactive_children.go`

Some offset walkers continue whenever `Offset != nil`.

Other walkers only continue when the offset token is both non-`nil` and
non-empty, sometimes after trimming whitespace.

The stricter behavior is safer. A present but empty token should be treated as
end-of-pagination, not as an instruction to repeat the request.

This is a lower-priority issue than the active page-number bugs, but it should
be normalized while the pagination area is being touched.

### 7. Cursor extraction is duplicated

Evidence:

- `internal/util/pagination/pagination.go`
- multiple cursor walkers in `internal/declarative/state/client.go`
- multiple cursor walkers in `internal/cmd/root/products/konnect/eventgateway`

The repo already has `ExtractPageAfterCursor`, but many cursor walkers still
parse `Meta.Page.Next` manually with `url.Parse`.

The duplication is not the highest-risk bug, but it is unnecessary and makes
cursor handling less consistent than it needs to be.

### 8. Stale SDK assumption in portal snippet helper

Evidence:

- `internal/konnect/helpers/portals.go`

`GetSnippetsForPortal` still says the "public SDK v0.6.0 doesn't support
pagination for ListPortalSnippets", but the pinned SDK is `v0.31.0` and
`ListPortalSnippetsRequest` does support `page[size]` and `page[number]`.

This is both technical debt and a direct cause of the underfetch bug above.

## Non-Defects / Special Cases

The following behaviors are not bugs by themselves and should remain
resource-specific:

- `ListPortalPages`
  - the current SDK request type does not expose page-number parameters
- `ListAPIDocuments`
  - the current SDK request type does not expose page-number parameters
- cursor-based event gateway resources
  - these should not be forced into page-number helpers
- offset-based gateway resources
  - these should not be forced into `PaginateAll`

## Implications for Issue Closure

### Issue `#239`

`#239` can no longer be treated as only:

- fixing the old off-by-one formula
- consolidating more listers onto the current `PaginateAll`

That would not be sufficient and would be risky.

To close `#239` confidently, the repository must first correct shared helper
semantics, then centralize only where the shared helper is actually safe.

### Issue `#39`

`#39` should not be closed by replacing one silent break with another silent
behavior.

The correct closure standard for `#39` is:

- no shared pagination helper may silently truncate data
- helpers must either fetch all data or return an explicit error

## Test-First Strategy

### Rule

No internal implementation changes in `internal/` should be made before the
new tests are added.

### Commit Sequence

1. `test:` commit
   - add failing unit, integration, and e2e tests
   - do not change internal pagination logic in this commit
2. `declarative:` / `cmd:` / `konnect:` implementation commit
   - update helper contracts and fix the code to satisfy the tests
3. optional cleanup commit
   - remove stale comments
   - update guides and docs
   - normalize remaining low-risk inconsistencies

It is acceptable for the test commit to fail until the implementation commit
lands on the same branch.

## Planned Test Additions

### Unit Tests

#### `internal/declarative/state/pagination_test.go`

Add tests that describe the intended shared-helper behavior:

1. `TestPaginateAll_DoesNotStopOnFilteredEmptyPage`
   - page 1 returns `[]` after filtering
   - metadata shows more pages exist
   - page 2 returns the first managed result
   - expected result includes page 2 data
   - current code should fail this test

2. `TestPaginateAll_ExactPageMultipleDoesNotOverFetch`
   - total items exactly equal `pageSize * N`
   - assert the helper performs exactly `N` fetches
   - protects against off-by-one regressions

3. `TestPaginateAllFiltered_DoesNotSilentlyTruncate`
   - expected final behavior: explicit error or a redesigned helper that can
     prove completion
   - current code should not be allowed to succeed with partial data

4. `TestPaginateAllFiltered_LargeDataset`
   - use more than 10 pages
   - assert "success with truncated result" is impossible

#### `internal/declarative/state/client_test.go`

Add resource-level tests for active callers:

1. `ListManagedPortals` with:
   - page 1 containing only unmanaged portals
   - page 2 containing a managed portal
   - expected result includes page 2 managed portal

2. `ListManagedAPIs` with the same shape

3. `ListManagedControlPlanes` with the same shape

One resource test is enough to fail the helper bug, but covering a few real
callers makes it harder to regress.

4. `ListPortalTeamRoles` exact page-boundary behavior
   - total exactly equals one or more full pages
   - assert no extra request beyond the last real page

### Integration Tests

#### Declarative integration

Add a new integration-focused pagination test file under
`test/integration/declarative/`.

Primary target:

- create a declarative resource set for an already-existing managed resource
- mock the SDK list operation so:
  - page 1 contains only unmanaged objects
  - page 2 contains the managed object matching the declarative file
- expected plan is idempotent or update-oriented, not create-oriented

This test matters because it verifies the bug through the declarative planner
path, not only through a helper unit test.

Suggested cases:

1. managed portal appears only on page 2
2. managed API appears only on page 2

#### Dump / command integration

Add integration coverage for `dump tf-import --include-child-resources`.

The preferred location is a new integration test file under `test/integration`
that exercises the command layer with mocked APIs and asserts output content.

Primary targets:

1. portal snippets spanning two pages
   - expected Terraform import output contains all snippet blocks
2. API versions spanning two pages
   - expected output contains all version import blocks
3. API publications spanning two pages
4. API implementations spanning two pages

If command-level integration setup becomes too expensive, helper-level tests
may be used temporarily, but command integration is the preferred end state
because it verifies the path the user actually runs.

### E2E Tests

At least two new E2E scenarios should be added. They do not need to be the
first failing tests, but they are required before both issues are considered
done.

The E2E goal is to validate multi-page behavior against real Konnect using
small forced page sizes where the CLI supports them.

#### Scenario 1: portal snippet multi-page listing

Suggested path:

- `test/e2e/scenarios/portal/snippets-pagination/scenario.yaml`

Suggested flow:

1. reset org
2. create one portal plus more than one page of snippets
3. run `get portal snippets --portal-name <name> --page-size 5 -o json`
4. assert all snippets are returned
5. optionally run `get portal snippets <name>` lookups by name/title on later
   pages

Why this matters:

- validates real Konnect page-number pagination
- validates a command path that already threads request page size
- gives live coverage for a resource family related to the helper drift found
  in `GetSnippetsForPortal`

#### Scenario 2: API version multi-page listing

Suggested path:

- `test/e2e/scenarios/apis/versions-pagination/scenario.yaml`

Suggested flow:

1. reset org
2. create one API with more than one page of versions
3. run `get api versions --api-name <name> --page-size 5 -o json`
4. assert all versions are returned

Why this matters:

- validates a second page-number API family against live Konnect
- protects one of the resource types originally highlighted by `#239`

#### Scenario 3: tf-import child pagination

This scenario is highly desirable, but it depends on the implementation
threading request page size through the child-fetch helpers used by
`dump tf-import`.

Suggested path:

- `test/e2e/scenarios/dump/tf-import-child-pagination/scenario.yaml`

Suggested flow:

1. create a portal with many snippets or an API with many versions
2. run `dump tf-import --include-child-resources --page-size 5`
3. assert the output contains all expected child import blocks

This scenario should be added in the same branch before issue closure if the
helper signature and command plumbing make deterministic paging possible.

### Why E2E Will Not Cover Everything

The declarative `PaginateAll` filtered-page bug needs more than one full page
at the helper's internal page size of `100` and requires carefully arranged
managed vs unmanaged resources.

That is possible but heavy for live Konnect E2E, especially in a reset-based
test suite.

This is why the closure plan requires:

- unit tests for helper semantics
- integration tests for planner behavior
- e2e tests for live multi-page command behavior

The three layers serve different purposes.

## Planned Implementation Changes After Tests Land

### Phase 1: Fix shared page-number helper contracts

1. Redesign `PaginateAll` so pagination completion is based on raw page
   progress and metadata, not on the filtered result slice length.
2. Redesign `PaginateAllFiltered` so it cannot silently truncate:
   - preferred direction: the helper receives raw page results and applies the
     filter itself
   - alternate acceptable direction: the helper returns an explicit error when
     it cannot prove completion
3. Update the implementation guide so future declarative resource work uses the
   corrected contract.

### Phase 2: Fix remaining manual page-number bugs

1. Fix `ListPortalTeamRoles`.
2. Reevaluate all manual page-number loops in `internal/declarative/state` and
   refactor only where the corrected helper is a true fit.

### Phase 3: Fix `dump tf-import` child underfetch

Thread pagination through the helper functions that currently underfetch and
that are supported by the SDK:

- API versions
- API publications
- API implementations
- portal snippets

Keep these as special cases until proven otherwise:

- portal pages
- API documents

### Phase 4: Normalize cursor and offset handling

1. Prefer `ExtractPageAfterCursor` for cursor walkers.
2. Standardize offset walkers on:
   - continue only when the token is present and non-empty
   - trim whitespace before reuse where appropriate

### Phase 5: Cleanup

1. Remove stale SDK version comments.
2. Reevaluate dead or unused pagination abstractions such as
   `internal/declarative/common/pagination.go`.
3. Align low-value helper duplication with the chosen shared contracts.

## Acceptance Criteria

Both issues are only considered resolved when all of the following are true:

1. New unit tests, integration tests, and the planned E2E multi-page
   scenarios are present.
2. No shared pagination helper can silently truncate data.
3. Declarative managed resource listing does not stop on a filtered-empty page.
4. No remaining stale off-by-one page-number formula exists in active code.
5. `dump tf-import --include-child-resources` paginates all child resources
   where the current SDK supports pagination.
6. Cursor and offset walkers use consistent end-of-pagination rules.
7. `planning/DECLARATIVE_RESOURCE_IMPLEMENTATION_GUIDE.md` is updated so new
   code does not copy the broken pattern.
8. `make test`, `make test-integration`, and the relevant E2E scenarios pass.

## Open Questions

1. Should `PaginateAllFiltered` remain as a public helper, or should it be
   replaced with a safer contract now that it is currently unused?
2. Do we want to thread request page size into all child helper functions used
   by `dump tf-import`, or only the ones that are currently buggy?
3. Do we want one shared offset helper as part of this work, or should offset
   cleanup stay local to the touched files?

These questions do not block the test-first plan. They only affect the final
shape of the internal implementation.
