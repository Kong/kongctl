package helpers

import (
	"context"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

// APIAPI defines the interface for operations on APIs
type APIAPI interface {
	// API operations
	ListApis(ctx context.Context, request kkOps.ListApisRequest,
		opts ...kkOps.Option) (*kkOps.ListApisResponse, error)
	CreateAPI(ctx context.Context, request kkComps.CreateAPIRequest,
		opts ...kkOps.Option) (*kkOps.CreateAPIResponse, error)
	UpdateAPI(ctx context.Context, apiID string, request kkComps.UpdateAPIRequest,
		opts ...kkOps.Option) (*kkOps.UpdateAPIResponse, error)
	DeleteAPI(ctx context.Context, apiID string,
		opts ...kkOps.Option) (*kkOps.DeleteAPIResponse, error)
}

// PublicAPIAPI provides an implementation of the APIAPI interface using the public SDK
type PublicAPIAPI struct {
	SDK *kkSDK.SDK
}

// ListApis implements the APIAPI interface
func (a *PublicAPIAPI) ListApis(ctx context.Context, request kkOps.ListApisRequest,
	opts ...kkOps.Option,
) (*kkOps.ListApisResponse, error) {
	return a.SDK.API.ListApis(ctx, request, opts...)
}

// CreateAPI implements the APIAPI interface
func (a *PublicAPIAPI) CreateAPI(ctx context.Context, request kkComps.CreateAPIRequest,
	opts ...kkOps.Option,
) (*kkOps.CreateAPIResponse, error) {
	return a.SDK.API.CreateAPI(ctx, request, opts...)
}

// UpdateAPI implements the APIAPI interface
func (a *PublicAPIAPI) UpdateAPI(ctx context.Context, apiID string, request kkComps.UpdateAPIRequest,
	opts ...kkOps.Option,
) (*kkOps.UpdateAPIResponse, error) {
	return a.SDK.API.UpdateAPI(ctx, apiID, request, opts...)
}

// DeleteAPI implements the APIAPI interface
func (a *PublicAPIAPI) DeleteAPI(ctx context.Context, apiID string,
	opts ...kkOps.Option,
) (*kkOps.DeleteAPIResponse, error) {
	return a.SDK.API.DeleteAPI(ctx, apiID, opts...)
}

// GetDocumentationsForAPI is a deprecated function
// Use GetDocumentsForAPI from the apidocumentation.go file instead
func GetDocumentationsForAPI(_ context.Context, _ APIAPI, _ string) ([]interface{}, error) {
	return []interface{}{}, nil
}