package executor

import (
	"context"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
)

// PortalCustomizationAdapter implements SingletonOperations for portal customization
// Portal customization is a singleton resource that always exists and only supports updates
type PortalCustomizationAdapter struct {
	client *state.Client
}

// NewPortalCustomizationAdapter creates a new portal customization adapter
func NewPortalCustomizationAdapter(client *state.Client) *PortalCustomizationAdapter {
	return &PortalCustomizationAdapter{client: client}
}

// MapUpdateFields maps fields to PortalCustomization
func (p *PortalCustomizationAdapter) MapUpdateFields(_ context.Context, fields map[string]any,
	update *kkComps.PortalCustomizationV3,
) error {
	// Handle theme
	if themeData, ok := fields[planner.FieldTheme].(map[string]any); ok {
		theme := &kkComps.Theme{}

		if name, ok := themeData[planner.FieldName].(string); ok {
			theme.Name = &name
		}
		if mode, ok := themeData["mode"].(string); ok {
			modeValue := kkComps.PortalCustomizationV3Mode(mode)
			theme.Mode = &modeValue
		}

		// Handle colors
		if colorsData, ok := themeData["colors"].(map[string]any); ok {
			colors := &kkComps.Colors{}
			if primary, ok := colorsData["primary"].(string); ok {
				colors.Primary = &primary
			}
			theme.Colors = colors
		}

		update.Theme = theme
	}

	// Handle layout
	if layout, ok := fields[planner.FieldLayout].(string); ok {
		update.Layout = &layout
	}

	// Handle CSS
	if css, ok := fields[planner.FieldCSS].(string); ok {
		update.CSS = &css
	}

	// Handle menu
	if menuData, ok := fields[planner.FieldMenu].(map[string]any); ok {
		menu := &kkComps.Menu{}

		if mainItems := toAnySlice(menuData["main"]); mainItems != nil {
			menu.Main = mapPortalMenuItems(mainItems)
		}
		if footerItems := toAnySlice(menuData["footer_sections"]); footerItems != nil {
			menu.FooterSections = mapFooterSections(footerItems)
		}

		update.Menu = menu
	}

	if specRendererData, ok := fields[planner.FieldSpecRenderer].(map[string]any); ok {
		specRenderer := &kkComps.SpecRenderer{}

		if tryItUI, ok := specRendererData[planner.FieldTryItUI].(bool); ok {
			specRenderer.TryItUI = &tryItUI
		}
		if tryItInsomnia, ok := specRendererData[planner.FieldTryItInsomnia].(bool); ok {
			specRenderer.TryItInsomnia = &tryItInsomnia
		}
		if infiniteScroll, ok := specRendererData[planner.FieldInfiniteScroll].(bool); ok {
			specRenderer.InfiniteScroll = &infiniteScroll
		}
		if showSchemas, ok := specRendererData[planner.FieldShowSchemas].(bool); ok {
			specRenderer.ShowSchemas = &showSchemas
		}
		if hideInternal, ok := specRendererData[planner.FieldHideInternal].(bool); ok {
			specRenderer.HideInternal = &hideInternal
		}
		if hideDeprecated, ok := specRendererData[planner.FieldHideDeprecated].(bool); ok {
			specRenderer.HideDeprecated = &hideDeprecated
		}
		if allowCustomServerURLs, ok := specRendererData[planner.FieldAllowCustomServerURLs].(bool); ok {
			specRenderer.AllowCustomServerUrls = &allowCustomServerURLs
		}

		update.SpecRenderer = specRenderer
	}

	if robots, ok := fields[planner.FieldRobots].(string); ok {
		update.Robots = &robots
	}

	return nil
}

// Update updates the portal customization
func (p *PortalCustomizationAdapter) Update(ctx context.Context, portalID string,
	req kkComps.PortalCustomizationV3,
) error {
	return p.client.UpdatePortalCustomization(ctx, portalID, req)
}

// ResourceType returns the resource type name
func (p *PortalCustomizationAdapter) ResourceType() string {
	return planner.ResourceTypePortalCustomization
}

// toAnySlice normalizes []map[string]any or []any to []any, returning nil for any other type.
func toAnySlice(v any) []any {
	switch s := v.(type) {
	case []any:
		return s
	case []map[string]any:
		result := make([]any, len(s))
		for i, m := range s {
			result[i] = m
		}
		return result
	}
	return nil
}

func mapPortalMenuItems(raw []any) []kkComps.PortalMenuItem {
	var items []kkComps.PortalMenuItem
	for _, entry := range raw {
		itemMap, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		menuItem := kkComps.PortalMenuItem{
			Path:  itemMap["path"].(string),
			Title: itemMap[planner.FieldTitle].(string),
		}
		if visibility, ok := itemMap[planner.FieldVisibility].(string); ok {
			menuItem.Visibility = kkComps.Visibility(visibility)
		}
		if external, ok := itemMap["external"].(bool); ok {
			menuItem.External = external
		}
		items = append(items, menuItem)
	}
	return items
}

func mapFooterSections(raw []any) []kkComps.PortalFooterMenuSection {
	var sections []kkComps.PortalFooterMenuSection
	for _, entry := range raw {
		sectionMap, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		section := kkComps.PortalFooterMenuSection{
			Title: sectionMap[planner.FieldTitle].(string),
		}
		if items := toAnySlice(sectionMap["items"]); items != nil {
			section.Items = mapPortalMenuItems(items)
		}
		sections = append(sections, section)
	}
	return sections
}
