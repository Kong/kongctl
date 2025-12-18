package helpers

import (
	"context"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

// CatalogServicesAPI defines the interface for catalog service operations needed by the CLI.
type CatalogServicesAPI interface {
	ListCatalogServices(ctx context.Context, request kkOps.ListCatalogServicesRequest,
		opts ...kkOps.Option) (*kkOps.ListCatalogServicesResponse, error)
	CreateCatalogService(ctx context.Context, request kkComps.CreateCatalogService,
		opts ...kkOps.Option) (*kkOps.CreateCatalogServiceResponse, error)
	UpdateCatalogService(ctx context.Context, id string, request kkComps.UpdateCatalogService,
		opts ...kkOps.Option) (*kkOps.UpdateCatalogServiceResponse, error)
	DeleteCatalogService(ctx context.Context, id string,
		opts ...kkOps.Option) (*kkOps.DeleteCatalogServiceResponse, error)
	FetchCatalogService(ctx context.Context, id string,
		opts ...kkOps.Option) (*kkOps.FetchCatalogServiceResponse, error)
}

// CatalogServicesAPIImpl provides the real SDK implementation.
type CatalogServicesAPIImpl struct {
	SDK *kkSDK.SDK
}

func (c *CatalogServicesAPIImpl) ListCatalogServices(ctx context.Context, request kkOps.ListCatalogServicesRequest,
	opts ...kkOps.Option,
) (*kkOps.ListCatalogServicesResponse, error) {
	return c.SDK.CatalogServices.ListCatalogServices(ctx, request, opts...)
}

func (c *CatalogServicesAPIImpl) CreateCatalogService(ctx context.Context, request kkComps.CreateCatalogService,
	opts ...kkOps.Option,
) (*kkOps.CreateCatalogServiceResponse, error) {
	return c.SDK.CatalogServices.CreateCatalogService(ctx, request, opts...)
}

func (c *CatalogServicesAPIImpl) UpdateCatalogService(
	ctx context.Context,
	id string,
	request kkComps.UpdateCatalogService,
	opts ...kkOps.Option,
) (*kkOps.UpdateCatalogServiceResponse, error) {
	return c.SDK.CatalogServices.UpdateCatalogService(ctx, id, request, opts...)
}

func (c *CatalogServicesAPIImpl) DeleteCatalogService(ctx context.Context, id string,
	opts ...kkOps.Option,
) (*kkOps.DeleteCatalogServiceResponse, error) {
	return c.SDK.CatalogServices.DeleteCatalogService(ctx, id, opts...)
}

func (c *CatalogServicesAPIImpl) FetchCatalogService(ctx context.Context, id string,
	opts ...kkOps.Option,
) (*kkOps.FetchCatalogServiceResponse, error) {
	return c.SDK.CatalogServices.FetchCatalogService(ctx, id, opts...)
}
