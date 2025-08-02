# API Publication Reference Resolution Implementation Plan

## 1. Problem Statement

The `kongctl sync` command fails when creating `api_publication` resources that contain `auth_strategy_ids` references. The error occurs because:

- The `auth_strategy_ids` field expects an array of UUIDs
- The configuration provides an array of reference strings (e.g., `["oauth2-strategy", "apikey-strategy"]`)
- The current system cannot resolve array references to their corresponding UUIDs
- The API rejects the request with: `auth_strategy_ids.0 must match format "uuid"`

### Root Causes
1. **Planner Gap**: The planner doesn't set up references for the `auth_strategy_ids` array field
2. **Resolver Limitation**: The reference resolver only handles single string values, not arrays
3. **Adapter Assumption**: The new adapter expects pre-resolved references but receives raw reference strings

## 2. Recommended Solution: Fix Reference Resolver (Option 1)

This approach maintains architectural consistency by enhancing the existing reference resolution mechanism to support arrays. This is preferable to execution-time resolution because it:
- Maintains separation of concerns between planning and execution phases
- Enables dependency tracking and circular reference detection
- Provides better error handling during the planning phase
- Creates a reusable solution for future array reference fields

## 3. Implementation Steps

### Step 1: Update ReferenceInfo Structure

**File**: `internal/declarative/planner/types.go`

Add support for array references in the ReferenceInfo struct:

```go
type ReferenceInfo struct {
    // Existing fields for single references
    Ref          string                 `json:"ref,omitempty"`
    ID           string                 `json:"id,omitempty"`
    LookupFields map[string]string      `json:"lookup_fields,omitempty"`
    
    // New fields for array references
    Refs         []string               `json:"refs,omitempty"`         // Array of reference strings
    ResolvedIDs  []string               `json:"resolved_ids,omitempty"` // Array of resolved UUIDs
    LookupArrays map[string][]string    `json:"lookup_arrays,omitempty"` // Array lookup fields
    IsArray      bool                   `json:"is_array,omitempty"`     // Flag to indicate array reference
}
```

### Step 2: Enhance Reference Resolver

**File**: `internal/declarative/planner/resolver.go`

#### 2.1 Update extractReference to handle arrays:

```go
// Update the function signature to return multiple references
func (r *ReferenceResolver) extractReferences(fieldName string, value interface{}) ([]string, bool) {
    if !r.isReferenceField(fieldName) {
        return nil, false
    }
    
    var refs []string
    
    switch v := value.(type) {
    case string:
        // Single string reference
        if !isUUID(v) {
            refs = append(refs, v)
        }
    case []string:
        // Array of string references
        for _, s := range v {
            if !isUUID(s) {
                refs = append(refs, s)
            }
        }
    case []interface{}:
        // Array from unmarshaled JSON/YAML
        for _, item := range v {
            if s, ok := item.(string); ok && !isUUID(s) {
                refs = append(refs, s)
            }
        }
    case FieldChange:
        // Handle FieldChange for updates
        if fc, ok := v.(FieldChange); ok && fc.New != nil {
            return r.extractReferences(fieldName, fc.New)
        }
    }
    
    return refs, len(refs) > 0
}
```

#### 2.2 Update findReferences to use the new function:

```go
func (r *ReferenceResolver) findReferences(change *PlannedChange) map[string]ReferenceInfo {
    references := make(map[string]ReferenceInfo)
    
    for fieldName, value := range change.Fields {
        if refs, hasRef := r.extractReferences(fieldName, value); hasRef {
            if len(refs) == 1 {
                // Single reference
                references[fieldName] = ReferenceInfo{
                    Ref:     refs[0],
                    IsArray: false,
                }
            } else if len(refs) > 1 {
                // Array of references
                references[fieldName] = ReferenceInfo{
                    Refs:    refs,
                    IsArray: true,
                }
            }
        }
    }
    
    // Merge with explicitly set references
    for k, v := range change.References {
        references[k] = v
    }
    
    return references
}
```

### Step 3: Update Planner for auth_strategy_ids

**File**: `internal/declarative/planner/api_planner.go`

