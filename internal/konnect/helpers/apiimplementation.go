package helpers

import (
	"context"
	"fmt"

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
	if a.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}

	if a.SDK.APIImplementation == nil {
		return nil, fmt.Errorf("SDK.APIImplementation is nil")
	}

	return a.SDK.APIImplementation.ListAPIImplementations(ctx, request, opts...)
}

// GetImplementationsForAPI fetches all implementation objects for a specific API
func GetImplementationsForAPI(ctx context.Context, kkClient APIImplementationAPI, apiID string) ([]interface{}, error) {
	if kkClient == nil {
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

	// Call the SDK's ListAPIImplementations method
	res, err := kkClient.ListAPIImplementations(ctx, req)
	if err != nil {
		return nil, err
	}

	if res == nil {
		return []interface{}{}, nil
	}

	if res.ListAPIImplementationsResponse == nil {
		return []interface{}{}, nil
	}

	// Check if we have data in the response
	if len(res.ListAPIImplementationsResponse.Data) == 0 {
		return []interface{}{}, nil
	}

	// Convert to []interface{} and return
	result := make([]interface{}, len(res.ListAPIImplementationsResponse.Data))
	for i, impl := range res.ListAPIImplementationsResponse.Data {
		result[i] = impl
	}

	return result, nil
}