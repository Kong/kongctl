package helpers

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	kkComponents "github.com/Kong/sdk-konnect-go/models/components"
)

func TestHydratePortalAuthSettingsOIDCConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                string
		settings            *kkComponents.PortalAuthenticationSettingsResponse
		initialOIDCConfig   string
		body                string
		wantErr             bool
		wantIssuer          string
		wantClientID        string
		wantScopes          []string
		wantClaimEmail      string
		expectBodyPreserved bool
	}{
		{
			name: "hydrates missing oidc config from raw body",
			settings: &kkComponents.PortalAuthenticationSettingsResponse{
				BasicAuthEnabled: true,
				OidcAuthEnabled:  true,
			},
			body: `{
					"basic_auth_enabled": true,
					"oidc_auth_enabled": true,
					"oidc_config": {
						"issuer": "https://accounts.google.com",
						"client_id": "client-id-1",
						"scopes": ["openid", "profile"],
						"claim_mappings": {
							"email": "email",
							"groups": "groups",
							"name": "name"
						}
					}
				}`,
			wantIssuer:          "https://accounts.google.com",
			wantClientID:        "client-id-1",
			wantScopes:          []string{"openid", "profile"},
			wantClaimEmail:      "email",
			expectBodyPreserved: true,
		},
		{
			name: "does not overwrite existing oidc config",
			settings: &kkComponents.PortalAuthenticationSettingsResponse{
				BasicAuthEnabled: true,
			},
			initialOIDCConfig: `{"issuer":"https://existing.example.com","client_id":"existing-client"}`,
			body:              `{"oidc_config":{"issuer":"https://new.example.com","client_id":"new-client"}}`,
			wantIssuer:        "https://existing.example.com",
			wantClientID:      "existing-client",
		},
		{
			name: "returns error for invalid json",
			settings: &kkComponents.PortalAuthenticationSettingsResponse{
				BasicAuthEnabled: true,
			},
			body:    `{invalid`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.initialOIDCConfig != "" {
				if err := json.Unmarshal([]byte(tt.initialOIDCConfig), &tt.settings.OidcConfig); err != nil {
					t.Fatalf("failed to initialize oidc config fixture: %v", err)
				}
			}

			resp := &http.Response{
				Body: io.NopCloser(strings.NewReader(tt.body)),
			}

			err := HydratePortalAuthSettingsOIDCConfig(tt.settings, resp)
			if (err != nil) != tt.wantErr {
				t.Fatalf("HydratePortalAuthSettingsOIDCConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}

			if tt.wantIssuer == "" && tt.settings.GetOidcConfig() == nil {
				return
			}

			if got := tt.settings.GetOidcConfig().GetIssuer(); got != tt.wantIssuer {
				t.Fatalf("issuer mismatch: got %q want %q", got, tt.wantIssuer)
			}
			if got := tt.settings.GetOidcConfig().GetClientID(); got != tt.wantClientID {
				t.Fatalf("client id mismatch: got %q want %q", got, tt.wantClientID)
			}
			if len(tt.wantScopes) > 0 {
				gotScopes := tt.settings.GetOidcConfig().GetScopes()
				if len(gotScopes) != len(tt.wantScopes) {
					t.Fatalf("scopes length mismatch: got %d want %d", len(gotScopes), len(tt.wantScopes))
				}
				for i := range tt.wantScopes {
					if gotScopes[i] != tt.wantScopes[i] {
						t.Fatalf("scope[%d] mismatch: got %q want %q", i, gotScopes[i], tt.wantScopes[i])
					}
				}
			}
			if tt.wantClaimEmail != "" {
				if tt.settings.GetOidcConfig().GetClaimMappings() == nil {
					t.Fatalf("expected claim mappings to be set")
				}
				email := tt.settings.GetOidcConfig().GetClaimMappings().GetEmail()
				if email == nil || *email != tt.wantClaimEmail {
					got := "<nil>"
					if email != nil {
						got = *email
					}
					t.Fatalf("claim_mappings.email mismatch: got %q want %q", got, tt.wantClaimEmail)
				}
			}

			if tt.expectBodyPreserved {
				body, readErr := io.ReadAll(resp.Body)
				if readErr != nil {
					t.Fatalf("failed reading response body after hydrate: %v", readErr)
				}
				if strings.TrimSpace(string(body)) != strings.TrimSpace(tt.body) {
					t.Fatalf("response body was not preserved")
				}
			}
		})
	}
}
