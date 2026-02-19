package resources

import (
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

func init() {
	registerResourceType(
		ResourceTypePortalCustomization,
		func(rs *ResourceSet) *[]PortalCustomizationResource { return &rs.PortalCustomizations },
	)
}

// PortalCustomizationResource represents portal customization settings
type PortalCustomizationResource struct {
	kkComps.PortalCustomization `       yaml:",inline"          json:",inline"`
	Ref                         string `yaml:"ref,omitempty"    json:"ref,omitempty"`
	Portal                      string `yaml:"portal,omitempty" json:"portal,omitempty"` // Parent portal reference

	// Resolved Konnect ID (not serialized)
	konnectID string `yaml:"-" json:"-"`
}

// GetRef returns the reference identifier
func (c PortalCustomizationResource) GetRef() string {
	return c.Ref
}

// Validate ensures the portal customization resource is valid
func (c PortalCustomizationResource) Validate() error {
	if err := ValidateRef(c.Ref); err != nil {
		return fmt.Errorf("invalid customization ref: %w", err)
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

// SetDefaults applies default values
func (c *PortalCustomizationResource) SetDefaults() {
	// No defaults needed for customizations currently
}

// GetType returns the resource type
func (c PortalCustomizationResource) GetType() ResourceType {
	return ResourceTypePortalCustomization
}

// GetMoniker returns the resource moniker (for customizations, this is the ref)
func (c PortalCustomizationResource) GetMoniker() string {
	return c.Ref // Customizations don't have names
}

// GetDependencies returns references to other resources this customization depends on
func (c PortalCustomizationResource) GetDependencies() []ResourceRef {
	// Portal customizations don't have dependencies
	return []ResourceRef{}
}

// GetKonnectID returns the resolved Konnect ID if available
func (c PortalCustomizationResource) GetKonnectID() string {
	return c.konnectID
}

// GetKonnectMonikerFilter returns the filter string for Konnect API lookup
func (c PortalCustomizationResource) GetKonnectMonikerFilter() string {
	// Customizations don't support filtering
	return ""
}

// TryMatchKonnectResource attempts to match this resource with a Konnect resource
func (c *PortalCustomizationResource) TryMatchKonnectResource(_ any) bool {
	// Portal customizations are matched through parent portal
	return false
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
