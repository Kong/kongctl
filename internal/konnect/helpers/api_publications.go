package helpers

import (
	"context"
	"fmt"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComponents "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

// APIPublicationAPI defines the interface for operations on API Publications
type APIPublicationAPI interface {
	// API Publication operations
	PublishAPIToPortal(ctx context.Context, request kkOps.PublishAPIToPortalRequest,
		opts ...kkOps.Option) (*kkOps.PublishAPIToPortalResponse, error)
	DeletePublication(ctx context.Context, apiID string, portalID string,
		opts ...kkOps.Option) (*kkOps.DeletePublicationResponse, error)
	ListAPIPublications(ctx context.Context, request kkOps.ListAPIPublicationsRequest,
		opts ...kkOps.Option) (*kkOps.ListAPIPublicationsResponse, error)
}

// APIPublicationAPIImpl provides an implementation of the APIPublicationAPI interface
type APIPublicationAPIImpl struct {
	SDK *kkSDK.SDK
}

// PublishAPIToPortal implements the APIPublicationAPI interface
func (a *APIPublicationAPIImpl) PublishAPIToPortal(ctx context.Context, request kkOps.PublishAPIToPortalRequest,
	opts ...kkOps.Option,
) (*kkOps.PublishAPIToPortalResponse, error) {
	if a.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}

	if a.SDK.APIPublication == nil {
		return nil, fmt.Errorf("SDK.APIPublication is nil")
	}
	return a.SDK.APIPublication.PublishAPIToPortal(ctx, request, opts...)
}

// DeletePublication implements the APIPublicationAPI interface
func (a *APIPublicationAPIImpl) DeletePublication(ctx context.Context, apiID string, portalID string,
	opts ...kkOps.Option,
) (*kkOps.DeletePublicationResponse, error) {
	if a.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}

	if a.SDK.APIPublication == nil {
		return nil, fmt.Errorf("SDK.APIPublication is nil")
	}
	return a.SDK.APIPublication.DeletePublication(ctx, apiID, portalID, opts...)
}

// ListAPIPublications implements the APIPublicationAPI interface
func (a *APIPublicationAPIImpl) ListAPIPublications(ctx context.Context,
	request kkOps.ListAPIPublicationsRequest,
	opts ...kkOps.Option,
) (*kkOps.ListAPIPublicationsResponse, error) {
	if a.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}

	if a.SDK.APIPublication == nil {
		return nil, fmt.Errorf("SDK.APIPublication is nil")
	}
	return a.SDK.APIPublication.ListAPIPublications(ctx, request, opts...)
}

// GetPublicationsForAPI fetches all publication objects for a specific API
func GetPublicationsForAPI(ctx context.Context, kkClient APIPublicationAPI, apiID string) ([]any, error) {
	if kkClient == nil {
		return nil, fmt.Errorf("APIPublicationAPI client is nil")
	}

	// Create a filter to get publications for this API
	apiIDFilter := &kkComponents.UUIDFieldFilter{
		Eq: &apiID,
	}

	// Create a request to list API publications for this API
	req := kkOps.ListAPIPublicationsRequest{
		Filter: &kkComponents.APIPublicationFilterParameters{
			APIID: apiIDFilter,
		},
	}

	// Call the SDK's ListAPIPublications method
	res, err := kkClient.ListAPIPublications(ctx, req)
	if err != nil {
		return nil, err
	}

	if res == nil {
		return []any{}, nil
	}

	if res.ListAPIPublicationResponse == nil {
		return []any{}, nil
	}

	// Check if we have data in the response
	if len(res.ListAPIPublicationResponse.Data) == 0 {
		return []any{}, nil
	}

	// Convert to []any and return
	result := make([]any, len(res.ListAPIPublicationResponse.Data))
	for i, pub := range res.ListAPIPublicationResponse.Data {
		result[i] = pub
	}

	return result, nil
}
