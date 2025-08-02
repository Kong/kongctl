# Implementation Plan: Fix API Publication Creation Error

## Problem Summary

API publication creation fails with "API ID is required for publication operations" when publications are defined at the root level (extracted) rather than nested within API resources. This occurs due to a timing issue in reference resolution during the execution phase.

## Root Cause

When an API and its publications are created in the same sync operation:
1. The planner correctly sets up dependencies but passes an empty API ID for publications
2. During execution, even though the API is created first, its ID isn't propagated to dependent publication changes
3. The publication execution fails because it can't resolve the API reference

## Solution Approach

### Primary Solution: Reference Propagation

Implement a mechanism to propagate created resource IDs to dependent changes during execution.

#### 1. Add Reference Propagation Method in executor.go

**File**: `/internal/declarative/executor/executor.go`

After the `refToID` map definition (around line 88), add:

```go
// propagateCreatedResourceID updates references in pending changes when a resource is created
func (e *Executor) propagateCreatedResourceID(resourceType, resourceRef, resourceID string, pendingChanges []planner.PlannedChange) {
    for i := range pendingChanges {
        change := &pendingChanges[i]
        
        // Check all references in this change
        for refKey, refInfo := range change.References {
            // Match based on resource type from the reference key
            refResourceType := strings.TrimSuffix(refKey, "_id")
            if refResourceType == resourceType && refInfo.Ref == resourceRef {
                // Update the reference with the created resource ID
                refInfo.ID = resourceID
                change.References[refKey] = refInfo
                e.logger.WithFields(logrus.Fields{
                    "change_id":     change.ID,
                    "ref_key":       refKey,
                    "resource_type": resourceType,
                    "resource_ref":  resourceRef,
                    "resource_id":   resourceID,
                }).Debug("Propagated created resource ID to dependent change")
            }
        }
    }
}
```

#### 2. Call Propagation After Resource Creation

**File**: `/internal/declarative/executor/executor.go`

In the `executeChange` method, after line 284 where `refToID` is updated:

```go
// Store the reference for future lookups
if change.ResourceRef != "" {
    e.refToID[change.ResourceType][change.ResourceRef] = result.ID
}

// Propagate the created resource ID to any pending changes that reference it
remainingChanges := plan.ExecutionOrder[changeIndex+1:]
e.propagateCreatedResourceID(change.ResourceType, change.ResourceRef, result.ID, remainingChanges)
```

### Secondary Improvements

#### 3. Enhance Reference Resolution with Retry

**File**: `/internal/declarative/executor/executor.go`

Update `resolveAPIRef` method (starting at line 464):

```go
func (e *Executor) resolveAPIRef(refInfo planner.ResourceReference) (string, error) {
    // First, check if we've already created this API in the current execution
    if apiID, exists := e.refToID["api"][refInfo.Ref]; exists {
        e.logger.WithFields(logrus.Fields{
            "api_ref": refInfo.Ref,
            "api_id":  apiID,
        }).Debug("Resolved API reference from created resources")
        return apiID, nil
    }

    // Determine the lookup value
    lookupValue := refInfo.Ref
    if name, ok := refInfo.LookupFields["name"].(string); ok && name != "" {
        lookupValue = name
    }

    // Try to find the API in Konnect with retry for eventual consistency
    var lastErr error
    for attempt := 0; attempt < 3; attempt++ {
        if attempt > 0 {
            time.Sleep(time.Duration(attempt) * time.Second)
        }

        apiSvc := api.NewService(e.konnectClient, e.logger)
        apiResource, err := apiSvc.GetAPIByName(context.Background(), lookupValue)
        if err == nil && apiResource != nil {
            apiID := apiResource.ID
            e.logger.WithFields(logrus.Fields{
                "api_ref":      refInfo.Ref,
                "lookup_value": lookupValue,
                "api_id":       apiID,
                "attempt":      attempt + 1,
            }).Debug("Resolved API reference from Konnect")
            
            // Cache this resolution
            e.refToID["api"][refInfo.Ref] = apiID
            return apiID, nil
        }
        lastErr = err
    }

    return "", fmt.Errorf("failed to resolve API reference %s (lookup: %s) after 3 attempts: %w", 
        refInfo.Ref, lookupValue, lastErr)
}
```

#### 4. Improve Error Handling in API Publication Adapter

**File**: `/internal/declarative/executor/api_publication_adapter.go`

Update `getAPIID` method (starting at line 140):

