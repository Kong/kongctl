package resources

import (
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func TestPortalIntegrationUnmarshalNested(t *testing.T) {
	yamlConfig := `
portals:
  - ref: portal-integrations
    name: Portal Integrations
    integrations:
      ref: portal-integrations-config
      google_tag_manager:
        enabled: true
        config_data:
          id: GTM-ABC123
      google_analytics_4:
        enabled: false
        config_data:
          id: G-ABC123
`

	var parsed struct {
		Portals []PortalResource `yaml:"portals"`
	}

	require.NoError(t, yaml.UnmarshalStrict([]byte(yamlConfig), &parsed))
	require.Len(t, parsed.Portals, 1)
	require.NotNil(t, parsed.Portals[0].Integrations)

	integration := parsed.Portals[0].Integrations
	integration.SetDefaults()

	assert.Equal(t, "portal-integrations-config", integration.Ref)
	assert.Equal(t, "GTM-ABC123", integration.GoogleTagManager.ConfigData.ID)
	assert.Equal(t, "tracking", string(integration.GoogleTagManager.Type))
	assert.Equal(t, "G-ABC123", integration.GoogleAnalytics4.ConfigData.ID)
	assert.Equal(t, "analytics", string(integration.GoogleAnalytics4.Type))
}

func TestPortalIntegrationValidateRejectsInvalidIDs(t *testing.T) {
	resource := PortalIntegrationResource{
		Ref: "portal-integrations",
	}

	resource.GoogleTagManager = nil
	resource.GoogleAnalytics4 = nil
	assert.NoError(t, resource.Validate())

	gtm := testGoogleTagManagerIntegration("not-gtm")
	resource.GoogleTagManager = &gtm
	err := resource.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "google_tag_manager config_data.id")

	resource.GoogleTagManager = nil
	ga4 := testGoogleAnalytics4Integration("not-ga")
	resource.GoogleAnalytics4 = &ga4
	err = resource.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "google_analytics_4 config_data.id")
}

func testGoogleTagManagerIntegration(id string) kkComps.GoogleTagManagerIntegration {
	return kkComps.GoogleTagManagerIntegration{
		Enabled: true,
		Type:    kkComps.GoogleTagManagerIntegrationTypeTracking,
		ConfigData: kkComps.ConfigData{
			ID: id,
		},
	}
}

func testGoogleAnalytics4Integration(id string) kkComps.GoogleAnalytics4Integration {
	return kkComps.GoogleAnalytics4Integration{
		Enabled: true,
		Type:    kkComps.GoogleAnalytics4IntegrationTypeAnalytics,
		ConfigData: kkComps.GoogleAnalytics4IntegrationConfigData{
			ID: id,
		},
	}
}
