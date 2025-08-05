# API Document Flow Analysis Report

## Executive Summary

This report maps the complete code flow for api_document operations in the kongctl project, focusing on issue #36 where api_documents are being created instead of updated. The analysis reveals that the root cause is **identical to the recently fixed issue #34 (portal pages)** - child resource adapters missing the `GetByID()` method required by the BaseExecutor's validation fallback mechanism.

**Key Finding**: The planner correctly identifies that api_documents should be UPDATED, but the executor's validation fails during UPDATE operations, resulting in the error "api_document no longer exists" rather than falling back to CREATE.

## Complete Execution Flow

### Phase 1: Configuration to PlannedChange (WORKS CORRECTLY)

```
Configuration File (YAML)
    ↓
Loader.Load()
    ↓ 
APIResource with APIDocumentResource children
    ↓
Planner.planAPIChanges()
    ↓
Planner.planAPIDocumentChanges() [api_planner.go:1240-1309]
```

**Flow Details:**

1. **Document Discovery** (lines 1245-1256):
   ```go
   // List current documents
   currentDocuments, err := p.client.ListAPIDocuments(ctx, apiID)
   
   // Index current documents by slug
   currentBySlug := make(map[string]state.APIDocument)
   for _, d := range currentDocuments {
       normalizedSlug := strings.TrimPrefix(d.Slug, "/")
       currentBySlug[normalizedSlug] = d
   }
   ```

2. **UPDATE Decision Logic** (lines 1267-1287):
   ```go
   current, exists := currentBySlug[normalizedSlug]
   
   if !exists {
       // CREATE
       p.planAPIDocumentCreate(parentNamespace, apiRef, apiID, desiredDoc, []string{}, plan)
   } else {
       // UPDATE - documents support update
       // Fetch full document to get content for comparison
       fullDoc, err := p.client.GetAPIDocument(ctx, apiID, current.ID)
       if fullDoc != nil {
           current = *fullDoc
       }
       
       // Now compare with full content
       if p.shouldUpdateAPIDocument(current, desiredDoc) {
           p.planAPIDocumentUpdate(parentNamespace, apiRef, apiID, current.ID, desiredDoc, plan)
       }
   }
   ```

3. **PlannedChange Creation** (lines 1409-1443):
   ```go
   change := PlannedChange{
       ID:           p.nextChangeID(ActionUpdate, "api_document", document.GetRef()),
       ResourceType: "api_document",
       ResourceRef:  document.GetRef(),
       ResourceID:   documentID,  // ← CRITICAL: Correctly set for UPDATE
       Parent:       &ParentInfo{Ref: apiRef, ID: apiID},
       Action:       ActionUpdate,  // ← CRITICAL: Correctly identified as UPDATE
       Fields:       fields,
       References: map[string]ReferenceInfo{
           "api_id": {
               Ref: apiRef,
               ID:  apiID,  // ← Dependency relationship properly established
               LookupFields: map[string]string{"name": apiName},
           },
       },
   }
   ```

**Result**: PlannedChange correctly created with `Action: ActionUpdate` and proper `ResourceID`.

### Phase 2: Execution Flow (FAILS AT VALIDATION)

```
Executor.Execute()
    ↓
executeChange() [executor.go:174]
    ↓
updateResource() [executor.go:842]
    ↓
apiDocumentExecutor.Update() [BaseExecutor.Update()]
    ↓
validateResourceForUpdate() [base_executor.go:204] ← FAILS HERE
    ↓
ERROR: "api_document no longer exists"
```

**Detailed Execution Flow:**

1. **Executor Routing** (executor.go:244-254):
   ```go
   switch change.Action {
   case planner.ActionCreate:
       resourceID, err = e.createResource(ctx, change)
   case planner.ActionUpdate:  // ← Correctly routed here
       resourceID, err = e.updateResource(ctx, change)
   case planner.ActionDelete:
       err = e.deleteResource(ctx, change)
   }
   ```

2. **Resource Type Routing** (executor.go:853-864):
   ```go
   case "api_document":
       // First resolve API reference if needed
       if apiRef, ok := change.References["api_id"]; ok && apiRef.ID == "" {
           apiID, err := e.resolveAPIRef(ctx, apiRef)
           // Update the reference with the resolved ID
           apiRef.ID = apiID
           change.References["api_id"] = apiRef
       }
       return e.apiDocumentExecutor.Update(ctx, *change)  // ← Routes here
   ```

