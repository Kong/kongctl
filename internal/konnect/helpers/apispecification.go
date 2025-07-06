package helpers

import (
	"context"
	"fmt"

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
	if a.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}

	if a.SDK.APIVersion == nil {
		return nil, fmt.Errorf("SDK.APIVersion is nil")
	}
	return a.SDK.APIVersion.ListAPIVersions(ctx, request, opts...)
}

// GetVersionsForAPI fetches all version objects for a specific API
func GetVersionsForAPI(ctx context.Context, kkClient APIVersionAPI, apiID string) ([]interface{}, error) {
	if kkClient == nil {
		return nil, fmt.Errorf("APIVersionAPI client is nil")
	}

	// Create a request to list API versions for this API
	req := kkOps.ListAPIVersionsRequest{
		APIID: apiID,
	}

	// Call the SDK's ListAPIVersions method
	res, err := kkClient.ListAPIVersions(ctx, req)
	if err != nil {
		return nil, err
	}

	if res == nil {
		return []interface{}{}, nil
	}

	if res.ListAPIVersionResponse == nil {
		return []interface{}{}, nil
	}

	// Check if we have data in the response
	if len(res.ListAPIVersionResponse.Data) == 0 {
		return []interface{}{}, nil
	}

	// Convert to []interface{} and return
	result := make([]interface{}, len(res.ListAPIVersionResponse.Data))
	for i, version := range res.ListAPIVersionResponse.Data {
		result[i] = version
	}

	return result, nil
}