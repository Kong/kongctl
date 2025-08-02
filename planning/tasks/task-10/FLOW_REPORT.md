# API Publication Reference Resolution Flow Analysis

## Executive Summary

This flow analysis traces the complete data flow for API publication creation in kongctl, focusing on how `auth_strategy_ids` references fail to be resolved to UUIDs. The analysis reveals that the reference resolution mechanism is incomplete at multiple levels: the planner doesn't set up references for array fields, the reference resolver can't extract array references, and the new adapter expects pre-resolved references that never arrive.

## 1. API Publication Planning Flow

### 1.1 Configuration Parsing
**File**: User's YAML configuration
```yaml
publications:
  - ref: securebank-public-publication
    portal_id: securebank-portal
    auth_strategy_ids:
      - securebank-oauth2-strategy  # Reference string, not UUID
      - securebank-apikey-strategy   # Reference string, not UUID
```

### 1.2 Resource Definition
**File**: `internal/declarative/resources/api_publication.go`
- Lines 51-56: Defines reference field mappings
  ```go
  func (p APIPublicationResource) GetReferenceFieldMappings() map[string]string {
      return map[string]string{
          "portal_id":         "portal",
          "auth_strategy_ids": "application_auth_strategy",
      }
  }
  ```
- Line 137: `AuthStrategyIDs []string` field captures the array from config

### 1.3 Plan Generation
**File**: `internal/declarative/planner/api_planner.go`
- Function: `planAPIPublicationCreate` (lines 798-878)

#### Data Flow:
1. **Input**: `publication resources.APIPublicationResource` with `AuthStrategyIds` containing reference strings
2. **Field Setup** (lines 801-818):
   ```go
   fields["auth_strategy_ids"] = publication.AuthStrategyIds  // Array passed as-is!
   ```
3. **Reference Setup** (lines 846-875):
   - ✅ Portal reference properly set up (lines 868-875)
   - ✅ API reference properly set up (lines 858-865)
   - ❌ **NO auth_strategy_ids reference setup!**

#### Result:
```go
PlannedChange{
    Fields: {
        "auth_strategy_ids": ["securebank-oauth2-strategy", "securebank-apikey-strategy"], // Raw refs!
        "portal_id": "securebank-portal"
    },
    References: {
        "portal_id": ReferenceInfo{Ref: "securebank-portal", LookupFields: {"name": "..."}},
        "api_id": ReferenceInfo{Ref: "...", ID: "..."},
        // Missing: "auth_strategy_ids" reference!
    }
}
```

### 1.4 Reference Resolution During Planning
**File**: `internal/declarative/planner/resolver.go`
- Function: `extractReference` (lines 95-115)

#### Limitations:
1. Only handles single string values:
   ```go
   switch v := value.(type) {
   case string:
       if !isUUID(v) {
           return v, true
       }
   // No case for []string or []interface{}!
   ```
2. Returns `false` for array values, so auth_strategy_ids are never extracted
3. Even though "auth_strategy_ids" is listed as a reference field (line 124)

## 2. API Publication Apply/Execution Flow

### 2.1 Executor Entry Point
**File**: `internal/declarative/executor/executor.go`
- Function: `executeCreate` (lines 648-696 for api_publication case)

#### Reference Resolution (lines 674-693):
- ✅ Resolves api_id reference if needed
- ✅ Resolves portal_id reference if needed
- ❌ **NO auth_strategy_ids resolution!** (not in References map)

### 2.2 New Adapter Implementation (FAILING)
**File**: `internal/declarative/executor/api_publication_adapter.go`
- Function: `MapCreateFields` (lines 30-63)

#### Data Flow:
1. **Check References** (lines 36-44):
   ```go
   if authStrategyRefs, ok := change.References["auth_strategy_ids"]; ok {
       // Never enters here - no reference was set up!
   }
   ```
2. **Fallback to Fields** (lines 48-50):
   ```go
   else if authStrategyIDsList, ok := fields["auth_strategy_ids"].([]string); ok {
       create.AuthStrategyIds = authStrategyIDsList  // Still reference strings!
   }
   ```