3. **BaseExecutor Update** (base_executor.go:104-162):
   ```go
   func (b *BaseExecutor[TCreate, TUpdate]) Update(ctx context.Context, change planner.PlannedChange) (string, error) {
       // First, validate protection status at execution time
       resource, err := b.validateResourceForUpdate(ctx, resourceName, change)  // ← FAILS HERE
       if err != nil {
           return "", fmt.Errorf("failed to validate %s for update: %w", b.ops.ResourceType(), err)
       }
       if resource == nil {
           return "", fmt.Errorf("%s no longer exists", b.ops.ResourceType())  // ← ERROR MESSAGE
       }
   }
   ```

### Phase 3: Validation Failure Analysis

**validateResourceForUpdate() Three-Strategy Approach** (base_executor.go:204-249):

```go
func (b *BaseExecutor[TCreate, TUpdate]) validateResourceForUpdate(
    ctx context.Context, resourceName string, change planner.PlannedChange,
) (ResourceInfo, error) {
    
    // Strategy 1: Standard name-based lookup
    resource, err := b.ops.GetByName(ctx, resourceName)  // ← APIDocumentAdapter returns (nil, nil)
    if err == nil && resource != nil {
        return resource, nil
    }
    
    // Strategy 2: Try ID-based lookup if available (useful for child resources)
    if change.ResourceID != "" {
        if idLookup, ok := b.ops.(interface{ GetByID(context.Context, string) (ResourceInfo, error) }); ok {
            resource, err := idLookup.GetByID(ctx, change.ResourceID)  // ← APIDocumentAdapter MISSING this method
            if err == nil && resource != nil {
                return resource, nil
            }
        }
    }
    
    // Strategy 3: Namespace-specific lookup (not applicable here)
    
    // Return original result if all fallback strategies fail
    return b.ops.GetByName(ctx, resourceName)  // ← Returns (nil, nil), validation fails
}
```

**APIDocumentAdapter GetByName Implementation** (api_document_adapter.go:122-126):
```go
func (a *APIDocumentAdapter) GetByName(_ context.Context, _ string) (ResourceInfo, error) {
    // API documents don't have a direct "get by name" method
    // The planner handles this by searching through the list
    return nil, nil  // ← PROBLEM: Returns nil, validation fails
}
```

**Missing GetByID Implementation**: APIDocumentAdapter does not implement the `GetByID(context.Context, string) (ResourceInfo, error)` method that Strategy 2 requires.

## Dependency Relationship Analysis

### Hierarchical Resource Structure

```
API (top-level resource)
├── APIDocument (child resource) ← Issue #36
├── APIVersion (child resource)  ← Same issue
├── APIPublication (child resource) ← Same issue
└── APIImplementation (child resource) ← Same issue

Portal (top-level resource)
├── PortalPage (child resource) ← FIXED in issue #34
├── PortalSnippet (child resource) ← Same issue
├── PortalDomain (child resource) ← Same issue
└── PortalCustomization (child resource)
```

### Dependency Resolution Flow

1. **Context Propagation** (executor.go:844-846):
   ```go
   ctx = context.WithValue(ctx, contextKeyNamespace, change.Namespace)
   ctx = context.WithValue(ctx, contextKeyProtection, change.Protection)
   ctx = context.WithValue(ctx, contextKeyPlannedChange, *change)
   ```

2. **API ID Extraction** (api_document_adapter.go:144-159):
   ```go
   func (a *APIDocumentAdapter) getAPIID(ctx context.Context) (string, error) {
       // Get the planned change from context
       change, ok := ctx.Value(contextKeyPlannedChange).(planner.PlannedChange)
       
       // Get API ID from references
       if apiRef, ok := change.References["api_id"]; ok {
           if apiRef.ID != "" {
               return apiRef.ID, nil  // ← Successfully extracts API ID
           }
       }
   }
   ```

3. **Reference Resolution**: The dependency relationship is properly established through the References map, allowing child resources to access their parent IDs.

## Comparison with Portal Page Fix (Issue #34)

### Portal Page Solution Pattern

**PortalPageAdapter GetByID Implementation** (portal_page_adapter.go:194-212):
```go
func (p *PortalPageAdapter) GetByID(ctx context.Context, id string) (ResourceInfo, error) {
    // Get portal ID from context using existing pattern
    portalID, err := p.getPortalID(ctx)
    if err != nil {
        return nil, fmt.Errorf("failed to get portal ID for page lookup: %w", err)
    }
    
    // Use existing client method
    page, err := p.client.GetPortalPage(ctx, portalID, id)
    if err != nil {
        return nil, fmt.Errorf("failed to get portal page: %w", err)
    }
    if page == nil {
        return nil, nil
    }
    
    return &PortalPageResourceInfo{page: page}, nil
}
```

