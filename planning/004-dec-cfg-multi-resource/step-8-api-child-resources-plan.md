# Step 8: API Child Resources Implementation Plan

## Overview

This document outlines the comprehensive plan to properly implement API child 
resources after discovering issues with the initial implementation. The main 
issue was that resource types were not aligned with the SDK structures.

## Core Principles

1. **SDK as Source of Truth**: The Konnect Go SDK defines the authoritative 
   structure for all resources. Our resource types must conform to SDK types.

2. **Consistency**: Implementation patterns established for portals must be 
   followed for APIs and API child resources.

3. **Extensibility**: Add necessary interfaces, types, and functions to support 
   the resource patterns we've established.

## API Child Resources to Implement

Based on SDK research:

1. **API Versions** (`/v3/apis/{apiId}/versions`)
   - SDK Operations: CreateAPIVersion, ListAPIVersions
   - No Update or Delete operations in SDK
   - Key fields: Version (string), PublishStatus, Deprecated, SunsetDate, Spec

2. **API Publications** (`/v3/apis/{apiId}/api-publications`)
   - SDK Operations: CreateAPIPublication, DeleteAPIPublication, ListAPIPublications
   - No Update operation
   - Key fields: PortalID, AuthStrategyIds, AutoApproveRegistrations, Visibility

3. **API Implementations** (`/v3/apis/{apiId}/implementations`)
   - SDK Operations: CreateAPIImplementation, DeleteAPIImplementation, ListAPIImplementations
   - No Update operation
   - Key fields: ImplementationURL, Service (with ID and ControlPlaneID)

4. **API Documents** (`/v3/apis/{apiId}/documents`)
   - SDK Operations: CreateAPIDocument, UpdateAPIDocument, DeleteAPIDocument, GetAPIDocument, ListAPIDocuments
   - Full CRUD support
   - Key fields: Metadata (map), Content, ParentDocumentID

5. **API Attributes** (Part of API resource)
   - Not a separate resource - attributes like labels are fields on the API itself
   - No separate CRUD operations needed

## Implementation Phases

### Phase 1: Create Missing Resource Types

1. Create `internal/declarative/resources/api_document.go`:
   - Embed `kkComps.CreateAPIDocumentRequestInput`
   - Add Ref, API, and Kongctl fields
   - Implement all resource interfaces

### Phase 2: Fix Existing Resource Types

1. **api_version.go**:
   - Remove invalid `Name` field
   - Remove `GatewayService` field
   - Align with `CreateAPIVersionRequest` from SDK

2. **api_publication.go**:
   - Already correctly structured
   - Verify against SDK types

3. **api_implementation.go**:
   - Already correctly structured
   - Verify Service field mapping

### Phase 3: Extend Helper Interfaces

1. Update `internal/konnect/helpers/apis.go`:
   ```go
   type APIAPI interface {
       // Existing methods...
       
       // API Version operations
       CreateAPIVersion(ctx, apiID string, request kkComps.CreateAPIVersionRequest, opts) (*kkOps.CreateAPIVersionResponse, error)
       ListAPIVersions(ctx, request kkOps.ListAPIVersionsRequest, opts) (*kkOps.ListAPIVersionsResponse, error)
       
       // API Publication operations
       CreateAPIPublication(ctx, apiID string, request kkComps.CreateAPIPublicationRequestInput, opts) (*kkOps.CreateAPIPublicationResponse, error)
       DeleteAPIPublication(ctx, apiID, publicationID string, opts) (*kkOps.DeleteAPIPublicationResponse, error)
       ListAPIPublications(ctx, request kkOps.ListAPIPublicationsRequest, opts) (*kkOps.ListAPIPublicationsResponse, error)
       
       // API Implementation operations
       CreateAPIImplementation(ctx, apiID string, request kkComps.CreateAPIImplementationRequestInput, opts) (*kkOps.CreateAPIImplementationResponse, error)
       DeleteAPIImplementation(ctx, apiID, implementationID string, opts) (*kkOps.DeleteAPIImplementationResponse, error)
       ListAPIImplementations(ctx, request kkOps.ListAPIImplementationsRequest, opts) (*kkOps.ListAPIImplementationsResponse, error)
       
       // API Document operations
       CreateAPIDocument(ctx, apiID string, request kkComps.CreateAPIDocumentRequestInput, opts) (*kkOps.CreateAPIDocumentResponse, error)
       UpdateAPIDocument(ctx, apiID, documentID string, request kkComps.UpdateAPIDocumentRequestInput, opts) (*kkOps.UpdateAPIDocumentResponse, error)
       DeleteAPIDocument(ctx, apiID, documentID string, opts) (*kkOps.DeleteAPIDocumentResponse, error)
       ListAPIDocuments(ctx, request kkOps.ListAPIDocumentsRequest, opts) (*kkOps.ListAPIDocumentsResponse, error)
   }
   ```

