# Pagination Fix Plan

## Overview

This document outlines the plan to fix pagination inconsistencies in
`internal/declarative/state/client.go` and refactor to use the shared
`PaginateAll` helper for better maintainability.

## Problem Statement

Analysis of the codebase revealed pagination inconsistencies with three
different termination formulas being used:

1. **Formula 1** (Correct): `Meta.Page.Total <= float64(pageNumber * pageSize)`
   - Used by `PaginateAll` helper and 6 methods
   - Correctly stops after fetching all pages

2. **Formula 2** (Buggy): `Meta.Page.Total <= float64(pageSize * (pageNumber - 1))`
   - Used by 5 methods (see affected methods below)
   - Causes one unnecessary API call when total items is a multiple of page size

3. **Alternative approach**: `len(data) < pageSize`
   - Not currently used
   - More intuitive, doesn't depend on metadata accuracy

### Bug Impact

While functionally correct (tests pass), Formula 2 wastes API resources:

**Example: 250 total items, page size 100**
- Formula 2: Makes 4 API calls (pages 1, 2, 3, and unnecessary 4th returning 0 items)
- Formula 1: Makes 3 API calls (pages 1, 2, 3 only)
- Impact: ~33% more API calls in this scenario

## Affected Methods

All in `internal/declarative/state/client.go`:

1. `ListAPIVersions` (line ~1235)
2. `ListAPIPublications` (line ~1381)
3. `ListAPIImplementations` (line ~1488)
4. `ListPortalSnippets` (line ~2260)
5. `ListPortalTeams` (line ~2420)

## Implementation Plan

### Phase 1: Immediate Fix (Option A)

**Goal**: Fix the bug with minimal changes to maintain stability

**Changes**:
For each affected method, replace:
```go
if resp.ListXXXResponse.Meta.Page.Total <= float64(pageSize*(pageNumber-1)) {
    break
}
```

With:
```go
if resp.ListXXXResponse.Meta.Page.Total <= float64(pageSize*pageNumber) {
    break
}
```

**Affected Lines**:
- Line ~1235: `ListAPIVersions`
- Line ~1381: `ListAPIPublications`
- Line ~1488: `ListAPIImplementations`
- Line ~2260: `ListPortalSnippets`
- Line ~2420: `ListPortalTeams`

**Verification**:
1. Run existing tests: `make test`
2. Run integration tests: `make test-integration`
3. Verify no extra API calls using trace logs:
   ```sh
   KONGCTL_LOG_LEVEL=trace ./kongctl <command>
   ```

### Phase 2: Refactor to Shared Helper (Option C)

**Goal**: Eliminate duplication and centralize pagination logic

**Current State Analysis**:

The `PaginateAll` helper exists at
`internal/declarative/state/pagination.go`:
```go
func PaginateAll[T any](
    ctx context.Context,
    lister func(ctx context.Context, pageSize, pageNumber int64) ([]T, *PageMeta, error),
) ([]T, error)
```

**Successfully using `PaginateAll`**:
- `ListManagedPortals`
- `ListAllPortals`
- `ListManagedControlPlanes`
- `ListManagedAPIs`
- `ListManagedAuthStrategies`

**Manually paginating (need refactoring)**:
- `ListAllControlPlanes` (correct formula, should still refactor)
- `ListAllAPIs` (correct formula, should still refactor)
- `ListAPIVersions` (buggy formula - Phase 1 target)
- `ListAPIPublications` (buggy formula - Phase 1 target)
- `ListAPIImplementations` (buggy formula - Phase 1 target)
- `ListPortalSnippets` (buggy formula - Phase 1 target)
- `ListPortalTeams` (buggy formula - Phase 1 target)

**Special cases (different pagination patterns)**:
- `ListControlPlaneGroupMemberships` - Uses cursor-based pagination
- `ListGatewayServices` - Uses offset-based pagination

### Refactoring Approach

#### Step 1: Refactor Child Resource Methods

