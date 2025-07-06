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
	ListAPIDocuments(ctx context.Context, apiID string, filter *kkComponents.APIDocumentFilterParameters,
		opts ...kkOps.Option) (*kkOps.ListAPIDocumentsResponse, error)
}

// PublicAPIDocumentAPI provides an implementation of the APIDocumentAPI interface using the public SDK
type PublicAPIDocumentAPI struct {
	SDK *kkSDK.SDK
}

// ListAPIDocuments implements the APIDocumentAPI interface
func (a *PublicAPIDocumentAPI) ListAPIDocuments(ctx context.Context,
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
func GetDocumentsForAPI(ctx context.Context, kkClient APIDocumentAPI, apiID string) ([]interface{}, error) {
	if kkClient == nil {
		return nil, fmt.Errorf("APIDocumentAPI client is nil")
	}

	// Call the SDK's ListAPIDocuments method
	res, err := kkClient.ListAPIDocuments(ctx, apiID, nil)
	if err != nil {
		return nil, err
	}

	if res == nil {
		return []interface{}{}, nil
	}

	if res.ListAPIDocumentResponse == nil {
		return []interface{}{}, nil
	}

	// Check if we have data in the response
	if len(res.ListAPIDocumentResponse.Data) == 0 {
		return []interface{}{}, nil
	}

	// Convert to []interface{} and return
	result := make([]interface{}, len(res.ListAPIDocumentResponse.Data))
	for i, doc := range res.ListAPIDocumentResponse.Data {
		result[i] = doc
	}

	return result, nil
}