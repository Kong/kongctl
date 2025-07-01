package helpers

import (
	"context"
	"fmt"
	"os"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

// APIVersionAPI defines the interface for operations on API Versions
type APIVersionAPI interface {
	// API Version operations
	ListAPIVersions(ctx context.Context, request kkOps.ListAPIVersionsRequest,
		opts ...kkOps.Option) (*kkOps.ListAPIVersionsResponse, error)
}

// PublicAPIVersionAPI provides an implementation of the APIVersionAPI interface using the public SDK
type PublicAPIVersionAPI struct {
	SDK *kkSDK.SDK
}

// ListAPIVersions implements the APIVersionAPI interface
func (a *PublicAPIVersionAPI) ListAPIVersions(ctx context.Context,
	request kkOps.ListAPIVersionsRequest,
	opts ...kkOps.Option,
) (*kkOps.ListAPIVersionsResponse, error) {
	// Handle debugging based on environment variable
	debugEnabled := os.Getenv("KONGCTL_DEBUG") == EnvTrue

	// Helper function for debug logging
	debugLog := func(format string, args ...interface{}) {
		if debugEnabled {
			fmt.Fprintf(os.Stderr, "DEBUG: "+format+"\n", args...)
		}
	}

	if a.SDK == nil {
		debugLog("PublicAPIVersionAPI.SDK is nil")
		return nil, fmt.Errorf("SDK is nil")
	}

	if a.SDK.APIVersion == nil {
		debugLog("PublicAPIVersionAPI.SDK.APIVersion is nil")
		return nil, fmt.Errorf("SDK.APIVersion is nil")
	}

	debugLog("Calling a.SDK.APIVersion.ListAPIVersions")
	return a.SDK.APIVersion.ListAPIVersions(ctx, request, opts...)
}

// GetVersionsForAPI fetches all version objects for a specific API
func GetVersionsForAPI(ctx context.Context, kkClient APIVersionAPI, apiID string) ([]interface{}, error) {
	// We need to handle debugging differently here because this is in a separate package
	// Check if debug flag is set in environment
	debugEnabled := os.Getenv("KONGCTL_DEBUG") == EnvTrue

	// Helper function for debug logging
	debugLog := func(format string, args ...interface{}) {
		if debugEnabled {
			fmt.Fprintf(os.Stderr, "DEBUG: "+format+"\n", args...)
		}
	}

	debugLog("GetVersionsForAPI called with API ID: %s", apiID)

	if kkClient == nil {
		debugLog("APIVersionAPI client is nil")
		return nil, fmt.Errorf("APIVersionAPI client is nil")
	}

	// Create a request to list API versions for this API
	req := kkOps.ListAPIVersionsRequest{
		APIID: apiID,
	}
	debugLog("Created ListAPIVersionsRequest with APIID: %s", apiID)

	// Call the SDK's ListAPIVersions method
	debugLog("Calling ListAPIVersions...")
	res, err := kkClient.ListAPIVersions(ctx, req)
	if err != nil {
		debugLog("Error from ListAPIVersions: %v", err)
		return nil, err
	}

	debugLog("ListAPIVersions returned successfully")

	if res == nil {
		debugLog("Response is nil")
		return []interface{}{}, nil
	}

	if res.ListAPIVersionResponse == nil {
		debugLog("ListAPIVersionResponse is nil")
		return []interface{}{}, nil
	}

	debugLog("ListAPIVersionResponse has %d items", len(res.ListAPIVersionResponse.Data))

	// Check if we have data in the response
	if len(res.ListAPIVersionResponse.Data) == 0 {
		debugLog("No versions found for API %s", apiID)
		return []interface{}{}, nil
	}

	// Convert to []interface{} and return
	result := make([]interface{}, len(res.ListAPIVersionResponse.Data))
	for i, version := range res.ListAPIVersionResponse.Data {
		result[i] = version
		debugLog("Added version %d to result", i)
	}

	debugLog("Returning %d versions", len(result))
	return result, nil
}