# Flow Report: Step 2 - Resource Type Registry

**Date**: 2025-08-07  
**Task**: Step 2: Resource Type Registry Flow Analysis  
**Context**: 008-external-resources planning stage  
**Analysis**: External resource resolution execution flows and integration points  

## Executive Summary

This report maps the complete execution flow for external resource resolution in the Resource Type Registry implementation (Step 2). The analysis reveals a well-architected foundation from Step 1 that supports a clean, dependency-driven resolution flow. The implementation follows established patterns from the existing executor and state client architecture, ensuring consistency and maintainability.

**Key Flow Characteristics**:
- **Dependency-first resolution**: Parents resolved before children
- **Registry-driven metadata**: Centralized resource type definitions
- **Adapter pattern**: Clean separation between resolution logic and SDK operations
- **State client integration**: Leverages existing API wrapper patterns
- **Comprehensive validation**: Multi-stage validation with early failure detection

## 1. Main Resolution Flow

The core execution path for resolving external resources:

```
┌─────────────────────┐
│ YAML Configuration  │
│ external_resources: │
│   - ref: "portal-1" │
│     resource_type:  │
│       "portal"      │
│     selector: {...} │
└─────────────────────┘
           │
           ▼
┌─────────────────────┐    ┌──────────────────────┐
│ Configuration       │───▶│ Validation           │
│ Loading             │    │ - Resource type      │
│ - Parse YAML        │    │ - ID XOR selector    │
│ - Create structs    │    │ - Parent-child       │
└─────────────────────┘    │ - Selector fields    │
           │                └──────────────────────┘
           ▼                           │
┌─────────────────────┐               │
│ Dependency Analysis │◀──────────────┘
│ - Identify parents  │
│ - Resolution order  │
└─────────────────────┘
           │
           ▼
┌─────────────────────┐    ┌──────────────────────┐
│ Registry Lookup     │───▶│ ResolutionMetadata   │
│ - Get resource type │    │ - SelectorFields     │
│ - Validate exists   │    │ - SupportedParents   │
└─────────────────────┘    │ - ResolutionAdapter  │
           │                └──────────────────────┘
           ▼                           │
┌─────────────────────┐               │
│ Parent Resolution   │◀──────────────┘
│ (if needed)         │
│ - Resolve parent    │
│ - Create context    │
└─────────────────────┘
           │
           ▼
┌─────────────────────┐    ┌──────────────────────┐
│ Adapter Resolution  │───▶│ SDK Operation        │
│ - GetByID() or      │    │ - Direct API call    │
│ - GetBySelector()   │    │ - List + filter      │
│ - With parent ctx   │    │ - Parent context     │
└─────────────────────┘    └──────────────────────┘
           │
           ▼
┌─────────────────────┐
│ State Update        │
│ - resolvedID        │
│ - resolvedResource  │
│ - resolved = true   │
└─────────────────────┘
           │
           ▼
┌─────────────────────┐
│ Resource Available  │
│ for Reference       │
│ (via .ref field)    │
└─────────────────────┘
```

**Flow Steps**:
1. **Configuration Loading**: YAML parsed into `ExternalResourceResource` structs
2. **Validation**: Comprehensive validation of resource type, ID/selector, relationships
3. **Dependency Analysis**: Identify parent resources that must be resolved first
4. **Registry Lookup**: Get `ResolutionMetadata` for resource type
5. **Parent Resolution**: Resolve parent resources and create `ResolvedParent` context
6. **Adapter Resolution**: Call appropriate adapter method with parent context
7. **SDK Operation**: Execute Konnect SDK calls to retrieve resources
8. **State Update**: Update resource with resolved data
9. **Resource Available**: Resource accessible via `.ref` for other operations

## 2. Resource Type Registration Flow

How resource types are registered and accessed in the registry:

