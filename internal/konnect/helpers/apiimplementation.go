package helpers

import (
	"context"
	"fmt"
	"os"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComponents "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

// APIImplementationAPI defines the interface for operations on API Implementations
type APIImplementationAPI interface {
	// API Implementation operations
	ListAPIImplementations(ctx context.Context, request kkOps.ListAPIImplementationsRequest,
		opts ...kkOps.Option) (*kkOps.ListAPIImplementationsResponse, error)
}

// PublicAPIImplementationAPI provides an implementation of the APIImplementationAPI interface using the public SDK
type PublicAPIImplementationAPI struct {
	SDK *kkSDK.SDK
}

// ListAPIImplementations implements the APIImplementationAPI interface
func (a *PublicAPIImplementationAPI) ListAPIImplementations(ctx context.Context,
	request kkOps.ListAPIImplementationsRequest,
	opts ...kkOps.Option,
) (*kkOps.ListAPIImplementationsResponse, error) {
	// Handle debugging based on environment variable
	debugEnabled := os.Getenv("KONGCTL_DEBUG") == EnvTrue

	// Helper function for debug logging
	debugLog := func(format string, args ...interface{}) {
		if debugEnabled {
			fmt.Fprintf(os.Stderr, "DEBUG: "+format+"\n", args...)
		}
	}

	if a.SDK == nil {
		debugLog("PublicAPIImplementationAPI.SDK is nil")
		return nil, fmt.Errorf("SDK is nil")
	}

	if a.SDK.APIImplementation == nil {
		debugLog("PublicAPIImplementationAPI.SDK.APIImplementation is nil")
		return nil, fmt.Errorf("SDK.APIImplementation is nil")
	}

	debugLog("Calling a.SDK.APIImplementation.ListAPIImplementations")
	return a.SDK.APIImplementation.ListAPIImplementations(ctx, request, opts...)
}

// GetImplementationsForAPI fetches all implementation objects for a specific API
func GetImplementationsForAPI(ctx context.Context, kkClient APIImplementationAPI, apiID string) ([]interface{}, error) {
	// We need to handle debugging differently here because this is in a separate package
	// Check if debug flag is set in environment
	debugEnabled := os.Getenv("KONGCTL_DEBUG") == EnvTrue

	// Helper function for debug logging
	debugLog := func(format string, args ...interface{}) {
		if debugEnabled {
			fmt.Fprintf(os.Stderr, "DEBUG: "+format+"\n", args...)
		}
	}

	debugLog("GetImplementationsForAPI called with API ID: %s", apiID)

	if kkClient == nil {
		debugLog("APIImplementationAPI client is nil")
		return nil, fmt.Errorf("APIImplementationAPI client is nil")
	}

	// Create a filter to filter implementations by API ID
	apiIDFilter := &kkComponents.UUIDFieldFilter{
		Eq: &apiID,
	}

	// Create a request to list API implementations for this API
	req := kkOps.ListAPIImplementationsRequest{
		Filter: &kkComponents.APIImplementationFilterParameters{
			APIID: apiIDFilter,
		},
	}
	debugLog("Created ListAPIImplementationsRequest with API ID filter: %s", apiID)

	// Call the SDK's ListAPIImplementations method
	debugLog("Calling ListAPIImplementations...")
	res, err := kkClient.ListAPIImplementations(ctx, req)
	if err != nil {
		debugLog("Error from ListAPIImplementations: %v", err)
		return nil, err
	}

	debugLog("ListAPIImplementations returned successfully")

	if res == nil {
		debugLog("Response is nil")
		return []interface{}{}, nil
	}

	if res.ListAPIImplementationsResponse == nil {
		debugLog("ListAPIImplementationsResponse is nil")
		return []interface{}{}, nil
	}

	debugLog("ListAPIImplementationsResponse has %d items", len(res.ListAPIImplementationsResponse.Data))

	// Check if we have data in the response
	if len(res.ListAPIImplementationsResponse.Data) == 0 {
		debugLog("No implementations found for API %s", apiID)
		return []interface{}{}, nil
	}

	// Convert to []interface{} and return
	result := make([]interface{}, len(res.ListAPIImplementationsResponse.Data))
	for i, impl := range res.ListAPIImplementationsResponse.Data {
		result[i] = impl
		debugLog("Added implementation %d to result: %s", i, impl.ID)
	}

	debugLog("Returning %d implementations", len(result))
	return result, nil
}