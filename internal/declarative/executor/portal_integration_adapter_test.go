package executor

import (
	"context"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPortalIntegrationAdapterMapUpdateFields(t *testing.T) {
	adapter := NewPortalIntegrationAdapter(nil)
	fields := map[string]any{
		planner.FieldGoogleTagManager: map[string]any{
			planner.FieldEnabled: true,
			planner.FieldType:    "tracking",
			planner.FieldConfigData: map[string]any{
				planner.FieldID:        "GTM-ABC123",
				planner.FieldDataLayer: "kongDataLayer",
			},
		},
		planner.FieldGoogleAnalytics4: map[string]any{
			planner.FieldEnabled: false,
			planner.FieldType:    "analytics",
			planner.FieldConfigData: map[string]any{
				planner.FieldID: "G-ABC123",
				planner.FieldL:  "kongLayer",
			},
		},
	}

	var update kkComps.PortalIntegrations
	err := adapter.MapUpdateFields(context.Background(), fields, &update)
	require.NoError(t, err)

	require.NotNil(t, update.GoogleTagManager)
	assert.True(t, update.GoogleTagManager.Enabled)
	assert.Equal(t, "GTM-ABC123", update.GoogleTagManager.ConfigData.ID)
	require.NotNil(t, update.GoogleTagManager.ConfigData.DataLayer)
	assert.Equal(t, "kongDataLayer", *update.GoogleTagManager.ConfigData.DataLayer)

	require.NotNil(t, update.GoogleAnalytics4)
	assert.False(t, update.GoogleAnalytics4.Enabled)
	assert.Equal(t, "G-ABC123", update.GoogleAnalytics4.ConfigData.ID)
	require.NotNil(t, update.GoogleAnalytics4.ConfigData.L)
	assert.Equal(t, "kongLayer", *update.GoogleAnalytics4.ConfigData.L)
}
