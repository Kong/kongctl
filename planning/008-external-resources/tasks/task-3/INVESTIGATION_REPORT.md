# Investigation Report: Step 2 - Resource Type Registry

**Date**: 2025-08-07  
**Task**: Step 2: Resource Type Registry  
**Context**: 008-external-resources planning stage  
**Status**: Step 1 (Schema and Configuration) completed, ready for Step 2 implementation  

## Executive Summary

Investigation completed for Step 2 (Resource Type Registry) implementation. Step 1 foundation is solid with comprehensive schema, validation, and registry structure already implemented using "Resolution" naming theme. Step 2 requires implementing SDK operation mappings through ResolutionAdapter concrete implementations and integrating with existing state client patterns.

## Current Implementation Analysis (Step 1 Complete)

### External Resources Schema Foundation ✅

**Location**: `/internal/declarative/resources/external_resource.go`

**Key Components**:
- `ExternalResourceResource` struct with Ref, ResourceType, ID/Selector XOR validation
- `ExternalResourceSelector` with MatchFields for field-based matching
- `ExternalResourceParent` for hierarchical resource relationships
- Runtime state management (resolvedID, resolvedResource, resolved flag)
- Comprehensive validation using `ValidateResourceType`, `ValidateIDXORSelector`, `ValidateSelector`, `ValidateParent`

**Integration Point**: Already integrated in `ResourceSet.ExternalResources` in `/internal/declarative/resources/types.go`

### Resolution Registry Foundation ✅

**Location**: `/internal/declarative/external/`

**Key Files**:
- `types.go`: Core interfaces and types
- `registry.go`: Singleton registry with comprehensive resource type definitions

**Registry Structure**:
- `ResolutionMetadata`: Name, SelectorFields, SupportedParents, SupportedChildren, ResolutionAdapter
- `ResolutionRegistry`: Thread-safe singleton with built-in resource types
- `ResolutionAdapter` interface: `GetByID()` and `GetBySelector()` methods

**Built-in Resource Types** (all with ResolutionAdapter: nil):
- `portal`: Top-level, supports portal_customization, portal_custom_domain, portal_page, portal_snippet children
- `api`: Top-level, supports api_version, api_publication, api_implementation, api_document children  
- `control_plane`: Top-level, no children currently
- `application_auth_strategy`: Top-level, no children
- All child resource types with proper parent relationships defined

## Step 2 Implementation Requirements

### 1. ResolutionAdapter Implementations

**What's Missing**: Concrete implementations of ResolutionAdapter interface for each resource type.

**Required Adapters**:
- `PortalResolutionAdapter`
- `APIResolutionAdapter` 
- `ControlPlaneResolutionAdapter`
- `ApplicationAuthStrategyResolutionAdapter`
- `APIVersionResolutionAdapter`
- `APIPublicationResolutionAdapter`
- `APIImplementationResolutionAdapter`
- `APIDocumentResolutionAdapter`
- `PortalCustomizationResolutionAdapter`
- `PortalCustomDomainResolutionAdapter`
- `PortalPageResolutionAdapter`
- `PortalSnippetResolutionAdapter`

**Interface Requirements**:
```go
type ResolutionAdapter interface {
    GetByID(ctx context.Context, id string, parent *ResolvedParent) (interface{}, error)
    GetBySelector(ctx context.Context, selector map[string]string, parent *ResolvedParent) ([]interface{}, error)
}
```

### 2. SDK Operations Mapping

**Analysis of Existing Patterns**: 
- **Location**: `/internal/declarative/state/client.go`
- **Pattern**: State client wraps SDK APIs (PortalAPI, APIAPI, AppAuthAPI, etc.)
- **Methods**: `GetPortalByName`, `GetAPIByName` exist for name-based lookup
- **Client Structure**: Injected API interfaces from helpers package

**Required SDK Integration**:
- Extend state client with methods for external resource resolution
- Add `GetByID` methods for each resource type
- Add selector-based filtering for `GetBySelector` functionality
- Handle parent-child relationships in API calls

### 3. Parent-Child Relationship Implementation

**Current State**: Registry defines relationships but no runtime implementation

**Required Implementation**:
- Parent resolution before child resolution in dependency order
- `ResolvedParent` context passing to child resolution adapters
- Parent ID injection into child resource API calls (e.g., portal_pages require portal_id)

**Parent-Child Mappings**:
- `portal` → `portal_customization`, `portal_custom_domain`, `portal_page`, `portal_snippet`
- `api` → `api_version`, `api_publication`, `api_implementation`, `api_document`

### 4. Resource Type Validation Integration

**Current State**: Registry validation exists via `ValidateResourceType` 

**Validation Integration Points**:
- Configuration loading validation (already integrated)
- Runtime resolution validation  
- Parent-child relationship validation (already implemented)
- Selector field validation (already implemented)

## Existing Integration Points

### 1. State Client Pattern

**Location**: `/internal/declarative/state/client.go`

**Current Structure**:
- ClientConfig with API interface injection
- Client with wrapped SDK operations
- Normalized resource types (Portal, API, etc.)
- Helper methods like `GetPortalByName`, `GetAPIByName`

**Integration Strategy**: Extend client with resolution methods or create separate resolution client

### 2. Executor Pattern

