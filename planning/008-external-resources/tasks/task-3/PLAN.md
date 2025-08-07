# Implementation Plan: Step 2 - Resource Type Registry

**Date**: 2025-08-07  
**Task**: Step 2: Resource Type Registry Implementation  
**Context**: 008-external-resources planning stage  
**Objective**: Implement concrete ResolutionAdapter instances for all 12 resource types

## Executive Summary

This plan implements Step 2 of the external resources feature by creating concrete 
ResolutionAdapter implementations that enable actual resource resolution. Building on 
the solid foundation of Step 1 (schema, validation, and registry structure), Step 2 
transforms the registry from a metadata-only system into a fully functional resource 
resolution engine.

**Key Deliverables**:
- 12 concrete ResolutionAdapter implementations
- Extended state client with resolution methods
- Adapter factory with dependency injection
- Comprehensive parent-child relationship handling
- End-to-end resource resolution capability

**Implementation Strategy**: Phased approach with incremental validation, leveraging 
existing state client and helper API patterns for consistency.

## Implementation Phases

### Phase 1: Foundation Infrastructure
**Objective**: Create core infrastructure for adapter management
**Duration**: 1-2 implementation sessions  
**Risk Level**: Low

#### Phase 1.1: Base Adapter and Factory
**Files to Create**:
- `/internal/declarative/external/adapters/base_adapter.go`
- `/internal/declarative/external/adapters/factory.go`

**Implementation Steps**:

1. **Create Base Adapter Structure**:
   ```go
   // base_adapter.go
   package adapters

   import (
       "context"
       "fmt"
       "github.com/Kong/kongctl/internal/declarative/state"
   )

   // BaseAdapter provides common functionality for all resolution adapters
   type BaseAdapter struct {
       client *state.Client
   }

   // NewBaseAdapter creates a new base adapter with state client
   func NewBaseAdapter(client *state.Client) *BaseAdapter {
       return &BaseAdapter{client: client}
   }

   // ValidateParentContext validates parent context for child resources
   func (b *BaseAdapter) ValidateParentContext(parent *ResolvedParent, expectedType string) error {
       if parent == nil {
           return fmt.Errorf("parent context required for child resource")
       }
       if parent.Type != expectedType {
           return fmt.Errorf("invalid parent type: expected %s, got %s", expectedType, parent.Type)
       }
       return nil
   }

   // FilterBySelector filters resources by selector fields
   func (b *BaseAdapter) FilterBySelector(resources []interface{}, selector map[string]string, 
                                         getField func(interface{}, string) string) ([]interface{}, error) {
       var matches []interface{}
       for _, resource := range resources {
           match := true
           for field, value := range selector {
               if getField(resource, field) != value {
                   match = false
                   break
               }
           }
           if match {
               matches = append(matches, resource)
           }
       }

       if len(matches) == 0 {
           return nil, fmt.Errorf("no resources found matching selector: %v", selector)
       }
       if len(matches) > 1 {
           return nil, fmt.Errorf("selector matched %d resources, expected 1: %v", len(matches), selector)
       }

       return matches, nil
   }
   ```

2. **Create Adapter Factory**:
   ```go
   // factory.go
   package adapters

   import (
       "github.com/Kong/kongctl/internal/declarative/external"
       "github.com/Kong/kongctl/internal/declarative/state"
   )

   // AdapterFactory creates concrete resolution adapter implementations
   type AdapterFactory struct {
       client *state.Client
   }

   // NewAdapterFactory creates a new adapter factory
   func NewAdapterFactory(client *state.Client) *AdapterFactory {
       return &AdapterFactory{client: client}
   }

   // CreatePortalAdapter creates a portal resolution adapter
   func (f *AdapterFactory) CreatePortalAdapter() external.ResolutionAdapter {
       return NewPortalResolutionAdapter(f.client)
   }

   // CreateAPIAdapter creates an API resolution adapter
   func (f *AdapterFactory) CreateAPIAdapter() external.ResolutionAdapter {
       return NewAPIResolutionAdapter(f.client)
   }

   // ... Additional create methods for all 12 adapter types
   ```

3. **Update Registry for Adapter Injection**:
   **File to Modify**: `/internal/declarative/external/registry.go`
   
   Add adapter injection method:
   ```go
   // InjectAdapters updates the registry with concrete adapter implementations
   func (r *ResolutionRegistry) InjectAdapters(factory *AdapterFactory) {
       r.mu.Lock()
       defer r.mu.Unlock()

       r.resourceTypes["portal"].ResolutionAdapter = factory.CreatePortalAdapter()
       r.resourceTypes["api"].ResolutionAdapter = factory.CreateAPIAdapter()
       r.resourceTypes["control_plane"].ResolutionAdapter = factory.CreateControlPlaneAdapter()
       // ... Additional injections for all resource types
   }
   ```

