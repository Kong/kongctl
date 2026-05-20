package helpers

import (
	"context"
	"fmt"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComponents "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

// PortalIPAllowListAPI defines the interface for portal IP allow list operations.
type PortalIPAllowListAPI interface {
	CreatePortalIPAllowList(
		ctx context.Context,
		portalID string,
		request *kkComponents.CreatePortalSourceIPRestriction,
		opts ...kkOps.Option,
	) (*kkOps.CreatePortalIPAllowListResponse, error)
	ListPortalIPAllowList(
		ctx context.Context,
		request kkOps.ListPortalIPAllowListRequest,
		opts ...kkOps.Option,
	) (*kkOps.ListPortalIPAllowListResponse, error)
	PutPortalIPAllowList(
		ctx context.Context,
		request kkOps.PutPortalIPAllowListRequest,
		opts ...kkOps.Option,
	) (*kkOps.PutPortalIPAllowListResponse, error)
	UpdatePortalIPAllowList(
		ctx context.Context,
		request kkOps.UpdatePortalIPAllowListRequest,
		opts ...kkOps.Option,
	) (*kkOps.UpdatePortalIPAllowListResponse, error)
	DeletePortalIPAllowList(
		ctx context.Context,
		portalID string,
		id string,
		opts ...kkOps.Option,
	) (*kkOps.DeletePortalIPAllowListResponse, error)
}

// PortalIPAllowListAPIImpl provides an implementation using the Konnect SDK.
type PortalIPAllowListAPIImpl struct {
	SDK *kkSDK.SDK
}

// CreatePortalIPAllowList creates a portal IP allow list entry.
func (p *PortalIPAllowListAPIImpl) CreatePortalIPAllowList(
	ctx context.Context,
	portalID string,
	request *kkComponents.CreatePortalSourceIPRestriction,
	opts ...kkOps.Option,
) (*kkOps.CreatePortalIPAllowListResponse, error) {
	if p.SDK == nil || p.SDK.PortalsIPAllowList == nil {
		return nil, fmt.Errorf("SDK portal IP allow list API is nil")
	}
	return p.SDK.PortalsIPAllowList.CreatePortalIPAllowList(ctx, portalID, request, opts...)
}

// ListPortalIPAllowList lists portal IP allow list entries.
func (p *PortalIPAllowListAPIImpl) ListPortalIPAllowList(
	ctx context.Context,
	request kkOps.ListPortalIPAllowListRequest,
	opts ...kkOps.Option,
) (*kkOps.ListPortalIPAllowListResponse, error) {
	if p.SDK == nil || p.SDK.PortalsIPAllowList == nil {
		return nil, fmt.Errorf("SDK portal IP allow list API is nil")
	}
	return p.SDK.PortalsIPAllowList.ListPortalIPAllowList(ctx, request, opts...)
}

// PutPortalIPAllowList replaces a portal IP allow list entry.
func (p *PortalIPAllowListAPIImpl) PutPortalIPAllowList(
	ctx context.Context,
	request kkOps.PutPortalIPAllowListRequest,
	opts ...kkOps.Option,
) (*kkOps.PutPortalIPAllowListResponse, error) {
	if p.SDK == nil || p.SDK.PortalsIPAllowList == nil {
		return nil, fmt.Errorf("SDK portal IP allow list API is nil")
	}
	return p.SDK.PortalsIPAllowList.PutPortalIPAllowList(ctx, request, opts...)
}

// UpdatePortalIPAllowList updates a portal IP allow list entry.
func (p *PortalIPAllowListAPIImpl) UpdatePortalIPAllowList(
	ctx context.Context,
	request kkOps.UpdatePortalIPAllowListRequest,
	opts ...kkOps.Option,
) (*kkOps.UpdatePortalIPAllowListResponse, error) {
	if p.SDK == nil || p.SDK.PortalsIPAllowList == nil {
		return nil, fmt.Errorf("SDK portal IP allow list API is nil")
	}
	return p.SDK.PortalsIPAllowList.UpdatePortalIPAllowList(ctx, request, opts...)
}

// DeletePortalIPAllowList deletes a portal IP allow list entry.
func (p *PortalIPAllowListAPIImpl) DeletePortalIPAllowList(
	ctx context.Context,
	portalID string,
	id string,
	opts ...kkOps.Option,
) (*kkOps.DeletePortalIPAllowListResponse, error) {
	if p.SDK == nil || p.SDK.PortalsIPAllowList == nil {
		return nil, fmt.Errorf("SDK portal IP allow list API is nil")
	}
	return p.SDK.PortalsIPAllowList.DeletePortalIPAllowList(ctx, portalID, id, opts...)
}
