package executor

import (
	"context"
	"fmt"
	"log/slog"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/log"
)

// PortalAuthSettingsAdapter implements SingletonOperations for portal auth settings.
type PortalAuthSettingsAdapter struct {
	client *state.Client
}

// NewPortalAuthSettingsAdapter creates a new adapter.
func NewPortalAuthSettingsAdapter(client *state.Client) *PortalAuthSettingsAdapter {
	return &PortalAuthSettingsAdapter{client: client}
}

// MapUpdateFields maps planner fields into the SDK update request.
func (p *PortalAuthSettingsAdapter) MapUpdateFields(
	ctx context.Context,
	fields map[string]any,
	update *kkComps.PortalAuthenticationSettingsUpdateRequest,
) error {
	// Use context logger when available to align with repo logging style.
	if logger, ok := ctx.Value(log.LoggerKey).(*slog.Logger); ok && logger != nil {
		logger.Debug("mapping portal auth settings update fields", "fields", fields)
	}

	if v, ok := fields["basic_auth_enabled"].(bool); ok {
		update.BasicAuthEnabled = &v
	}
	if v, ok := fields["konnect_mapping_enabled"].(bool); ok {
		update.KonnectMappingEnabled = &v
	}
	if v, ok := fields["idp_mapping_enabled"].(bool); ok {
		update.IdpMappingEnabled = &v
	}

	if logger, ok := ctx.Value(log.LoggerKey).(*slog.Logger); ok && logger != nil {
		logger.Debug("mapped portal auth settings request", "request", update)
	}

	return nil
}

// Update executes the API call for portal auth settings.
func (p *PortalAuthSettingsAdapter) Update(
	ctx context.Context,
	portalID string,
	req kkComps.PortalAuthenticationSettingsUpdateRequest,
) error {
	if portalID == "" {
		return fmt.Errorf("portal ID required for portal auth settings update")
	}
	return p.client.UpdatePortalAuthSettings(ctx, portalID, req)
}

func (p *PortalAuthSettingsAdapter) ResourceType() string {
	return string(resources.ResourceTypePortalAuthSettings)
}