**Validation Checkpoints**:
- [ ] `make build` passes - all new files compile
- [ ] Base adapter provides reusable functionality
- [ ] Factory creates adapter instances with state client injection
- [ ] Registry supports adapter injection during initialization

#### Phase 1.2: State Client Extension
**File to Modify**: `/internal/declarative/state/client.go`

**Implementation Steps**:

1. **Add Resolution Methods for Top-Level Resources**:
   ```go
   // GetPortalByID retrieves a portal by ID
   func (c *Client) GetPortalByID(ctx context.Context, id string) (*Portal, error) {
       portal, err := c.portalAPI.GetPortal(ctx, id)
       if err != nil {
           return nil, fmt.Errorf("failed to get portal by ID %s: %w", id, err)
       }
       return &Portal{
           ID:          portal.ID,
           Name:        portal.Name,
           Description: portal.Description,
           // ... map other fields
       }, nil
   }

   // ListPortalsWithFilter retrieves all portals and filters by selector
   func (c *Client) ListPortalsWithFilter(ctx context.Context, selector map[string]string) ([]*Portal, error) {
       portals, err := c.portalAPI.ListPortals(ctx)
       if err != nil {
           return nil, fmt.Errorf("failed to list portals: %w", err)
       }

       var filtered []*Portal
       for _, portal := range portals {
           match := true
           for field, value := range selector {
               switch field {
               case "name":
                   if portal.Name != value {
                       match = false
                   }
               case "description":
                   if portal.Description != value {
                       match = false
                   }
               default:
                   return nil, fmt.Errorf("unsupported selector field: %s", field)
               }
               if !match {
                   break
               }
           }
           if match {
               filtered = append(filtered, &Portal{
                   ID:          portal.ID,
                   Name:        portal.Name,
                   Description: portal.Description,
                   // ... map other fields
               })
           }
       }
       return filtered, nil
   }
   ```

2. **Add Similar Methods for Other Resource Types**:
   - `GetAPIByID`, `ListAPIsWithFilter`
   - `GetControlPlaneByID`, `ListControlPlanesWithFilter`
   - `GetApplicationAuthStrategyByID`, `ListApplicationAuthStrategiesWithFilter`

**Validation Checkpoints**:
- [ ] State client compiles with new resolution methods
- [ ] Methods follow existing naming and error handling patterns
- [ ] Proper error wrapping with context

### Phase 2: Top-Level Resource Adapters
**Objective**: Implement adapters for independent top-level resources
**Duration**: 2-3 implementation sessions  
**Risk Level**: Medium

#### Phase 2.1: Portal Resolution Adapter
**File to Create**: `/internal/declarative/external/adapters/portal_resolution_adapter.go`

**Implementation Steps**:

1. **Create Portal Adapter Structure**:
   ```go
   // portal_resolution_adapter.go
   package adapters

   import (
       "context"
       "github.com/Kong/kongctl/internal/declarative/external"
       "github.com/Kong/kongctl/internal/declarative/state"
   )

   // PortalResolutionAdapter handles portal resource resolution
   type PortalResolutionAdapter struct {
       *BaseAdapter
   }

   // NewPortalResolutionAdapter creates a new portal resolution adapter
   func NewPortalResolutionAdapter(client *state.Client) *PortalResolutionAdapter {
       return &PortalResolutionAdapter{
           BaseAdapter: NewBaseAdapter(client),
       }
   }

   // GetByID retrieves a portal by ID
   func (p *PortalResolutionAdapter) GetByID(ctx context.Context, id string, parent *external.ResolvedParent) (interface{}, error) {
       if parent != nil {
           return nil, fmt.Errorf("portal is a top-level resource and cannot have a parent")
       }
       
       portal, err := p.client.GetPortalByID(ctx, id)
       if err != nil {
           return nil, fmt.Errorf("failed to resolve portal by ID %s: %w", id, err)
       }
       
       return portal, nil
   }

   // GetBySelector retrieves portals by selector fields
   func (p *PortalResolutionAdapter) GetBySelector(ctx context.Context, selector map[string]string, parent *external.ResolvedParent) ([]interface{}, error) {
       if parent != nil {
           return nil, fmt.Errorf("portal is a top-level resource and cannot have a parent")
       }
       
       portals, err := p.client.ListPortalsWithFilter(ctx, selector)
       if err != nil {
           return nil, fmt.Errorf("failed to resolve portals by selector %v: %w", selector, err)
       }
       
       if len(portals) == 0 {
           return nil, fmt.Errorf("no portals found matching selector: %v", selector)
       }
       if len(portals) > 1 {
           return nil, fmt.Errorf("selector matched %d portals, expected 1: %v", len(portals), selector)
       }
       
       // Convert to []interface{}
       result := make([]interface{}, len(portals))
       for i, portal := range portals {
           result[i] = portal
       }
       
       return result, nil
   }
   ```

