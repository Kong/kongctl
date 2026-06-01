package executor

import (
	"context"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPortalCustomizationAdapterMapUpdateFieldsSpecRendererAndRobots(t *testing.T) {
	adapter := NewPortalCustomizationAdapter(nil)
	fields := map[string]any{
		planner.FieldSpecRenderer: map[string]any{
			planner.FieldTryItUI:               false,
			planner.FieldTryItInsomnia:         true,
			planner.FieldInfiniteScroll:        false,
			planner.FieldShowSchemas:           true,
			planner.FieldHideInternal:          true,
			planner.FieldHideDeprecated:        false,
			planner.FieldAllowCustomServerURLs: true,
		},
		planner.FieldRobots: "User-agent: *",
	}
	var update kkComps.PortalCustomization

	err := adapter.MapUpdateFields(context.Background(), fields, &update)

	require.NoError(t, err)
	require.NotNil(t, update.SpecRenderer)
	require.NotNil(t, update.SpecRenderer.TryItUI)
	assert.False(t, *update.SpecRenderer.TryItUI)
	require.NotNil(t, update.SpecRenderer.TryItInsomnia)
	assert.True(t, *update.SpecRenderer.TryItInsomnia)
	require.NotNil(t, update.SpecRenderer.InfiniteScroll)
	assert.False(t, *update.SpecRenderer.InfiniteScroll)
	require.NotNil(t, update.SpecRenderer.ShowSchemas)
	assert.True(t, *update.SpecRenderer.ShowSchemas)
	require.NotNil(t, update.SpecRenderer.HideInternal)
	assert.True(t, *update.SpecRenderer.HideInternal)
	require.NotNil(t, update.SpecRenderer.HideDeprecated)
	assert.False(t, *update.SpecRenderer.HideDeprecated)
	require.NotNil(t, update.SpecRenderer.AllowCustomServerUrls)
	assert.True(t, *update.SpecRenderer.AllowCustomServerUrls)
	require.NotNil(t, update.Robots)
	assert.Equal(t, "User-agent: *", *update.Robots)
}
