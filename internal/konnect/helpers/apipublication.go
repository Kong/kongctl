package helpers

import (
	"context"
	"fmt"
	"os"

	kkInternal "github.com/Kong/sdk-konnect-go-internal"
	kkInternalComponents "github.com/Kong/sdk-konnect-go-internal/models/components"
	kkInternalOps "github.com/Kong/sdk-konnect-go-internal/models/operations"
)

// APIPublicationAPI defines the interface for operations on API Publications
type APIPublicationAPI interface {
	// API Publication operations
	ListAPIPublications(ctx context.Context, request kkInternalOps.ListAPIPublicationsRequest, opts ...kkInternalOps.Option) (*kkInternalOps.ListAPIPublicationsResponse, error)
}

// InternalAPIPublicationAPI provides an implementation of the APIPublicationAPI interface using the internal SDK
type InternalAPIPublicationAPI struct {
	SDK *kkInternal.SDK
}

// ListAPIPublications implements the APIPublicationAPI interface
func (a *InternalAPIPublicationAPI) ListAPIPublications(ctx context.Context, request kkInternalOps.ListAPIPublicationsRequest, opts ...kkInternalOps.Option) (*kkInternalOps.ListAPIPublicationsResponse, error) {
	// Handle debugging based on environment variable
	debugEnabled := os.Getenv("KONGCTL_DEBUG") == "true"

	// Helper function for debug logging
	debugLog := func(format string, args ...interface{}) {
		if debugEnabled {
			fmt.Fprintf(os.Stderr, "DEBUG: "+format+"\n", args...)
		}
	}
	
	if a.SDK == nil {
		debugLog("InternalAPIPublicationAPI.SDK is nil")
		return nil, fmt.Errorf("SDK is nil")
	}
	
	if a.SDK.APIPublication == nil {
		debugLog("InternalAPIPublicationAPI.SDK.APIPublication is nil")
		return nil, fmt.Errorf("SDK.APIPublication is nil")
	}
	
	debugLog("Calling a.SDK.APIPublication.ListAPIPublications")
	return a.SDK.APIPublication.ListAPIPublications(ctx, request, opts...)
}

// GetPublicationsForAPI fetches all publication objects for a specific API
func GetPublicationsForAPI(ctx context.Context, kkClient APIPublicationAPI, apiID string) ([]interface{}, error) {
	// We need to handle debugging differently here because this is in a separate package
	// Check if debug flag is set in environment
	debugEnabled := os.Getenv("KONGCTL_DEBUG") == "true"
	
	// Helper function for debug logging
	debugLog := func(format string, args ...interface{}) {
		if debugEnabled {
			fmt.Fprintf(os.Stderr, "DEBUG: "+format+"\n", args...)
		}
	}
	
	debugLog("GetPublicationsForAPI called with API ID: %s", apiID)
	
	if kkClient == nil {
		debugLog("APIPublicationAPI client is nil")
		return nil, fmt.Errorf("APIPublicationAPI client is nil")
	}
	
	// Create a filter to get publications for this API
	apiIDFilter := &kkInternalComponents.UUIDFieldFilter{
		Eq: &apiID,
	}
	
	// Create a request to list API publications for this API
	req := kkInternalOps.ListAPIPublicationsRequest{
		Filter: &kkInternalComponents.APIPublicationFilterParameters{
			APIID: apiIDFilter,
		},
	}
	debugLog("Created ListAPIPublicationsRequest with API ID filter: %s", apiID)

	// Call the SDK's ListAPIPublications method
	debugLog("Calling ListAPIPublications...")
	res, err := kkClient.ListAPIPublications(ctx, req)
	
	if err != nil {
		debugLog("Error from ListAPIPublications: %v", err)
		return nil, err
	}
	
	debugLog("ListAPIPublications returned successfully")
	
	if res == nil {
		debugLog("Response is nil")
		return []interface{}{}, nil
	}
	
	if res.ListAPIPublicationResponse == nil {
		debugLog("ListAPIPublicationResponse is nil")
		return []interface{}{}, nil
	}
	
	debugLog("ListAPIPublicationResponse has %d items", len(res.ListAPIPublicationResponse.Data))

	// Check if we have data in the response
	if len(res.ListAPIPublicationResponse.Data) == 0 {
		debugLog("No publications found for API %s", apiID)
		return []interface{}{}, nil
	}

	// Convert to []interface{} and return
	result := make([]interface{}, len(res.ListAPIPublicationResponse.Data))
	for i, pub := range res.ListAPIPublicationResponse.Data {
		result[i] = pub
		debugLog("Added publication %d to result", i)
	}
	
	debugLog("Returning %d publications", len(result))
	return result, nil
}