2. **Create Portal Adapter Test**:
   **File to Create**: `/internal/declarative/external/adapters/portal_resolution_adapter_test.go`
   
   Test scenarios:
   - GetByID with valid portal ID
   - GetByID with invalid portal ID (not found)
   - GetByID with parent context (should error)
   - GetBySelector with single match
   - GetBySelector with no matches (should error)
   - GetBySelector with multiple matches (should error)

#### Phase 2.2: API, ControlPlane, ApplicationAuthStrategy Adapters
**Files to Create**:
- `/internal/declarative/external/adapters/api_resolution_adapter.go`
- `/internal/declarative/external/adapters/control_plane_resolution_adapter.go`
- `/internal/declarative/external/adapters/application_auth_strategy_resolution_adapter.go`

**Implementation Pattern**: Follow same structure as PortalResolutionAdapter
- Validate no parent context for top-level resources
- Use corresponding state client methods
- Handle GetByID and GetBySelector flows
- Proper error wrapping and context

**Validation Checkpoints**:
- [ ] All 4 top-level adapters compile and test successfully
- [ ] Adapters properly reject parent context
- [ ] GetByID and GetBySelector methods work correctly
- [ ] Error handling follows established patterns
- [ ] Unit tests achieve >80% coverage

### Phase 3: Child Resource Adapters
**Objective**: Implement adapters for child resources with parent context handling
**Duration**: 3-4 implementation sessions  
**Risk Level**: Medium-High

#### Phase 3.1: Portal Child Resource Adapters
**Files to Create**:
- `/internal/declarative/external/adapters/portal_customization_resolution_adapter.go`
- `/internal/declarative/external/adapters/portal_custom_domain_resolution_adapter.go`
- `/internal/declarative/external/adapters/portal_page_resolution_adapter.go`
- `/internal/declarative/external/adapters/portal_snippet_resolution_adapter.go`

**Implementation Steps**:

1. **Add State Client Methods for Child Resources**:
   **File to Modify**: `/internal/declarative/state/client.go`
   
   ```go
   // GetPortalPageByID retrieves a portal page by portal ID and page ID
   func (c *Client) GetPortalPageByID(ctx context.Context, portalID, pageID string) (*PortalPage, error) {
       page, err := c.portalPageAPI.GetPortalPage(ctx, portalID, pageID)
       if err != nil {
           return nil, fmt.Errorf("failed to get portal page %s for portal %s: %w", pageID, portalID, err)
       }
       return &PortalPage{
           ID:        page.ID,
           PortalID:  portalID,
           Title:     page.Title,
           Content:   page.Content,
           // ... map other fields
       }, nil
   }

   // ListPortalPagesWithFilter retrieves portal pages for a portal and filters by selector
   func (c *Client) ListPortalPagesWithFilter(ctx context.Context, portalID string, selector map[string]string) ([]*PortalPage, error) {
       pages, err := c.portalPageAPI.ListPortalPages(ctx, portalID)
       if err != nil {
           return nil, fmt.Errorf("failed to list portal pages for portal %s: %w", portalID, err)
       }

       var filtered []*PortalPage
       for _, page := range pages {
           match := true
           for field, value := range selector {
               switch field {
               case "title":
                   if page.Title != value {
                       match = false
                   }
               case "slug":
                   if page.Slug != value {
                       match = false
                   }
               default:
                   return nil, fmt.Errorf("unsupported selector field for portal_page: %s", field)
               }
               if !match {
                   break
               }
           }
           if match {
               filtered = append(filtered, &PortalPage{
                   ID:       page.ID,
                   PortalID: portalID,
                   Title:    page.Title,
                   Content:  page.Content,
                   // ... map other fields
               })
           }
       }
       return filtered, nil
   }
   ```

