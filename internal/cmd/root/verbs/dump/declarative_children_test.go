package dump

import (
	"context"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	declstate "github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/require"
)

func TestNormalizePortalPageSlug(t *testing.T) {
	tests := []struct {
		name        string
		rawSlug     string
		parentPage  string
		wantSlug    string
		expectError bool
	}{
		{
			name:       "root slug",
			rawSlug:    "/",
			parentPage: "",
			wantSlug:   "/",
		},
		{
			name:        "root slug with parent",
			rawSlug:     "/",
			parentPage:  "parent-id",
			expectError: true,
		},
		{
			name:     "leading slash",
			rawSlug:  "/apis",
			wantSlug: "apis",
		},
		{
			name:     "trailing slash",
			rawSlug:  "apis/",
			wantSlug: "apis",
		},
		{
			name:     "no slashes",
			rawSlug:  "apis",
			wantSlug: "apis",
		},
		{
			name:     "trim whitespace",
			rawSlug:  "  /getting-started  ",
			wantSlug: "getting-started",
		},
		{
			name:        "multi segment with leading slash",
			rawSlug:     "/guides/publish-apis",
			expectError: true,
		},
		{
			name:        "multi segment without leading slash",
			rawSlug:     "guides/publish-apis",
			expectError: true,
		},
		{
			name:        "empty slug",
			rawSlug:     "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizePortalPageSlug(tt.rawSlug, tt.parentPage)
			if tt.expectError {
				if err == nil {
					t.Fatalf("expected error, got nil (slug=%q)", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.wantSlug {
				t.Fatalf("expected slug %q, got %q", tt.wantSlug, got)
			}
		})
	}
}

func TestResolveAPIPublicationRef(t *testing.T) {
	apiID := "api-123"
	portalID := "portal-456"

	t.Run("uses publication id when provided", func(t *testing.T) {
		ref, err := resolveAPIPublicationRef(apiID, portalID, "pub-789")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ref != "pub-789" {
			t.Fatalf("expected ref to be publication id, got %q", ref)
		}
	})

	t.Run("generates ref when id missing", func(t *testing.T) {
		ref, err := resolveAPIPublicationRef(apiID, portalID, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := buildChildRef("api-publication", apiID, portalID)
		if ref != expected {
			t.Fatalf("expected ref %q, got %q", expected, ref)
		}
	})

	t.Run("errors when portal id missing", func(t *testing.T) {
		if _, err := resolveAPIPublicationRef(apiID, "", ""); err == nil {
			t.Fatalf("expected error, got nil")
		}
	})
}

type stubDumpPortalAuthSettingsAPI struct {
	getResponse *kkOps.GetPortalAuthenticationSettingsResponse
	getErr      error
}

func (s *stubDumpPortalAuthSettingsAPI) UpdatePortalAuthenticationSettings(
	context.Context,
	string,
	*kkComps.PortalAuthenticationSettingsUpdateRequest,
	...kkOps.Option,
) (*kkOps.UpdatePortalAuthenticationSettingsResponse, error) {
	return nil, nil
}

func (s *stubDumpPortalAuthSettingsAPI) GetPortalAuthenticationSettings(
	_ context.Context,
	_ string,
	_ ...kkOps.Option,
) (*kkOps.GetPortalAuthenticationSettingsResponse, error) {
	if s.getResponse != nil || s.getErr != nil {
		return s.getResponse, s.getErr
	}
	return &kkOps.GetPortalAuthenticationSettingsResponse{
		PortalAuthenticationSettingsResponse: &kkComps.PortalAuthenticationSettingsResponse{},
	}, nil
}

type stubDumpPortalIdentityProviderAPI struct {
	listResponse *kkOps.GetPortalIdentityProvidersResponse
	listErr      error
}

func (s *stubDumpPortalIdentityProviderAPI) ListPortalIdentityProviders(
	_ context.Context,
	_ kkOps.GetPortalIdentityProvidersRequest,
	_ ...kkOps.Option,
) (*kkOps.GetPortalIdentityProvidersResponse, error) {
	if s.listResponse != nil || s.listErr != nil {
		return s.listResponse, s.listErr
	}
	return &kkOps.GetPortalIdentityProvidersResponse{}, nil
}

func (s *stubDumpPortalIdentityProviderAPI) GetPortalIdentityProvider(
	context.Context,
	string,
	string,
	...kkOps.Option,
) (*kkOps.GetPortalIdentityProviderResponse, error) {
	return nil, nil
}

func (s *stubDumpPortalIdentityProviderAPI) CreatePortalIdentityProvider(
	context.Context,
	string,
	kkComps.CreateIdentityProvider,
	...kkOps.Option,
) (*kkOps.CreatePortalIdentityProviderResponse, error) {
	return nil, nil
}

func (s *stubDumpPortalIdentityProviderAPI) UpdatePortalIdentityProvider(
	context.Context,
	kkOps.UpdatePortalIdentityProviderRequest,
	...kkOps.Option,
) (*kkOps.UpdatePortalIdentityProviderResponse, error) {
	return nil, nil
}

func (s *stubDumpPortalIdentityProviderAPI) DeletePortalIdentityProvider(
	context.Context,
	string,
	string,
	...kkOps.Option,
) (*kkOps.DeletePortalIdentityProviderResponse, error) {
	return nil, nil
}

func TestBuildPortalAuthSettings_IncludesOnlySupportedFields(t *testing.T) {
	t.Parallel()

	client := declstate.NewClient(declstate.ClientConfig{
		PortalAuthSettingsAPI: &stubDumpPortalAuthSettingsAPI{
			getResponse: &kkOps.GetPortalAuthenticationSettingsResponse{
				PortalAuthenticationSettingsResponse: &kkComps.PortalAuthenticationSettingsResponse{
					BasicAuthEnabled:       true,
					KonnectMappingEnabled:  true,
					IdpMappingEnabled:      boolPtr(false),
					OidcAuthEnabled:        true,
					SamlAuthEnabled:        boolPtr(true),
					OidcTeamMappingEnabled: true,
				},
			},
		},
	})

	resource, err := buildPortalAuthSettings(context.Background(), client, "portal-1")
	require.NoError(t, err)
	require.NotNil(t, resource)
	require.NotEmpty(t, resource.Ref)
	require.NotNil(t, resource.BasicAuthEnabled)
	require.True(t, *resource.BasicAuthEnabled)
	require.NotNil(t, resource.KonnectMappingEnabled)
	require.True(t, *resource.KonnectMappingEnabled)
	require.NotNil(t, resource.IdpMappingEnabled)
	require.False(t, *resource.IdpMappingEnabled)
	require.Nil(t, resource.OidcAuthEnabled)
	require.Nil(t, resource.SamlAuthEnabled)
	require.Nil(t, resource.OidcTeamMappingEnabled)
	require.Nil(t, resource.OidcIssuer)
	require.Nil(t, resource.OidcClientID)
	require.Nil(t, resource.OidcClientSecret)
	require.Nil(t, resource.OidcClaimMappings)
	require.Nil(t, resource.OidcScopes)
}

func TestBuildPortalIdentityProviders_MapsOIDCAndSAMLChildren(t *testing.T) {
	t.Parallel()

	oidcID := "portal-idp-oidc"
	samlID := "portal-idp-saml"
	oidcEnabled := true
	samlEnabled := false
	oidcType := kkComps.IdentityProviderTypeOidc
	samlType := kkComps.IdentityProviderTypeSaml

	client := declstate.NewClient(declstate.ClientConfig{
		PortalIdentityProviderAPI: &stubDumpPortalIdentityProviderAPI{
			listResponse: &kkOps.GetPortalIdentityProvidersResponse{
				IdentityProviders: []kkComps.IdentityProvider{
					{
						ID:      &oidcID,
						Type:    &oidcType,
						Enabled: &oidcEnabled,
						Config: &kkComps.IdentityProviderConfig{
							Type: kkComps.IdentityProviderConfigTypeOIDCIdentityProviderConfigOutput,
							OIDCIdentityProviderConfigOutput: &kkComps.OIDCIdentityProviderConfigOutput{
								IssuerURL: "https://issuer.example.test",
								ClientID:  "client-id",
								Scopes:    []string{"openid", "email"},
								ClaimMappings: &kkComps.OIDCIdentityProviderClaimMappings{
									Email: stringPtr("email"),
								},
							},
						},
					},
					{
						ID:      &samlID,
						Type:    &samlType,
						Enabled: &samlEnabled,
						Config: &kkComps.IdentityProviderConfig{
							Type: kkComps.IdentityProviderConfigTypeSAMLIdentityProviderConfig,
							SAMLIdentityProviderConfig: &kkComps.SAMLIdentityProviderConfig{
								IdpMetadataURL: stringPtr("https://issuer.example.test/saml.xml"),
							},
						},
					},
				},
			},
		},
	})

	resources, err := buildPortalIdentityProviders(context.Background(), client, "portal-1")
	require.NoError(t, err)
	require.Len(t, resources, 2)

	require.Equal(t, kkComps.IdentityProviderTypeOidc, *resources[0].Type)
	require.Equal(t, oidcEnabled, *resources[0].Enabled)
	require.NotNil(t, resources[0].Config)
	require.Equal(t, kkComps.CreateIdentityProviderConfigTypeOIDCIdentityProviderConfig, resources[0].Config.Type)
	require.NotNil(t, resources[0].Config.OIDCIdentityProviderConfig)
	require.Equal(t, "https://issuer.example.test", resources[0].Config.OIDCIdentityProviderConfig.IssuerURL)
	require.Equal(t, "client-id", resources[0].Config.OIDCIdentityProviderConfig.ClientID)
	require.Nil(t, resources[0].Config.OIDCIdentityProviderConfig.ClientSecret)

	require.Equal(t, kkComps.IdentityProviderTypeSaml, *resources[1].Type)
	require.Equal(t, samlEnabled, *resources[1].Enabled)
	require.NotNil(t, resources[1].Config)
	require.Equal(t, kkComps.CreateIdentityProviderConfigTypeSAMLIdentityProviderConfigInput, resources[1].Config.Type)
	require.NotNil(t, resources[1].Config.SAMLIdentityProviderConfigInput)
	require.Equal(
		t,
		"https://issuer.example.test/saml.xml",
		*resources[1].Config.SAMLIdentityProviderConfigInput.IdpMetadataURL,
	)
}

func boolPtr(value bool) *bool {
	return &value
}

func stringPtr(value string) *string {
	return &value
}
