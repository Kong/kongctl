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

	versions, err := paginateAllPageNumber(func(pageSize, pageNumber int64) (
		[]kkComps.ListAPIVersionResponseAPIVersionSummary, float64, error,
	) {
		req := kkOps.ListAPIVersionsRequest{
			APIID:      apiID,
			PageSize:   Int64(pageSize),
			PageNumber: Int64(pageNumber),
		}

		res, err := kkClient.ListAPIVersions(ctx, req)
		if err != nil {
			return nil, 0, err
		}

		if res == nil || res.ListAPIVersionResponse == nil {
			return []kkComps.ListAPIVersionResponseAPIVersionSummary{}, 0, nil
		}

		return res.ListAPIVersionResponse.Data, res.ListAPIVersionResponse.Meta.Page.Total, nil
	})
	if err != nil {
		return nil, err
	}

	result := make([]any, len(versions))
	for i, version := range versions {
		result[i] = version
	}

	return result, nil
}