2. **Create Portal Page Resolution Adapter**:
   ```go
   // portal_page_resolution_adapter.go
   package adapters

   import (
       "context"
       "github.com/Kong/kongctl/internal/declarative/external"
       "github.com/Kong/kongctl/internal/declarative/state"
   )

   // PortalPageResolutionAdapter handles portal page resource resolution
   type PortalPageResolutionAdapter struct {
       *BaseAdapter
   }

   // NewPortalPageResolutionAdapter creates a new portal page resolution adapter
   func NewPortalPageResolutionAdapter(client *state.Client) *PortalPageResolutionAdapter {
       return &PortalPageResolutionAdapter{
           BaseAdapter: NewBaseAdapter(client),
       }
   }

   // GetByID retrieves a portal page by ID with parent context
   func (p *PortalPageResolutionAdapter) GetByID(ctx context.Context, id string, parent *external.ResolvedParent) (interface{}, error) {
       if err := p.ValidateParentContext(parent, "portal"); err != nil {
           return nil, err
       }
       
       page, err := p.client.GetPortalPageByID(ctx, parent.ID, id)
       if err != nil {
           return nil, fmt.Errorf("failed to resolve portal page by ID %s for portal %s: %w", id, parent.ID, err)
       }
       
       return page, nil
   }

   // GetBySelector retrieves portal pages by selector fields with parent context
   func (p *PortalPageResolutionAdapter) GetBySelector(ctx context.Context, selector map[string]string, parent *external.ResolvedParent) ([]interface{}, error) {
       if err := p.ValidateParentContext(parent, "portal"); err != nil {
           return nil, err
       }
       
       pages, err := p.client.ListPortalPagesWithFilter(ctx, parent.ID, selector)
       if err != nil {
           return nil, fmt.Errorf("failed to resolve portal pages by selector %v for portal %s: %w", selector, parent.ID, err)
       }
       
       if len(pages) == 0 {
           return nil, fmt.Errorf("no portal pages found matching selector %v for portal %s", selector, parent.ID)
       }
       if len(pages) > 1 {
           return nil, fmt.Errorf("selector matched %d portal pages for portal %s, expected 1: %v", len(pages), parent.ID, selector)
       }
       
       // Convert to []interface{}
       result := make([]interface{}, len(pages))
       for i, page := range pages {
           result[i] = page
       }
       
       return result, nil
   }
   ```

#### Phase 3.2: API Child Resource Adapters
**Files to Create**:
- `/internal/declarative/external/adapters/api_version_resolution_adapter.go`
- `/internal/declarative/external/adapters/api_publication_resolution_adapter.go`
- `/internal/declarative/external/adapters/api_implementation_resolution_adapter.go`
- `/internal/declarative/external/adapters/api_document_resolution_adapter.go`

**Implementation Pattern**: Follow same structure as PortalPageResolutionAdapter
- Validate parent context is "api" type
- Use parent.ID in API calls to child resources
- Handle GetByID and GetBySelector with parent context
- Add corresponding state client methods

**Validation Checkpoints**:
- [ ] All 8 child resource adapters compile and test successfully
- [ ] Adapters properly validate and use parent context
- [ ] GetByID uses parent ID in API calls correctly
- [ ] GetBySelector filters within parent scope
- [ ] Parent validation rejects incorrect parent types
- [ ] Unit tests cover parent-child scenarios

### Phase 4: Registry Integration and Testing
**Objective**: Complete registry integration and comprehensive testing
**Duration**: 2-3 implementation sessions  
**Risk Level**: Low

#### Phase 4.1: Registry Integration
**File to Modify**: `/internal/declarative/external/registry.go`

**Implementation Steps**:

1. **Complete Adapter Injection**:
   ```go
   // InjectAdapters updates the registry with all concrete adapter implementations
   func (r *ResolutionRegistry) InjectAdapters(factory *AdapterFactory) {
       r.mu.Lock()
       defer r.mu.Unlock()

       // Top-level resource adapters
       r.resourceTypes["portal"].ResolutionAdapter = factory.CreatePortalAdapter()
       r.resourceTypes["api"].ResolutionAdapter = factory.CreateAPIAdapter()
       r.resourceTypes["control_plane"].ResolutionAdapter = factory.CreateControlPlaneAdapter()
       r.resourceTypes["application_auth_strategy"].ResolutionAdapter = factory.CreateApplicationAuthStrategyAdapter()

       // Portal child resource adapters
       r.resourceTypes["portal_customization"].ResolutionAdapter = factory.CreatePortalCustomizationAdapter()
       r.resourceTypes["portal_custom_domain"].ResolutionAdapter = factory.CreatePortalCustomDomainAdapter()
       r.resourceTypes["portal_page"].ResolutionAdapter = factory.CreatePortalPageAdapter()
       r.resourceTypes["portal_snippet"].ResolutionAdapter = factory.CreatePortalSnippetAdapter()

       // API child resource adapters
       r.resourceTypes["api_version"].ResolutionAdapter = factory.CreateAPIVersionAdapter()
       r.resourceTypes["api_publication"].ResolutionAdapter = factory.CreateAPIPublicationAdapter()
       r.resourceTypes["api_implementation"].ResolutionAdapter = factory.CreateAPIImplementationAdapter()
       r.resourceTypes["api_document"].ResolutionAdapter = factory.CreateAPIDocumentAdapter()
   }
   ```

2. **Add Registry Initialization with Adapters**:
   ```go
   // InitializeWithAdapters creates and configures the registry with concrete adapters
   func InitializeWithAdapters(client *state.Client) {
       registry := GetInstance()
       factory := adapters.NewAdapterFactory(client)
       registry.InjectAdapters(factory)
   }
   ```

#### Phase 4.2: Integration Testing
**File to Modify**: `/internal/declarative/external/registry_test.go`

**Test Scenarios**:

1. **Adapter Registration Tests**:
   ```go
   func TestRegistryAdapterInjection(t *testing.T) {
       // Test that all 12 resource types get concrete adapters
       // Test that adapters are retrievable via registry
       // Test that adapters are not nil after injection
   }
   ```