2. Create separate helper files for each child resource type (optional):
   - `apiversion.go`
   - `apipublication.go`
   - `apiimplementation.go` (already exists)
   - `apidocument.go`

### Phase 4: Implement State Client Support

1. Add types to `internal/declarative/state/client.go`:
   ```go
   type APIVersion struct {
       ID            string
       Version       string
       PublishStatus string
       Deprecated    bool
       // ... other SDK fields
   }
   
   type APIPublication struct {
       ID       string
       PortalID string
       // ... other SDK fields
   }
   
   type APIImplementation struct {
       ID                string
       ImplementationURL string
       Service           *Service
       // ... other SDK fields
   }
   
   type APIDocument struct {
       ID       string
       Metadata map[string]string
       Content  string
       // ... other SDK fields
   }
   ```

2. Add methods for each child resource type:
   - `ListAPIVersions(ctx, apiID string) ([]APIVersion, error)`
   - `CreateAPIVersion(ctx, apiID string, version APIVersion) (*APIVersion, error)`
   - Similar methods for Publications, Implementations, Documents

### Phase 5: Complete Planner Implementation

1. Update `api_planner.go`:
   - Implement `planAPIChildResourcesCreate` properly
   - Add separate planning functions for each child type
   - Handle the fact that some resources don't support Update

2. Add child resource planning logic:
   ```go
   func (p *Planner) planAPIVersionChanges(ctx context.Context, apiID string, desired []resources.APIVersionResource, plan *Plan) error
   func (p *Planner) planAPIPublicationChanges(ctx context.Context, apiID string, desired []resources.APIPublicationResource, plan *Plan) error
   func (p *Planner) planAPIImplementationChanges(ctx context.Context, apiID string, desired []resources.APIImplementationResource, plan *Plan) error
   func (p *Planner) planAPIDocumentChanges(ctx context.Context, apiID string, desired []resources.APIDocumentResource, plan *Plan) error
   ```

### Phase 6: Add Executor Operations

1. Create operation files:
   - `api_version_operations.go`
   - `api_publication_operations.go`
   - `api_implementation_operations.go`
   - `api_document_operations.go`

2. Register operations in executor:
   - Note: Some only need CREATE/DELETE (no UPDATE)

### Phase 7: Testing

1. Unit tests for:
   - Resource type validation
   - Helper interface implementations
   - State client methods
   - Planner logic
   - Executor operations

2. Integration tests for:
   - End-to-end API with child resources
   - Error handling
   - Edge cases

## Key Considerations

1. **No Update Operations**: Several child resources (Versions, Publications, 
   Implementations) don't support updates in the SDK. The planner must handle 
   this by planning DELETE + CREATE when changes are detected.

2. **Parent-Child Relationships**: All child resources require the parent API 
   ID. The planner must ensure proper ordering of operations.

3. **Reference Resolution**: Child resources can reference other resources 
   (e.g., Publications reference Portals and AuthStrategies).

4. **Label Handling**: Only parent API resources have labels. Child resources 
   don't support labels per the SDK.

## Success Criteria

1. All child resource types properly aligned with SDK structures
2. Helper interfaces extended with all necessary operations
3. State client supports all child resource operations
4. Planner correctly handles all child resources
5. Executor can perform all operations
6. Tests pass for all components
7. Integration tests demonstrate end-to-end functionality