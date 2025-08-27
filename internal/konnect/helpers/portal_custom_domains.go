package helpers

import (
	"context"
	"fmt"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComponents "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

// PortalCustomDomainAPI defines the interface for operations on Portal Custom Domains
type PortalCustomDomainAPI interface {
	// Portal Custom Domain operations (singleton resource per portal)
	CreatePortalCustomDomain(ctx context.Context, portalID string, request kkComponents.CreatePortalCustomDomainRequest,
		opts ...kkOps.Option) (*kkOps.CreatePortalCustomDomainResponse, error)
	UpdatePortalCustomDomain(ctx context.Context, portalID string, request kkComponents.UpdatePortalCustomDomainRequest,
		opts ...kkOps.Option) (*kkOps.UpdatePortalCustomDomainResponse, error)
	DeletePortalCustomDomain(ctx context.Context, portalID string,
		opts ...kkOps.Option) (*kkOps.DeletePortalCustomDomainResponse, error)
	GetPortalCustomDomain(ctx context.Context, portalID string,
		opts ...kkOps.Option) (*kkOps.GetPortalCustomDomainResponse, error)
}

// PortalCustomDomainAPIImpl provides an implementation of the PortalCustomDomainAPI interface
type PortalCustomDomainAPIImpl struct {
	SDK *kkSDK.SDK
}

// CreatePortalCustomDomain implements the PortalCustomDomainAPI interface
func (p *PortalCustomDomainAPIImpl) CreatePortalCustomDomain(
	ctx context.Context, portalID string, request kkComponents.CreatePortalCustomDomainRequest,
	opts ...kkOps.Option,
) (*kkOps.CreatePortalCustomDomainResponse, error) {
	if p.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}
	return p.SDK.PortalCustomDomains.CreatePortalCustomDomain(ctx, portalID, request, opts...)
}

// UpdatePortalCustomDomain implements the PortalCustomDomainAPI interface
func (p *PortalCustomDomainAPIImpl) UpdatePortalCustomDomain(
	ctx context.Context, portalID string, request kkComponents.UpdatePortalCustomDomainRequest,
	opts ...kkOps.Option,
) (*kkOps.UpdatePortalCustomDomainResponse, error) {
	if p.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}
	return p.SDK.PortalCustomDomains.UpdatePortalCustomDomain(ctx, portalID, request, opts...)
}

// DeletePortalCustomDomain implements the PortalCustomDomainAPI interface
func (p *PortalCustomDomainAPIImpl) DeletePortalCustomDomain(
	ctx context.Context, portalID string,
	opts ...kkOps.Option,
) (*kkOps.DeletePortalCustomDomainResponse, error) {
	if p.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}
	return p.SDK.PortalCustomDomains.DeletePortalCustomDomain(ctx, portalID, opts...)
}

// GetPortalCustomDomain implements the PortalCustomDomainAPI interface
func (p *PortalCustomDomainAPIImpl) GetPortalCustomDomain(
	ctx context.Context, portalID string,
	opts ...kkOps.Option,
) (*kkOps.GetPortalCustomDomainResponse, error) {
	if p.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}
	return p.SDK.PortalCustomDomains.GetPortalCustomDomain(ctx, portalID, opts...)
}
