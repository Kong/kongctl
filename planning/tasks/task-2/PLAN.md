# Implementation Plan: Fix API Versions and Publications in Sync Operations

## Executive Summary

### Problem
The sync command fails to create API versions and publications despite them being defined in the YAML configuration. The root cause is a bug in the `filterResourcesByNamespace` function that uses API names instead of refs when building the filter map, causing a mismatch when filtering child resources that reference parents by ref.

### Solution Approach
Fix the `filterResourcesByNamespace` function to use API refs consistently, matching the pattern already correctly implemented for portal resources. This is a focused, low-risk change that will restore the intended functionality without affecting other parts of the system.

## Specific Code Changes

### 1. Primary Fix: Update filterResourcesByNamespace

**File**: `/internal/declarative/planner/planner.go`

**Lines to modify**: 545-570

**Changes**:
```go
// Line 545: Change variable name and use Ref instead of Name
- apiNames := make(map[string]bool)
+ apiRefs := make(map[string]bool)

// Line 547: Update to use Ref field
- apiNames[api.Name] = true
+ apiRefs[api.Ref] = true

// Line 552: Update reference in APIVersions filter
- if apiNames[version.API] {
+ if apiRefs[version.API] {

// Line 558: Update reference in APIPublications filter  
- if apiNames[publication.API] {
+ if apiRefs[publication.API] {

// Line 564: Update reference in APIImplementations filter
- if apiNames[implementation.API] {
+ if apiRefs[implementation.API] {

// Line 570: Update reference in APIDocuments filter
- if apiNames[document.API] {
+ if apiRefs[document.API] {
```

### 2. No Additional Code Changes Required

The fix is isolated to this single function. No other code changes are needed because:
- The loader already correctly extracts child resources and sets parent refs
- The API planner already has logic to handle child resources
- The resource structures and relationships are properly defined

## Implementation Steps

### Step 1: Verify Current Behavior
1. Build the current version: `make build`
2. Run the example sync command and capture output:
   ```bash
   ./kongctl sync -f docs/examples/declarative/portal/getting-started/portal.yaml \
                  -f docs/examples/declarative/portal/getting-started/apis.yaml \
                  --pat $(cat ~/.konnect/claude.pat) --dry-run
   ```
3. Confirm the plan shows APIs but no versions or publications

### Step 2: Apply the Fix
1. Open `/internal/declarative/planner/planner.go`
2. Navigate to line 545 (in the `filterResourcesByNamespace` function)
3. Apply all six changes listed above (lines 545, 547, 552, 558, 564, 570)
4. Save the file

### Step 3: Build and Verify
1. Rebuild: `make build`
2. Run linter: `make lint`
3. Run unit tests: `make test`
4. Run the same sync command again and verify versions/publications appear in the plan

### Step 4: Test Edge Cases
1. Test with APIs that have no child resources
2. Test with APIs that have only versions (no publications)
3. Test with APIs that have only publications (no versions)
4. Test with multiple APIs having cross-references

### Step 5: Integration Testing
1. Run full integration tests: `make test-integration`
2. Test actual sync operation (without --dry-run) in a test environment
3. Verify resources are created correctly in Konnect

## Testing Strategy

### Unit Tests
1. **Add test for filterResourcesByNamespace**:
   - Test filtering with parent-child relationships
   - Test that child resources are included when parent is present
   - Test that child resources are excluded when parent is absent

2. **Location for new tests**: `/internal/declarative/planner/planner_test.go`

### Integration Tests
1. **Enhance existing sync tests** to verify child resource creation
2. **Add specific test case** for API versions and publications
3. **Location**: `/test/integration/declarative/sync_test.go`

### Manual Testing Checklist
- [ ] APIs with nested versions and publications
- [ ] APIs with root-level versions and publications
- [ ] Mixed scenarios (some nested, some root-level)
- [ ] Error scenarios (invalid parent references)
- [ ] Performance with large numbers of child resources

## Edge Cases and Considerations

### 1. Backward Compatibility
- The fix maintains backward compatibility
- Existing configurations will continue to work
- Previously broken configurations will now work correctly

### 2. Parent Reference Validation
- Child resources with invalid parent refs will be correctly filtered out
- No change to existing validation behavior

### 3. Namespace Handling
- The fix respects namespace boundaries
- Child resources are only included if their parent is in the same namespace

### 4. Performance Impact
- Minimal performance impact (same algorithm, just using correct field)
- No additional loops or data structures

### 5. Related Resource Types
- Portal resources already use the correct pattern (portalRefs)
- This fix aligns API handling with the portal pattern
- Consider reviewing other resource types for similar issues

## Alternative Approaches (Not Recommended)

### 1. Use Name Field for Parent References
- Would require changing loader to set version.API = api.Name
- Breaking change for existing configurations
- Not recommended: refs are the intended identifier

### 2. Build Dual Maps (Name and Ref)
- Could maintain both apiNames and apiRefs maps
- Adds complexity without clear benefit
- Not recommended: single source of truth is better

### 3. Change Child Resource Structure
- Could embed child resources within parent
- Major architectural change
- Not recommended: current structure supports flexibility

## Risk Assessment

**Risk Level**: Low
- Focused change in a single function
- Follows existing pattern (portal resources)
- Well-understood issue with clear fix
- Easy to test and verify

## Success Criteria

1. Sync command creates API versions when defined in YAML
2. Sync command creates API publications when defined in YAML  
3. All existing tests continue to pass
4. No regression in other resource types
5. Parent-child relationships are maintained correctly

## Timeline

- Implementation: 30 minutes
- Testing: 1 hour
- Code review and adjustments: 30 minutes
- Total estimated time: 2 hours

## Conclusion

This plan addresses a critical bug that prevents proper API lifecycle management in Kong Konnect. The fix is straightforward, low-risk, and aligns with existing patterns in the codebase. Implementation should proceed immediately to restore full declarative configuration capabilities for API resources.