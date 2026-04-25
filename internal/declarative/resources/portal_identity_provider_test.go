package resources

import (
	"encoding/json"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func TestPortalIdentityProviderResourceMarshalIncludesMetadata(t *testing.T) {
	t.Parallel()

	enabled := false
	loginPath := "/login/oidc"
	config := kkComps.CreateCreateIdentityProviderConfigOIDCIdentityProviderConfig(
		kkComps.OIDCIdentityProviderConfig{
			IssuerURL: "https://accounts.google.com",
			ClientID:  "client-id-1",
			Scopes:    []string{"openid", "email"},
		},
	)
	resource := PortalIdentityProviderResource{
		Ref:    "portal-oidc",
		Portal: "portal-1",
		CreateIdentityProvider: kkComps.CreateIdentityProvider{
			Type:      kkComps.IdentityProviderTypeOidc.ToPointer(),
			Enabled:   &enabled,
			LoginPath: &loginPath,
			Config:    &config,
		},
	}

	raw, err := json.Marshal(resource)
	require.NoError(t, err)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(raw, &payload))

	require.Equal(t, "portal-oidc", payload["ref"])
	require.Equal(t, "portal-1", payload["portal"])
	require.Equal(t, "oidc", payload["type"])
	require.Equal(t, false, payload["enabled"])
	require.Equal(t, "/login/oidc", payload["login_path"])

	configPayload, ok := payload["config"].(map[string]any)
	require.True(t, ok, "expected config payload, got %v", payload["config"])
	require.Equal(t, "client-id-1", configPayload["client_id"])
	require.Equal(t, "https://accounts.google.com", configPayload["issuer_url"])

	yamlBytes, err := yaml.Marshal(resource)
	require.NoError(t, err)

	output := string(yamlBytes)
	require.Contains(t, output, "ref: portal-oidc")
	require.Contains(t, output, "portal: portal-1")
	require.Contains(t, output, "type: oidc")
	require.NotContains(t, output, "konnectID")
}

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
