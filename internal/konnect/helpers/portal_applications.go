package helpers

import (
	"context"
	"fmt"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComponents "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

// PortalApplicationAPI defines the interface for operations on Portal Applications
type PortalApplicationAPI interface {
	ListApplications(
		ctx context.Context,
		request kkOps.ListApplicationsRequest,
		opts ...kkOps.Option,
	) (*kkOps.ListApplicationsResponse, error)
	GetApplication(
		ctx context.Context,
		portalID string,
		applicationID string,
		opts ...kkOps.Option,
	) (*kkOps.GetApplicationResponse, error)
}

// PortalApplicationAPIImpl provides an implementation of the PortalApplicationAPI interface
type PortalApplicationAPIImpl struct {
	SDK *kkSDK.SDK
}

// ListApplications lists all applications for the given portal
func (p *PortalApplicationAPIImpl) ListApplications(
	ctx context.Context, request kkOps.ListApplicationsRequest,
	opts ...kkOps.Option,
) (*kkOps.ListApplicationsResponse, error) {
	if p.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}
	return p.SDK.Applications.ListApplications(ctx, request, opts...)
}

// GetApplication returns a specific application scoped to a portal
func (p *PortalApplicationAPIImpl) GetApplication(
	ctx context.Context, portalID string, applicationID string,
	opts ...kkOps.Option,
) (*kkOps.GetApplicationResponse, error) {
	if p.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}
	return p.SDK.Applications.GetApplication(ctx, portalID, applicationID, opts...)
}

var _ PortalApplicationAPI = (*PortalApplicationAPIImpl)(nil)

// ApplicationSummary represents the common fields needed from an application union
func ApplicationSummary(app kkComponents.Application) (id string, name string) {
	if app.KeyAuthApplication != nil {
		return app.KeyAuthApplication.GetID(), app.KeyAuthApplication.GetName()
	}
	if app.ClientCredentialsApplication != nil {
		return app.ClientCredentialsApplication.GetID(), app.ClientCredentialsApplication.GetName()
	}
	return "", ""
}
