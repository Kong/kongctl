package executor

import (
	"context"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
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
	update *kkComps.PortalCustomization,
) error {
	// Handle theme
	if themeData, ok := fields["theme"].(map[string]any); ok {
		theme := &kkComps.Theme{}

		if name, ok := themeData["name"].(string); ok {
			theme.Name = &name
		}
		if mode, ok := themeData["mode"].(string); ok {
			modeValue := kkComps.Mode(mode)
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
	if layout, ok := fields["layout"].(string); ok {
		update.Layout = &layout
	}

	// Handle CSS
	if css, ok := fields["css"].(string); ok {
		update.CSS = &css
	}

	// Handle menu
	if menuData, ok := fields["menu"].(map[string]any); ok {
		menu := &kkComps.Menu{}

		// Handle main menu items
		if mainItems, ok := menuData["main"].([]map[string]any); ok {
			var mainMenu []kkComps.PortalMenuItem
			for _, itemMap := range mainItems {
				menuItem := kkComps.PortalMenuItem{
					Path:  itemMap["path"].(string),
					Title: itemMap["title"].(string),
				}

				if visibility, ok := itemMap["visibility"].(string); ok {
					visValue := kkComps.Visibility(visibility)
					menuItem.Visibility = visValue
				}
				if external, ok := itemMap["external"].(bool); ok {
					menuItem.External = external
				}

				mainMenu = append(mainMenu, menuItem)
			}
			menu.Main = mainMenu
		} else if mainItemsInterface, ok := menuData["main"].([]any); ok {
			// Handle []any case
			var mainMenu []kkComps.PortalMenuItem
			for _, item := range mainItemsInterface {
				if itemMap, ok := item.(map[string]any); ok {
					menuItem := kkComps.PortalMenuItem{
						Path:  itemMap["path"].(string),
						Title: itemMap["title"].(string),
					}

					if visibility, ok := itemMap["visibility"].(string); ok {
						visValue := kkComps.Visibility(visibility)
						menuItem.Visibility = visValue
					}
					if external, ok := itemMap["external"].(bool); ok {
						menuItem.External = external
					}

					mainMenu = append(mainMenu, menuItem)
				}
			}
			menu.Main = mainMenu
		}

		// Handle footer sections
		if footerSections, ok := menuData["footer_sections"].([]map[string]any); ok {
			var footerSectionsList []kkComps.PortalFooterMenuSection
			for _, sectionMap := range footerSections {
				footerSection := kkComps.PortalFooterMenuSection{
					Title: sectionMap["title"].(string),
				}

				// Process items in the section
				if items, ok := sectionMap["items"].([]map[string]any); ok {
					var sectionItems []kkComps.PortalMenuItem
					for _, itemMap := range items {
						footerItem := kkComps.PortalMenuItem{
							Path:  itemMap["path"].(string),
							Title: itemMap["title"].(string),
						}

						if visibility, ok := itemMap["visibility"].(string); ok {
							visValue := kkComps.Visibility(visibility)
							footerItem.Visibility = visValue
						}
						if external, ok := itemMap["external"].(bool); ok {
							footerItem.External = external
						}

						sectionItems = append(sectionItems, footerItem)
					}
					footerSection.Items = sectionItems
				} else if itemsInterface, ok := sectionMap["items"].([]any); ok {
					// Handle []any case
					var sectionItems []kkComps.PortalMenuItem
					for _, item := range itemsInterface {
						if itemMap, ok := item.(map[string]any); ok {
							footerItem := kkComps.PortalMenuItem{
								Path:  itemMap["path"].(string),
								Title: itemMap["title"].(string),
							}

							if visibility, ok := itemMap["visibility"].(string); ok {
								visValue := kkComps.Visibility(visibility)
								footerItem.Visibility = visValue
							}
							if external, ok := itemMap["external"].(bool); ok {
								footerItem.External = external
							}

							sectionItems = append(sectionItems, footerItem)
						}
					}
					footerSection.Items = sectionItems
				}

				footerSectionsList = append(footerSectionsList, footerSection)
			}
			menu.FooterSections = footerSectionsList
		} else if footerSectionsInterface, ok := menuData["footer_sections"].([]any); ok {
			// Handle []any case
			var footerSectionsList []kkComps.PortalFooterMenuSection
			for _, section := range footerSectionsInterface {
				if sectionMap, ok := section.(map[string]any); ok {
					footerSection := kkComps.PortalFooterMenuSection{
						Title: sectionMap["title"].(string),
					}

					// Process items - handle both types
					var sectionItems []kkComps.PortalMenuItem
					if items, ok := sectionMap["items"].([]map[string]any); ok {
						for _, itemMap := range items {
							footerItem := kkComps.PortalMenuItem{
								Path:  itemMap["path"].(string),
								Title: itemMap["title"].(string),
							}

							if visibility, ok := itemMap["visibility"].(string); ok {
								visValue := kkComps.Visibility(visibility)
								footerItem.Visibility = visValue
							}
							if external, ok := itemMap["external"].(bool); ok {
								footerItem.External = external
							}

							sectionItems = append(sectionItems, footerItem)
						}
					} else if itemsInterface, ok := sectionMap["items"].([]any); ok {
						for _, item := range itemsInterface {
							if itemMap, ok := item.(map[string]any); ok {
								footerItem := kkComps.PortalMenuItem{
									Path:  itemMap["path"].(string),
									Title: itemMap["title"].(string),
								}

								if visibility, ok := itemMap["visibility"].(string); ok {
									visValue := kkComps.Visibility(visibility)
									footerItem.Visibility = visValue
								}
								if external, ok := itemMap["external"].(bool); ok {
									footerItem.External = external
								}

								sectionItems = append(sectionItems, footerItem)
							}
						}
					}
					footerSection.Items = sectionItems
					footerSectionsList = append(footerSectionsList, footerSection)
				}
			}
			menu.FooterSections = footerSectionsList
		}

		update.Menu = menu
	}

	return nil
}

// Update updates the portal customization
func (p *PortalCustomizationAdapter) Update(ctx context.Context, portalID string,
	req kkComps.PortalCustomization,
) error {
	return p.client.UpdatePortalCustomization(ctx, portalID, req)
}

// ResourceType returns the resource type name
func (p *PortalCustomizationAdapter) ResourceType() string {
	return "portal_customization"
}
