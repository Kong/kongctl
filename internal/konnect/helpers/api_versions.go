package helpers

import (
	"context"
	"fmt"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

// APIVersionAPI defines the interface for operations on API Versions
type APIVersionAPI interface {
	// API Version operations
	CreateAPIVersion(ctx context.Context, apiID string, request kkComps.CreateAPIVersionRequest,
		opts ...kkOps.Option) (*kkOps.CreateAPIVersionResponse, error)
	ListAPIVersions(ctx context.Context, request kkOps.ListAPIVersionsRequest,
		opts ...kkOps.Option) (*kkOps.ListAPIVersionsResponse, error)
	UpdateAPIVersion(ctx context.Context, request kkOps.UpdateAPIVersionRequest,
		opts ...kkOps.Option) (*kkOps.UpdateAPIVersionResponse, error)
	DeleteAPIVersion(ctx context.Context, apiID string, versionID string,
		opts ...kkOps.Option) (*kkOps.DeleteAPIVersionResponse, error)
	FetchAPIVersion(ctx context.Context, apiID string, versionID string,
		opts ...kkOps.Option) (*kkOps.FetchAPIVersionResponse, error)
}

// APIVersionAPIImpl provides an implementation of the APIVersionAPI interface
type APIVersionAPIImpl struct {
	SDK *kkSDK.SDK
}

// CreateAPIVersion implements the APIVersionAPI interface
func (a *APIVersionAPIImpl) CreateAPIVersion(ctx context.Context, apiID string, request kkComps.CreateAPIVersionRequest,
	opts ...kkOps.Option,
) (*kkOps.CreateAPIVersionResponse, error) {
	if a.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}

	if a.SDK.APIVersion == nil {
		return nil, fmt.Errorf("SDK.APIVersion is nil")
	}
	return a.SDK.APIVersion.CreateAPIVersion(ctx, apiID, request, opts...)
}

// ListAPIVersions implements the APIVersionAPI interface
func (a *APIVersionAPIImpl) ListAPIVersions(ctx context.Context,
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

// UpdateAPIVersion implements the APIVersionAPI interface
func (a *APIVersionAPIImpl) UpdateAPIVersion(ctx context.Context, request kkOps.UpdateAPIVersionRequest,
	opts ...kkOps.Option,
) (*kkOps.UpdateAPIVersionResponse, error) {
	if a.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}

	if a.SDK.APIVersion == nil {
		return nil, fmt.Errorf("SDK.APIVersion is nil")
	}
	return a.SDK.APIVersion.UpdateAPIVersion(ctx, request, opts...)
}

// DeleteAPIVersion implements the APIVersionAPI interface
func (a *APIVersionAPIImpl) DeleteAPIVersion(ctx context.Context, apiID string, versionID string,
	opts ...kkOps.Option,
) (*kkOps.DeleteAPIVersionResponse, error) {
	if a.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}

	if a.SDK.APIVersion == nil {
		return nil, fmt.Errorf("SDK.APIVersion is nil")
	}
	return a.SDK.APIVersion.DeleteAPIVersion(ctx, apiID, versionID, opts...)
}

// FetchAPIVersion implements the APIVersionAPI interface
func (a *APIVersionAPIImpl) FetchAPIVersion(ctx context.Context, apiID string, versionID string,
	opts ...kkOps.Option,
) (*kkOps.FetchAPIVersionResponse, error) {
	if a.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}

	if a.SDK.APIVersion == nil {
		return nil, fmt.Errorf("SDK.APIVersion is nil")
	}
	return a.SDK.APIVersion.FetchAPIVersion(ctx, apiID, versionID, opts...)
}

// GetVersionsForAPI fetches all version objects for a specific API
func GetVersionsForAPI(ctx context.Context, kkClient APIVersionAPI, apiID string) ([]any, error) {
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
		return []any{}, nil
	}

	if res.ListAPIVersionResponse == nil {
		return []any{}, nil
	}

	// Check if we have data in the response
	if len(res.ListAPIVersionResponse.Data) == 0 {
		return []any{}, nil
	}

	// Convert to []any and return
	result := make([]any, len(res.ListAPIVersionResponse.Data))
	for i, version := range res.ListAPIVersionResponse.Data {
		result[i] = version
	}

	return result, nil
}