#### 3.1 Modify planAPIPublicationCreate function (around line 798):

```go
func (p *Planner) planAPIPublicationCreate(publication resources.APIPublicationResource) *PlannedChange {
    fields := make(map[string]interface{})
    fields["portal_id"] = publication.PortalID
    fields["api_id"] = publication.APIID
    
    // Still include in fields for backward compatibility
    if publication.AuthStrategyIds != nil {
        fields["auth_strategy_ids"] = publication.AuthStrategyIds
    }
    
    // ... existing field setup ...
    
    change := &PlannedChange{
        Type:       "api_publication",
        Action:     ActionCreate,
        Fields:     fields,
        References: make(map[string]ReferenceInfo),
    }
    
    // ... existing api_id and portal_id reference setup ...
    
    // NEW: Set up auth_strategy_ids references
    if publication.AuthStrategyIds != nil && len(publication.AuthStrategyIds) > 0 {
        var authStrategyNames []string
        
        // Look up names for each auth strategy reference
        for _, ref := range publication.AuthStrategyIds {
            // Find the auth strategy in desired state
            for _, strategy := range p.desiredAuthStrategies {
                if strategy.Ref == ref {
                    authStrategyNames = append(authStrategyNames, strategy.Name)
                    break
                }
            }
        }
        
        // Set up array reference
        change.References["auth_strategy_ids"] = ReferenceInfo{
            Refs:         publication.AuthStrategyIds,
            IsArray:      true,
            LookupArrays: map[string][]string{
                "names": authStrategyNames,
            },
        }
    }
    
    return change
}
```

### Step 4: Update Executor Resolution Logic

**File**: `internal/declarative/executor/executor.go`

#### 4.1 Update reference resolution in executeCreate (around line 648):

```go
func (e *Executor) executeCreate(ctx context.Context, change planner.PlannedChange) (string, error) {
    // ... existing code ...
    
    switch change.Type {
    case "api_publication":
        // Resolve references before creation
        for fieldName, refInfo := range change.References {
            if refInfo.IsArray {
                // Handle array references
                resolvedIDs := make([]string, 0, len(refInfo.Refs))
                
                for i, ref := range refInfo.Refs {
                    var resolvedID string
                    var err error
                    
                    switch fieldName {
                    case "auth_strategy_ids":
                        // Check if already resolved
                        if refInfo.ResolvedIDs != nil && i < len(refInfo.ResolvedIDs) && refInfo.ResolvedIDs[i] != "" {
                            resolvedID = refInfo.ResolvedIDs[i]
                        } else {
                            // Resolve using name from LookupArrays
                            var lookupName string
                            if names, ok := refInfo.LookupArrays["names"]; ok && i < len(names) {
                                lookupName = names[i]
                            }
                            
                            resolvedID, err = e.resolveAuthStrategyRef(ctx, ref)
                            if err != nil {
                                return "", fmt.Errorf("failed to resolve auth strategy reference %q: %w", ref, err)
                            }
                        }
                    }
                    
                    if resolvedID == "" {
                        return "", fmt.Errorf("failed to resolve reference %q for field %s", ref, fieldName)
                    }
                    
                    resolvedIDs = append(resolvedIDs, resolvedID)
                }
                
                // Update the reference with resolved IDs
                refInfo.ResolvedIDs = resolvedIDs
                change.References[fieldName] = refInfo
                
            } else {
                // Handle single references (existing code)
                // ... existing single reference resolution ...
            }
        }
        
        // ... rest of api_publication creation ...
    }
}
```

### Step 5: Update API Publication Adapter

**File**: `internal/declarative/executor/api_publication_adapter.go`

#### 5.1 Modify MapCreateFields to handle resolved arrays:

```go
func (a *APIPublicationAdapter) MapCreateFields(ctx context.Context, fields map[string]interface{},
    create *kkComps.APIPublication) error {
    
    change, _ := ctx.Value(contextKeyPlannedChange).(planner.PlannedChange)
    
    // Handle API ID
    if apiRef, ok := change.References["api_id"]; ok && apiRef.ID != "" {
        create.APIID = apiRef.ID
    } else if apiID, ok := fields["api_id"].(string); ok {
        create.APIID = apiID
    }
    
    // Handle Portal ID
    if portalRef, ok := change.References["portal_id"]; ok && portalRef.ID != "" {
        create.PortalID = &portalRef.ID
    } else if portalID, ok := fields["portal_id"].(string); ok {
        create.PortalID = &portalID
    }
    
    // NEW: Handle auth_strategy_ids array references
    if authStrategyRefs, ok := change.References["auth_strategy_ids"]; ok && authStrategyRefs.IsArray {
        if authStrategyRefs.ResolvedIDs != nil && len(authStrategyRefs.ResolvedIDs) > 0 {
            create.AuthStrategyIds = authStrategyRefs.ResolvedIDs
        }
    } else if authStrategyIDs, ok := fields["auth_strategy_ids"].([]interface{}); ok {
        // Fallback: Convert interface array to string array
        ids := make([]string, 0, len(authStrategyIDs))
        for _, id := range authStrategyIDs {
            if strID, ok := id.(string); ok {
                ids = append(ids, strID)
            }
        }
        create.AuthStrategyIds = ids
    } else if authStrategyIDsList, ok := fields["auth_strategy_ids"].([]string); ok {
        // Direct array assignment
        create.AuthStrategyIds = authStrategyIDsList
    }
    
    // ... rest of the field mapping ...
    
    return nil
}
```

### Step 6: Portal Custom Domain Fix

**File**: `internal/declarative/planner/portal_planner.go`

Add reference setup for portal custom domains:

```go
func (p *Planner) planPortalDomainCreate(domain resources.PortalDomainResource) *PlannedChange {
    fields := make(map[string]interface{})
    fields["domain"] = domain.Domain
    fields["portal_id"] = domain.PortalID
    
    // ... existing field setup ...
    
    change := &PlannedChange{
        Type:       "portal_custom_domain",
        Action:     ActionCreate,
        Fields:     fields,
        References: make(map[string]ReferenceInfo),
    }
    
    // NEW: Set up portal reference
    if domain.PortalID != "" && !isUUID(domain.PortalID) {
        // Look up portal name
        var portalName string
        for _, portal := range p.desiredPortals {
            if portal.Ref == domain.PortalID {
                portalName = portal.Name
                break
            }
        }
        
        change.References["portal_id"] = ReferenceInfo{
            Ref:          domain.PortalID,
            LookupFields: map[string]string{
                "name": portalName,
            },
        }
    }
    
    return change
}
```

## 4. Testing Approach

### 4.1 Unit Tests

**File**: `internal/declarative/planner/resolver_test.go`

```go
func TestExtractReferences_ArraySupport(t *testing.T) {
    resolver := NewReferenceResolver()
    
    tests := []struct {
        name      string
        fieldName string
        value     interface{}
        wantRefs  []string
        wantOk    bool
    }{
        {
            name:      "array of strings",
            fieldName: "auth_strategy_ids",
            value:     []string{"oauth-strategy", "apikey-strategy"},
            wantRefs:  []string{"oauth-strategy", "apikey-strategy"},
            wantOk:    true,
        },
        {
            name:      "array with UUIDs filtered",
            fieldName: "auth_strategy_ids",
            value:     []string{"oauth-strategy", "550e8400-e29b-41d4-a716-446655440000"},
            wantRefs:  []string{"oauth-strategy"},
            wantOk:    true,
        },
        {
            name:      "interface array",
            fieldName: "auth_strategy_ids",
            value:     []interface{}{"oauth-strategy", "apikey-strategy"},
            wantRefs:  []string{"oauth-strategy", "apikey-strategy"},
            wantOk:    true,
        },
        {
            name:      "single string still works",
            fieldName: "portal_id",
            value:     "my-portal",
            wantRefs:  []string{"my-portal"},
            wantOk:    true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            gotRefs, gotOk := resolver.extractReferences(tt.fieldName, tt.value)
            assert.Equal(t, tt.wantOk, gotOk)
            assert.Equal(t, tt.wantRefs, gotRefs)
        })
    }
}
```

### 4.2 Integration Tests