```
┌─────────────────────┐
│ System Startup      │
└─────────────────────┘
           │
           ▼
┌─────────────────────┐    ┌──────────────────────┐
│ Registry            │───▶│ Built-in Resource    │
│ Initialization      │    │ Types Registration   │
│ - Singleton create  │    │ - portal             │
│ - Thread safety     │    │ - api                │
└─────────────────────┘    │ - control_plane      │
           │                │ - All child types    │
           ▼                └──────────────────────┘
┌─────────────────────┐               │
│ Adapter Factory     │◀──────────────┘
│ Creation            │
│ - State client inj. │
│ - Create adapters   │
└─────────────────────┘
           │
           ▼
┌─────────────────────┐    ┌──────────────────────┐
│ Adapter Injection   │───▶│ Registry Update      │
│ - PortalAdapter     │    │ - metadata.Adapter   │
│ - APIAdapter        │    │   = concrete impl    │
│ - ControlPlaneAdap. │    │ - Ready for lookup   │
│ - All 12 adapters   │    └──────────────────────┘
└─────────────────────┘               │
           │                          │
           ▼                          ▼
┌─────────────────────┐    ┌──────────────────────┐
│ Runtime Lookup      │───▶│ Adapter Resolution   │
│ Registry.Get(type)  │    │ - metadata.Adapter   │
│ - Thread-safe       │    │ - Concrete instance  │
│ - Validate exists   │    │ - Ready for use      │
└─────────────────────┘    └──────────────────────┘
```

**Registry Components**:
- **ResolutionRegistry**: Thread-safe singleton with all resource type metadata
- **ResolutionMetadata**: Per-type configuration (name, selector fields, relationships, adapter)
- **AdapterFactory**: Creates concrete adapter implementations with dependency injection
- **Runtime Lookup**: Registry.Get(type) returns metadata with concrete adapter

**Resource Type Hierarchy**:
```
Top-Level Resources:
├── portal (supports: portal_customization, portal_custom_domain, portal_page, portal_snippet)
├── api (supports: api_version, api_publication, api_implementation, api_document)
├── control_plane (no children currently)
└── application_auth_strategy (no children currently)

Child Resources:
├── portal_customization (parent: portal)
├── portal_custom_domain (parent: portal)  
├── portal_page (parent: portal)
├── portal_snippet (parent: portal)
├── api_version (parent: api)
├── api_publication (parent: api)
├── api_implementation (parent: api)
└── api_document (parent: api)
```

## 3. SDK Operation Mapping Flow

How resolution adapters interact with Konnect SDK operations:

```
┌─────────────────────┐    ┌──────────────────────┐
│ ResolutionAdapter   │───▶│ State Client         │
│ Interface           │    │ Extension            │
│ - GetByID()         │    │ - Resolution methods │
│ - GetBySelector()   │    │ - SDK wrapper        │
└─────────────────────┘    └──────────────────────┘
           │                           │
           ▼                           ▼
┌─────────────────────┐    ┌──────────────────────┐
│ Concrete Adapters   │───▶│ Helper API           │
│ - PortalAdapter     │    │ Interfaces           │
│ - APIAdapter        │    │ - PortalAPI          │
│ - ControlPlaneAdap. │    │ - APIAPI             │
│ - All child adapts  │    │ - AppAuthAPI         │
└─────────────────────┘    │ - Page/Custom APIs   │
           │                └──────────────────────┘
           ▼                           │
┌─────────────────────┐               ▼
│ Resolution Method   │    ┌──────────────────────┐
│ Selection           │───▶│ SDK Method Mapping   │
│ - ID provided?      │    │                      │
│   → GetByID flow    │    │ GetByID Flow:        │
│ - Selector provided?│    │ - portalAPI.GetPortal│
│   → GetBySelector   │    │ - apiAPI.GetAPI      │
└─────────────────────┘    │ - directAPI.Get*     │
           │                │                      │
           ▼                │ GetBySelector Flow:  │
┌─────────────────────┐    │ - portalAPI.List*    │
│ Parent Context      │    │ - Filter by selector │
│ Handling            │    │ - Pagination support │
│ - Parent resolved?  │    │                      │
│ - Create context    │    │ Child Resource Flow: │
│ - Pass to adapter   │    │ - Use parent ID      │
└─────────────────────┘    │ - portal_page API    │
           │                └──────────────────────┘
           ▼
┌─────────────────────┐
│ SDK Call Execution  │
│ - HTTP request      │
│ - Response parsing  │
│ - Error handling    │
└─────────────────────┘
```

**SDK Method Patterns**:

**GetByID Operations**:
- Direct SDK calls using resource ID
- `portalAPI.GetPortal(id)` → portal resource
- `apiAPI.GetAPI(id)` → API resource
- Child resources: `portalPageAPI.GetPortalPage(portalId, pageId)`

