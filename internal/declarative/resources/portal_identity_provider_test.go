package resources

import (
	"encoding/json"
	"testing"
)

func TestPortalIdentityProviderResourceUnmarshalJSON_OmittedEnabledStaysNil(t *testing.T) {
	t.Parallel()

	input := []byte(`{
		"ref": "portal-oidc",
		"portal": "portal-1",
		"type": "oidc",
		"config": {
			"issuer_url": "https://accounts.google.com",
			"client_id": "client-id-1",
			"client_secret": "client-secret-1",
			"scopes": ["openid"]
		}
	}`)

	var resource PortalIdentityProviderResource
	if err := json.Unmarshal(input, &resource); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resource.Enabled != nil {
		t.Fatalf("expected enabled to remain nil when omitted, got %v", *resource.Enabled)
	}
}

func TestPortalIdentityProviderResourceUnmarshalJSON_PreservesExplicitEnabled(t *testing.T) {
	t.Parallel()

	input := []byte(`{
		"ref": "portal-oidc",
		"portal": "portal-1",
		"type": "oidc",
		"enabled": true,
		"config": {
			"issuer_url": "https://accounts.google.com",
			"client_id": "client-id-1",
			"client_secret": "client-secret-1",
			"scopes": ["openid"]
		}
	}`)

	var resource PortalIdentityProviderResource
	if err := json.Unmarshal(input, &resource); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resource.Enabled == nil {
		t.Fatal("expected enabled to be set")
	}
	if !*resource.Enabled {
		t.Fatal("expected enabled to be true")
	}
}
