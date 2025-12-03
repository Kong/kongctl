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
	if v, ok := fields["oidc_auth_enabled"].(bool); ok {
		update.OidcAuthEnabled = &v
	}
	if v, ok := fields["saml_auth_enabled"].(bool); ok {
		update.SamlAuthEnabled = &v
	}
	if v, ok := fields["oidc_team_mapping_enabled"].(bool); ok {
		update.OidcTeamMappingEnabled = &v
	}
	if v, ok := fields["konnect_mapping_enabled"].(bool); ok {
		update.KonnectMappingEnabled = &v
	}
	if v, ok := fields["idp_mapping_enabled"].(bool); ok {
		update.IdpMappingEnabled = &v
	}
	if v, ok := fields["oidc_issuer"].(string); ok {
		update.OidcIssuer = &v
	}
	if v, ok := fields["oidc_client_id"].(string); ok {
		update.OidcClientID = &v
	}
	if v, ok := fields["oidc_client_secret"].(string); ok {
		update.OidcClientSecret = &v
	}
	if v, ok := fields["oidc_scopes"].([]string); ok {
		update.OidcScopes = v
	}
	if v, ok := fields["oidc_claim_mappings"].(*kkComps.PortalClaimMappings); ok {
		update.OidcClaimMappings = v
	} else if v, ok := fields["oidc_claim_mappings"].(kkComps.PortalClaimMappings); ok {
		update.OidcClaimMappings = &v
	} else if m, ok := fields["oidc_claim_mappings"].(map[string]any); ok {
		claim := &kkComps.PortalClaimMappings{}
		if name, ok := m["name"].(string); ok {
			claim.Name = &name
		}
		if email, ok := m["email"].(string); ok {
			claim.Email = &email
		}
		if groups, ok := m["groups"].(string); ok {
			claim.Groups = &groups
		}
		update.OidcClaimMappings = claim
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