**GetBySelector Operations**:
- List all resources, then filter by selector fields
- `portalAPI.ListPortals()` → filter by name, description
- `apiAPI.ListAPIs()` → filter by name, description
- Pagination handling for large result sets

**Parent Context Integration**:
- Child resources receive `ResolvedParent` context
- Parent ID used in child resource SDK calls
- Example: Portal pages need portal ID for API operations

## 4. Parent-Child Resolution Flow

Hierarchical dependency resolution with parent-first ordering:

```
┌─────────────────────┐
│ External Resources  │
│ Configuration       │
│ - portal (parent)   │
│ - portal_page (child│
└─────────────────────┘
           │
           ▼
┌─────────────────────┐    ┌──────────────────────┐
│ Dependency Graph    │───▶│ Resolution Order     │
│ Analysis            │    │ Determination        │
│ - Identify parents  │    │ 1. Parents first     │
│ - Build dep tree    │    │ 2. Children second   │
│ - Detect cycles     │    │ 3. Siblings parallel │
└─────────────────────┘    └──────────────────────┘
           │                           │
           ▼                           ▼
┌─────────────────────┐    ┌──────────────────────┐
│ Parent Resolution   │───▶│ Parent Resource      │
│ Phase               │    │ Resolution           │
│ - Resolve parent    │    │ - Registry lookup    │
│ - Validate success  │    │ - Adapter call       │
│ - Create context    │    │ - SDK operation      │
└─────────────────────┘    └──────────────────────┘
           │                           │
           ▼                           ▼
┌─────────────────────┐    ┌──────────────────────┐
│ ResolvedParent      │───▶│ Child Resource       │
│ Context Creation    │    │ Resolution           │
│ - Parent ID         │    │ - Receive context    │
│ - Parent resource   │    │ - Use parent ID      │
│ - Parent type       │    │ - SDK call w/parent  │
└─────────────────────┘    └──────────────────────┘
           │
           ▼
┌─────────────────────┐
│ Hierarchical        │
│ Validation          │
│ - Parent exists     │
│ - Relationship valid│
│ - Child accessible  │
└─────────────────────┘
```

**Parent-Child Examples**:

**Portal → Portal Page**:
```yaml
external_resources:
  - ref: "main-portal"
    resource_type: "portal"
    selector:
      match_fields:
        name: "Developer Portal"
  
  - ref: "getting-started-page"  
    resource_type: "portal_page"
    parent:
      resource_type: "portal"
      ref: "main-portal"  # References resolved parent
    selector:
      match_fields:
        title: "Getting Started"
```

**Resolution Sequence**:
1. **Portal Resolution**: `portalAPI.ListPortals()` → filter by name "Developer Portal"
2. **Portal Context**: Create `ResolvedParent{ID: portal.ID, Resource: portal, Type: "portal"}`
3. **Page Resolution**: `portalPageAPI.ListPortalPages(portal.ID)` → filter by title "Getting Started"

**API → API Version**:
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

**Resolution Sequence**:
1. **API Resolution**: `apiAPI.GetAPI("api-123")`
2. **API Context**: Create `ResolvedParent{ID: "api-123", Resource: api, Type: "api"}`
3. **Version Resolution**: `apiVersionAPI.GetAPIVersion("api-123", "version-456")`

## 5. Validation Flow

Multi-stage validation ensuring configuration and runtime correctness:

```
┌─────────────────────┐
│ Configuration       │
│ Loading Phase       │
└─────────────────────┘
           │
           ▼
┌─────────────────────┐    ┌──────────────────────┐
│ Schema Validation   │───▶│ Resource Type        │
│ - YAML structure    │    │ Validation           │
│ - Required fields   │    │ - Registry lookup    │
│ - Field types       │    │ - Type exists?       │
└─────────────────────┘    └──────────────────────┘
           │                           │
           ▼                           ▼
┌─────────────────────┐    ┌──────────────────────┐
│ ID XOR Selector     │───▶│ Selector Field       │
│ Validation          │    │ Validation           │
│ - Exactly one set   │    │ - Fields allowed?    │
│ - Not both/neither  │    │ - Field types match  │
└─────────────────────┘    └──────────────────────┘
           │                           │
           ▼                           ▼
┌─────────────────────┐    ┌──────────────────────┐
│ Parent Relationship │───▶│ Configuration        │
│ Validation          │    │ Validation Complete  │
│ - Parent type valid │    │ - All checks pass    │
│ - Relationship ok   │    │ - Ready for runtime  │
└─────────────────────┘    └──────────────────────┘
           │
           ▼
┌─────────────────────┐
│ Runtime Validation  │
│ Phase               │
└─────────────────────┘
           │
           ▼
┌─────────────────────┐    ┌──────────────────────┐
│ Parent Existence    │───▶│ Resource Resolution  │
│ Validation          │    │ Validation           │
│ - Parent resolved?  │    │ - Resource found?    │
│ - Parent accessible │    │ - Correct type?      │
└─────────────────────┘    │ - Accessible fields? │
           │                └──────────────────────┘
           ▼                           │
┌─────────────────────┐               ▼
│ Selector Matching   │    ┌──────────────────────┐
│ Validation          │    │ Resolution Success   │
│ - Resources found?  │    │ - Resource resolved  │
│ - Unique result?    │    │ - State updated      │
│ - Fields match?     │    │ - Available for ref  │
└─────────────────────┘    └──────────────────────┘
```

