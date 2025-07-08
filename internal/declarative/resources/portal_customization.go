package resources

import (
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

// PortalCustomizationResource represents portal customization settings
type PortalCustomizationResource struct {
	kkComps.PortalCustomization `yaml:",inline" json:",inline"`
	Ref    string `yaml:"ref,omitempty" json:"ref,omitempty"`
	Portal string `yaml:"portal,omitempty" json:"portal,omitempty"` // Parent portal reference
}

// GetRef returns the reference identifier
func (c PortalCustomizationResource) GetRef() string {
	return c.Ref
}

// Validate ensures the portal customization resource is valid
func (c PortalCustomizationResource) Validate() error {
	if c.Ref == "" {
		return fmt.Errorf("customization ref is required")
	}
	
	// Theme validation
	if c.Theme != nil {
		if c.Theme.Colors != nil && c.Theme.Colors.Primary != nil {
			// Validate hex color format
			if !isValidHexColor(*c.Theme.Colors.Primary) {
				return fmt.Errorf("invalid theme primary color: must be a valid hex color")
			}
		}
	}
	
	// Menu validation
	if c.Menu != nil {
		// Validate menu items
		if c.Menu.Main != nil {
			for i, item := range c.Menu.Main {
				if item.Path == "" {
					return fmt.Errorf("menu item %d: path is required", i)
				}
				if item.Title == "" {
					return fmt.Errorf("menu item %d: title is required", i)
				}
			}
		}
	}
	
	return nil
}

// isValidHexColor validates hex color format
func isValidHexColor(color string) bool {
	if len(color) != 7 && len(color) != 4 {
		return false
	}
	if color[0] != '#' {
		return false
	}
	for i := 1; i < len(color); i++ {
		c := color[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
	}
	return true
}