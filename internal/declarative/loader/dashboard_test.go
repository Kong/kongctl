package loader

import (
	"os"
	"path/filepath"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoaderLoadsDashboardDefinitionFromFile(t *testing.T) {
	dir := t.TempDir()

	exportPath := filepath.Join(dir, "dashboard.json")
	require.NoError(t, os.WriteFile(exportPath, []byte(`{
		"id": "dashboard-id",
		"name": "Exported Dashboard",
		"definition": {
			"tiles": [],
			"preset_filters": [
				{
					"field": "control_plane",
					"operator": "in",
					"value": ["cp-id"]
				}
			]
		}
	}`), 0o600))

	configPath := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(`
analytics:
  dashboards:
    - ref: traffic-summary
      name: Traffic Summary
      definition: !file ./dashboard.json#definition
      labels:
        team: platform
`), 0o600))

	rs, err := New().LoadFile(configPath)
	require.NoError(t, err)
	require.Len(t, rs.Dashboards, 1)

	dashboard := rs.Dashboards[0]
	assert.Equal(t, "traffic-summary", dashboard.Ref)
	assert.Equal(t, "Traffic Summary", dashboard.Name)
	assert.NotNil(t, dashboard.Definition.Tiles)
	require.Len(t, dashboard.Definition.PresetFilters, 1)
	assert.Equal(t, kkComps.AllFilterItemsFieldControlPlane, dashboard.Definition.PresetFilters[0].Field)
	assert.Equal(t, kkComps.AllFilterItemsOperatorIn, dashboard.Definition.PresetFilters[0].Operator)
	assert.Equal(t, map[string]string{"team": "platform"}, dashboard.Labels)
}

func TestLoaderDashboardDefinitionPresenceFromYAML(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(`
analytics:
  dashboards:
    - ref: traffic-summary
      name: Traffic Summary
      definition:
        preset_filters:
          - field: control_plane
            operator: in
            value: ["cp-id"]
`), 0o600))

	_, err := New().LoadFile(configPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "definition.tiles is required for dashboard traffic-summary")
}

func TestLoaderRejectsDuplicateDashboardNamesInNamespace(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(`
analytics:
  dashboards:
    - ref: traffic-summary
      name: Traffic Summary
      definition:
        tiles: []
    - ref: traffic-summary-copy
      name: Traffic Summary
      definition:
        tiles: []
`), 0o600))

	_, err := New().LoadFile(configPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate dashboard name 'Traffic Summary' in namespace 'default'")
}