**Validation Stages**:

**1. Configuration Loading Validation** (Step 1 - Already Implemented):
- `ValidateResourceType()`: Checks resource type exists in registry
- `ValidateIDXORSelector()`: Ensures exactly one of ID or selector is specified
- `ValidateSelector()`: Validates selector fields against registry metadata  
- `ValidateParent()`: Validates parent-child relationships

**2. Runtime Resolution Validation**:
- Parent existence validation before child resolution
- Resource existence validation during SDK calls
- Field validation for selector matching
- Uniqueness validation for selector results

**3. Registry Validation**:
- Resource type must exist in ResolutionRegistry
- Parent-child relationships must be in SupportedParents/SupportedChildren
- Selector fields must be in metadata.SelectorFields
- ResolutionAdapter must be non-nil for resolution

**Error Cases**:
- Invalid resource type → Configuration loading error
- Both ID and selector specified → Configuration validation error  
- Parent-child relationship not supported → Configuration validation error
- Parent resolution fails → Runtime error, child resolution blocked
- Multiple resources match selector → Runtime error (ambiguous)
- No resources match selector → Runtime error (not found)
- SDK API call fails → Network/auth/authorization error

## 6. Plan/Apply Integration Flow

Integration with existing plan and apply command execution:

```
┌─────────────────────┐
│ Plan/Apply Command  │
│ Execution Start     │
└─────────────────────┘
           │
           ▼
┌─────────────────────┐    ┌──────────────────────┐
│ Configuration       │───▶│ ResourceSet Loading  │
│ Loading             │    │ - All resource types │
│ - YAML parsing      │    │ - ExternalResources  │
│ - Validation        │    │ - Regular resources  │
└─────────────────────┘    └──────────────────────┘
           │                           │
           ▼                           ▼
┌─────────────────────┐    ┌──────────────────────┐
│ External Resource   │───▶│ Dependency-First     │
│ Resolution Phase    │    │ Resolution           │
│ - BEFORE other      │    │ - Parents first      │
│   resources         │    │ - Children second    │
│ - Read-only phase   │    │ - Parallel siblings  │
└─────────────────────┘    └──────────────────────┘
           │                           │
           ▼                           ▼
┌─────────────────────┐    ┌──────────────────────┐
│ External Resources  │───▶│ Resource Reference   │
│ Available           │    │ Resolution           │
│ - Via .ref field    │    │ - Other resources    │
│ - Resolved IDs      │    │   can reference      │
│ - Resolved data     │    │ - Via external.ref   │
└─────────────────────┘    └──────────────────────┘
           │                           │
           ▼                           ▼
┌─────────────────────┐    ┌──────────────────────┐
│ Plan Phase          │───▶│ Apply Phase          │
│ - Show ext resources│    │ - Skip ext resources │
│ - Show references   │    │   (already resolved) │
│ - Plan other res    │    │ - Apply other res    │
└─────────────────────┘    │ - Use ext ref data   │
           │                └──────────────────────┘
           ▼                           │
┌─────────────────────┐               ▼
│ Plan Output         │    ┌──────────────────────┐
│ External resources: │    │ Apply Success        │
│ ✓ portal-1 (found)  │    │ - All operations ok  │  
│ ✓ api-v1 (found)    │    │ - Ext refs resolved  │
│ Resources to apply: │    │ - Resources applied  │
│ + portal config     │    └──────────────────────┘
│ + api registration  │
└─────────────────────┘
```