### Required API Document Solution

**APIDocumentAdapter GetByID Implementation** (MISSING - needs to be implemented):
```go
func (a *APIDocumentAdapter) GetByID(ctx context.Context, id string) (ResourceInfo, error) {
    // Get API ID from context using existing pattern
    apiID, err := a.getAPIID(ctx)
    if err != nil {
        return nil, fmt.Errorf("failed to get API ID for document lookup: %w", err)
    }
    
    // Use existing client method
    document, err := a.client.GetAPIDocument(ctx, apiID, id)
    if err != nil {
        return nil, fmt.Errorf("failed to get API document: %w", err)
    }
    if document == nil {
        return nil, nil
    }
    
    return &APIDocumentResourceInfo{document: document}, nil
}
```

### Infrastructure Availability

**All Required Components Already Exist:**

1. **State Client Method** (state/client.go:1070):
   ```go
   func (c *Client) GetAPIDocument(ctx context.Context, apiID, documentID string) (*APIDocument, error)
   ```

2. **Context Extraction Method** (api_document_adapter.go:144-159):
   ```go
   func (a *APIDocumentAdapter) getAPIID(ctx context.Context) (string, error)
   ```

3. **Resource Info Wrapper** (api_document_adapter.go:162-181):
   ```go
   type APIDocumentResourceInfo struct {
       document *state.APIDocument
   }
   ```

## Affected Resources Summary

### Child Resources Missing GetByID() (AFFECTED)

| Resource | Adapter | GetByName() Returns | GetByID() Exists | Status |
|----------|---------|-------------------|------------------|--------|
| api_document | APIDocumentAdapter | `(nil, nil)` | ❌ No | **Issue #36** |
| portal_snippet | PortalSnippetAdapter | `(nil, nil)` | ❌ No | Likely affected |
| portal_domain | PortalDomainAdapter | `(nil, nil)` | ❌ No | Likely affected |
| api_version | APIVersionAdapter | `(nil, nil)` | ❌ No | Likely affected |
| api_publication | APIPublicationAdapter | `(nil, nil)` | ❌ No | Likely affected |

### Child Resources with GetByID() (FIXED)

| Resource | Adapter | GetByName() Returns | GetByID() Exists | Status |
|----------|---------|-------------------|------------------|--------|
| portal_page | PortalPageAdapter | `(nil, nil)` | ✅ Yes | **Fixed in #34** |

### Top-Level Resources (NOT AFFECTED)

| Resource | Adapter | GetByName() Returns | GetByID() Needed | Status |
|----------|---------|-------------------|------------------|--------|
| api | APIAdapter | Proper implementation | ❌ No | Works correctly |
| portal | PortalAdapter | Proper implementation | ❌ No | Works correctly |
| auth_strategy | AuthStrategyAdapter | Proper implementation | ❌ No | Works correctly |

## Key Decision Points

### 1. Planning Phase Decision: CREATE vs UPDATE

**Location**: `api_planner.go:1267-1287`

**Decision Logic**:
```go
current, exists := currentBySlug[normalizedSlug]

if !exists {
    // CREATE - Document doesn't exist
    p.planAPIDocumentCreate(...)
} else {
    // UPDATE - Document exists and needs updating
    if p.shouldUpdateAPIDocument(current, desiredDoc) {
        p.planAPIDocumentUpdate(...) // ← CORRECTLY CHOSEN
    }
}
```

**Result**: ✅ **CORRECT** - Planner correctly identifies UPDATE is needed.

### 2. Execution Phase Decision: Validation Success/Failure

**Location**: `base_executor.go:117-123`

**Decision Logic**:
```go
resource, err := b.validateResourceForUpdate(ctx, resourceName, change)
if err != nil {
    return "", fmt.Errorf("failed to validate %s for update: %w", b.ops.ResourceType(), err)
}
if resource == nil {
    return "", fmt.Errorf("%s no longer exists", b.ops.ResourceType()) // ← FAILS HERE
}
```

**Result**: ❌ **INCORRECT** - Executor validation fails even though resource exists.

### 3. Error Handling: UPDATE Failure Consequences

**Location**: `executor.go:257-267`

**Decision Logic**:
```go
if err != nil {
    execError := ExecutionError{
        ChangeID:     change.ID,
        ResourceType: change.ResourceType,
        Action:       string(change.Action), // Still "UPDATE"
        Error:        err.Error(), // "api_document no longer exists"
    }
    result.Errors = append(result.Errors, execError)
    result.FailureCount++
}
```