**Location**: `/internal/declarative/executor/`

**Pattern Analysis**:
- Resource adapters implement `ResourceOperations` interface
- Adapters use state client for SDK operations  
- Field mapping and CRUD operations
- Example: `PortalAdapter.GetByName()` calls `client.GetPortalByName()`

**Integration Strategy**: ResolutionAdapters can follow similar pattern using state client

### 3. Helper API Interfaces

**Location**: `/internal/konnect/helpers/`

**Current APIs**:
- `PortalAPI`, `APIAPI`, `AppAuthStrategiesAPI`
- `PortalPageAPI`, `PortalCustomizationAPI`, etc.

**Integration Strategy**: ResolutionAdapters should use same API interfaces for consistency

## Configuration Integration

### Current Integration ✅

**Location**: `/internal/declarative/resources/types.go`

```go
type ResourceSet struct {
    ExternalResources []ExternalResourceResource `yaml:"external_resources,omitempty"` 
    // ... other resources
}
```

**YAML Structure**:
```yaml
external_resources:
  - ref: "existing-portal"
    resource_type: "portal"
    selector:
      match_fields:
        name: "Developer Portal"
  - ref: "api-v1"  
    resource_type: "api_version"
    parent:
      resource_type: "api"
      id: "api-123"
    id: "version-456"
```

## Implementation Architecture

### 1. Resolution Adapter Factory

**Suggested Location**: `/internal/declarative/external/adapters/`

**Structure**:
- `factory.go`: Creates adapters with state client injection
- Individual adapter files: `portal_resolution_adapter.go`, etc.
- Common base adapter with shared functionality

### 2. State Client Extension

**Options**:
1. **Extend existing client**: Add resolution methods to `/internal/declarative/state/client.go`  
2. **Separate resolution client**: Create `/internal/declarative/external/resolution_client.go`

**Recommendation**: Extend existing client for consistency with executor patterns

### 3. SDK Method Requirements

**For GetByID**: Direct SDK calls using resource ID
- `portalAPI.GetPortal(id)`
- `apiAPI.GetAPI(id)`
- Child resources need parent context: `portalPageAPI.GetPortalPage(portalId, pageId)`

**For GetBySelector**: List all + filter by selector fields
- `portalAPI.ListPortals()` → filter by name, description
- `apiAPI.ListAPIs()` → filter by name, description  
- Pagination handling for large result sets

## Risk Assessment

### Low Risk Items ✅
- Registry structure is solid and extensible
- Validation framework is comprehensive
- Configuration integration is complete
- Parent-child relationships are well-defined

### Medium Risk Items ⚠️
- SDK API method discovery for all resource types
- Selector filtering performance on large resource sets
- Parent context propagation in hierarchical resolution

### High Risk Items 🚨  
- None identified - foundation is solid

## Recommended Implementation Steps

### Step 2a: State Client Extension
- Add resolution methods to state client
- Implement direct ID lookups for all resource types
- Add list-and-filter methods for selector-based resolution

### Step 2b: Resolution Adapter Implementation  
- Create adapter factory with state client injection
- Implement concrete adapters for each resource type
- Handle parent context in child resource adapters

### Step 2c: Registry Integration
- Update registry initialization to inject adapters
- Test adapter registration and retrieval
- Validate parent-child relationship handling

### Step 2d: Testing and Validation
- Unit tests for each resolution adapter
- Integration tests with mock SDK responses
- Parent-child resolution testing
- Selector field validation testing

## Files Requiring Modification

### New Files (Step 2):
- `/internal/declarative/external/adapters/factory.go`
- `/internal/declarative/external/adapters/*_resolution_adapter.go` (12 files)
- `/internal/declarative/external/adapters/base_adapter.go`

### Modified Files (Step 2):
- `/internal/declarative/external/registry.go` (inject adapters)
- `/internal/declarative/state/client.go` (add resolution methods)
- `/internal/declarative/external/registry_test.go` (test adapter integration)

## Dependencies

### Internal Dependencies ✅
- State client and SDK wrapper patterns established
- Helper API interfaces available  
- Resource validation framework complete
- Configuration loading integration ready

### External Dependencies ✅
- Konnect Go SDK available
- All required API interfaces exist in helpers package
- Registry singleton pattern established

## Success Criteria for Step 2

- [ ] All 12 resource types have working ResolutionAdapter implementations
- [ ] Registry properly injects adapters during initialization  
- [ ] GetByID works for all resource types with direct SDK calls
- [ ] GetBySelector works with field-based filtering
- [ ] Parent-child resolution propagates context correctly
- [ ] Resource type validation integration maintains existing behavior
- [ ] Unit tests cover all adapter implementations
- [ ] Integration tests validate SDK interaction

## Conclusion

Step 1 foundation is exceptionally well-implemented with the "Resolution" naming theme providing clear purpose disambiguation. Step 2 implementation is straightforward as it primarily involves creating concrete ResolutionAdapter implementations following existing executor patterns. The registry structure, validation framework, and configuration integration are production-ready.

Key insight: The implementation can leverage existing state client patterns and helper API interfaces, making Step 2 primarily an exercise in adapter implementation rather than architectural design.

**Ready for Step 2 implementation** with clear path forward and low technical risk.