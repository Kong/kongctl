package helpers

import (
	"context"
	"fmt"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComponents "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

// PortalIntegrationsAPI defines the interface for portal integrations operations (singleton).
type PortalIntegrationsAPI interface {
	GetPortalIntegrations(
		ctx context.Context,
		portalID string,
		opts ...kkOps.Option,
	) (*kkOps.GetPortalIntegrationsResponse, error)
	UpsertPortalIntegrations(
		ctx context.Context,
		portalID string,
		portalIntegrations *kkComponents.PortalIntegrations,
		opts ...kkOps.Option,
	) (*kkOps.UpsertPortalIntegrationsResponse, error)
}

// PortalIntegrationsAPIImpl provides an implementation using the Konnect SDK.
type PortalIntegrationsAPIImpl struct {
	SDK *kkSDK.SDK
}

// GetPortalIntegrations fetches portal integration configuration.
func (p *PortalIntegrationsAPIImpl) GetPortalIntegrations(
	ctx context.Context,
	portalID string,
	opts ...kkOps.Option,
) (*kkOps.GetPortalIntegrationsResponse, error) {
	if p.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}
	return p.SDK.PortalIntegrations.GetPortalIntegrations(ctx, portalID, opts...)
}

// UpsertPortalIntegrations replaces portal integration configuration.
func (p *PortalIntegrationsAPIImpl) UpsertPortalIntegrations(
	ctx context.Context,
	portalID string,
	portalIntegrations *kkComponents.PortalIntegrations,
	opts ...kkOps.Option,
) (*kkOps.UpsertPortalIntegrationsResponse, error) {
	if p.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}
	return p.SDK.PortalIntegrations.UpsertPortalIntegrations(ctx, portalID, portalIntegrations, opts...)
}
