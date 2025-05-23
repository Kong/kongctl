package helpers

import (
	"context"
	"fmt"
	"os"

	kkInternal "github.com/Kong/sdk-konnect-go-internal"
	kkInternalOps "github.com/Kong/sdk-konnect-go-internal/models/operations"
)

// APIDocumentAPI defines the interface for operations on API Documents
type APIDocumentAPI interface {
	// API Document operations
	ListAPIDocuments(ctx context.Context, request kkInternalOps.ListAPIDocumentsRequest,
		opts ...kkInternalOps.Option) (*kkInternalOps.ListAPIDocumentsResponse, error)
}

// InternalAPIDocumentAPI provides an implementation of the APIDocumentAPI interface using the internal SDK
type InternalAPIDocumentAPI struct {
	SDK *kkInternal.SDK
}

// ListAPIDocuments implements the APIDocumentAPI interface
func (a *InternalAPIDocumentAPI) ListAPIDocuments(ctx context.Context,
	request kkInternalOps.ListAPIDocumentsRequest,
	opts ...kkInternalOps.Option) (*kkInternalOps.ListAPIDocumentsResponse, error) {
	// Handle debugging based on environment variable
	debugEnabled := os.Getenv("KONGCTL_DEBUG") == EnvTrue

	// Helper function for debug logging
	debugLog := func(format string, args ...interface{}) {
		if debugEnabled {
			fmt.Fprintf(os.Stderr, "DEBUG: "+format+"\n", args...)
		}
	}
	
	if a.SDK == nil {
		debugLog("InternalAPIDocumentAPI.SDK is nil")
		return nil, fmt.Errorf("SDK is nil")
	}
	
	if a.SDK.APIDocumentation == nil {
		debugLog("InternalAPIDocumentAPI.SDK.APIDocumentation is nil")
		return nil, fmt.Errorf("SDK.APIDocumentation is nil")
	}
	
	debugLog("Calling a.SDK.APIDocumentation.ListAPIDocuments")
	return a.SDK.APIDocumentation.ListAPIDocuments(ctx, request, opts...)
}

// GetDocumentsForAPI fetches all document objects for a specific API
func GetDocumentsForAPI(ctx context.Context, kkClient APIDocumentAPI, apiID string) ([]interface{}, error) {
	// We need to handle debugging differently here because this is in a separate package
	// Check if debug flag is set in environment
	debugEnabled := os.Getenv("KONGCTL_DEBUG") == EnvTrue
	
	// Helper function for debug logging
	debugLog := func(format string, args ...interface{}) {
		if debugEnabled {
			fmt.Fprintf(os.Stderr, "DEBUG: "+format+"\n", args...)
		}
	}
	
	debugLog("GetDocumentsForAPI called with API ID: %s", apiID)
	
	if kkClient == nil {
		debugLog("APIDocumentAPI client is nil")
		return nil, fmt.Errorf("APIDocumentAPI client is nil")
	}
	
	// Create a request to list API documents for this API
	req := kkInternalOps.ListAPIDocumentsRequest{
		APIID: apiID,
	}
	debugLog("Created ListAPIDocumentsRequest with APIID: %s", apiID)

	// Call the SDK's ListAPIDocuments method
	debugLog("Calling ListAPIDocuments...")
	res, err := kkClient.ListAPIDocuments(ctx, req)
	
	if err != nil {
		debugLog("Error from ListAPIDocuments: %v", err)
		return nil, err
	}
	
	debugLog("ListAPIDocuments returned successfully")
	
	if res == nil {
		debugLog("Response is nil")
		return []interface{}{}, nil
	}
	
	if res.ListAPIDocumentResponse == nil {
		debugLog("ListAPIDocumentResponse is nil")
		return []interface{}{}, nil
	}
	
	debugLog("ListAPIDocumentResponse has %d items", len(res.ListAPIDocumentResponse.Data))

	// Check if we have data in the response
	if len(res.ListAPIDocumentResponse.Data) == 0 {
		debugLog("No documents found for API %s", apiID)
		return []interface{}{}, nil
	}

	// Convert to []interface{} and return
	result := make([]interface{}, len(res.ListAPIDocumentResponse.Data))
	for i, doc := range res.ListAPIDocumentResponse.Data {
		result[i] = doc
		debugLog("Added document %d to result", i)
	}
	
	debugLog("Returning %d documents", len(result))
	return result, nil
}