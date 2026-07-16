package executor

import (
	"context"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPortalCustomizationAdapterMapUpdateFieldsMenuMain_MapSlice(t *testing.T) {
	adapter := NewPortalCustomizationAdapter(nil)
	fields := map[string]any{
		planner.FieldMenu: map[string]any{
			"main": []map[string]any{
				{
					"path":                  "/docs",
					planner.FieldTitle:      "Docs",
					planner.FieldVisibility: "public",
					"external":              false,
				},
			},
		},
	}
	var update kkComps.PortalCustomization

	err := adapter.MapUpdateFields(context.Background(), fields, &update)

	require.NoError(t, err)
	require.NotNil(t, update.Menu)
	require.Len(t, update.Menu.Main, 1)
	assert.Equal(t, "/docs", update.Menu.Main[0].Path)
	assert.Equal(t, "Docs", update.Menu.Main[0].Title)
	assert.Equal(t, kkComps.PortalMenuItemVisibility("public"), update.Menu.Main[0].Visibility)
	assert.False(t, update.Menu.Main[0].External)
}

func TestPortalCustomizationAdapterMapUpdateFieldsMenuMain_AnySlice(t *testing.T) {
	adapter := NewPortalCustomizationAdapter(nil)
	fields := map[string]any{
		planner.FieldMenu: map[string]any{
			"main": []any{
				map[string]any{
					"path":                  "/blog",
					planner.FieldTitle:      "Blog",
					planner.FieldVisibility: "private",
					"external":              true,
				},
			},
		},
	}
	var update kkComps.PortalCustomization

	err := adapter.MapUpdateFields(context.Background(), fields, &update)

	require.NoError(t, err)
	require.NotNil(t, update.Menu)
	require.Len(t, update.Menu.Main, 1)
	assert.Equal(t, "/blog", update.Menu.Main[0].Path)
	assert.Equal(t, "Blog", update.Menu.Main[0].Title)
	assert.Equal(t, kkComps.PortalMenuItemVisibility("private"), update.Menu.Main[0].Visibility)
	assert.True(t, update.Menu.Main[0].External)
}

func TestPortalCustomizationAdapterMapUpdateFieldsMenuFooterSections_MapSlice(t *testing.T) {
	adapter := NewPortalCustomizationAdapter(nil)
	fields := map[string]any{
		planner.FieldMenu: map[string]any{
			"footer_sections": []map[string]any{
				{
					planner.FieldTitle: "Company",
					"items": []map[string]any{
						{
							"path":                  "/about",
							planner.FieldTitle:      "About",
							planner.FieldVisibility: "public",
							"external":              false,
						},
					},
				},
			},
		},
	}
	var update kkComps.PortalCustomization

	err := adapter.MapUpdateFields(context.Background(), fields, &update)

	require.NoError(t, err)
	require.NotNil(t, update.Menu)
	require.Len(t, update.Menu.FooterSections, 1)
	assert.Equal(t, "Company", update.Menu.FooterSections[0].Title)
	require.Len(t, update.Menu.FooterSections[0].Items, 1)
	assert.Equal(t, "/about", update.Menu.FooterSections[0].Items[0].Path)
	assert.Equal(t, "About", update.Menu.FooterSections[0].Items[0].Title)
}

func TestPortalCustomizationAdapterMapUpdateFieldsMenuFooterSections_AnySlice(t *testing.T) {
	adapter := NewPortalCustomizationAdapter(nil)
	fields := map[string]any{
		planner.FieldMenu: map[string]any{
			"footer_sections": []any{
				map[string]any{
					planner.FieldTitle: "Legal",
					"items": []any{
						map[string]any{
							"path":                  "/privacy",
							planner.FieldTitle:      "Privacy",
							planner.FieldVisibility: "public",
							"external":              false,
						},
					},
				},
			},
		},
	}
	var update kkComps.PortalCustomization

	err := adapter.MapUpdateFields(context.Background(), fields, &update)

	require.NoError(t, err)
	require.NotNil(t, update.Menu)
	require.Len(t, update.Menu.FooterSections, 1)
	assert.Equal(t, "Legal", update.Menu.FooterSections[0].Title)
	require.Len(t, update.Menu.FooterSections[0].Items, 1)
	assert.Equal(t, "/privacy", update.Menu.FooterSections[0].Items[0].Path)
	assert.Equal(t, "Privacy", update.Menu.FooterSections[0].Items[0].Title)
}

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