**Integration Points**:

**1. Executor Integration** (`/internal/declarative/executor/`):
- External resource resolution happens BEFORE regular resource operations
- External resources are read-only (not created/updated/deleted)
- Other resources can reference external resources via `.ref` field

**2. State Client Integration** (`/internal/declarative/state/client.go`):
- State client extended with resolution methods
- Same client used by both external resolution and regular operations
- Consistent API interface patterns

**3. Resource Reference Resolution**:
- Other resources reference external resources via `.ref` field
- Example: Portal configuration references external portal
- External resource resolved data available during plan/apply

**Execution Order**:
1. **Configuration Loading**: Parse all resources including external
2. **External Resource Resolution**: Resolve all external resources first
3. **Resource Planning**: Plan regular resources with external context
4. **Resource Application**: Apply regular resources with external references

**Plan Output Enhancement**:
```
External Resources:
✓ portal-1 (resolved): Developer Portal [portal-abc123]
✓ api-v1 (resolved): Petstore API v2.0 [api-def456]

Resources to Apply:
+ Portal Configuration (references: portal-1)
+ API Documentation (references: api-v1)  
+ Service Registration (references: api-v1)
```

## 7. Error Handling Flow

Comprehensive error propagation and recovery patterns:

```
┌─────────────────────┐
│ Error Source        │
│ Identification      │
└─────────────────────┘
           │
           ▼
┌─────────────────────┐    ┌──────────────────────┐
│ Configuration       │───▶│ Validation Errors    │
│ Errors              │    │ - Invalid type       │
│ - Parse errors      │    │ - Bad ID/selector    │
│ - Schema errors     │    │ - Bad relationships  │
│ - Validation fails  │    │ - Bad selector fields│
└─────────────────────┘    └──────────────────────┘
           │                           │
           ▼                           ▼
┌─────────────────────┐    ┌──────────────────────┐
│ Registry Errors     │───▶│ Resolution Errors    │
│ - Type not found    │    │ - Parent fails       │
│ - No adapter        │    │ - Resource not found │
│ - Bad relationships │    │ - Multiple matches   │
└─────────────────────┘    │ - SDK failures       │
           │                └──────────────────────┘
           ▼                           │
┌─────────────────────┐               ▼
│ SDK/Network Errors  │    ┌──────────────────────┐
│ - Auth failures     │    │ Error Context        │
│ - Network timeouts  │    │ Propagation          │
│ - API rate limits   │    │ - Error wrapping     │
│ - Resource access   │    │ - Context preserved  │
└─────────────────────┘    │ - Clear messages     │
           │                └──────────────────────┘
           ▼                           │
┌─────────────────────┐               ▼
│ Error Recovery      │    ┌──────────────────────┐
│ Strategies          │    │ User Error           │
│ - Retry on network  │    │ Reporting            │
│ - Fail fast config  │    │ - Clear description  │
│ - Block dependents  │    │ - Actionable info    │
└─────────────────────┘    │ - Error context      │
                           └──────────────────────┘
```

**Error Categories**:

**1. Configuration Errors** (Fail Fast):
```go
// Invalid resource type
return fmt.Errorf("unknown resource type '%s': must be one of %v", 
    resourceType, registry.GetSupportedTypes())

// Invalid parent-child relationship  
return fmt.Errorf("resource type '%s' cannot have parent '%s': supported parents are %v",
    childType, parentType, metadata.SupportedParents)

// Invalid selector fields
return fmt.Errorf("invalid selector field '%s' for resource type '%s': supported fields are %v",
    field, resourceType, metadata.SelectorFields)
```

**2. Registry Errors** (System Issues):
```go
// Resource type not registered
return fmt.Errorf("resource type '%s' not found in registry", resourceType)

// No resolution adapter
return fmt.Errorf("no resolution adapter configured for resource type '%s'", resourceType)
```

**3. Resolution Errors** (Runtime Issues):
```go
// Parent resolution failure
return fmt.Errorf("failed to resolve parent %s (type: %s): %w", 
    parent.Ref, parent.ResourceType, err)

// Resource not found
return fmt.Errorf("resource not found: type=%s, id=%s", resourceType, id)

// Ambiguous selector match
return fmt.Errorf("selector matched %d resources, expected 1: type=%s, selector=%v", 
    len(matches), resourceType, selector)
```

