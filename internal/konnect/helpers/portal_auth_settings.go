package helpers

import (
	"context"
	"fmt"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComponents "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

// PortalAuthSettingsAPI defines the interface for portal authentication settings operations (singleton).
type PortalAuthSettingsAPI interface {
	UpdatePortalAuthenticationSettings(
		ctx context.Context,
		portalID string,
		request *kkComponents.PortalAuthenticationSettingsUpdateRequest,
		opts ...kkOps.Option,
	) (*kkOps.UpdatePortalAuthenticationSettingsResponse, error)
	GetPortalAuthenticationSettings(
		ctx context.Context,
		portalID string,
		opts ...kkOps.Option,
	) (*kkOps.GetPortalAuthenticationSettingsResponse, error)
}

// PortalAuthSettingsAPIImpl provides an implementation using the Konnect SDK.
type PortalAuthSettingsAPIImpl struct {
	SDK *kkSDK.SDK
}

// UpdatePortalAuthenticationSettings updates portal auth settings.
func (p *PortalAuthSettingsAPIImpl) UpdatePortalAuthenticationSettings(
	ctx context.Context,
	portalID string,
	request *kkComponents.PortalAuthenticationSettingsUpdateRequest,
	opts ...kkOps.Option,
) (*kkOps.UpdatePortalAuthenticationSettingsResponse, error) {
	if p.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}
	return p.SDK.PortalAuthSettings.UpdatePortalAuthenticationSettings(ctx, portalID, request, opts...)
}

// GetPortalAuthenticationSettings fetches portal auth settings.
func (p *PortalAuthSettingsAPIImpl) GetPortalAuthenticationSettings(
	ctx context.Context,
	portalID string,
	opts ...kkOps.Option,
) (*kkOps.GetPortalAuthenticationSettingsResponse, error) {
	if p.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}
	return p.SDK.PortalAuthSettings.GetPortalAuthenticationSettings(ctx, portalID, opts...)
}