2. **End-to-End Resolution Tests**:
   ```go
   func TestPortalResolutionFlow(t *testing.T) {
       // Mock state client with test data
       // Test GetByID and GetBySelector flows
       // Validate resolved resources have correct data
   }

   func TestParentChildResolutionFlow(t *testing.T) {
       // Mock portal and portal page data
       // Test parent resolution followed by child resolution
       // Validate parent context propagation
   }
   ```

3. **Error Handling Tests**:
   ```go
   func TestResolutionErrors(t *testing.T) {
       // Test resource not found scenarios
       // Test multiple matches for selector
       // Test parent validation failures
       // Test SDK call failures
   }
   ```

**Validation Checkpoints**:
- [ ] Registry properly initializes with all 12 adapters
- [ ] Adapter retrieval works for all resource types
- [ ] End-to-end resolution flows work correctly
- [ ] Parent-child relationships function properly
- [ ] Error scenarios are handled gracefully
- [ ] All tests pass with `make test`

#### Phase 4.3: Quality Validation
**Quality Gates**:

1. **Build Validation**:
   ```bash
   make build
   ```
   - All new files compile successfully
   - No circular dependencies introduced
   - Interface implementations complete

2. **Lint Validation**:
   ```bash
   make lint
   ```
   - Zero linting issues
   - Consistent error handling patterns
   - Proper documentation comments

3. **Test Validation**:
   ```bash
   make test
   ```
   - All unit tests pass
   - New adapter tests with >80% coverage
   - Registry integration tests pass
   - No test flakiness or race conditions

4. **Integration Test Validation** (if applicable):
   ```bash
   make test-integration
   ```
   - End-to-end flows work with real SDK calls
   - Authentication and network handling work
   - Error scenarios properly handled

## File Structure

### New Files to Create

**Core Infrastructure**:
```
/internal/declarative/external/adapters/
├── factory.go                          # Adapter factory with dependency injection
├── base_adapter.go                     # Common functionality for all adapters
```

**Top-Level Resource Adapters**:
```
/internal/declarative/external/adapters/
├── portal_resolution_adapter.go        # Portal resource resolution
├── api_resolution_adapter.go           # API resource resolution
├── control_plane_resolution_adapter.go # Control plane resolution
├── application_auth_strategy_resolution_adapter.go # Auth strategy resolution
```

**Portal Child Resource Adapters**:
```
/internal/declarative/external/adapters/
├── portal_customization_resolution_adapter.go  # Portal customization
├── portal_custom_domain_resolution_adapter.go  # Portal custom domain
├── portal_page_resolution_adapter.go           # Portal page
├── portal_snippet_resolution_adapter.go        # Portal snippet
```

**API Child Resource Adapters**:
```
/internal/declarative/external/adapters/
├── api_version_resolution_adapter.go           # API version
├── api_publication_resolution_adapter.go       # API publication
├── api_implementation_resolution_adapter.go    # API implementation
├── api_document_resolution_adapter.go          # API document
```

**Test Files**:
```
/internal/declarative/external/adapters/
├── factory_test.go                     # Factory creation and injection tests
├── portal_resolution_adapter_test.go   # Portal adapter unit tests
├── api_resolution_adapter_test.go      # API adapter unit tests
├── [...all other adapter test files...]
```

### Files to Modify

**Registry Integration**:
- `/internal/declarative/external/registry.go`
  - Add `InjectAdapters()` method
  - Add `InitializeWithAdapters()` helper
  - Update adapter injection during initialization

**State Client Extension**:
- `/internal/declarative/state/client.go`
  - Add resolution methods for all 12 resource types
  - Add `GetByID` methods: `GetPortalByID`, `GetAPIByID`, etc.
  - Add `ListWithFilter` methods: `ListPortalsWithFilter`, `ListAPIsWithFilter`, etc.
  - Add child resource methods with parent ID parameters

**Test Updates**:
- `/internal/declarative/external/registry_test.go`
  - Test adapter registration and retrieval
  - Test end-to-end resolution flows
  - Test parent-child relationship handling

## Code Architecture

### Core Interfaces

**ResolutionAdapter Interface** (Already defined in Step 1):
```go
type ResolutionAdapter interface {
    GetByID(ctx context.Context, id string, parent *ResolvedParent) (interface{}, error)
    GetBySelector(ctx context.Context, selector map[string]string, parent *ResolvedParent) ([]interface{}, error)
}
```

**ResolvedParent Structure** (Already defined in Step 1):
```go
type ResolvedParent struct {
    ID       string
    Resource interface{}
    Type     string
}
```

### Adapter Implementations

