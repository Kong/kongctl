package helpers

import (
	"context"
	"fmt"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComponents "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

// PortalPageAPI defines the interface for operations on Portal Pages
type PortalPageAPI interface {
	// Portal Page operations
	CreatePortalPage(ctx context.Context, portalID string, request kkComponents.CreatePortalPageRequest,
		opts ...kkOps.Option) (*kkOps.CreatePortalPageResponse, error)
	UpdatePortalPage(ctx context.Context, request kkOps.UpdatePortalPageRequest,
		opts ...kkOps.Option) (*kkOps.UpdatePortalPageResponse, error)
	DeletePortalPage(ctx context.Context, portalID string, pageID string,
		opts ...kkOps.Option) (*kkOps.DeletePortalPageResponse, error)
	ListPortalPages(ctx context.Context, request kkOps.ListPortalPagesRequest,
		opts ...kkOps.Option) (*kkOps.ListPortalPagesResponse, error)
	GetPortalPage(ctx context.Context, portalID string, pageID string,
		opts ...kkOps.Option) (*kkOps.GetPortalPageResponse, error)
}

// PortalPageAPIImpl provides an implementation of the PortalPageAPI interface
type PortalPageAPIImpl struct {
	SDK *kkSDK.SDK
}

// CreatePortalPage implements the PortalPageAPI interface
func (p *PortalPageAPIImpl) CreatePortalPage(
	ctx context.Context, portalID string, request kkComponents.CreatePortalPageRequest,
	opts ...kkOps.Option,
) (*kkOps.CreatePortalPageResponse, error) {
	if p.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}
	return p.SDK.Pages.CreatePortalPage(ctx, portalID, request, opts...)
}

// UpdatePortalPage implements the PortalPageAPI interface
func (p *PortalPageAPIImpl) UpdatePortalPage(
	ctx context.Context, request kkOps.UpdatePortalPageRequest,
	opts ...kkOps.Option,
) (*kkOps.UpdatePortalPageResponse, error) {
	if p.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}
	return p.SDK.Pages.UpdatePortalPage(ctx, request, opts...)
}

// DeletePortalPage implements the PortalPageAPI interface
func (p *PortalPageAPIImpl) DeletePortalPage(
	ctx context.Context, portalID string, pageID string,
	opts ...kkOps.Option,
) (*kkOps.DeletePortalPageResponse, error) {
	if p.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}
	return p.SDK.Pages.DeletePortalPage(ctx, portalID, pageID, opts...)
}

// ListPortalPages implements the PortalPageAPI interface
func (p *PortalPageAPIImpl) ListPortalPages(
	ctx context.Context, request kkOps.ListPortalPagesRequest,
	opts ...kkOps.Option,
) (*kkOps.ListPortalPagesResponse, error) {
	if p.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}
	return p.SDK.Pages.ListPortalPages(ctx, request, opts...)
}

// GetPortalPage implements the PortalPageAPI interface
func (p *PortalPageAPIImpl) GetPortalPage(
	ctx context.Context, portalID string, pageID string,
	opts ...kkOps.Option,
) (*kkOps.GetPortalPageResponse, error) {
	if p.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}
	return p.SDK.Pages.GetPortalPage(ctx, portalID, pageID, opts...)
}