```go
func (a *APIPublicationAdapter) getAPIID(ctx context.Context) (string, error) {
    // Get the planned change from context
    change, ok := ctx.Value(contextKeyPlannedChange).(planner.PlannedChange)
    if !ok {
        return "", fmt.Errorf("planned change not found in context")
    }

    // Get API ID from references
    if apiRef, ok := change.References["api_id"]; ok {
        if apiRef.ID != "" {
            return apiRef.ID, nil
        }
        
        // Log detailed information for debugging
        a.logger.WithFields(logrus.Fields{
            "change_id":     change.ID,
            "resource_ref":  change.ResourceRef,
            "api_ref":       apiRef.Ref,
            "lookup_fields": apiRef.LookupFields,
        }).Debug("API ID not found in reference, checking fallback options")
    }

    // Fallback: check if parent is set directly (for nested publications)
    if a.resource.Parent != "" {
        a.logger.WithFields(logrus.Fields{
            "parent": a.resource.Parent,
        }).Debug("Using parent field as API ID fallback")
        return a.resource.Parent, nil
    }

    return "", fmt.Errorf("API ID is required for publication operations (change: %s, ref: %s)", 
        change.ID, change.ResourceRef)
}
```

### Optional Enhancement: Pre-Resolution Phase

#### 5. Add Resource Resolution Before Planning

**File**: `/internal/declarative/planner/planner.go`

In `GeneratePlan` method, add before line 1570:

```go
// Pre-resolve existing resources from Konnect
if mode == PlanModeSync {
    if err := p.resolveExistingResources(ctx); err != nil {
        p.logger.WithError(err).Warn("Failed to pre-resolve existing resources, continuing without pre-resolution")
    }
}
```

Add new method:

```go
func (p *Planner) resolveExistingResources(ctx context.Context) error {
    p.logger.Debug("Pre-resolving existing resources from Konnect")
    
    // Resolve APIs
    apiSvc := api.NewService(p.konnectClient, p.logger)
    for _, desiredAPI := range p.GetDesiredAPIs() {
        if desiredAPI.GetKonnectID() == "" && desiredAPI.Name != "" {
            existingAPI, err := apiSvc.GetAPIByName(ctx, desiredAPI.Name)
            if err == nil && existingAPI != nil {
                desiredAPI.SetKonnectID(existingAPI.ID)
                p.logger.WithFields(logrus.Fields{
                    "api_name": desiredAPI.Name,
                    "api_id":   existingAPI.ID,
                }).Debug("Pre-resolved existing API")
            }
        }
    }
    
    // Similar resolution for other resource types as needed
    
    return nil
}
```

## Testing Strategy

### 1. Create Test Configuration Files

Create test files in a test directory:

**test_new_api_with_publications.yaml**:
```yaml
apis:
  - ref: test-api-1
    name: Test API One
    product_id: test-product
    specification:
      content: |
        openapi: 3.0.0
        info:
          title: Test API
          version: 1.0.0
        paths: {}

api_publications:
  - ref: test-pub-1
    api: test-api-1
    portal: test-portal
    auto_publish: true
```

**test_existing_api_publications.yaml**:
```yaml
# Assumes 'existing-api' already exists in Konnect
api_publications:
  - ref: existing-pub-1
    api: existing-api
    portal: test-portal
    auto_publish: true
```

### 2. Test Execution

```bash
# Test with dry-run first
./kongctl sync -f test_new_api_with_publications.yaml --dry-run --pat $(cat ~/.konnect/claude.pat)

# Execute actual sync
./kongctl sync -f test_new_api_with_publications.yaml --pat $(cat ~/.konnect/claude.pat)

# Verify the publication was created
./kongctl list api-publications --pat $(cat ~/.konnect/claude.pat)
```

### 3. Add Unit Tests

Create unit tests for:
- `propagateCreatedResourceID` method
- Enhanced `resolveAPIRef` with retry logic
- `getAPIID` with various reference states

## Implementation Order

1. **Phase 1 - Core Fix** (Priority: High)
   - Implement reference propagation in executor.go
   - Test with basic scenarios

2. **Phase 2 - Robustness** (Priority: Medium)
   - Add retry logic to resolveAPIRef
   - Improve error messages in api_publication_adapter
   - Add comprehensive logging

3. **Phase 3 - Enhancement** (Priority: Low)
   - Implement pre-resolution phase
   - Add more sophisticated caching

## Success Criteria

1. API publications can be created when defined at root level
2. Works for both new APIs and existing APIs
3. Clear error messages when legitimate failures occur
4. No regression in nested publication scenarios
5. All existing tests continue to pass

## Rollback Plan

If issues arise:
1. The changes are isolated and can be reverted independently
2. Reference propagation can be disabled by commenting out the propagation call
3. Retry logic has a maximum attempt limit to prevent infinite loops
4. Pre-resolution phase is optional and logs warnings instead of failing

## Monitoring

Add monitoring for:
- Number of reference propagations per sync
- Reference resolution success/failure rates
- Retry attempts in resolveAPIRef
- Time spent in pre-resolution phase