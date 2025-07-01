package helpers

import (
	"context"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

// APIAPI defines the interface for operations on APIs
type APIAPI interface {
	// API operations
	ListApis(ctx context.Context, request kkOps.ListApisRequest,
		opts ...kkOps.Option) (*kkOps.ListApisResponse, error)
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

// GetDocumentationsForAPI is a deprecated function
// Use GetDocumentsForAPI from the apidocumentation.go file instead
func GetDocumentationsForAPI(_ context.Context, _ APIAPI, _ string) ([]interface{}, error) {
	return []interface{}{}, nil
}