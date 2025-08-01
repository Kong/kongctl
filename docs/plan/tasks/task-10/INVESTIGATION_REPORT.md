# API Publication Reference Resolution Investigation Report

## Executive Summary

The sync command is failing when creating api_publication resources that have auth_strategy_ids references. The error shows that auth_strategy_ids.0 must match format "uuid" but it's receiving the ref string instead of the resolved UUID. This investigation reveals that the issue stems from incomplete reference resolution implementation in the planner and executor components.

## Issue Description

### Error Message
```
auth_strategy_ids.0 must match format "uuid"
```

### Root Cause
The auth_strategy_ids field is an array of references that need to be resolved to UUIDs before being sent to the Konnect API. However, the current implementation has several gaps:

1. The reference resolver only handles single string references, not arrays
2. The planner doesn't set up reference mappings for auth_strategy_ids
3. The new APIPublicationAdapter expects pre-resolved references but doesn't receive them

## Investigation Findings

### 1. Configuration Format

In configuration files, api_publications use auth_strategy_ids as an array of references:

```yaml
publications:
  - ref: securebank-public-publication
    portal_id: securebank-portal
    auth_strategy_ids:
      - securebank-oauth2-strategy  # This is a reference, not a UUID
      - securebank-apikey-strategy
```

### 2. Resource Definition

In `internal/declarative/resources/api_publication.go`:
- The resource defines `GetReferenceFieldMappings()` which maps:
  - `"portal_id"` → `"portal"`
  - `"auth_strategy_ids"` → `"application_auth_strategy"`
- This tells the system that auth_strategy_ids should reference application_auth_strategy resources

### 3. Planner Implementation Issues

In `internal/declarative/planner/api_planner.go`:
```go
func (p *Planner) planAPIPublicationCreate(...) {
    fields := make(map[string]interface{})
    fields["portal_id"] = publication.PortalID
    if publication.AuthStrategyIds != nil {
        fields["auth_strategy_ids"] = publication.AuthStrategyIds  // Array passed as-is
    }
    // ...
    
    // Portal reference is set up properly
    change.References["portal_id"] = ReferenceInfo{
        Ref: publication.PortalID,
        LookupFields: map[string]string{"name": portalName},
    }
    
    // BUT: No reference setup for auth_strategy_ids!
}
```

The planner:
- Correctly sets up references for portal_id
- Does NOT set up references for auth_strategy_ids
- Passes the auth_strategy_ids array directly in the fields map

### 4. Reference Resolver Limitations

In `internal/declarative/planner/resolver.go`:
```go
func (r *ReferenceResolver) extractReference(fieldName string, value interface{}) (string, bool) {
    // Only handles string values
    switch v := value.(type) {
    case string:
        if !isUUID(v) {
            return v, true
        }
    case FieldChange:
        // ...
    }
    return "", false
}
```

The resolver:
- Only processes single string values
- Cannot handle arrays of references
- Lists "auth_strategy_ids" as a reference field but can't process it

### 5. Executor Implementation Mismatch

#### Deprecated Implementation (Working)
In `internal/declarative/executor/api_publication_operations.go`:
```go
// The deprecated implementation handles resolution at execution time
if authStrategyIDs, ok := change.Fields["auth_strategy_ids"].([]interface{}); ok {
    ids := make([]string, 0, len(authStrategyIDs))
    for _, id := range authStrategyIDs {
        if strID, ok := id.(string); ok {
            if isUUID(strID) {
                ids = append(ids, strID)
            } else {
                // Resolves reference at execution time
                resolvedID, err := e.resolveAuthStrategyRef(ctx, strID)
                if err != nil {
                    return "", fmt.Errorf("failed to resolve auth strategy reference %q: %w", strID, err)
                }
                ids = append(ids, resolvedID)
            }
        }
    }
    publication.AuthStrategyIds = ids
}
```

#### New Implementation (Not Working)
In `internal/declarative/executor/api_publication_adapter.go`:
```go
func (a *APIPublicationAdapter) MapCreateFields(ctx context.Context, fields map[string]interface{},
    create *kkComps.APIPublication) error {
    change, _ := ctx.Value(contextKeyPlannedChange).(planner.PlannedChange)
    
    // Expects references to be pre-resolved
    if authStrategyRefs, ok := change.References["auth_strategy_ids"]; ok {
        if authStrategyRefs.ID != "" {
            // Expects a comma-separated list of resolved IDs
            ids := strings.Split(authStrategyRefs.ID, ",")
            create.AuthStrategyIds = ids
        }
    } else if authStrategyIDs, ok := fields["auth_strategy_ids"].(string); ok {
        // Falls back to string field (comma-separated)
        ids := strings.Split(authStrategyIDs, ",")
        create.AuthStrategyIds = ids
    } else if authStrategyIDsList, ok := fields["auth_strategy_ids"].([]string); ok {
        // Falls back to array field (unresolved references)
        create.AuthStrategyIds = authStrategyIDsList  // BUG: These are refs, not UUIDs!
    }
    // ...
}
```