**Base Adapter Pattern**:
```go
type BaseAdapter struct {
    client *state.Client
}

func (b *BaseAdapter) ValidateParentContext(parent *ResolvedParent, expectedType string) error
func (b *BaseAdapter) FilterBySelector(resources []interface{}, selector map[string]string, getField func(interface{}, string) string) ([]interface{}, error)
```

**Concrete Adapter Pattern**:
```go
type PortalResolutionAdapter struct {
    *BaseAdapter
}

func (p *PortalResolutionAdapter) GetByID(ctx context.Context, id string, parent *ResolvedParent) (interface{}, error)
func (p *PortalResolutionAdapter) GetBySelector(ctx context.Context, selector map[string]string, parent *ResolvedParent) ([]interface{}, error)
```

**Factory Pattern**:
```go
type AdapterFactory struct {
    client *state.Client
}

func (f *AdapterFactory) CreatePortalAdapter() external.ResolutionAdapter
func (f *AdapterFactory) CreateAPIAdapter() external.ResolutionAdapter
// ... Additional create methods for all 12 adapter types
```

## SDK Integration

### Helper API Interface Usage

**Existing Helper Interfaces** (Located in `/internal/konnect/helpers/`):
- `PortalAPI`: Portal operations
- `APIAPI`: API operations  
- `AppAuthStrategiesAPI`: Application auth strategy operations
- `PortalPageAPI`: Portal page operations
- `PortalCustomizationAPI`: Portal customization operations

**State Client Method Patterns**:

**Top-Level Resources**:
```go
// Direct ID lookup
func (c *Client) GetPortalByID(ctx context.Context, id string) (*Portal, error)
func (c *Client) GetAPIByID(ctx context.Context, id string) (*API, error)

// List and filter by selector
func (c *Client) ListPortalsWithFilter(ctx context.Context, selector map[string]string) ([]*Portal, error)
func (c *Client) ListAPIsWithFilter(ctx context.Context, selector map[string]string) ([]*API, error)
```

**Child Resources with Parent Context**:
```go
// Child resource ID lookup with parent ID
func (c *Client) GetPortalPageByID(ctx context.Context, portalID, pageID string) (*PortalPage, error)
func (c *Client) GetAPIVersionByID(ctx context.Context, apiID, versionID string) (*APIVersion, error)

// Child resource list and filter within parent scope
func (c *Client) ListPortalPagesWithFilter(ctx context.Context, portalID string, selector map[string]string) ([]*PortalPage, error)
func (c *Client) ListAPIVersionsWithFilter(ctx context.Context, apiID string, selector map[string]string) ([]*APIVersion, error)
```

### SDK Method Mapping

**GetByID Operations**:
- `portal`: `portalAPI.GetPortal(ctx, id)`
- `api`: `apiAPI.GetAPI(ctx, id)`
- `control_plane`: `controlPlaneAPI.GetControlPlane(ctx, id)`
- `application_auth_strategy`: `appAuthAPI.GetApplicationAuthStrategy(ctx, id)`
- `portal_page`: `portalPageAPI.GetPortalPage(ctx, portalID, pageID)`
- `api_version`: `apiVersionAPI.GetAPIVersion(ctx, apiID, versionID)`

**GetBySelector Operations** (List + Filter):
- `portal`: `portalAPI.ListPortals(ctx)` → filter by name, description
- `api`: `apiAPI.ListAPIs(ctx)` → filter by name, description
- `portal_page`: `portalPageAPI.ListPortalPages(ctx, portalID)` → filter by title, slug
- `api_version`: `apiVersionAPI.ListAPIVersions(ctx, apiID)` → filter by version, name

## Parent-Child Relationships

### Resolution Order

**Dependency Graph**:
```
Top-Level (Resolved First):
├── portal
├── api  
├── control_plane
└── application_auth_strategy

Children (Resolved After Parents):
├── portal_customization (parent: portal)
├── portal_custom_domain (parent: portal)
├── portal_page (parent: portal)
├── portal_snippet (parent: portal)
├── api_version (parent: api)
├── api_publication (parent: api)
├── api_implementation (parent: api)
└── api_document (parent: api)
```

### Parent Context Handling

**Parent Resolution Flow**:
1. **Parent Resource Resolved**: Portal resolved with ID "portal-123"
2. **ResolvedParent Created**: `{ID: "portal-123", Resource: portal, Type: "portal"}`
3. **Child Resolution**: Portal page adapter receives ResolvedParent context
4. **Child API Call**: `portalPageAPI.GetPortalPage(ctx, "portal-123", "page-456")`

**Parent Validation**:
```go
func (p *PortalPageResolutionAdapter) GetByID(ctx context.Context, id string, parent *ResolvedParent) (interface{}, error) {
    if err := p.ValidateParentContext(parent, "portal"); err != nil {
        return nil, err
    }
    // Use parent.ID in API call
    return p.client.GetPortalPageByID(ctx, parent.ID, id)
}
```

### Configuration Examples