**4. SDK Errors** (External Issues):
```go
// Network/API failures
return fmt.Errorf("SDK call failed for %s: %w", operation, err)

// Authentication failures  
return fmt.Errorf("authentication failed for %s API: %w", resourceType, err)
```

**Error Propagation Pattern**:
1. **Error Wrapping**: All errors wrapped with context using `fmt.Errorf("context: %w", err)`
2. **Early Failure**: Configuration errors prevent resolution from starting
3. **Dependency Blocking**: Parent resolution failure blocks child resolution
4. **Context Preservation**: Error messages include resource type, ID/selector, operation

**Recovery Strategies**:
- **Configuration Errors**: Fix configuration, no automatic recovery
- **Network Errors**: Retry with exponential backoff
- **Resource Not Found**: May be acceptable for optional external resources
- **Authentication Errors**: Re-authenticate, then retry

## 8. Architecture Overview

Complete system architecture showing component interactions:

```
┌─────────────────────────────────────────────────────────────────┐
│                     External Resource Resolution System          │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─────────────────┐    ┌──────────────────────────────────────┐ │
│  │ Configuration   │───▶│           Registry Layer              │ │
│  │ Layer           │    │                                      │ │
│  │                 │    │  ┌─────────────┐  ┌─────────────────┐ │ │
│  │ - YAML Config   │    │  │ Resolution  │  │ Resolution      │ │ │
│  │ - Validation    │    │  │ Registry    │  │ Metadata        │ │ │
│  │ - Parsing       │    │  │ (Singleton) │  │ (Per Type)      │ │ │
│  └─────────────────┘    │  └─────────────┘  └─────────────────┘ │ │
│           │              │                                      │ │
│           │              └──────────────────────────────────────┘ │
│           ▼                               │                       │
│  ┌─────────────────┐                     ▼                       │
│  │ Resolution      │    ┌──────────────────────────────────────┐ │
│  │ Engine          │───▶│           Adapter Layer               │ │
│  │                 │    │                                      │ │
│  │ - Dependency    │    │  ┌─────────────┐  ┌─────────────────┐ │ │
│  │   Analysis      │    │  │ Adapter     │  │ Concrete        │ │ │
│  │ - Parent-Child  │    │  │ Factory     │  │ Adapters        │ │ │
│  │ - Resolution    │    │  │             │  │ (Portal, API,   │ │ │
│  │   Ordering      │    │  └─────────────┘  │  ControlPlane)  │ │ │
│  └─────────────────┘    │                   └─────────────────┘ │ │
│           │              │                                      │ │
│           │              └──────────────────────────────────────┘ │
│           ▼                               │                       │
│  ┌─────────────────┐                     ▼                       │
│  │ State           │    ┌──────────────────────────────────────┐ │
│  │ Management      │───▶│             SDK Layer                 │ │
│  │                 │    │                                      │ │
│  │ - Resolved IDs  │    │  ┌─────────────┐  ┌─────────────────┐ │ │
│  │ - Resolved Data │    │  │ State       │  │ Helper API      │ │ │
│  │ - Status Flags  │    │  │ Client      │  │ Interfaces      │ │ │
│  │                 │    │  │ (Extended)  │  │ (Portal, API,   │ │ │
│  └─────────────────┘    │  └─────────────┘  │  AppAuth, etc.) │ │ │
│                         │                   └─────────────────┘ │ │
│                         │                                      │ │
│                         └──────────────────────────────────────┘ │
│                                          │                       │
└──────────────────────────────────────────┼───────────────────────┘
                                           ▼
             ┌──────────────────────────────────────────────┐
             │              Konnect SDK                     │
             │                                              │
             │  ┌─────────────┐  ┌─────────────────────────┐ │
             │  │ HTTP Client │  │ API Operations          │ │
             │  │ - Auth      │  │ - GetPortal, GetAPI     │ │
             │  │ - Retry     │  │ - ListPortals, ListAPIs │ │
             │  │ - Timeout   │  │ - Child resource calls  │ │
             │  └─────────────┘  └─────────────────────────┘ │
             └──────────────────────────────────────────────┘
                                    │
                                    ▼
             ┌──────────────────────────────────────────────┐
             │                Konnect API                   │
             └──────────────────────────────────────────────┘
```

**Component Interactions**:

