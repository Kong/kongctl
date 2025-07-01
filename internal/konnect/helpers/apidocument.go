package helpers

import (
	"context"
	"fmt"
	"os"

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
	// Handle debugging based on environment variable
	debugEnabled := os.Getenv("KONGCTL_DEBUG") == EnvTrue

	// Helper function for debug logging
	debugLog := func(format string, args ...interface{}) {
		if debugEnabled {
			fmt.Fprintf(os.Stderr, "DEBUG: "+format+"\n", args...)
		}
	}

	if a.SDK == nil {
		debugLog("PublicAPIDocumentAPI.SDK is nil")
		return nil, fmt.Errorf("SDK is nil")
	}

	if a.SDK.APIDocumentation == nil {
		debugLog("PublicAPIDocumentAPI.SDK.APIDocumentation is nil")
		return nil, fmt.Errorf("SDK.APIDocumentation is nil")
	}

	debugLog("Calling a.SDK.APIDocumentation.ListAPIDocuments")
	return a.SDK.APIDocumentation.ListAPIDocuments(ctx, apiID, filter, opts...)
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

	// Call the SDK's ListAPIDocuments method
	debugLog("Calling ListAPIDocuments...")
	res, err := kkClient.ListAPIDocuments(ctx, apiID, nil)
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