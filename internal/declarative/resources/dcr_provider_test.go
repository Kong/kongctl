package resources

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDCRProviderResourceDefaultsValidationAndPayloads(t *testing.T) {
	provider := DCRProviderResource{
		BaseResource: BaseResource{Ref: "okta-dcr"},
		DisplayName:  "Okta DCR",
		ProviderType: "okta",
		Issuer:       "https://issuer.example.com",
		DCRConfig: map[string]any{
			"initial_client_id": "client-id",
		},
		Labels: map[string]string{"team": "platform"},
	}

	provider.SetDefaults()
	require.NoError(t, provider.Validate())

	assert.Equal(t, ResourceTypeDCRProvider, provider.GetType())
	assert.Equal(t, "okta-dcr", provider.GetMoniker())
	assert.Empty(t, provider.GetDependencies())
	assert.Equal(t, "name[eq]=okta-dcr", provider.GetKonnectMonikerFilter())

	assert.Equal(t, map[string]any{
		"name":          "okta-dcr",
		"display_name":  "Okta DCR",
		"provider_type": "okta",
		"issuer":        "https://issuer.example.com",
		"dcr_config": map[string]any{
			"initial_client_id": "client-id",
		},
		"labels": map[string]string{"team": "platform"},
	}, provider.ToCreatePayload())

	assert.Equal(t, map[string]any{
		"display_name": "Okta DCR",
		"issuer":       "https://issuer.example.com",
		"dcr_config": map[string]any{
			"initial_client_id": "client-id",
		},
		"labels": map[string]string{"team": "platform"},
	}, provider.ToUpdatePayload())
}

func TestDCRProviderResourceValidateRequiresCoreFields(t *testing.T) {
	tests := []struct {
		name     string
		provider DCRProviderResource
		wantErr  string
	}{
		{
			name: "provider_type",
			provider: DCRProviderResource{
				BaseResource: BaseResource{Ref: "dcr"},
				Issuer:       "https://issuer.example.com",
				DCRConfig:    map[string]any{},
			},
			wantErr: "provider_type is required",
		},
		{
			name: "issuer",
			provider: DCRProviderResource{
				BaseResource: BaseResource{Ref: "dcr"},
				ProviderType: "okta",
				DCRConfig:    map[string]any{},
			},
			wantErr: "issuer is required",
		},
		{
			name: "dcr_config",
			provider: DCRProviderResource{
				BaseResource: BaseResource{Ref: "dcr"},
				ProviderType: "okta",
				Issuer:       "https://issuer.example.com",
			},
			wantErr: "dcr_config is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.provider.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}
