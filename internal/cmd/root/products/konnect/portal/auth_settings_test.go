package portal

import (
	"strings"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/stretchr/testify/require"

	"github.com/kong/kongctl/internal/cmd/output/tableview"
)

func TestPortalAuthSettingsDetailView_ExcludesDeprecatedFields(t *testing.T) {
	idpMappingEnabled := true
	samlAuthEnabled := true

	settings := &kkComps.PortalAuthenticationSettingsResponse{
		BasicAuthEnabled:       true,
		IdpMappingEnabled:      &idpMappingEnabled,
		KonnectMappingEnabled:  false,
		OidcAuthEnabled:        true,
		SamlAuthEnabled:        &samlAuthEnabled,
		OidcTeamMappingEnabled: true,
	}

	detail := portalAuthSettingsDetailView(settings)

	require.Contains(t, detail, "Basic Auth Enabled: true")
	require.Contains(t, detail, "IdP Mapping Enabled: true")
	require.Contains(t, detail, "Konnect Mapping Enabled: false")
	require.NotContains(t, strings.ToLower(detail), "oidc")
	require.NotContains(t, strings.ToLower(detail), "saml")
}

func TestBuildPortalAuthSettingsChildView_UsesDetailMode(t *testing.T) {
	settings := &kkComps.PortalAuthenticationSettingsResponse{
		BasicAuthEnabled:      true,
		KonnectMappingEnabled: true,
	}

	view := buildPortalAuthSettingsChildView(settings)

	require.Equal(t, tableview.ChildViewModeDetail, view.Mode)
	require.Equal(t, "Authentication Settings", view.Title)
	require.NotNil(t, view.DetailRenderer)
	require.Contains(t, view.DetailRenderer(0), "Basic Auth Enabled: true")
}
