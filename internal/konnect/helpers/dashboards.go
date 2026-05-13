package helpers

import (
	"context"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

// DashboardsAPI defines the interface for Analytics dashboard operations needed by kongctl.
type DashboardsAPI interface {
	DashboardsList(ctx context.Context, request kkOps.DashboardsListRequest,
		opts ...kkOps.Option) (*kkOps.DashboardsListResponse, error)
	DashboardsCreate(ctx context.Context, request kkComps.DashboardUpdateRequest,
		opts ...kkOps.Option) (*kkOps.DashboardsCreateResponse, error)
	DashboardsGet(ctx context.Context, dashboardID string,
		opts ...kkOps.Option) (*kkOps.DashboardsGetResponse, error)
	DashboardsUpdate(ctx context.Context, dashboardID string, request kkComps.DashboardUpdateRequest,
		opts ...kkOps.Option) (*kkOps.DashboardsUpdateResponse, error)
	DashboardsDelete(ctx context.Context, dashboardID string,
		opts ...kkOps.Option) (*kkOps.DashboardsDeleteResponse, error)
}

// DashboardsAPIImpl provides the real SDK implementation.
type DashboardsAPIImpl struct {
	SDK *kkSDK.SDK
}

func (d *DashboardsAPIImpl) DashboardsList(ctx context.Context, request kkOps.DashboardsListRequest,
	opts ...kkOps.Option,
) (*kkOps.DashboardsListResponse, error) {
	return d.SDK.Dashboards.DashboardsList(ctx, request, opts...)
}

func (d *DashboardsAPIImpl) DashboardsCreate(ctx context.Context, request kkComps.DashboardUpdateRequest,
	opts ...kkOps.Option,
) (*kkOps.DashboardsCreateResponse, error) {
	return d.SDK.Dashboards.DashboardsCreate(ctx, request, opts...)
}

func (d *DashboardsAPIImpl) DashboardsGet(ctx context.Context, dashboardID string,
	opts ...kkOps.Option,
) (*kkOps.DashboardsGetResponse, error) {
	return d.SDK.Dashboards.DashboardsGet(ctx, dashboardID, opts...)
}

func (d *DashboardsAPIImpl) DashboardsUpdate(
	ctx context.Context,
	dashboardID string,
	request kkComps.DashboardUpdateRequest,
	opts ...kkOps.Option,
) (*kkOps.DashboardsUpdateResponse, error) {
	return d.SDK.Dashboards.DashboardsUpdate(ctx, dashboardID, request, opts...)
}

func (d *DashboardsAPIImpl) DashboardsDelete(ctx context.Context, dashboardID string,
	opts ...kkOps.Option,
) (*kkOps.DashboardsDeleteResponse, error) {
	return d.SDK.Dashboards.DashboardsDelete(ctx, dashboardID, opts...)
}
