package executor

import (
	"context"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/state"
)

// PortalAssetLogoAdapter implements SingletonOperations for portal logo assets
// Logo assets are singleton resources that always exist and only support updates
type PortalAssetLogoAdapter struct {
	client *state.Client
}

// NewPortalAssetLogoAdapter creates a new portal asset logo adapter
func NewPortalAssetLogoAdapter(client *state.Client) *PortalAssetLogoAdapter {
	return &PortalAssetLogoAdapter{client: client}
}

// MapUpdateFields maps fields to ReplacePortalImageAsset
func (p *PortalAssetLogoAdapter) MapUpdateFields(_ context.Context, fields map[string]any,
	update *kkComps.ReplacePortalImageAsset,
) error {
	// Extract data URL from fields
	dataURL, ok := fields["data_url"].(string)
	if !ok {
		return fmt.Errorf("data_url field is required and must be a string")
	}

	if dataURL == "" {
		return fmt.Errorf("data_url cannot be empty")
	}

	update.Data = dataURL
	return nil
}

// Update updates the portal logo
func (p *PortalAssetLogoAdapter) Update(ctx context.Context, portalID string,
	req kkComps.ReplacePortalImageAsset,
) error {
	return p.client.ReplacePortalAssetLogo(ctx, portalID, req.Data)
}

// ResourceType returns the resource type name
func (p *PortalAssetLogoAdapter) ResourceType() string {
	return "portal_asset_logo"
}