**1. Configuration → Registry**:
- YAML configuration validated against registry metadata
- Resource types must exist in registry
- Parent-child relationships validated

**2. Registry → Adapter Layer**:
- Registry provides metadata including adapter instances
- Adapter factory creates concrete implementations
- Adapters registered in registry during initialization

**3. Adapter → SDK Layer**:
- Adapters use state client for SDK operations
- State client wraps helper API interfaces
- Helper interfaces provide SDK method abstraction

**4. SDK → Konnect API**:
- SDK handles HTTP communication, auth, retry logic
- API operations map to REST endpoints
- Responses parsed into Go structs

**Key Design Principles**:
- **Separation of Concerns**: Each layer has distinct responsibility
- **Dependency Injection**: Components receive dependencies at creation
- **Interface Abstraction**: Clear interfaces between layers
- **Registry Pattern**: Centralized resource type management
- **Adapter Pattern**: Pluggable resolution implementations

## 9. Implementation Impact

Files and modifications required for Step 2 implementation:

### New Files to Create

**Adapter Factory and Base**:
```
/internal/declarative/external/adapters/
├── factory.go                 # AdapterFactory with state client injection
├── base_adapter.go           # Common adapter functionality
├── portal_resolution_adapter.go
├── api_resolution_adapter.go
├── control_plane_resolution_adapter.go
├── application_auth_strategy_resolution_adapter.go
├── api_version_resolution_adapter.go
├── api_publication_resolution_adapter.go
├── api_implementation_resolution_adapter.go
├── api_document_resolution_adapter.go
├── portal_customization_resolution_adapter.go
├── portal_custom_domain_resolution_adapter.go
├── portal_page_resolution_adapter.go
└── portal_snippet_resolution_adapter.go
```

**Test Files**:
```
/internal/declarative/external/adapters/
├── factory_test.go
├── portal_resolution_adapter_test.go
├── api_resolution_adapter_test.go
└── [...other adapter test files...]
```

### Files to Modify

**Registry Integration**:
- `/internal/declarative/external/registry.go`
  - Add adapter injection during initialization
  - Update registry creation to include concrete adapters

**State Client Extension**:
- `/internal/declarative/state/client.go`
  - Add resolution methods for each resource type
  - Extend with GetByID and list-filter methods
  - Maintain consistency with existing patterns

**Test Updates**:
- `/internal/declarative/external/registry_test.go`
  - Test adapter registration and retrieval
  - Validate parent-child resolution with adapters

### Implementation Sequence

**Phase 1: Foundation** (Low Risk)
1. Create `base_adapter.go` with common functionality
2. Create `factory.go` with state client injection pattern
3. Update registry to support adapter injection

**Phase 2: Core Adapters** (Medium Risk)
1. Implement top-level adapters (Portal, API, ControlPlane, AppAuthStrategy)
2. Add state client extension methods
3. Test basic ID-based resolution

**Phase 3: Child Adapters** (Medium Risk)
1. Implement child resource adapters with parent context
2. Test parent-child resolution flows
3. Validate selector-based resolution

**Phase 4: Integration** (Low Risk)
1. Integration testing with mock SDK responses
2. End-to-end testing with configuration loading
3. Error handling validation

### Quality Gates

**Build Validation**:
- `make build` - All new files compile successfully
- No circular dependencies introduced
- Interface implementations complete

**Test Validation**:
- `make test` - All unit tests pass
- New adapter tests with >80% coverage
- Registry integration tests pass

**Lint Validation**:
- `make lint` - Zero linting issues
- Consistent error handling patterns
- Proper documentation comments

## Conclusion

The flow analysis reveals a well-architected system built on the solid foundation of Step 1. The Resource Type Registry implementation follows established patterns from the existing codebase while providing clean separation of concerns through the registry and adapter patterns.

**Key Strengths**:
- **Dependency-driven resolution** ensures correct ordering
- **Registry pattern** provides centralized resource type management  
- **Adapter pattern** enables pluggable resolution implementations
- **State client integration** maintains consistency with existing patterns
- **Comprehensive validation** catches errors early

**Implementation Readiness**:
- Foundation is complete and production-ready
- Clear implementation path with low technical risk
- Existing patterns provide reliable implementation templates
- Comprehensive error handling strategies defined

The system is ready for Step 2 implementation with a clear roadmap and well-defined component interactions.