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
	ListPortalTeamGroupMappings(
		ctx context.Context,
		request kkOps.ListPortalTeamGroupMappingsRequest,
		opts ...kkOps.Option,
	) (*kkOps.ListPortalTeamGroupMappingsResponse, error)
	UpdatePortalTeamGroupMappings(
		ctx context.Context,
		portalID string,
		request *kkComponents.PortalTeamGroupMappingsUpdateRequest,
		opts ...kkOps.Option,
	) (*kkOps.UpdatePortalTeamGroupMappingsResponse, error)
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

// ListPortalTeamGroupMappings lists portal team IdP group mappings.
func (p *PortalAuthSettingsAPIImpl) ListPortalTeamGroupMappings(
	ctx context.Context,
	request kkOps.ListPortalTeamGroupMappingsRequest,
	opts ...kkOps.Option,
) (*kkOps.ListPortalTeamGroupMappingsResponse, error) {
	if p.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}
	return p.SDK.PortalAuthSettings.ListPortalTeamGroupMappings(ctx, request, opts...)
}

// UpdatePortalTeamGroupMappings partially updates portal team IdP group mappings.
func (p *PortalAuthSettingsAPIImpl) UpdatePortalTeamGroupMappings(
	ctx context.Context,
	portalID string,
	request *kkComponents.PortalTeamGroupMappingsUpdateRequest,
	opts ...kkOps.Option,
) (*kkOps.UpdatePortalTeamGroupMappingsResponse, error) {
	if p.SDK == nil {
		return nil, fmt.Errorf("SDK is nil")
	}
	return p.SDK.PortalAuthSettings.UpdatePortalTeamGroupMappings(ctx, portalID, request, opts...)
}
