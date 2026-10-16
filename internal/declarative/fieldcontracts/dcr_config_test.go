package fieldcontracts_test

import (
	"encoding/json"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/fieldcontracts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDCRConfigProviderStages(t *testing.T) {
	providers := []struct {
		name       string
		union      kkComps.DcrConfigType
		observable []string
		secret     string
		booleans   []string
	}{
		{
			"auth0", kkComps.DcrConfigTypeUpdateDcrConfigAuth0InRequest,
			[]string{"initial_client_id", "initial_client_audience"},
			"initial_client_secret",
			[]string{"use_developer_managed_scopes"},
		},
		{
			"azureAd", kkComps.DcrConfigTypeUpdateDcrConfigAzureAdInRequest,
			[]string{"initial_client_id"},
			"initial_client_secret", nil,
		},
		{
			"curity", kkComps.DcrConfigTypeUpdateDcrConfigCurityInRequest,
			[]string{"initial_client_id"},
			"initial_client_secret", nil,
		},
		{"okta", kkComps.DcrConfigTypeUpdateDcrConfigOktaInRequest, nil, "dcr_token", nil},
		{
			"http", kkComps.DcrConfigTypeUpdateDcrConfigHTTPInRequest,
			[]string{"dcr_base_url"},
			"api_key",
			[]string{"disable_event_hooks", "disable_refresh_secret", "allow_multiple_credentials"},
		},
	}
	for _, provider := range providers {
		t.Run(provider.name, func(t *testing.T) {
			assertMapped := func(t *testing.T, config map[string]any, want map[string]any) {
				t.Helper()
				mapped, err := fieldcontracts.MapDCRConfigUpdate(provider.name, config)
				require.NoError(t, err)
				require.NotNil(t, mapped)
				assert.Equal(t, provider.union, mapped.Type)
				data, err := json.Marshal(mapped)
				require.NoError(t, err)
				var actual map[string]any
				require.NoError(t, json.Unmarshal(data, &actual))
				assert.Equal(t, want, actual)
			}
			assertMapped(t, nil, map[string]any{})
			assertMapped(t, map[string]any{}, map[string]any{})
			unknown := map[string]any{"unrecognized": true}
			require.NoError(t, fieldcontracts.ValidateDCRConfig(provider.name, unknown))
			assert.False(t, fieldcontracts.DCRConfigChanged(provider.name, nil, unknown))
			assertMapped(t, unknown, map[string]any{})

			for _, key := range append(append([]string{}, provider.observable...), provider.secret) {
				t.Run(key, func(t *testing.T) {
					desired := map[string]any{key: ""}
					require.NoError(t, fieldcontracts.ValidateDCRConfig(provider.name, desired))
					assert.Equal(
						t,
						key != provider.secret,
						fieldcontracts.DCRConfigChanged(provider.name, nil, desired),
					)
					assert.False(t, fieldcontracts.DCRConfigChanged(provider.name, desired, desired))
					assert.False(t, fieldcontracts.DCRConfigChanged(provider.name, desired, nil))
					assertMapped(t, desired, desired)
					// Saved-plan mapping silently ignores wrong types, even though ordinary
					// validation only type-checks booleans and comparison retains raw values.
					malformed := map[string]any{key: 42}
					require.NoError(t, fieldcontracts.ValidateDCRConfig(provider.name, malformed))
					assertMapped(t, malformed, map[string]any{})
				})
			}
			for _, key := range provider.booleans {
				t.Run(key, func(t *testing.T) {
					desired := map[string]any{key: false}
					require.NoError(t, fieldcontracts.ValidateDCRConfig(provider.name, desired))
					assert.False(t, fieldcontracts.DCRConfigChanged(provider.name, nil, desired))
					assert.True(t, fieldcontracts.DCRConfigChanged(provider.name, map[string]any{key: nil}, desired))
					assert.True(t, fieldcontracts.DCRConfigChanged(provider.name, map[string]any{key: true}, desired))
					assertMapped(t, desired, desired)
					desired[key] = true
					assert.True(t, fieldcontracts.DCRConfigChanged(provider.name, nil, desired))
					assertMapped(t, desired, desired)
					desired[key] = nil
					require.NoError(t, fieldcontracts.ValidateDCRConfig(provider.name, desired))
					assert.True(t, fieldcontracts.DCRConfigChanged(provider.name, nil, desired))
					assert.False(t, fieldcontracts.DCRConfigChanged(provider.name, desired, desired))
					assertMapped(t, desired, map[string]any{})
					desired[key] = "false"
					require.EqualError(t, fieldcontracts.ValidateDCRConfig(provider.name, desired),
						"dcr_config."+key+" must be a boolean")
					assertMapped(t, desired, map[string]any{})
				})
			}
		})
	}
}

func TestDCRConfigValidationAndMappingErrors(t *testing.T) {
	config := map[string]any{
		"disable_event_hooks": "false", "disable_refresh_secret": "false", "allow_multiple_credentials": "false",
	}
	for _, key := range []string{"disable_event_hooks", "disable_refresh_secret", "allow_multiple_credentials"} {
		require.EqualError(t, fieldcontracts.ValidateDCRConfig("http", config), "dcr_config."+key+" must be a boolean")
		delete(config, key)
	}
	require.NoError(t, fieldcontracts.ValidateDCRConfig("unrecognized", map[string]any{"disable_event_hooks": "false"}))
	assert.False(t, fieldcontracts.DCRConfigChanged("unrecognized", nil, map[string]any{"initial_client_id": "id"}))

	for _, provider := range []string{"http", "unrecognized"} {
		for _, raw := range []any{nil, "invalid", []any{}, map[string]string{}} {
			mapped, err := fieldcontracts.MapDCRConfigUpdate(provider, raw)
			require.EqualError(t, err, "DCR config must be an object")
			assert.Nil(t, mapped)
		}
	}
	mapped, err := fieldcontracts.MapDCRConfigUpdate("unrecognized", map[string]any{})
	require.EqualError(t, err, "unsupported DCR provider type: unrecognized")
	assert.Nil(t, mapped)
}
