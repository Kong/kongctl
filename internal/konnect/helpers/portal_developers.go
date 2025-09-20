package helpers

import (
	"context"
	"fmt"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

// PortalDeveloperAPI defines the interface for operations on portal developers
type PortalDeveloperAPI interface {
	ListPortalDevelopers(
		ctx context.Context,
		request kkOps.ListPortalDevelopersRequest,
		opts ...kkOps.Option,
	) (*kkOps.ListPortalDevelopersResponse, error)
	GetDeveloper(
		ctx context.Context,
		portalID string,
		developerID string,
		opts ...kkOps.Option,
	) (*kkOps.GetDeveloperResponse, error)
}

// PortalDeveloperAPIImpl provides an implementation backed by the SDK
type PortalDeveloperAPIImpl struct {
	SDK *kkSDK.SDK
}

// ListPortalDevelopers lists developers for a portal
func (p *PortalDeveloperAPIImpl) ListPortalDevelopers(
	ctx context.Context, request kkOps.ListPortalDevelopersRequest,
	opts ...kkOps.Option,
) (*kkOps.ListPortalDevelopersResponse, error) {
	if p.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}
	return p.SDK.PortalDevelopers.ListPortalDevelopers(ctx, request, opts...)
}

// GetDeveloper fetches a specific developer for a portal
func (p *PortalDeveloperAPIImpl) GetDeveloper(
	ctx context.Context, portalID string, developerID string,
	opts ...kkOps.Option,
) (*kkOps.GetDeveloperResponse, error) {
	if p.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}
	return p.SDK.PortalDevelopers.GetDeveloper(ctx, portalID, developerID, opts...)
}

var _ PortalDeveloperAPI = (*PortalDeveloperAPIImpl)(nil)