**Portal with Portal Page**:
```yaml
external_resources:
  - ref: "main-portal"
    resource_type: "portal"
    selector:
      match_fields:
        name: "Developer Portal"
  
  - ref: "getting-started"
    resource_type: "portal_page"
    parent:
      resource_type: "portal"
      ref: "main-portal"
    selector:
      match_fields:
        title: "Getting Started Guide"
```

**API with API Version**:
```yaml
external_resources:
  - ref: "petstore-api"
    resource_type: "api"
    id: "api-123"
  
  - ref: "petstore-v2"
    resource_type: "api_version"
    parent:
      resource_type: "api"
      ref: "petstore-api"
    id: "version-456"
```

## Testing Strategy

### Unit Testing Approach

**Adapter Unit Tests**:
- Mock state client with test data
- Test GetByID with valid/invalid IDs
- Test GetBySelector with various selector combinations
- Test parent context validation (for child resources)
- Test error scenarios (not found, multiple matches, SDK failures)

**Test File Structure**:
```go
// portal_resolution_adapter_test.go
func TestPortalResolutionAdapter_GetByID(t *testing.T) {
    tests := []struct {
        name          string
        id            string
        parent        *ResolvedParent
        mockResponse  *Portal
        mockError     error
        expectedError string
    }{
        {
            name:         "successful ID lookup",
            id:           "portal-123",
            parent:       nil,
            mockResponse: &Portal{ID: "portal-123", Name: "Test Portal"},
            mockError:    nil,
        },
        {
            name:          "parent context provided for top-level resource",
            id:            "portal-123", 
            parent:        &ResolvedParent{Type: "api"},
            expectedError: "portal is a top-level resource and cannot have a parent",
        },
        // ... additional test cases
    }
}
```

### Integration Testing

**Registry Integration Tests**:
```go
func TestResolutionRegistryIntegration(t *testing.T) {
    // Test complete flow from registry lookup to adapter resolution
    // Mock state client with realistic test data
    // Test parent-child resolution chains
    // Validate resolved resources have correct structure and data
}
```

**Parent-Child Flow Tests**:
```go
func TestParentChildResolutionFlow(t *testing.T) {
    // Mock portal resolution returning valid portal
    // Mock portal page resolution using portal ID
    // Test complete parent → child resolution flow
    // Validate parent context propagation
}
```

### Quality Gates

**Coverage Requirements**:
- Unit tests: >80% coverage for all adapter implementations
- Integration tests: Cover major resolution flows
- Error scenarios: Test all identified error conditions

**Test Execution Commands**:
```bash
# Run all tests with coverage
go test -race -count=1 -cover ./internal/declarative/external/...

# Run specific adapter tests
go test -v ./internal/declarative/external/adapters/

# Run integration tests
go test -v -tags=integration ./internal/declarative/external/
```

## Risk Mitigation

### Identified Risks and Mitigation Strategies

#### Medium Risk: SDK API Method Discovery
**Risk**: Some resource types may not have expected helper API methods
**Impact**: Adapter implementation may be blocked or require workarounds
**Mitigation**:
- Start implementation with known working methods (Portal, API)
- Extend helper interfaces if methods are missing but exist in SDK
- Create wrapper methods in state client for direct SDK access if needed
**Contingency**: Implement direct SDK calls with proper error handling

#### Medium Risk: Selector Performance on Large Resource Sets
**Risk**: List-and-filter approach may be slow for Konnect instances with many resources
**Impact**: Resolution operations may timeout or perform poorly
**Mitigation**:
- Implement pagination support in list operations
- Consider SDK-level filtering parameters where available
- Add performance monitoring and warnings
**Contingency**: Implement client-side caching for frequently accessed resources

#### Medium Risk: Parent Context Complexity
**Risk**: Parent-child relationship mapping more complex than expected
**Impact**: Child resource resolution may fail or require significant rework
**Mitigation**:
- Start with simple parent-child pairs (Portal → PortalPage)
- Test parent context propagation thoroughly
- Implement incremental complexity
**Contingency**: Simplify initial implementation to basic ID-passing, add complexity later

#### Low Risk: Helper Interface Completeness
**Risk**: Helper API interfaces may not cover all needed operations
**Impact**: May need to extend interfaces or create new ones
**Mitigation**:
- Review existing helper interfaces before implementation
- Follow established patterns for interface extensions
- Maintain consistency with existing codebase
**Contingency**: Create additional helper methods following existing patterns

### Implementation Risk Mitigation

**Phase-Based Implementation**:
- Each phase has clear validation checkpoints
- Early phases establish foundation for later phases
- Issues identified early before complexity increases

**Incremental Validation**:
- Build validation after each significant change
- Unit test creation alongside implementation
- Integration testing to catch interaction issues

**Rollback Strategy**:
- Each phase creates distinct, testable components
- Registry can function with partially implemented adapters
- Clear separation allows selective rollback if needed

## Success Criteria

