package resources

import (
	"fmt"
	"reflect"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

// ControlPlaneResource represents a control plane in declarative configuration
type ControlPlaneResource struct {
	kkComps.CreateControlPlaneRequest `             yaml:",inline"           json:",inline"`
	Ref                               string                   `yaml:"ref"               json:"ref"`
	Kongctl                           *KongctlMeta             `yaml:"kongctl,omitempty" json:"kongctl,omitempty"`
	External                          *ExternalBlock           `yaml:"_external,omitempty" json:"_external,omitempty"`
	GatewayServices                   []GatewayServiceResource `yaml:"gateway_services,omitempty" json:"gateway_services,omitempty"` //nolint:lll

	// Resolved Konnect ID (not serialized)
	konnectID string `yaml:"-" json:"-"`
}

// GetRef returns the reference identifier used for cross-resource references
func (c ControlPlaneResource) GetRef() string {
	return c.Ref
}

// GetReferenceFieldMappings returns the field mappings for reference validation
func (c ControlPlaneResource) GetReferenceFieldMappings() map[string]string {
	return map[string]string{} // No outbound references
}

// Validate ensures the control plane resource is valid
func (c ControlPlaneResource) Validate() error {
	if err := ValidateRef(c.Ref); err != nil {
		return fmt.Errorf("invalid control plane ref: %w", err)
	}

	if c.External != nil {
		if err := c.External.Validate(); err != nil {
			return fmt.Errorf("invalid _external block: %w", err)
		}
	}
	return nil
}

// SetDefaults applies default values to control plane resource
func (c *ControlPlaneResource) SetDefaults() {
	// If Name is not set, use ref as default
	if c.Name == "" {
		c.Name = c.Ref
	}

	for i := range c.GatewayServices {
		c.GatewayServices[i].SetDefaults()
	}
}

// GetType returns the resource type
func (c ControlPlaneResource) GetType() ResourceType {
	return ResourceTypeControlPlane
}

// GetMoniker returns the resource moniker (for control planes, this is the name)
func (c ControlPlaneResource) GetMoniker() string {
	return c.Name
}

// GetDependencies returns references to other resources this control plane depends on
func (c ControlPlaneResource) GetDependencies() []ResourceRef {
	// Control planes don't depend on other resources
	return []ResourceRef{}
}

// GetKonnectID returns the resolved Konnect ID if available
func (c ControlPlaneResource) GetKonnectID() string {
	return c.konnectID
}

// GetKonnectMonikerFilter returns the filter string for Konnect API lookup
func (c ControlPlaneResource) GetKonnectMonikerFilter() string {
	if c.IsExternal() {
		return ""
	}

	if c.Name == "" {
		return ""
	}
	return fmt.Sprintf("name[eq]=%s", c.Name)
}

// TryMatchKonnectResource attempts to match this resource with a Konnect resource
func (c *ControlPlaneResource) TryMatchKonnectResource(konnectResource any) bool {
	// Use reflection to access fields from state.ControlPlane
	v := reflect.ValueOf(konnectResource)

	// Handle pointer types
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	// Ensure we have a struct
	if v.Kind() != reflect.Struct {
		return false
	}

	// Look for ID field for matching
	idField := v.FieldByName("ID")
	if !idField.IsValid() {
		return false
	}

	if c.IsExternal() && c.External != nil {
		matched := false
		if c.External.ID != "" {
			matched = (idField.String() == c.External.ID)
		} else if c.External.Selector != nil {
			matched = c.External.Selector.Match(konnectResource)
		}

		if matched {
			c.konnectID = idField.String()
			return true
		}

		return false
	}

	// Non-external control planes match by name
	nameField := v.FieldByName("Name")
	if nameField.IsValid() && nameField.Kind() == reflect.String &&
		nameField.String() == c.Name {
		c.konnectID = idField.String()
		return true
	}

	return false
}

// IsExternal returns true if this control plane is externally managed
func (c *ControlPlaneResource) IsExternal() bool {
	return c.External != nil && c.External.IsExternal()
}
