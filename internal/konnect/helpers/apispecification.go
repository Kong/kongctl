package helpers

import (
	"context"
	"fmt"
	"os"

	kkInternal "github.com/Kong/sdk-konnect-go-internal"
	kkInternalOps "github.com/Kong/sdk-konnect-go-internal/models/operations"
)

// APISpecificationAPI defines the interface for operations on API Specifications
type APISpecificationAPI interface {
	// API Specification operations
	ListAPISpecs(ctx context.Context, request kkInternalOps.ListAPISpecsRequest, opts ...kkInternalOps.Option) (*kkInternalOps.ListAPISpecsResponse, error)
}

// InternalAPISpecificationAPI provides an implementation of the APISpecificationAPI interface using the internal SDK
type InternalAPISpecificationAPI struct {
	SDK *kkInternal.SDK
}

// ListAPISpecs implements the APISpecificationAPI interface
func (a *InternalAPISpecificationAPI) ListAPISpecs(ctx context.Context, request kkInternalOps.ListAPISpecsRequest, opts ...kkInternalOps.Option) (*kkInternalOps.ListAPISpecsResponse, error) {
	// Handle debugging based on environment variable
	debugEnabled := os.Getenv("KONGCTL_DEBUG") == "true"
	
	// Helper function for debug logging
	debugLog := func(format string, args ...interface{}) {
		if debugEnabled {
			fmt.Fprintf(os.Stderr, "DEBUG: "+format+"\n", args...)
		}
	}
	
	if a.SDK == nil {
		debugLog("InternalAPISpecificationAPI.SDK is nil")
		return nil, fmt.Errorf("SDK is nil")
	}
	
	if a.SDK.APISpecification == nil {
		debugLog("InternalAPISpecificationAPI.SDK.APISpecification is nil")
		return nil, fmt.Errorf("SDK.APISpecification is nil")
	}
	
	debugLog("Calling a.SDK.APISpecification.ListAPISpecs")
	return a.SDK.APISpecification.ListAPISpecs(ctx, request, opts...)
}

// GetSpecificationsForAPI fetches all specification objects for a specific API
func GetSpecificationsForAPI(ctx context.Context, kkClient APISpecificationAPI, apiID string) ([]interface{}, error) {
	// We need to handle debugging differently here because this is in a separate package
	// Check if debug flag is set in environment
	debugEnabled := os.Getenv("KONGCTL_DEBUG") == "true"
	
	// Helper function for debug logging
	debugLog := func(format string, args ...interface{}) {
		if debugEnabled {
			fmt.Fprintf(os.Stderr, "DEBUG: "+format+"\n", args...)
		}
	}
	
	debugLog("GetSpecificationsForAPI called with API ID: %s", apiID)
	
	if kkClient == nil {
		debugLog("APISpecificationAPI client is nil")
		return nil, fmt.Errorf("APISpecificationAPI client is nil")
	}
	
	// Create a request to list API specifications for this API
	req := kkInternalOps.ListAPISpecsRequest{
		APIID: apiID,
	}
	debugLog("Created ListAPISpecsRequest with APIID: %s", apiID)

	// Call the SDK's ListAPISpecs method
	debugLog("Calling ListAPISpecs...")
	res, err := kkClient.ListAPISpecs(ctx, req)
	
	if err != nil {
		debugLog("Error from ListAPISpecs: %v", err)
		return nil, err
	}
	
	debugLog("ListAPISpecs returned successfully")
	
	if res == nil {
		debugLog("Response is nil")
		return []interface{}{}, nil
	}
	
	if res.ListAPISpecResponse == nil {
		debugLog("ListAPISpecResponse is nil")
		return []interface{}{}, nil
	}
	
	debugLog("ListAPISpecResponse has %d items", len(res.ListAPISpecResponse.Data))

	// Check if we have data in the response
	if len(res.ListAPISpecResponse.Data) == 0 {
		debugLog("No specifications found for API %s", apiID)
		return []interface{}{}, nil
	}

	// Convert to []interface{} and return
	result := make([]interface{}, len(res.ListAPISpecResponse.Data))
	for i, spec := range res.ListAPISpecResponse.Data {
		result[i] = spec
		debugLog("Added specification %d to result", i)
	}
	
	debugLog("Returning %d specifications", len(result))
	return result, nil
}