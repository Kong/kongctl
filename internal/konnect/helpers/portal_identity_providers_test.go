package helpers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

type portalIdentityProviderCapturingClient struct {
	t           *testing.T
	request     *http.Request
	requestBody []byte
}

func (c *portalIdentityProviderCapturingClient) Do(req *http.Request) (*http.Response, error) {
	c.t.Helper()

	c.request = req.Clone(req.Context())
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	c.requestBody = body

	return &http.Response{
		StatusCode: http.StatusCreated,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body: io.NopCloser(bytes.NewReader([]byte(`{
			"id": "provider-1",
			"type": "oidc",
			"enabled": false,
			"created_at": "2026-04-17T00:00:00Z",
			"updated_at": "2026-04-17T00:00:00Z",
			"config": {
				"issuer_url": "https://accounts.google.com",
				"client_id": "client-id-1",
				"scopes": ["openid"]
			}
		}`))),
	}, nil
}

func newPortalIdentityProviderTestAPI(client kkSDK.HTTPClient) *PortalIdentityProviderAPIImpl {
	token := "test-token"
	return &PortalIdentityProviderAPIImpl{SDK: kkSDK.New(
		kkSDK.WithServerURL("https://example.test"),
		kkSDK.WithSecurity(kkComps.Security{PersonalAccessToken: &token}),
		kkSDK.WithClient(client),
	)}
}

func TestPortalIdentityProviderAPIImplCreatePortalIdentityProviderIncludesExplicitEnabled(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		enabled bool
	}{
		{name: "true", enabled: true},
		{name: "false", enabled: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &portalIdentityProviderCapturingClient{t: t}
			api := newPortalIdentityProviderTestAPI(client)
			config := kkComps.CreateCreateIdentityProviderConfigOIDCIdentityProviderConfig(
				kkComps.OIDCIdentityProviderConfig{
					IssuerURL: "https://accounts.google.com",
					ClientID:  "client-id-1",
					Scopes:    []string{"openid"},
				},
			)

			resp, err := api.CreatePortalIdentityProvider(t.Context(), "portal-123", kkComps.CreateIdentityProvider{
				Type:    kkComps.IdentityProviderTypeOidc.ToPointer(),
				Enabled: &tt.enabled,
				Config:  &config,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if client.request == nil {
				t.Fatal("expected request to be captured")
			}
			if client.request.Method != http.MethodPost {
				t.Fatalf("unexpected method: %s", client.request.Method)
			}
			if got := client.request.URL.String(); got != "https://example.test/v3/portals/portal-123/identity-providers" {
				t.Fatalf("unexpected URL: %s", got)
			}

			var requestBody map[string]any
			if err := json.Unmarshal(client.requestBody, &requestBody); err != nil {
				t.Fatalf("failed to decode request body: %v", err)
			}
			if got := requestBody["enabled"]; got != tt.enabled {
				t.Fatalf("unexpected enabled: %v", got)
			}
			if got := requestBody["type"]; got != "oidc" {
				t.Fatalf("unexpected type: %v", got)
			}

			configBody, ok := requestBody["config"].(map[string]any)
			if !ok {
				t.Fatalf("expected config object, got %#v", requestBody["config"])
			}
			if got := configBody["client_id"]; got != "client-id-1" {
				t.Fatalf("unexpected client_id: %v", got)
			}

			if resp == nil || resp.PortalIdentityProvider == nil || resp.PortalIdentityProvider.ID == nil {
				t.Fatalf("expected identity provider response, got %#v", resp)
			}
		})
	}
}

func TestPortalIdentityProviderAPIImplCreatePortalIdentityProviderOmitsLoginPath(t *testing.T) {
	t.Parallel()

	client := &portalIdentityProviderCapturingClient{t: t}
	api := newPortalIdentityProviderTestAPI(client)
	loginPath := "oidc-login"

	_, err := api.CreatePortalIdentityProvider(t.Context(), "portal-123", kkComps.CreateIdentityProvider{
		Type:      kkComps.IdentityProviderTypeOidc.ToPointer(),
		LoginPath: &loginPath,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var requestBody map[string]any
	if err := json.Unmarshal(client.requestBody, &requestBody); err != nil {
		t.Fatalf("failed to decode request body: %v", err)
	}
	if _, ok := requestBody["login_path"]; ok {
		t.Fatalf("expected login_path to be omitted from portal create body, got %v", requestBody["login_path"])
	}
	if _, ok := requestBody["enabled"]; ok {
		t.Fatalf("expected enabled to be omitted from portal create body, got %v", requestBody["enabled"])
	}
}
