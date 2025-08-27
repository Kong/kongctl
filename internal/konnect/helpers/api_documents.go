package helpers

import (
	"context"
	"fmt"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComponents "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

// APIDocumentAPI defines the interface for operations on API Documents
type APIDocumentAPI interface {
	// API Document operations
	CreateAPIDocument(ctx context.Context, apiID string, request kkComponents.CreateAPIDocumentRequest,
		opts ...kkOps.Option) (*kkOps.CreateAPIDocumentResponse, error)
	UpdateAPIDocument(ctx context.Context, apiID string, documentID string, request kkComponents.APIDocument,
		opts ...kkOps.Option) (*kkOps.UpdateAPIDocumentResponse, error)
	DeleteAPIDocument(ctx context.Context, apiID string, documentID string,
		opts ...kkOps.Option) (*kkOps.DeleteAPIDocumentResponse, error)
	ListAPIDocuments(ctx context.Context, apiID string, filter *kkComponents.APIDocumentFilterParameters,
		opts ...kkOps.Option) (*kkOps.ListAPIDocumentsResponse, error)
	FetchAPIDocument(ctx context.Context, apiID string, documentID string,
		opts ...kkOps.Option) (*kkOps.FetchAPIDocumentResponse, error)
}

// APIDocumentAPIImpl provides an implementation of the APIDocumentAPI interface
type APIDocumentAPIImpl struct {
	SDK *kkSDK.SDK
}

// CreateAPIDocument implements the APIDocumentAPI interface
func (a *APIDocumentAPIImpl) CreateAPIDocument(
	ctx context.Context, apiID string, request kkComponents.CreateAPIDocumentRequest,
	opts ...kkOps.Option,
) (*kkOps.CreateAPIDocumentResponse, error) {
	if a.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}

	if a.SDK.APIDocumentation == nil {
		return nil, fmt.Errorf("SDK.APIDocumentation is nil")
	}
	return a.SDK.APIDocumentation.CreateAPIDocument(ctx, apiID, request, opts...)
}

// UpdateAPIDocument implements the APIDocumentAPI interface
func (a *APIDocumentAPIImpl) UpdateAPIDocument(
	ctx context.Context, apiID string, documentID string, request kkComponents.APIDocument,
	opts ...kkOps.Option,
) (*kkOps.UpdateAPIDocumentResponse, error) {
	if a.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}

	if a.SDK.APIDocumentation == nil {
		return nil, fmt.Errorf("SDK.APIDocumentation is nil")
	}
	req := kkOps.UpdateAPIDocumentRequest{
		APIID:       apiID,
		DocumentID:  documentID,
		APIDocument: request,
	}
	return a.SDK.APIDocumentation.UpdateAPIDocument(ctx, req, opts...)
}

// DeleteAPIDocument implements the APIDocumentAPI interface
func (a *APIDocumentAPIImpl) DeleteAPIDocument(ctx context.Context, apiID string, documentID string,
	opts ...kkOps.Option,
) (*kkOps.DeleteAPIDocumentResponse, error) {
	if a.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}

	if a.SDK.APIDocumentation == nil {
		return nil, fmt.Errorf("SDK.APIDocumentation is nil")
	}
	return a.SDK.APIDocumentation.DeleteAPIDocument(ctx, apiID, documentID, opts...)
}

// FetchAPIDocument implements the APIDocumentAPI interface
func (a *APIDocumentAPIImpl) FetchAPIDocument(ctx context.Context, apiID string, documentID string,
	opts ...kkOps.Option,
) (*kkOps.FetchAPIDocumentResponse, error) {
	if a.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}

	if a.SDK.APIDocumentation == nil {
		return nil, fmt.Errorf("SDK.APIDocumentation is nil")
	}
	return a.SDK.APIDocumentation.FetchAPIDocument(ctx, apiID, documentID, opts...)
}

// ListAPIDocuments implements the APIDocumentAPI interface
func (a *APIDocumentAPIImpl) ListAPIDocuments(ctx context.Context,
	apiID string, filter *kkComponents.APIDocumentFilterParameters,
	opts ...kkOps.Option,
) (*kkOps.ListAPIDocumentsResponse, error) {
	if a.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}

	if a.SDK.APIDocumentation == nil {
		return nil, fmt.Errorf("SDK.APIDocumentation is nil")
	}
	return a.SDK.APIDocumentation.ListAPIDocuments(ctx, apiID, filter, opts...)
}

// GetDocumentsForAPI fetches all document objects for a specific API
func GetDocumentsForAPI(ctx context.Context, kkClient APIDocumentAPI, apiID string) ([]any, error) {
	if kkClient == nil {
		return nil, fmt.Errorf("APIDocumentAPI client is nil")
	}

	// Call the SDK's ListAPIDocuments method
	res, err := kkClient.ListAPIDocuments(ctx, apiID, nil)
	if err != nil {
		return nil, err
	}

	if res == nil {
		return []any{}, nil
	}

	if res.ListAPIDocumentResponse == nil {
		return []any{}, nil
	}

	// Check if we have data in the response
	if len(res.ListAPIDocumentResponse.Data) == 0 {
		return []any{}, nil
	}

	// Convert to []any and return
	result := make([]any, len(res.ListAPIDocumentResponse.Data))
	for i, doc := range res.ListAPIDocumentResponse.Data {
		result[i] = doc
	}

	return result, nil
}
