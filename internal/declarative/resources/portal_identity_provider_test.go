package resources

import (
	"encoding/json"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/stretchr/testify/require"
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

func TestPortalIdentityProviderResourceValidate_RejectsMismatchedOIDCType(t *testing.T) {
	t.Parallel()

	config := kkComps.CreateCreateIdentityProviderConfigOIDCIdentityProviderConfig(
		kkComps.OIDCIdentityProviderConfig{
			IssuerURL: "https://accounts.google.com",
			ClientID:  "client-id-1",
		},
	)
	resource := PortalIdentityProviderResource{
		Ref: "portal-idp",
		CreateIdentityProvider: kkComps.CreateIdentityProvider{
			Type:   kkComps.IdentityProviderTypeSaml.ToPointer(),
			Config: &config,
		},
	}

	err := resource.Validate()
	require.EqualError(t, err, `identity provider type "saml" does not match oidc config`)
}

func TestPortalIdentityProviderResourceValidate_RejectsMismatchedSAMLType(t *testing.T) {
	t.Parallel()

	config := kkComps.CreateCreateIdentityProviderConfigSAMLIdentityProviderConfigInput(
		kkComps.SAMLIdentityProviderConfigInput{
			IdpMetadataURL: new("https://example.test/saml.xml"),
		},
	)
	resource := PortalIdentityProviderResource{
		Ref: "portal-idp",
		CreateIdentityProvider: kkComps.CreateIdentityProvider{
			Type:   kkComps.IdentityProviderTypeOidc.ToPointer(),
			Config: &config,
		},
	}

	err := resource.Validate()
	require.EqualError(t, err, `identity provider type "oidc" does not match saml config`)
}
