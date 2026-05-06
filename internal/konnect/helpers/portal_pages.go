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

func (p *PortalPageAPIImpl) portalPages() (*kkSDK.PortalPages, error) {
	if p.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}
	if p.SDK.PortalPages == nil {
		return nil, fmt.Errorf("SDK does not support PortalPages API")
	}
	return p.SDK.PortalPages, nil
}

// CreatePortalPage implements the PortalPageAPI interface
func (p *PortalPageAPIImpl) CreatePortalPage(
	ctx context.Context, portalID string, request kkComponents.CreatePortalPageRequest,
	opts ...kkOps.Option,
) (*kkOps.CreatePortalPageResponse, error) {
	pages, err := p.portalPages()
	if err != nil {
		return nil, err
	}
	return pages.CreatePortalPage(ctx, portalID, request, opts...)
}

// UpdatePortalPage implements the PortalPageAPI interface
func (p *PortalPageAPIImpl) UpdatePortalPage(
	ctx context.Context, request kkOps.UpdatePortalPageRequest,
	opts ...kkOps.Option,
) (*kkOps.UpdatePortalPageResponse, error) {
	pages, err := p.portalPages()
	if err != nil {
		return nil, err
	}
	return pages.UpdatePortalPage(ctx, request, opts...)
}

// DeletePortalPage implements the PortalPageAPI interface
func (p *PortalPageAPIImpl) DeletePortalPage(
	ctx context.Context, portalID string, pageID string,
	opts ...kkOps.Option,
) (*kkOps.DeletePortalPageResponse, error) {
	pages, err := p.portalPages()
	if err != nil {
		return nil, err
	}
	return pages.DeletePortalPage(ctx, portalID, pageID, opts...)
}

// ListPortalPages implements the PortalPageAPI interface
func (p *PortalPageAPIImpl) ListPortalPages(
	ctx context.Context, request kkOps.ListPortalPagesRequest,
	opts ...kkOps.Option,
) (*kkOps.ListPortalPagesResponse, error) {
	pages, err := p.portalPages()
	if err != nil {
		return nil, err
	}
	return pages.ListPortalPages(ctx, request, opts...)
}

// GetPortalPage implements the PortalPageAPI interface
func (p *PortalPageAPIImpl) GetPortalPage(
	ctx context.Context, portalID string, pageID string,
	opts ...kkOps.Option,
) (*kkOps.GetPortalPageResponse, error) {
	pages, err := p.portalPages()
	if err != nil {
		return nil, err
	}
	return pages.GetPortalPage(ctx, portalID, pageID, opts...)
}