These methods paginate child resources and can be migrated to `PaginateAll`:

**1. ListAPIVersions**

Current (manual pagination):
```go
func (c *Client) ListAPIVersions(ctx context.Context, apiID string) ([]APIVersion, error) {
    var allVersions []APIVersion
    var pageNumber int64 = 1
    pageSize := int64(100)

    for {
        req := kkOps.ListAPIVersionsRequest{
            APIID:      apiID,
            PageSize:   &pageSize,
            PageNumber: &pageNumber,
        }

        resp, err := c.apiVersionAPI.ListAPIVersions(ctx, req)
        if err != nil {
            return nil, fmt.Errorf("failed to list API versions: %w", err)
        }

        if resp.ListAPIVersionResponse == nil || len(resp.ListAPIVersionResponse.Data) == 0 {
            break
        }

        for _, v := range resp.ListAPIVersionResponse.Data {
            version := APIVersion{
                ID:      v.ID,
                Version: v.Version,
                // ... other fields
            }
            allVersions = append(allVersions, version)
        }

        pageNumber++

        if resp.ListAPIVersionResponse.Meta.Page.Total <= float64(pageSize*(pageNumber-1)) {
            break
        }
    }

    return allVersions, nil
}
```

Refactored (using `PaginateAll`):
```go
func (c *Client) ListAPIVersions(ctx context.Context, apiID string) ([]APIVersion, error) {
    if c.apiVersionAPI == nil {
        return nil, fmt.Errorf("API version client not configured")
    }

    lister := func(ctx context.Context, pageSize, pageNumber int64) ([]APIVersion, *PageMeta, error) {
        req := kkOps.ListAPIVersionsRequest{
            APIID:      apiID,
            PageSize:   &pageSize,
            PageNumber: &pageNumber,
        }

        resp, err := c.apiVersionAPI.ListAPIVersions(ctx, req)
        if err != nil {
            return nil, nil, WrapAPIError(err, "list API versions", nil)
        }

        if resp.ListAPIVersionResponse == nil {
            return []APIVersion{}, &PageMeta{Total: 0}, nil
        }

        var versions []APIVersion
        for _, v := range resp.ListAPIVersionResponse.Data {
            version := APIVersion{
                ID:      v.ID,
                Version: v.Version,
                // ... other fields
            }
            versions = append(versions, version)
        }

        meta := &PageMeta{Total: resp.ListAPIVersionResponse.Meta.Page.Total}
        return versions, meta, nil
    }

    return PaginateAll(ctx, lister)
}
```

**Benefits**:
- Removes ~30 lines of boilerplate
- Centralizes pagination logic
- Eliminates potential for pagination bugs
- Consistent error handling

**2. Apply same pattern to other child resource methods**:
- `ListAPIPublications` - Similar structure
- `ListAPIImplementations` - Similar structure
- `ListPortalSnippets` - Similar structure
- `ListPortalTeams` - Similar structure

#### Step 2: Refactor "List All" Methods

These methods are already using correct formula but can still benefit
from refactoring:

**1. ListAllControlPlanes**

Current implementation manually paginates with correct formula. Can be
refactored using same pattern as managed resources.

**2. ListAllAPIs**

Same as above.

#### Step 3: Document Special Pagination Cases

Some resources use different pagination mechanisms and should be
documented but NOT refactored:

**Cursor-based pagination** (ListControlPlaneGroupMemberships):
```go
// Uses pageAfter cursor instead of pageNumber
for {
    req := kkOps.GetControlPlanesIDGroupMembershipsRequest{
        ID:       groupID,
        PageSize: &pageSize,
    }

    if pageAfter != nil {
        req.PageAfter = pageAfter
    }

    // ... fetch and process

    // Extract next cursor from response
    nextCursor := pagination.ExtractPageAfterCursor(meta.Page.Next)
    if nextCursor == "" {
        break
    }
    pageAfter = &nextCursor
}
```

