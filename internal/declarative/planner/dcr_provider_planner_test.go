package planner

import (
	"testing"

	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDCRProviderPlannerShouldUpdateDCRProvider(t *testing.T) {
	planner := &dcrProviderPlannerImpl{}
	current := state.DCRProvider{
		ID:             "provider-id",
		Name:           "auth0-dcr",
		DisplayName:    "Old Okta DCR",
		DisplayNameSet: true,
		ProviderType:   "auth0",
		Issuer:         "https://old.example.com",
		DCRConfig: map[string]any{
			"initial_client_id": "old-client",
		},
		NormalizedLabels: map[string]string{
			labels.NamespaceKey: "default",
			"team":              "platform",
		},
	}
	desired := resources.DCRProviderResource{
		BaseResource: resources.BaseResource{Ref: "auth0-dcr"},
		Name:         "auth0-dcr",
		DisplayName:  "New Okta DCR",
		ProviderType: "auth0",
		Issuer:       "https://new.example.com",
		DCRConfig: map[string]any{
			"initial_client_id": "new-client",
		},
		Labels: map[string]string{"team": "identity"},
	}

	needsUpdate, fields, changedFields := planner.shouldUpdateDCRProvider(current, desired)
	require.True(t, needsUpdate)
	assert.Equal(t, "New Okta DCR", fields["display_name"])
	assert.Equal(t, "https://new.example.com", fields["issuer"])
	assert.Equal(t, map[string]any{"initial_client_id": "new-client"}, fields["dcr_config"])
	assert.Equal(t, map[string]string{"team": "identity"}, fields["labels"])
	assert.NotContains(t, fields, FieldError)
	assert.Contains(t, changedFields, "display_name")
	assert.Contains(t, changedFields, "issuer")
	assert.Contains(t, changedFields, "dcr_config")
	assert.Contains(t, changedFields, "labels")
}

func TestDCRProviderPlannerRejectsProviderTypeChange(t *testing.T) {
	planner := &dcrProviderPlannerImpl{}
	current := state.DCRProvider{
		Name:         "dcr-provider",
		ProviderType: "okta",
	}
	desired := resources.DCRProviderResource{
		Name:         "dcr-provider",
		ProviderType: "auth0",
	}

	needsUpdate, fields, changedFields := planner.shouldUpdateDCRProvider(current, desired)
	require.True(t, needsUpdate)
	assert.Empty(t, changedFields)
	require.Contains(t, fields, FieldError)
	assert.Contains(t, fields[FieldError], "changing provider_type from okta to auth0 is not supported")
}

func TestDCRProviderPlannerIgnoresUnobservableResponseFields(t *testing.T) {
	planner := &dcrProviderPlannerImpl{}

	t.Run("okta write-only dcr token and omitted display name", func(t *testing.T) {
		current := state.DCRProvider{
			ID:           "provider-id",
			Name:         "okta-dcr",
			ProviderType: "okta",
			Issuer:       "https://issuer.example.com",
			DCRConfig:    map[string]any{},
		}
		desired := resources.DCRProviderResource{
			Name:         "okta-dcr",
			DisplayName:  "Okta DCR Provider",
			ProviderType: "okta",
			Issuer:       "https://issuer.example.com",
			DCRConfig: map[string]any{
				"dcr_token": "write-only-token",
			},
		}

		needsUpdate, fields, changedFields := planner.shouldUpdateDCRProvider(current, desired)
		require.False(t, needsUpdate)
		assert.Empty(t, fields)
		assert.Empty(t, changedFields)
	})

	t.Run("http write-only api key and omitted response defaults", func(t *testing.T) {
		current := state.DCRProvider{
			ID:           "provider-id",
			Name:         "http-dcr",
			ProviderType: "http",
			Issuer:       "https://issuer.example.com",
			DCRConfig: map[string]any{
				"dcr_base_url":               "https://dcr.example.com/v1/dcr",
				"allow_multiple_credentials": false,
			},
		}
		desired := resources.DCRProviderResource{
			Name:         "http-dcr",
			DisplayName:  "HTTP DCR Provider",
			ProviderType: "http",
			Issuer:       "https://issuer.example.com",
			DCRConfig: map[string]any{
				"dcr_base_url": "https://dcr.example.com/v1/dcr",
				"api_key":      "write_only_api_key",
			},
		}

		needsUpdate, fields, changedFields := planner.shouldUpdateDCRProvider(current, desired)
		require.False(t, needsUpdate)
		assert.Empty(t, fields)
		assert.Empty(t, changedFields)
	})

	t.Run("auth0 missing false default does not trigger update", func(t *testing.T) {
		current := state.DCRProvider{
			ID:           "provider-id",
			Name:         "auth0-dcr",
			ProviderType: "auth0",
			Issuer:       "https://issuer.example.com",
			DCRConfig: map[string]any{
				"initial_client_id":       "auth0_initial_client_id",
				"initial_client_audience": "https://audience.example.com",
			},
		}
		desired := resources.DCRProviderResource{
			Name:         "auth0-dcr",
			ProviderType: "auth0",
			Issuer:       "https://issuer.example.com",
			DCRConfig: map[string]any{
				"initial_client_id":            "auth0_initial_client_id",
				"initial_client_audience":      "https://audience.example.com",
				"use_developer_managed_scopes": false,
			},
		}

		needsUpdate, fields, changedFields := planner.shouldUpdateDCRProvider(current, desired)
		require.False(t, needsUpdate)
		assert.Empty(t, fields)
		assert.Empty(t, changedFields)
	})
}

func TestDCRProviderPlannerNormalizesIssuerTrailingSlash(t *testing.T) {
	planner := &dcrProviderPlannerImpl{}
	current := state.DCRProvider{
		ID:           "provider-id",
		Name:         "auth0-dcr",
		ProviderType: "auth0",
		Issuer:       "https://my-issuer.auth0.com/api/v2",
		DCRConfig: map[string]any{
			"grant_types": []any{"client_credentials"},
		},
	}
	desired := resources.DCRProviderResource{
		BaseResource: resources.BaseResource{Ref: "auth0-dcr"},
		Name:         "auth0-dcr",
		ProviderType: "auth0",
		Issuer:       "https://my-issuer.auth0.com/api/v2/",
		DCRConfig: map[string]any{
			"grant_types": []any{"client_credentials"},
		},
	}

	needsUpdate, fields, changedFields := planner.shouldUpdateDCRProvider(current, desired)
	require.False(t, needsUpdate)
	assert.Empty(t, fields)
	assert.Empty(t, changedFields)
}

func TestDCRProviderOnlyConfigContributesNamespace(t *testing.T) {
	namespace := "dcr-providers-example"
	planner := &Planner{}
	rs := &resources.ResourceSet{
		DCRProviders: []resources.DCRProviderResource{
			{
				BaseResource: resources.BaseResource{
					Ref: "okta-dcr",
					Kongctl: &resources.KongctlMeta{
						Namespace: &namespace,
					},
				},
				Name:         "okta-dcr-provider",
				ProviderType: "okta",
				Issuer:       "https://my-issuer.okta.com/default",
				DCRConfig: map[string]any{
					"dcr_token": "replace-with-okta-dcr-token",
				},
			},
		},
	}

	namespaces := planner.getResourceNamespaces(rs)
	require.Equal(t, []string{"dcr-providers-example"}, namespaces)
}
