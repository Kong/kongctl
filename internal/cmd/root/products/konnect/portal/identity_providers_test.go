package portal

import (
	"strings"
	"testing"
	"time"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/stretchr/testify/require"
)

func TestPortalIdentityProviderDetailView_OmitsLoginPath(t *testing.T) {
	providerID := "11111111-1111-1111-1111-111111111111"
	enabled := true
	providerType := kkComps.IdentityProviderTypeOidc
	now := time.Date(2026, time.April, 21, 12, 30, 0, 0, time.UTC)

	provider := kkComps.IdentityProvider{
		ID:      &providerID,
		Type:    &providerType,
		Enabled: &enabled,
		Config: &kkComps.IdentityProviderConfig{
			OIDCIdentityProviderConfigOutput: &kkComps.OIDCIdentityProviderConfigOutput{
				IssuerURL: "https://issuer.example.test",
				ClientID:  "client-id",
			},
			Type: kkComps.IdentityProviderConfigTypeOIDCIdentityProviderConfigOutput,
		},
		CreatedAt: &now,
		UpdatedAt: &now,
	}

	detail := portalIdentityProviderDetailView(provider)

	require.Contains(t, detail, "type: oidc")
	require.Contains(t, detail, "enabled: true")
	require.Contains(t, detail, "config:")
	require.NotContains(t, strings.ToLower(detail), "login path")
}

func TestBuildPortalIdentityProvidersChildView_UsesCompactCollectionColumns(t *testing.T) {
	providerID := "11111111-1111-1111-1111-111111111111"
	enabled := true
	providerType := kkComps.IdentityProviderTypeOidc

	view := buildPortalIdentityProvidersChildView([]kkComps.IdentityProvider{
		{
			ID:      &providerID,
			Type:    &providerType,
			Enabled: &enabled,
		},
	})

	require.Equal(t, []string{"ID", "TYPE", "ENABLED"}, view.Headers)
	require.Equal(t, "Identity Providers", view.Title)
	require.Len(t, view.Rows, 1)
	require.Equal(t, "oidc", view.Rows[0][1])
	require.Equal(t, "true", view.Rows[0][2])
	require.NotNil(t, view.DetailRenderer)
	require.NotNil(t, view.DetailContext)
}