### 6. Portal Custom Domain (Similar Issue)

The portal_custom_domain resource has a similar pattern where it needs to resolve a portal reference, but this is handled differently as it's a single reference, not an array.

## Solution Options

### Option 1: Fix Reference Resolver (Recommended)

Enhance the reference resolver to handle arrays of references:

1. Update `extractReference` to handle arrays:
```go
func (r *ReferenceResolver) extractReference(fieldName string, value interface{}) ([]string, bool) {
    if !r.isReferenceField(fieldName) {
        return nil, false
    }
    
    switch v := value.(type) {
    case string:
        if !isUUID(v) {
            return []string{v}, true
        }
    case []string:
        refs := []string{}
        for _, s := range v {
            if !isUUID(s) {
                refs = append(refs, s)
            }
        }
        return refs, len(refs) > 0
    case []interface{}:
        refs := []string{}
        for _, item := range v {
            if s, ok := item.(string); ok && !isUUID(s) {
                refs = append(refs, s)
            }
        }
        return refs, len(refs) > 0
    }
    return nil, false
}
```

2. Update planner to set up auth_strategy_ids references:
```go
// In planAPIPublicationCreate
if publication.AuthStrategyIds != nil && len(publication.AuthStrategyIds) > 0 {
    // Set up references for resolution
    var authStrategyNames []string
    for _, ref := range publication.AuthStrategyIds {
        // Look up auth strategy name for reference
        for _, strategy := range p.desiredAuthStrategies {
            if strategy.Ref == ref {
                authStrategyNames = append(authStrategyNames, strategy.Name)
                break
            }
        }
    }
    
    change.References["auth_strategy_ids"] = ReferenceInfo{
        Refs: publication.AuthStrategyIds,  // Array of refs
        LookupFields: map[string][]string{
            "names": authStrategyNames,
        },
    }
}
```

### Option 2: Resolve at Execution Time

Move reference resolution logic into the APIPublicationAdapter, similar to the deprecated implementation:

```go
func (a *APIPublicationAdapter) MapCreateFields(...) error {
    // ... existing code ...
    
    // Handle auth_strategy_ids resolution
    if authStrategyIDs, ok := fields["auth_strategy_ids"].([]string); ok {
        resolvedIDs := []string{}
        for _, ref := range authStrategyIDs {
            if isUUID(ref) {
                resolvedIDs = append(resolvedIDs, ref)
            } else {
                // Resolve reference using executor context
                id, err := a.resolveAuthStrategyRef(ctx, ref)
                if err != nil {
                    return fmt.Errorf("failed to resolve auth strategy reference %q: %w", ref, err)
                }
                resolvedIDs = append(resolvedIDs, id)
            }
        }
        create.AuthStrategyIds = resolvedIDs
    }
}
```

### Option 3: Pre-process References

Add a pre-processing step in the planner that converts array references to resolved IDs before creating the PlannedChange.

## Recommendation

**Option 1 (Fix Reference Resolver)** is the recommended approach because:
1. It maintains separation of concerns (planning vs execution)
2. It allows for better error handling and validation during planning
3. It's consistent with how other references are handled
4. It enables the planner to detect circular dependencies

## Impact Analysis

### Affected Components
1. `internal/declarative/planner/resolver.go` - Needs array handling
2. `internal/declarative/planner/api_planner.go` - Needs reference setup
3. `internal/declarative/executor/api_publication_adapter.go` - Needs to handle resolved arrays
4. Similar pattern may affect other resources with array references

### Testing Requirements
1. Unit tests for array reference resolution
2. Integration tests for api_publication creation with auth_strategy_ids
3. End-to-end tests for sync command with api_publications

## Conclusion

The API publication reference resolution issue is caused by incomplete implementation of array reference handling in the planner and resolver components. The system correctly identifies that auth_strategy_ids contains references but lacks the capability to resolve arrays of references. The recommended solution is to enhance the reference resolver to handle arrays and ensure the planner properly sets up these references for resolution.