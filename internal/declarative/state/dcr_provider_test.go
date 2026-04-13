package state

import (
	"encoding/json"
	"testing"

	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeDCRProviderFromAny(t *testing.T) {
	payload := json.RawMessage(`{
		"created_at": "2026-03-13T17:15:08.497Z",
		"updated_at": "2026-03-13T17:15:08.497Z",
		"id": "d67a4203-b1e8-4631-a626-5fe7c55efe88",
		"name": "test-okta-dcr-provider",
		"display_name": "Test Okta DCR Provider",
		"provider_type": "okta",
		"issuer": "https://example.com",
		"dcr_config": {},
		"labels": {
			"KONGCTL-namespace": "default",
			"team": "platform"
		},
		"active": false
	}`)

	provider, err := normalizeDCRProviderFromAny(payload)
	require.NoError(t, err)
	require.NotNil(t, provider)

	assert.Equal(t, "d67a4203-b1e8-4631-a626-5fe7c55efe88", provider.ID)
	assert.Equal(t, "test-okta-dcr-provider", provider.Name)
	assert.Equal(t, "Test Okta DCR Provider", provider.DisplayName)
	assert.True(t, provider.DisplayNameSet)
	assert.Equal(t, "okta", provider.ProviderType)
	assert.Equal(t, "https://example.com", provider.Issuer)
	assert.Equal(t, map[string]any{}, provider.DCRConfig)
	assert.Equal(t, map[string]string{
		labels.NamespaceKey: "default",
		"team":              "platform",
	}, provider.NormalizedLabels)
}
