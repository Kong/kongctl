package helpers

import (
	"context"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

// APIAPI defines the interface for operations on APIs
type APIAPI interface {
	// API operations
	ListApis(ctx context.Context, request kkOps.ListApisRequest,
		opts ...kkOps.Option) (*kkOps.ListApisResponse, error)
	FetchAPI(ctx context.Context, apiID string,
		opts ...kkOps.Option) (*kkOps.FetchAPIResponse, error)
	CreateAPI(ctx context.Context, request kkComps.CreateAPIRequest,
		opts ...kkOps.Option) (*kkOps.CreateAPIResponse, error)
	UpdateAPI(ctx context.Context, apiID string, request kkComps.UpdateAPIRequest,
		opts ...kkOps.Option) (*kkOps.UpdateAPIResponse, error)
	DeleteAPI(ctx context.Context, apiID string,
		opts ...kkOps.Option) (*kkOps.DeleteAPIResponse, error)
	
	// API Version operations
	CreateAPIVersion(ctx context.Context, apiID string, request kkComps.CreateAPIVersionRequest,
		opts ...kkOps.Option) (*kkOps.CreateAPIVersionResponse, error)
	ListAPIVersions(ctx context.Context, request kkOps.ListAPIVersionsRequest,
		opts ...kkOps.Option) (*kkOps.ListAPIVersionsResponse, error)
	UpdateAPIVersion(ctx context.Context, request kkOps.UpdateAPIVersionRequest,
		opts ...kkOps.Option) (*kkOps.UpdateAPIVersionResponse, error)
	DeleteAPIVersion(ctx context.Context, request kkOps.DeleteAPIVersionRequest,
		opts ...kkOps.Option) (*kkOps.DeleteAPIVersionResponse, error)
	
	// API Publication operations
	PublishAPIToPortal(ctx context.Context, request kkOps.PublishAPIToPortalRequest,
		opts ...kkOps.Option) (*kkOps.PublishAPIToPortalResponse, error)
	DeletePublication(ctx context.Context, apiID string, portalID string,
		opts ...kkOps.Option) (*kkOps.DeletePublicationResponse, error)
	ListAPIPublications(ctx context.Context, request kkOps.ListAPIPublicationsRequest,
		opts ...kkOps.Option) (*kkOps.ListAPIPublicationsResponse, error)
	
	// API Implementation operations
	// Note: SDK does not support create/update operations for implementations
	ListAPIImplementations(ctx context.Context, request kkOps.ListAPIImplementationsRequest,
		opts ...kkOps.Option) (*kkOps.ListAPIImplementationsResponse, error)
	
	// API Document operations
	CreateAPIDocument(ctx context.Context, apiID string, request kkComps.CreateAPIDocumentRequest,
		opts ...kkOps.Option) (*kkOps.CreateAPIDocumentResponse, error)
	UpdateAPIDocument(ctx context.Context, apiID string, documentID string, request kkComps.APIDocument,
		opts ...kkOps.Option) (*kkOps.UpdateAPIDocumentResponse, error)
	DeleteAPIDocument(ctx context.Context, apiID string, documentID string,
		opts ...kkOps.Option) (*kkOps.DeleteAPIDocumentResponse, error)
	ListAPIDocuments(ctx context.Context, request kkOps.ListAPIDocumentsRequest,
		opts ...kkOps.Option) (*kkOps.ListAPIDocumentsResponse, error)
	FetchAPIDocument(ctx context.Context, apiID string, documentID string,
		opts ...kkOps.Option) (*kkOps.FetchAPIDocumentResponse, error)
}

// PublicAPIAPI provides an implementation of the APIAPI interface using the public SDK
type PublicAPIAPI struct {
	SDK *kkSDK.SDK
}

// ListApis implements the APIAPI interface
func (a *PublicAPIAPI) ListApis(ctx context.Context, request kkOps.ListApisRequest,
	opts ...kkOps.Option,
) (*kkOps.ListApisResponse, error) {
	return a.SDK.API.ListApis(ctx, request, opts...)
}

// FetchAPI implements the APIAPI interface
func (a *PublicAPIAPI) FetchAPI(ctx context.Context, apiID string,
	opts ...kkOps.Option,
) (*kkOps.FetchAPIResponse, error) {
	return a.SDK.API.FetchAPI(ctx, apiID, opts...)
}

// CreateAPI implements the APIAPI interface
func (a *PublicAPIAPI) CreateAPI(ctx context.Context, request kkComps.CreateAPIRequest,
	opts ...kkOps.Option,
) (*kkOps.CreateAPIResponse, error) {
	return a.SDK.API.CreateAPI(ctx, request, opts...)
}

// UpdateAPI implements the APIAPI interface
func (a *PublicAPIAPI) UpdateAPI(ctx context.Context, apiID string, request kkComps.UpdateAPIRequest,
	opts ...kkOps.Option,
) (*kkOps.UpdateAPIResponse, error) {
	return a.SDK.API.UpdateAPI(ctx, apiID, request, opts...)
}

// DeleteAPI implements the APIAPI interface
func (a *PublicAPIAPI) DeleteAPI(ctx context.Context, apiID string,
	opts ...kkOps.Option,
) (*kkOps.DeleteAPIResponse, error) {
	return a.SDK.API.DeleteAPI(ctx, apiID, opts...)
}

// API Version operations

// CreateAPIVersion implements the APIAPI interface
func (a *PublicAPIAPI) CreateAPIVersion(ctx context.Context, apiID string, request kkComps.CreateAPIVersionRequest,
	opts ...kkOps.Option,
) (*kkOps.CreateAPIVersionResponse, error) {
	return a.SDK.APIVersion.CreateAPIVersion(ctx, apiID, request, opts...)
}

// ListAPIVersions implements the APIAPI interface
func (a *PublicAPIAPI) ListAPIVersions(ctx context.Context, request kkOps.ListAPIVersionsRequest,
	opts ...kkOps.Option,
) (*kkOps.ListAPIVersionsResponse, error) {
	return a.SDK.APIVersion.ListAPIVersions(ctx, request, opts...)
}

// UpdateAPIVersion implements the APIAPI interface
func (a *PublicAPIAPI) UpdateAPIVersion(ctx context.Context, request kkOps.UpdateAPIVersionRequest,
	opts ...kkOps.Option,
) (*kkOps.UpdateAPIVersionResponse, error) {
	return a.SDK.APIVersion.UpdateAPIVersion(ctx, request, opts...)
}

// DeleteAPIVersion implements the APIAPI interface
func (a *PublicAPIAPI) DeleteAPIVersion(ctx context.Context, request kkOps.DeleteAPIVersionRequest,
	opts ...kkOps.Option,
) (*kkOps.DeleteAPIVersionResponse, error) {
	return a.SDK.APIVersion.DeleteAPIVersion(ctx, request.APIID, request.SpecID, opts...)
}

// API Publication operations

// PublishAPIToPortal implements the APIAPI interface
func (a *PublicAPIAPI) PublishAPIToPortal(ctx context.Context, request kkOps.PublishAPIToPortalRequest,
	opts ...kkOps.Option,
) (*kkOps.PublishAPIToPortalResponse, error) {
	return a.SDK.APIPublication.PublishAPIToPortal(ctx, request, opts...)
}

// DeletePublication implements the APIAPI interface
func (a *PublicAPIAPI) DeletePublication(ctx context.Context, apiID string, portalID string,
	opts ...kkOps.Option,
) (*kkOps.DeletePublicationResponse, error) {
	return a.SDK.APIPublication.DeletePublication(ctx, apiID, portalID, opts...)
}

// ListAPIPublications implements the APIAPI interface
func (a *PublicAPIAPI) ListAPIPublications(ctx context.Context, request kkOps.ListAPIPublicationsRequest,
	opts ...kkOps.Option,
) (*kkOps.ListAPIPublicationsResponse, error) {
	return a.SDK.APIPublication.ListAPIPublications(ctx, request, opts...)
}

// API Implementation operations

// ListAPIImplementations implements the APIAPI interface
// Note: Implementation management is not yet available in the SDK
func (a *PublicAPIAPI) ListAPIImplementations(ctx context.Context, request kkOps.ListAPIImplementationsRequest,
	opts ...kkOps.Option,
) (*kkOps.ListAPIImplementationsResponse, error) {
	// The implementation operations are already in the apiimplementation.go helper
	return a.SDK.APIImplementation.ListAPIImplementations(ctx, request, opts...)
}


// API Document operations

// CreateAPIDocument implements the APIAPI interface
func (a *PublicAPIAPI) CreateAPIDocument(ctx context.Context, apiID string, request kkComps.CreateAPIDocumentRequest,
	opts ...kkOps.Option,
) (*kkOps.CreateAPIDocumentResponse, error) {
	return a.SDK.APIDocumentation.CreateAPIDocument(ctx, apiID, request, opts...)
}

// UpdateAPIDocument implements the APIAPI interface
func (a *PublicAPIAPI) UpdateAPIDocument(
	ctx context.Context, apiID string, documentID string, request kkComps.APIDocument,
	opts ...kkOps.Option,
) (*kkOps.UpdateAPIDocumentResponse, error) {
	req := kkOps.UpdateAPIDocumentRequest{
		APIID:       apiID,
		DocumentID:  documentID,
		APIDocument: request,
	}
	return a.SDK.APIDocumentation.UpdateAPIDocument(ctx, req, opts...)
}

// DeleteAPIDocument implements the APIAPI interface
func (a *PublicAPIAPI) DeleteAPIDocument(ctx context.Context, apiID string, documentID string,
	opts ...kkOps.Option,
) (*kkOps.DeleteAPIDocumentResponse, error) {
	return a.SDK.APIDocumentation.DeleteAPIDocument(ctx, apiID, documentID, opts...)
}

// ListAPIDocuments implements the APIAPI interface
func (a *PublicAPIAPI) ListAPIDocuments(ctx context.Context, request kkOps.ListAPIDocumentsRequest,
	opts ...kkOps.Option,
) (*kkOps.ListAPIDocumentsResponse, error) {
	// The SDK method has different signature
	return a.SDK.APIDocumentation.ListAPIDocuments(ctx, request.APIID, nil, opts...)
}

// FetchAPIDocument implements the APIAPI interface
func (a *PublicAPIAPI) FetchAPIDocument(ctx context.Context, apiID string, documentID string,
	opts ...kkOps.Option,
) (*kkOps.FetchAPIDocumentResponse, error) {
	return a.SDK.APIDocumentation.FetchAPIDocument(ctx, apiID, documentID, opts...)
}

// GetDocumentationsForAPI is a deprecated function
// Use GetDocumentsForAPI from the apidocumentation.go file instead
func GetDocumentationsForAPI(_ context.Context, _ APIAPI, _ string) ([]interface{}, error) {
	return []interface{}{}, nil
}