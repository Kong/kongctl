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
		ID:           "provider-id",
		Name:         "okta-dcr",
		DisplayName:  "Old Okta DCR",
		ProviderType: "okta",
		Issuer:       "https://old.example.com",
		DCRConfig: map[string]any{
			"initial_client_id": "old-client",
		},
		NormalizedLabels: map[string]string{
			labels.NamespaceKey: "default",
			"team":              "platform",
		},
	}
	desired := resources.DCRProviderResource{
		BaseResource: resources.BaseResource{Ref: "okta-dcr"},
		Name:         "okta-dcr",
		DisplayName:  "New Okta DCR",
		ProviderType: "okta",
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