**Offset-based pagination** (ListGatewayServices):
```go
// Uses offset instead of pageNumber
for {
    req := kkOps.ListServiceRequest{
        ControlPlaneID: controlPlaneID,
        Size:           &pageSize,
    }

    if hasOffset {
        req.Offset = &offsetVal
    }

    // ... fetch and process

    if resp.Object.Offset != nil && *resp.Object.Offset != "" {
        offsetVal = *resp.Object.Offset
        hasOffset = true
        continue
    }
    break
}
```

These should remain as-is because they don't fit the `PaginateAll` pattern.

### Implementation Steps for Phase 2

1. **Create feature branch**: `refactor/pagination-consolidation`

2. **Refactor child resource methods** (one commit per method):
   - `ListAPIVersions`
   - `ListAPIPublications`
   - `ListAPIImplementations`
   - `ListPortalSnippets`
   - `ListPortalTeams`

3. **Refactor "list all" methods** (one commit):
   - `ListAllControlPlanes`
   - `ListAllAPIs`

4. **Add documentation** to `pagination.go`:
   ```go
   // PaginateAll is a generic helper for paginating through API responses.
   // It handles the common pattern of:
   // 1. Making paginated API calls
   // 2. Accumulating results
   // 3. Stopping when all pages are fetched
   //
   // Use this for standard page-number-based pagination.
   // For cursor-based or offset-based pagination, implement manually.
   ```

5. **Verification**:
   - Run all tests: `make test`
   - Run integration tests: `make test-integration`
   - Verify API call counts with trace logging
   - Code review focusing on correctness

### Testing Strategy

#### Unit Tests

No new unit tests needed if existing tests cover the methods. However,
consider adding edge case tests:

```go
// Test pagination with exact page size multiples
func TestPaginationExactMultiple(t *testing.T) {
    // Mock 200 items with page size 100
    // Verify exactly 2 API calls made
}

// Test pagination with partial last page
func TestPaginationPartialPage(t *testing.T) {
    // Mock 250 items with page size 100
    // Verify exactly 3 API calls made
}

// Test pagination with empty result
func TestPaginationEmpty(t *testing.T) {
    // Mock 0 items
    // Verify exactly 1 API call made
}
```

#### Integration Tests

Use trace logging to verify actual API call counts:

```sh
# Before refactoring
KONGCTL_LOG_LEVEL=trace ./kongctl <command> 2>&1 | grep "HTTP request" | wc -l

# After refactoring
KONGCTL_LOG_LEVEL=trace ./kongctl <command> 2>&1 | grep "HTTP request" | wc -l

# Should be fewer or equal API calls
```

## Benefits Summary

### Phase 1 (Immediate Fix)
- ✅ Eliminates unnecessary API calls
- ✅ Reduces API load and latency
- ✅ Minimal risk (small change)
- ✅ Can be done in current PR

### Phase 2 (Refactoring)
- ✅ Eliminates ~150-200 lines of duplicated code
- ✅ Centralizes pagination logic (single source of truth)
- ✅ Makes future pagination bugs impossible in refactored methods
- ✅ Improves code maintainability
- ✅ Consistent error handling across all methods
- ✅ Easier to add features (e.g., configurable page sizes)

## Success Criteria

### Phase 1
- [ ] All 5 methods use correct termination formula
- [ ] All existing tests pass
- [ ] Integration tests pass
- [ ] No regression in functionality

### Phase 2
- [ ] 7 methods refactored to use `PaginateAll`
- [ ] All tests pass (unit + integration)
- [ ] API call counts verified with trace logging
- [ ] Code review approved
- [ ] Documentation updated

## Related Files

- `internal/declarative/state/client.go` - Main file with affected methods
- `internal/declarative/state/pagination.go` - Shared `PaginateAll` helper
- `internal/util/pagination/pagination.go` - Cursor extraction utilities

## References

- PR Review Comment: Line 2420 in `internal/declarative/state/client.go`
- Pagination analysis by Claude Code (this document)
- Kong Konnect SDK documentation for pagination patterns
