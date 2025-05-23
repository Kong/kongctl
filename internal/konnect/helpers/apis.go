package helpers

import (
	"context"

	kkInternal "github.com/Kong/sdk-konnect-go-internal"
	kkInternalOps "github.com/Kong/sdk-konnect-go-internal/models/operations"
)

// APIAPI defines the interface for operations on APIs
type APIAPI interface {
	// API operations
	ListApis(ctx context.Context, request kkInternalOps.ListApisRequest,
		opts ...kkInternalOps.Option) (*kkInternalOps.ListApisResponse, error)
}

// InternalAPIAPI provides an implementation of the APIAPI interface using the internal SDK
type InternalAPIAPI struct {
	SDK *kkInternal.SDK
}

// ListApis implements the APIAPI interface
func (a *InternalAPIAPI) ListApis(ctx context.Context, request kkInternalOps.ListApisRequest,
	opts ...kkInternalOps.Option,
) (*kkInternalOps.ListApisResponse, error) {
	return a.SDK.API.ListApis(ctx, request, opts...)
}

// GetDocumentationsForAPI is a deprecated function
// Use GetDocumentsForAPI from the apidocumentation.go file instead
func GetDocumentationsForAPI(_ context.Context, _ APIAPI, _ string) ([]interface{}, error) {
	return []interface{}{}, nil
}
