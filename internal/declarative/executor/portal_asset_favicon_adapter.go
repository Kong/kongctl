package executor

import (
	"context"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/state"
)

// PortalAssetFaviconAdapter implements SingletonOperations for portal favicon assets
// Favicon assets are singleton resources that always exist and only support updates
type PortalAssetFaviconAdapter struct {
	client *state.Client
}

// NewPortalAssetFaviconAdapter creates a new portal asset favicon adapter
func NewPortalAssetFaviconAdapter(client *state.Client) *PortalAssetFaviconAdapter {
	return &PortalAssetFaviconAdapter{client: client}
}

// MapUpdateFields maps fields to ReplacePortalImageAsset
func (p *PortalAssetFaviconAdapter) MapUpdateFields(_ context.Context, fields map[string]any,
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

// Update updates the portal favicon
func (p *PortalAssetFaviconAdapter) Update(ctx context.Context, portalID string,
	req kkComps.ReplacePortalImageAsset,
) error {
	return p.client.ReplacePortalAssetFavicon(ctx, portalID, req.Data)
}

// ResourceType returns the resource type name
func (p *PortalAssetFaviconAdapter) ResourceType() string {
	return "portal_asset_favicon"
}
