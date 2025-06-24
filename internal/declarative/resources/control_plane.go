package resources

import (
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

// ControlPlaneResource represents a control plane in declarative configuration
type ControlPlaneResource struct {
	kkComps.CreateControlPlaneRequest `yaml:",inline"`
	Ref     string       `yaml:"ref"`
	Kongctl *KongctlMeta `yaml:"kongctl,omitempty"`
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
	if c.Ref == "" {
		return fmt.Errorf("control plane ref is required")
	}
	return nil
}

// SetDefaults applies default values to control plane resource
func (c *ControlPlaneResource) SetDefaults() {
	// If Name is not set, use ref as default
	if c.Name == "" {
		c.Name = c.Ref
	}
}