**Result**: ❌ **FAILS** - No fallback to CREATE, operation fails with error.

## Sequence Diagrams

### Current Flow (Failing)

```
User -> Planner: Apply configuration
Planner -> StateClient: ListAPIDocuments(apiID)
StateClient -> Planner: [existing documents]
Planner -> Planner: Compare & identify UPDATE needed
Planner -> Plan: Add UPDATE change with ResourceID

User -> Executor: Execute plan
Executor -> BaseExecutor: Update(change)
BaseExecutor -> APIDocumentAdapter: GetByName(resourceName)
APIDocumentAdapter -> BaseExecutor: (nil, nil)
BaseExecutor -> APIDocumentAdapter: GetByID(resourceID) [INTERFACE CHECK]
Note: APIDocumentAdapter does not implement GetByID()
BaseExecutor -> BaseExecutor: All strategies failed
BaseExecutor -> Executor: Error("api_document no longer exists")
Executor -> User: 409 Resource Conflict
```

### Fixed Flow (With GetByID Implementation)

```
User -> Planner: Apply configuration
Planner -> StateClient: ListAPIDocuments(apiID)
StateClient -> Planner: [existing documents]
Planner -> Planner: Compare & identify UPDATE needed
Planner -> Plan: Add UPDATE change with ResourceID

User -> Executor: Execute plan
Executor -> BaseExecutor: Update(change)
BaseExecutor -> APIDocumentAdapter: GetByName(resourceName)
APIDocumentAdapter -> BaseExecutor: (nil, nil)
BaseExecutor -> APIDocumentAdapter: GetByID(resourceID)
APIDocumentAdapter -> StateClient: GetAPIDocument(apiID, resourceID)
StateClient -> APIDocumentAdapter: APIDocument
APIDocumentAdapter -> BaseExecutor: APIDocumentResourceInfo
BaseExecutor -> BaseExecutor: Validation passed
BaseExecutor -> APIDocumentAdapter: Update(resourceID, request)
APIDocumentAdapter -> StateClient: UpdateAPIDocument(apiID, resourceID, request)
StateClient -> APIDocumentAdapter: Success
APIDocumentAdapter -> BaseExecutor: resourceID
BaseExecutor -> Executor: resourceID
Executor -> User: Success
```

## Code Snippets: Problem Areas

### 1. Missing GetByID() Method

**File**: `internal/declarative/executor/api_document_adapter.go`

**Problem**: Missing method implementation
```go
// MISSING: GetByID method should be implemented here
// func (a *APIDocumentAdapter) GetByID(ctx context.Context, id string) (ResourceInfo, error)
```

### 2. Validation Fallback Strategy

**File**: `internal/declarative/executor/base_executor.go:215-227`

**Problematic Code**:
```go
// Strategy 2: Try ID-based lookup if available (useful for child resources)
if change.ResourceID != "" {
    if idLookup, ok := b.ops.(interface{ GetByID(context.Context, string) (ResourceInfo, error) }); ok {
        resource, err := idLookup.GetByID(ctx, change.ResourceID)
        if err == nil && resource != nil {
            logger.Debug("Resource found via ID lookup", ...)
            return resource, nil
        }
    }
}
```

**Issue**: Interface check `ok` returns `false` for APIDocumentAdapter because it doesn't implement GetByID().

### 3. GetByName() Implementation

**File**: `internal/declarative/executor/api_document_adapter.go:122-126`

**Problematic Code**:
```go
func (a *APIDocumentAdapter) GetByName(_ context.Context, _ string) (ResourceInfo, error) {
    // API documents don't have a direct "get by name" method
    // The planner handles this by searching through the list
    return nil, nil  // ← This causes validation to fail
}
```

**Issue**: Returns `(nil, nil)` because API documents can't be looked up by name alone (need API ID context).

## Solution Implementation

### Immediate Fix (High Priority)

**Implement APIDocumentAdapter.GetByID()** following the PortalPageAdapter pattern:

```go
// GetByID gets an API document by ID using API context
func (a *APIDocumentAdapter) GetByID(ctx context.Context, id string) (ResourceInfo, error) {
    // Get API ID from context using existing pattern
    apiID, err := a.getAPIID(ctx)
    if err != nil {
        return nil, fmt.Errorf("failed to get API ID for document lookup: %w", err)
    }
    
    // Use existing client method
    document, err := a.client.GetAPIDocument(ctx, apiID, id)
    if err != nil {
        return nil, fmt.Errorf("failed to get API document: %w", err)
    }
    if document == nil {
        return nil, nil
    }
    
    return &APIDocumentResourceInfo{document: document}, nil
}
```

