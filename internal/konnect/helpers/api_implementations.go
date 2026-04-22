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
	CreateAPIImplementation(ctx context.Context, apiID string, apiImplementation kkComponents.APIImplementation,
		opts ...kkOps.Option) (*kkOps.CreateAPIImplementationResponse, error)
	DeleteAPIImplementation(ctx context.Context, apiID string, implementationID string,
		opts ...kkOps.Option) (*kkOps.DeleteAPIImplementationResponse, error)
}

// APIImplementationAPIImpl provides an implementation of the APIImplementationAPI interface
type APIImplementationAPIImpl struct {
	SDK *kkSDK.SDK
}

// ListAPIImplementations implements the APIImplementationAPI interface
func (a *APIImplementationAPIImpl) ListAPIImplementations(ctx context.Context,
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

// CreateAPIImplementation implements the APIImplementationAPI interface
func (a *APIImplementationAPIImpl) CreateAPIImplementation(ctx context.Context,
	apiID string, apiImplementation kkComponents.APIImplementation,
	opts ...kkOps.Option,
) (*kkOps.CreateAPIImplementationResponse, error) {
	if a.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}

	if a.SDK.APIImplementation == nil {
		return nil, fmt.Errorf("SDK.APIImplementation is nil")
	}

	return a.SDK.APIImplementation.CreateAPIImplementation(ctx, apiID, apiImplementation, opts...)
}

// DeleteAPIImplementation implements the APIImplementationAPI interface
func (a *APIImplementationAPIImpl) DeleteAPIImplementation(ctx context.Context,
	apiID string, implementationID string,
	opts ...kkOps.Option,
) (*kkOps.DeleteAPIImplementationResponse, error) {
	if a.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}

	if a.SDK.APIImplementation == nil {
		return nil, fmt.Errorf("SDK.APIImplementation is nil")
	}

	return a.SDK.APIImplementation.DeleteAPIImplementation(ctx, apiID, implementationID, opts...)
}

// GetImplementationsForAPI fetches all implementation objects for a specific API
func GetImplementationsForAPI(ctx context.Context, kkClient APIImplementationAPI, apiID string) ([]any, error) {
	if kkClient == nil {
		return nil, fmt.Errorf("APIImplementationAPI client is nil")
	}

	apiIDFilter := &kkComponents.UUIDFieldFilter{
		Eq: &apiID,
	}

	implementations, err := paginateAllPageNumber(func(pageSize, pageNumber int64) (
		[]kkComponents.APIImplementationListItem, float64, error,
	) {
		req := kkOps.ListAPIImplementationsRequest{
			PageSize:   Int64(pageSize),
			PageNumber: Int64(pageNumber),
			Filter: &kkComponents.APIImplementationFilterParameters{
				APIID: apiIDFilter,
			},
		}

		res, err := kkClient.ListAPIImplementations(ctx, req)
		if err != nil {
			return nil, 0, err
		}

		if res == nil || res.ListAPIImplementationsResponse == nil {
			return []kkComponents.APIImplementationListItem{}, 0, nil
		}

		return res.ListAPIImplementationsResponse.Data, res.ListAPIImplementationsResponse.Meta.Page.Total, nil
	})
	if err != nil {
		return nil, err
	}

	result := make([]any, len(implementations))
	for i, impl := range implementations {
		result[i] = impl
	}

	return result, nil
}