### Functional Requirements

- [ ] **All 12 Resource Types Have Working Adapters**: Every resource type defined in the registry has a concrete ResolutionAdapter implementation

- [ ] **Registry Adapter Injection**: Registry properly injects concrete adapters during initialization and they are retrievable via `Registry.Get(type)`

- [ ] **GetByID Resolution**: All resource types support direct ID lookup through `GetByID()` method with proper SDK integration

- [ ] **GetBySelector Resolution**: All resource types support selector-based filtering through `GetBySelector()` method

- [ ] **Parent-Child Resolution**: Child resources properly receive and use parent context from `ResolvedParent` structure

- [ ] **Resource Type Validation**: Registry validation continues to work and prevents invalid resource type usage

- [ ] **State Client Integration**: Extended state client methods work consistently with existing patterns and error handling

### Quality Requirements

- [ ] **Build Success**: `make build` completes successfully with all new files and modifications

- [ ] **Lint Clean**: `make lint` reports zero issues with consistent code style and documentation

- [ ] **Test Coverage**: `make test` passes with >80% coverage for new adapter implementations

- [ ] **Error Handling**: Comprehensive error handling with clear, actionable error messages for all failure scenarios

- [ ] **Performance**: Resolution operations complete within reasonable time limits (< 10s for typical operations)

### Integration Requirements

- [ ] **Configuration Loading**: External resources configuration loads and validates correctly with new adapters

- [ ] **Plan/Apply Integration**: External resource resolution works within existing plan/apply command flows

- [ ] **SDK Compatibility**: All adapters work correctly with current Konnect Go SDK versions

- [ ] **Thread Safety**: Registry and adapters function correctly in concurrent environments

### Documentation Requirements

- [ ] **Code Documentation**: All public interfaces and methods have proper Go documentation comments

- [ ] **Error Documentation**: Error conditions and messages are clear and actionable

- [ ] **Example Usage**: Working examples of external resource configurations for all supported resource types

## Implementation Timeline

### Week 1: Foundation Infrastructure
**Duration**: 3-5 implementation sessions
- **Day 1-2**: Phase 1.1 - Base adapter and factory implementation
- **Day 3-4**: Phase 1.2 - State client extension with resolution methods
- **Day 5**: Validation and testing of foundation components

**Deliverables**:
- BaseAdapter with common functionality
- AdapterFactory with dependency injection
- Extended state client with resolution methods
- Registry adapter injection capability

### Week 2: Top-Level Resource Adapters  
**Duration**: 4-6 implementation sessions
- **Day 1-2**: Phase 2.1 - Portal resolution adapter with tests
- **Day 3-4**: Phase 2.2 - API, ControlPlane, ApplicationAuthStrategy adapters
- **Day 5**: Integration testing and validation

**Deliverables**:
- 4 top-level resource adapters fully implemented
- Comprehensive unit tests for all top-level adapters
- Registry integration with top-level adapters
- Validation of GetByID and GetBySelector flows

### Week 3: Child Resource Adapters
**Duration**: 5-7 implementation sessions  
- **Day 1-2**: Phase 3.1 - Portal child resource adapters (4 adapters)
- **Day 3-4**: Phase 3.2 - API child resource adapters (4 adapters)
- **Day 5**: Parent-child relationship testing and validation

**Deliverables**:
- 8 child resource adapters fully implemented
- Parent context validation and propagation working
- Complete unit test coverage for child adapters
- Integration testing of parent-child resolution flows

### Week 4: Integration and Quality Validation
**Duration**: 3-4 implementation sessions
- **Day 1**: Phase 4.1 - Complete registry integration
- **Day 2**: Phase 4.2 - Integration testing and error scenario validation
- **Day 3**: Phase 4.3 - Quality gates and comprehensive testing
- **Day 4**: Documentation updates and final validation

**Deliverables**:
- Complete registry with all 12 adapters injected
- Comprehensive integration test suite
- All quality gates passing (build, lint, test)
- Updated documentation and examples

### Total Timeline Estimate
**4 weeks** with parallel work on documentation and testing throughout the implementation phases.

**Critical Path**:
Foundation → Top-Level Adapters → Child Adapters → Integration

**Risk Buffer**:
Additional 1-2 weeks allocated for unforeseen issues or scope expansion.

---

## Conclusion

This implementation plan transforms the Step 1 registry foundation into a fully functional external resource resolution system. The phased approach with clear validation checkpoints ensures high-quality implementation while maintaining consistency with existing codebase patterns.

The plan leverages the solid architecture established in Step 1 and builds upon proven patterns from the state client and executor systems. With comprehensive testing, error handling, and parent-child relationship support, Step 2 will provide a robust foundation for the remaining external resources implementation phases.

**Next Steps**: Begin Phase 1.1 implementation with base adapter and factory creation, followed by state client extension to establish the core infrastructure for all subsequent phases.