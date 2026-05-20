package resources

import (
	"fmt"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func TestApplicationAuthStrategyResource_UnmarshalJSON_KeyAuthConfigAliases(t *testing.T) {
	tests := []struct {
		name      string
		configKey string
	}{
		{
			name:      "underscore config key",
			configKey: "key_auth",
		},
		{
			name:      "hyphen config key",
			configKey: "key-auth",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := fmt.Sprintf(`
ref: key-auth
name: key-auth
display_name: Key Auth
strategy_type: key_auth
configs:
  %s:
    key_names:
      - X-API-Key
      - Api-Key
`, tt.configKey)

			var strategy ApplicationAuthStrategyResource
			err := yaml.Unmarshal([]byte(input), &strategy)
			require.NoError(t, err)

			assert.Equal(t, kkComps.CreateAppAuthStrategyRequestTypeKeyAuth, strategy.Type)
			require.NotNil(t, strategy.AppAuthStrategyKeyAuthRequest)
			assert.Equal(
				t,
				[]string{"X-API-Key", "Api-Key"},
				strategy.AppAuthStrategyKeyAuthRequest.Configs.KeyAuth.KeyNames,
			)
		})
	}
}

func TestApplicationAuthStrategyResource_UnmarshalJSON_OIDCConfigAliases(t *testing.T) {
	tests := []struct {
		name      string
		configKey string
	}{
		{
			name:      "underscore config key",
			configKey: "openid_connect",
		},
		{
			name:      "hyphen config key",
			configKey: "openid-connect",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := fmt.Sprintf(`
ref: oidc
name: oidc
display_name: OIDC
strategy_type: openid_connect
configs:
  %s:
    issuer: https://issuer.example.com
    credential_claim:
      - sub
    auth_methods:
      - bearer
    scopes:
      - openid
      - profile
`, tt.configKey)

			var strategy ApplicationAuthStrategyResource
			err := yaml.Unmarshal([]byte(input), &strategy)
			require.NoError(t, err)

			assert.Equal(t, kkComps.CreateAppAuthStrategyRequestTypeOpenidConnect, strategy.Type)
			require.NotNil(t, strategy.AppAuthStrategyOpenIDConnectRequest)
			assert.Equal(
				t,
				"https://issuer.example.com",
				strategy.AppAuthStrategyOpenIDConnectRequest.Configs.OpenidConnect.Issuer,
			)
			assert.Equal(
				t,
				[]string{"openid", "profile"},
				strategy.AppAuthStrategyOpenIDConnectRequest.Configs.OpenidConnect.Scopes,
			)
		})
	}
}

func TestApplicationAuthStrategyResource_UnmarshalJSON_OIDCMissingOptionalSlicesRemainNil(t *testing.T) {
	tests := []struct {
		name      string
		configKey string
	}{
		{
			name:      "underscore config key",
			configKey: "openid_connect",
		},
		{
			name:      "hyphen config key",
			configKey: "openid-connect",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := fmt.Sprintf(`
ref: oidc
name: oidc
display_name: OIDC
strategy_type: openid_connect
configs:
  %s:
    issuer: https://issuer.example.com
    scopes:
      - openid
`, tt.configKey)

			var strategy ApplicationAuthStrategyResource
			err := yaml.Unmarshal([]byte(input), &strategy)
			require.NoError(t, err)

			require.NotNil(t, strategy.AppAuthStrategyOpenIDConnectRequest)
			cfg := strategy.AppAuthStrategyOpenIDConnectRequest.Configs.OpenidConnect
			assert.Nil(t, cfg.CredentialClaim)
			assert.Nil(t, cfg.AuthMethods)
			assert.Equal(t, []string{"openid"}, cfg.Scopes)
		})
	}
}

func TestApplicationAuthStrategyResource_UnmarshalJSON_OIDCDCRProviderReference(t *testing.T) {
	input := `
ref: oidc
name: oidc
display_name: OIDC
strategy_type: openid_connect
dcr_provider_id: okta-dcr
configs:
  openid_connect:
    issuer: https://issuer.example.com
    scopes:
      - openid
`

	var strategy ApplicationAuthStrategyResource
	err := yaml.Unmarshal([]byte(input), &strategy)
	require.NoError(t, err)

	assert.Equal(t, "okta-dcr", strategy.GetDCRProviderID())
	require.NotNil(t, strategy.AppAuthStrategyOpenIDConnectRequest)
	assert.Equal(t, "okta-dcr", *strategy.AppAuthStrategyOpenIDConnectRequest.DcrProviderID)
	assert.Equal(t, []ResourceRef{
		{Kind: ResourceTypeDCRProvider, Ref: "okta-dcr"},
	}, strategy.GetDependencies())
}
