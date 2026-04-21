package helpers

import (
	"bytes"
	"context"
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

func TestPortalIdentityProviderAPIImplCreatePortalIdentityProviderStripsEnabledFromCreateBody(t *testing.T) {
	t.Parallel()

	client := &portalIdentityProviderCapturingClient{t: t}
	api := &PortalIdentityProviderAPIImpl{
		SDK:        kkSDK.New(),
		BaseURL:    "https://example.test",
		Token:      "test-token",
		HTTPClient: client,
	}

	config := kkComps.CreateCreateIdentityProviderConfigOIDCIdentityProviderConfig(kkComps.OIDCIdentityProviderConfig{
		IssuerURL: "https://accounts.google.com",
		ClientID:  "client-id-1",
		Scopes:    []string{"openid"},
	})
	enabled := true

	resp, err := api.CreatePortalIdentityProvider(context.Background(), "portal-123", kkComps.CreateIdentityProvider{
		Type:    kkComps.IdentityProviderTypeOidc.ToPointer(),
		Enabled: &enabled,
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

	if _, ok := requestBody["enabled"]; ok {
		t.Fatalf("expected enabled to be removed from create body, got %v", requestBody["enabled"])
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

	if resp == nil || resp.IdentityProvider == nil || resp.IdentityProvider.ID == nil {
		t.Fatalf("expected identity provider response, got %#v", resp)
	}
}