**File**: `test/integration/api_publication_test.go`

```go
func TestAPIPublicationWithAuthStrategies(t *testing.T) {
    // Set up test with auth strategies and API publication
    config := `
apis:
  - ref: test-api
    name: Test API
    # ... api config ...

auth_strategies:
  - ref: oauth-strategy
    name: OAuth Strategy
    type: oauth2
    # ... strategy config ...
  - ref: apikey-strategy  
    name: API Key Strategy
    type: key_auth
    # ... strategy config ...

publications:
  - ref: test-publication
    api_id: test-api
    portal_id: test-portal
    auth_strategy_ids:
      - oauth-strategy
      - apikey-strategy
`
    
    // Run sync command
    err := runSync(t, config)
    require.NoError(t, err)
    
    // Verify publication was created with resolved UUIDs
    publication := getAPIPublication(t, "test-publication")
    assert.Len(t, publication.AuthStrategyIds, 2)
    
    // Verify UUIDs are valid
    for _, id := range publication.AuthStrategyIds {
        assert.True(t, isUUID(id), "Expected UUID but got: %s", id)
    }
}
```

## 5. Validation Steps

### 5.1 Manual Testing

1. **Basic Validation**:
   ```bash
   # Create test configuration with api_publication and auth_strategy_ids
   cat > test-config.yaml <<EOF
   auth_strategies:
     - ref: oauth-strategy
       name: OAuth2 Strategy
       type: oauth2
       # ... config ...
   
   publications:
     - ref: test-pub
       api_id: my-api
       portal_id: my-portal
       auth_strategy_ids:
         - oauth-strategy
   EOF
   
   # Run sync command
   ./kongctl sync -f test-config.yaml --pat $(cat ~/.konnect/claude.pat)
   
   # Verify no UUID format errors
   ```

2. **Edge Cases**:
   - Empty auth_strategy_ids array
   - Single auth strategy reference
   - Multiple auth strategy references
   - Mix of references and UUIDs (if pre-resolved)

### 5.2 Automated Validation

Add to CI/CD pipeline:
```bash
# Run all tests including new integration tests
make test
make test-integration

# Verify no regression in existing functionality
./scripts/test-reference-resolution.sh
```

## 6. Risk Mitigation

### 6.1 Backward Compatibility

- The solution maintains backward compatibility by:
  - Still supporting single string references
  - Keeping the existing field structure
  - Not breaking existing reference resolution logic

### 6.2 Error Handling

- **Partial Resolution Failure**: If any reference in an array fails to resolve, the entire operation should fail with a clear error message
- **Circular Dependencies**: The existing dependency tracking will detect circular references in arrays
- **Performance**: Array resolution is O(n) where n is the number of references

### 6.3 Rollback Plan

If issues arise:
1. The deprecated implementation in `api_publication_operations.go` can be temporarily re-enabled
2. The feature flag `USE_NEW_ADAPTERS` can be disabled
3. A hotfix can revert to execution-time resolution

## 7. Implementation Order

Execute in this specific order to ensure each change builds on the previous:

1. **Step 1**: Update ReferenceInfo structure (no functional impact)
2. **Step 2**: Enhance reference resolver (backwards compatible)
3. **Step 3**: Update planner for auth_strategy_ids
4. **Step 4**: Update executor resolution logic
5. **Step 5**: Fix API publication adapter
6. **Step 6**: Fix portal custom domain planner
7. **Testing**: Run unit tests after each step
8. **Integration**: Run full integration test suite
9. **Validation**: Manual testing with real Konnect environment

## 8. Success Criteria

The implementation is successful when:

1. `kongctl sync` successfully creates api_publications with auth_strategy_ids
2. No "must match format uuid" errors occur
3. All existing tests continue to pass
4. New unit and integration tests pass
5. Manual validation with multiple auth strategies succeeds
6. Portal custom domains with portal_id references work correctly

## 9. Future Considerations

This implementation creates a pattern for handling array references that can be applied to:
- Other resources with array reference fields
- Bulk operations that need multiple reference resolutions
- Complex nested reference structures

The solution is designed to be extensible and maintainable for future enhancements.