### Follow-up Fixes (Medium Priority)

Implement GetByID() methods for other affected child resource adapters:

1. **PortalSnippetAdapter.GetByID()**
2. **PortalDomainAdapter.GetByID()**
3. **APIVersionAdapter.GetByID()**
4. **APIPublicationAdapter.GetByID()**

### Testing Strategy

1. **Integration Test Scenario**:
   ```yaml
   # 1. Create initial state
   apis:
     - name: test-api
       documents:
         - title: "Initial Doc"
           content: "Initial content"
           slug: "test-doc"
   ```

   ```yaml  
   # 2. Modify document content
   apis:
     - name: test-api
       documents:
         - title: "Updated Doc"
           content: "Updated content"  # Changed
           slug: "test-doc"
   ```

   ```bash
   # 3. Apply should succeed with UPDATE
   kongctl apply -f modified-config.yaml
   ```

2. **Verification Points**:
   - Plan shows UPDATE action (not CREATE)
   - Execution succeeds without "no longer exists" error
   - Document content is actually updated
   - No duplicate documents are created

## Impact Assessment

### Currently Broken Operations

- ✅ **Planning**: Works correctly, identifies UPDATE needed
- ❌ **UPDATE Execution**: Fails with "api_document no longer exists"
- ✅ **CREATE Execution**: Works (no validation needed)
- ✅ **DELETE Execution**: Works (different validation logic)

### Working Operations

- **Top-level resource updates**: APIs, portals, auth strategies
- **Portal page updates**: Fixed in issue #34
- **All CREATE operations**: No validation required
- **All DELETE operations**: Use different validation approach

### Risk Assessment

- **Risk Level**: Low - surgical fix following established pattern
- **Blast Radius**: Only affects UPDATE operations for child resources
- **Rollback**: Simple - revert GetByID() method additions
- **Dependencies**: None - uses existing infrastructure

## Conclusion

Issue #36 is a **direct continuation of issue #34** with identical root cause and solution pattern. The problem is architectural: child resource adapters missing GetByID() methods that the BaseExecutor's validation fallback mechanism requires.

**The fix is well-established and low-risk**:
1. Planning phase works correctly (identifies UPDATE needed)
2. Infrastructure exists (client methods, context extraction, resource wrappers)
3. Pattern established (PortalPageAdapter GetByID() implementation)
4. Solution surgical (add one method per affected adapter)

The implementation should follow the PortalPageAdapter.GetByID() method as a direct template, with appropriate substitutions for API-specific client calls and context extraction methods.

## Key Files Referenced

### Core Issue Files
- `/Users/rick.spurgeon@konghq.com/go/src/github.com/Kong/kongctl-portal-page-bug/internal/declarative/executor/api_document_adapter.go` - Missing GetByID()
- `/Users/rick.spurgeon@konghq.com/go/src/github.com/Kong/kongctl-portal-page-bug/internal/declarative/executor/base_executor.go` - Validation logic with fallback

### Reference Implementation  
- `/Users/rick.spurgeon@konghq.com/go/src/github.com/Kong/kongctl-portal-page-bug/internal/declarative/executor/portal_page_adapter.go` - GetByID() pattern

### Supporting Infrastructure
- `/Users/rick.spurgeon@konghq.com/go/src/github.com/Kong/kongctl-portal-page-bug/internal/declarative/state/client.go` - Client methods
- `/Users/rick.spurgeon@konghq.com/go/src/github.com/Kong/kongctl-portal-page-bug/internal/declarative/planner/api_planner.go` - Planning logic
- `/Users/rick.spurgeon@konghq.com/go/src/github.com/Kong/kongctl-portal-page-bug/internal/declarative/executor/executor.go` - Execution routing

### Other Affected Adapters
- `/Users/rick.spurgeon@konghq.com/go/src/github.com/Kong/kongctl-portal-page-bug/internal/declarative/executor/portal_snippet_adapter.go`
- `/Users/rick.spurgeon@konghq.com/go/src/github.com/Kong/kongctl-portal-page-bug/internal/declarative/executor/portal_domain_adapter.go`
- `/Users/rick.spurgeon@konghq.com/go/src/github.com/Kong/kongctl-portal-page-bug/internal/declarative/executor/api_version_adapter.go`
- `/Users/rick.spurgeon@konghq.com/go/src/github.com/Kong/kongctl-portal-page-bug/internal/declarative/executor/api_publication_adapter.go`