3. **Result**: Sends reference strings to API, causing UUID format error

### 2.3 Deprecated Implementation (WORKING)
**File**: `internal/declarative/executor/api_publication_operations.go`
- Function: `createAPIPublication` (lines 66-102)

#### Key Difference - Runtime Resolution:
```go
for _, strID := range authStrategyIDs {
    if isUUID(strID) {
        ids = append(ids, strID)
    } else {
        // Resolves at execution time!
        resolvedID, err := e.resolveAuthStrategyRef(ctx, strID)
        if err != nil {
            return "", fmt.Errorf("failed to resolve auth strategy reference %q: %w", strID, err)
        }
        ids = append(ids, resolvedID)
    }
}
```

### 2.4 Auth Strategy Resolution
**File**: `internal/declarative/executor/executor.go`
- Function: `resolveAuthStrategyRef` (lines ~960-978)

#### Resolution Process:
1. Check if created in current execution via `e.refToID` map
2. If not found, lookup via `e.client.GetAuthStrategyByName(ctx, ref)`
3. Return the UUID

## 3. Reference Resolution Mechanism

### 3.1 Current State
- **Single References**: Working (portal_id, api_id)
- **Array References**: Not implemented (auth_strategy_ids)

### 3.2 Working Example: Portal ID Resolution
1. **Planning**: Sets up reference in `change.References["portal_id"]`
2. **Execution**: Executor resolves before calling adapter
3. **Adapter**: Receives pre-resolved ID from `change.References["portal_id"].ID`

### 3.3 Broken Flow: Auth Strategy IDs
1. **Planning**: No reference setup, array passed in fields
2. **Execution**: No resolution attempt (not in References)
3. **Adapter**: Receives raw reference strings, sends to API
4. **API**: Rejects with "must match format uuid" error

## 4. Portal Custom Domain Flow

**File**: `internal/declarative/executor/portal_domain_adapter.go`
- Function: `getPortalID` (lines 121-136)

Similar pattern but simpler:
- Single portal_id reference (not array)
- Relies on pre-resolution by executor
- Would fail if portal_id wasn't resolved (line 135 error)

## 5. Data Transformation Summary

### auth_strategy_ids Field Journey:
1. **Config**: `["securebank-oauth2-strategy", "securebank-apikey-strategy"]`
2. **Resource**: Stored as `AuthStrategyIds []string`
3. **Planner**: Passed to `Fields["auth_strategy_ids"]` without reference setup
4. **Resolver**: Skipped (can't handle arrays)
5. **Executor**: No resolution (not in References map)
6. **Adapter**: Passes raw refs to `create.AuthStrategyIds`
7. **API Call**: Fails with UUID format error

### Expected Flow (per deprecated implementation):
1. **Config**: `["securebank-oauth2-strategy", "securebank-apikey-strategy"]`
2. **Resource**: Stored as `AuthStrategyIds []string`
3. **Planner**: Should set up array references
4. **Resolver**: Should handle array extraction
5. **Executor**: Should resolve each ref to UUID
6. **Adapter**: Should receive `["uuid-1234...", "uuid-5678..."]`
7. **API Call**: Success

## 6. Root Cause Analysis

The issue stems from three interconnected problems:

1. **Planner Gap**: `planAPIPublicationCreate` doesn't set up references for `auth_strategy_ids`
2. **Resolver Limitation**: `extractReference` can't handle array values
3. **Adapter Assumption**: New adapter expects pre-resolved references but doesn't handle resolution

The deprecated implementation worked because it resolved references at execution time within the operation itself, bypassing the need for planner/resolver support.

## 7. Solution Path

To fix this issue, one of these approaches is needed:

1. **Fix Planning & Resolution** (Recommended):
   - Enhance planner to set up array references
   - Update resolver to extract array references
   - Modify reference resolution to handle arrays

2. **Adapter-Level Resolution**:
   - Add resolution logic to APIPublicationAdapter
   - Similar to deprecated implementation
   - Less architecturally clean but simpler

3. **Pre-process Arrays**:
   - Convert array refs to resolved IDs during planning
   - Store as comma-separated string
   - Requires careful handling of dependencies