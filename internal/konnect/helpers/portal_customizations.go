package helpers

import (
	"context"
	"fmt"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComponents "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

// PortalCustomizationAPI defines the interface for operations on Portal Customizations
type PortalCustomizationAPI interface {
	// Portal Customization operations (singleton resource - no create/delete)
	UpdatePortalCustomization(ctx context.Context, portalID string, request *kkComponents.PortalCustomization,
		opts ...kkOps.Option) (*kkOps.UpdatePortalCustomizationResponse, error)
	GetPortalCustomization(ctx context.Context, portalID string,
		opts ...kkOps.Option) (*kkOps.GetPortalCustomizationResponse, error)
}

// PortalCustomizationAPIImpl provides an implementation of the PortalCustomizationAPI interface
type PortalCustomizationAPIImpl struct {
	SDK *kkSDK.SDK
}

// UpdatePortalCustomization implements the PortalCustomizationAPI interface
func (p *PortalCustomizationAPIImpl) UpdatePortalCustomization(
	ctx context.Context, portalID string, request *kkComponents.PortalCustomization,
	opts ...kkOps.Option,
) (*kkOps.UpdatePortalCustomizationResponse, error) {
	if p.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}
	return p.SDK.PortalCustomization.UpdatePortalCustomization(ctx, portalID, request, opts...)
}

// GetPortalCustomization implements the PortalCustomizationAPI interface
func (p *PortalCustomizationAPIImpl) GetPortalCustomization(
	ctx context.Context, portalID string,
	opts ...kkOps.Option,
) (*kkOps.GetPortalCustomizationResponse, error) {
	if p.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}
	return p.SDK.PortalCustomization.GetPortalCustomization(ctx, portalID, opts...)
}